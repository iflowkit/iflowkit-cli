package sync

import (
	"fmt"
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/app"
)

func buildTransportCommitMessage(transportID, transportType, commitType, extra string) string {
	extra = strings.TrimSpace(extra)
	base := fmt.Sprintf("%s %s %s", transportID, transportType, commitType)
	if extra == "" {
		return base
	}
	return base + " " + extra
}

// gitHasChangesInPath reports whether there are working-tree or index changes for the given pathspec.
func gitHasChangesInPath(ctx *app.Context, dir, pathspec string) (bool, error) {
	out, err := runGitOutput(ctx, dir, "status", "--porcelain", "--", pathspec)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// gitCommitAndPushPath stages a path (force-add), commits it if changed, then pushes the current branch.
// This is used to ensure .iflowkit runtime records are actually persisted to the remote.
func gitCommitAndPushPath(ctx *app.Context, dir, branch, pathspec, message string) error {
	changed, err := gitHasChangesInPath(ctx, dir, pathspec)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	// Force-add in case the user's global gitignore ignores .iflowkit.
	if err := runGit(ctx, dir, "add", "-f", "--", pathspec); err != nil {
		return err
	}
	if err := runGit(ctx, dir, "commit", "-m", message); err != nil {
		if !strings.Contains(err.Error(), "nothing to commit") {
			return err
		}
	}

	upstream, _ := gitUpstreamRef(ctx, dir)
	if upstream == "" {
		return runGit(ctx, dir, "push", "-u", "origin", branch)
	}
	return runGit(ctx, dir, "push", "origin", branch)
}

// gitCommitAndPushLogs stages ALL changes, then unstages IntegrationPackage/, force-adds .iflowkit,
// commits the remaining staged changes (if any), and pushes the current branch.
//
// This is used for "logs" commits that must include .iflowkit transport records and any other
// non-IntegrationPackage files.
func gitCommitAndPushLogs(ctx *app.Context, dir, branch, message string) error {
	// Stage everything first.
	if err := runGit(ctx, dir, "add", "-A"); err != nil {
		return err
	}
	// Remove IntegrationPackage changes from the index so they don't leak into "logs".
	_ = runGit(ctx, dir, "reset", "HEAD", "--", "IntegrationPackage")

	// Force-add .iflowkit in case the user's global gitignore ignores it.
	_ = runGit(ctx, dir, "add", "-f", "--", ".iflowkit")

	out, err := runGitOutput(ctx, dir, "diff", "--cached", "--name-only")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) == "" {
		return nil
	}

	if err := runGit(ctx, dir, "commit", "-m", message); err != nil {
		if !strings.Contains(err.Error(), "nothing to commit") {
			return err
		}
	}

	upstream, _ := gitUpstreamRef(ctx, dir)
	if upstream == "" {
		return runGit(ctx, dir, "push", "-u", "origin", branch)
	}
	return runGit(ctx, dir, "push", "origin", branch)
}
