package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CompareFolderTrees compares the file contents of two folders.
//
// The returned paths are repo-relative (slash-separated) and prefixed with repoRelPrefix.
//
// Ignore rules (when provided) are applied on the returned repo-relative paths.
func CompareFolderTrees(repoRelPrefix string, absA string, absB string, ign *RepoIgnore) ([]string, error) {
	repoRelPrefix = strings.Trim(strings.TrimSpace(filepath.ToSlash(repoRelPrefix)), "/")
	if repoRelPrefix == "" {
		return nil, fmt.Errorf("repoRelPrefix is required")
	}
	if absA == "" || absB == "" {
		return nil, fmt.Errorf("compare folders: both paths are required")
	}

	hA, err := hashDir(repoRelPrefix, absA, ign)
	if err != nil {
		return nil, err
	}
	hB, err := hashDir(repoRelPrefix, absB, ign)
	if err != nil {
		return nil, err
	}

	diff := make(map[string]struct{})
	for p, a := range hA {
		b, ok := hB[p]
		if !ok || b != a {
			diff[p] = struct{}{}
		}
	}
	for p, b := range hB {
		a, ok := hA[p]
		if !ok || a != b {
			diff[p] = struct{}{}
		}
	}

	out := make([]string, 0, len(diff))
	for p := range diff {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

func hashDir(repoRelPrefix string, absRoot string, ign *RepoIgnore) (map[string]string, error) {
	info, err := os.Stat(absRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", absRoot)
	}

	hashes := make(map[string]string, 256)
	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		repoRel := repoRelPrefix + "/" + rel
		repoRel = strings.TrimPrefix(repoRel, "./")
		if ign != nil && ign.IsIgnored(repoRel) {
			return nil
		}
		h, err := sha256File(path)
		if err != nil {
			return err
		}
		hashes[repoRel] = h
		return nil
	})
	if err != nil {
		return nil, err
	}
	return hashes, nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
