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
	"github.com/iflowkit/iflowkit-cli/internal/store"
	"github.com/iflowkit/iflowkit-cli/internal/validate"
)

func runProfile(ctx *Context, args []string) error {
	if len(args) == 0 {
		printProfileHelp(ctx, nil)
		return nil
	}
	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "init":
		return profileInit(ctx, subArgs)
	case "list":
		return profileList(ctx, subArgs)
	case "current":
		return profileCurrent(ctx, subArgs)
	case "show":
		return profileShow(ctx, subArgs)
	case "use":
		return profileUse(ctx, subArgs)
	case "delete":
		return profileDelete(ctx, subArgs)
	case "export":
		return profileExport(ctx, subArgs)
	case "import":
		return profileImport(ctx, subArgs)
	default:
		return fmt.Errorf("unknown subcommand: profile %s", sub)
	}
}

func profileInit(ctx *Context, argv []string) error {
	fs := flag.NewFlagSet("profile init", flag.ContinueOnError)
	fs.SetOutput(ctx.Stderr)
	overwrite := fs.Bool("overwrite", false, "Overwrite without prompting")
	if err := fs.Parse(argv); err != nil {
		return wrapFlagError(err)
	}

	io := prompt.NewIO(ctx.Stdin, ctx.Stdout)

	id, err := io.AskString("Profile id (safe folder name)", nil, validate.ProfileID)
	if err != nil {
		return err
	}
	exists, err := ctx.Stores.Profiles.Exists(id)
	if err != nil {
		return err
	}
	if exists && !*overwrite {
		ok, err := io.AskYesNo("Profile already exists. Overwrite?", false)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted")
		}
	}

	name, err := io.AskString("Profile name", nil, validate.RequiredNonEmpty("name"))
	if err != nil {
		return err
	}
	gitURL, err := io.AskString("Git server URL (scheme + host)", nil, validate.URLWithSchemeHost("gitServerUrl"))
	if err != nil {
		return err
	}
	cpiPath, err := io.AskString("CPI path (root path)", nil, validate.RequiredNonEmpty("cpiPath"))
	if err != nil {
		return err
	}
	levels, err := io.AskInt("CPI tenant levels (2 or 3)", nil, validate.IntInSet("cpiTenantLevels", 2, 3))
	if err != nil {
		return err
	}

	p := models.Profile{
		SchemaVersion:   models.CurrentProfileSchemaVersion,
		ID:              id,
		Name:            name,
		GitServerURL:    gitURL,
		CPIPath:         cpiPath,
		CPITenantLevels: levels,
	}
	if err := ctx.Stores.Profiles.Write(p, true); err != nil {
		return err
	}
	ctx.Logger.Info("profile initialized", logging.F("profile_id", id))
	fmt.Fprintf(ctx.Stdout, "Profile created: %s\n", id)
	return nil
}

func profileList(ctx *Context, argv []string) error {
	fs := flag.NewFlagSet("profile list", flag.ContinueOnError)
	fs.SetOutput(ctx.Stderr)
	if err := fs.Parse(argv); err != nil {
		return wrapFlagError(err)
	}

	profiles, err := ctx.Stores.Profiles.List()
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		fmt.Fprintln(ctx.Stdout, "(no profiles found)")
		return nil
	}
	for _, p := range profiles {
		fmt.Fprintf(ctx.Stdout, "- %s\t%s\n", p.ID, p.Name)
	}
	return nil
}

func profileCurrent(ctx *Context, argv []string) error {
	fs := flag.NewFlagSet("profile current", flag.ContinueOnError)
	fs.SetOutput(ctx.Stderr)
	if err := fs.Parse(argv); err != nil {
		return wrapFlagError(err)
	}

	activeBytes, _ := os.ReadFile(ctx.Paths.ActiveProfileFile)
	active := store.CleanSingleLine(string(activeBytes))

	resolved, src, err := ctx.Stores.ResolveProfileID(ctx.Flags.ProfileID)
	fmt.Fprintf(ctx.Stdout, "Active:   %s\n", active)
	if err != nil {
		fmt.Fprintln(ctx.Stdout, "Resolved: (none)")
		fmt.Fprintf(ctx.Stdout, "Error:    %v\n", err)
		return nil
	}
	fmt.Fprintf(ctx.Stdout, "Resolved: %s (%s)\n", resolved, src)
	return nil
}

