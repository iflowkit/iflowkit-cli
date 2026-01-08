package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/logging"
	"github.com/iflowkit/iflowkit-cli/internal/paths"
)

type Stores struct {
	Paths    *paths.Paths
	Logger   *logging.Logger
	Profiles *ProfileStore
	Config   *ConfigStore
	Tenants  *TenantStore
}

func NewStores(p *paths.Paths, lg *logging.Logger) *Stores {
	return &Stores{
		Paths:    p,
		Logger:   lg,
		Profiles: NewProfileStore(p, lg),
		Config:   NewConfigStore(p, lg),
		Tenants:  NewTenantStore(p, lg),
	}
}

func (s *Stores) ResolveProfileID(explicit string) (id string, source string, err error) {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit), "--profile", nil
	}
	b, err := os.ReadFile(s.Paths.ActiveProfileFile)
	if err == nil {
		id = CleanSingleLine(string(b))
		if id != "" {
			return id, "active_profile", nil
		}
	}
	return "", "", fmt.Errorf("no profile selected; run `iflowkit profile init` or `iflowkit profile use --id <profileId>`")
}

func (s *Stores) SetActiveProfileID(id string) error {
	if err := os.MkdirAll(filepath.Dir(s.Paths.ActiveProfileFile), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.Paths.ActiveProfileFile), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(id); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_ = os.Remove(s.Paths.ActiveProfileFile) // allow rename on Windows
	return os.Rename(tmp.Name(), s.Paths.ActiveProfileFile)
}

func CleanSingleLine(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return strings.TrimSpace(s)
}
