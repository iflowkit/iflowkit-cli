package validate

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

var profileIDRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

func ProfileID(s string) error {
	if strings.TrimSpace(s) != s || s == "" {
		return fmt.Errorf("id must not be empty and must not contain leading/trailing spaces")
	}
	if strings.ContainsAny(s, " \t\r\n") {
		return fmt.Errorf("id must not contain whitespace")
	}
	if strings.Contains(s, string(filepath.Separator)) || strings.Contains(s, "/") || strings.Contains(s, "\\") {
		return fmt.Errorf("id must not contain path separators")
	}
	if s == "." || s == ".." || strings.Contains(s, "..") {
		return fmt.Errorf("id must not contain '..'")
	}
	if !profileIDRe.MatchString(s) {
		return fmt.Errorf("id must match %s", profileIDRe.String())
	}
	return nil
}

func RequiredNonEmpty(field string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%s is required", field)
		}
		return nil
	}
}

func URLWithSchemeHost(field string) func(string) error {
	return func(s string) error {
		u, err := url.Parse(s)
		if err != nil {
			return fmt.Errorf("%s is not a valid URL: %w", field, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("%s must include scheme and host", field)
		}
		return nil
	}
}

func IntInSet(field string, allowed ...int) func(int) error {
	return func(v int) error {
		for _, a := range allowed {
			if v == a {
				return nil
			}
		}
		return fmt.Errorf("%s must be one of %v", field, allowed)
	}
}

func PathString(field string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%s is required", field)
		}
		return nil
	}
}

func Env(env string) error {
	switch env {
	case "dev", "qas", "prd":
		return nil
	default:
		return fmt.Errorf("invalid --env %q (allowed: dev|qas|prd)", env)
	}
}
