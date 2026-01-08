package git

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// BuildRemoteURL constructs the repository URL as:
//
//	gitServerUrl + cpiPath + <packageId>.git
//
// Example:
//
//	gitServerUrl=https://github.com
//	cpiPath=/acebe
//	packageId=com.iflowkit.cpi.email
//	=> https://github.com/acebe/com.iflowkit.cpi.email.git
func BuildRemoteURL(gitServerURL, cpiPath, packageID string) (string, error) {
	gitServerURL = strings.TrimSpace(gitServerURL)
	cpiPath = strings.TrimSpace(cpiPath)
	packageID = strings.TrimSpace(packageID)
	if gitServerURL == "" {
		return "", fmt.Errorf("gitServerUrl is empty")
	}
	if packageID == "" {
		return "", fmt.Errorf("packageId is empty")
	}

	// If it's a URL, join paths safely.
	if strings.Contains(gitServerURL, "://") {
		u, err := url.Parse(gitServerURL)
		if err != nil {
			return "", fmt.Errorf("invalid gitServerUrl: %w", err)
		}
		// Preserve any base path included in gitServerUrl.
		base := strings.Trim(u.Path, "/")
		cp := strings.Trim(cpiPath, "/")
		parts := []string{}
		if base != "" {
			parts = append(parts, base)
		}
		if cp != "" {
			parts = append(parts, cp)
		}
		parts = append(parts, packageID+".git")
		u.Path = "/" + path.Join(parts...)
		return u.String(), nil
	}

	// Best-effort fallback for non-scheme remotes.
	// We treat it as host[/base] and add cpiPath and repo.
	trim := strings.Trim(gitServerURL, "/")
	base := trim
	cp := strings.Trim(cpiPath, "/")
	if cp != "" {
		base = base + "/" + cp
	}
	return base + "/" + packageID + ".git", nil
}

// DetectProviderFromRemote infers git provider from remote URL host.
func DetectProviderFromRemote(remote string) string {
	if strings.Contains(remote, "://") {
		if u, err := url.Parse(remote); err == nil {
			return detectProvider(u.Hostname())
		}
	}
	return detectProvider(remote)
}

// SplitRemoteNamespaceAndRepo extracts "namespace" and "repo" from an https remote.
// For https://github.com/acebe/com.iflowkit.cpi.email.git => namespace=acebe, repo=com.iflowkit.cpi.email
func SplitRemoteNamespaceAndRepo(remote string) (namespace string, repo string, err error) {
	u, err := url.Parse(remote)
	if err != nil {
		return "", "", fmt.Errorf("invalid remote URL: %w", err)
	}
	p := strings.Trim(u.Path, "/")
	if p == "" {
		return "", "", fmt.Errorf("remote URL missing path: %s", remote)
	}
	segs := strings.Split(p, "/")
	if len(segs) < 2 {
		return "", "", fmt.Errorf("remote URL must include namespace and repo: %s", remote)
	}
	repoSeg := segs[len(segs)-1]
	repoSeg = strings.TrimSuffix(repoSeg, ".git")
	ns := strings.Join(segs[:len(segs)-1], "/")
	if ns == "" || repoSeg == "" {
		return "", "", fmt.Errorf("invalid remote URL: %s", remote)
	}
	return ns, repoSeg, nil
}

// RemoteHost extracts the hostname from an https remote.
func RemoteHost(remote string) (string, error) {
	u, err := url.Parse(remote)
	if err != nil {
		return "", fmt.Errorf("invalid remote URL: %w", err)
	}
	h := strings.TrimSpace(u.Hostname())
	if h == "" {
		return "", fmt.Errorf("remote URL missing host: %s", remote)
	}
	return h, nil
}
