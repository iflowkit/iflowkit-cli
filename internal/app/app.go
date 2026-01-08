package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/common/errorx"
	"github.com/iflowkit/iflowkit-cli/internal/logging"
	"github.com/iflowkit/iflowkit-cli/internal/paths"
	"github.com/iflowkit/iflowkit-cli/internal/store"
)

type GlobalFlags struct {
	ProfileID string
	LogLevel  string
	LogFormat string
}

type Context struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	Paths  *paths.Paths
	Logger *logging.Logger
	Stores *store.Stores
	Flags  GlobalFlags
}

func Run(argv []string) error {
	ctx := &Context{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}

	flags := GlobalFlags{
		ProfileID: "",
		LogLevel:  "info",
		LogFormat: "text",
	}

	fs := flag.NewFlagSet("iflowkit", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // we control all output
	fs.StringVar(&flags.ProfileID, "profile", "", "Profile id to use for the command (overrides active profile)")
	fs.StringVar(&flags.LogLevel, "log-level", "info", "Log level: trace|debug|info|warn|error")
	fs.StringVar(&flags.LogFormat, "log-format", "text", "Log format: text|json")

	if err := fs.Parse(argv); err != nil {
		fmt.Fprintln(ctx.Stderr, err.Error())
		printRootHelp(ctx)
		return err
	}
	args := fs.Args()
	ctx.Flags = flags

	p, err := paths.New()
	if err != nil {
		fmt.Fprintln(ctx.Stderr, err.Error())
		return err
	}
	ctx.Paths = p

	_ = os.MkdirAll(p.ConfigRoot, 0o755)
	_ = os.MkdirAll(p.ProfilesDir, 0o755)
	_ = os.MkdirAll(p.LogsDir, 0o755)

	lg, err := logging.New(logging.Options{
		LogsDir: p.LogsDir,
		Level:   flags.LogLevel,
		Format:  flags.LogFormat,
		Stdout:  ctx.Stdout,
		Stderr:  ctx.Stderr,
		Cmdline: append([]string{"iflowkit"}, argv...),
	})
	if err != nil {
		fmt.Fprintln(ctx.Stderr, err.Error())
		return err
	}
	defer lg.Close()
	ctx.Logger = lg
	ctx.Stores = store.NewStores(p, lg)

	if len(args) == 0 {
		printRootHelp(ctx)
		return nil
	}

	// Dispatch.
	cmdPath := args
	ctx.Logger.Info("command started", logging.F("cmd", strings.Join(cmdPath, " ")))

	err = dispatch(ctx, cmdPath)
	if err != nil {
		// Do not spam usage for all errors; only known cases.
		fmt.Fprintln(ctx.Stderr, errorx.UserError(err))
		ctx.Logger.Error("command failed", logging.F("error", err.Error()))
		return err
	}
	ctx.Logger.Info("command finished", logging.F("cmd", strings.Join(cmdPath, " ")))

	return nil
}

func dispatch(ctx *Context, args []string) error {
	if len(args) == 0 {
		return nil
	}

	// help is special
	if args[0] == "help" {
		return runHelp(ctx, args[1:])
	}

	switch args[0] {
	case "where":
		return runWhere(ctx, args[1:])
	case "profile":
		return runProfile(ctx, args[1:])
	case "tenant":
		return runTenant(ctx, args[1:])
	case "config":
		return runConfig(ctx, args[1:])
	default:
		if ext, ok := getExternalCommand(args[0]); ok {
			return ext.Run(ctx, args[1:])
		}
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func runHelp(ctx *Context, path []string) error {
	if len(path) == 0 {
		printRootHelp(ctx)
		return nil
	}
	// Print help for a command path.
	switch path[0] {
	case "where":
		printWhereHelp(ctx)
	case "profile":
		printProfileHelp(ctx, path[1:])
	case "tenant":
		printTenantHelp(ctx, path[1:])
	case "config":
		printConfigHelp(ctx, path[1:])
	default:
		if ext, ok := getExternalCommand(path[0]); ok {
			if ext.Help != nil {
				ext.Help(ctx, path[1:])
				return nil
			}
			return fmt.Errorf("no help available for command: %s", strings.Join(path, " "))
		}
		return fmt.Errorf("unknown command: %s", strings.Join(path, " "))
	}
	return nil
}

func printRootHelp(ctx *Context) {
	out := ctx.Stdout
	fmt.Fprintln(out, "iFlowKit CLI")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  iflowkit [--profile <profileId>] [--log-level <trace|debug|info|warn|error>] [--log-format <text|json>] <command> [args]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Commands:")
	fmt.Fprintln(out, "  help        Show help")
	fmt.Fprintln(out, "  where       Show local config locations and current profile context")
	fmt.Fprintln(out, "  profile     Customer profile management")
	fmt.Fprintln(out, "  tenant      CPI tenant (service key) management")
	fmt.Fprintln(out, "  config      Developer preferences management")
	for _, name := range listExternalCommandNames() {
		fmt.Fprintf(out, "  %-10s %s\n", name, "Product module")
	}
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Examples:")
	fmt.Fprintln(out, "  iflowkit config init")
	fmt.Fprintln(out, "  iflowkit profile init")
	fmt.Fprintln(out, "  iflowkit profile use --id acme")
	fmt.Fprintln(out, "  iflowkit tenant import --file service-key.json --env dev")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Config root:")
	fmt.Fprintln(out, "  "+filepath.Join(mustUserConfigDir(), "iflowkit"))
}

func mustUserConfigDir() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return "(unknown)"
	}
	return d
}

func wrapFlagError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	return err
}
