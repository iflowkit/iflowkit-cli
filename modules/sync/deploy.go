package sync

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/app"
	"github.com/iflowkit/iflowkit-cli/internal/common/cpix"
	"github.com/iflowkit/iflowkit-cli/internal/logging"
	"github.com/iflowkit/iflowkit-cli/internal/validate"
)

// runSyncDeploy handles `iflowkit sync deploy ...`.
func runSyncDeploy(ctx *app.Context, args []string) error {
	if len(args) == 0 {
		return runSyncDeployStatus(ctx, nil)
	}
	switch args[0] {
	case "status":
		return runSyncDeployStatus(ctx, args[1:])
	default:
		fmt.Fprintln(ctx.Stdout, "Usage: iflowkit sync deploy status [--env dev|qas|prd] [--transport <transportId>]")
		return fmt.Errorf("unknown sync deploy command: %s", args[0])
	}
}

// runSyncDeployStatus lists deployment status in CPI for the objects in a given transport record.
// Output is intentionally minimal: kind, name (id), status, deployed date.
func runSyncDeployStatus(ctx *app.Context, args []string) error {
	fs := flag.NewFlagSet("iflowkit sync deploy status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var transportID string
	var env string
	fs.StringVar(&transportID, "transport", "", "Transport ID (defaults to last transport)")
	fs.StringVar(&env, "env", "dev", "Tenant environment (dev|qas|prd)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	cwd, _ := os.Getwd()
	repoRoot, err := findSyncRepoRoot(cwd)
	if err != nil {
		return err
	}

	env = strings.ToLower(strings.TrimSpace(env))
	if err := validate.Env(env); err != nil {
		return err
	}
	store, err := NewTransportStore(repoRoot, env)
	if err != nil {
		return err
	}

	// Select record.
	var rec TransportRecord
	if strings.TrimSpace(transportID) != "" {
		transportID = strings.TrimSpace(transportID)
		r, err := store.LoadRecord(transportID)
		if err != nil {
			return fmt.Errorf("cannot read transport record: %w", err)
		}
		rec = r
	} else {
		r, _, ok, err := store.LoadLatestTransportRecord()
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(ctx.Stdout, "No transport records found.")
			return nil
		}
		rec = *r
	}

	if len(rec.Objects) == 0 {
		fmt.Fprintln(ctx.Stdout, "No objects recorded for this transport.")
		return nil
	}

	// Resolve profile + target tenant.
	profileID, source, err := ctx.Stores.ResolveProfileID(ctx.Flags.ProfileID)
	if err != nil {
		return err
	}
	_, err = ctx.Stores.Profiles.Read(profileID)
	if err != nil {
		return err
	}
	ctx.Logger.Info("resolved profile", logging.F("profile", profileID), logging.F("source", source))

	tenant, err := ctx.Stores.Tenants.Read(profileID, env)
	if err != nil {
		return fmt.Errorf("%s tenant not found for profile %q: %w", strings.ToUpper(env), profileID, err)
	}
	client := cpix.NewClient(tenant, ctx.Logger)

	// Sort for stable output.
	objs := append([]SyncObject(nil), rec.Objects...)
	sort.Slice(objs, func(i, j int) bool {
		if objs[i].Kind == objs[j].Kind {
			return objs[i].ID < objs[j].ID
		}
		return objs[i].Kind < objs[j].Kind
	})

	fmt.Fprintf(ctx.Stdout, "%-14s %-48s %-14s %s\n", "KIND", "NAME", "STATUS", "DEPLOYED_AT")
	for _, o := range objs {
		rt, found, err := client.GetIntegrationRuntimeArtifact(context.Background(), o.ID)
		st := rt.Status
		deployedAt := rt.DeployedOn
		if err != nil {
			ctx.Logger.Warn("deployment status check failed", logging.F("id", o.ID), logging.F("error", err.Error()))
			st = "ERROR"
			deployedAt = ""
			found = true
		}
		if !found {
			st = "NOT_FOUND"
			deployedAt = ""
		}
		fmt.Fprintf(ctx.Stdout, "%-14s %-48s %-14s %s\n", o.Kind, o.ID, st, deployedAt)
	}
	return nil
}
