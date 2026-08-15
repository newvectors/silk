package gui

import (
	"testing"

	"github.com/uk0/silk/core"
)

// factoryDialog builds a Dialog the way the designer palette and the form
// loader do -- reflection plus Init, with NewDialog nowhere in the picture.
func factoryDialog(t *testing.T) *Dialog {
	t.Helper()
	d, ok := core.New("gui.Dialog").(*Dialog)
	if !ok {
		t.Fatal("gui.Dialog factory did not produce a *Dialog")
	}
	return d
}

// addDialogButton drives AddButton and turns a panic into a readable failure.
func addDialogButton(t *testing.T, dlg *Dialog, text string, result DialogResult) *Button {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("AddButton(%q) on a factory-built dialog panicked: %v", text, r)
		}
	}()
	return dlg.AddButton(text, result)
}

// AddButton writes the result key into resultMap and hands the button to the
// button bar; a factory dialog arrives with both nil unless Init supplies them.
func TestFactoryDialogAddButton(t *testing.T) {
	dlg := factoryDialog(t)

	btn := addDialogButton(t, dlg, "OK", DialogOK)
	if btn == nil {
		t.Fatal("AddButton returned a nil button")
	}
	if got, ok := dlg.resultMap[dialogResultKey(DialogOK)]; !ok || got != DialogOK {
		t.Errorf("AddButton did not record the result: resultMap = %v", dlg.resultMap)
	}
	if len(dlg.buttons) != 1 || dlg.buttons[0] != btn {
		t.Errorf("AddButton did not register the button with the dialog: buttons = %v", dlg.buttons)
	}
	if dlg.btnBox == nil {
		t.Fatal("a factory-built dialog has no button bar")
	}
	if btn.Parent() != IWidget(dlg.btnBox) {
		t.Errorf("the new button was parented to %v, want the dialog button bar", btn.Parent())
	}
}

// Esc rejects with cancelResult. That field lands as the zero value DialogOK on
// a factory-built dialog unless Init sets it, which turns the reject key into
// the accept key and makes Esc silently confirm.
func TestFactoryDialogEscCancels(t *testing.T) {
	dlg := factoryDialog(t)
	addDialogButton(t, dlg, "OK", DialogOK)
	addDialogButton(t, dlg, "Cancel", DialogCancel)

	dlg.OnKeyDown(KeyEsc, false)

	if dlg.Result() != DialogCancel {
		t.Errorf("Esc on a factory-built dialog returned %s, want %s (DialogCancel)", dialogResultKey(dlg.Result()), dialogResultKey(DialogCancel))
	}
	if !dlg.closed {
		t.Error("Esc did not close the dialog")
	}
}

// NewDialog must still produce exactly the state it produced before the map,
// the cancel result and the button bar moved into Init: only title and parent
// come from the arguments.
func TestNewDialogConstructorParity(t *testing.T) {
	parent := NewVBox()
	dlg := NewDialog("t", parent)

	if dlg.Self() != IWidget(dlg) {
		t.Errorf("NewDialog: Self() = %v, want the dialog itself", dlg.Self())
	}
	if dlg.Title() != "t" {
		t.Errorf("NewDialog: title = %q, want %q", dlg.Title(), "t")
	}
	if dlg.Parent() != IWidget(parent) {
		t.Errorf("NewDialog: parent = %v, want the widget passed in", dlg.Parent())
	}
	if dlg.resultMap == nil {
		t.Error("NewDialog: resultMap is nil")
	} else if len(dlg.resultMap) != 0 {
		t.Errorf("NewDialog: resultMap = %v, want empty", dlg.resultMap)
	}
	if dlg.cancelResult != DialogCancel {
		t.Errorf("NewDialog: cancelResult = %s, want %s (DialogCancel)", dialogResultKey(dlg.cancelResult), dialogResultKey(DialogCancel))
	}
	if dlg.btnBox == nil {
		t.Error("NewDialog: btnBox is nil")
	} else {
		if dlg.btnBox.Parent() != IWidget(dlg) {
			t.Errorf("NewDialog: btnBox parent = %v, want the dialog", dlg.btnBox.Parent())
		}
		if dlg.btnBox.cbSubmit == nil {
			t.Error("NewDialog: btnBox submit callback is not bound")
		}
	}
	if dlg.contentWidget != nil {
		t.Errorf("NewDialog: contentWidget = %v, want nil", dlg.contentWidget)
	}
	if dlg.buttons != nil {
		t.Errorf("NewDialog: buttons = %v, want nil", dlg.buttons)
	}
	if dlg.defaultBtn != nil {
		t.Errorf("NewDialog: defaultBtn = %v, want nil", dlg.defaultBtn)
	}
	if dlg.closed {
		t.Error("NewDialog: closed = true, want false")
	}
	if dlg.result != DialogOK {
		t.Errorf("NewDialog: result = %v, want the zero value until ShowModal runs", dlg.result)
	}

	// A dialog built without a parent keeps the same defaults and stays unparented.
	orphan := NewDialog("t", nil)
	if orphan.Parent() != nil {
		t.Errorf("NewDialog(nil parent): parent = %v, want nil", orphan.Parent())
	}
	if orphan.cancelResult != DialogCancel {
		t.Errorf("NewDialog(nil parent): cancelResult = %s, want %s (DialogCancel)", dialogResultKey(orphan.cancelResult), dialogResultKey(DialogCancel))
	}
	if orphan.resultMap == nil {
		t.Error("NewDialog(nil parent): resultMap is nil")
	}
	if orphan.btnBox == nil {
		t.Error("NewDialog(nil parent): btnBox is nil")
	}
}
