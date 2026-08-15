package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/uk0/silk/paint"
)

// Widgets built by the core factory — every widget the designer drops and
// every widget the .tdoc loader restores — get Init and nothing else. These
// tests drive the consequence of each geometry / format constant that used to
// live in a New* constructor: a section that can be opened, rows that get
// drawn, a field that clears its label, a readout that reads as a number.
// Asserting the constant itself (headerH == 32) would pass a "fix" that set
// the field and left the hit test reading zero.

// TestFactoryAccordionCanOpenASection: with headerH at 0 every header is a
// zero-height strip, hitTestHeader never matches, and the panel is inert —
// no click anywhere can open a section.
func TestFactoryAccordionCanOpenASection(t *testing.T) {
	a := factoryWidget(t, "gui.Accordion").(*Accordion)
	a.SetSize(200, 300)
	a.AddSection("One", NewLabel("c1"))
	a.AddSection("Two", NewLabel("c2"))
	a.AddSection("Three", NewLabel("c3"))

	// AddSection expands the first section; collapse it so the three headers
	// tile the top of the widget with no content between them.
	a.ToggleSection(0)
	if flags := expandedFlags(a); flags[0] {
		t.Fatalf("setup: ToggleSection(0) left the first section expanded (%v)", flags)
	}

	// A click 10 px down the panel lands in the first header of any sane
	// header height.
	a.OnLeftDown(20, 10)
	if !a.sections[0].Expanded {
		t.Fatalf("clicking 10 px into the first header opened nothing (expanded = %v); "+
			"hitTestHeader(10) = %d, so no section can be opened by mouse at all",
			expandedFlags(a), a.hitTestHeader(10))
	}

	// And the second header must be reachable too — that click has to cross a
	// whole header's worth of pixels to get there.
	y2 := a.headerH + a.sections[0].Content.SizeHints().Height + a.headerH*0.5
	a.OnLeftDown(20, y2)
	if !a.sections[1].Expanded {
		t.Errorf("clicking the second header at y=%v opened nothing (expanded = %v); hitTestHeader(%v) = %d",
			y2, expandedFlags(a), y2, a.hitTestHeader(y2))
	}
}

// TestFactoryVirtualListDrawsRows: Draw and hitTest both bail on
// itemHeight <= 0, so a factory list is permanently blank and unclickable
// however many items it is given.
func TestFactoryVirtualListDrawsRows(t *testing.T) {
	const viewportH = 120.0
	vl := factoryWidget(t, "gui.VirtualList").(*VirtualList)
	vl.SetSize(200, viewportH)
	vl.SetItemCount(50)
	drawn := 0
	painted := 0.0 // bottom edge of the lowest row drawn
	vl.SetItemDrawer(func(g paint.Painter, index int, x, y, w, h float64) {
		drawn++
		if y+h > painted {
			painted = y + h
		}
	})

	vl.Draw(newInkRecorder())
	if drawn == 0 {
		t.Fatalf("a %v px viewport over 50 items drew no rows — the list renders nothing (itemHeight = %v)",
			viewportH, vl.ItemHeight())
	}
	if painted < viewportH {
		t.Fatalf("%d row(s) cover the top %v px of a %v px viewport over 50 items — the rest stays blank",
			drawn, painted, viewportH)
	}

	// The rows that were drawn must also be clickable: same itemHeight drives
	// the hit test.
	if got := vl.hitTest(1); got != 0 {
		t.Errorf("hitTest(1) = %d, want row 0 — a click on the first drawn row selects nothing", got)
	}
}

// TestFactoryFormLayoutFieldClearsItsLabel: a zero labelWidth starts the
// field column at x=0, right on top of the label text the row draws there.
func TestFactoryFormLayoutFieldClearsItsLabel(t *testing.T) {
	fl := factoryWidget(t, "gui.FormLayout").(*FormLayout)
	fl.SetSize(300, 100)
	const labelText = "用户名"
	fl.AddRow(labelText, NewEdit())
	fl.Layout()

	lx, _, lw, _ := fl.rows[0].label.Bounds()
	fx, _, _, _ := fl.rows[0].widget.Bounds()
	textW := Theme().Font.TextExtents(labelText).Width

	if fx < textW {
		t.Errorf("field starts at x=%v but the label text %q is %v px wide — the field paints over its own label",
			fx, labelText, textW)
	}
	if fx < lx+lw {
		t.Errorf("field box starts at x=%v, inside the label box [%v,%v)", fx, lx, lx+lw)
	}
}

