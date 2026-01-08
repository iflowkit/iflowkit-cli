package sync

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/iflowkit/iflowkit-cli/internal/app"
	"github.com/iflowkit/iflowkit-cli/internal/common/cpix"
	"github.com/iflowkit/iflowkit-cli/internal/common/filex"
	"github.com/iflowkit/iflowkit-cli/internal/logging"
)

func runSyncPush(ctx *app.Context, args []string) (retErr error) {
	fs := flag.NewFlagSet("iflowkit sync push", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var message string
	var to string
	fs.StringVar(&message, "message", "", "Optional message appended to generated commit messages")
	fs.StringVar(&to, "to", "", "Confirm target tenant (required for prd)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		syncPushHelp(ctx)
		return err
	}
	message = strings.TrimSpace(message)

	cwd, _ := os.Getwd()
	repoRoot, err := findSyncRepoRoot(cwd)
	if err != nil {
		return err
	}

	meta, err := loadPackageMetadata(repoRoot)
	if err != nil {
		return err
	}
	if err := meta.ValidateRequired(); err != nil {
		return err
	}
	contentPath := resolveContentFolder(meta)

	branch, err := gitCurrentBranch(ctx, repoRoot)
	if err != nil {
		return err
	}
	if !isAllowedPushBranch(branch) {
		return fmt.Errorf("sync push is only allowed on environment branches (dev%s, prd) and work branches (feature/*, bugfix/*). current=%s", func() string {
			if meta.CPITenantLevels == 3 {
				return ", qas"
			}
			return ""
		}(), branch)
	}

	tenant, isEnvBranch, err := resolveTargetTenant(meta, branch)
	if err != nil {
		return err
	}
	if err := validateToFlag(to, tenant); err != nil {
		return err
	}

	ctx.Logger.Info("sync push started", logging.F("repo", repoRoot), logging.F("branch", branch), logging.F("tenant", tenant), logging.F("packageId", meta.PackageID))

	// Ensure .iflowkit (transport records, package.json, etc.) is pushed as well.
	// We commit/push .iflowkit at the end of the command to avoid creating a commit for every progress update.
	transportTouched := false
	transportID := ""      // selected transport id for this run
	plannedCreatedAt := "" // only set when we generate a new transport id before creating the transport record
	pushSucceeded := false
	defer func() {
		if !transportTouched {
			return
		}
		msg := buildTransportCommitMessage(transportID, "push", "logs", message)
		if err := gitCommitAndPushLogs(ctx, repoRoot, branch, msg); err != nil {
			if retErr == nil {
				retErr = err
				return
			}
			ctx.Logger.Warn("failed to push .iflowkit metadata", logging.F("error", err.Error()))
			return
		}

		// Only tag on success, and only for environment branches (dev/qas/prd).
		if retErr != nil || !pushSucceeded || !isEnvBranch {
			return
		}
		tagger := NewGitTagger(repoRoot)
		if err := tagger.TagBranchWithTransportID(ctx, branch, transportID); err != nil {
			if retErr == nil {
				retErr = err
				return
			}
			ctx.Logger.Warn("failed to create/push git tag", logging.F("tag", transportTagName(transportID, branch)), logging.F("error", err.Error()))
		}
	}()

	gitUserName, gitUserEmail, _ := gitUserIdentity(ctx, repoRoot)

	// Prefer resuming an incomplete PUSH transport record (retry after CPI failure).
	store, err := NewTransportStore(repoRoot, tenant)
	if err != nil {
		return err
	}
	pendingRec, pendingPath, hasPending, err := store.LoadLatestPendingTransport(meta.PackageID, branch, "push")
	if err != nil {
		return err
	}
	if hasPending {
		transportID = pendingRec.TransportID
		plannedCreatedAt = pendingRec.CreatedAt
	}

	// --- Git phase ---
	_ = runGit(ctx, repoRoot, "fetch", "origin") // best-effort
	upstreamRef, _ := gitUpstreamRef(ctx, repoRoot)

	// If IntegrationPackage has uncommitted changes, create a strict-format "contents" commit.
	contentDirty, err := gitHasChangesInPath(ctx, repoRoot, contentPath)
	if err != nil {
		return err
	}
	if contentDirty {
		if transportID == "" {
			id, createdAt := newTransportIDs(time.Now())
			transportID = id
			plannedCreatedAt = createdAt
		}
		if err := runGit(ctx, repoRoot, "add", "-A", "--", contentPath); err != nil {
			return err
		}
		msg := buildTransportCommitMessage(transportID, "push", "contents", message)
		if err := runGit(ctx, repoRoot, "commit", "-m", msg, "--", contentPath); err != nil {
			if !strings.Contains(err.Error(), "nothing to commit") {
				return err
			}
		}
	}

	// Determine changes that are not yet pushed (committed but ahead of upstream).
	baseRef := upstreamRef
	if baseRef == "" && gitRemoteBranchExists(ctx, repoRoot, branch) {
		baseRef = "origin/" + branch
	}
	changedPaths, commitsToPush, err := gitPendingChanges(ctx, repoRoot, baseRef, branch)
	if err != nil {
		return err
	}
	ign, err := LoadRepoIgnore(repoRoot)
	if err != nil {
		return err
	}
	pathsForObjects := ign.Filter(changedPaths)
	keysFromDiff := detectChangedArtifacts(meta, pathsForObjects)
	keysToUpload, keysToDelete := partitionChangedKeys(repoRoot, meta, keysFromDiff)
	objsFromDiff := keysToObjects(keysToUpload)
	deletedObjsFromDiff := keysToObjects(keysToDelete)

	// If there is nothing to push and nothing to delete and no pending retry, exit.
	if len(keysToUpload) == 0 && len(keysToDelete) == 0 && len(commitsToPush) == 0 && !hasPending {
		fmt.Fprintln(ctx.Stdout, "No changes detected; nothing to do.")
		return nil
	}

	// Push any pending commits (including those created by sync push).
	if len(commitsToPush) > 0 || upstreamRef == "" {
		if upstreamRef == "" {
			// No upstream configured yet (e.g. first push of a feature branch)
			if err := runGit(ctx, repoRoot, "push", "-u", "origin", branch); err != nil {
				return err
			}
		} else {
			if err := runGit(ctx, repoRoot, "push", "origin", branch); err != nil {
				return err
			}
		}
	}

	// --- CPI plan (stored as transport record, used for retry) ---
	var rec TransportRecord
	recPath := ""
	if hasPending {
		rec = *pendingRec
		recPath = pendingPath
		transportTouched = true
		transportID = rec.TransportID
		// merge new work into existing pending record
		rec.GitCommits = mergeStringList(rec.GitCommits, commitsToPush)
		if rec.GitUserName == "" {
			rec.GitUserName = gitUserName
		}
		if rec.GitUserEmail == "" {
			rec.GitUserEmail = gitUserEmail
		}
		if len(keysToUpload) > 0 {
			rec.UploadRemaining = mergeUpload(rec.UploadRemaining, keysToUpload)
			rec.Objects = mergeObjects(rec.Objects, objsFromDiff)
		}
		if len(keysToDelete) > 0 {
			rec.DeleteRemaining = mergeUpload(rec.DeleteRemaining, keysToDelete)
			rec.DeletedObjects = mergeObjects(rec.DeletedObjects, deletedObjsFromDiff)
		}
		// If older pending records missed Objects, rebuild from remaining upload set.
		if len(rec.Objects) == 0 && len(rec.UploadRemaining) > 0 {
			rec.Objects = mergeObjects(rec.Objects, keysToObjectsFromSlice(rec.UploadRemaining))
		}
		if len(rec.DeletedObjects) == 0 && len(rec.DeleteRemaining) > 0 {
			rec.DeletedObjects = mergeObjects(rec.DeletedObjects, keysToObjectsFromSlice(rec.DeleteRemaining))
		}
		if rec.TransportStatus == "completed" {
			rec.TransportStatus = "pending"
		}
	} else {
		if len(keysToUpload) == 0 && len(keysToDelete) == 0 {
			// Git push completed (or nothing to push). No CPI-relevant changes.
			fmt.Fprintln(ctx.Stdout, "Git push completed. No CPI artifact changes detected under IntegrationPackage/.")
			return nil
		}
		newID, createdAt := transportID, plannedCreatedAt
		if newID == "" {
			newID, createdAt = newTransportIDs(time.Now())
		}
		rec = TransportRecord{
			SchemaVersion:   1,
			TransportID:     newID,
			TransportType:   "push",
			PackageID:       meta.PackageID,
			Branch:          branch,
			CreatedAt:       createdAt,
			GitCommits:      commitsToPush,
			GitUserName:     gitUserName,
			GitUserEmail:    gitUserEmail,
			Objects:         objsFromDiff,
			DeletedObjects:  deletedObjsFromDiff,
			TransportStatus: "pending",
			UploadRemaining: mapKeysToSortedSlice(keysToUpload),
			DeleteRemaining: mapKeysToSortedSlice(keysToDelete),
			DeployRemaining: nil,
		}
		transportTouched = true
		transportID = rec.TransportID
	}
	// Persist the (possibly merged) plan before CPI operations.
	recPath, err = store.PersistTransportRecord(rec)
	if err != nil {
		return err
	}

	// --- CPI phase (mapped tenant) ---
	profileID, source, err := ctx.Stores.ResolveProfileID(ctx.Flags.ProfileID)
	if err != nil {
		return err
	}
	_, err = ctx.Stores.Profiles.Read(profileID)
	if err != nil {
		return err
	}
	ctx.Logger.Info("resolved profile", logging.F("profile", profileID), logging.F("source", source))

	tenantKey, err := ctx.Stores.Tenants.Read(profileID, tenant)
	if err != nil {
		return fmt.Errorf("%s tenant not found for profile %q; import it with `iflowkit tenant import --env %s --file <service-key.json>`: %w", tenantDisplay(tenant), profileID, tenant, err)
	}

	client := cpix.NewClient(tenantKey, ctx.Logger)
	csrf, cookies, err := client.FetchCSRFToken(context.Background())
	if err != nil {
		return err
	}

	deleted := 0
	updated := 0
	deployed := 0

	// First, delete artifacts that were removed from the repo.
	orderedDelete := append([]artifactKey{}, rec.DeleteRemaining...)
	sort.Slice(orderedDelete, func(i, j int) bool {
		if orderedDelete[i].Kind == orderedDelete[j].Kind {
			return orderedDelete[i].ID < orderedDelete[j].ID
		}
		return orderedDelete[i].Kind < orderedDelete[j].Kind
	})

	for _, k := range orderedDelete {
		ctx.Logger.Info("deleting artifact from CPI", logging.F("kind", k.Kind), logging.F("id", k.ID), logging.F("version", "active"))
		if err := deleteArtifactInCPI(context.Background(), client, k.Kind, k.ID, csrf, cookies); err != nil {
			rec.TransportStatus = "pending"
			rec.Error = err.Error()
			_, _ = store.PersistTransportRecord(rec)
			return err
		}
		deleted++
		rec.DeleteRemaining = removeUpload(rec.DeleteRemaining, k)
		_, _ = store.PersistTransportRecord(rec)
	}

	// Group by kind so we can fetch CPI lists once per kind.
	byKind := make(map[string][]string)
	for _, k := range rec.UploadRemaining {
		byKind[k.Kind] = append(byKind[k.Kind], k.ID)
	}
	for kind := range byKind {
		sort.Strings(byKind[kind])
	}

	artsByKind := make(map[string]map[string]cpix.ArtifactInfo)
	for kind := range byKind {
		endpoint, ok := listEndpointForKind(meta.PackageID, kind)
		if !ok {
			ctx.Logger.Warn("unknown artifact kind; skipping", logging.F("kind", kind))
			continue
		}
		m, err := client.ListArtifacts(context.Background(), endpoint)
		if err != nil {
			rec.TransportStatus = "pending"
			rec.Error = err.Error()
			_, _ = store.PersistTransportRecord(rec)
			return err
		}
		artsByKind[kind] = m
	}

	// Upload each artifact folder.
	orderedUpload := make([]artifactKey, 0, len(rec.UploadRemaining))
	orderedUpload = append(orderedUpload, rec.UploadRemaining...)
	sort.Slice(orderedUpload, func(i, j int) bool {
		if orderedUpload[i].Kind == orderedUpload[j].Kind {
			return orderedUpload[i].ID < orderedUpload[j].ID
		}
		return orderedUpload[i].Kind < orderedUpload[j].Kind
	})

	for _, k := range orderedUpload {
		arts := artsByKind[k.Kind]
		art, ok := arts[k.ID]
		if !ok {
			ctx.Logger.Warn("artifact not found in CPI list; skipping", logging.F("kind", k.Kind), logging.F("id", k.ID))
			rec.UploadRemaining = removeUpload(rec.UploadRemaining, k)
			_, _ = store.PersistTransportRecord(rec)
			continue
		}

		artifactDir := filepath.Join(repoRoot, meta.BaseFolder, k.Kind, k.ID)
		st, err := os.Stat(artifactDir)
		if err != nil || !st.IsDir() {
			ctx.Logger.Warn("artifact directory missing; skipping", logging.F("dir", artifactDir), logging.F("kind", k.Kind), logging.F("id", k.ID))
			rec.UploadRemaining = removeUpload(rec.UploadRemaining, k)
			_, _ = store.PersistTransportRecord(rec)
			continue
		}

		zipBytes, err := filex.ZipDirToBytes(artifactDir)
		if err != nil {
			rec.TransportStatus = "pending"
			rec.Error = err.Error()
			_, _ = store.PersistTransportRecord(rec)
			return err
		}

		ctx.Logger.Info("uploading artifact to CPI", logging.F("kind", k.Kind), logging.F("id", k.ID))
		entitySet := kindToEntitySet(k.Kind)
		if entitySet == "" {
			ctx.Logger.Warn("artifact kind is not supported for CPI updates; skipping", logging.F("kind", k.Kind), logging.F("id", k.ID))
			rec.UploadRemaining = removeUpload(rec.UploadRemaining, k)
			_, _ = store.PersistTransportRecord(rec)
			continue
		}
		if err := client.UpdateArtifact(context.Background(), entitySet, art, zipBytes, csrf, cookies); err != nil {
			rec.TransportStatus = "pending"
			rec.Error = err.Error()
			_, _ = store.PersistTransportRecord(rec)
			return err
		}
		updated++
		rec.UploadRemaining = removeUpload(rec.UploadRemaining, k)
		// Deploy is triggered per kind via dedicated CPI endpoints.
		switch k.Kind {
		case "iFlows", "Scripts", "ValueMappings", "MessageMappings":
			rec.DeployRemaining = mergeDeployRemaining(rec.DeployRemaining, []deployTarget{{Kind: k.Kind, ID: k.ID}})
		}
		_, _ = store.PersistTransportRecord(rec)
	}

	// Deploy iFlows that were updated (or previously pending).
	orderedDeploy := make([]deployTarget, 0, len(rec.DeployRemaining))
	orderedDeploy = append(orderedDeploy, rec.DeployRemaining...)
	sort.Slice(orderedDeploy, func(i, j int) bool {
		if orderedDeploy[i].Kind == orderedDeploy[j].Kind {
			return orderedDeploy[i].ID < orderedDeploy[j].ID
		}
		return orderedDeploy[i].Kind < orderedDeploy[j].Kind
	})

	for _, d := range orderedDeploy {
		ctx.Logger.Info("deploying artifact", logging.F("kind", d.Kind), logging.F("id", d.ID), logging.F("version", "active"))
		var derr error
		switch d.Kind {
		case "iFlows":
			derr = client.DeployIntegrationDesigntimeArtifact(context.Background(), d.ID, "active", csrf, cookies)
		case "Scripts":
			derr = client.DeployScriptCollectionDesigntimeArtifact(context.Background(), d.ID, "active", csrf, cookies)
		case "ValueMappings":
			derr = client.DeployValueMappingDesigntimeArtifact(context.Background(), d.ID, "active", csrf, cookies)
		case "MessageMappings":
			derr = client.DeployMessageMappingDesigntimeArtifact(context.Background(), d.ID, "active", csrf, cookies)
		default:
			ctx.Logger.Warn("deploy kind not supported; skipping", logging.F("kind", d.Kind), logging.F("id", d.ID))
			rec.DeployRemaining = removeDeployTarget(rec.DeployRemaining, d)
			_, _ = store.PersistTransportRecord(rec)
			continue
		}
		if derr != nil {
			rec.TransportStatus = "pending"
			rec.Error = derr.Error()
			_, _ = store.PersistTransportRecord(rec)
			return derr
		}
		deployed++
		rec.DeployRemaining = removeDeployTarget(rec.DeployRemaining, d)
		_, _ = store.PersistTransportRecord(rec)
		ctx.Logger.Info("artifact deployed", logging.F("kind", d.Kind), logging.F("id", d.ID), logging.F("version", "active"))
	}

	rec.TransportStatus = "completed"
	rec.Error = ""
	_, _ = store.PersistTransportRecord(rec)
	pushSucceeded = true

	fmt.Fprintf(ctx.Stdout, "Sync push completed on branch %s. Git pushed (if needed). CPI %s deleted %d artifact(s), updated %d artifact(s) and deployed %d artifact(s). Transport record: %s\n", branch, tenantDisplay(tenant), deleted, updated, deployed, filepath.ToSlash(strings.TrimPrefix(recPath, repoRoot+string(os.PathSeparator))))
	ctx.Logger.Info("sync push completed", logging.F("branch", branch), logging.F("tenant", tenant), logging.F("deletedArtifacts", deleted), logging.F("updatedArtifacts", updated), logging.F("deployedArtifacts", deployed))
	return nil
}
