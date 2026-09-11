//go:build windows

package ged

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

func configureTerminalProcess(c *exec.Cmd) {}

// Use a specific child PID, never image-name/global matching. /T terminates
// its descendant tree; /F prevents a detached GUI child ignoring the stop.
// https://learn.microsoft.com/windows-server/administration/windows-commands/taskkill
func killTerminalProcess(p *os.Process) error {
	if p == nil {
		return os.ErrProcessDone
	}
	tool := "taskkill.exe"
	if root := os.Getenv("SystemRoot"); root != "" {
		tool = filepath.Join(root, "System32", tool)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, tool, "/PID", strconv.Itoa(p.Pid), "/T", "/F").Run(); err != nil {
		if fallback := p.Kill(); fallback != nil {
			return fmt.Errorf("taskkill: %v; direct child: %w", err, fallback)
		}
		return fmt.Errorf("taskkill failed (%v); direct child stopped, descendant cleanup not confirmed", err)
	}
	return nil
}
