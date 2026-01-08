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

type gitlabProvider struct{}

func (p *gitlabProvider) Name() string { return ProviderGitLab }

func (p *gitlabProvider) NormalizeRepoDisplayName(name string) string {
	// GitLab has a separate display name from the URL path.
	return normalizeCommon(name, 255)
}

func (p *gitlabProvider) CreateRepo(ctx context.Context, token string, host string, namespace string, repoPath string, displayName string, private bool) error {
	apiBase := gitlabAPIBase(host)
	client := &http.Client{Timeout: 30 * time.Second}

	// Resolve group id (if namespace is provided).
	groupID := 0
	if strings.TrimSpace(namespace) != "" {
		if id, err := gitlabResolveGroupID(ctx, client, token, apiBase, namespace); err == nil {
			groupID = id
		}
	}

	vis := "private"
	if !private {
		vis = "public"
	}

	payload := map[string]any{
		"name":       displayName,
		"path":       repoPath,
		"visibility": vis,
	}
	if groupID > 0 {
		payload["namespace_id"] = groupID
	}

	urlStr := fmt.Sprintf("%s/projects", apiBase)
	if err := gitlabCreate(ctx, client, token, urlStr, payload); err != nil {
		if isGitLabAlreadyExists(err) {
			return nil
		}
		// If group creation failed due to permissions and we tried with namespace_id, retry without it.
		if groupID > 0 {
			delete(payload, "namespace_id")
			err2 := gitlabCreate(ctx, client, token, urlStr, payload)
			if err2 == nil {
				return nil
			}
			if isGitLabAlreadyExists(err2) {
				return nil
			}
			return err2
		}
		return err
	}
	return nil
}

func gitlabAPIBase(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return "https://gitlab.com/api/v4"
	}
	return fmt.Sprintf("https://%s/api/v4", h)
}

func gitlabResolveGroupID(ctx context.Context, client *http.Client, token string, apiBase string, namespace string) (int, error) {
	// GitLab allows GET /groups/:id where :id can be a URL-encoded full path.
	urlStr := fmt.Sprintf("%s/groups/%s", apiBase, url.PathEscape(strings.Trim(namespace, "/")))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "iflowkit-cli")
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("gitlab group resolve failed (%s): %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var gr struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(body, &gr); err != nil {
		return 0, err
	}
	if gr.ID == 0 {
		return 0, fmt.Errorf("gitlab group resolve returned empty id")
	}
	return gr.ID, nil
}

func gitlabCreate(ctx context.Context, client *http.Client, token string, urlStr string, payload map[string]any) error {
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "iflowkit-cli")
	req.Header.Set("PRIVATE-TOKEN", token)

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
	return fmt.Errorf("gitlab repo create failed (%s): %s", resp.Status, msg)
}

func isGitLabAlreadyExists(err error) bool {
	s := strings.ToLower(err.Error())
	// Common GitLab message: "has already been taken".
	return strings.Contains(s, "already been taken") || strings.Contains(s, "already exists")
}
