//go:build !windows

package gui

import (
	"testing"

	"github.com/go-gl/glfw/v3.3/glfw"
)

func TestButtonEventPositionOverridesStaleMoveAndAbsentCursorEnter(t *testing.T) {
	for _, valid := range []bool{true, false} {
		t.Run(map[bool]string{true: "stale-move", false: "cursor-left"}[valid], func(t *testing.T) {
			isolatePopupState(t)
			old := mouseButtonEventPosition
			t.Cleanup(func() { mouseButtonEventPosition = old })
			root := NewLabel("owner")
			root.SetSize(800, 700)
			native, win := testPopupWindow(root, WtForm, nil)
			win.enabled = true
			win.pointer = pointerEventState{x: 651.21484375, y: 682.25390625, valid: valid}
			button := NewButton1("menu", nil)
			button.SetBounds(227, 8, 38, 32)
			button.SetParent(root)
			clicks, eventReads := 0, 0
			button.Action().BindFunc0(func() { clicks++ })
			mouseButtonEventPosition = func(_ *glfw.Window, b glfw.MouseButton, a glfw.Action) (float64, float64, bool) {
				eventReads++
				if b != glfw.MouseButtonLeft || (a != glfw.Press && a != glfw.Release) {
					t.Fatalf("unexpected mouse event %v/%v", b, a)
				}
				return 246, 23, true
			}
			onMouseButton(native, glfw.MouseButtonLeft, glfw.Press, 0)
			onMouseButton(native, glfw.MouseButtonLeft, glfw.Release, 0)
			if clicks != 1 || eventReads != 2 || win.pointer.x != 246 || win.pointer.y != 23 {
				t.Fatalf("button event lost: clicks=%d reads=%d pointer=%+v", clicks, eventReads, win.pointer)
			}
		})
	}
}

func TestNativeMouseButtonPositionWithoutWindowIsUnavailable(t *testing.T) {
	if _, _, ok := nativeMouseButtonEventPosition(nil, glfw.MouseButtonLeft, glfw.Press); ok {
		t.Fatal("absent native window produced an event coordinate")
	}
}
