package git

import "context"

// Provider performs provider-specific operations such as repository creation
// and display-name normalization.
//
// host is the remote host (e.g. github.com).
// repoPath is the URL/path segment used in remote URLs (e.g. repo name).
// displayName is a human-friendly name derived from CPI package Name.
// namespace is the owner or group path extracted from the remote URL.
type Provider interface {
	Name() string
	NormalizeRepoDisplayName(name string) string
	CreateRepo(ctx context.Context, token string, host string, namespace string, repoPath string, displayName string, private bool) error
}
