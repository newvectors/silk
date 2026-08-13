package gui

import (
	"testing"

	"github.com/uk0/silk/geom"
	"github.com/uk0/silk/paint"
)

// These tests pin the two dark-mode row-contrast defects that shared one draw
// loop: the zebra stripe was a hardcoded near-white literal (a white band under
// zinc-200 text on a zinc-900 view), and a selected row filled with
// HighLightColor still drew its text in TextColor (zinc-200 on blue-400).
// silkide starts in dark mode, so both were the default experience.

// rowFill is one recorded fill: the brush colour at the moment Fill() ran plus
// the rectangle that was queued for it, so a fill can be identified by the row
// it covers rather than by hoping only one fill of that colour exists.
type rowFill struct {
	color      paint.Color
	x, y, w, h float64
}

// rowInkRecorder is a no-op paint.Painter (it embeds nopPainter, spinner_test.go)
// that records every fill and the brush in force at each DrawText1, so the real
// Table.Draw / TreeView.drawCellContent can be driven without a GLFW window or a
// render target. Only the methods those two draw paths reach are overridden.
type rowInkRecorder struct {
	paint.Painter
	brush paint.Color
	rect  geom.Rect
	fills []rowFill
	inks  []paint.Color
	font  paint.Font
}

func newRowInkRecorder() *rowInkRecorder {
	// drawCellContent reads g.Font().FontExtents() without setting a font
	// first, so the recorder must already carry one.
	return &rowInkRecorder{Painter: nopPainter{}, font: Theme().Font}
}

func (r *rowInkRecorder) Save() int    { return 0 }
func (r *rowInkRecorder) Restore() int { return 0 }
func (r *rowInkRecorder) Clip()        {}
func (r *rowInkRecorder) Rectangle(x, y, w, h float64) {
	r.rect = geom.Rect{x, y, w, h}
}
func (r *rowInkRecorder) Rectangle1(rect geom.Rect)                  { r.rect = rect }
func (r *rowInkRecorder) Translate(tx, ty float64)                   {}
func (r *rowInkRecorder) MoveTo(x, y float64)                        {}
func (r *rowInkRecorder) LineTo(x, y float64)                        {}
func (r *rowInkRecorder) Arc(xc, yc, radius, angle1, angle2 float64) {}
func (r *rowInkRecorder) Line(x1, y1, x2, y2 float64)                {}
func (r *rowInkRecorder) Stroke()                                    {}
func (r *rowInkRecorder) SetPen1(c paint.Color, w float64)           {}
func (r *rowInkRecorder) SetBrush1(c paint.Color)                    { r.brush = c }
func (r *rowInkRecorder) SetFont(f paint.Font)                       { r.font = f }
func (r *rowInkRecorder) Font() paint.Font                           { return r.font }
func (r *rowInkRecorder) DrawText1(x, y float64, text string)        { r.inks = append(r.inks, r.brush) }
func (r *rowInkRecorder) DrawIcon1(icon paint.Icon, x, y, size float64, gray bool) {
}

func (r *rowInkRecorder) Fill() {
	r.fills = append(r.fills, rowFill{r.brush, r.rect.X, r.rect.Y, r.rect.Width, r.rect.Height})
}
func (r *rowInkRecorder) FillPreserve() { r.Fill() }

