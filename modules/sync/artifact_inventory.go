package sync

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/models"
)

// listLocalArtifactKeys returns all locally present artifacts under IntegrationPackage/<Kind>/<Id>/.
// Only directory names are considered artifacts; list JSON files are ignored.
func listLocalArtifactKeys(repoRoot string, meta models.SyncMetadata) (map[artifactKey]struct{}, error) {
	baseAbs := filepath.Join(repoRoot, resolveContentFolder(meta))

	knownKinds := []string{"iFlows", "ValueMappings", "MessageMappings", "Scripts", "CustomTags"}
	keys := make(map[artifactKey]struct{})

	for _, kind := range knownKinds {
		kindDir := filepath.Join(baseAbs, kind)
		entries, err := os.ReadDir(kindDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			id := strings.TrimSpace(e.Name())
			if id == "" {
				continue
			}
			// ignore hidden folders
			if strings.HasPrefix(id, ".") {
				continue
			}
			keys[artifactKey{Kind: kind, ID: id}] = struct{}{}
		}
	}

	return keys, nil
}

// setDiff returns all keys in a that are not present in b.
func setDiff(a, b map[artifactKey]struct{}) map[artifactKey]struct{} {
	out := make(map[artifactKey]struct{})
	for k := range a {
		if _, ok := b[k]; ok {
			continue
		}
		out[k] = struct{}{}
	}
	return out
}

// partitionChangedKeys splits changed keys into (toUpload, toDelete) by checking local folder existence.
// Only known CPI artifact kinds (iFlows/Scripts/ValueMappings/MessageMappings) are eligible for deletion.
func partitionChangedKeys(repoRoot string, meta models.SyncMetadata, changed map[artifactKey]struct{}) (map[artifactKey]struct{}, map[artifactKey]struct{}) {
	baseFolder := resolveContentFolder(meta)
	toUpload := make(map[artifactKey]struct{})
	toDelete := make(map[artifactKey]struct{})

	deletableKind := func(kind string) bool {
		switch kind {
		case "iFlows", "Scripts", "ValueMappings", "MessageMappings":
			return true
		default:
			return false
		}
	}

	for k := range changed {
		artifactDir := filepath.Join(repoRoot, baseFolder, k.Kind, k.ID)
		st, err := os.Stat(artifactDir)
		if err == nil && st.IsDir() {
			toUpload[k] = struct{}{}
			continue
		}
		// Missing dir: treat as deletion only if kind is deletable.
		if deletableKind(k.Kind) {
			toDelete[k] = struct{}{}
		}
	}
	return toUpload, toDelete
}

// mapKeysToSortedSlice returns a stable sorted slice of keys.
func mapKeysToSortedSlice(m map[artifactKey]struct{}) []artifactKey {
	out := make([]artifactKey, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].ID < out[j].ID
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}
