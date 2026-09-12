package gui

import "github.com/uk0/silk/i18n"

// Explicit form geometry keeps errors and long filenames out of the button
// bar and remains bounded when the owner uses a larger UI font.
type printDialogBody struct {
	Widget
	labels       []*Label
	path, title  *Edit
	choose       *Button
	printer      *ComboBox
	copies       *SpinBox
	status, note *Label
}

func newPrintDialogBody() *printDialogBody {
	p := new(printDialogBody)
	p.Init(p)
	for _, s := range []string{"PDF document", "Printer", "Copies", "Job title"} {
		l := NewLabel(i18n.T(s))
		l.SetParent(p)
		p.labels = append(p.labels, l)
	}
	p.path = NewEdit()
	p.path.SetParent(p)
	p.title = NewEdit()
	p.title.SetParent(p)
	p.choose = NewButton1(i18n.T("Choose PDF..."), nil)
	p.choose.SetParent(p)
	p.printer = NewComboBox()
	p.printer.SetParent(p)
	p.copies = NewSpinBox()
	p.copies.SetRange(1, 99)
	p.copies.SetValue(1)
	p.copies.SetParent(p)
	p.status = NewLabel("")
	p.status.SetWrap(true)
	p.status.SetParent(p)
	p.note = NewLabel(i18n.T("Printer-default paper and tray; fit PDF to printable area. Submission does not confirm paper output."))
	p.note.SetWrap(true)
	p.note.SetParent(p)
	return p
}

func (p *printDialogBody) SizeHints() SizeHints { return SizeHints{Width: 532, Height: 262} }
func (p *printDialogBody) Layout() {
	w, h := p.Size()
	labelW := 100.0
	for _, l := range p.labels {
		labelW = max(labelW, l.SizeHints().Width+12)
	}
	labelW = min(labelW, w*0.42)
	fieldW := max(0, w-labelW)
	row := max(34, p.path.SizeHints().Height+6)
	for i, l := range p.labels {
		l.SetBounds(0, float64(i)*row, max(0, labelW-8), row-6)
	}
	browseW := min(fieldW, max(96, p.choose.SizeHints().Width))
	p.path.SetBounds(labelW, 0, max(0, fieldW-browseW-8), row-6)
	p.choose.SetBounds(max(labelW, w-browseW), 0, browseW, row-6)
	p.printer.SetBounds(labelW, row, fieldW, row-6)
	p.copies.SetBounds(labelW, 2*row, min(120, fieldW), row-6)
	p.title.SetBounds(labelW, 3*row, fieldW, row-6)
	y := 4*row + 6
	remaining := max(0, h-y)
	p.status.SetBounds(0, y, w, remaining*0.56)
	p.note.SetBounds(0, y+remaining*0.56, w, remaining*0.44)
}