// TestTableAltRowColorFollowsThemeSurface: the zebra stripe must be derived
// from the active theme's view surface, not fixed. A stripe that sits closer to
// TextColor than to ViewBGColor is a band the text disappears into.
func TestTableAltRowColorFollowsThemeSurface(t *testing.T) {
	orig := CurrentThemeMode()
	t.Cleanup(func() { SetThemeMode(orig) })

	got := map[ThemeMode]paint.Color{}
	for _, tc := range []struct {
		mode ThemeMode
		name string
	}{{ThemeLight, "light"}, {ThemeDark, "dark"}} {
		SetThemeMode(tc.mode)
		th := Theme()
		alt := tableAltRowColor(th)
		got[tc.mode] = alt

		dSurface := colorChannelDelta(alt, th.ViewBGColor)
		dInk := colorChannelDelta(alt, th.TextColor)
		if dSurface >= dInk {
			t.Errorf("%s: alt row %v is %d from ViewBGColor %v but %d from TextColor %v; "+
				"the stripe must stay a step off the surface, not drift toward the ink",
				tc.name, alt, dSurface, th.ViewBGColor, dInk, th.TextColor)
		}
		if alt == th.ViewBGColor {
			t.Errorf("%s: alt row %v equals ViewBGColor, so there is no stripe at all", tc.name, alt)
		}
		if alt.A != 255 {
			t.Errorf("%s: alt row %v is not opaque; the stripe paints over the view fill", tc.name, alt)
		}
	}

	if got[ThemeLight] == got[ThemeDark] {
		t.Errorf("alt row colour is %v in both modes: it is a constant, not derived from the theme",
			got[ThemeLight])
	}
}

// TestSelectionTextColorSwitchesWithSelection: text on a HighLightColor fill
// must not be the same ink used on the view surface.
func TestSelectionTextColorSwitchesWithSelection(t *testing.T) {
	orig := CurrentThemeMode()
	t.Cleanup(func() { SetThemeMode(orig) })

	for _, tc := range []struct {
		mode ThemeMode
		name string
	}{{ThemeLight, "light"}, {ThemeDark, "dark"}} {
		SetThemeMode(tc.mode)
		th := Theme()
		sel := selectionTextColor(th, true)
		un := selectionTextColor(th, false)
		if sel == un {
			t.Errorf("%s: selected and unselected ink are both %v; the highlight fill gets no ink of its own",
				tc.name, sel)
		}
		if un != th.TextColor {
			t.Errorf("%s: unselected ink = %v, want TextColor %v", tc.name, un, th.TextColor)
		}
	}

	SetThemeMode(ThemeDark)
	th := Theme()
	sel := selectionTextColor(th, true)
	dActive := colorChannelDelta(sel, th.MenuActiveTextColor)
	dText := colorChannelDelta(sel, th.TextColor)
	if dActive >= dText {
		t.Errorf("dark: selected ink %v is %d from MenuActiveTextColor %v but %d from TextColor %v; "+
			"ink on the accent fill must follow MenuActiveTextColor",
			sel, dActive, th.MenuActiveTextColor, dText, th.TextColor)
	}
}

// TestTreeViewDrawCellContentInkFollowsSelection drives the real
// drawCellContent twice under dark mode — once on a selected row, once not —
// and asserts the brush that was live when the text was drawn differs and
// matches selectionTextColor both times. TreeView fills a selected row with
// HighLightColor at tree-view.go:900, so the cell text below it must not stay
// on TextColor.
func TestTreeViewDrawCellContentInkFollowsSelection(t *testing.T) {
	orig := CurrentThemeMode()
	t.Cleanup(func() { SetThemeMode(orig) })
	SetThemeMode(ThemeDark)

	tv := newKbTree()
	row := tv.rows[0]

	inkFor := func(selected bool) paint.Color {
		row.selected = selected
		rec := newRowInkRecorder()
		tv.drawCellContent(rec, row, 0, 0, 0, 120, 20)
		if len(rec.inks) == 0 {
			t.Fatalf("selected=%v: drawCellContent drew no text", selected)
		}
		return rec.inks[0]
	}

	gotSel := inkFor(true)
	gotUn := inkFor(false)

	th := Theme()
	if gotSel == gotUn {
		t.Fatalf("dark: selected and unselected cell text both drawn in %v; "+
			"selected rows keep TextColor on the HighLightColor fill", gotSel)
	}
	if want := selectionTextColor(th, true); gotSel != want {
		t.Errorf("dark: selected cell ink = %v, want selectionTextColor(true) = %v", gotSel, want)
	}
	if want := selectionTextColor(th, false); gotUn != want {
		t.Errorf("dark: unselected cell ink = %v, want selectionTextColor(false) = %v", gotUn, want)
	}
}