// TestFactoryToolBarIconButtonFitsItsIcon: icon buttons are sized as
// iconSize + margins, so a zero iconSize leaves a content box of 4 px for an
// icon the theme still draws at Theme().IconSize.
func TestFactoryToolBarIconButtonFitsItsIcon(t *testing.T) {
	tb := factoryWidget(t, "gui.ToolBar").(*ToolBar)
	tb.AddAction("", paint.LoadIcon("build"), func() {})
	tb.SetBounds(0, 0, 400, 48)
	tb.Layout()

	_, _, w, h := tb.Items()[0].Bounds()
	m := Theme().ButtonMargin
	iconW := w - m.L - m.R
	iconH := h - m.T - m.B
	want := Theme().IconSize
	if iconW < want || iconH < want {
		t.Errorf("icon-only button is %vx%v, leaving a %vx%v content box inside its margins for a %v px icon",
			w, h, iconW, iconH, want)
	}
}

// TestFactoryDigitalDisplayTextIsANumber: with format "" the readout prints
// Sprintf's complaint instead of the value, and none of those characters are
// in segTable so the panel renders as blanks.
func TestFactoryDigitalDisplayTextIsANumber(t *testing.T) {
	d := factoryWidget(t, "gui.DigitalDisplay").(*DigitalDisplay)
	d.SetValue(12.5)

	got := d.Text()
	if strings.Contains(got, "%!") {
		t.Fatalf("Text() = %q — the readout shows a formatting error, not a value (format %q)", got, d.Format())
	}
	if got != "12.5" {
		t.Errorf("Text() for value 12.5 = %q, want \"12.5\"", got)
	}
}

// TestFactoryBreadcrumbDrawsItsSeparator: an empty separator runs the trail
// together as "home  docs" and collapses the gaps the hit test measures.
func TestFactoryBreadcrumbDrawsItsSeparator(t *testing.T) {
	b := factoryWidget(t, "gui.Breadcrumb").(*Breadcrumb)
	b.SetSize(300, 24)
	b.AddItem("home", nil)
	b.AddItem("docs", nil)

	rec := newInkRecorder()
	b.Draw(rec)

	sep := b.Separator()
	want := []string{"home", sep, "docs"}
	if len(rec.texts) != 3 || rec.texts[0] != want[0] || rec.texts[1] != want[1] || rec.texts[2] != want[2] {
		t.Fatalf("the trail drew %v, want %v — nothing separates the two crumbs (separator %q)",
			rec.texts, want, sep)
	}
}

// TestFactoryPaginationReachesTheLastPage: total/sibling/boundary at 0 leave
// the pager with prev, the current page and next — the pinned last page is
// never rendered, so there is no cell to click to get there.
func TestFactoryPaginationReachesTheLastPage(t *testing.T) {
	pg := factoryWidget(t, "gui.Pagination").(*Pagination)
	pg.SetSize(240, 30)
	pg.SetTotalPages(10)

	// Cells left to right: prev, 1, 2, …, 10, next. The pinned last page is
	// the fifth cell.
	x := 4*paginationCellW + paginationCellW*0.5
	pg.OnLeftDown(x, 15)
	if pg.CurrentPage() != 10 {
		t.Errorf("clicking the last-page cell at x=%v left CurrentPage at %d; the pager laid out %d cells, "+
			"so page %d is not on screen to click",
			x, pg.CurrentPage(), len(pg.cells()), pg.TotalPages())
	}
}

