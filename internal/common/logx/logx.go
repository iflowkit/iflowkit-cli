package logx

import "github.com/iflowkit/iflowkit-cli/internal/logging"

// Re-export the logger types so packages can depend on a single, stable import path.
type Logger = logging.Logger
type Options = logging.Options
type Field = logging.Field
type Level = logging.Level

var (
	F          = logging.F
	New        = logging.New
	ParseLevel = logging.ParseLevel
)
