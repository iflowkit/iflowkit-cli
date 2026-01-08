package models

import (
	"encoding/json"
	"fmt"
)

// SyncMetadata is stored under .iflowkit/package.json inside a sync-initialized repository.
// It is intentionally independent from the core config/profile schema_version fields
// and uses camelCase as required by the sync module.
type SyncMetadata struct {
	SchemaVersion   int    `json:"schemaVersion"`
	ProfileID       string `json:"profileId"`
	CPITenantLevels int    `json:"cpiTenantLevels"`
	PackageID       string `json:"packageId"`
	PackageName     string `json:"packageName"`
	BaseFolder      string `json:"baseFolder"`
	GitRemote       string `json:"gitRemote"`
	GitProvider     string `json:"gitProvider"`
	CreatedAt       string `json:"createdAt"`
}

func (m SyncMetadata) ValidateRequired() error {
	if m.SchemaVersion == 0 {
		return fmt.Errorf("sync metadata missing required field: schemaVersion")
	}
	if m.ProfileID == "" {
		return fmt.Errorf("sync metadata missing required field: profileId")
	}
	if m.CPITenantLevels == 0 {
		return fmt.Errorf("sync metadata missing required field: cpiTenantLevels")
	}
	if m.PackageID == "" {
		return fmt.Errorf("sync metadata missing required field: packageId")
	}
	if m.PackageName == "" {
		return fmt.Errorf("sync metadata missing required field: packageName")
	}
	if m.BaseFolder == "" {
		return fmt.Errorf("sync metadata missing required field: baseFolder")
	}
	if m.GitRemote == "" {
		return fmt.Errorf("sync metadata missing required field: gitRemote")
	}
	if m.GitProvider == "" {
		return fmt.Errorf("sync metadata missing required field: gitProvider")
	}
	if m.CreatedAt == "" {
		return fmt.Errorf("sync metadata missing required field: createdAt")
	}
	return nil
}

func (m SyncMetadata) PrettyJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}
