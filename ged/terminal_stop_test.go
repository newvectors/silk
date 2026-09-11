package ged

import "testing"

func TestTerminalStopRunningIdempotentAndKeepsPTY(t *testing.T) {
	p := NewTerminalPanel()
	p.session = &terminalSession{} // user-owned PTY must not be touched
	userSession := p.session
	if p.StopRunning() || p.CommandRunning() || p.session != userSession {
		t.Fatal("idle Stop affected a user terminal session")
	}
	p.mu.Lock()
	p.running = true
	p.cancel = make(chan struct{})
	cancel := p.cancel
	p.mu.Unlock()
	if !p.StopRunning() || p.StopRunning() {
		t.Fatal("cancellation was not exactly once")
	}
	select {
	case <-cancel:
	default:
		t.Fatal("runner was not canceled")
	}
	if !p.CommandRunning() || p.session != userSession {
		t.Fatal("Stop released worker slot early or changed the user PTY")
	}
}
