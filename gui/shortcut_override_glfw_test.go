//go:build !windows

package gui

import (
	"runtime"
	"testing"

	"github.com/go-gl/glfw/v3.3/glfw"
)

type shortcutOverrideTestWidget struct {
	Widget
	calls int
}

func (w *shortcutOverrideTestWidget) HandleShortcut(key int, mods uint8) bool {
	if key == 'Z' && mods == ModAction {
		w.calls++
		return true
	}
	return false
}
func TestFocusedShortcutOverridePrecedesHostButStaysWindowScoped(t *testing.T) {
	isolatePopupState(t)
	old := shortcutHandlers
	shortcutHandlers = make(map[shortcutKey]func())
	t.Cleanup(func() { shortcutHandlers = old })
	frame := NewFrame()
	native, win := testPopupWindow(frame, WtForm, nil)
	win.enabled = true
	child := new(shortcutOverrideTestWidget)
	child.Init(child)
	child.SetParent(frame)
	child.SetFocus()
	globalCalls := 0
	RegisterShortcut(ModAction, 'Z', func() { globalCalls++ })
	RegisterShortcut(ModAction, 'S', func() { globalCalls++ })
	mods := glfw.ModControl
	if runtime.GOOS == "darwin" {
		mods = glfw.ModSuper
	}
	onKey(native, glfw.KeyZ, 0, glfw.Press, mods)
	if child.calls != 1 || globalCalls != 0 {
		t.Fatal("global Undo stole the focused widget's action")
	}
	onKey(native, glfw.KeyS, 0, glfw.Press, mods)
	if globalCalls != 1 {
		t.Fatal("unclaimed action no longer reaches host")
	}
	other := NewFrame()
	_, otherWin := testPopupWindow(other, WtForm, nil)
	otherWin.enabled = true
	foreign := new(shortcutOverrideTestWidget)
	foreign.Init(foreign)
	foreign.SetParent(other)
	foreign.SetFocus()
	onKey(native, glfw.KeyZ, 0, glfw.Press, mods)
	if foreign.calls != 0 || globalCalls != 2 {
		t.Fatal("stale foreign focus intercepted the receiving window's shortcut")
	}
	child.SetFocus()
	child.SetEnabled(false)
	onKey(native, glfw.KeyZ, 0, glfw.Press, mods)
	if child.calls != 1 || globalCalls != 3 {
		t.Fatal("disabled widget intercepted shortcut")
	}
	child.SetEnabled(true)
	frame.SetEnabled(false)
	onKey(native, glfw.KeyZ, 0, glfw.Press, mods)
	if child.calls != 1 || globalCalls != 4 {
		t.Fatal("widget inside disabled ancestor intercepted shortcut")
	}
	frame.SetEnabled(true)
	onKey(native, glfw.KeyZ, 0, glfw.Press, mods)
	if child.calls != 2 || globalCalls != 4 {
		t.Fatal("re-enabled ancestor did not restore focused shortcut")
	}
}
