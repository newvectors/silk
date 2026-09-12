package gui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/uk0/silk/i18n"
	"github.com/uk0/silk/printing"
)

type printDialogDeps struct {
	client *printing.Client
	modal  func(*Dialog) DialogResult
	choose func() string
	spawn  func(func())
	post   func(func())
	notify func(IWidget, printing.Job, error)
}

// ShowPrintDialog must be called on the UI thread. It returns after the modal
// confirmation/cancel, not after printing. On confirmation it synchronously
// stages a private copy before returning; callers may then delete their original
// temporary PDF. Submission and printer discovery run off-thread. Completion
// callbacks are delivered on the UI thread, including an explicit ErrCanceled.
// No job is sent without an affirmative Print decision.
func ShowPrintDialog(owner IWidget, path string, options printing.Options, finished ...func(printing.Job, error)) {
	showPrintDialogWith(owner, path, options, finished, printDialogDeps{
		client: printing.NewClient(nil), modal: func(d *Dialog) DialogResult { return d.ShowModal() },
		choose: OpenFileDialog, spawn: func(fn func()) { go fn() }, post: Post, notify: notifyPrintResult,
	})
}

func showPrintDialogWith(owner IWidget, path string, options printing.Options, finished []func(printing.Job, error), deps printDialogDeps) {
	finish := func(job printing.Job, err error) {
		deps.notify(owner, job, err)
		for _, fn := range finished {
			if fn != nil {
				fn(job, err)
			}
		}
	}
	s := newPrintDialog(owner, path, options, deps.choose)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	deps.spawn(func() {
		printers, err := deps.client.Printers(ctx)
		deps.post(func() {
			if !s.closed {
				s.setPrinters(printers, err, options.PrinterName)
			}
		})
	})
	result := deps.modal(s.dialog)
	s.closed = true
	cancel()
	if result != DialogOK {
		finish(printing.Job{}, printing.ErrCanceled)
		return
	}
	// Recheck rather than trusting only a disabled UI button. This also covers
	// stale selection, keyboard defaults and host-specific modal implementations.
	if !s.canPrint() {
		finish(printing.Job{}, fmt.Errorf("%w: select a PDF and a printer", printing.ErrInvalidOptions))
		return
	}
	options.PrinterName = s.printers[s.body.printer.ActiveIndex()].Name
	options.Copies = s.body.copies.Value()
	options.Title = s.body.title.Text()
	if options.Title == "" {
		options.Title = filepath.Base(s.body.path.Text())
	}
	staged, cleanup, err := stagePrintDocument(s.body.path.Text())
	if err != nil {
		finish(printing.Job{}, err)
		return
	}
	deps.spawn(func() {
		defer cleanup()
		ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
		defer cancel()
		job, err := deps.client.SubmitPDF(ctx, staged, options)
		deps.post(func() { finish(job, err) })
	})
}

type printDialogState struct {
	dialog       *Dialog
	body         *printDialogBody
	confirm      *Button
	printers     []printing.Printer
	discoveryErr error
	closed       bool
}

func newPrintDialog(owner IWidget, path string, options printing.Options, choose func() string) *printDialogState {
	s := &printDialogState{dialog: NewDialog(i18n.T("Print PDF"), owner)}
	s.dialog.SetVisible(false)
	s.body = newPrintDialogBody()
	s.body.path.SetText(path)
	s.body.copies.SetValue(max(1, options.Copies))
	s.body.title.SetText(options.Title)
	s.dialog.SetContent(s.body)
	s.dialog.AddButton(i18n.T("Cancel"), DialogCancel)
	s.confirm = s.dialog.AddButton(i18n.T("Print"), DialogOK)
	s.confirm.SetEnabled(false)
	s.dialog.markDefaultButton()
	s.body.status.SetText(i18n.T("Finding printers..."))
	s.body.choose.Action().BindFunc0(func() {
		if path := choose(); path != "" {
			s.body.path.SetText(path)
		}
	})
	s.body.path.SigTextChanged(func(_ interface{}, _ string) { s.refresh() })
	s.body.printer.SigSelectionChanged(func(_ interface{}, _ int) { s.refresh() })
	s.dialog.SetSize(580, 350)
	return s
}

func (s *printDialogState) canPrint() bool {
	i := s.body.printer.ActiveIndex()
	return s.discoveryErr == nil && strings.TrimSpace(s.body.path.Text()) != "" && i >= 0 && i < len(s.printers)
}

