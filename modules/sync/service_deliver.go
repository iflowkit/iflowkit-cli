package sync

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iflowkit/iflowkit-cli/internal/app"
	"github.com/iflowkit/iflowkit-cli/internal/common/cpix"
	"github.com/iflowkit/iflowkit-cli/internal/logging"
	"github.com/iflowkit/iflowkit-cli/internal/models"
)

func runSyncDeliver(ctx *app.Context, args []string) (retErr error) {
	fs := flag.NewFlagSet("iflowkit sync deliver", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var to string
	var message string
	fs.StringVar(&to, "to", "", "Target environment (qas|prd)")
	fs.StringVar(&message, "message", "", "Optional message appended to generated commit messages")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		syncDeliverHelp(ctx)
		return err
	}
	to = strings.ToLower(strings.TrimSpace(to))
	message = strings.TrimSpace(message)
	if to != "qas" && to != "prd" {
		return fmt.Errorf("--to must be qas or prd")
	}
	// Safety: PRD requires explicit confirmation. For deliver, --to prd is the confirmation.
	if to == "prd" {
		if err := validateToFlag("prd", "prd"); err != nil {
			return err
		}
	}

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

	levels := meta.CPITenantLevels
	if to == "qas" && levels != 3 {
		return fmt.Errorf("qas deliver is not enabled: cpiTenantLevels=%d (expected 3)", levels)
	}
	if to == "prd" && levels != 2 && levels != 3 {
		return fmt.Errorf("invalid cpiTenantLevels=%d (expected 2 or 3)", levels)
	}

	// Determine promotion path.
	sourceBranch := "dev"
	targetBranch := to
	if to == "prd" && levels == 3 {
		sourceBranch = "qas"
	}

	ctx.Logger.Info("sync deliver started", logging.F("repo", repoRoot), logging.F("to", to), logging.F("from", sourceBranch), logging.F("packageId", meta.PackageID))

	originalBranch, _ := gitCurrentBranch(ctx, repoRoot)
	defer func() {
		if originalBranch == "" {
			return
		}
		_ = runGit(ctx, repoRoot, "checkout", originalBranch)
	}()

	// Require a clean working tree to avoid accidental merges.
	if dirty := gitPorcelainPaths(ctx, repoRoot); len(dirty) > 0 {
		return fmt.Errorf("working tree is not clean (%d paths). commit/stash changes before running deliver", len(dirty))
	}

	ign, err := LoadRepoIgnore(repoRoot)
	if err != nil {
		return err
	}

	// Ensure env branches exist on origin when required.
	if levels == 3 {
		// qas branch should exist when we either deliver to qas or use it as a source for prd.
		if to == "qas" || sourceBranch == "qas" {
			if err := ensureEnvBranchOnRemote(ctx, repoRoot, meta, "qas"); err != nil {
				return err
			}
		}
	}
	if to == "prd" {
		if err := ensureEnvBranchOnRemote(ctx, repoRoot, meta, "prd"); err != nil {
			return err
		}
	}

	// Always refresh source branch.
	if err := ensureBranchFetchedAndCheckedOut(ctx, repoRoot, sourceBranch); err != nil {
		return err
	}
	// And target branch.
	if err := ensureBranchFetchedAndCheckedOut(ctx, repoRoot, targetBranch); err != nil {
		return err
	}

	// Transport bookkeeping.
	transportTouched := false
	transportID := ""
	deliverSucceeded := false
	defer func() {
		if !transportTouched {
			return
		}
		msg := buildTransportCommitMessage(transportID, "deliver", "logs", message)
		if err := gitCommitAndPushLogs(ctx, repoRoot, targetBranch, msg); err != nil {
			if retErr == nil {
				retErr = err
				return
			}
			ctx.Logger.Warn("failed to push .iflowkit metadata", logging.F("error", err.Error()))
			return
		}
		if retErr != nil || !deliverSucceeded {
			return
		}
		tagger := NewGitTagger(repoRoot)
		if err := tagger.TagBranchWithTransportID(ctx, targetBranch, transportID); err != nil {
			if retErr == nil {
				retErr = err
				return
			}
			ctx.Logger.Warn("failed to create/push git tag", logging.F("tag", transportTagName(transportID, targetBranch)), logging.F("error", err.Error()))
		}
	}()

	store, err := NewTransportStore(repoRoot, to)
	if err != nil {
		return err
	}
	// Retry support: if there is a pending deliver record, continue CPI work.
	pendingRec, pendingPath, hasPending, err := store.LoadLatestPendingTransport(meta.PackageID, targetBranch, "deliver")
	if err != nil {
		return err
	}
	var rec TransportRecord
	if hasPending {
		rec = *pendingRec
		transportTouched = true
		transportID = rec.TransportID
		ctx.Logger.Info("resuming pending deliver transport", logging.F("transportId", rec.TransportID), logging.F("record", filepath.ToSlash(strings.TrimPrefix(pendingPath, repoRoot+string(os.PathSeparator)))))
		// Ensure we are on the target branch tip.
		if err := ensureBranchFetchedAndCheckedOut(ctx, repoRoot, targetBranch); err != nil {
			return err
		}
	} else {
		// Preflight: tenant must match target branch (ignoring .iflowkit/ignore patterns).
		if err := ensureBranchFetchedAndCheckedOut(ctx, repoRoot, targetBranch); err != nil {
			return err
		}
		eq, diffPaths, err := compareTenantWithCurrentBranch(ctx, repoRoot, meta, to, ign)
		if err != nil {
			return err
		}
		if !eq {
			return fmt.Errorf("%s tenant and %s branch differ (after applying .iflowkit/ignore). first diffs: %s", tenantDisplay(to), targetBranch, strings.Join(samplePaths(diffPaths, 10), ", "))
		}

		// Create a new transport id so merge commit uses the strict format.
		id, createdAt := newTransportIDs(time.Now())
		transportID = id

		// Merge source -> target.
		if err := ensureBranchFetchedAndCheckedOut(ctx, repoRoot, sourceBranch); err != nil {
			return err
		}
		if err := ensureBranchFetchedAndCheckedOut(ctx, repoRoot, targetBranch); err != nil {
			return err
		}
		preMerge, _ := runGitOutput(ctx, repoRoot, "rev-parse", "HEAD")
		preMerge = strings.TrimSpace(preMerge)
		mergeMsg := buildTransportCommitMessage(transportID, "deliver", "contents", message)
		ctx.Logger.Info("merging branches", logging.F("from", sourceBranch), logging.F("to", targetBranch), logging.F("transportId", transportID))
		if err := runGit(ctx, repoRoot, "merge", "--no-ff", "-m", mergeMsg, sourceBranch); err != nil {
			// Best-effort cleanup so the user doesn't remain stuck in a conflicted state.
			_ = runGit(ctx, repoRoot, "merge", "--abort")
			return err
		}

		// Compute changed artifact set for CPI based on IntegrationPackage diffs.
		baseFolder := resolveContentFolder(meta)
		diffOut, _ := runGitOutput(ctx, repoRoot, "diff", "--name-only", preMerge+"..HEAD", "--", baseFolder)
		changedPaths := splitLines(diffOut)
		changedPaths = ign.Filter(changedPaths)
		keysChanged := detectChangedArtifacts(meta, changedPaths)
		toUpload, toDelete := partitionChangedKeys(repoRoot, meta, keysChanged)
		objs := keysToObjects(toUpload)
		deletedObjs := keysToObjects(toDelete)

		// Determine commits to push (oldest->newest).
		_ = runGit(ctx, repoRoot, "fetch", "origin", targetBranch)
		remoteRef := "origin/" + targetBranch
		commitsToPush := []string{}
		if gitRemoteBranchExists(ctx, repoRoot, targetBranch) {
			if out, err := runGitOutput(ctx, repoRoot, "rev-list", "--reverse", remoteRef+"..HEAD"); err == nil {
				commitsToPush = splitLines(out)
			}
		} else {
			if out, err := runGitOutput(ctx, repoRoot, "rev-list", "--max-count=1", "HEAD"); err == nil {
				commitsToPush = splitLines(out)
			}
		}

		gitUserName, gitUserEmail, _ := gitUserIdentity(ctx, repoRoot)
		rec = TransportRecord{
			SchemaVersion:   1,
			TransportID:     transportID,
			TransportType:   "deliver",
			PackageID:       meta.PackageID,
			Branch:          targetBranch,
			CreatedAt:       createdAt,
			GitCommits:      commitsToPush,
			GitUserName:     gitUserName,
			GitUserEmail:    gitUserEmail,
			Objects:         objs,
			DeletedObjects:  deletedObjs,
			TransportStatus: "pending",
			UploadRemaining: mapKeysToSortedSlice(toUpload),
			DeleteRemaining: mapKeysToSortedSlice(toDelete),
			DeployRemaining: nil,
		}

		// Persist plan before CPI work.
		recPath, err := store.PersistTransportRecord(rec)
		if err != nil {
			return err
		}
		transportTouched = true
		ctx.Logger.Info("deliver transport record created", logging.F("path", filepath.ToSlash(strings.TrimPrefix(recPath, repoRoot+string(os.PathSeparator)))), logging.F("upload", len(rec.UploadRemaining)), logging.F("delete", len(rec.DeleteRemaining)))

		// Push target branch after merge.
		if err := runGit(ctx, repoRoot, "push", "origin", targetBranch); err != nil {
			rec.TransportStatus = "pending"
			rec.Error = err.Error()
			_, _ = store.PersistTransportRecord(rec)
			return err
		}
	}

	// CPI phase.
	deleted, updated, deployed, err := applyTransportToTenant(ctx, repoRoot, meta, to, &rec, store)
	if err != nil {
		return err
	}
	deliverSucceeded = true

	// Persist the latest record state so the deferred logs commit includes it.
	if _, err := store.PersistTransportRecord(rec); err != nil {
		return err
	}

	fmt.Fprintf(ctx.Stdout, "Sync deliver completed. Updated CPI %s: deleted %d, updated %d, deployed %d. Target branch: %s. Transport: %s\n", tenantDisplay(to), deleted, updated, deployed, targetBranch, transportID)
	ctx.Logger.Info("sync deliver completed", logging.F("to", to), logging.F("from", sourceBranch), logging.F("branch", targetBranch), logging.F("transportId", transportID), logging.F("deletedArtifacts", deleted), logging.F("updatedArtifacts", updated), logging.F("deployedArtifacts", deployed))
	return nil
}

