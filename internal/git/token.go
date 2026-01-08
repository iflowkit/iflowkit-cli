package git

import (
	"fmt"
	"os"
	"strings"
)

// ResolveToken reads an auth token from environment variables.
//
// Priority:
//
//	IFLOWKIT_GIT_TOKEN
//	Provider-specific fallbacks:
//	  GitHub:   GITHUB_TOKEN, GH_TOKEN
//	  GitLab:   GITLAB_TOKEN, GITLAB_PRIVATE_TOKEN
func ResolveToken(provider string) (string, error) {
	keys := []string{"IFLOWKIT_GIT_TOKEN"}
	switch provider {
	case ProviderGitHub:
		keys = append(keys, "GITHUB_TOKEN", "GH_TOKEN")
	case ProviderGitLab:
		keys = append(keys, "GITLAB_TOKEN", "GITLAB_PRIVATE_TOKEN")
	}
	for _, k := range keys {
		v := strings.TrimSpace(os.Getenv(k))
		if v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("git auth token not found; set IFLOWKIT_GIT_TOKEN (or provider-specific token env var)")
}
