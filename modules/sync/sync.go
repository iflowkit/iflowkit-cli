package sync

import (
	"fmt"

	"github.com/iflowkit/iflowkit-cli/internal/app"
)

func init() {
	app.RegisterCommand(app.ExternalCommand{
		Name: "sync",
		Help: syncHelp,
		Run:  runSync,
	})
}

func runSync(ctx *app.Context, args []string) error {
	if len(args) == 0 {
		syncHelp(ctx, nil)
		return nil
	}
	switch args[0] {
	case "init":
		return runSyncInit(ctx, args[1:])
	case "pull":
		return runSyncPull(ctx, args[1:])
	case "push":
		return runSyncPush(ctx, args[1:])
	case "deploy":
		return runSyncDeploy(ctx, args[1:])
	case "deliver":
		return runSyncDeliver(ctx, args[1:])
	case "compare":
		return runSyncCompare(ctx, args[1:])
	default:
		syncHelp(ctx, args)
		return fmt.Errorf("unknown sync command: %s", args[0])
	}
}
