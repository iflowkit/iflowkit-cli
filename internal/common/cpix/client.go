package cpix

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iflowkit/iflowkit-cli/internal/common/filex"
	"github.com/iflowkit/iflowkit-cli/internal/common/logx"
	"github.com/iflowkit/iflowkit-cli/internal/models"
)

type Client struct {
	baseURL      string
	tokenURL     string
	clientID     string
	clientSecret string
	httpClient   *http.Client
	lg           *logx.Logger

	// token cache
	token    string
	tokenExp time.Time
}

func NewClient(t models.TenantServiceKey, lg *logx.Logger) *Client {
	return &Client{
		baseURL:      strings.TrimRight(t.OAuth.URL, "/"),
		tokenURL:     t.OAuth.TokenURL,
		clientID:     t.OAuth.ClientID,
		clientSecret: t.OAuth.ClientSecret,
		httpClient:   &http.Client{Timeout: 60 * time.Second},
		lg:           lg,
	}
}

type IntegrationPackage struct {
	ID   string
	Name string
}

// ArtifactInfo contains read/write information for a design-time artifact.
// Note: For uploads, CPI's Integration Content API typically expects a JSON payload
// with a base64-encoded zip (ArtifactContent). MediaSrc/edit_media are often usable
// for downloads, but not reliably for updates.
type ArtifactInfo struct {
	ID        string
	Name      string
	Version   string
	URI       string
	MediaSrc  string
	EditMedia string
}

// ReadIntegrationPackage reads the main IntegrationPackages('<id>') payload.
// It returns parsed metadata and the raw JSON bytes (saved later as IntegrationPackage.json).
func (c *Client) ReadIntegrationPackage(packageID string) (IntegrationPackage, []byte, error) {
	path := fmt.Sprintf("/api/v1/IntegrationPackages('%s')", escapeODataID(packageID))
	b, err := c.getRaw(context.Background(), path, "application/json")
	if err != nil {
		return IntegrationPackage{}, nil, err
	}
	var resp integrationPackageResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		return IntegrationPackage{}, nil, fmt.Errorf("invalid CPI response: %w", err)
	}
	return IntegrationPackage{ID: resp.D.ID, Name: resp.D.Name}, b, nil
}

// ExportIntegrationPackageFromRaw writes raw main payload and exports related artifacts.
func (c *Client) ExportIntegrationPackageFromRaw(packageID string, rawMainJSON []byte, destDir string) error {
	if err := filex.EnsureDir(destDir); err != nil {
		return err
	}
	if err := filex.AtomicWriteFile(filepath.Join(destDir, "IntegrationPackage.json"), rawMainJSON, 0o644); err != nil {
		return err
	}

	mainPath := fmt.Sprintf("/api/v1/IntegrationPackages('%s')", escapeODataID(packageID))

	sets := []artifactSet{
		{Folder: "iFlows", ListEndpoint: mainPath + "/IntegrationDesigntimeArtifacts", ListFile: "IntegrationDesigntimeArtifacts.json"},
		{Folder: "ValueMappings", ListEndpoint: mainPath + "/ValueMappingDesigntimeArtifacts", ListFile: "ValueMappingDesigntimeArtifacts.json"},
		{Folder: "MessageMappings", ListEndpoint: mainPath + "/MessageMappingDesigntimeArtifacts", ListFile: "MessageMappingDesigntimeArtifacts.json"},
		{Folder: "Scripts", ListEndpoint: mainPath + "/ScriptCollectionDesigntimeArtifacts", ListFile: "ScriptCollectionDesigntimeArtifacts.json"},
		{Folder: "CustomTags", ListEndpoint: mainPath + "/CustomTags", ListFile: "CustomTags.json"},
	}

	for _, s := range sets {
		if err := c.exportArtifactSet(destDir, s); err != nil {
			return err
		}
	}
	return nil
}

type artifactSet struct {
	Folder       string
	ListEndpoint string
	ListFile     string
}