func profileShow(ctx *Context, argv []string) error {
	fs := flag.NewFlagSet("profile show", flag.ContinueOnError)
	fs.SetOutput(ctx.Stderr)
	if err := fs.Parse(argv); err != nil {
		return wrapFlagError(err)
	}

	id, _, err := ctx.Stores.ResolveProfileID(ctx.Flags.ProfileID)
	if err != nil {
		return err
	}
	p, err := ctx.Stores.Profiles.Read(id)
	if err != nil {
		return err
	}
	b, _ := p.PrettyJSON()
	fmt.Fprintln(ctx.Stdout, string(b))
	return nil
}

func profileUse(ctx *Context, argv []string) error {
	fs := flag.NewFlagSet("profile use", flag.ContinueOnError)
	fs.SetOutput(ctx.Stderr)
	id := fs.String("id", "", "Profile id")
	if err := fs.Parse(argv); err != nil {
		return wrapFlagError(err)
	}
	if *id == "" {
		printProfileUseHelp(ctx)
		return fmt.Errorf("--id is required")
	}
	if err := ctx.Stores.Profiles.RequireExists(*id); err != nil {
		return err
	}
	if err := ctx.Stores.SetActiveProfileID(*id); err != nil {
		return err
	}
	ctx.Logger.Info("active profile set", logging.F("profile_id", *id))
	fmt.Fprintf(ctx.Stdout, "Active profile set: %s\n", *id)
	return nil
}

func profileDelete(ctx *Context, argv []string) error {
	fs := flag.NewFlagSet("profile delete", flag.ContinueOnError)
	fs.SetOutput(ctx.Stderr)
	id := fs.String("id", "", "Profile id")
	yes := fs.Bool("yes", false, "Confirm deletion")
	if err := fs.Parse(argv); err != nil {
		return wrapFlagError(err)
	}
	if *id == "" || !*yes {
		printProfileDeleteHelp(ctx)
		if *id == "" {
			return fmt.Errorf("--id is required")
		}
		return fmt.Errorf("refusing to delete without --yes")
	}
	if err := ctx.Stores.Profiles.Delete(*id); err != nil {
		return err
	}
	ctx.Logger.Warn("profile deleted", logging.F("profile_id", *id))
	fmt.Fprintf(ctx.Stdout, "Profile deleted: %s\n", *id)
	return nil
}

func profileExport(ctx *Context, argv []string) error {
	fs := flag.NewFlagSet("profile export", flag.ContinueOnError)
	fs.SetOutput(ctx.Stderr)
	id := fs.String("id", "", "Profile id")
	out := fs.String("out", "", "Output file path")
	overwrite := fs.Bool("overwrite", false, "Overwrite without prompting")
	if err := fs.Parse(argv); err != nil {
		return wrapFlagError(err)
	}
	if *id == "" {
		printProfileExportHelp(ctx)
		return fmt.Errorf("--id is required")
	}
	if err := ctx.Stores.Profiles.RequireExists(*id); err != nil {
		return err
	}

	finalOut := *out
	if finalOut == "" {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		cfg, err := ctx.Stores.Config.ReadOptional()
		if err != nil {
			return err
		}
		if cfg != nil && cfg.ProfileExportDir != "" {
			dir = cfg.ProfileExportDir
		}
		finalOut = filepath.Join(dir, fmt.Sprintf("%s-profile.iflowkit", *id))
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

	srcDir := ctx.Stores.Profiles.ProfileDir(*id)
	if err := archive.ExportProfile(srcDir, *id, finalOut); err != nil {
		return err
	}
	ctx.Logger.Info("profile exported", logging.F("profile_id", *id), logging.F("out", finalOut))
	fmt.Fprintf(ctx.Stdout, "Exported: %s\n", finalOut)
	return nil
}

func profileImport(ctx *Context, argv []string) error {
	fs := flag.NewFlagSet("profile import", flag.ContinueOnError)
	fs.SetOutput(ctx.Stderr)
	file := fs.String("file", "", "Input .iflowkit archive")
	overwrite := fs.Bool("overwrite", false, "Overwrite without prompting")
	if err := fs.Parse(argv); err != nil {
		return wrapFlagError(err)
	}
	if *file == "" {
		printProfileImportHelp(ctx)
		return fmt.Errorf("--file is required")
	}
	if _, err := os.Stat(*file); err != nil {
		return err
	}

	id, kind, err := archive.PeekArchive(*file)
	if err != nil {
		return err
	}
	if kind != "profile" {
		return fmt.Errorf("archive kind is %q, expected %q", kind, "profile")
	}

	exists, err := ctx.Stores.Profiles.Exists(id)
	if err != nil {
		return err
	}
	if exists && !*overwrite {
		io := prompt.NewIO(ctx.Stdin, ctx.Stdout)
		ok, err := io.AskYesNo(fmt.Sprintf("Profile %q exists. Overwrite?", id), false)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted")
		}
	}

	destDir := ctx.Stores.Profiles.ProfileDir(id)
	if err := archive.ImportProfile(*file, destDir, true); err != nil {
		return err
	}

	// Validate imported profile again (ensures required fields exist).
	p, err := ctx.Stores.Profiles.Read(id)
	if err != nil {
		return err
	}
	_ = p

	ctx.Logger.Info("profile imported", logging.F("profile_id", id), logging.F("file", *file))
	fmt.Fprintf(ctx.Stdout, "Imported profile: %s\n", id)
	return nil
}

