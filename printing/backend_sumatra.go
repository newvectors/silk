package printing

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// Windows has no universal built-in PDF-to-printer API. Require a known PDF
// renderer instead of sending arbitrary PDF bytes as RAW printer language, or
// launching an unknown file association whose document lifetime is unknowable.
// SumatraPDF's documented -print-to exits after printing and reports errors by
// exit status. No shell, DDE, -reuse-instance or command-string interpolation.
// https://www.sumatrapdfreader.org/docs/Command-line-arguments
func submitWithSumatra(ctx context.Context, r commandRunner, path string, opts Options) (Job, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	tool, err := findCommand(r, "SumatraPDF.exe")
	if err != nil {
		return Job{}, fmt.Errorf("%w; install SumatraPDF and add its directory to PATH to print PDF on Windows", err)
	}
	args := []string{"-new-window", "-silent", "-print-to", opts.PrinterName, "-print-settings", strconv.Itoa(opts.Copies) + "x,fit", path}
	out, err := r.Run(ctx, tool, args)
	if err != nil {
		return Job{}, commandError(ctx, "SumatraPDF", out, err, true)
	}
	return Job{PrinterName: opts.PrinterName, Dispatched: true}, nil
}
