package gitx

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/common/logx"
)

// Run executes a git command in the given directory.
func Run(lg *logx.Logger, dir string, args ...string) error {
	if lg != nil {
		lg.Info("git", logx.F("args", strings.Join(args, " ")))
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	if outStr != "" && lg != nil {
		lg.Debug("git output", logx.F("output", outStr))
	}
	if err != nil {
		return fmt.Errorf("git %s failed: %s", strings.Join(args, " "), outStr)
	}
	return nil
}

// Output executes a git command in the given directory and returns the trimmed combined output.
func Output(lg *logx.Logger, dir string, args ...string) (string, error) {
	if lg != nil {
		lg.Info("git", logx.F("args", strings.Join(args, " ")))
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	if outStr != "" && lg != nil {
		lg.Debug("git output", logx.F("output", outStr))
	}
	if err != nil {
		return outStr, fmt.Errorf("git %s failed: %s", strings.Join(args, " "), outStr)
	}
	return outStr, nil
}

func LookPath() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git executable not found in PATH")
	}
	return nil
}
