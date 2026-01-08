package app

import (
	"sort"
	"sync"
)

// ExternalCommand is a top-level command that can be registered by product modules
// (for example under /modules/*) without changing the core CLI.
type ExternalCommand struct {
	Name string

	// Help prints usage for the command path that starts after the command name.
	// Example: for `iflowkit help sync init`, the path passed to Help is []string{"init"}.
	Help func(ctx *Context, path []string)

	// Run executes the command. The args passed to Run are the arguments after the command name.
	// Example: for `iflowkit sync init --id X`, args passed to Run are []string{"init", "--id", "X"}.
	Run func(ctx *Context, args []string) error
}

var (
	externalMu   sync.RWMutex
	externalCmds = map[string]ExternalCommand{}
)

// RegisterCommand registers a top-level command. Intended to be called from init() functions
// of product modules (via blank imports in main).
func RegisterCommand(cmd ExternalCommand) {
	if cmd.Name == "" || cmd.Run == nil {
		return
	}
	externalMu.Lock()
	defer externalMu.Unlock()
	externalCmds[cmd.Name] = cmd
}

func getExternalCommand(name string) (ExternalCommand, bool) {
	externalMu.RLock()
	defer externalMu.RUnlock()
	c, ok := externalCmds[name]
	return c, ok
}

func listExternalCommandNames() []string {
	externalMu.RLock()
	defer externalMu.RUnlock()
	names := make([]string, 0, len(externalCmds))
	for k := range externalCmds {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
