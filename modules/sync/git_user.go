package sync

import (
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/app"
)

func gitUserIdentity(ctx *app.Context, dir string) (string, string, error) {
	name, _ := runGitOutput(ctx, dir, "config", "--get", "user.name")
	email, _ := runGitOutput(ctx, dir, "config", "--get", "user.email")
	if strings.TrimSpace(name) == "" {
		name, _ = runGitOutput(ctx, dir, "log", "-1", "--pretty=format:%an")
	}
	if strings.TrimSpace(email) == "" {
		email, _ = runGitOutput(ctx, dir, "log", "-1", "--pretty=format:%ae")
	}
	return strings.TrimSpace(name), strings.TrimSpace(email), nil
}
