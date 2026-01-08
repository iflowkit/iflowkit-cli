package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/app"
)

// writeInitTransport creates an init transport record under .iflowkit/transports/<tenant>/
// and appends it to .iflowkit/transports/<tenant>/index.json.
//
// The init transport is always marked as completed and does not affect push/pull retry state.
// Unlike push/pull records, init includes ALL objects currently present in the exported repo.
func writeInitTransport(ctx *app.Context, repoRoot string, baseFolder string, packageID string, tenant string, branch string, transportID string, createdAt string) error {
	_ = ctx // reserved for future logging

	objs, err := collectAllObjectsFromExport(repoRoot, baseFolder)
	if err != nil {
		return err
	}

	rec := TransportRecord{
		SchemaVersion:   1,
		TransportID:     transportID,
		TransportType:   "init",
		PackageID:       packageID,
		Branch:          branch,
		CreatedAt:       createdAt,
		GitCommits:      []string{},
		Objects:         objs,
		TransportStatus: "completed",
	}

	store, err := NewTransportStore(repoRoot, tenant)
	if err != nil {
		return err
	}
	_, err = store.PersistTransportRecord(rec)
	return err
}

// collectAllObjectsFromExport builds a full object list for init by scanning the exported
// IntegrationPackage/ folder and (when present) its CPI list JSON files.
//
// This is intentionally filesystem-based so init does not need extra CPI round-trips.
func collectAllObjectsFromExport(repoRoot string, baseFolder string) ([]SyncObject, error) {
	baseFolder = strings.TrimSpace(baseFolder)
	if baseFolder == "" {
		baseFolder = "IntegrationPackage"
	}
	baseAbs := filepath.Join(repoRoot, filepath.FromSlash(baseFolder))

	kinds := []string{"iFlows", "ValueMappings", "MessageMappings", "Scripts", "CustomTags"}
	set := make(map[string]SyncObject, 256)

	add := func(kind, id string) {
		kind = strings.TrimSpace(kind)
		id = strings.TrimSpace(id)
		if kind == "" || id == "" {
			return
		}
		set[kind+"|"+id] = SyncObject{Kind: kind, ID: id}
	}

	// Best-effort parse of CPI list JSON (OData) to capture objects that are not exported as folders
	// (e.g., CustomTags may not have a media zip to download).
	parseListIDs := func(path string, kind string) {
		b, err := os.ReadFile(path)
		if err != nil {
			return
		}
		var lr struct {
			D struct {
				Results []struct {
					ID string `json:"Id"`
				} `json:"results"`
			} `json:"d"`
		}
		if err := json.Unmarshal(b, &lr); err != nil {
			return
		}
		for _, it := range lr.D.Results {
			add(kind, it.ID)
		}
	}

	for _, kind := range kinds {
		kindDir := filepath.Join(baseAbs, kind)
		ents, err := os.ReadDir(kindDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range ents {
			name := strings.TrimSpace(e.Name())
			if name == "" || strings.HasPrefix(name, ".") {
				continue
			}
			if e.IsDir() {
				add(kind, name)
				continue
			}
			// Parse list files located directly under the kind folder.
			if strings.HasSuffix(strings.ToLower(name), ".json") {
				parseListIDs(filepath.Join(kindDir, name), kind)
			}
		}
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
	return out, nil
}
