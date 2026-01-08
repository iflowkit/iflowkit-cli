package sync

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/app"
)

func gitCurrentBranch(ctx *app.Context, dir string) (string, error) {
	out, err := runGitOutput(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", fmt.Errorf("unable to determine current branch")
	}
	return out, nil
}

func gitUpstreamRef(ctx *app.Context, dir string) (string, error) {
	out, err := runGitOutput(ctx, dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// gitPendingChanges returns:
// - changedPaths: files changed between baseRef and HEAD
// - commits: commit hashes between baseRef and HEAD (oldest->newest)
func gitPendingChanges(ctx *app.Context, dir, baseRef, branch string) ([]string, []string, error) {
	var changedPaths []string
	var commits []string

	if baseRef != "" {
		out, err := runGitOutput(ctx, dir, "diff", "--name-only", baseRef+"..HEAD")
		if err != nil {
			return nil, nil, err
		}
		changedPaths = splitLines(out)

		cout, err := runGitOutput(ctx, dir, "rev-list", "--reverse", baseRef+"..HEAD")
		if err != nil {
			return nil, nil, err
		}
		commits = splitLines(cout)
	} else {
		// No upstream: best-effort. Capture HEAD commit as the "pushed" commit.
		cout, err := runGitOutput(ctx, dir, "rev-list", "--max-count=1", "HEAD")
		if err == nil {
			commits = splitLines(cout)
		}
	}

	// If baseRef was empty, try to compute changes against origin/dev (feature branches created locally).
	if len(changedPaths) == 0 && baseRef == "" && (strings.HasPrefix(branch, "feature/") || strings.HasPrefix(branch, "bugfix/")) {
		// origin/dev may not exist, ignore errors.
		if out, err := runGitOutput(ctx, dir, "diff", "--name-only", "origin/dev..HEAD"); err == nil {
			changedPaths = splitLines(out)
		}
	}

	return changedPaths, commits, nil
}

// gitAheadBehind computes how many commits rightRef is ahead of leftRef and vice versa.
func gitAheadBehind(ctx *app.Context, dir, leftRef, rightRef string) (behind int, ahead int) {
	// leftRef...rightRef: behind=commits only in leftRef, ahead=commits only in rightRef.
	out, err := runGitOutput(ctx, dir, "rev-list", "--left-right", "--count", leftRef+"..."+rightRef)
	if err != nil {
		return 0, 0
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0
	}
	b, _ := strconv.Atoi(fields[0])
	a, _ := strconv.Atoi(fields[1])
	return b, a
}

// gitPorcelainPaths parses `git status --porcelain` and returns a stable list of file paths.
func gitPorcelainPaths(ctx *app.Context, dir string) []string {
	out, err := runGitOutput(ctx, dir, "status", "--porcelain")
	if err != nil {
		return nil
	}
	paths := []string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format examples:
		//  M file
		// ?? file
		// R  old -> new
		if strings.Contains(line, "->") {
			parts := strings.Split(line, "->")
			p := strings.TrimSpace(parts[len(parts)-1])
			if p != "" {
				paths = append(paths, p)
			}
			continue
		}
		if len(line) > 3 {
			p := strings.TrimSpace(line[2:])
			if p != "" {
				paths = append(paths, p)
			}
		}
	}
	// Unique + stable.
	set := make(map[string]struct{}, len(paths))
	out2 := make([]string, 0, len(paths))
	for _, p := range paths {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if _, ok := set[p]; ok {
			continue
		}
		set[p] = struct{}{}
		out2 = append(out2, p)
	}
	sort.Strings(out2)
	return out2
}