func compareTenantWithCurrentBranch(ctx *app.Context, repoRoot string, meta models.SyncMetadata, tenantEnv string, ign *RepoIgnore) (equal bool, diffPaths []string, err error) {
	// This helper assumes the current checkout is the target branch.
	baseFolder := resolveContentFolder(meta)
	branchBase := filepath.Join(repoRoot, baseFolder)
	if _, err := os.Stat(branchBase); err != nil {
		// Treat missing IntegrationPackage as diff.
		return false, []string{filepath.ToSlash(baseFolder)}, nil
	}

	// Export tenant to a temp folder and compare hashes.
	tmp, err := os.MkdirTemp("", "iflowkit-compare-*")
	if err != nil {
		return false, nil, err
	}
	defer os.RemoveAll(tmp)

	// Resolve tenant and export.
	profileID, source, err := ctx.Stores.ResolveProfileID(ctx.Flags.ProfileID)
	if err != nil {
		return false, nil, err
	}
	_, err = ctx.Stores.Profiles.Read(profileID)
	if err != nil {
		return false, nil, err
	}
	ctx.Logger.Info("resolved profile", logging.F("profile", profileID), logging.F("source", source))

	tenantKey, err := ctx.Stores.Tenants.Read(profileID, tenantEnv)
	if err != nil {
		return false, nil, fmt.Errorf("%s tenant not found for profile %q; import it with `iflowkit tenant import --env %s --file <service-key.json>`: %w", tenantDisplay(tenantEnv), profileID, tenantEnv, err)
	}

	c := cpix.NewClient(tenantKey, ctx.Logger)
	_, raw, err := c.ReadIntegrationPackage(meta.PackageID)
	if err != nil {
		return false, nil, err
	}
	tenantBase := filepath.Join(tmp, baseFolder)
	if err := os.MkdirAll(tenantBase, 0o755); err != nil {
		return false, nil, err
	}
	if err := c.ExportIntegrationPackageFromRaw(meta.PackageID, raw, tenantBase); err != nil {
		return false, nil, err
	}

	diffs, err := CompareFolderTrees(baseFolder, tenantBase, branchBase, ign)
	if err != nil {
		return false, nil, err
	}
	return len(diffs) == 0, diffs, nil
}

func samplePaths(in []string, max int) []string {
	if len(in) <= max {
		return in
	}
	out := make([]string, 0, max)
	for i := 0; i < max; i++ {
		out = append(out, in[i])
	}
	out = append(out, fmt.Sprintf("... (+%d more)", len(in)-max))
	return out
}
