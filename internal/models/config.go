package models

import (
	"encoding/json"
	"fmt"
)

const CurrentConfigSchemaVersion = 1

type Config struct {
	SchemaVersion    int    `json:"schema_version"`
	ProfileExportDir string `json:"profileExportDir"`
}

func (c Config) PrettyJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func (c Config) ValidateRequired() error {
	if c.SchemaVersion == 0 {
		return fmt.Errorf("config.json missing required field: schema_version")
	}
	if c.ProfileExportDir == "" {
		return fmt.Errorf("config.json missing required field: profileExportDir")
	}
	return nil
}
