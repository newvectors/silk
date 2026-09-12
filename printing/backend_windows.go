//go:build windows

package printing

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	printSpooler       = windows.NewLazySystemDLL("winspool.drv")
	enumPrintersW      = printSpooler.NewProc("EnumPrintersW")
	getDefaultPrinterW = printSpooler.NewProc("GetDefaultPrinterW")
)

type windowsBackend struct{ runner commandRunner }

func newSystemBackend() Backend { return &windowsBackend{runner: systemRunner{}} }

type printerInfo4 struct {
	name, server *uint16
	attributes   uint32
}

// Level 4 reads installed printers/connections from the local cache; unlike
// level 2 it does not OpenPrinter against every remote server. Still call it
// off the UI thread. https://learn.microsoft.com/windows/win32/printdocs/enumprinters
func (b *windowsBackend) Printers(ctx context.Context) ([]Printer, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	var needed, count uint32
	const flags = 2 | 4 // PRINTER_ENUM_LOCAL | PRINTER_ENUM_CONNECTIONS
	r, _, callErr := enumPrintersW.Call(flags, 0, 4, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&count)))
	if r == 0 && callErr != syscall.ERROR_INSUFFICIENT_BUFFER {
		return nil, fmt.Errorf("printing: EnumPrintersW: %w", callErr)
	}
	if needed == 0 {
		return nil, ErrNoPrinters
	}
	if needed > 16<<20 {
		return nil, fmt.Errorf("printing: printer list exceeds safe buffer limit")
	}
	buf := make([]byte, needed)
	r, _, callErr = enumPrintersW.Call(flags, 0, 4, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&count)))
	if r == 0 {
		return nil, fmt.Errorf("printing: EnumPrintersW: %w", callErr)
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	stride := unsafe.Sizeof(printerInfo4{})
	if uint64(count)*uint64(stride) > uint64(len(buf)) {
		return nil, fmt.Errorf("printing: invalid printer array size")
	}
	def, err := windowsDefaultPrinter()
	if err != nil {
		return nil, err
	}
	printers := make([]Printer, 0, count)
	for i := uint32(0); i < count; i++ {
		info := (*printerInfo4)(unsafe.Pointer(&buf[uintptr(i)*stride]))
		name, err := printerString(buf, info.name)
		if err != nil {
			return nil, err
		}
		if name != "" {
			printers = append(printers, Printer{Name: name, Default: strings.EqualFold(name, def)})
		}
	}
	runtime.KeepAlive(buf)
	if len(printers) == 0 {
		return nil, ErrNoPrinters
	}
	sort.Slice(printers, func(i, j int) bool { return printers[i].Name < printers[j].Name })
	return printers, nil
}

func windowsDefaultPrinter() (string, error) {
	var needed uint32
	r, _, err := getDefaultPrinterW.Call(0, uintptr(unsafe.Pointer(&needed)))
	if r == 0 && err == syscall.ERROR_FILE_NOT_FOUND {
		return "", nil
	}
	if r == 0 && err != syscall.ERROR_INSUFFICIENT_BUFFER {
		return "", fmt.Errorf("printing: GetDefaultPrinterW: %w", err)
	}
	if needed == 0 {
		return "", nil
	}
	if needed > 32768 {
		return "", fmt.Errorf("printing: default printer name exceeds safe buffer limit")
	}
	buf := make([]uint16, needed)
	r, _, err = getDefaultPrinterW.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&needed)))
	if r == 0 {
		return "", fmt.Errorf("printing: GetDefaultPrinterW: %w", err)
	}
	return windows.UTF16ToString(buf), nil
}

func printerString(buf []byte, p *uint16) (string, error) {
	if p == nil {
		return "", nil
	}
	if len(buf) == 0 {
		return "", fmt.Errorf("printing: empty printer buffer")
	}
	start, end := uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&buf[0]))+uintptr(len(buf))
	address := uintptr(unsafe.Pointer(p))
	if address < start || address >= end || (address-start)%2 != 0 {
		return "", fmt.Errorf("printing: invalid printer name pointer")
	}
	units := unsafe.Slice(p, int((end-address)/2))
	for i, c := range units {
		if c == 0 {
			return windows.UTF16ToString(units[:i]), nil
		}
	}
	return "", fmt.Errorf("printing: unterminated printer name")
}

func (b *windowsBackend) SubmitPDF(ctx context.Context, path string, options Options) (Job, error) {
	return submitWithSumatra(ctx, b.runner, path, options)
}
