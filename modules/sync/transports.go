package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/iflowkit/iflowkit-cli/internal/common/filex"
	"github.com/iflowkit/iflowkit-cli/internal/validate"
)

const (
	transportRecordExt = ".transport.json"
	transportIndexFile = "index.json"
)

// TransportRecord is stored at:
//
//	.iflowkit/transports/<tenant>/<transportId>.transport.json
type TransportRecord struct {
	SchemaVersion int    `json:"schemaVersion"`
	TransportID   string `json:"transportId"`
	TransportType string `json:"transportType"` // init | pull | push | deliver
	PackageID     string `json:"packageId"`
	Branch        string `json:"branch"`
	CreatedAt     string `json:"createdAt"`

	GitCommits []string `json:"gitCommits"`

	GitUserName  string `json:"gitUserName,omitempty"`
	GitUserEmail string `json:"gitUserEmail,omitempty"`

	Objects []SyncObject `json:"objects"`

	// DeletedObjects lists artifacts that were deleted during this transport.
	// For pull: objects deleted in CPI and removed from the repo.
	// For push: objects deleted in the repo and removed from CPI.
	DeletedObjects []SyncObject `json:"deletedObjects,omitempty"`

	TransportStatus string `json:"transportStatus"` // pending | completed
	Error           string `json:"error,omitempty"`

	UploadRemaining []artifactKey  `json:"uploadRemaining"`
	DeleteRemaining []artifactKey  `json:"deleteRemaining,omitempty"`
	DeployRemaining []deployTarget `json:"deployRemaining"`
}

// TransportIndex is stored at:
//
//	.iflowkit/transports/<tenant>/index.json
//
// It keeps a flat list of transport items for quick lookup.
type TransportIndex struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Items         []TransportIndexItem `json:"items"`
}

type TransportIndexItem struct {
	Seq             int    `json:"seq"`
	TransportID     string `json:"transportId"`
	TransportType   string `json:"transportType"`
	TransportStatus string `json:"transportStatus"`
	CreatedAt       string `json:"createdAt"`
}

// TransportStore manages per-tenant transport records.
type TransportStore struct {
	repoRoot string
	tenant   string
}

func NewTransportStore(repoRoot, tenant string) (*TransportStore, error) {
	tenant = normalizeTenant(tenant)
	if err := validate.Env(tenant); err != nil {
		return nil, err
	}
	return &TransportStore{repoRoot: repoRoot, tenant: tenant}, nil
}

func normalizeTenant(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func (s *TransportStore) tenantDir() string {
	return filepath.Join(s.repoRoot, ".iflowkit", "transports", s.tenant)
}

func (s *TransportStore) ensureDir() error {
	return filex.EnsureDir(s.tenantDir())
}

func (s *TransportStore) recordPath(transportID string) string {
	return filepath.Join(s.tenantDir(), sanitizeTransportID(transportID)+transportRecordExt)
}

func (s *TransportStore) indexPath() string {
	return filepath.Join(s.tenantDir(), transportIndexFile)
}

// newTransportIDs returns:
// - transportId: YYYYMMDDTHHMMSSmmmZ (UTC)
// - createdAt : RFC3339, seconds precision, UTC
func newTransportIDs(t time.Time) (transportID string, createdAt string) {
	utc := t.UTC()
	createdAt = utc.Truncate(time.Second).Format(time.RFC3339)
	ms := utc.Nanosecond() / int(time.Millisecond)
	transportID = utc.Format("20060102T150405") + fmt.Sprintf("%03dZ", ms)
	return transportID, createdAt
}

func sanitizeTransportID(s string) string {
	s = strings.TrimSpace(s)
	repl := strings.NewReplacer(
		"-", "",
		":", "",
		".", "",
		"+", "",
		"/", "",
		"\\", "",
		" ", "",
		"\t", "",
		"\n", "",
		"\r", "",
	)
	return repl.Replace(s)
}

func normalizeTransportType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	switch t {
	case "init", "pull", "push", "deliver":
		return t
	default:
		return "push"
	}
}

func normalizeTransportStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "completed" {
		return "completed"
	}
	return "pending"
}

// PersistTransportRecord saves the record file and updates index.json.
func (s *TransportStore) PersistTransportRecord(rec TransportRecord) (string, error) {
	if err := s.ensureDir(); err != nil {
		return "", err
	}
	rec.TransportID = strings.TrimSpace(rec.TransportID)
	if rec.TransportID == "" {
		return "", fmt.Errorf("transportId is required")
	}
	if rec.SchemaVersion == 0 {
		rec.SchemaVersion = 1
	}
	rec.TransportType = normalizeTransportType(rec.TransportType)
	rec.TransportStatus = normalizeTransportStatus(rec.TransportStatus)
	if rec.Objects == nil {
		rec.Objects = []SyncObject{}
	}
	if rec.DeletedObjects == nil {
		rec.DeletedObjects = []SyncObject{}
	}
	if rec.UploadRemaining == nil {
		rec.UploadRemaining = []artifactKey{}
	}
	if rec.DeleteRemaining == nil {
		rec.DeleteRemaining = []artifactKey{}
	}
	if rec.DeployRemaining == nil {
		rec.DeployRemaining = []deployTarget{}
	}

	path := s.recordPath(rec.TransportID)
	if err := saveTransportRecord(path, rec); err != nil {
		return "", err
	}
	if err := s.upsertIndex(rec); err != nil {
		return "", err
	}
	return path, nil
}

func saveTransportRecord(path string, r TransportRecord) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return filex.AtomicWriteFile(path, b, 0o644)
}

func (s *TransportStore) LoadRecord(transportID string) (TransportRecord, error) {
	b, err := os.ReadFile(s.recordPath(transportID))
	if err != nil {
		return TransportRecord{}, err
	}
	var r TransportRecord
	if err := json.Unmarshal(b, &r); err != nil {
		return TransportRecord{}, fmt.Errorf("invalid transport record: %w", err)
	}
	r.TransportType = normalizeTransportType(r.TransportType)
	r.TransportStatus = normalizeTransportStatus(r.TransportStatus)
	if r.Objects == nil {
		r.Objects = []SyncObject{}
	}
	if r.DeletedObjects == nil {
		r.DeletedObjects = []SyncObject{}
	}
	if r.UploadRemaining == nil {
		r.UploadRemaining = []artifactKey{}
	}
	if r.DeleteRemaining == nil {
		r.DeleteRemaining = []artifactKey{}
	}
	if r.DeployRemaining == nil {
		r.DeployRemaining = []deployTarget{}
	}
	return r, nil
}

func (s *TransportStore) loadIndex() (*TransportIndex, error) {
	b, err := os.ReadFile(s.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var idx TransportIndex
	if err := json.Unmarshal(b, &idx); err != nil {
		return nil, fmt.Errorf("invalid transport index: %w", err)
	}
	if idx.SchemaVersion == 0 {
		idx.SchemaVersion = 1
	}
	if idx.Items == nil {
		idx.Items = []TransportIndexItem{}
	}
	return &idx, nil
}

func (s *TransportStore) saveIndex(idx TransportIndex) error {
	if idx.SchemaVersion == 0 {
		idx.SchemaVersion = 1
	}
	if idx.Items == nil {
		idx.Items = []TransportIndexItem{}
	}
	b, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return filex.AtomicWriteFile(s.indexPath(), b, 0o644)
}

func (s *TransportStore) upsertIndex(rec TransportRecord) error {
	idx, err := s.loadIndex()
	if err != nil {
		return err
	}
	if idx == nil {
		idx = &TransportIndex{SchemaVersion: 1, Items: []TransportIndexItem{}}
	}

	for i := range idx.Items {
		if idx.Items[i].TransportID == rec.TransportID {
			idx.Items[i].TransportType = rec.TransportType
			idx.Items[i].TransportStatus = normalizeTransportStatus(rec.TransportStatus)
			idx.Items[i].CreatedAt = rec.CreatedAt
			return s.saveIndex(*idx)
		}
	}

	maxSeq := 0
	for _, it := range idx.Items {
		if it.Seq > maxSeq {
			maxSeq = it.Seq
		}
	}

	idx.Items = append(idx.Items, TransportIndexItem{
		Seq:             maxSeq + 1,
		TransportID:     rec.TransportID,
		TransportType:   rec.TransportType,
		TransportStatus: normalizeTransportStatus(rec.TransportStatus),
		CreatedAt:       rec.CreatedAt,
	})
	return s.saveIndex(*idx)
}

func parseCreatedAtOrZero(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

func (s *TransportStore) listRecordPaths() ([]string, error) {
	dir := s.tenantDir()
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	paths := make([]string, 0, len(ents))
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, transportRecordExt) {
			continue
		}
		paths = append(paths, filepath.Join(dir, name))
	}
	sort.Strings(paths)
	return paths, nil
}

