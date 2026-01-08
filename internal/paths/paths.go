package paths

import (
	"os"
	"path/filepath"
)

type Paths struct {
	ConfigRoot        string
	ProfilesDir       string
	ConfigFile        string
	ActiveProfileFile string
	LogsDir           string
}

func New() (*Paths, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(base, "iflowkit")
	return &Paths{
		ConfigRoot:        root,
		ProfilesDir:       filepath.Join(root, "profiles"),
		ConfigFile:        filepath.Join(root, "config.json"),
		ActiveProfileFile: filepath.Join(root, "active_profile"),
		LogsDir:           filepath.Join(root, "logs"),
	}, nil
}
