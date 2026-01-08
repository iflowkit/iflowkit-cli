package sync

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/app"
	"github.com/iflowkit/iflowkit-cli/internal/logging"
)

func runSyncCompare(ctx *app.Context, args []string) error {
	fs := flag.NewFlagSet("iflowkit sync compare", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var to string
	fs.StringVar(&to, "to", "", "Target environment branch (qas|prd)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		syncCompareHelp(ctx)
		return err
	}
	to = strings.ToLower(strings.TrimSpace(to))
	if to != "qas" && to != "prd" {
		return fmt.Errorf("--to must be qas or prd")
	}

	cwd, _ := os.Getwd()
	repoRoot, err := findSyncRepoRoot(cwd)
	if err != nil {
		return err
	}
	meta, err := loadPackageMetadata(repoRoot)
	if err != nil {
		return err
	}
	if err := meta.ValidateRequired(); err != nil {
		return err
	}

	levels := meta.CPITenantLevels
	if to == "qas" && levels != 3 {
		return fmt.Errorf("qas compare is not enabled: cpiTenantLevels=%d (expected 3)", levels)
	}
	if to == "prd" && levels != 2 && levels != 3 {
		return fmt.Errorf("invalid cpiTenantLevels=%d (expected 2 or 3)", levels)
	}

	branch, _ := gitCurrentBranch(ctx, repoRoot)
	ctx.Logger.Info("sync compare started", logging.F("repo", repoRoot), logging.F("from", branch), logging.F("to", to))

	// Best-effort refresh.
	_ = runGit(ctx, repoRoot, "fetch", "origin")

	// Target branch must exist on the remote.
	if !gitRemoteBranchExists(ctx, repoRoot, to) {
		return fmt.Errorf("target branch origin/%s does not exist", to)
	}

	ign, err := LoadRepoIgnore(repoRoot)
	if err != nil {
		return err
	}

	baseFolder := resolveContentFolder(meta)
	rightRef := "origin/" + to
	// Compare current working branch HEAD (may include local commits) with the remote branch.
	out, err := runGitOutput(ctx, repoRoot, "diff", "--name-only", rightRef+"..HEAD", "--", baseFolder)
	if err != nil {
		return err
	}
	changedPaths := splitLines(out)
	changedPaths = ign.Filter(changedPaths)
	keys := detectChangedArtifacts(meta, changedPaths)
	objs := keysToObjects(keys)

	if len(objs) == 0 {
		fmt.Fprintf(ctx.Stdout, "No IntegrationPackage differences between %s and %s (after applying .iflowkit/ignore).\n", branch, rightRef)
		return nil
	}

	fmt.Fprintf(ctx.Stdout, "IntegrationPackage differences (after applying .iflowkit/ignore): %s vs %s\n", branch, rightRef)
	for _, o := range objs {
		fmt.Fprintf(ctx.Stdout, "%s - %s\n", o.Kind, o.ID)
	}
	return nil
}
