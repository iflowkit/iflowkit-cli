package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iflowkit/iflowkit-cli/internal/store"
)

func runWhere(ctx *Context, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("where does not accept subcommands")
	}
	p := ctx.Paths

	fmt.Fprintf(ctx.Stdout, "Config root:         %s\n", p.ConfigRoot)
	fmt.Fprintf(ctx.Stdout, "Profiles dir:        %s\n", p.ProfilesDir)
	fmt.Fprintf(ctx.Stdout, "Config file:         %s\n", p.ConfigFile)
	fmt.Fprintf(ctx.Stdout, "Active profile file: %s\n", p.ActiveProfileFile)
	fmt.Fprintf(ctx.Stdout, "Logs dir:            %s\n", p.LogsDir)
	fmt.Fprintln(ctx.Stdout, "")

	active, _ := os.ReadFile(p.ActiveProfileFile)
	activeStr := store.CleanSingleLine(string(active))
	fmt.Fprintf(ctx.Stdout, "Active profile id:   %s\n", activeStr)

	resolved, src, err := ctx.Stores.ResolveProfileID(ctx.Flags.ProfileID)
	if err != nil {
		fmt.Fprintln(ctx.Stdout, "Resolved profile:    (none)")
		fmt.Fprintf(ctx.Stdout, "Resolution error:    %v\n", err)
		return nil
	}
	fmt.Fprintf(ctx.Stdout, "Resolved profile:    %s (%s)\n", resolved, src)
	fmt.Fprintf(ctx.Stdout, "Resolved path:       %s\n", filepath.Join(p.ProfilesDir, resolved))
	return nil
}

func printWhereHelp(ctx *Context) {
	fmt.Fprintln(ctx.Stdout, "Usage:")
	fmt.Fprintln(ctx.Stdout, "  iflowkit where")
	fmt.Fprintln(ctx.Stdout, "")
	fmt.Fprintln(ctx.Stdout, "Shows local config locations and current profile context.")
}
