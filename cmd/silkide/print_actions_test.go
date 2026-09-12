package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/uk0/silk/gui"
	"github.com/uk0/silk/printing"
)

func TestPrintDesignStagesActualPDFAndCleansHostTemporaryFile(t *testing.T) {
	w := testWorkbench(t)
	w.activate(modeDesign)
	old := showDesignPrintDialog
	t.Cleanup(func() { showDesignPrintDialog = old })
	calls := 0
	path := ""
	showDesignPrintDialog = func(owner gui.IWidget, p string, opts printing.Options, finished ...func(printing.Job, error)) {
		calls++
		path = p
		if owner != w.canvas || opts.Copies != 1 {
			t.Fatal("print dialog lost current owner/options")
		}
		data, err := os.ReadFile(p)
		if err != nil || !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.Contains(data, []byte("/Subtype /Image")) {
			t.Fatalf("print opened before a complete rendered PDF existed: %v", err)
		}
		for _, fn := range finished {
			fn(printing.Job{}, printing.ErrCanceled)
		}
	}
	b := menuCommandButton(buildHamburgerMenu(gui.NewButton(), w.tabs, w.canvas), "file.print")
	if b == nil || !b.IsEnabled() || b.Icon() == nil {
		t.Fatal("Print Design menu missing")
	}
	b.Action().Trigger(b)
	if calls != 1 {
		t.Fatalf("print dialog calls=%d", calls)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("host temporary PDF not removed after dialog staged/canceled")
	}
}

func TestPrintDesignCannotPrintHiddenDocument(t *testing.T) {
	w := testWorkbench(t)
	w.activate(modeWelcome)
	old := showDesignPrintDialog
	t.Cleanup(func() { showDesignPrintDialog = old })
	showDesignPrintDialog = func(gui.IWidget, string, printing.Options, ...func(printing.Job, error)) {
		t.Fatal("hidden design reached print backend")
	}
	b := menuCommandButton(buildHamburgerMenu(gui.NewButton(), w.tabs, w.canvas), "file.print")
	if b == nil || b.IsEnabled() {
		t.Fatal("Print Design enabled outside design mode")
	}
	performPrintDesign(w.canvas)
	performPrintDesign(nil)
}
