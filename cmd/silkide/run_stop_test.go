package main

import (
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/uk0/silk/core"
	"github.com/uk0/silk/ged"
	"github.com/uk0/silk/gui"
)

type debugLaunchResult struct {
	session *core.DebugSession
	err     error
}

type debugLifecycleHarness struct {
	launches chan chan debugLaunchResult
	posts    chan func()
	closed   chan *core.DebugSession
	resumed  chan *core.DebugSession
}

func newDebugLifecycleHarness(t *testing.T) *debugLifecycleHarness {
	t.Helper()
	savedHooks, savedAttempt, savedDebug := debuggerHooks, globalDebugAttempt, globalDebug
	savedPanel, savedTerminal, savedEditors := globalDebugPanel, globalTerminal, openEditors
	globalDebugAttempt, globalDebug, globalDebugPanel, globalTerminal = nil, nil, nil, nil
	openEditors = map[string]*gui.CodeEditor{}
	h := &debugLifecycleHarness{
		launches: make(chan chan debugLaunchResult, 8), posts: make(chan func(), 8),
		closed: make(chan *core.DebugSession, 8), resumed: make(chan *core.DebugSession, 8),
	}
	debuggerHooks = debugLifecycleHooks{
		launch: func(string, []string) (*core.DebugSession, error) {
			result := make(chan debugLaunchResult, 1)
			h.launches <- result
			r := <-result
			return r.session, r.err
		},
		close: func(s *core.DebugSession) { h.closed <- s },
		resume: func(s *core.DebugSession) (*core.StopState, error) {
			h.resumed <- s
			return nil, nil
		},
		post: func(fn func()) { h.posts <- fn },
	}
	t.Cleanup(func() {
		stopDebugger()
		debuggerHooks, globalDebugAttempt, globalDebug = savedHooks, savedAttempt, savedDebug
		globalDebugPanel, globalTerminal, openEditors = savedPanel, savedTerminal, savedEditors
	})
	return h
}

func awaitDebugValue[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for debugger lifecycle")
		var zero T
		return zero
	}
}

func TestRunStopCancelsLaunchAndDuplicateStart(t *testing.T) {
	h := newDebugLifecycleHarness(t)
	runProjectInDebugger(nil)
	request := awaitDebugValue(t, h.launches)
	runProjectInDebugger(nil)
	select {
	case <-h.launches:
		t.Fatal("duplicate start launched another debugger")
	default:
	}
	stopDebugger()
	stopDebugger() // idempotent while launch is still blocked
	sess := &core.DebugSession{}
	request <- debugLaunchResult{session: sess}
	if closed := awaitDebugValue(t, h.closed); closed != sess {
		t.Fatal("canceled launch leaked its session")
	}
	if globalDebug != nil || globalDebugAttempt != nil || len(h.resumed) != 0 || len(h.posts) != 0 {
		t.Fatal("canceled launch published or resumed")
	}
}

func TestRunStopCancelsQueuedPublication(t *testing.T) {
	h := newDebugLifecycleHarness(t)
	runProjectInDebugger(nil)
	request := awaitDebugValue(t, h.launches)
	sess := &core.DebugSession{}
	request <- debugLaunchResult{session: sess}
	publish := awaitDebugValue(t, h.posts)
	stopDebugger()
	publish() // this callback was already queued when Stop was clicked
	if awaitDebugValue(t, h.closed) != sess || len(h.resumed) != 0 || globalDebug != nil {
		t.Fatal("queued canceled launch became live")
	}
}

func TestRunStopOldLaunchCannotReplaceNewSession(t *testing.T) {
	h := newDebugLifecycleHarness(t)
	runProjectInDebugger(nil)
	oldRequest := awaitDebugValue(t, h.launches)
	stopDebugger()
	runProjectInDebugger(nil)
	newRequest := awaitDebugValue(t, h.launches)
	oldSession, newSession := &core.DebugSession{}, &core.DebugSession{}
	newRequest <- debugLaunchResult{session: newSession}
	awaitDebugValue(t, h.posts)()
	if awaitDebugValue(t, h.resumed) != newSession {
		t.Fatal("new debugger did not resume")
	}
	oldRequest <- debugLaunchResult{session: oldSession}
	if awaitDebugValue(t, h.closed) != oldSession || globalDebug != newSession {
		t.Fatal("late old debugger replaced or closed the new session")
	}
	stopDebugger()
	if awaitDebugValue(t, h.closed) != newSession {
		t.Fatal("new debugger was not independently stoppable")
	}
}

func TestRunStopOldFailureCannotClearNewAttempt(t *testing.T) {
	h := newDebugLifecycleHarness(t)
	runProjectInDebugger(nil)
	oldRequest := awaitDebugValue(t, h.launches)
	stopDebugger()
	runProjectInDebugger(nil)
	newRequest := awaitDebugValue(t, h.launches)
	current := globalDebugAttempt
	oldRequest <- debugLaunchResult{err: errors.New("old launch failed")}
	awaitDebugValue(t, h.posts)()
	if globalDebugAttempt != current {
		t.Fatal("old failure cleared the new launch")
	}
	newRequest <- debugLaunchResult{err: errors.New("new launch failed")}
	awaitDebugValue(t, h.posts)()
	if globalDebugAttempt != nil {
		t.Fatal("current failure left a stuck starting state")
	}
}

func TestRunStopOldExitDoesNotClearNewPanel(t *testing.T) {
	h := newDebugLifecycleHarness(t)
	oldSession, current := &core.DebugSession{}, &core.DebugSession{}
	globalDebug = current
	globalDebugPanel = ged.NewDebugPanel()
	globalDebugPanel.SetVariables([]core.Variable{{Name: "current", Value: "42"}})
	finishDebugSession(oldSession)
	if awaitDebugValue(t, h.closed) != oldSession || globalDebug != current || len(globalDebugPanel.Variables()) != 1 {
		t.Fatal("stale exit cleared the current session/panel")
	}
}

func TestRunStopGenericStopsTerminalAndDebugger(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real POSIX runner smoke; Windows PID-tree path is platform-specific")
	}
	h := newDebugLifecycleHarness(t)
	globalTerminal = ged.NewTerminalPanel()
	terminal := globalTerminal
	terminal.Run("sleep 30")
	t.Cleanup(func() { terminal.StopRunning() })
	if !terminal.CommandRunning() {
		t.Fatal("runner did not start")
	}
	current := &core.DebugSession{}
	globalDebug = current
	stopProjectExecution()
	if awaitDebugValue(t, h.closed) != current || globalDebug != nil {
		t.Fatal("generic Stop did not close debugger")
	}
	deadline := time.Now().Add(4 * time.Second)
	for terminal.CommandRunning() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if terminal.CommandRunning() {
		t.Fatal("generic Stop did not terminate the Run process")
	}
	stopProjectExecution() // idle is harmless and produces feedback
}
