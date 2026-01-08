package sync

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/common/filex"
)

const repoIgnoreFileName = "ignore"

// defaultRepoIgnorePatterns are applied even when .iflowkit/ignore is missing.
//
// These patterns cover files that are known to change across CPI export operations
// without functional impact (timestamps, metadata, etc.).
var defaultRepoIgnorePatterns = []string{
	"IntegrationPackage/**/metainfo.prop",
	"IntegrationPackage/**/src/main/resources/parameters.prop",
}

// RepoIgnore matches repo-relative paths (slash-separated) against ignore patterns
// defined in .iflowkit/ignore (plus built-in defaults).
//
// Supported pattern syntax:
//   - '*'  matches any characters except '/'
//   - '?'  matches a single character except '/'
//   - '**' matches across path segments
//
// Lines starting with '#' are comments.
// Empty lines are ignored.
//
// If a pattern does not contain '/', it is treated as '**/<pattern>' (match anywhere).
type RepoIgnore struct {
	patterns []ignorePattern
}

type ignorePattern struct {
	raw   string
	regex *regexp.Regexp
}

func repoIgnorePath(repoRoot string) string {
	return filepath.Join(repoRoot, ".iflowkit", repoIgnoreFileName)
}

// EnsureRepoIgnoreFile creates .iflowkit/ignore with a default template if missing.
func EnsureRepoIgnoreFile(repoRoot string) error {
	if err := filex.EnsureDir(filepath.Join(repoRoot, ".iflowkit")); err != nil {
		return err
	}
	p := repoIgnorePath(repoRoot)
	if _, err := os.Stat(p); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	lines := []string{
		"# iflowkit sync ignore patterns (repo-relative paths)",
		"#",
		"# Default volatile files (safe to ignore):",
	}
	lines = append(lines, defaultRepoIgnorePatterns...)
	lines = append(lines, "")
	return filex.AtomicWriteFile(p, []byte(strings.Join(lines, "\n")), 0o644)
}

// LoadRepoIgnore reads ignore patterns from .iflowkit/ignore and merges them with
// built-in defaults.
func LoadRepoIgnore(repoRoot string) (*RepoIgnore, error) {
	ri := &RepoIgnore{patterns: []ignorePattern{}}

	seen := map[string]struct{}{}
	add := func(raw string, origin string, lineNo int) error {
		norm := filepath.ToSlash(strings.TrimSpace(raw))
		norm = strings.TrimPrefix(norm, "./")
		if norm == "" {
			return nil
		}
		if _, ok := seen[norm]; ok {
			return nil
		}
		compiled, err := compileIgnorePattern(norm)
		if err != nil {
			if origin == "" {
				return err
			}
			return fmt.Errorf("invalid ignore pattern at %s:%d: %w", origin, lineNo, err)
		}
		seen[norm] = struct{}{}
		ri.patterns = append(ri.patterns, compiled)
		return nil
	}

	// Defaults first.
	for _, p := range defaultRepoIgnorePatterns {
		if err := add(p, "", 0); err != nil {
			return nil, err
		}
	}

	// File patterns.
	p := repoIgnorePath(repoRoot)
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return ri, nil
		}
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := add(line, p, lineNo); err != nil {
			return nil, err
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return ri, nil
}

func compileIgnorePattern(raw string) (ignorePattern, error) {
	pat := strings.TrimSpace(filepath.ToSlash(raw))
	if pat == "" {
		return ignorePattern{}, fmt.Errorf("empty pattern")
	}
	if !strings.Contains(pat, "/") {
		pat = "**/" + pat
	}
	pat = strings.TrimPrefix(pat, "./")

	rx, err := globToRegex(pat)
	if err != nil {
		return ignorePattern{}, err
	}
	re, err := regexp.Compile(rx)
	if err != nil {
		return ignorePattern{}, err
	}
	return ignorePattern{raw: raw, regex: re}, nil
}

// globToRegex converts a glob supporting '**' into a full-string regex.
// Rules:
//   - '**' => '.*'
//   - '*'  => '[^/]*'
//   - '?'  => '[^/]'
func globToRegex(glob string) (string, error) {
	glob = filepath.ToSlash(glob)

	// path.Clean can break patterns containing '**', so only clean when absent.
	if !strings.Contains(glob, "**") {
		glob = path.Clean(glob)
		glob = filepath.ToSlash(glob)
	}

	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(glob); i++ {
		ch := glob[i]
		if ch == '*' {
			if i+1 < len(glob) && glob[i+1] == '*' {
				b.WriteString(".*")
				i++
				continue
			}
			b.WriteString("[^/]*")
			continue
		}
		if ch == '?' {
			b.WriteString("[^/]")
			continue
		}
		switch ch {
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			b.WriteByte('\\')
		}
		b.WriteByte(ch)
	}
	b.WriteString("$")
	return b.String(), nil
}

func (ri *RepoIgnore) IsIgnored(p string) bool {
	if ri == nil || len(ri.patterns) == 0 {
		return false
	}
	norm := filepath.ToSlash(strings.TrimSpace(p))
	norm = strings.TrimPrefix(norm, "./")
	if norm == "" {
		return false
	}
	for _, pat := range ri.patterns {
		if pat.regex != nil && pat.regex.MatchString(norm) {
			return true
		}
	}
	return false
}

// Filter removes ignored paths and de-duplicates the remaining paths.
func (ri *RepoIgnore) Filter(paths []string) []string {
	if ri == nil || len(ri.patterns) == 0 {
		return dedupeStable(paths)
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		if ri.IsIgnored(p) {
			continue
		}
		out = append(out, p)
	}
	return dedupeStable(out)
}

func dedupeStable(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, p := range in {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}
