package cpix

import (
	"context"
	"io"
	"net/http"
	"strings"
)

// DeleteArtifact deletes a design-time artifact via the Integration Content OData API.
// If version is empty, the Version key is omitted from the entity key.
func (c *Client) DeleteArtifact(ctx context.Context, entitySet, id, version, csrfToken, cookieHeader string) error {
	tok, err := c.getToken(ctx)
	if err != nil {
		return err
	}

	// Build OData entity URL.
	var urlStr string
	if strings.TrimSpace(version) == "" {
		urlStr = c.baseURL + "/api/v1/" + entitySet + "(Id='" + escapeODataID(id) + "')"
	} else {
		urlStr = c.baseURL + "/api/v1/" + entitySet + "(Id='" + escapeODataID(id) + "',Version='" + escapeODataID(version) + "')"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, urlStr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	// For OData deletes we usually need If-Match to avoid ETag handling.
	req.Header.Set("If-Match", "*")
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
		return &HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status, Body: strings.TrimSpace(string(b))}
	}
	return nil
}

// IsNotFound returns true if the error is an HTTP 404.
func IsNotFound(err error) bool {
	return isHTTPStatus(err, 404)
}

// IsBadRequest returns true if the error is an HTTP 400.
func IsBadRequest(err error) bool {
	return isHTTPStatus(err, 400)
}
