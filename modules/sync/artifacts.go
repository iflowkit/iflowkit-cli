package sync

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/common/cpix"
	"github.com/iflowkit/iflowkit-cli/internal/models"
)

var knownArtifactKinds = map[string]struct{}{
	"iFlows":          {},
	"ValueMappings":   {},
	"MessageMappings": {},
	"Scripts":         {},
	"CustomTags":      {},
}

// detectChangedArtifacts identifies artifacts affected by the given set of changed file paths.
//
// Expected layout:
//
//	<BaseFolder>/<Kind>/<ArtifactId>/<file>
func detectChangedArtifacts(meta models.SyncMetadata, changedPaths []string) map[artifactKey]struct{} {
	baseFolder := resolveContentFolder(meta)
	base := filepath.ToSlash(strings.Trim(baseFolder, "/")) + "/"

	keys := make(map[artifactKey]struct{})
	for _, p := range changedPaths {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p == "" || !strings.HasPrefix(p, base) {
			continue
		}
		rel := strings.TrimPrefix(p, base)
		parts := strings.SplitN(rel, "/", 3)
		if len(parts) < 3 {
			continue
		}
		kind := strings.TrimSpace(parts[0])
		id := strings.TrimSpace(parts[1])
		if kind == "" || id == "" {
			continue
		}
		if _, ok := knownArtifactKinds[kind]; !ok {
			continue
		}
		// ignore list files directly under the kind folder
		if strings.Contains(id, ".json") {
			continue
		}
		keys[artifactKey{Kind: kind, ID: id}] = struct{}{}
	}
	return keys
}

func keysToObjects(m map[artifactKey]struct{}) []SyncObject {
	out := make([]SyncObject, 0, len(m))
	for k := range m {
		if k.isZero() {
			continue
		}
		out = append(out, SyncObject{Kind: k.Kind, ID: k.ID})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].ID < out[j].ID
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func keysToObjectsFromSlice(list []artifactKey) []SyncObject {
	set := make(map[string]SyncObject, len(list))
	for _, k := range list {
		if k.isZero() {
			continue
		}
		set[k.Kind+"|"+k.ID] = SyncObject{Kind: k.Kind, ID: k.ID}
	}
	out := make([]SyncObject, 0, len(set))
	for _, v := range set {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].ID < out[j].ID
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func kindToEntitySet(kind string) string {
	switch kind {
	case "iFlows":
		return "IntegrationDesigntimeArtifacts"
	case "ValueMappings":
		return "ValueMappingDesigntimeArtifacts"
	case "MessageMappings":
		return "MessageMappingDesigntimeArtifacts"
	case "Scripts":
		return "ScriptCollectionDesigntimeArtifacts"
	default:
		return ""
	}
}

// listEndpointForKind returns the CPI list endpoint (relative path) for a given kind.
func listEndpointForKind(packageID string, kind string) (string, bool) {
	mainPath := fmt.Sprintf("/api/v1/IntegrationPackages('%s')", cpix.EscapeODataID(packageID))

	switch kind {
	case "iFlows":
		return mainPath + "/IntegrationDesigntimeArtifacts", true
	case "ValueMappings":
		return mainPath + "/ValueMappingDesigntimeArtifacts", true
	case "MessageMappings":
		return mainPath + "/MessageMappingDesigntimeArtifacts", true
	case "Scripts":
		return mainPath + "/ScriptCollectionDesigntimeArtifacts", true
	case "CustomTags":
		return mainPath + "/CustomTags", true
	default:
		return "", false
	}
}
