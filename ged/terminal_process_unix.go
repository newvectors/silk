//go:build !windows

package ged

import (
	"os"
	"os/exec"
	"syscall"
)

func configureTerminalProcess(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// The PID is our unreaped runner shell, also the leader of its own group.
// Killing -PID includes go run/build descendants without touching the IDE,
// other terminal sessions, or unrelated processes with the same executable.
func killTerminalProcess(p *os.Process) error {
	if p == nil {
		return os.ErrProcessDone
	}
	if err := syscall.Kill(-p.Pid, syscall.SIGKILL); err == nil {
		return nil
	}
	return p.Kill()
}
