package sync

import "strings"

func isAllowedPushBranch(branch string) bool {
	branch = strings.TrimSpace(branch)
	if branch == "dev" {
		return true
	}
	if branch == "qas" {
		return true
	}
	if branch == "prd" {
		return true
	}
	if strings.HasPrefix(branch, "feature/") {
		return true
	}
	if strings.HasPrefix(branch, "bugfix/") {
		return true
	}
	return false
}