// LoadLatestTransportRecord returns the most recent transport record for this tenant.
func (s *TransportStore) LoadLatestTransportRecord() (*TransportRecord, string, bool, error) {
	idx, err := s.loadIndex()
	if err != nil {
		return nil, "", false, err
	}
	if idx != nil && len(idx.Items) > 0 {
		last := idx.Items[len(idx.Items)-1]
		r, err := s.LoadRecord(last.TransportID)
		if err == nil {
			p := s.recordPath(last.TransportID)
			cp := r
			return &cp, p, true, nil
		}
	}

	paths, err := s.listRecordPaths()
	if err != nil {
		return nil, "", false, err
	}
	if len(paths) == 0 {
		return nil, "", false, nil
	}

	var best *TransportRecord
	bestPath := ""
	bestTime := time.Time{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var rec TransportRecord
		if err := json.Unmarshal(b, &rec); err != nil {
			continue
		}
		tm := parseCreatedAtOrZero(rec.CreatedAt)
		if best == nil || tm.After(bestTime) {
			cp := rec
			best = &cp
			bestPath = p
			bestTime = tm
		}
	}
	if best == nil {
		return nil, "", false, nil
	}
	best.TransportType = normalizeTransportType(best.TransportType)
	best.TransportStatus = normalizeTransportStatus(best.TransportStatus)
	if best.Objects == nil {
		best.Objects = []SyncObject{}
	}
	if best.DeletedObjects == nil {
		best.DeletedObjects = []SyncObject{}
	}
	if best.UploadRemaining == nil {
		best.UploadRemaining = []artifactKey{}
	}
	if best.DeleteRemaining == nil {
		best.DeleteRemaining = []artifactKey{}
	}
	if best.DeployRemaining == nil {
		best.DeployRemaining = []deployTarget{}
	}
	return best, bestPath, true, nil
}

// LoadLatestPendingTransport finds the most recent transport record that is not completed.
// If transportType is provided, only that transportType (init|pull|push) is considered.
func (s *TransportStore) LoadLatestPendingTransport(packageID, branch, transportType string) (*TransportRecord, string, bool, error) {
	transportType = strings.TrimSpace(transportType)
	idx, err := s.loadIndex()
	if err != nil {
		return nil, "", false, err
	}
	if idx == nil {
		return nil, "", false, nil
	}
	for i := len(idx.Items) - 1; i >= 0; i-- {
		it := idx.Items[i]
		if normalizeTransportStatus(it.TransportStatus) == "completed" {
			continue
		}
		if transportType != "" && normalizeTransportType(it.TransportType) != normalizeTransportType(transportType) {
			continue
		}
		r, err := s.LoadRecord(it.TransportID)
		if err != nil {
			continue
		}
		if packageID != "" && r.PackageID != packageID {
			continue
		}
		if branch != "" && r.Branch != branch {
			continue
		}
		if transportType != "" && normalizeTransportType(r.TransportType) != normalizeTransportType(transportType) {
			continue
		}
		p := s.recordPath(r.TransportID)
		cp := r
		return &cp, p, true, nil
	}
	return nil, "", false, nil
}
