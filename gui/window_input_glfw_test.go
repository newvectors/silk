//go:build !windows

package gui

import (
	"github.com/go-gl/glfw/v3.3/glfw"
	"runtime"
	"testing"
)

func TestPointerEventPositionUsesDeliveredCoordinates(t *testing.T) {
	p := pointerEventState{x: 80, y: 170, valid: true}
	x, y := p.position(func() (float64, float64) { t.Fatal("polled unrelated OS pointer"); return 500, 1400 })
	if x != 80 || y != 170 {
		t.Fatalf("position = %v,%v", x, y)
	}
	p.valid = false
	x, y = p.position(func() (float64, float64) { return 20, 30 })
	if x != 20 || y != 30 {
		t.Fatalf("fallback = %v,%v", x, y)
	}
}

func TestModifierKeyStateUsesEventSnapshot(t *testing.T) {
	mods := glfw.ModSuper | glfw.ModShift
	previous := eventModifiers
	eventModifiers = &mods
	defer func() { eventModifiers = previous }()
	for _, tt := range []struct {
		key  int
		held bool
	}{{KeyLWin, true}, {KeyRWin, true}, {KeyShift, true}, {KeyCtrl, false}, {KeyMenu, false}} {
		if got := IsKeyDown(tt.key); got != tt.held {
			t.Errorf("key %d: %v", tt.key, got)
		}
	}
	if _, modifier := modifierKeyState('S', mods); modifier {
		t.Fatal("ordinary key treated as modifier")
	}
}

func TestCodeEditorSelectAllAcceptsPlatformActionModifier(t *testing.T) {
	mods := glfw.ModControl
	if runtime.GOOS == "darwin" {
		mods = glfw.ModSuper
	}
	previous := eventModifiers
	eventModifiers = &mods
	defer func() { eventModifiers = previous }()
	ed := NewCodeEditor()
	ed.SetText("before\nsecond line")
	ed.OnKeyDown('A', false)
	// Normal text input carries no action modifier.
	eventModifiers = nil
	ed.OnTextInput("after")
	if ed.Text() != "after" {
		t.Fatalf("select-all did not select the complete buffer: %q", ed.Text())
	}
}

func TestNativeDestructionIsDeferredAndDrainedOnce(t *testing.T) {
	previous := pendingNativeDestruction
	pendingNativeDestruction = nil
	defer func() { pendingNativeDestruction = previous }()
	calls := 0
	pendingNativeDestruction = append(pendingNativeDestruction, func() { calls++ })
	if calls != 0 {
		t.Fatal("destroyed during callback")
	}
	flushNativeDestruction()
	flushNativeDestruction()
	if calls != 1 {
		t.Fatalf("destroy count %d", calls)
	}
}

func TestDragLoopStillRecordsDeliveredPointerMoves(t *testing.T) {
	previousMap, previousPointer, previousDrag := winMap, pointerWindow, dndActive
	defer func() { winMap, pointerWindow, dndActive = previousMap, previousPointer, previousDrag }()
	native := new(glfw.Window) // No native calls should occur during suspended dispatch.
	win := &Window{widget: NewLabel("target")}
	winMap = map[*glfw.Window]*Window{native: win}
	dndActive = true
	onCursorPos(native, 211, 144)
	if pointerWindow != win || !win.pointer.valid || win.pointer.x != 211 || win.pointer.y != 144 {
		t.Fatalf("drag lost event coordinates: %+v", win.pointer)
	}
}

func TestClosingPreviewLeavesMainFormAlive(t *testing.T) {
	previous := winMap
	defer func() { winMap = previous }()
	main, preview := new(glfw.Window), new(glfw.Window)
	winMap = map[*glfw.Window]*Window{main: {wt: WtForm}, preview: {wt: WtForm}}
	delete(winMap, preview)
	if !hasOpenFormWindow() {
		t.Fatal("closing preview would quit over the main form")
	}
	delete(winMap, main)
	if hasOpenFormWindow() {
		t.Fatal("last form close would keep application alive")
	}
	winMap[preview] = &Window{wt: WtPopup}
	if hasOpenFormWindow() {
		t.Fatal("popup kept the application alive")
	}
}

func TestCodeEditorFindUsesPlatformActionModifier(t *testing.T) {
	mods := glfw.ModControl
	if runtime.GOOS == "darwin" {
		mods = glfw.ModSuper
	}
	previous := eventModifiers
	eventModifiers = &mods
	defer func() { eventModifiers = previous }()
	ed := NewCodeEditor()
	ed.SetText("find me")
	ed.OnKeyDown('F', false)
	if !ed.findActive {
		t.Fatal("find did not open")
	}
	ed.OnKeyDown('H', false)
	if !ed.replaceVisible {
		t.Fatal("replace did not open")
	}
	ed.OnKeyDown('F', false)
	if ed.findActive {
		t.Fatal("find did not close")
	}
}
