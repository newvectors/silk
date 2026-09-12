package gui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/uk0/silk/core"
	"github.com/uk0/silk/printing"
)

type printTestBackend struct {
	printers     []printing.Printer
	discoveryErr error
	submit       func(string, printing.Options) (printing.Job, error)
	jobs         int
}

func (b *printTestBackend) Printers(context.Context) ([]printing.Printer, error) {
	return b.printers, b.discoveryErr
}
func (b *printTestBackend) SubmitPDF(_ context.Context, path string, opts printing.Options) (printing.Job, error) {
	b.jobs++
	if b.submit != nil {
		return b.submit(path, opts)
	}
	return printing.Job{ID: "Office-1", PrinterName: opts.PrinterName}, nil
}
func printTestPDF(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "中文 source.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\n% fake test document\n%%EOF\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func printTestDeps(b *printTestBackend) printDialogDeps {
	return printDialogDeps{client: printing.NewClient(b), choose: func() string { return "" }, spawn: func(fn func()) { fn() }, post: func(fn func()) { fn() }, notify: func(IWidget, printing.Job, error) {}}
}
func TestPrintButtonFactoryAndPropertiesDoNotPrint(t *testing.T) {
	b, ok := core.New("gui.PrintButton").(*PrintButton)
	if !ok || b.Copies() != 1 || b.IconName() != "printer" || b.Action() == nil || b.showPrintDialog == nil {
		t.Fatalf("factory defaults=%+v", b)
	}
	calls := 0
	b.showPrintDialog = func(IWidget, string, printing.Options, ...func(printing.Job, error)) { calls++ }
	list := new(recordingPropertyList)
	b.EnumProperties(list)
	for _, tc := range []struct {
		name  string
		value any
		get   func() any
	}{
		{"打印文档", "/reports/sample.pdf", func() any { return b.DocumentPath() }},
		{"打印机", "Office", func() any { return b.PrinterName() }},
		{"份数", 3, func() any { return b.Copies() }},
		{"任务标题", "中文任务", func() any { return b.JobTitle() }},
	} {
		p, ok := list.find(tc.name)
		if !ok {
			t.Fatalf("property %q absent", tc.name)
		}
		reflect.ValueOf(p.set).Call([]reflect.Value{reflect.ValueOf(tc.value)})
		if tc.get() != tc.value {
			t.Errorf("property %q=%v", tc.name, tc.get())
		}
	}
	b.SetCopies(1000)
	if b.Copies() != 99 {
		t.Fatal("copies exceed bound")
	}
	b.SetCopies(-1)
	if b.Copies() != 1 {
		t.Fatal("copies below bound")
	}
	if calls != 0 || b.IsPrinting() {
		t.Fatal("construction/property editing caused printing")
	}
}
func TestPrintButtonOnClickKeepsBuiltInConfirmationAndBusyGuard(t *testing.T) {
	b := NewPrintButton()
	calls, clicks, finishes := 0, 0, 0
	var complete func(printing.Job, error)
	b.Action().BindFunc0(func() { clicks++; b.SetDocumentPath("configured-in-OnClick.pdf") })
	b.showPrintDialog = func(owner IWidget, path string, opts printing.Options, finished ...func(printing.Job, error)) {
		calls++
		if owner != b || path != "configured-in-OnClick.pdf" || opts.Copies != 1 {
			t.Fatalf("dialog owner/path/options: %v %q %+v", owner, path, opts)
		}
		if !b.IsPrinting() {
			t.Fatal("busy state not set before modal")
		}
		complete = finished[0]
	}
	b.SigPrintFinished(func(id, msg string) {
		finishes++
		if id != "Office-9" || msg != "" || b.IsPrinting() {
			t.Errorf("completion: %q %q busy=%v", id, msg, b.IsPrinting())
		}
	})
	b.Action().Trigger(b)
	b.Print()
	if clicks != 1 || calls != 1 || finishes != 0 || !b.IsPrinting() {
		t.Fatalf("clicks=%d modal=%d finished=%d", clicks, calls, finishes)
	}
	complete(printing.Job{ID: "Office-9"}, nil)
	if finishes != 1 || b.IsPrinting() {
		t.Fatal("completion did not release busy state")
	}
	b.SetEnabled(false)
	b.Print()
	if calls != 1 {
		t.Fatal("disabled PrintButton printed")
	}
	b.SetEnabled(true)
	b.SigPrintFinished(func(id, msg string) {
		if id != "" || !strings.Contains(msg, "canceled") {
			t.Fatalf("cancellation callback=%q %q", id, msg)
		}
	})
	b.Print()
	complete(printing.Job{}, printing.ErrCanceled)
	if b.IsPrinting() {
		t.Fatal("cancel leaves button permanently busy")
	}
}
func TestPrintDialogOwnerNoDocumentAndPrinterSelection(t *testing.T) {
	owner := NewForm()
	selected := ""
	s := newPrintDialog(owner, "", printing.Options{}, func() string { return selected })
	if s.dialog.Parent() != owner || s.dialog.Window() != nil || s.dialog.IsVisible() {
		t.Fatal("dialog is unowned, native, or visible before showing")
	}
	if s.confirm.IsEnabled() || s.canPrint() {
		t.Fatal("empty dialog can submit")
	}
	s.setPrinters([]printing.Printer{{Name: "Office", Default: true}}, nil, "")
	if s.canPrint() || s.confirm.IsEnabled() {
		t.Fatal("printer discovery prints without a document")
	}
	s.body.choose.Action().Trigger(s.body.choose)
	if s.body.path.Text() != "" {
		t.Fatal("canceled file chooser changed path")
	}
	selected = printTestPDF(t)
	s.body.choose.Action().Trigger(s.body.choose)
	if s.body.path.Text() != selected || !s.canPrint() || !s.confirm.IsEnabled() {
		t.Fatal("chosen PDF did not enable confirmation")
	}
	if s.dialog.defaultButton() != s.confirm {
		t.Fatal("Enter is not wired to enabled Print confirmation")
	}
	s.setPrinters([]printing.Printer{{Name: "Office"}}, nil, "")
	if s.canPrint() || s.confirm.IsEnabled() || s.body.printer.ActiveIndex() != -1 {
		t.Fatal("no-default destination selected without user choice")
	}
	s.body.printer.SetActiveIndex(0)
	if !s.canPrint() {
		t.Fatal("explicit printer selection failed")
	}
	s.setPrinters(nil, printing.ErrNoPrinters, "")
	if s.canPrint() || s.confirm.IsEnabled() || s.body.status.Text() != PrintErrorMessage(printing.ErrNoPrinters) {
		t.Fatal("no-printer error was hidden or enabled Print")
	}
}
func TestPrintDialogCancelDoesNotStageOrSubmit(t *testing.T) {
	b := &printTestBackend{printers: []printing.Printer{{Name: "Office", Default: true}}}
	deps := printTestDeps(b)
	var late func()
	deps.post = func(fn func()) { late = fn }
	deps.modal = func(*Dialog) DialogResult { return DialogCancel }
	finishes := 0
	showPrintDialogWith(NewForm(), "missing-file.pdf", printing.Options{}, []func(printing.Job, error){func(_ printing.Job, err error) {
		finishes++
		if !errors.Is(err, printing.ErrCanceled) {
			t.Fatalf("cancel=%v", err)
		}
	}}, deps)
	if late == nil {
		t.Fatal("discovery completion not scheduled")
	}
	late() // Completion after closing must not resurrect or submit the dialog.
	if b.jobs != 0 || finishes != 1 {
		t.Fatalf("jobs=%d finishes=%d", b.jobs, finishes)
	}
}
func TestPrintDialogRejectsForcedConfirmationWithoutDestination(t *testing.T) {
	b := &printTestBackend{discoveryErr: printing.ErrNoPrinters}
	deps := printTestDeps(b)
	deps.modal = func(*Dialog) DialogResult { return DialogOK }
	var result error
	showPrintDialogWith(nil, printTestPDF(t), printing.Options{}, []func(printing.Job, error){func(_ printing.Job, err error) { result = err }}, deps)
	if !errors.Is(result, printing.ErrInvalidOptions) || b.jobs != 0 {
		t.Fatalf("forced result=%v jobs=%d", result, b.jobs)
	}
}
func TestPrintDialogStagesBeforeReturningAndCleansAfterSubmission(t *testing.T) {
	original := printTestPDF(t)
	contents, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	b := &printTestBackend{printers: []printing.Printer{{Name: "Office", Default: true}}}
	deps := printTestDeps(b)
	var work func()
	spawns := 0
	deps.spawn = func(fn func()) {
		spawns++
		if spawns == 1 {
			fn()
		} else {
			work = fn
		}
	}
	deps.modal = func(d *Dialog) DialogResult {
		body := d.Content().(*printDialogBody)
		body.copies.SetValue(2)
		body.title.SetText("中文确认任务")
		if !d.buttons[len(d.buttons)-1].IsEnabled() {
			t.Fatal("valid dialog cannot confirm")
		}
		return DialogOK
	}
	finishes := 0
	staged := ""
	b.submit = func(path string, opts printing.Options) (printing.Job, error) {
		staged = path
		if path == original || filepath.Base(path) != "document.pdf" {
			t.Fatalf("did not stage private copy: %q", path)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != string(contents) {
			t.Fatalf("staged contents=%q %v", got, err)
		}
		if opts.Copies != 2 || opts.PrinterName != "Office" || opts.Title != "中文确认任务" {
			t.Fatalf("captured options=%+v", opts)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
			t.Fatalf("private copy mode=%v", info.Mode())
		}
		return printing.Job{ID: "Office-21", PrinterName: "Office"}, nil
	}
	showPrintDialogWith(NewForm(), original, printing.Options{}, []func(printing.Job, error){func(job printing.Job, err error) {
		finishes++
		if job.ID != "Office-21" || err != nil {
			t.Errorf("completion=%+v %v", job, err)
		}
	}}, deps)
	if work == nil || b.jobs != 0 || finishes != 0 {
		t.Fatal("submission was synchronous or did not schedule")
	}
	if err := os.Remove(original); err != nil {
		t.Fatal(err)
	} // documented caller cleanup point
	work()
	if b.jobs != 1 || finishes != 1 {
		t.Fatalf("jobs=%d finishes=%d", b.jobs, finishes)
	}
	if _, err := os.Stat(filepath.Dir(staged)); !os.IsNotExist(err) {
		t.Fatalf("private stage survived completion: %v", err)
	}
}
func TestPrintDialogInvalidPDFAndSubmissionErrorAreReported(t *testing.T) {
	for _, badDoc := range []bool{true, false} {
		b := &printTestBackend{printers: []printing.Printer{{Name: "Office", Default: true}}, submit: func(string, printing.Options) (printing.Job, error) {
			return printing.Job{}, printing.ErrBackendUnavailable
		}}
		deps := printTestDeps(b)
		deps.modal = func(*Dialog) DialogResult { return DialogOK }
		doc := printTestPDF(t)
		want := printing.ErrBackendUnavailable
		if badDoc {
			if err := os.WriteFile(doc, []byte("not PDF"), 0600); err != nil {
				t.Fatal(err)
			}
			want = printing.ErrInvalidDocument
		}
		finished, notified := 0, 0
		deps.notify = func(_ IWidget, _ printing.Job, err error) {
			notified++
			if !errors.Is(err, want) {
				t.Fatalf("notification=%v", err)
			}
		}
		showPrintDialogWith(nil, doc, printing.Options{}, []func(printing.Job, error){func(_ printing.Job, err error) {
			finished++
			if !errors.Is(err, want) {
				t.Fatalf("callback=%v", err)
			}
		}}, deps)
		if finished != 1 || notified != 1 || badDoc && b.jobs != 0 {
			t.Fatalf("finished=%d notices=%d jobs=%d", finished, notified, b.jobs)
		}
	}
}
func TestPrintDialogLayoutKeepsFieldsAndFooterSeparate(t *testing.T) {
	s := newPrintDialog(NewForm(), "", printing.Options{}, func() string { return "" })
	for _, width := range []float64{380, 580, 800} {
		s.dialog.SetSize(width, 350)
		s.dialog.Layout()
		s.body.Layout()
		body := s.body.Bounds1()
		bar := s.dialog.btnBox.Bounds1()
		if body.Bottom() > bar.Top() {
			t.Fatalf("content overlaps confirmation bar: %v / %v", body, bar)
		}
		fields := []IWidget{s.body.path, s.body.choose, s.body.printer, s.body.copies, s.body.title, s.body.status, s.body.note}
		for _, field := range fields {
			r := field.Bounds1()
			if r.X < 0 || r.Y < 0 || r.Width < 0 || r.Height < 0 || r.Right() > body.Width+1e-6 || r.Bottom() > body.Height+1e-6 {
				t.Errorf("width=%v field %T outside body: %v body=%v", width, field, r, body)
			}
		}
		if s.body.path.Bounds1().Right() > s.body.choose.Bounds1().Left() {
			t.Fatal("file path overlaps chooser button")
		}
		if s.body.status.Bounds1().Bottom() > s.body.note.Bounds1().Top() {
			t.Fatal("status overlaps limitations note")
		}
	}
}
