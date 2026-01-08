package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iflowkit/iflowkit-cli/internal/app"
	"github.com/iflowkit/iflowkit-cli/internal/common/cpix"
	"github.com/iflowkit/iflowkit-cli/internal/common/filex"
	"github.com/iflowkit/iflowkit-cli/internal/logging"
	"github.com/iflowkit/iflowkit-cli/internal/models"
	"github.com/iflowkit/iflowkit-cli/internal/validate"
)

// ensureEnvBranchOnRemote guarantees that origin/<env> exists.
//
// If missing, it bootstraps the branch by exporting the tenant state
// and creating an init transport record and tag.
func ensureEnvBranchOnRemote(ctx *app.Context, repoRoot string, meta models.SyncMetadata, env string) error {
	env = strings.ToLower(strings.TrimSpace(env))
	if err := validate.Env(env); err != nil {
		return err
	}
	_ = runGit(ctx, repoRoot, "fetch", "origin")
	if gitRemoteBranchExists(ctx, repoRoot, env) {
		// Ensure local branch exists and is up to date.
		return ensureBranchFetchedAndCheckedOut(ctx, repoRoot, env)
	}
	ctx.Logger.Info("bootstrapping missing environment branch from tenant", logging.F("env", env))
	_, err := bootstrapEnvBranchFromTenant(ctx, repoRoot, meta, env)
	return err
}

// bootstrapEnvBranchFromTenant creates or overwrites a local <env> branch
// by exporting the tenant state and pushing it to origin/<env>.
//
// Returns the created transportId.
func bootstrapEnvBranchFromTenant(ctx *app.Context, repoRoot string, meta models.SyncMetadata, env string) (string, error) {
	env = strings.ToLower(strings.TrimSpace(env))
	if err := validate.Env(env); err != nil {
		return "", err
	}

	// Resolve tenant.
	profileID, source, err := ctx.Stores.ResolveProfileID(ctx.Flags.ProfileID)
	if err != nil {
		return "", err
	}
	_, err = ctx.Stores.Profiles.Read(profileID)
	if err != nil {
		return "", err
	}
	ctx.Logger.Info("resolved profile", logging.F("profile", profileID), logging.F("source", source))

	tenantKey, err := ctx.Stores.Tenants.Read(profileID, env)
	if err != nil {
		return "", fmt.Errorf("%s tenant not found for profile %q; import it with `iflowkit tenant import --env %s --file <service-key.json>`: %w", tenantDisplay(env), profileID, env, err)
	}

	// Start from dev branch when possible (shared repo history).
	if err := ensureBranchFetchedAndCheckedOut(ctx, repoRoot, "dev"); err != nil {
		return "", err
	}
	if err := runGit(ctx, repoRoot, "checkout", "-B", env, "dev"); err != nil {
		return "", err
	}

	// Export tenant state.
	c := cpix.NewClient(tenantKey, ctx.Logger)
	_, raw, err := c.ReadIntegrationPackage(meta.PackageID)
	if err != nil {
		return "", err
	}
	baseFolder := resolveContentFolder(meta)
	baseAbs := filepath.Join(repoRoot, baseFolder)
	if err := os.RemoveAll(baseAbs); err != nil {
		return "", err
	}
	if err := filex.EnsureDir(baseAbs); err != nil {
		return "", err
	}
	if err := c.ExportIntegrationPackageFromRaw(meta.PackageID, raw, baseAbs); err != nil {
		return "", err
	}

	transportID, createdAt := newTransportIDs(time.Now())
	if err := writeInitTransport(ctx, repoRoot, meta.BaseFolder, meta.PackageID, env, env, transportID, createdAt); err != nil {
		return "", err
	}

	// Commit and push.
	contentMsg := buildTransportCommitMessage(transportID, "init", "contents", "bootstrap")
	if err := runGit(ctx, repoRoot, "add", "-A", "--", baseFolder); err != nil {
		return "", err
	}
	if err := runGit(ctx, repoRoot, "commit", "-m", contentMsg, "--", baseFolder); err != nil {
		if !strings.Contains(err.Error(), "nothing to commit") {
			return "", err
		}
	}
	// Push the branch (contents commit) first.
	if err := runGit(ctx, repoRoot, "push", "-u", "origin", env); err != nil {
		return "", err
	}
	// Commit/push .iflowkit metadata.
	logsMsg := buildTransportCommitMessage(transportID, "init", "logs", "bootstrap")
	if err := gitCommitAndPushLogs(ctx, repoRoot, env, logsMsg); err != nil {
		return "", err
	}

	// Tag.
	tagger := NewGitTagger(repoRoot)
	if err := tagger.TagBranchWithTransportID(ctx, env, transportID); err != nil {
		return "", err
	}

	ctx.Logger.Info("environment branch bootstrapped", logging.F("env", env), logging.F("branch", env), logging.F("transportId", transportID))
	return transportID, nil
}
