package models

import (
	"encoding/json"
	"fmt"
)

const CurrentProfileSchemaVersion = 1

type Profile struct {
	SchemaVersion   int    `json:"schema_version"`
	ID              string `json:"id"`
	Name            string `json:"name"`
	GitServerURL    string `json:"gitServerUrl"`
	CPIPath         string `json:"cpiPath"`
	CPITenantLevels int    `json:"cpiTenantLevels"`
}

func (p Profile) PrettyJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

func (p Profile) ValidateRequired() error {
	if p.SchemaVersion == 0 {
		return fmt.Errorf("profile.json missing required field: schema_version")
	}
	if p.ID == "" {
		return fmt.Errorf("profile.json missing required field: id")
	}
	if p.Name == "" {
		return fmt.Errorf("profile.json missing required field: name")
	}
	if p.GitServerURL == "" {
		return fmt.Errorf("profile.json missing required field: gitServerUrl")
	}
	if p.CPIPath == "" {
		return fmt.Errorf("profile.json missing required field: cpiPath")
	}
	if p.CPITenantLevels == 0 {
		return fmt.Errorf("profile.json missing required field: cpiTenantLevels")
	}
	return nil
}