// TestFactoryColorPickerSwatchIsClickable: an empty palette draws no swatches,
// so paletteHitTestIndex has nothing to return and the picker cannot pick.
func TestFactoryColorPickerSwatchIsClickable(t *testing.T) {
	cp := factoryWidget(t, "gui.ColorPicker").(*ColorPicker)
	cp.SetSize(320, 26)
	before := cp.Color()

	x0, y0, cell := cp.paletteOrigin()
	cp.OnLeftDown(x0+cell*0.5, y0+cell*0.5)

	if cp.Color() == before {
		t.Fatalf("clicking the first swatch left the colour at %v — the palette holds %d swatches, "+
			"so there is nothing under the pointer", before, len(cp.Palette()))
	}
	if cp.Color().A == 0 {
		t.Errorf("picked colour %v is fully transparent", cp.Color())
	}
}

// TestFactoryIndustrialSymbolsArePainted: the SCADA symbols are filled with
// their state colour, so a zero-alpha colour paints the whole symbol
// invisible — only the hard-coded outline survives.
func TestFactoryIndustrialSymbolsArePainted(t *testing.T) {
	cases := []struct {
		name string
		draw func(rec *inkRecorder)
	}{
		{"gui.Valve", func(rec *inkRecorder) {
			v := factoryWidget(t, "gui.Valve").(*Valve)
			v.SetSize(60, 40)
			v.Draw(rec)
		}},
		{"gui.Pipe", func(rec *inkRecorder) {
			p := factoryWidget(t, "gui.Pipe").(*Pipe)
			p.SetSize(120, 24)
			p.Draw(rec)
		}},
		{"gui.Indicator", func(rec *inkRecorder) {
			i := factoryWidget(t, "gui.Indicator").(*Indicator)
			i.SetSize(32, 32)
			i.Draw(rec)
		}},
	}
	for _, c := range cases {
		rec := newInkRecorder()
		c.draw(rec)
		if len(rec.fills) == 0 {
			t.Errorf("%s drew no filled shape at all", c.name)
			continue
		}
		visible := false
		for _, col := range rec.fills {
			if col.A > 0 {
				visible = true
				break
			}
		}
		if !visible {
			t.Errorf("factory %s filled %d shape(s), every one of them with a transparent brush %v — "+
				"the symbol never appears on screen", c.name, len(rec.fills), rec.fills)
		}
	}
}

// TestFactoryIndustrialRangesAcceptReadings: min/max are the engineering
// range the value clamps and scales against, so a factory 0..0 range pins
// every reading to 0 — a tag binding drives the widget and nothing moves.
func TestFactoryIndustrialRangesAcceptReadings(t *testing.T) {
	tank := factoryWidget(t, "gui.Tank").(*Tank)
	tank.SetLevel(1)
	if got := tank.EngValue(); got != 100 {
		t.Errorf("a full factory Tank reports EngValue %v, want 100 — its range is %v..%v",
			got, tank.Min(), tank.Max())
	}

	th := factoryWidget(t, "gui.Thermometer").(*Thermometer)
	th.SetValue(50)
	if got := th.Value(); got != 50 {
		t.Errorf("Thermometer.SetValue(50) reads back %v — the reading is clamped away by the range", got)
	}

	vb := factoryWidget(t, "gui.ValueBar").(*ValueBar)
	vb.SetValue(50)
	if got := vb.Value(); got != 50 {
		t.Errorf("ValueBar.SetValue(50) reads back %v — the reading is clamped away by the range", got)
	}
}

// TestFactoryCalendarOpensOnTodaysMonth: a zero month puts the grid in
// January of year 1 — the header reads "1年1月" and no cell in the grid is a
// date any user is looking for.
func TestFactoryCalendarOpensOnTodaysMonth(t *testing.T) {
	c := factoryWidget(t, "gui.Calendar").(*Calendar)
	c.SetSize(240, 240)

	today := dayStart(time.Now())
	m := c.DisplayedMonth()
	grid := monthGrid(m.Year(), m.Month())
	row, col := -1, -1
	for r := range grid {
		for i := 0; i < 7; i++ {
			if grid[r][i].Equal(today) {
				row, col = r, i
			}
		}
	}
	if row < 0 {
		t.Fatalf("the grid shows %s — today (%s) is not on it, so today cannot be clicked",
			m.Format("2006-01"), today.Format("2006-01-02"))
	}

	x := calendarPad + (float64(col)+0.5)*calendarCellW
	y := calendarGridTop() + (float64(row)+0.5)*calendarCellH
	c.OnLeftDown(x, y)
	if got := c.SelectedDate(); !got.Equal(today) {
		t.Errorf("clicking today's cell selected %s, want %s", got.Format("2006-01-02"), today.Format("2006-01-02"))
	}
}

