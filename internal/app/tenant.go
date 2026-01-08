package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/iflowkit/iflowkit-cli/internal/logging"
	"github.com/iflowkit/iflowkit-cli/internal/models"
	"github.com/iflowkit/iflowkit-cli/internal/validate"
)

func runTenant(ctx *Context, args []string) error {
	if len(args) == 0 {
		printTenantHelp(ctx, nil)
		return nil
	}
	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "import":
		return tenantImport(ctx, subArgs)
	case "show":
		return tenantShow(ctx, subArgs)
	case "set":
		return tenantSet(ctx, subArgs)
	case "delete":
		return tenantDelete(ctx, subArgs)
	default:
		return fmt.Errorf("unknown subcommand: tenant %s", sub)
	}
}

func tenantImport(ctx *Context, argv []string) error {
	fs := flag.NewFlagSet("tenant import", flag.ContinueOnError)
	fs.SetOutput(ctx.Stderr)
	file := fs.String("file", "", "Service key JSON file")
	env := fs.String("env", "dev", "Environment: dev|qas|prd")
	if err := fs.Parse(argv); err != nil {
		return wrapFlagError(err)
	}
	if *file == "" {
		printTenantImportHelp(ctx)
		return fmt.Errorf("--file is required")
	}
	if err := validate.Env(*env); err != nil {
		return err
	}

	profileID, _, err := ctx.Stores.ResolveProfileID(ctx.Flags.ProfileID)
	if err != nil {
		return err
	}
	if err := ctx.Stores.Profiles.RequireExists(profileID); err != nil {
		return err
	}

	b, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	var t models.TenantServiceKey
	if err := json.Unmarshal(b, &t); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := t.ValidateRequired(); err != nil {
		return err
	}

	if err := ctx.Stores.Tenants.Write(profileID, *env, t); err != nil {
		return err
	}
	ctx.Logger.Info("tenant imported", logging.F("profile_id", profileID), logging.F("env", *env))
	fmt.Fprintf(ctx.Stdout, "Tenant stored: %s/%s\n", profileID, *env)
	return nil
}

func tenantShow(ctx *Context, argv []string) error {
	fs := flag.NewFlagSet("tenant show", flag.ContinueOnError)
	fs.SetOutput(ctx.Stderr)
	env := fs.String("env", "dev", "Environment: dev|qas|prd")
	if err := fs.Parse(argv); err != nil {
		return wrapFlagError(err)
	}
	if err := validate.Env(*env); err != nil {
		return err
	}

	profileID, _, err := ctx.Stores.ResolveProfileID(ctx.Flags.ProfileID)
	if err != nil {
		return err
	}

	t, err := ctx.Stores.Tenants.Read(profileID, *env)
	if err != nil {
		return err
	}
	b, _ := t.PrettyJSON()
	fmt.Fprintln(ctx.Stdout, string(b))
	return nil
}

func tenantSet(ctx *Context, argv []string) error {
	fs := flag.NewFlagSet("tenant set", flag.ContinueOnError)
	fs.SetOutput(ctx.Stderr)
	env := fs.String("env", "dev", "Environment: dev|qas|prd")
	url := fs.String("url", "", "CPI base URL")
	tokenURL := fs.String("token-url", "", "OAuth token URL")
	clientID := fs.String("client-id", "", "OAuth client id")
	clientSecret := fs.String("client-secret", "", "OAuth client secret")
	createdAt := fs.String("created-at", "", "Created date (RFC3339/RFC3339Nano). Defaults to now (UTC).")
	if err := fs.Parse(argv); err != nil {
		return wrapFlagError(err)
	}
	if err := validate.Env(*env); err != nil {
		return err
	}
	if *url == "" || *tokenURL == "" || *clientID == "" || *clientSecret == "" {
		printTenantSetHelp(ctx)
		return fmt.Errorf("required: --url, --token-url, --client-id, --client-secret")
	}
	if err := validate.URLWithSchemeHost("url")(*url); err != nil {
		return err
	}
	if err := validate.URLWithSchemeHost("tokenurl")(*tokenURL); err != nil {
		return err
	}

	ca := *createdAt
	if ca == "" {
		ca = time.Now().UTC().Format(time.RFC3339Nano)
	} else {
		if _, err := time.Parse(time.RFC3339Nano, ca); err != nil {
			if _, err2 := time.Parse(time.RFC3339, ca); err2 != nil {
				return fmt.Errorf("--created-at must be RFC3339 or RFC3339Nano: %w", err)
			}
		}
	}

	profileID, _, err := ctx.Stores.ResolveProfileID(ctx.Flags.ProfileID)
	if err != nil {
		return err
	}

	t := models.TenantServiceKey{
		OAuth: models.TenantOAuth{
			CreateDate:   ca,
			ClientID:     *clientID,
			ClientSecret: *clientSecret,
			TokenURL:     *tokenURL,
			URL:          *url,
		},
	}
	if err := t.ValidateRequired(); err != nil {
		return err
	}

	if err := ctx.Stores.Tenants.Write(profileID, *env, t); err != nil {
		return err
	}
	ctx.Logger.Info("tenant set", logging.F("profile_id", profileID), logging.F("env", *env))
	fmt.Fprintf(ctx.Stdout, "Tenant stored: %s/%s\n", profileID, *env)
	return nil
}

