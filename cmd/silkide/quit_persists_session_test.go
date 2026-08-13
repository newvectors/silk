package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"

	"github.com/uk0/silk/gui"
)

// Cmd+Q used to end the event loop directly, and the writes that let the next
// launch come up where the user left off — the window geometry and the open
// file set — hang off the frame's closed callback. So a keyboard quit
// persisted nothing at all: the following launch restored the session BEFORE
// last, tabs the user had already moved on from, at a geometry they had
// already changed. The title bar's red button was always right, because it
// goes through Frame.Close.
//
// These press the real Cmd+Q binding out of the shortcut registry against a
// frame wired the way main() wires it, and read the answer back through a
// fresh preferences instance — the on-disk store is the only thing the next
// launch gets to see.

const (
	// The session and geometry an earlier run left behind. Every store below
	// already holds these before the quit under test, so an implementation
	// that writes nothing cannot pass by reading back a default.
	lastLaunchPath   = "/from/last/launch.go"
	lastLaunchWidth  = 1024
	lastLaunchHeight = 640
	lastLaunchX      = 17
	lastLaunchY      = 23
)

// seedLastLaunch points the settings store at a private directory and fills it
// with the previous run's state. The returned preferences is a second instance,
// like the one main() builds at startup, so everything the test asserts has
// been through the file.
func seedLastLaunch(t *testing.T) *preferences {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp) // Linux fallback path
	t.Setenv("HOME", tmp)            // macOS Application Support path

	seed := newPreferences()
	seed.SetOpenSession([]string{lastLaunchPath})
	seed.SetWindowSize(lastLaunchWidth, lastLaunchHeight)
	seed.SetWindowPos(lastLaunchX, lastLaunchY)
	return newPreferences()
}

// wireQuitFrame builds a frame and hooks it up the way main() does: the closed
// callback persists the session and then ends the loop, and closeApp is the
// door Cmd+Q has to come through to reach it. The counters say how many times
// the frame actually tore down — a quit that calls persistSessionState itself
// leaves them at zero.
func wireQuitFrame(t *testing.T, prefs *preferences) (closing, closed *int) {
	t.Helper()
	var closingN, closedN int
	frame := gui.NewFrame()
	frame.SetClosingCallback(func(*gui.Frame) { closingN++ })
	frame.SetClosedCallback(func(*gui.Frame) {
		closedN++
		persistSessionState(frame, nil, prefs)
		quitApp()
	})
	saved := closeApp
	closeApp = func() { frame.Close() }
	t.Cleanup(func() { closeApp = saved })
	return &closingN, &closedN
}

// TestCmdQPersistsThisSessionNotLastLaunchs: the quit has to leave THIS
// session's open file on disk. Reading back the pre-seeded path means the
// keyboard quit skipped the frame teardown entirely and the next launch
// reopens a session the user is two runs past.
func TestCmdQPersistsThisSessionNotLastLaunchs(t *testing.T) {
	prefs := seedLastLaunch(t)

	dir := t.TempDir()
	path := writeFile(t, dir, "open-now.go", quitOriginal)
	tabs, _, _ := openEditedTab(t, path, quitOriginal)

	closing, closed := wireQuitFrame(t, prefs)
	quits := recordQuit(t)

	cmdQ(t, tabs)()

	if got, want := newPreferences().OpenSession(), []string{path}; !reflect.DeepEqual(got, want) {
		t.Errorf("the store holds session %v after Cmd+Q, want this session's %v", got, want)
	}
	// The test frame never attached a window — creating one off the main
	// thread takes AppKit down with it — so persistSessionState takes its
	// no-window branch and last launch's geometry is what correctly survives.
	// What this pins is that the branch exists: without the guard the nil
	// Window is dereferenced on the way out of every quit, and a persist that
	// reached for bounds it does not have would stamp 0x0 over the real
	// geometry the user set.
	if w, h := newPreferences().WindowSize(); w != lastLaunchWidth || h != lastLaunchHeight {
		t.Errorf("window size = (%d, %d), want the geometry the close captured (%d, %d)",
			w, h, lastLaunchWidth, lastLaunchHeight)
	}
	if x, y := newPreferences().WindowPos(); x != lastLaunchX || y != lastLaunchY {
		t.Errorf("window pos = (%d, %d), want the position the close captured (%d, %d)",
			x, y, lastLaunchX, lastLaunchY)
	}
	// Through Frame.Close, not around it: the closed callback is where the
	// writes above live, and it is also what closes the docks and stops the
	// timers. Counting the callbacks is how a quit that merely happens to
	// persist the right thing is told apart from a quit that shuts down.
	if *closing != 1 {
		t.Errorf("Cmd+Q ran the frame's closing callback %d times, want 1", *closing)
	}
	if *closed != 1 {
		t.Errorf("Cmd+Q ran the frame's closed callback %d times, want 1", *closed)
	}
	if *quits != 1 {
		t.Errorf("Cmd+Q ended the event loop %d times, want 1", *quits)
	}
}

