//go:build !windows

package ged

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func terminalTestProcessRunning(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	// Container PID 1 may not immediately reap an orphan. A zombie is dead,
	// not an escaped child executing after Stop.
	state, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	return err == nil && !strings.HasPrefix(strings.TrimSpace(string(state)), "Z")
}

func TestTerminalStopBeforeWorkerDoesNotStartProcess(t *testing.T) {
	p := NewTerminalPanel()
	path := filepath.Join(t.TempDir(), "should-not-exist")
	quotedPath := "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
	cancel := make(chan struct{})
	close(cancel)
	p.running = true
	p.runWorker("echo started > "+quotedPath, t.TempDir(), nil, cancel)
	if p.CommandRunning() {
		t.Fatal("canceled worker retained the running slot")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("canceled worker spawned a process: %v", err)
	}
}

func TestTerminalStopRunningKillsOwnedDescendantsOnly(t *testing.T) {
	other := exec.Command("sleep", "30")
	if err := other.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = other.Process.Kill(); _ = other.Wait() })
	p := NewTerminalPanel()
	t.Cleanup(func() { p.StopRunning() })
	path := filepath.Join(t.TempDir(), "child.pid")
	quotedPath := "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
	p.Run("sleep 30 & child=$!; echo \"$child\" > " + quotedPath + "; wait \"$child\"")
	var child int
	deadline := time.Now().Add(3 * time.Second)
	for child == 0 && time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil {
			child, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		}
		time.Sleep(10 * time.Millisecond)
	}
	if child == 0 {
		t.Fatal("runner never started its child")
	}
	t.Cleanup(func() {
		if terminalTestProcessRunning(child) {
			_ = syscall.Kill(child, syscall.SIGKILL)
		}
	})
	if !terminalTestProcessRunning(child) || !p.StopRunning() || p.StopRunning() {
		t.Fatal("runner child missing or cancellation not idempotent")
	}
	deadline = time.Now().Add(4 * time.Second)
	for (p.CommandRunning() || terminalTestProcessRunning(child)) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if p.CommandRunning() || terminalTestProcessRunning(child) {
		t.Fatal("Stop left a descendant or blocked output pipes alive")
	}
	if !terminalTestProcessRunning(other.Process.Pid) {
		t.Fatal("Stop killed an unrelated process with the same executable name")
	}
}
