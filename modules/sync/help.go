package sync

import (
	"fmt"

	"github.com/iflowkit/iflowkit-cli/internal/app"
)

func syncHelp(ctx *app.Context, path []string) {
	if len(path) > 0 {
		switch path[0] {
		case "init":
			syncInitHelp(ctx)
			return
		case "push":
			syncPushHelp(ctx)
			return
		case "pull":
			syncPullHelp(ctx)
			return
		case "deploy":
			syncDeployHelp(ctx)
			return
		case "deliver":
			syncDeliverHelp(ctx)
			return
		case "compare":
			syncCompareHelp(ctx)
			return
		}
	}

	out := ctx.Stdout
	fmt.Fprintln(out, "Sync module")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  iflowkit sync <command> [args]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Commands:")
	fmt.Fprintln(out, "  init   Initialize a Git repository for a CPI IntegrationPackage and export DEV artifacts")
	fmt.Fprintln(out, "  pull   Refresh local repo from CPI (based on current branch) and push CPI state to that branch")
	fmt.Fprintln(out, "  push   Push local changes to git and update CPI tenant (based on current branch)")
	fmt.Fprintln(out, "  deliver Promote changes between environment branches and update the target tenant")
	fmt.Fprintln(out, "  compare Show IntegrationPackage differences between current branch and an environment branch")
	fmt.Fprintln(out, "  deploy Inspect local deployment records (status/remaining work)")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Help:")
	fmt.Fprintln(out, "  iflowkit help sync")
	fmt.Fprintln(out, "  iflowkit help sync init")
	fmt.Fprintln(out, "  iflowkit help sync pull")
	fmt.Fprintln(out, "  iflowkit help sync push")
	fmt.Fprintln(out, "  iflowkit help sync deliver")
	fmt.Fprintln(out, "  iflowkit help sync compare")
	fmt.Fprintln(out, "  iflowkit help sync deploy")
	fmt.Fprintln(out, "")
}

func syncCompareHelp(ctx *app.Context) {
	out := ctx.Stdout
	fmt.Fprintln(out, "Compare IntegrationPackage content between branches")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  iflowkit sync compare --to qas|prd")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "What it does:")
	fmt.Fprintln(out, "  - Compares the current branch with origin/<to> using git diff (IntegrationPackage/ only)")
	fmt.Fprintln(out, "  - Applies ignore patterns from .iflowkit/ignore (plus built-in defaults)")
	fmt.Fprintln(out, "  - Prints a summary list: Kind - ObjectId")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Rules:")
	fmt.Fprintln(out, "  - --to qas: only when cpiTenantLevels=3")
	fmt.Fprintln(out, "  - --to prd: when cpiTenantLevels=2 or 3")
	fmt.Fprintln(out, "")
}

func syncDeliverHelp(ctx *app.Context) {
	out := ctx.Stdout
	fmt.Fprintln(out, "Promote changes between environments (branch merge + tenant update)")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  iflowkit sync deliver --to qas|prd [--message <commitMessage>]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Rules:")
	fmt.Fprintln(out, "  - --to qas: only when cpiTenantLevels=3 (DEV -> QAS)")
	fmt.Fprintln(out, "  - --to prd: when cpiTenantLevels=3 (QAS -> PRD) or cpiTenantLevels=2 (DEV -> PRD)")
	fmt.Fprintln(out, "  - PRD safety: --to prd is mandatory (this flag is the confirmation)")
	fmt.Fprintln(out, "  - Compares tenant vs target branch using .iflowkit/ignore; if different, the command fails")
	fmt.Fprintln(out, "  - If origin/qas or origin/prd does not exist, it is bootstrapped from the tenant (init transport + tag)")
	fmt.Fprintln(out, "  - Writes a transport record (*.transport.json) with transportType=deliver under .iflowkit/transports/<tenant>/")
	fmt.Fprintln(out, "  - On success, creates and pushes a git tag named <transportId> on the target branch")
	fmt.Fprintln(out, "")
}

