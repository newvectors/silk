# Printing SDK

`Printers(ctx)` and `DefaultPrinter(ctx)` only inspect installed destinations.
`SubmitPDF(ctx, path, Options{PrinterName: "…", Copies: 1})` explicitly submits
a readable PDF. Keep the file until the synchronous call returns. No package
initialization, test, or enumeration operation submits a print job.

- macOS/Linux: installed CUPS destinations (`lpstat -p/-d`); `lp` returns the
  spooler job ID. Requires CUPS client tools and a configured printer.
- Windows: Unicode Winspool `EnumPrintersW` level 4 / `GetDefaultPrinterW`;
  PDF submission requires **SumatraPDF.exe on PATH**. It is not downloaded or
  bundled. Unknown file associations and RAW PDF-to-device writes are not used.
  `Job.Dispatched` means the renderer exited successfully without exposing a
  queue ID, not verified paper output. Windows job titles are renderer-owned.
- Copies: 1–99 (zero means one). Fit to printable area; printer-default paper,
  tray and duplex settings. No custom-paper, page-range or printer-preferences
  UI is claimed. Invalid/encrypted/restricted PDFs may be refused by the renderer.
- Cancellation before submission is `ErrCanceled`. A timeout/cancellation
  during submission is `ErrSubmissionUnknown`: inspect the print queue before
  retrying, because killing the client cannot recall an accepted job.

The GUI confirmation dialog stages its own private PDF before returning and
submits asynchronously, so its caller may delete the original temporary PDF
after `gui.ShowPrintDialog` returns. The private copy is removed after backend
completion. Applications can inject a `Backend` using `NewClient` for testing
without paper consumption.

Primary references: [CUPS lp](https://openprinting.github.io/cups/doc/man-lp.html),
[lpstat](https://openprinting.github.io/cups/doc/man-lpstat.html),
[Winspool enumeration](https://learn.microsoft.com/windows/win32/printdocs/enumprinters),
[SumatraPDF command line](https://www.sumatrapdfreader.org/docs/Command-line-arguments).
