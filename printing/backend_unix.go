//go:build !windows

package printing

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type cupsBackend struct{ runner commandRunner }

func newSystemBackend() Backend { return &cupsBackend{runner: systemRunner{}} }

func (b *cupsBackend) Printers(ctx context.Context) ([]Printer, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tool, err := findCommand(b.runner, "lpstat")
	if err != nil {
		return nil, err
	}
	out, err := b.runner.Run(ctx, tool, []string{"-p"})
	if err != nil {
		if strings.Contains(out, "No destinations added") {
			return nil, ErrNoPrinters
		}
		return nil, commandError(ctx, "lpstat -p", out, err, false)
	}
	printers := parseCUPSPrinters(out)
	if len(printers) == 0 {
		return nil, ErrNoPrinters
	}
	out, err = b.runner.Run(ctx, tool, []string{"-d"})
	if err != nil && !strings.Contains(out, "no system default destination") {
		return nil, commandError(ctx, "lpstat -d", out, err, false)
	}
	def := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(out), "system default destination:"))
	for i := range printers {
		printers[i].Default = strings.EqualFold(printers[i].Name, def)
	}
	return printers, nil
}

func parseCUPSPrinters(output string) []Printer {
	seen := map[string]bool{}
	var printers []Printer
	for _, line := range strings.Split(output, "\n") {
		words := strings.Fields(line)
		if len(words) < 2 || words[0] != "printer" || seen[words[1]] {
			continue
		}
		seen[words[1]] = true
		printers = append(printers, Printer{Name: words[1]})
	}
	sort.Slice(printers, func(i, j int) bool { return printers[i].Name < printers[j].Name })
	return printers
}

var cupsJobID = regexp.MustCompile(`(?m)^request id is (\S+)`)

// CUPS lp copies its input before returning; no external PDF-viewer lifetime
// is involved. The -- separator and argument vector prevent shell injection.
// https://openprinting.github.io/cups/doc/man-lp.html
func (b *cupsBackend) SubmitPDF(ctx context.Context, path string, options Options) (Job, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tool, err := findCommand(b.runner, "lp")
	if err != nil {
		return Job{}, err
	}
	args := []string{"-d", options.PrinterName, "-n", strconv.Itoa(options.Copies), "-t", options.Title, "-o", "fit-to-page", "--", path}
	out, err := b.runner.Run(ctx, tool, args)
	if err != nil {
		return Job{}, commandError(ctx, "lp", out, err, true)
	}
	m := cupsJobID.FindStringSubmatch(out)
	if len(m) != 2 {
		return Job{}, fmt.Errorf("%w: lp returned no job ID", ErrSubmissionUnknown)
	}
	return Job{ID: m[1], PrinterName: options.PrinterName}, nil
}