// TestTableDrawAltRowFillIsThemeDerived drives the real Table.Draw under dark
// mode and inspects the fill that lands on the odd row. Before the fix that
// fill was {245,245,250,255} — a near-white band across a zinc-900 view.
func TestTableDrawAltRowFillIsThemeDerived(t *testing.T) {
	orig := CurrentThemeMode()
	t.Cleanup(func() { SetThemeMode(orig) })
	SetThemeMode(ThemeDark)

	tbl := NewTable()
	m := NewSimpleTableModel([]string{"name", "age"})
	m.AddRow("Alice", "25")
	m.AddRow("Bob", "40")
	tbl.SetModel(m)
	// Big enough for the header plus both rows with no scroll bars, so the
	// only fills Draw makes are the view background, the header cells and the
	// zebra stripe.
	tbl.SetBounds(0, 0, 400, 300)
	tbl.Layout()

	rec := newRowInkRecorder()
	tbl.Draw(rec)

	rh := tbl.RowHeight()
	// Row 1 is the odd row; its fill is queued at y = rh in the body's own
	// translated space, which the recorder sees untransformed.
	var altFills []paint.Color
	for _, f := range rec.fills {
		if f.y == rh && f.h == rh {
			altFills = append(altFills, f.color)
		}
	}
	if len(altFills) != 1 {
		t.Fatalf("expected exactly one fill over the odd row, got %d (all fills: %v)",
			len(altFills), rec.fills)
	}
	if want := tableAltRowColor(Theme()); altFills[0] != want {
		t.Errorf("dark: odd-row fill = %v, want tableAltRowColor = %v", altFills[0], want)
	}

	for _, f := range rec.fills {
		if f.color.R > 200 && f.color.G > 200 && f.color.B > 200 {
			t.Errorf("dark: Table.Draw filled %v at (%.0f,%.0f %.0fx%.0f) — a near-white band on a %v view",
				f.color, f.x, f.y, f.w, f.h, Theme().ViewBGColor)
		}
	}
}

// TestListWidgetDrawSelectedRowInkFollowsFill drives the real ListWidget.Draw
// under dark mode with row 1 selected. The fill and the text used to be decided
// by two separate expressions, so the highlighted row kept TextColor — zinc-200
// on blue-400. They now share one condition, and this asserts the result at the
// painter: the highlighted row's ink differs from the plain row's.
func TestListWidgetDrawSelectedRowInkFollowsFill(t *testing.T) {
	orig := CurrentThemeMode()
	t.Cleanup(func() { SetThemeMode(orig) })
	SetThemeMode(ThemeDark)

	lw := NewListWidget()
	lw.Append(ListItem{Text: "first"})
	lw.Append(ListItem{Text: "second"})
	lw.SetBounds(0, 0, 200, 200)
	lw.Layout()
	lw.setActiveIndex(1)

	rec := newRowInkRecorder()
	lw.Draw(rec)

	// Neither icons nor check boxes are visible, so the only text drawn is one
	// label per row, top to bottom: inks[0] is row 0, inks[1] the selected row.
	if len(rec.inks) != 2 {
		t.Fatalf("expected one text draw per row, got %d (%v)", len(rec.inks), rec.inks)
	}
	th := Theme()
	if want := selectionTextColor(th, false); rec.inks[0] != want {
		t.Errorf("dark: plain row ink = %v, want %v", rec.inks[0], want)
	}
	if want := selectionTextColor(th, true); rec.inks[1] != want {
		t.Errorf("dark: selected row ink = %v, want %v; the row is filled with HighLightColor %v",
			rec.inks[1], want, th.HighLightColor)
	}

	// And the row that got the highlight fill is the one that got the selected
	// ink — the fill and the ink must never disagree about which row it is.
	rh := lw.RowHeight()
	found := false
	for _, f := range rec.fills {
		if f.color == th.HighLightColor {
			if f.y != rh {
				t.Errorf("highlight fill landed at y=%.0f, want the selected row at y=%.0f", f.y, rh)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("no HighLightColor fill recorded; the selected row was never highlighted")
	}
}
