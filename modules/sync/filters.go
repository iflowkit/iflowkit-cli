package sync

import (
	"path/filepath"
	"strings"
)

func filterNonTransportChanges(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		norm := filepath.ToSlash(strings.TrimSpace(p))
		if norm == "" {
			continue
		}
		if strings.HasPrefix(norm, ".iflowkit/transports/") || norm == ".iflowkit/transports" {
			continue
		}
		out = append(out, p)
	}
	return out
}
