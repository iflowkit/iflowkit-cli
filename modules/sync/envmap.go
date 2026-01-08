package sync

import (
	"fmt"
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/models"
)

// resolveTargetTenant maps the current git branch to a CPI tenant environment.
//
// Rules:
//   - dev -> dev
//   - qas -> qas (only when cpiTenantLevels==3)
//   - prd -> prd
//   - feature/*, bugfix/* -> dev
//
// It returns:
//   - tenant: dev|qas|prd
//   - isEnvBranch: true only for the exact env branches (dev/qas/prd)
func resolveTargetTenant(meta models.SyncMetadata, branch string) (tenant string, isEnvBranch bool, err error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "", false, fmt.Errorf("cannot resolve tenant from empty branch")
	}

	levels := meta.CPITenantLevels
	switch branch {
	case "dev":
		return "dev", true, nil
	case "qas":
		if levels != 3 {
			return "", false, fmt.Errorf("branch 'qas' is not enabled: cpiTenantLevels=%d (expected 3)", levels)
		}
		return "qas", true, nil
	case "prd":
		if levels != 2 && levels != 3 {
			return "", false, fmt.Errorf("invalid cpiTenantLevels=%d (expected 2 or 3)", levels)
		}
		return "prd", true, nil
	default:
		if strings.HasPrefix(branch, "feature/") || strings.HasPrefix(branch, "bugfix/") {
			return "dev", false, nil
		}
		return "", false, fmt.Errorf("branch %q is not supported by sync (allowed: dev%s, prd, feature/*, bugfix/*)", branch, func() string {
			if levels == 3 {
				return ", qas"
			}
			return ""
		}())
	}
}

// validateToFlag enforces the safety rule for PRD operations.
//
// If tenant==prd, --to prd is mandatory.
// If --to is provided for other tenants, it must match the resolved tenant.
func validateToFlag(toFlag string, tenant string) error {
	toFlag = strings.ToLower(strings.TrimSpace(toFlag))
	tenant = strings.ToLower(strings.TrimSpace(tenant))

	if tenant == "prd" {
		if toFlag != "prd" {
			return fmt.Errorf("refusing to run against PRD without explicit confirmation: pass --to prd")
		}
		return nil
	}

	if toFlag == "" {
		return nil
	}
	if toFlag != tenant {
		return fmt.Errorf("--to %s does not match target tenant %s", toFlag, tenant)
	}
	return nil
}

func tenantDisplay(env string) string {
	env = strings.ToUpper(strings.TrimSpace(env))
	if env == "" {
		return "UNKNOWN"
	}
	return env
}
