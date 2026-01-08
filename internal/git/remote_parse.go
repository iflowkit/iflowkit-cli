package git

import (
	"fmt"
	"net/url"
	"strings"
)

func parseRemoteBase(gitServerURL, cpiPath string) (RemoteInfo, error) {
	gitServerURL = strings.TrimSpace(gitServerURL)
	cpiPath = strings.TrimSpace(cpiPath)
	if gitServerURL == "" {
		return RemoteInfo{}, fmt.Errorf("gitServerUrl is empty")
	}

	// Normalize cpiPath to a relative segment list (no leading slash).
	trimmedCPI := strings.Trim(cpiPath, "/")

	// Case 1: URL with scheme (https://host/namespace)
	if strings.Contains(gitServerURL, "://") {
		u, err := url.Parse(gitServerURL)
		if err != nil {
			return RemoteInfo{}, fmt.Errorf("invalid gitServerUrl: %w", err)
		}
		host := u.Hostname()
		ns := strings.Trim(u.Path, "/")
		if trimmedCPI != "" {
			ns = strings.Trim(ns+"/"+trimmedCPI, "/")
		}
		return RemoteInfo{Provider: detectProvider(host), Host: host, NamespacePath: ns}, nil
	}

	// Case 2: scp-like SSH remote: git@host:namespace
	// Example: git@gitlab.example.com:group/subgroup
	if at := strings.Index(gitServerURL, "@"); at >= 0 {
		rest := gitServerURL[at+1:]
		if colon := strings.Index(rest, ":"); colon >= 0 {
			host := rest[:colon]
			ns := strings.Trim(rest[colon+1:], "/")
			if trimmedCPI != "" {
				ns = strings.Trim(ns+"/"+trimmedCPI, "/")
			}
			return RemoteInfo{Provider: detectProvider(host), Host: host, NamespacePath: ns}, nil
		}
	}

	// Case 3: host/path without scheme (best-effort)
	parts := strings.SplitN(gitServerURL, "/", 2)
	host := parts[0]
	ns := ""
	if len(parts) == 2 {
		ns = strings.Trim(parts[1], "/")
	}
	if trimmedCPI != "" {
		ns = strings.Trim(ns+"/"+trimmedCPI, "/")
	}
	return RemoteInfo{Provider: detectProvider(host), Host: host, NamespacePath: ns}, nil
}

func detectProvider(host string) string {
	h := strings.ToLower(host)
	switch {
	case strings.Contains(h, "github"):
		return ProviderGitHub
	case strings.Contains(h, "gitlab"):
		return ProviderGitLab
	default:
		return ProviderUnknown
	}
}
