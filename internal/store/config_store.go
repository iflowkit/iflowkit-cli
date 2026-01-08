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

type ConfigStore struct {
	path string
	lg   *logging.Logger
}

func NewConfigStore(p *paths.Paths, lg *logging.Logger) *ConfigStore {
	return &ConfigStore{path: p.ConfigFile, lg: lg}
}

func (s *ConfigStore) ReadOptional() (*models.Config, error) {
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cfg, err := parseConfig(b)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *ConfigStore) Read() (models.Config, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return models.Config{}, err
	}
	return parseConfig(b)
}

func parseConfig(b []byte) (models.Config, error) {
	var cfg models.Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return models.Config{}, fmt.Errorf("invalid config.json: %w", err)
	}
	if err := cfg.ValidateRequired(); err != nil {
		return models.Config{}, err
	}
	if cfg.SchemaVersion != models.CurrentConfigSchemaVersion {
		return models.Config{}, fmt.Errorf("unsupported config schema_version %d (current: %d)", cfg.SchemaVersion, models.CurrentConfigSchemaVersion)
	}
	if err := validate.PathString("profileExportDir")(cfg.ProfileExportDir); err != nil {
		return models.Config{}, err
	}
	return cfg, nil
}

func (s *ConfigStore) Write(cfg models.Config, overwrite bool) error {
	if err := cfg.ValidateRequired(); err != nil {
		return err
	}
	if err := validate.PathString("profileExportDir")(cfg.ProfileExportDir); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(s.path); err == nil && !overwrite {
		return fmt.Errorf("config.json already exists")
	}
	if overwrite {
		_ = os.Remove(s.path)
	}
	return filex.AtomicWriteFile(s.path, b, 0o644)
}