// TestCmdQCancelledPersistsNothingAndClosesNothing: routing the quit through
// Frame.Close must not cost the guard. Frame.Close runs the teardown
// unconditionally — it never asks the can-close callback — so Cancel has to
// stop the gesture before it, leaving the frame standing and last launch's
// session exactly where it was.
func TestCmdQCancelledPersistsNothingAndClosesNothing(t *testing.T) {
	prefs := seedLastLaunch(t)

	dir := t.TempDir()
	path := writeFile(t, dir, "unsaved.go", quitOriginal)
	tabs, ed, _ := openEditedTab(t, path, quitEditedA)

	closing, closed := wireQuitFrame(t, prefs)
	answerQuitDirty(t, gui.DialogCancel)
	quits := recordQuit(t)

	cmdQ(t, tabs)()

	if *closing != 0 {
		t.Errorf("a cancelled Cmd+Q ran the frame's closing callback %d times, want 0", *closing)
	}
	if *closed != 0 {
		t.Errorf("a cancelled Cmd+Q ran the frame's closed callback %d times, want 0", *closed)
	}
	if *quits != 0 {
		t.Errorf("a cancelled Cmd+Q ended the event loop %d times, want 0", *quits)
	}
	if got, want := newPreferences().OpenSession(), []string{lastLaunchPath}; !reflect.DeepEqual(got, want) {
		t.Errorf("a cancelled Cmd+Q left session %v on disk, want last launch's %v untouched", got, want)
	}
	if got := ed.Text(); got != quitEditedA {
		t.Errorf("buffer = %q; want the unsaved edit %q", got, quitEditedA)
	}
}

// TestMainSendsCloseAppThroughFrameClose: the two tests above supply their own
// closeApp, so the running IDE's is unchecked by them — main could hand Cmd+Q a
// closeApp that persists the session itself and skips the frame, and the whole
// package would stay green while every quit left the docks open and the timers
// running. main cannot be called from a test (it opens a window and enters the
// event loop), so this reads the assignment out of the syntax tree the way
// TestMainInstallsTheQuitGuard reads the guard.
func TestMainSendsCloseAppThroughFrameClose(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("cannot parse main.go: %v", err)
	}
	var mainFn *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "main" && fn.Recv == nil {
			mainFn = fn
		}
	}
	if mainFn == nil {
		t.Fatal("main.go has no func main")
	}
	frame := boundBy(mainFn, "gui.NewFrameWindow", 0)
	if frame == "" {
		t.Fatal("main no longer builds its frame with gui.NewFrameWindow; update this test to match")
	}

	// Assigned as a plain statement of main's own body and ahead of the loop:
	// core.EventLoop returns only when the app is already quitting, so a
	// closeApp set after it is a closeApp Cmd+Q never sees.
	var lit *ast.FuncLit
	assign, loop := -1, -1
	for i, stmt := range mainFn.Body.List {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
				continue
			}
			if id, ok := s.Lhs[0].(*ast.Ident); ok && id.Name == "closeApp" {
				assign = i
				lit, _ = s.Rhs[0].(*ast.FuncLit)
			}
		case *ast.ExprStmt:
			call, ok := s.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			if fn, ok := call.Fun.(*ast.SelectorExpr); ok && fn.Sel.Name == "EventLoop" {
				loop = i
			}
		}
	}
	if assign < 0 {
		t.Fatal("main never sets closeApp; Cmd+Q falls back to ending the loop and the session is never written")
	}
	if lit == nil {
		t.Fatal("main sets closeApp to something other than a function literal; update this test to match")
	}
	if loop < 0 {
		t.Fatal("main no longer runs core.EventLoop at its top level; update this test to match")
	}
	if assign > loop {
		t.Error("closeApp is set after core.EventLoop, which returns only when the app is already quitting: Cmd+Q never sees it")
	}

	var closes, persists int
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			if id, ok := fn.X.(*ast.Ident); ok && id.Name == frame && fn.Sel.Name == "Close" {
				closes++
			}
		case *ast.Ident:
			if fn.Name == "persistSessionState" {
				persists++
			}
		}
		return true
	})
	if closes != 1 {
		t.Errorf("closeApp calls %s.Close %d times, want 1: the keyboard quit has to run the same teardown the close button does", frame, closes)
	}
	if persists != 0 {
		t.Errorf("closeApp persists the session itself (%d calls); that is the closed callback's job, and doing it here means the quit skips the rest of the teardown", persists)
	}
}
