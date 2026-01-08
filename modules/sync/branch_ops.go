package sync

import (
	"fmt"
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/app"
)

func gitLocalBranchExists(ctx *app.Context, dir, branch string) bool {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return false
	}
	ref := fmt.Sprintf("refs/heads/%s", branch)
	_, err := runGitOutput(ctx, dir, "show-ref", "--verify", "--quiet", ref)
	return err == nil
}

// checkoutBranch ensures the branch exists locally and checks it out.
// If local branch is missing but remote exists, it creates the branch from origin/<branch>.
func checkoutBranch(ctx *app.Context, dir, branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch is required")
	}
	if gitLocalBranchExists(ctx, dir, branch) {
		return runGit(ctx, dir, "checkout", branch)
	}
	if gitRemoteBranchExists(ctx, dir, branch) {
		return runGit(ctx, dir, "checkout", "-b", branch, "origin/"+branch)
	}
	return runGit(ctx, dir, "checkout", "-b", branch)
}

// fastForwardFromRemote updates the local branch to match origin/<branch> if possible.
// If local branch diverged from origin/<branch>, it returns an error.
func fastForwardFromRemote(ctx *app.Context, dir, branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch is required")
	}
	if !gitRemoteBranchExists(ctx, dir, branch) {
		return nil
	}
	remoteRef := "origin/" + branch
	behind, ahead := gitAheadBehind(ctx, dir, remoteRef, "HEAD")
	if behind > 0 && ahead > 0 {
		return fmt.Errorf("local branch diverged from %s (ahead=%d, behind=%d)", remoteRef, ahead, behind)
	}
	if behind > 0 {
		return runGit(ctx, dir, "merge", "--ff-only", remoteRef)
	}
	return nil
}

// ensureBranchFetchedAndCheckedOut fetches the branch from origin (best-effort),
// checks it out, and fast-forwards it to origin/<branch> when possible.
func ensureBranchFetchedAndCheckedOut(ctx *app.Context, dir, branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch is required")
	}
	_ = runGit(ctx, dir, "fetch", "origin", branch)
	if err := checkoutBranch(ctx, dir, branch); err != nil {
		return err
	}
	return fastForwardFromRemote(ctx, dir, branch)
}
