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
	"github.com/iflowkit/iflowkit-cli/internal/common/filex"
	"github.com/iflowkit/iflowkit-cli/internal/logging"
)

// runSyncPull refreshes the local repo content from CPI (mapped tenant) and pushes the CPI state to origin/<branch>.
// It must be executed within an existing sync repository (contains .iflowkit/package.json).
// Only environment branches are allowed (dev, qas, prd).
func runSyncPull(ctx *app.Context, args []string) (retErr error) {
	fs := flag.NewFlagSet("iflowkit sync pull", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var message string
	var to string
	fs.StringVar(&message, "message", "", "Optional message appended to generated commit messages")
	fs.StringVar(&to, "to", "", "Confirm target tenant (required for prd)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		syncPullHelp(ctx)
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
	tenant, isEnvBranch, err := resolveTargetTenant(meta, branch)
	if err != nil {
		return err
	}
	if !isEnvBranch {
		return fmt.Errorf("sync pull is only allowed on environment branches (dev%s, prd). current=%s", func() string {
			if meta.CPITenantLevels == 3 {
				return ", qas"
			}
			return ""
		}(), branch)
	}
	if err := validateToFlag(to, tenant); err != nil {
		return err
	}

	ctx.Logger.Info("sync pull started", logging.F("repo", repoRoot), logging.F("branch", branch), logging.F("tenant", tenant), logging.F("packageId", meta.PackageID))

	// Ensure .iflowkit (transport records, package.json, etc.) is pushed as well.
	transportTouched := false
	transportID := ""
	defer func() {
		if !transportTouched {
			return
		}
		msg := buildTransportCommitMessage(transportID, "pull", "logs", message)
		if err := gitCommitAndPushLogs(ctx, repoRoot, branch, msg); err != nil {
			if retErr == nil {
				retErr = err
				return
			}
			ctx.Logger.Warn("failed to push .iflowkit metadata", logging.F("error", err.Error()))
		}
	}()

	// --- Git preflight ---
	// 1) Fetch remote state.
	_ = runGit(ctx, repoRoot, "fetch", "origin", branch) // best-effort

	// 2) If local is behind origin/dev, fast-forward. If diverged, stop.
	remoteRef := "origin/" + branch
	if gitRemoteBranchExists(ctx, repoRoot, branch) {
		behind, ahead := gitAheadBehind(ctx, repoRoot, remoteRef, "HEAD")
		if behind > 0 && ahead > 0 {
			return fmt.Errorf("local branch diverged from %s (ahead=%d, behind=%d); resolve with rebase/merge before running sync pull", remoteRef, ahead, behind)
		}
		if behind > 0 {
			if err := runGit(ctx, repoRoot, "merge", "--ff-only", remoteRef); err != nil {
				return err
			}
		}
	}

	// 3) Handle working tree changes.
	changed := gitPorcelainPaths(ctx, repoRoot)
	changed = filterNonTransportChanges(changed)
	if len(changed) > 0 {
		// Safety: stash any local work so we can safely overwrite IntegrationPackage/.
		_, createdAt := newTransportIDs(time.Now())
		msg := fmt.Sprintf("iflowkit sync pull %s", createdAt)
		ctx.Logger.Info("working tree has local changes; stashing", logging.F("paths", len(changed)), logging.F("message", msg))
		if err := runGit(ctx, repoRoot, "stash", "push", "-u", "-m", msg); err != nil {
			return err
		}
		fmt.Fprintf(ctx.Stdout, "Stashed local changes (%d paths). You can restore with: git stash list / git stash pop\n", len(changed))
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

	c := cpix.NewClient(tenantKey, ctx.Logger)
	_, raw, err := c.ReadIntegrationPackage(meta.PackageID)
	if err != nil {
		return err
	}

	// Capture local artifact inventory before overwrite to detect deletions coming from CPI.
	beforeKeys, _ := listLocalArtifactKeys(repoRoot, meta)

	// Re-export CPI content into IntegrationPackage/. Remove old folder first so deletions are reflected.
	baseAbs := filepath.Join(repoRoot, resolveContentFolder(meta))
	if err := os.RemoveAll(baseAbs); err != nil {
		return err
	}
	if err := filex.EnsureDir(baseAbs); err != nil {
		return err
	}
	if err := c.ExportIntegrationPackageFromRaw(meta.PackageID, raw, baseAbs); err != nil {
		return err
	}
	// Capture inventory after export and compute deleted objects.
	afterKeys, _ := listLocalArtifactKeys(repoRoot, meta)
	deletedKeys := setDiff(beforeKeys, afterKeys)

	// Compute changed paths (including untracked) against current HEAD.
	diffOut, _ := runGitOutput(ctx, repoRoot, "diff", "--name-only")
	untrackedOut, _ := runGitOutput(ctx, repoRoot, "ls-files", "--others", "--exclude-standard")
	changedPaths := splitLines(diffOut + "\n" + untrackedOut)

	ign, err := LoadRepoIgnore(repoRoot)
	if err != nil {
		return err
	}

	// Ignore noisy diffs when computing changed CPI objects for the transport record.
	keys := detectChangedArtifacts(meta, ign.Filter(changedPaths))
	keys = setDiff(keys, deletedKeys) // exclude deletions from the "objects" list
	objs := keysToObjects(keys)
	deletedObjs := keysToObjects(deletedKeys)
	if len(changedPaths) == 0 {
		fmt.Fprintf(ctx.Stdout, "Already up to date with CPI %s; no changes to push.\n", tenantDisplay(tenant))
		return nil
	}

	// Create transport id now so the content commit uses the strict message format.
	newID, createdAt := newTransportIDs(time.Now())
	transportID = newID

	// Commit + push the CPI state (IntegrationPackage only) to origin/<branch>.
	if err := runGit(ctx, repoRoot, "add", "-A", "--", contentPath); err != nil {
		return err
	}
	msg := buildTransportCommitMessage(transportID, "pull", "contents", message)
	if err := runGit(ctx, repoRoot, "commit", "-m", msg, "--", contentPath); err != nil {
		if !strings.Contains(err.Error(), "nothing to commit") {
			return err
		}
	}

	// Collect commits to push (if any).
	commitsToPush := []string{}
	if gitRemoteBranchExists(ctx, repoRoot, branch) {
		if out, err := runGitOutput(ctx, repoRoot, "rev-list", "--reverse", remoteRef+"..HEAD"); err == nil {
			commitsToPush = splitLines(out)
		}
	} else {
		if out, err := runGitOutput(ctx, repoRoot, "rev-list", "--max-count=1", "HEAD"); err == nil {
			commitsToPush = splitLines(out)
		}
	}

	gitUserName, gitUserEmail, _ := gitUserIdentity(ctx, repoRoot)
	rec := TransportRecord{
		SchemaVersion:   1,
		TransportID:     newID,
		TransportType:   "pull",
		PackageID:       meta.PackageID,
		Branch:          branch,
		CreatedAt:       createdAt,
		GitCommits:      commitsToPush,
		GitUserName:     gitUserName,
		GitUserEmail:    gitUserEmail,
		Objects:         objs,
		DeletedObjects:  deletedObjs,
		TransportStatus: "pending",
		UploadRemaining: nil,
		DeployRemaining: nil,
	}

	store, err := NewTransportStore(repoRoot, tenant)
	if err != nil {
		return err
	}
	recPath, err := store.PersistTransportRecord(rec)
	if err != nil {
		return err
	}
	transportTouched = true
	transportID = newID

	if err := runGit(ctx, repoRoot, "push", "origin", branch); err != nil {
		rec.TransportStatus = "pending"
		rec.Error = err.Error()
		_, _ = store.PersistTransportRecord(rec)
		return err
	}

	rec.TransportStatus = "completed"
	rec.Error = ""
	_, _ = store.PersistTransportRecord(rec)

	fmt.Fprintf(ctx.Stdout, "Sync pull completed. CPI %s state exported and pushed to origin/%s. Deleted %d artifact(s), changed %d artifact(s). Transport record: %s\n", tenantDisplay(tenant), branch, len(deletedObjs), len(objs), filepath.ToSlash(strings.TrimPrefix(recPath, repoRoot+string(os.PathSeparator))))
	ctx.Logger.Info("sync pull completed", logging.F("branch", branch), logging.F("tenant", tenant), logging.F("deletedObjects", len(deletedObjs)), logging.F("changedObjects", len(objs)))
	return nil
}
