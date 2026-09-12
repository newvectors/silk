package printing

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type fakeBackend struct {
	printers  []Printer
	err       error
	submitted int
	path      string
	options   Options
}

func (b *fakeBackend) Printers(context.Context) ([]Printer, error) { return b.printers, b.err }
func (b *fakeBackend) SubmitPDF(_ context.Context, path string, opts Options) (Job, error) {
	b.submitted++
	b.path, b.options = path, opts
	return Job{ID: "office-42", PrinterName: opts.PrinterName}, nil
}
func pdfFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "-report 中文 ' ; $(not-a-command).pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\n% test fixture; fake backend only\n%%EOF\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestClientValidatesAndResolvesPrinterBeforeSubmission(t *testing.T) {
	path := pdfFixture(t)
	b := &fakeBackend{printers: []Printer{{Name: "Office", Default: true}, {Name: "Other"}}}
	client := NewClient(b)
	if p, err := client.DefaultPrinter(context.Background()); err != nil || p.Name != "Office" {
		t.Fatalf("default=%+v %v", p, err)
	}
	if b.submitted != 0 {
		t.Fatal("enumeration submitted a job")
	}
	job, err := client.SubmitPDF(context.Background(), path, Options{})
	if err != nil || job.ID != "office-42" {
		t.Fatalf("submit=%+v %v", job, err)
	}
	want := Options{PrinterName: "Office", Copies: 1, Title: filepath.Base(path)}
	if b.path != path || b.options != want {
		t.Fatalf("path/options=%q %+v", b.path, b.options)
	}
	_, err = client.SubmitPDF(context.Background(), path, Options{PrinterName: "oThEr", Copies: 99, Title: `literal ' ; $(not-a-command)`})
	if err != nil || b.options.PrinterName != "Other" || b.options.Copies != 99 {
		t.Fatalf("explicit=%+v %v", b.options, err)
	}
}
func TestClientFailuresNeverSubmit(t *testing.T) {
	path := pdfFixture(t)
	cases := []struct {
		name       string
		opts       Options
		printers   []Printer
		backendErr error
		want       error
	}{
		{"no printers", Options{}, nil, nil, ErrNoPrinters},
		{"no default", Options{}, []Printer{{Name: "Office"}}, nil, ErrNoDefaultPrinter},
		{"unknown destination", Options{PrinterName: "missing"}, []Printer{{Name: "Office", Default: true}}, nil, ErrPrinterNotFound},
		{"negative copies", Options{Copies: -1}, nil, nil, ErrInvalidOptions},
		{"too many copies", Options{Copies: 100}, nil, nil, ErrInvalidOptions},
		{"newline title", Options{Title: "a\nb"}, nil, nil, ErrInvalidOptions},
		{"nul destination", Options{PrinterName: "a\x00b"}, nil, nil, ErrInvalidOptions},
		{"missing backend", Options{}, nil, ErrBackendUnavailable, ErrBackendUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := &fakeBackend{printers: tc.printers, err: tc.backendErr}
			_, err := NewClient(b).SubmitPDF(context.Background(), path, tc.opts)
			if !errors.Is(err, tc.want) || b.submitted != 0 {
				t.Fatalf("error=%v submissions=%d", err, b.submitted)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b := &fakeBackend{printers: []Printer{{Name: "Office", Default: true}}}
	if _, err := NewClient(b).SubmitPDF(ctx, path, Options{}); !errors.Is(err, ErrCanceled) || !errors.Is(err, context.Canceled) || b.submitted != 0 {
		t.Fatalf("cancellation=%v jobs=%d", err, b.submitted)
	}
	if _, err := NewClient(b).Printers(nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil context=%v", err)
	}
}
func TestValidatePDF(t *testing.T) {
	good := pdfFixture(t)
	if abs, err := ValidatePDF(good); err != nil || abs != good {
		t.Fatalf("valid=%q %v", abs, err)
	}
	bad := filepath.Join(t.TempDir(), "fake.pdf")
	if err := os.WriteFile(bad, []byte("not a PDF"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", "bad\x00.pdf", t.TempDir(), good + "missing", bad} {
		if _, err := ValidatePDF(path); !errors.Is(err, ErrInvalidDocument) {
			t.Errorf("%q => %v", path, err)
		}
	}
	b := &fakeBackend{printers: []Printer{{Name: "Office", Default: true}}}
	if _, err := NewClient(b).SubmitPDF(context.Background(), bad, Options{}); !errors.Is(err, ErrInvalidDocument) || b.submitted != 0 {
		t.Fatalf("invalid submitted: %v", err)
	}
}

type commandCall struct {
	name string
	args []string
}
type fakeRunner struct {
	calls   []commandCall
	missing string
	run     func(context.Context, string, []string) (string, error)
}

func (r *fakeRunner) LookPath(name string) (string, error) {
	if r.missing == name {
		return "", errors.New("not installed")
	}
	return "/tools/" + name, nil
}
func (r *fakeRunner) Run(ctx context.Context, name string, args []string) (string, error) {
	r.calls = append(r.calls, commandCall{name, append([]string(nil), args...)})
	return r.run(ctx, name, args)
}
func TestSumatraUsesExplicitRendererAndLiteralArguments(t *testing.T) {
	path := `C:\Reports\中文 ; quote ' report.pdf`
	opts := Options{PrinterName: `\\server\Office Laser`, Copies: 3}
	r := &fakeRunner{run: func(context.Context, string, []string) (string, error) { return "", nil }}
	job, err := submitWithSumatra(context.Background(), r, path, opts)
	want := []string{"-new-window", "-silent", "-print-to", opts.PrinterName, "-print-settings", "3x,fit", path}
	if err != nil || !job.Dispatched || job.ID != "" || len(r.calls) != 1 || r.calls[0].name != "/tools/SumatraPDF.exe" || !reflect.DeepEqual(r.calls[0].args, want) {
		t.Fatalf("job=%+v err=%v calls=%+v", job, err, r.calls)
	}
	r.missing = "SumatraPDF.exe"
	if _, err := submitWithSumatra(context.Background(), r, path, opts); !errors.Is(err, ErrBackendUnavailable) || len(r.calls) != 1 {
		t.Fatalf("missing renderer=%v calls=%d", err, len(r.calls))
	}
}
func TestRendererFailureAndCancellationAreNotFalseSuccess(t *testing.T) {
	failure := errors.New("exit status 3")
	r := &fakeRunner{run: func(context.Context, string, []string) (string, error) { return "Cannot print", failure }}
	if job, err := submitWithSumatra(context.Background(), r, "doc.pdf", Options{Copies: 1}); !errors.Is(err, failure) || job.Dispatched {
		t.Fatalf("job=%+v error=%v", job, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.run = func(context.Context, string, []string) (string, error) { cancel(); return "", context.Canceled }
	if _, err := submitWithSumatra(ctx, r, "doc.pdf", Options{Copies: 1}); !errors.Is(err, ErrSubmissionUnknown) || errors.Is(err, ErrCanceled) {
		t.Fatalf("unsafe cancellation claim: %v", err)
	}
}
func TestBoundedOutputConcurrentWrites(t *testing.T) {
	b := new(boundedOutput)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := []byte(strings.Repeat("x", 4096))
			if n, err := b.Write(p); n != len(p) || err != nil {
				t.Errorf("Write=%d %v", n, err)
			}
		}()
	}
	wg.Wait()
	if got := len(b.String()); got != 8192 {
		t.Fatalf("output length=%d", got)
	}
}