// TestFactoryCalendarHasNoPhantomHover: hoverRow/hoverCol at 0 instead of -1
// paint the top-left day cell hovered before the pointer has ever entered the
// widget. What it paints untouched must match what it paints once the pointer
// has left.
func TestFactoryCalendarHasNoPhantomHover(t *testing.T) {
	c := factoryWidget(t, "gui.Calendar").(*Calendar)
	c.SetSize(240, 240)

	untouched := newInkRecorder()
	c.Draw(untouched)

	c.OnMouseLeave()
	left := newInkRecorder()
	c.Draw(left)

	if len(untouched.fills) != len(left.fills) {
		t.Errorf("untouched the grid paints %d filled cell(s) %v, after the pointer leaves it paints %d %v — "+
			"the hover highlight is lit before the pointer ever arrives (hoverRow/hoverCol = %d/%d)",
			len(untouched.fills), untouched.fills, len(left.fills), left.fills, c.hoverRow, c.hoverCol)
	}
}

// TestFactoryDatePickerShowsARealDate: an unset date renders as 0000-00-00,
// and the popup opens on month 0 of year 0.
func TestFactoryDatePickerShowsARealDate(t *testing.T) {
	p := factoryWidget(t, "gui.DatePicker").(*DatePicker)

	now := time.Now()
	want := now.Format("2006-01-02")
	if got := p.displayText(); got != want {
		t.Errorf("the field reads %q, want today's %q", got, want)
	}
	if p.Month() < 1 || p.Month() > 12 || p.Day() < 1 {
		t.Errorf("date reads %04d-%02d-%02d — the popup would open on a month that does not exist",
			p.Year(), p.Month(), p.Day())
	}
	if got := p.Format(); got != "2006-01-02" {
		t.Errorf("Format() = %q, want \"2006-01-02\"", got)
	}
}

// TestFactoryTimelineLaysStepsVertically: vertical=false lays the steps out
// across the widget while SizeHints still asks for a tall narrow box, so the
// steps run straight out of it.
func TestFactoryTimelineLaysStepsVertically(t *testing.T) {
	tl := factoryWidget(t, "gui.Timeline").(*Timeline)
	tl.SetSize(200, 180)
	tl.AddItem("one", "", 0)
	tl.AddItem("two", "", 0)
	tl.AddItem("three", "", 0)

	rec := newInkRecorder()
	tl.Draw(rec)

	if len(rec.arcs) != 3 {
		t.Fatalf("3 steps drew %d markers", len(rec.arcs))
	}
	for i := 1; i < len(rec.arcs); i++ {
		if rec.arcs[i].y <= rec.arcs[i-1].y || rec.arcs[i].x != rec.arcs[i-1].x {
			t.Fatalf("step markers at %v run across the widget, not down it", rec.arcs)
		}
	}
}

// TestFactoryDiffViewDrawsTheGutter: showGutter=false suppresses the
// per-side line numbers and gives their reserved width back to the text.
func TestFactoryDiffViewDrawsTheGutter(t *testing.T) {
	dv := factoryWidget(t, "gui.DiffView").(*DiffView)
	dv.SetSize(480, 240)
	dv.SetTexts("A\nB", "A\nB")

	rec := newInkRecorder()
	dv.Draw(rec)

	numbered := false
	for _, td := range rec.textAt {
		if td.s == "1" && td.x < diffGutterWidth {
			numbered = true
		}
	}
	if !numbered {
		t.Fatalf("no line number reached the left gutter; the view drew %v", rec.texts)
	}
	for _, td := range rec.textAt {
		if td.s == "A" && td.x < diffGutterWidth {
			t.Errorf("the diff text %q starts at x=%v, inside the %v px gutter — it would sit on the numbers",
				td.s, td.x, diffGutterWidth)
		}
	}
}