func printProfileHelp(ctx *Context, path []string) {
	out := ctx.Stdout
	if len(path) == 0 {
		fmt.Fprintln(out, "Usage:")
		fmt.Fprintln(out, "  iflowkit profile <command> [args]")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Commands:")
		fmt.Fprintln(out, "  init     Create/overwrite profile.json interactively")
		fmt.Fprintln(out, "  list     List profiles")
		fmt.Fprintln(out, "  current  Show active profile and resolved profile")
		fmt.Fprintln(out, "  show     Show profile.json for the resolved profile")
		fmt.Fprintln(out, "  use      Set active profile id")
		fmt.Fprintln(out, "  delete   Delete an entire profile folder")
		fmt.Fprintln(out, "  export   Export a profile folder to a .iflowkit archive")
		fmt.Fprintln(out, "  import   Import a .iflowkit profile archive")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Try:")
		fmt.Fprintln(out, "  iflowkit help profile init")
		return
	}
	switch path[0] {
	case "init":
		printProfileInitHelp(ctx)
	case "use":
		printProfileUseHelp(ctx)
	case "delete":
		printProfileDeleteHelp(ctx)
	case "export":
		printProfileExportHelp(ctx)
	case "import":
		printProfileImportHelp(ctx)
	default:
		fmt.Fprintln(out, "Unknown profile command.")
	}
}

func printProfileInitHelp(ctx *Context) {
	fmt.Fprintln(ctx.Stdout, "Usage:")
	fmt.Fprintln(ctx.Stdout, "  iflowkit profile init [--overwrite]")
	fmt.Fprintln(ctx.Stdout, "")
	fmt.Fprintln(ctx.Stdout, "Creates/overwrites profile.json interactively.")
}

func printProfileUseHelp(ctx *Context) {
	fmt.Fprintln(ctx.Stdout, "Usage:")
	fmt.Fprintln(ctx.Stdout, "  iflowkit profile use --id <profileId>")
}

func printProfileDeleteHelp(ctx *Context) {
	fmt.Fprintln(ctx.Stdout, "Usage:")
	fmt.Fprintln(ctx.Stdout, "  iflowkit profile delete --id <profileId> --yes")
}

func printProfileExportHelp(ctx *Context) {
	fmt.Fprintln(ctx.Stdout, "Usage:")
	fmt.Fprintln(ctx.Stdout, "  iflowkit profile export --id <profileId> [--out <file.iflowkit>] [--overwrite]")
}

func printProfileImportHelp(ctx *Context) {
	fmt.Fprintln(ctx.Stdout, "Usage:")
	fmt.Fprintln(ctx.Stdout, "  iflowkit profile import --file <*.iflowkit> [--overwrite]")
}
