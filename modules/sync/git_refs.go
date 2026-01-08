package sync

import (
	"fmt"

	"github.com/iflowkit/iflowkit-cli/internal/app"
)

func gitRemoteBranchExists(ctx *app.Context, dir, branch string) bool {
	ref := fmt.Sprintf("refs/remotes/origin/%s", branch)
	_, err := runGitOutput(ctx, dir, "show-ref", "--verify", "--quiet", ref)
	return err == nil
}
