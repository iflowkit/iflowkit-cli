package git

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type githubProvider struct{}

func (p *githubProvider) Name() string { return ProviderGitHub }

func (p *githubProvider) NormalizeRepoDisplayName(name string) string {
	// GitHub repository name itself is used in the URL. In iFlowKit, the URL path is
	// derived from packageId, so displayName is primarily used as description.
	return normalizeCommon(name, 100)
}

func (p *githubProvider) CreateRepo(ctx context.Context, token string, host string, namespace string, repoPath string, displayName string, private bool) error {
	owner := firstSegment(namespace)
	if owner == "" {
		return fmt.Errorf("unable to determine GitHub owner from namespace: %q", namespace)
	}
	apiBase := githubAPIBase(host)
	client := &http.Client{Timeout: 30 * time.Second}

	payload := map[string]any{
		"name":        repoPath,
		"private":     private,
		"description": displayName,
		"auto_init":   false,
	}

	// Try org repo creation first, then fallback to user.
	orgURL := fmt.Sprintf("%s/orgs/%s/repos", apiBase, url.PathEscape(owner))
	if err := githubCreate(ctx, client, token, orgURL, payload); err == nil {
		return nil
	} else if !isGitHubAuthOrNotFound(err) {
		// If it's a hard error other than forbidden/not found, keep it.
		// If repo already exists, treat as success.
		if isAlreadyExists(err) {
			return nil
		}
		return err
	}

	userURL := fmt.Sprintf("%s/user/repos", apiBase)
	if err := githubCreate(ctx, client, token, userURL, payload); err != nil {
		if isAlreadyExists(err) {
			return nil
		}
		return err
	}
	return nil
}

func githubAPIBase(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return "https://api.github.com"
	}
	if h == "github.com" {
		return "https://api.github.com"
	}
	return fmt.Sprintf("https://%s/api/v3", h)
}

func githubCreate(ctx context.Context, client *http.Client, token string, urlStr string, payload map[string]any) error {
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "iflowkit-cli")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = resp.Status
	}
	// Wrap with a provider-specific error string.
	return fmt.Errorf("github repo create failed (%s): %s", resp.Status, msg)
}

func isGitHubAuthOrNotFound(err error) bool {
	// best-effort: look for common status codes in error string
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "403") || strings.Contains(s, "404") || strings.Contains(s, "forbidden") || strings.Contains(s, "not found")
}

func isAlreadyExists(err error) bool {
	s := strings.ToLower(err.Error())
	// GitHub returns 422 with message "name already exists on this account".
	return strings.Contains(s, "already exists") || strings.Contains(s, "422")
}

func firstSegment(ns string) string {
	ns = strings.Trim(ns, "/")
	if ns == "" {
		return ""
	}
	parts := strings.Split(ns, "/")
	return strings.TrimSpace(parts[0])
}