func syncPullHelp(ctx *app.Context) {
	out := ctx.Stdout
	fmt.Fprintln(out, "Refresh local repo from CPI and push CPI state to Git")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  iflowkit sync pull [--to dev|qas|prd] [--message <commitMessage>]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "What it does:")
	fmt.Fprintln(out, "  - Must be executed inside an existing sync repo (finds .iflowkit/package.json)")
	fmt.Fprintln(out, "  - Allowed branch: dev, qas (only when cpiTenantLevels=3), prd")
	fmt.Fprintln(out, "  - Reads the IntegrationPackage from the mapped tenant and re-exports to IntegrationPackage/")
	fmt.Fprintln(out, "  - Commits and pushes the CPI state to origin/<current-branch>")
	fmt.Fprintln(out, "  - PRD safety: must pass --to prd")
	fmt.Fprintln(out, "  - Writes a transport record (*.transport.json) with transportType=pull")
	fmt.Fprintln(out, "")
}

func syncDeployHelp(ctx *app.Context) {
	out := ctx.Stdout
	fmt.Fprintln(out, "Inspect deployment records")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  iflowkit sync deploy status [--env dev|qas|prd] [--transport <transportId>]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Notes:")
	fmt.Fprintln(out, "  - Reads local records under .iflowkit/transports/")
	fmt.Fprintln(out, "  - By default, shows the most recent record")
	fmt.Fprintln(out, "")
}

func syncInitHelp(ctx *app.Context) {
	out := ctx.Stdout
	fmt.Fprintln(out, "Initialize a sync repository from DEV tenant")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  iflowkit sync init --id <packageId> [--dir <parentPath>]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Notes:")
	fmt.Fprintln(out, "  - Uses DEV tenant only (no --env)")
	fmt.Fprintln(out, "  - If --dir is provided, the repo is created under <parentPath>/<packageId>")
	fmt.Fprintln(out, "  - Creates a private repo on GitHub/GitLab when possible")
	fmt.Fprintln(out, "  - Pushes exported content to branch 'dev'")
	fmt.Fprintln(out, "  - Writes sync metadata to .iflowkit/package.json")
	fmt.Fprintln(out, "  - Writes/updates .gitignore (does not ignore .iflowkit/transports)")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Examples:")
	fmt.Fprintln(out, "  iflowkit sync init --id com.iflowkit.cpi.email")
	fmt.Fprintln(out, "")
}

func syncPushHelp(ctx *app.Context) {
	out := ctx.Stdout
	fmt.Fprintln(out, "Push local changes to Git and update CPI tenant")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  iflowkit sync push [--to dev|qas|prd] [--message <commitMessage>]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "What it does:")
	fmt.Fprintln(out, "  - Finds .iflowkit/package.json by walking up from current directory")
	fmt.Fprintln(out, "  - Detects local changes via git diff (including untracked files)")
	fmt.Fprintln(out, "  - Commits and pushes the current branch to origin")
	fmt.Fprintln(out, "  - Updates the mapped CPI tenant only for changed artifacts under IntegrationPackage/")
	fmt.Fprintln(out, "  - PRD safety: must pass --to prd")
	fmt.Fprintln(out, "  - Deploys updated iFlows after upload")
	fmt.Fprintln(out, "  - Uses .iflowkit/transports/<tenant>/index.json and *.transport.json records as retry state after CPI failures")
	fmt.Fprintln(out, "  - On environment branches (dev/qas/prd), creates and pushes a git tag named <transportId>")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Branch rules:")
	fmt.Fprintln(out, "  - Environment branches: dev, qas (only when cpiTenantLevels=3), prd")
	fmt.Fprintln(out, "  - Work branches: feature/*, bugfix/* (mapped to DEV tenant)")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Examples:")
	fmt.Fprintln(out, "  iflowkit sync push")
	fmt.Fprintln(out, "  iflowkit sync push --message \"Update iFlow step\"")
	fmt.Fprintln(out, "")
}
