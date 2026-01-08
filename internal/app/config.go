package app

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iflowkit/iflowkit-cli/internal/archive"
	"github.com/iflowkit/iflowkit-cli/internal/logging"
	"github.com/iflowkit/iflowkit-cli/internal/models"
	"github.com/iflowkit/iflowkit-cli/internal/prompt"
	"github.com/iflowkit/iflowkit-cli/internal/validate"
)

func runConfig(ctx *Context, args []string) error {
	if len(args) == 0 {
		printConfigHelp(ctx, nil)
		return nil
	}
	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "init":
		return configInit(ctx, subArgs)
	case "show":
		return configShow(ctx, subArgs)
	case "export":
		return configExport(ctx, subArgs)
	case "import":
		return configImport(ctx, subArgs)
	default:
		return fmt.Errorf("unknown subcommand: config %s", sub)
	}
}

func configInit(ctx *Context, argv []string) error {
	fs := flag.NewFlagSet("config init", flag.ContinueOnError)
	fs.SetOutput(ctx.Stderr)
	if err := fs.Parse(argv); err != nil {
		return wrapFlagError(err)
	}

	io := prompt.NewIO(ctx.Stdin, ctx.Stdout)

	existing, err := ctx.Stores.Config.ReadOptional()
	if err != nil {
		return err
	}
	current := ""
	if existing != nil {
		current = existing.ProfileExportDir
	}

	exportDir, err := io.AskString("Profile export directory", &current, validate.RequiredNonEmpty("profileExportDir"))
	if err != nil {
		return err
	}
	if err := validate.PathString("profileExportDir")(exportDir); err != nil {
		return err
	}

	if _, err := os.Stat(exportDir); os.IsNotExist(err) {
		ok, err := io.AskYesNo("Directory does not exist. Create it?", true)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted")
		}
		if err := os.MkdirAll(exportDir, 0o755); err != nil {
			return err
		}
	}

	cfg := models.Config{
		SchemaVersion:    models.CurrentConfigSchemaVersion,
		ProfileExportDir: exportDir,
	}
	if err := ctx.Stores.Config.Write(cfg, true); err != nil {
		return err
	}
	ctx.Logger.Info("config initialized", logging.F("config_file", ctx.Paths.ConfigFile))
	fmt.Fprintf(ctx.Stdout, "Config saved: %s\n", ctx.Paths.ConfigFile)
	return nil
}

func configShow(ctx *Context, argv []string) error {
	fs := flag.NewFlagSet("config show", flag.ContinueOnError)
	fs.SetOutput(ctx.Stderr)
	if err := fs.Parse(argv); err != nil {
		return wrapFlagError(err)
	}

	cfg, err := ctx.Stores.Config.Read()
	if err != nil {
		return err
	}
	b, _ := cfg.PrettyJSON()
	fmt.Fprintln(ctx.Stdout, string(b))
	return nil
}

func configExport(ctx *Context, argv []string) error {
	fs := flag.NewFlagSet("config export", flag.ContinueOnError)
	fs.SetOutput(ctx.Stderr)
	out := fs.String("out", "", "Output file path (defaults to ./config.iflowkit)")
	overwrite := fs.Bool("overwrite", false, "Overwrite without prompting")
	if err := fs.Parse(argv); err != nil {
		return wrapFlagError(err)
	}

	finalOut := *out
	if finalOut == "" {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		finalOut = filepath.Join(dir, "config.iflowkit")
	}

	if _, err := os.Stat(finalOut); err == nil && !*overwrite {
		io := prompt.NewIO(ctx.Stdin, ctx.Stdout)
		ok, err := io.AskYesNo(fmt.Sprintf("File exists: %s. Overwrite?", finalOut), false)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted")
		}
	}

	if err := archive.ExportConfig(ctx.Paths.ConfigFile, finalOut); err != nil {
		return err
	}
	ctx.Logger.Info("config exported", logging.F("out", finalOut))
	fmt.Fprintf(ctx.Stdout, "Exported: %s\n", finalOut)
	return nil
}

func configImport(ctx *Context, argv []string) error {
	fs := flag.NewFlagSet("config import", flag.ContinueOnError)
	fs.SetOutput(ctx.Stderr)
	file := fs.String("file", "", "Input .iflowkit archive")
	overwrite := fs.Bool("overwrite", false, "Overwrite without prompting")
	if err := fs.Parse(argv); err != nil {
		return wrapFlagError(err)
	}
	if *file == "" {
		printConfigImportHelp(ctx)
		return fmt.Errorf("--file is required")
	}
	if _, err := os.Stat(*file); err != nil {
		return err
	}

	_, kind, err := archive.PeekArchive(*file)
	if err != nil {
		return err
	}
	if kind != "config" {
		return fmt.Errorf("archive kind is %q, expected %q", kind, "config")
	}

	if _, err := os.Stat(ctx.Paths.ConfigFile); err == nil && !*overwrite {
		io := prompt.NewIO(ctx.Stdin, ctx.Stdout)
		ok, err := io.AskYesNo("config.json exists. Overwrite?", false)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted")
		}
	}

	if err := archive.ImportConfig(*file, ctx.Paths.ConfigFile, true); err != nil {
		return err
	}
	// Validate after import
	if _, err := ctx.Stores.Config.Read(); err != nil {
		return err
	}

	ctx.Logger.Info("config imported", logging.F("file", *file))
	fmt.Fprintf(ctx.Stdout, "Imported config: %s\n", ctx.Paths.ConfigFile)
	return nil
}

func printConfigHelp(ctx *Context, path []string) {
	out := ctx.Stdout
	if len(path) == 0 {
		fmt.Fprintln(out, "Usage:")
		fmt.Fprintln(out, "  iflowkit config <command> [args]")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Commands:")
		fmt.Fprintln(out, "  init    Create/overwrite config.json interactively")
		fmt.Fprintln(out, "  show    Show config.json")
		fmt.Fprintln(out, "  export  Export config.json as a .iflowkit archive")
		fmt.Fprintln(out, "  import  Import config.json from a .iflowkit archive")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Try:")
		fmt.Fprintln(out, "  iflowkit help config init")
		return
	}
	switch path[0] {
	case "import":
		printConfigImportHelp(ctx)
	case "export":
		printConfigExportHelp(ctx)
	default:
		fmt.Fprintln(out, "Unknown config command.")
	}
}

func printConfigImportHelp(ctx *Context) {
	fmt.Fprintln(ctx.Stdout, "Usage:")
	fmt.Fprintln(ctx.Stdout, "  iflowkit config import --file <file.iflowkit> [--overwrite]")
}

func printConfigExportHelp(ctx *Context) {
	fmt.Fprintln(ctx.Stdout, "Usage:")
	fmt.Fprintln(ctx.Stdout, "  iflowkit config export [--out <file.iflowkit>] [--overwrite]")
}
