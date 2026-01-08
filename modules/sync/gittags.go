package sync

import (
	"fmt"
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/app"
	"github.com/iflowkit/iflowkit-cli/internal/logging"
)

// GitTagger creates and pushes git tags.
//
// We keep this logic isolated so it can be reused by other commands (e.g. pull/init) if needed.
type GitTagger struct {
	repoRoot string
	remote   string
}

func NewGitTagger(repoRoot string) *GitTagger {
	return &GitTagger{repoRoot: repoRoot, remote: "origin"}
}

// transportTagName builds a tag name that is unique per environment branch.
//
// Format: <transportId>_<branchName>
//
// branchName is normalized for tags:
//   - strips common ref prefixes (refs/heads/, refs/remotes/)
//   - strips remote prefix "origin/" if present
//   - replaces '/' and whitespace with '-'
func transportTagName(transportID string, branch string) string {
	transportID = strings.TrimSpace(transportID)
	b := strings.TrimSpace(branch)
	b = strings.TrimPrefix(b, "refs/heads/")
	b = strings.TrimPrefix(b, "refs/remotes/")
	if strings.HasPrefix(b, "origin/") {
		b = strings.TrimPrefix(b, "origin/")
	}
	b = strings.ReplaceAll(b, "/", "-")
	b = strings.ReplaceAll(b, " ", "-")
	b = strings.Trim(b, "-")
	if b == "" {
		b = "unknown"
	}
	return fmt.Sprintf("%s_%s", transportID, b)
}

// TagBranchWithTransportID creates a lightweight tag on the tip of the given branch
// and pushes it to the configured remote.
//
// Tag name format: <transportId>_<branchName>
//
// If the tag already exists (locally or remotely), this is treated as success.
func (t *GitTagger) TagBranchWithTransportID(ctx *app.Context, branch string, transportID string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch is required for tagging")
	}
	transportID = strings.TrimSpace(transportID)
	if transportID == "" {
		return fmt.Errorf("transportId is required for tagging")
	}

	tagName := transportTagName(transportID, branch)

	// Resolve branch tip commit.
	commit, err := runGitOutput(ctx, t.repoRoot, "rev-parse", branch)
	if err != nil {
		return fmt.Errorf("cannot resolve branch %q: %w", branch, err)
	}
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return fmt.Errorf("cannot resolve branch %q commit", branch)
	}

	// If tag already exists locally, we still attempt to push it (idempotent).
	if !t.localTagExists(ctx, tagName) {
		ctx.Logger.Info("creating git tag", logging.F("tag", tagName), logging.F("ref", branch))
		if err := runGit(ctx, t.repoRoot, "tag", tagName, commit); err != nil {
			// Race condition: another process may have created it between checks.
			if t.localTagExists(ctx, tagName) {
				ctx.Logger.Info("tag already exists after create attempt; continuing", logging.F("tag", tagName))
			} else {
				return err
			}
		}
	}

	// Push tag to remote.
	ctx.Logger.Info("pushing git tag", logging.F("tag", tagName), logging.F("remote", t.remote))
	if err := runGit(ctx, t.repoRoot, "push", t.remote, tagName); err != nil {
		// If remote already has this tag, treat as success.
		if t.remoteTagExists(ctx, tagName) {
			ctx.Logger.Info("tag already exists on remote; continuing", logging.F("tag", tagName), logging.F("remote", t.remote))
			return nil
		}
		return err
	}
	return nil
}

// TagDevWithTransportID is kept as a convenience wrapper.
func (t *GitTagger) TagDevWithTransportID(ctx *app.Context, transportID string) error {
	return t.TagBranchWithTransportID(ctx, "dev", transportID)
}

func (t *GitTagger) localTagExists(ctx *app.Context, tag string) bool {
	out, err := runGitOutput(ctx, t.repoRoot, "tag", "-l", tag)
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == tag
}

func (t *GitTagger) remoteTagExists(ctx *app.Context, tag string) bool {
	out, err := runGitOutput(ctx, t.repoRoot, "ls-remote", "--tags", t.remote, tag)
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != ""
}