// TestFactorySpinnerDrawsAndCanBeArmed: Draw no-ops while !busy, so a factory
// spinner is a blank hole in the layout. Init sets the flag but deliberately
// leaves the heartbeat unarmed, which SetBusy(true) must still be able to do.
func TestFactorySpinnerDrawsAndCanBeArmed(t *testing.T) {
	sp := factoryWidget(t, "gui.Spinner").(*Spinner)
	defer sp.SetBusy(false)
	sp.SetSize(24, 24)

	rec := newInkRecorder()
	sp.Draw(rec)
	if len(rec.fills) == 0 {
		t.Fatalf("a factory spinner painted nothing (busy = %v, dots = %d)", sp.IsBusy(), sp.DotCount())
	}

	if sp.anim != nil {
		t.Fatalf("Init armed a repaint heartbeat; every reflect-built widget would carry one")
	}
	sp.SetBusy(true)
	if sp.anim == nil || sp.anim.State() != AnimRunning {
		t.Errorf("SetBusy(true) on a factory spinner left the heartbeat unarmed — it never repaints")
	}
}

// --- recorder ---

// inkText is one DrawText1 call: the string and where it landed.
type inkText struct {
	x, y float64
	s    string
}

// inkRecorder captures what a Draw actually put on the surface: the brush
// live at each fill, the arcs and the text. It embeds nopPainter
// (spinner_test.go) so an op none of these widgets used before panics here
// rather than passing silently.
type inkRecorder struct {
	paint.Painter
	brush  paint.Color
	fills  []paint.Color // brush colour at each Fill / FillPreserve
	arcs   []recArc
	texts  []string
	textAt []inkText
	state  int
}

func newInkRecorder() *inkRecorder {
	return &inkRecorder{Painter: nopPainter{}}
}

func (r *inkRecorder) Save() int         { r.state++; return r.state }
func (r *inkRecorder) Restore() int      { r.state--; return r.state }
func (r *inkRecorder) CurrentState() int { return r.state }

func (r *inkRecorder) Rectangle(x, y, w, h float64) {}
func (r *inkRecorder) Clip()                        {}
func (r *inkRecorder) MoveTo(x, y float64)          {}
func (r *inkRecorder) LineTo(x, y float64)          {}
func (r *inkRecorder) Arc(xc, yc, radius, a1, a2 float64) {
	r.arcs = append(r.arcs, recArc{xc, yc, radius})
}

// SetBrush covers the gradient fills (Tank's liquid, a lit Indicator); the
// pattern carries no single colour, so record it as opaque-unknown rather
// than leaving the previous solid colour standing.
func (r *inkRecorder) SetBrush(b paint.Brush)  { r.brush = paint.Color{A: 255} }
func (r *inkRecorder) SetBrush1(c paint.Color) { r.brush = c }

func (r *inkRecorder) SetPen1(c paint.Color, width float64) {}
func (r *inkRecorder) SetPen(p paint.Pen)                   {}
func (r *inkRecorder) Fill()                                { r.fills = append(r.fills, r.brush) }
func (r *inkRecorder) FillPreserve()                        { r.fills = append(r.fills, r.brush) }
func (r *inkRecorder) Stroke()                              {}
func (r *inkRecorder) SetFont(f paint.Font)                 {}
func (r *inkRecorder) Font() paint.Font                     { return Theme().Font }
func (r *inkRecorder) Translate(tx, ty float64)             {}
func (r *inkRecorder) DrawText(s string)                    { r.texts = append(r.texts, s) }
func (r *inkRecorder) DrawText1(x, y float64, s string) {
	r.texts = append(r.texts, s)
	r.textAt = append(r.textAt, inkText{x, y, s})
}
