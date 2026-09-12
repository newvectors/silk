package gui

import (
	"github.com/uk0/silk/core"
	"github.com/uk0/silk/i18n"
	"github.com/uk0/silk/printing"
)

func init() { core.RegisterFactory("gui.PrintButton", core.TypeOf((*PrintButton)(nil))) }

// PrintButton always asks for confirmation. DocumentPath may be empty: its
// dialog then offers a PDF file chooser. Property setters never print.
type PrintButton struct {
	Button
	documentPath, printerName, jobTitle string
	copies                              int
	printing                            bool
	finished                            func(jobID, errorText string)
	showPrintDialog                     func(IWidget, string, printing.Options, ...func(printing.Job, error))
}

func NewPrintButton() *PrintButton { b := new(PrintButton); b.Init(b); return b }
func (b *PrintButton) Init(self IWidget) {
	b.Button.Init(self)
	b.copies = 1
	b.showPrintDialog = ShowPrintDialog
	b.SetAction(&printButtonAction{print: b.Print})
	b.SetText(i18n.T("Print"))
	b.SetIconName("printer")
}

// A user OnClick binding can configure the document but cannot replace the
// internal print action. This wrapper preserves ordinary Action callbacks.
type printButtonAction struct {
	Action
	print func()
}

func (a *printButtonAction) Trigger(sender interface{}) {
	if !a.IsEnabled() {
		return
	}
	a.Action.Trigger(sender)
	if a.print != nil {
		a.print()
	}
}

func (b *PrintButton) DocumentPath() string        { return b.documentPath }
func (b *PrintButton) SetDocumentPath(path string) { b.documentPath = path }
func (b *PrintButton) PrinterName() string         { return b.printerName }
func (b *PrintButton) SetPrinterName(name string)  { b.printerName = name }
func (b *PrintButton) Copies() int                 { return b.copies }
func (b *PrintButton) SetCopies(copies int)        { b.copies = max(1, min(99, copies)) }
func (b *PrintButton) JobTitle() string            { return b.jobTitle }
func (b *PrintButton) SetJobTitle(title string)    { b.jobTitle = title }
func (b *PrintButton) IsPrinting() bool            { return b.printing }

// SigPrintFinished runs on the UI thread. Empty errorText means submission
// succeeded, not physical printing. jobID may be empty for a Windows renderer.
func (b *PrintButton) SigPrintFinished(fn func(jobID, errorText string)) { b.finished = fn }

func (b *PrintButton) Print() {
	if b.printing || !b.IsEnabled() {
		return
	}
	b.printing = true
	b.showPrintDialog(b, b.documentPath, printing.Options{PrinterName: b.printerName, Copies: b.copies, Title: b.jobTitle}, func(job printing.Job, err error) {
		b.printing = false
		if b.finished != nil {
			message := ""
			if err != nil {
				message = err.Error()
			}
			b.finished(job.ID, message)
		}
	})
}

func (b *PrintButton) EnumProperties(list core.IPropertyList) {
	b.Button.EnumProperties(list)
	list.AddProperty("打印文档", b.DocumentPath, b.SetDocumentPath)
	list.AddProperty("打印机", b.PrinterName, b.SetPrinterName)
	list.AddProperty("份数", b.Copies, b.SetCopies)
	list.AddProperty("任务标题", b.JobTitle, b.SetJobTitle)
}
