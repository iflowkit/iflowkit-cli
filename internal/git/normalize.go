package git

import (
	"regexp"
	"strings"
)

var (
	// Keep it simple and provider-friendly.
	nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
)

func normalizeCommon(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-._")
	if s == "" {
		return "repo"
	}
	if maxLen > 0 && len(s) > maxLen {
		s = s[:maxLen]
		s = strings.Trim(s, "-._")
		if s == "" {
			return "repo"
		}
	}
	return s
}
