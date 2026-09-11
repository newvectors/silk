//go:build !windows

package gui

import (
	"testing"

	"github.com/go-gl/glfw/v3.3/glfw"
)

func TestWindowEscapeAfterFirstTabIntoDialogButton(t *testing.T) {
	isolatePopupState(t)
	dialog := NewDialog("dialog", nil)
	button := dialog.AddButton("Close", DialogOK)
	native, win := testPopupWindow(dialog, WtForm, nil)
	win.enabled, win.inModal = true, true
	dialog.SetFocus()
	onKey(native, glfw.KeyTab, 0, glfw.Press, 0)
	if focusWidget != button {
		t.Fatalf("first Tab target = %T, want the interactive button", focusWidget)
	}
	onKey(native, glfw.KeyEscape, 0, glfw.Press, 0)
	if !dialog.closed || win.inModal {
		t.Fatal("Escape did not cancel the dialog after Tab reached its button")
	}
}

func TestWindowEscapeFromExplicitDialogButtonBarFocus(t *testing.T) {
	isolatePopupState(t)
	dialog := NewDialog("dialog", nil)
	dialog.AddButton("Close", DialogOK)
	native, win := testPopupWindow(dialog, WtForm, nil)
	win.enabled, win.inModal = true, true
	// NoFocus excludes the bar from Tab traversal, but programmatic focus
	// must still exercise the window-level fix instead of Button.OnKeyDown.
	dialog.btnBox.SetFocus()
	onKey(native, glfw.KeyEscape, 0, glfw.Press, 0)
	if !dialog.closed || win.inModal {
		t.Fatal("the non-popup ButtonBox swallowed its owning dialog's Escape")
	}
}

func TestWindowEscapeCancelsDialogAfterTabIntoContent(t *testing.T) {
	isolatePopupState(t)
	dialog := NewDialog("dialog", nil)
	dialog.AddButton("OK", DialogOK)
	edit := NewEdit()
	dialog.SetContent(edit)
	native, win := testPopupWindow(dialog, WtForm, nil)
	win.enabled, win.inModal = true, true
	dialog.SetFocus()
	for i := 0; i < 5 && focusWidget != edit; i++ {
		onKey(native, glfw.KeyTab, 0, glfw.Press, 0)
	}
	if focusWidget != edit {
		t.Fatalf("Tab did not reach content: %T", focusWidget)
	}
	onKey(native, glfw.KeyEscape, 0, glfw.Press, 0)
	if !dialog.closed || win.inModal || dialog.result != DialogCancel {
		t.Fatal("window Escape was swallowed by the focused dialog content")
	}
}

func TestWindowEscapeIgnoresForeignFocusAndKeepsPopupPriority(t *testing.T) {
	isolatePopupState(t)
	dialog := NewDialog("dialog", nil)
	native, win := testPopupWindow(dialog, WtForm, nil)
	win.enabled, win.inModal = true, true
	foreign := NewDialog("foreign", nil)
	_, foreignWin := testPopupWindow(foreign, WtForm, nil)
	foreignWin.enabled, foreignWin.inModal = true, true
	foreign.AddButton("OK", DialogOK).SetFocus()
	onKey(native, glfw.KeyEscape, 0, glfw.Press, 0)
	if !dialog.closed || foreign.closed || foreignWin.inModal == false {
		t.Fatal("Escape followed stale global focus into another native window")
	}

	dialog.closed, win.inModal = false, true
	menu := NewPopupMenu()
	menu.SetParent(dialog)
	_, popupWin := testPopupWindow(menu, WtPopup, win)
	popupWin.enabled = true
	menu.setVisible(true)
	menu.AddButton1("command", nil).SetFocus()
	onKey(native, glfw.KeyEscape, 0, glfw.Press, 0)
	if menu.IsVisible() || dialog.closed || !win.inModal {
		t.Fatal("Escape must dismiss the owned popup without cancelling its dialog")
	}
	onKey(native, glfw.KeyEscape, 0, glfw.Repeat, 0)
	if dialog.closed {
		t.Fatal("the repeat of the popup Escape cancelled the underlying dialog")
	}
}

func TestWindowDialogEscapeDoesNotStealContentEnterOrPlainKeys(t *testing.T) {
	isolatePopupState(t)
	dialog := NewDialog("dialog", nil)
	dialog.AddButton("OK", DialogOK)
	edit := NewEdit()
	dialog.SetContent(edit)
	native, win := testPopupWindow(dialog, WtForm, nil)
	win.enabled, win.inModal = true, true
	edit.SetText("abcd")
	edit.SetSelection(4, 4)
	edit.SetFocus()
	onKey(native, glfw.KeyLeft, 0, glfw.Press, 0)
	onKey(native, glfw.KeyEnter, 0, glfw.Press, 0)
	if edit.CaretPos() != 3 || dialog.closed {
		t.Fatal("dialog cancellation routing stole normal content keys")
	}
}
