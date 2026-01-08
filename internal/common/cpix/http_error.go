package cpix

import "fmt"

// HTTPStatusError represents a non-2xx CPI response.
// It allows callers to branch on StatusCode for fallbacks.
type HTTPStatusError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPStatusError) Error() string {
	if e == nil {
		return ""
	}
	if e.Body != "" {
		return fmt.Sprintf("CPI request failed (%s): %s", e.Status, e.Body)
	}
	return fmt.Sprintf("CPI request failed (%s)", e.Status)
}

func isHTTPStatus(err error, codes ...int) bool {
	e, ok := err.(*HTTPStatusError)
	if !ok || e == nil {
		return false
	}
	for _, c := range codes {
		if e.StatusCode == c {
			return true
		}
	}
	return false
}
