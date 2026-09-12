// Package printing enumerates installed printers and submits PDF documents.
// Submission is an explicit operation: enumeration never creates print jobs.
// A successful submission does not mean that paper has left the printer.
package printing

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrNoPrinters         = errors.New("printing: no printers installed")
	ErrNoDefaultPrinter   = errors.New("printing: no default printer configured")
	ErrPrinterNotFound    = errors.New("printing: selected printer not found")
	ErrBackendUnavailable = errors.New("printing: required print backend is unavailable")
	ErrInvalidDocument    = errors.New("printing: a readable PDF document is required")
	ErrInvalidOptions     = errors.New("printing: invalid print options")
	ErrCanceled           = errors.New("printing: canceled before submission")
	ErrSubmissionUnknown  = errors.New("printing: submission status unknown; check the printer queue before retrying")
)

type Printer struct {
	Name    string
	Default bool
}

// Options deliberately omits paper/tray/duplex settings: printer defaults are
// used and PDF content is fitted to the printable area. Copies is 1..99; zero
// selects one copy. Windows PDF renderers choose their own spooler job title.
type Options struct {
	PrinterName string
	Copies      int
	Title       string
}

type Job struct {
	ID          string
	PrinterName string
	// Dispatched means the PDF renderer accepted the document but exposes no
	// spooler job ID. It must not be presented as verified physical printing.
	Dispatched bool
}

// Backend allows applications/tests to provide an explicitly chosen spooler.
// SubmitPDF is called only after PDF/options/printer validation succeeds.
type Backend interface {
	Printers(context.Context) ([]Printer, error)
	SubmitPDF(context.Context, string, Options) (Job, error)
}

type Client struct{ backend Backend }

func NewClient(backend Backend) *Client {
	if backend == nil {
		backend = newSystemBackend()
	}
	return &Client{backend: backend}
}

func Printers(ctx context.Context) ([]Printer, error)     { return NewClient(nil).Printers(ctx) }
func DefaultPrinter(ctx context.Context) (Printer, error) { return NewClient(nil).DefaultPrinter(ctx) }

// SubmitPDF waits until the spool command/renderer exits; the caller must keep
// path readable until return. Canceling an in-flight command cannot recall a
// queued job: its result is ErrSubmissionUnknown, not a promise of cancellation.
func SubmitPDF(ctx context.Context, path string, options Options) (Job, error) {
	return NewClient(nil).SubmitPDF(ctx, path, options)
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("%w: nil context", ErrInvalidOptions)
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(ErrCanceled, err)
	}
	return nil
}

func (c *Client) Printers(ctx context.Context) ([]Printer, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	printers, err := c.backend.Printers(ctx)
	if err != nil {
		return nil, err
	}
	if len(printers) == 0 {
		return nil, ErrNoPrinters
	}
	return printers, nil
}

func (c *Client) DefaultPrinter(ctx context.Context) (Printer, error) {
	printers, err := c.Printers(ctx)
	if err != nil {
		return Printer{}, err
	}
	for _, p := range printers {
		if p.Default {
			return p, nil
		}
	}
	return Printer{}, ErrNoDefaultPrinter
}

func (c *Client) SubmitPDF(ctx context.Context, path string, options Options) (Job, error) {
	if err := checkContext(ctx); err != nil {
		return Job{}, err
	}
	path, err := ValidatePDF(path)
	if err != nil {
		return Job{}, err
	}
	if options.Copies == 0 {
		options.Copies = 1
	}
	if options.Copies < 1 || options.Copies > 99 || strings.ContainsAny(options.PrinterName+options.Title, "\x00\r\n") {
		return Job{}, ErrInvalidOptions
	}
	printers, err := c.Printers(ctx)
	if err != nil {
		return Job{}, err
	}
	wanted := options.PrinterName
	options.PrinterName = ""
	for _, p := range printers {
		if wanted == "" && p.Default || wanted != "" && strings.EqualFold(wanted, p.Name) {
			options.PrinterName = p.Name
			break
		}
	}
	if options.PrinterName == "" {
		if wanted == "" {
			return Job{}, ErrNoDefaultPrinter
		}
		return Job{}, fmt.Errorf("%w: %s", ErrPrinterNotFound, wanted)
	}
	if options.Title == "" {
		options.Title = filepath.Base(path)
	}
	if err := checkContext(ctx); err != nil {
		return Job{}, err
	}
	return c.backend.SubmitPDF(ctx, path, options)
}

// ValidatePDF rejects directories, non-PDF files and unreadable documents.
// It checks the signature, not full PDF validity; the system renderer performs
// the actual PDF validation. Absolute paths also prevent option-like filenames
// from being interpreted as command switches.
func ValidatePDF(path string) (string, error) {
	if path == "" || strings.ContainsRune(path, 0) {
		return "", ErrInvalidDocument
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidDocument, err)
	}
	// Reject known pipes/devices before opening: opening a FIFO for reading
	// could otherwise block the UI during confirmation indefinitely.
	if info, err := os.Stat(abs); err != nil || !info.Mode().IsRegular() {
		return "", ErrInvalidDocument
	}
	f, err := os.Open(abs)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidDocument, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", ErrInvalidDocument
	}
	var header [5]byte
	if _, err := io.ReadFull(f, header[:]); err != nil || string(header[:]) != "%PDF-" {
		return "", ErrInvalidDocument
	}
	return abs, nil
}
