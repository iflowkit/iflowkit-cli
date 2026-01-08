package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iflowkit/iflowkit-cli/internal/common/filex"
	"github.com/iflowkit/iflowkit-cli/internal/logging"
	"github.com/iflowkit/iflowkit-cli/internal/models"
	"github.com/iflowkit/iflowkit-cli/internal/paths"
	"github.com/iflowkit/iflowkit-cli/internal/validate"
)

type ProfileStore struct {
	profilesDir string
	lg          *logging.Logger
}

func NewProfileStore(p *paths.Paths, lg *logging.Logger) *ProfileStore {
	return &ProfileStore{profilesDir: p.ProfilesDir, lg: lg}
}

func (s *ProfileStore) ProfileDir(id string) string {
	return filepath.Join(s.profilesDir, id)
}

func (s *ProfileStore) profileFile(id string) string {
	return filepath.Join(s.ProfileDir(id), "profile.json")
}

func (s *ProfileStore) Exists(id string) (bool, error) {
	_, err := os.Stat(s.profileFile(id))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *ProfileStore) RequireExists(id string) error {
	exists, err := s.Exists(id)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("profile %q not found; run `iflowkit profile init`", id)
	}
	return nil
}

func (s *ProfileStore) Read(id string) (models.Profile, error) {
	b, err := os.ReadFile(s.profileFile(id))
	if err != nil {
		return models.Profile{}, err
	}
	var p models.Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return models.Profile{}, fmt.Errorf("invalid profile.json: %w", err)
	}

	if err := p.ValidateRequired(); err != nil {
		return models.Profile{}, err
	}
	if err := validate.ProfileID(p.ID); err != nil {
		return models.Profile{}, err
	}
	if p.SchemaVersion != models.CurrentProfileSchemaVersion {
		return models.Profile{}, fmt.Errorf("unsupported profile schema_version %d (current: %d)", p.SchemaVersion, models.CurrentProfileSchemaVersion)
	}
	if err := validate.RequiredNonEmpty("name")(p.Name); err != nil {
		return models.Profile{}, err
	}
	if err := validate.URLWithSchemeHost("gitServerUrl")(p.GitServerURL); err != nil {
		return models.Profile{}, err
	}
	if err := validate.RequiredNonEmpty("cpiPath")(p.CPIPath); err != nil {
		return models.Profile{}, err
	}
	if err := validate.IntInSet("cpiTenantLevels", 2, 3)(p.CPITenantLevels); err != nil {
		return models.Profile{}, err
	}
	return p, nil
}

func (s *ProfileStore) Write(p models.Profile, overwrite bool) error {
	if err := validate.ProfileID(p.ID); err != nil {
		return err
	}
	if err := p.ValidateRequired(); err != nil {
		return err
	}
	if err := validate.URLWithSchemeHost("gitServerUrl")(p.GitServerURL); err != nil {
		return err
	}
	if err := validate.IntInSet("cpiTenantLevels", 2, 3)(p.CPITenantLevels); err != nil {
		return err
	}

	dir := s.ProfileDir(p.ID)
	if err := os.MkdirAll(filepath.Join(dir, "tenants"), 0o755); err != nil {
		return err
	}
	file := s.profileFile(p.ID)

	if _, err := os.Stat(file); err == nil && !overwrite {
		return fmt.Errorf("profile %q already exists", p.ID)
	}

	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}

	if overwrite {
		_ = os.Remove(file)
	}
	if err := filex.AtomicWriteFile(file, b, 0o644); err != nil {
		return err
	}
	return nil
}

func (s *ProfileStore) Delete(id string) error {
	return os.RemoveAll(s.ProfileDir(id))
}

func (s *ProfileStore) List() ([]models.Profile, error) {
	entries, err := os.ReadDir(s.profilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]models.Profile, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		p, err := s.Read(id)
		if err != nil {
			s.lg.Warn("skipping invalid profile", logging.F("profile_id", id), logging.F("error", err.Error()))
			continue
		}
		out = append(out, p)
	}
	return out, nil
}