func (c *Client) exportArtifactSet(destDir string, s artifactSet) error {
	folder := filepath.Join(destDir, s.Folder)
	if err := filex.EnsureDir(folder); err != nil {
		return err
	}

	if c.lg != nil {
		c.lg.Info("reading CPI artifacts", logx.F("folder", s.Folder))
	}
	listJSON, err := c.getRaw(context.Background(), s.ListEndpoint, "application/json")
	if err != nil {
		// Some packages might not have certain artifact types; keep this soft.
		if c.lg != nil {
			c.lg.Warn("artifact list request failed", logx.F("folder", s.Folder), logx.F("error", err.Error()))
		}
		return nil
	}
	if err := filex.AtomicWriteFile(filepath.Join(folder, s.ListFile), listJSON, 0o644); err != nil {
		return err
	}
	var lr listResponse
	if err := json.Unmarshal(listJSON, &lr); err != nil {
		return fmt.Errorf("invalid CPI list response (%s): %w", s.Folder, err)
	}

	for _, it := range lr.D.Results {
		id := strings.TrimSpace(it.ID)
		media := strings.TrimSpace(it.Metadata.MediaSrc)
		if id == "" || media == "" {
			continue
		}
		if c.lg != nil {
			c.lg.Info("downloading artifact", logx.F("folder", s.Folder), logx.F("id", id))
		}
		zipPath := filepath.Join(folder, fmt.Sprintf("%s.zip", id))
		if err := c.downloadToFile(context.Background(), media, "application/zip", zipPath); err != nil {
			return err
		}
		target := filepath.Join(folder, id)
		if err := filex.ExtractZipFile(zipPath, target); err != nil {
			return err
		}
		_ = os.Remove(zipPath)
	}
	return nil
}

// ListArtifacts returns a map[id]ArtifactInfo for a list endpoint.
// listEndpoint can be a relative path ("/api/v1/..."), or a full URL.
func (c *Client) ListArtifacts(ctx context.Context, listEndpoint string) (map[string]ArtifactInfo, error) {
	listJSON, err := c.getRaw(ctx, listEndpoint, "application/json")
	if err != nil {
		return nil, err
	}
	var lr listResponse
	if err := json.Unmarshal(listJSON, &lr); err != nil {
		return nil, fmt.Errorf("invalid CPI list response: %w", err)
	}
	m := make(map[string]ArtifactInfo, len(lr.D.Results))
	for _, it := range lr.D.Results {
		id := strings.TrimSpace(it.ID)
		if id == "" {
			continue
		}
		m[id] = ArtifactInfo{
			ID:        id,
			Name:      strings.TrimSpace(it.Name),
			Version:   strings.TrimSpace(it.Version),
			URI:       strings.TrimSpace(it.Metadata.URI),
			MediaSrc:  strings.TrimSpace(it.Metadata.MediaSrc),
			EditMedia: strings.TrimSpace(it.Metadata.EditMedia),
		}
	}
	return m, nil
}

