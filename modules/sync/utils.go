package sync

import (
	"github.com/iflowkit/iflowkit-cli/internal/app"
	"github.com/iflowkit/iflowkit-cli/internal/common/gitx"
)

// runGit executes a git command in dir and logs it through ctx.
func runGit(ctx *app.Context, dir string, args ...string) error {
	return gitx.Run(ctx.Logger, dir, args...)
}

// runGitOutput executes a git command in dir and returns trimmed combined output.
func runGitOutput(ctx *app.Context, dir string, args ...string) (string, error) {
	return gitx.Output(ctx.Logger, dir, args...)
}