func (s *printDialogState) setPrinters(printers []printing.Printer, err error, wanted string) {
	s.printers = printers
	s.discoveryErr = err
	if err == nil && len(printers) == 0 {
		s.discoveryErr = printing.ErrNoPrinters
	}
	labels := make([]string, len(printers))
	idx := -1
	for i, p := range printers {
		labels[i] = p.Name
		if p.Default {
			labels[i] += " (" + i18n.T("Default") + ")"
		}
		if wanted != "" && strings.EqualFold(wanted, p.Name) || wanted == "" && p.Default {
			idx = i
		}
	}
	s.body.printer.SetItems(labels)
	s.body.printer.SetActiveIndex(idx)
	s.refresh()
}

func (s *printDialogState) refresh() {
	s.confirm.SetEnabled(s.canPrint())
	s.dialog.markDefaultButton()
	message := ""
	switch {
	case s.discoveryErr != nil:
		message = PrintErrorMessage(s.discoveryErr)
	case len(s.printers) == 0:
		message = i18n.T("Finding printers...")
	case strings.TrimSpace(s.body.path.Text()) == "":
		message = i18n.T("Select a PDF document to print.")
	case s.body.printer.ActiveIndex() < 0:
		message = i18n.T("Select a printer.")
	default:
		message = i18n.T("Press Print to send this document to the selected printer.")
	}
	s.body.status.SetText(message)
}

// PrintErrorMessage provides actionable text without interpreting absence of
// a backend/printer as a successful or silently ignored print operation.
func PrintErrorMessage(err error) string {
	switch {
	case errors.Is(err, printing.ErrNoPrinters):
		return i18n.T("No printers installed. Configure a printer in system settings.")
	case errors.Is(err, printing.ErrBackendUnavailable):
		return i18n.T("Printing tools unavailable. Install CUPS client tools, or SumatraPDF on PATH for Windows.")
	case errors.Is(err, printing.ErrNoDefaultPrinter):
		return i18n.T("No default printer. Select a printer explicitly.")
	case errors.Is(err, printing.ErrPrinterNotFound):
		return i18n.T("The selected printer is unavailable. Reopen Print and choose another printer.")
	case errors.Is(err, printing.ErrInvalidDocument):
		return i18n.T("Choose an existing, readable PDF file.")
	case errors.Is(err, printing.ErrSubmissionUnknown):
		return i18n.T("Submission status unknown. Check the printer queue before retrying.")
	case errors.Is(err, printing.ErrCanceled):
		return i18n.T("Print canceled")
	default:
		return i18n.Tf("Print submission failed: %v", err)
	}
}

func notifyPrintResult(owner IWidget, job printing.Job, err error) {
	if errors.Is(err, printing.ErrCanceled) {
		return
	}
	if err != nil {
		ShowToast(owner, PrintErrorMessage(err), 5000, ToastError)
		return
	}
	if job.Dispatched {
		ShowToast(owner, i18n.T("PDF renderer accepted the document. Check the printer queue for progress."), 5000, ToastInfo)
	} else {
		ShowToast(owner, i18n.Tf("Print job submitted: %s", job.ID), 3500, ToastSuccess)
	}
}

// Copy a bounded document to a private, immutable-for-this-job path. A file
// chooser is not a print operation; staging occurs only after confirmation.
func stagePrintDocument(path string) (string, func(), error) {
	abs, err := printing.ValidatePDF(path)
	if err != nil {
		return "", nil, err
	}
	src, err := os.Open(abs)
	if err != nil {
		return "", nil, fmt.Errorf("%w: %v", printing.ErrInvalidDocument, err)
	}
	defer src.Close()
	const maxDocument = 256 << 20
	if info, err := src.Stat(); err != nil || !info.Mode().IsRegular() || info.Size() > maxDocument {
		return "", nil, printing.ErrInvalidDocument
	}
	dir, err := os.MkdirTemp("", "silk-print-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	dst := filepath.Join(dir, "document.pdf")
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	n, copyErr := io.Copy(out, io.LimitReader(src, maxDocument+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || n > maxDocument {
		cleanup()
		return "", nil, fmt.Errorf("%w: copy failed or PDF exceeds 256 MiB", printing.ErrInvalidDocument)
	}
	if _, err := printing.ValidatePDF(dst); err != nil {
		cleanup()
		return "", nil, err
	}
	return dst, cleanup, nil
}