func tenantDelete(ctx *Context, argv []string) error {
	fs := flag.NewFlagSet("tenant delete", flag.ContinueOnError)
	fs.SetOutput(ctx.Stderr)
	env := fs.String("env", "dev", "Environment: dev|qas|prd")
	yes := fs.Bool("yes", false, "Confirm deletion")
	if err := fs.Parse(argv); err != nil {
		return wrapFlagError(err)
	}
	if err := validate.Env(*env); err != nil {
		return err
	}
	if !*yes {
		printTenantDeleteHelp(ctx)
		return fmt.Errorf("refusing to delete without --yes")
	}

	profileID, _, err := ctx.Stores.ResolveProfileID(ctx.Flags.ProfileID)
	if err != nil {
		return err
	}

	if err := ctx.Stores.Tenants.Delete(profileID, *env); err != nil {
		return err
	}
	ctx.Logger.Warn("tenant deleted", logging.F("profile_id", profileID), logging.F("env", *env))
	fmt.Fprintf(ctx.Stdout, "Tenant deleted: %s/%s\n", profileID, *env)
	return nil
}

func printTenantHelp(ctx *Context, path []string) {
	out := ctx.Stdout
	if len(path) == 0 {
		fmt.Fprintln(out, "Usage:")
		fmt.Fprintln(out, "  iflowkit tenant <command> [args]")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Commands:")
		fmt.Fprintln(out, "  import   Import a CPI service key JSON")
		fmt.Fprintln(out, "  show     Show tenant service key")
		fmt.Fprintln(out, "  set      Set tenant service key fields directly")
		fmt.Fprintln(out, "  delete   Delete tenant service key")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Try:")
		fmt.Fprintln(out, "  iflowkit help tenant import")
		return
	}
	switch path[0] {
	case "import":
		printTenantImportHelp(ctx)
	case "set":
		printTenantSetHelp(ctx)
	case "delete":
		printTenantDeleteHelp(ctx)
	default:
		fmt.Fprintln(out, "Unknown tenant command.")
	}
}

func printTenantImportHelp(ctx *Context) {
	fmt.Fprintln(ctx.Stdout, "Usage:")
	fmt.Fprintln(ctx.Stdout, "  iflowkit tenant import --file <service-key.json> [--env dev|qas|prd]")
}

func printTenantSetHelp(ctx *Context) {
	fmt.Fprintln(ctx.Stdout, "Usage:")
	fmt.Fprintln(ctx.Stdout, "  iflowkit tenant set --env dev|qas|prd --url <url> --token-url <tokenUrl> --client-id <id> --client-secret <secret> [--created-at <rfc3339>]")
}

func printTenantDeleteHelp(ctx *Context) {
	fmt.Fprintln(ctx.Stdout, "Usage:")
	fmt.Fprintln(ctx.Stdout, "  iflowkit tenant delete --env dev|qas|prd --yes")
}
