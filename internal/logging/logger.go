package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	Trace Level = iota
	Debug
	Info
	Warn
	Error
)

func (l Level) String() string {
	switch l {
	case Trace:
		return "TRACE"
	case Debug:
		return "DEBUG"
	case Info:
		return "INFO"
	case Warn:
		return "WARN"
	case Error:
		return "ERROR"
	default:
		return "INFO"
	}
}

var levelOrder = map[string]int{
	"TRACE": 0,
	"DEBUG": 1,
	"INFO":  2,
	"WARN":  3,
	"ERROR": 4,
}

var levelColors = map[string]string{
	"TRACE": "\033[36m",
	"DEBUG": "\033[35m",
	"INFO":  "\033[34m",
	"WARN":  "\033[33m",
	"ERROR": "\033[31m",
}

const colorReset = "\033[0m"

type Field struct {
	Key string
	Val any
}

func F(key string, val any) Field { return Field{Key: key, Val: val} }

type Options struct {
	LogsDir string
	Level   string // trace|debug|info|warn|error
	Format  string // text|json
	Stdout  io.Writer
	Stderr  io.Writer
	Cmdline []string
}

type Logger struct {
	mu      sync.Mutex
	min     Level
	format  string
	console io.Writer
	file    *os.File
}

func New(opts Options) (*Logger, error) {
	min, err := ParseLevel(opts.Level)
	if err != nil {
		return nil, err
	}
	format := strings.ToLower(strings.TrimSpace(opts.Format))
	if format != "text" && format != "json" {
		return nil, fmt.Errorf("invalid --log-format %q (allowed: text|json)", opts.Format)
	}

	if err := os.MkdirAll(opts.LogsDir, 0o755); err != nil {
		return nil, err
	}
	date := time.Now().Format("2006-01-02")
	logPath := filepath.Join(opts.LogsDir, fmt.Sprintf("%s.log", date))
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	l := &Logger{
		min:     min,
		format:  format,
		console: opts.Stdout,
		file:    f,
	}

	l.Info("logger initialized", F("argv", opts.Cmdline), F("log_file", logPath), F("format", format), F("level", min.String()))
	return l, nil
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

func ParseLevel(s string) (Level, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "trace":
		return Trace, nil
	case "debug":
		return Debug, nil
	case "info":
		return Info, nil
	case "warn", "warning":
		return Warn, nil
	case "error":
		return Error, nil
	default:
		return Info, fmt.Errorf("invalid --log-level %q (allowed: trace|debug|info|warn|error)", s)
	}
}

func (l *Logger) Enabled(level Level) bool { return level >= l.min }

func (l *Logger) Trace(msg string, fields ...Field) { l.log(Trace, msg, fields...) }
func (l *Logger) Debug(msg string, fields ...Field) { l.log(Debug, msg, fields...) }
func (l *Logger) Info(msg string, fields ...Field)  { l.log(Info, msg, fields...) }
func (l *Logger) Warn(msg string, fields ...Field)  { l.log(Warn, msg, fields...) }
func (l *Logger) Error(msg string, fields ...Field) { l.log(Error, msg, fields...) }

func (l *Logger) log(level Level, msg string, fields ...Field) {
	if !l.Enabled(level) {
		return
	}
	rec := map[string]any{
		"ts":    time.Now().Format(time.RFC3339Nano),
		"level": level.String(),
		"msg":   msg,
	}
	for _, f := range fields {
		if f.Key == "" {
			continue
		}
		rec[f.Key] = f.Val
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.format == "json" {
		b, _ := json.Marshal(rec)
		l.writeLine(string(b))
		return
	}

	// text format
	ts := rec["ts"].(string)
	lvl := rec["level"].(string)

	// Stable field ordering for readability.
	keys := make([]string, 0, len(rec))
	for k := range rec {
		if k == "ts" || k == "level" || k == "msg" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, rec[k]))
	}
	plain := fmt.Sprintf("%s %s %s", ts, padLevel(lvl), msg)
	if len(pairs) > 0 {
		plain = plain + " " + strings.Join(pairs, " ")
	}

	// Console colorization only for text logs.
	coloredLevel := colorizeLevel(lvl)
	consoleLine := fmt.Sprintf("%s %s %s", ts, coloredLevel, msg)
	if len(pairs) > 0 {
		consoleLine = consoleLine + " " + strings.Join(pairs, " ")
	}

	// Write to file without color, and to console with color.
	l.writeLineBoth(plain, consoleLine)
}

func padLevel(lvl string) string {
	if len(lvl) >= 5 {
		return lvl
	}
	return lvl + strings.Repeat(" ", 5-len(lvl))
}

func colorizeLevel(lvl string) string {
	col := levelColors[lvl]
	if col == "" {
		return padLevel(lvl)
	}
	return col + padLevel(lvl) + colorReset
}

func (l *Logger) writeLine(line string) {
	_, _ = io.WriteString(l.file, line+"\n")
	_, _ = io.WriteString(l.console, line+"\n")
}

func (l *Logger) writeLineBoth(fileLine, consoleLine string) {
	_, _ = io.WriteString(l.file, fileLine+"\n")
	_, _ = io.WriteString(l.console, consoleLine+"\n")
}
