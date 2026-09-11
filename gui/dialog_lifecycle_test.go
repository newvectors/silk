//go:build !windows

package gui

import "testing"

// These fixtures represent an attached, visible native form without creating
// a GLFW window. No GL resources are allocated, so destroy exercises the same
// widget-side teardown as a modal loop without an event pump or a display.
func attachHeadlessDialogWindow(widget IWidget) *Window {
	win := &Window{widget: widget, wt: WtForm, enabled: true}
	b := widget.NakedWidget()
	b.win = win
	b.wt = WtForm
	b.hidden = false
	return win
}

func TestModalWidgetDestroyHidesAndReleasesWindow(t *testing.T) {
	cases := []struct {
		name string
		make func() IWidget
	}{
		{"Dialog", func() IWidget { return NewDialog("settings", nil) }},
		{"InputBox", func() IWidget { return NewInputBox() }},
		{"MessageBox", func() IWidget { return NewMessageBox() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			owner := NewForm()
			sibling := NewLabel("ordinary owner content")
			sibling.SetParent(owner)
			dialog := tc.make()
			dialog.SetParent(owner)
			ownerWindow := attachHeadlessDialogWindow(owner)
			ownerVisible, siblingVisible := owner.IsVisible(), sibling.IsVisible()
			win := attachHeadlessDialogWindow(dialog)

			win.destroy()

			if dialog.Window() != nil {
				t.Fatal("closed dialog retains a destroyed native Window")
			}
			if dialog.IsVisible() {
				t.Fatal("closed dialog became a visible inline child of its owner")
			}
			if win.widget != nil {
				t.Fatal("destroyed window retains its widget")
			}
			if dialog.Parent() != owner || sibling.Parent() != owner {
				t.Fatal("destroy changed widget ownership")
			}
			if owner.IsVisible() != ownerVisible || sibling.IsVisible() != siblingVisible {
				t.Fatal("closing the dialog changed owner or sibling visibility")
			}
			if owner.Window() != ownerWindow {
				t.Fatal("closing the dialog destroyed its owner's attachment")
			}

			// Native destruction can be reached again after a deferred close.
			win.destroy()
			if dialog.Window() != nil || dialog.IsVisible() {
				t.Fatal("repeated destroy resurrected the closed dialog")
			}

			// The widget keeps its logical owner for a later ShowModal. Model
			// the new attachment here rather than opening a real native window.
			next := attachHeadlessDialogWindow(dialog)
			if next.ParentWindow() != ownerWindow {
				t.Fatal("new attachment lost its original owner")
			}
			win.destroy()
			if dialog.Window() != next || !dialog.IsVisible() {
				t.Fatal("old deferred destruction damaged the new attachment")
			}
			next.destroy()
			if dialog.Window() != nil || dialog.IsVisible() {
				t.Fatal("second modal lifetime did not close cleanly")
			}
		})
	}
}

func TestWindowDestroyDoesNotHideWidgetDetachedForInlineReuse(t *testing.T) {
	owner := NewForm()
	widget := NewDialog("inline reuse", owner)
	win := attachHeadlessDialogWindow(widget)
	// Widget.DetachWindow clears win before destroying the former window. It
	// deliberately preserves visibility while changing to WtInherit.
	widget.NakedWidget().win = nil
	widget.NakedWidget().wt = WtInherit
	win.destroy()
	if !widget.IsVisible() || widget.Parent() != owner || widget.Window() != nil {
		t.Fatal("destroy hid or detached a widget already returned to inline mode")
	}
}
