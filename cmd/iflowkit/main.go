package main

import (
	"os"

	"github.com/iflowkit/iflowkit-cli/internal/app"

	// Product modules (register via init).
	_ "github.com/iflowkit/iflowkit-cli/modules/sync"
)

func main() {
	if err := app.Run(os.Args[1:]); err != nil {
		os.Exit(1)
	}
}
