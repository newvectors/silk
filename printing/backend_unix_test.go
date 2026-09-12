//go:build !windows

package printing

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
)

func TestCUPSPrinterDiscovery(t *testing.T) {
	r := &fakeRunner{run: func(_ context.Context, _ string, args []string) (string, error) {
		if args[0] == "-p" {
			return "printer Zebra disabled since today\n\tPaused\nprinter Office is idle. enabled since today\nprinter Zebra disabled since today\n", nil
		}
		return "system default destination: Office\n", nil
	}}
	got, err := (&cupsBackend{r}).Printers(context.Background())
	want := []Printer{{Name: "Office", Default: true}, {Name: "Zebra"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("printers=%+v %v", got, err)
	}
	if len(r.calls) != 2 || r.calls[0].name != "/tools/lpstat" || !reflect.DeepEqual(r.calls[0].args, []string{"-p"}) || !reflect.DeepEqual(r.calls[1].args, []string{"-d"}) {
		t.Fatalf("discovery calls=%+v", r.calls)
	}
}
func TestCUPSNoPrinterOrDefaultIsExplicit(t *testing.T) {
	failure := errors.New("exit 1")
	r := &fakeRunner{run: func(context.Context, string, []string) (string, error) {
		return "lpstat: No destinations added.\n", failure
	}}
	b := &cupsBackend{r}
	if _, err := b.Printers(context.Background()); !errors.Is(err, ErrNoPrinters) {
		t.Fatalf("no printers=%v", err)
	}
	r.run = func(_ context.Context, _ string, args []string) (string, error) {
		if args[0] == "-p" {
			return "printer Office is idle.\n", nil
		}
		return "no system default destination\n", failure
	}
	got, err := b.Printers(context.Background())
	if err != nil || len(got) != 1 || got[0].Default {
		t.Fatalf("no default=%+v %v", got, err)
	}
	r.run = func(context.Context, string, []string) (string, error) { return "scheduler unreachable", failure }
	if _, err := b.Printers(context.Background()); !errors.Is(err, failure) || errors.Is(err, ErrNoPrinters) {
		t.Fatalf("scheduler error lost: %v", err)
	}
	r.missing = "lpstat"
	if _, err := b.Printers(context.Background()); !errors.Is(err, ErrBackendUnavailable) {
		t.Fatalf("missing lpstat=%v", err)
	}
}
func TestCUPSSubmissionUsesSeparateArgumentsAndRequiresJobID(t *testing.T) {
	r := &fakeRunner{run: func(context.Context, string, []string) (string, error) {
		return "request id is Office-123 (1 file(s))\n", nil
	}}
	b := &cupsBackend{r}
	opts := Options{PrinterName: "Office", Copies: 2, Title: `report '; $(not-executed)`}
	path := pdfFixture(t)
	job, err := b.SubmitPDF(context.Background(), path, opts)
	want := []string{"-d", "Office", "-n", "2", "-t", opts.Title, "-o", "fit-to-page", "--", path}
	if err != nil || job.ID != "Office-123" || job.Dispatched || len(r.calls) != 1 || !reflect.DeepEqual(r.calls[0].args, want) {
		t.Fatalf("job=%+v err=%v calls=%+v", job, err, r.calls)
	}
	r.run = func(context.Context, string, []string) (string, error) { return "", nil }
	if _, err := b.SubmitPDF(context.Background(), path, opts); !errors.Is(err, ErrSubmissionUnknown) {
		t.Fatalf("no job ID reported success: %v", err)
	}
	r.missing = "lp"
	if _, err := b.SubmitPDF(context.Background(), path, opts); !errors.Is(err, ErrBackendUnavailable) {
		t.Fatalf("missing lp=%v", err)
	}
}
func TestCUPSInFlightCancelMayAlreadyBeQueued(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &fakeRunner{run: func(context.Context, string, []string) (string, error) { cancel(); return "", context.Canceled }}
	if _, err := (&cupsBackend{r}).SubmitPDF(ctx, "doc.pdf", Options{Copies: 1}); !errors.Is(err, ErrSubmissionUnknown) || errors.Is(err, ErrCanceled) {
		t.Fatalf("unsafe cancellation claim: %v", err)
	}
}
func TestValidatePDFRejectsFIFOWithoutOpeningIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "document.pdf")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidatePDF(path); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("fifo=%v", err)
	}
}
