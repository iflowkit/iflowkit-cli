package archive

import "time"

const ManifestFileName = "iflowkit_archive.json"
const CurrentArchiveSchemaVersion = 1

type Manifest struct {
	Kind          string `json:"kind"` // "profile" or "config"
	SchemaVersion int    `json:"schema_version"`
	CreatedAt     string `json:"created_at"`
	ProfileID     string `json:"profile_id,omitempty"`
}

func NewManifest(kind string, profileID string) Manifest {
	return Manifest{
		Kind:          kind,
		SchemaVersion: CurrentArchiveSchemaVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		ProfileID:     profileID,
	}
}
