package sync

// artifactKey uniquely identifies an artifact by its kind folder and artifact id.
type artifactKey struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// SyncObject describes a CPI object (artifact) by kind and id.
type SyncObject struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type deployTarget struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

func (k artifactKey) isZero() bool {
	return k.Kind == "" || k.ID == ""
}

func (o SyncObject) isZero() bool {
	return o.Kind == "" || o.ID == ""
}

func (d deployTarget) isZero() bool {
	return d.Kind == "" || d.ID == ""
}
