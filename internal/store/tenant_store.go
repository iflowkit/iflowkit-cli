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
)

type TenantStore struct {
	profilesDir string
	lg          *logging.Logger
}

func NewTenantStore(p *paths.Paths, lg *logging.Logger) *TenantStore {
	return &TenantStore{profilesDir: p.ProfilesDir, lg: lg}
}

func (s *TenantStore) tenantFile(profileID, env string) string {
	return filepath.Join(s.profilesDir, profileID, "tenants", fmt.Sprintf("%s.json", env))
}

func (s *TenantStore) Write(profileID, env string, t models.TenantServiceKey) error {
	if err := t.ValidateRequired(); err != nil {
		return err
	}
	file := s.tenantFile(profileID, env)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	_ = os.Remove(file)
	return filex.AtomicWriteFile(file, b, 0o644)
}

func (s *TenantStore) Read(profileID, env string) (models.TenantServiceKey, error) {
	b, err := os.ReadFile(s.tenantFile(profileID, env))
	if err != nil {
		return models.TenantServiceKey{}, err
	}
	var t models.TenantServiceKey
	if err := json.Unmarshal(b, &t); err != nil {
		return models.TenantServiceKey{}, fmt.Errorf("invalid tenant JSON: %w", err)
	}
	if err := t.ValidateRequired(); err != nil {
		return models.TenantServiceKey{}, err
	}
	return t, nil
}

func (s *TenantStore) Delete(profileID, env string) error {
	return os.Remove(s.tenantFile(profileID, env))
}