// UpdateArtifact uploads the given artifact content as a base64 encoded zip.
// This follows the Integration Content API pattern (JSON payload + CSRF).
func (c *Client) UpdateArtifact(ctx context.Context, artifactEntitySet string, info ArtifactInfo, zipBytes []byte, csrfToken, cookieHeader string) error {
	if info.URI == "" {
		// Fallback: build an entity URL if CPI didn't return one.
		if info.Version == "" {
			return fmt.Errorf("cannot update artifact %q: missing __metadata.uri and Version", info.ID)
		}
		info.URI = c.baseURL + fmt.Sprintf("/api/v1/%s(Id='%s',Version='%s')", artifactEntitySet, escapeODataID(info.ID), escapeODataID(info.Version))
	}

	payload := struct {
		ArtifactContent string `json:"ArtifactContent"`
		Name            string `json:"Name,omitempty"`
	}{
		ArtifactContent: base64.StdEncoding.EncodeToString(zipBytes),
		Name:            info.Name,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.doJSONWrite(ctx, http.MethodPut, info.URI, body, csrfToken, cookieHeader, nil)
}

func (c *Client) doJSONWrite(ctx context.Context, method, urlStr string, body []byte, csrfToken, cookieHeader string, extraHeaders map[string]string) error {
	tok, err := c.getToken(ctx)
	if err != nil {
		return err
	}
	if strings.HasPrefix(urlStr, "/") {
		urlStr = c.baseURL + urlStr
	}
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	// For OData updates we usually need If-Match to avoid ETag handling.
	req.Header.Set("If-Match", "*")
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}
	if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("CPI upload failed (%s): %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

// FetchCSRFToken fetches a CSRF token and returns it along with cookie header value.
// CPI OData write operations often require X-CSRF-Token + session cookies.
func (c *Client) FetchCSRFToken(ctx context.Context) (string, string, error) {
	tok, err := c.getToken(ctx)
	if err != nil {
		return "", "", err
	}

	urlStr := c.baseURL + "/api/v1/IntegrationPackages?$top=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-CSRF-Token", "Fetch")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	csrf := strings.TrimSpace(resp.Header.Get("X-CSRF-Token"))
	cookies := make([]string, 0, 4)
	for _, sc := range resp.Header.Values("Set-Cookie") {
		// Keep only the name=value part.
		part := strings.SplitN(sc, ";", 2)[0]
		part = strings.TrimSpace(part)
		if part != "" {
			cookies = append(cookies, part)
		}
	}
	cookieHeader := strings.Join(cookies, "; ")

	if csrf == "" {
		return "", cookieHeader, fmt.Errorf("CSRF token missing in response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", cookieHeader, fmt.Errorf("CSRF token fetch failed: %s", resp.Status)
	}
	return csrf, cookieHeader, nil
}

// PutZip uploads a zip archive to the given CPI media URL using PUT.
func (c *Client) PutZip(ctx context.Context, pathOrURL string, zipBytes []byte, csrfToken, cookieHeader string) error {
	tok, err := c.getToken(ctx)
	if err != nil {
		return err
	}

	urlStr := pathOrURL
	if strings.HasPrefix(pathOrURL, "/") {
		urlStr = c.baseURL + pathOrURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, urlStr, bytes.NewReader(zipBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/zip")
	req.Header.Set("Accept", "application/json")
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}
	if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("CPI upload failed (%s): %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

// DeployIntegrationDesigntimeArtifact triggers a deployment of an iFlow in CPI.
// It uses the standard DeployIntegrationDesigntimeArtifact endpoint.
func (c *Client) DeployIntegrationDesigntimeArtifact(ctx context.Context, id, version, csrfToken, cookieHeader string) error {
	return c.deployByEndpoint(ctx, "DeployIntegrationDesigntimeArtifact", id, version, csrfToken, cookieHeader)
}

// DeployScriptCollectionDesigntimeArtifact triggers a deployment of a script collection artifact.
func (c *Client) DeployScriptCollectionDesigntimeArtifact(ctx context.Context, id, version, csrfToken, cookieHeader string) error {
	return c.deployByEndpoint(ctx, "DeployScriptCollectionDesigntimeArtifact", id, version, csrfToken, cookieHeader)
}

// DeployValueMappingDesigntimeArtifact triggers a deployment of a value mapping artifact.
func (c *Client) DeployValueMappingDesigntimeArtifact(ctx context.Context, id, version, csrfToken, cookieHeader string) error {
	return c.deployByEndpoint(ctx, "DeployValueMappingDesigntimeArtifact", id, version, csrfToken, cookieHeader)
}

// DeployMessageMappingDesigntimeArtifact triggers a deployment of a message mapping artifact.
func (c *Client) DeployMessageMappingDesigntimeArtifact(ctx context.Context, id, version, csrfToken, cookieHeader string) error {
	return c.deployByEndpoint(ctx, "DeployMessageMappingDesigntimeArtifact", id, version, csrfToken, cookieHeader)
}

func (c *Client) deployByEndpoint(ctx context.Context, endpointName, id, version, csrfToken, cookieHeader string) error {
	tok, err := c.getToken(ctx)
	if err != nil {
		return err
	}
	// CPI deployment should target the currently active design-time version.
	// Using the concrete Version from list responses may not trigger a deployment in some tenants.
	// Therefore we always deploy with Version='active'.
	version = "active"

	urlStr := c.baseURL + fmt.Sprintf("/api/v1/%s?Id='%s'&Version='%s'", endpointName, escapeODataID(id), escapeODataID(version))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}
	if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("CPI deploy failed (%s): %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

// GetIntegrationRuntimeArtifact returns the runtime deployment status for an artifact id.
// It queries IntegrationRuntimeArtifacts and returns (item, found).
func (c *Client) GetIntegrationRuntimeArtifact(ctx context.Context, id string) (RuntimeArtifactStatus, bool, error) {
	q := url.Values{}
	q.Set("$top", "1")
	q.Set("$filter", fmt.Sprintf("Id eq '%s'", escapeODataID(id)))
	path := "/api/v1/IntegrationRuntimeArtifacts?" + q.Encode()
	b, err := c.getRaw(ctx, path, "application/json")
	if err != nil {
		return RuntimeArtifactStatus{}, false, err
	}
	var resp runtimeListResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		return RuntimeArtifactStatus{}, false, fmt.Errorf("invalid CPI runtime response: %w", err)
	}
	if len(resp.D.Results) == 0 {
		return RuntimeArtifactStatus{}, false, nil
	}
	it := resp.D.Results[0]
	return RuntimeArtifactStatus{
		ID:         strings.TrimSpace(it.ID),
		Name:       strings.TrimSpace(it.Name),
		Status:     strings.TrimSpace(it.Status),
		DeployedOn: strings.TrimSpace(it.DeployedOn),
	}, true, nil
}

// RuntimeArtifactStatus is a simplified view of IntegrationRuntimeArtifacts.
type RuntimeArtifactStatus struct {
	ID         string
	Name       string
	Status     string
	DeployedOn string
}

// Internal runtime list response (OData v2 style).
type runtimeListResponse struct {
	D struct {
		Results []runtimeArtifactItem `json:"results"`
	} `json:"d"`
}

type runtimeArtifactItem struct {
	ID         string `json:"Id"`
	Name       string `json:"Name"`
	Status     string `json:"Status"`
	DeployedOn string `json:"DeployedOn"`
}

// --- HTTP ---

func (c *Client) getToken(ctx context.Context) (string, error) {
	if c.token != "" && time.Now().Before(c.tokenExp) {
		return c.token, nil
	}
	u, err := url.Parse(c.tokenURL)
	if err != nil {
		return "", fmt.Errorf("invalid tokenurl: %w", err)
	}
	q := u.Query()
	if q.Get("grant_type") == "" {
		q.Set("grant_type", "client_credentials")
		u.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(nil))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.clientID, c.clientSecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token request failed: %s", strings.TrimSpace(string(b)))
	}
	var tr tokenResponse
	if err := json.Unmarshal(b, &tr); err != nil {
		return "", fmt.Errorf("invalid token response: %w", err)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("token response missing access_token")
	}
	c.token = tr.AccessToken
	// expires_in might be missing; default to 5 minutes.
	exp := 300
	if tr.ExpiresIn > 0 {
		exp = tr.ExpiresIn
	}
	c.tokenExp = time.Now().Add(time.Duration(exp-30) * time.Second)
	return c.token, nil
}

func (c *Client) getRaw(ctx context.Context, pathOrURL string, accept string) ([]byte, error) {
	tok, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}
	urlStr := pathOrURL
	if strings.HasPrefix(pathOrURL, "/") {
		urlStr = c.baseURL + pathOrURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	req.Header.Set("Authorization", "Bearer "+tok)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("CPI request failed (%s): %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return b, nil
}

func (c *Client) downloadToFile(ctx context.Context, urlStr string, accept string, dest string) error {
	b, err := c.getRaw(ctx, urlStr, accept)
	if err != nil {
		return err
	}
	return filex.AtomicWriteFile(dest, b, 0o644)
}

func escapeODataID(id string) string {
	// OData uses single quotes; escape by doubling them.
	return strings.ReplaceAll(id, "'", "''")
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type integrationPackageResponse struct {
	D struct {
		ID   string `json:"Id"`
		Name string `json:"Name"`
	} `json:"d"`
}

type listResponse struct {
	D struct {
		Results []artifactItem `json:"results"`
	} `json:"d"`
}

type artifactItem struct {
	ID       string `json:"Id"`
	Name     string `json:"Name"`
	Version  string `json:"Version"`
	Metadata struct {
		URI       string `json:"uri"`
		MediaSrc  string `json:"media_src"`
		EditMedia string `json:"edit_media"`
	} `json:"__metadata"`
}
