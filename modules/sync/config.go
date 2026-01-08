package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/models"
)

func findSyncRepoRoot(start string) (string, error) {
	p := start
	for {
		if p == "" {
			break
		}
		if st, err := os.Stat(filepath.Join(p, ".iflowkit")); err == nil && st.IsDir() {
			return p, nil
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	return "", fmt.Errorf("not inside a sync repository: .iflowkit directory not found")
}

func loadPackageMetadata(repoRoot string) (models.SyncMetadata, error) {
	pkgPath := filepath.Join(repoRoot, ".iflowkit", "package.json")
	b, err := os.ReadFile(pkgPath)
	if err != nil {
		return models.SyncMetadata{}, fmt.Errorf("missing metadata file: expected %s", pkgPath)
	}
	var meta models.SyncMetadata
	if err := json.Unmarshal(b, &meta); err != nil {
		return models.SyncMetadata{}, fmt.Errorf("invalid package.json: %w", err)
	}
	return meta, nil
}

func resolveContentFolder(meta models.SyncMetadata) string {
	p := strings.TrimSpace(meta.BaseFolder)
	if p == "" {
		return "IntegrationPackage"
	}
	return strings.Trim(p, "/")
}
