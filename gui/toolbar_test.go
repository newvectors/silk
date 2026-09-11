package gui

import (
	"github.com/uk0/silk/paint"
	"math"
	"testing"
)

// toolBarLabels are one glyph each on purpose: the text-only button formula is
// text width + margins, so short labels put several buttons inside the narrow
// widths these tests lay out at. Wider labels would push every item into the
// overflow at 120 px and the prefix/suffix assertions would stop distinguishing
// a working split from "everything disappeared".
var toolBarLabels = []string{"A", "B", "C", "D", "E", "F"}

// newLabelToolBar builds a toolbar of text-only buttons and a counter per
// button, incremented by that button's own action.
func newLabelToolBar() (*ToolBar, []int) {
	tb := NewToolBar()
	fired := make([]int, len(toolBarLabels))
	for i, s := range toolBarLabels {
		idx := i
		tb.AddAction(s, nil, func() { fired[idx]++ })
	}
	return tb, fired
}

// visibleToolBarTexts returns the texts of the items the layout actually placed,
// in order.
func visibleToolBarTexts(tb *ToolBar) []string {
	var out []string
	for _, item := range tb.Items() {
		if item.IsVisible() {
			out = append(out, item.(*Button).Text())
		}
	}
	return out
}

// overflowToolBarTexts returns the texts of the items handed to the chevron.
func overflowToolBarTexts(tb *ToolBar) []string {
	var out []string
	for _, item := range tb.OverflowItems() {
		out = append(out, item.(*Button).Text())
	}
	return out
}

// toolBarChevronShowing reports whether the chevron is on the bar. It reads the
// field instead of calling overflowButton(), which builds the button on demand:
// on a bar that never overflowed there is nothing to build, and a freshly built
// button answers "visible" to a question about a chevron the user never saw.
func toolBarChevronShowing(tb *ToolBar) bool {
	return tb.overflowBtn != nil && tb.overflowBtn.IsVisible()
}

// toolBarFitWidth returns the narrowest whole-pixel width that holds the whole
// run: the right edge the layout gave the last item, plus the bar's own right
// margin, rounded up. It is measured from a placement rather than recomputed
// from the button formula because the run is as wide as the font makes it, and
// a hard-coded number would quietly become a slack fixture on the next OS.
func toolBarFitWidth(t *testing.T, tb *ToolBar) float64 {
	t.Helper()
	tb.SetBounds(0, 0, 4000, 32)
	tb.Layout()
	if got := overflowToolBarTexts(tb); len(got) != 0 {
		t.Fatalf("a 4000 px bar overflowed %v; the run cannot be measured", got)
	}
	items := tb.Items()
	x, _, w, _ := items[len(items)-1].Bounds()
	return math.Ceil(x + w + Theme().ButtonMargin.R)
}

func sameTexts(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestToolBarWideLayoutPlacesEveryItem pins the case the overflow must not
// disturb: when the run fits, nothing is hidden, nothing is reordered, and no
// chevron eats space. A split computed from the wrong budget — or one that
// always reserves the chevron — shows up here as a missing last item.
func TestToolBarWideLayoutPlacesEveryItem(t *testing.T) {
	tb, _ := newLabelToolBar()
	tb.SetBounds(0, 0, 1000, 32)
	tb.Layout()

	if got := len(tb.OverflowItems()); got != 0 {
		t.Errorf("1000 px bar overflowed %d item(s) (%v); every item fits and the chevron should not appear",
			got, overflowToolBarTexts(tb))
	}
	if got := visibleToolBarTexts(tb); !sameTexts(got, toolBarLabels) {
		t.Errorf("visible items = %v, want all of %v", got, toolBarLabels)
	}
	prev := -1.0
	for i, item := range tb.Items() {
		x, _, _, _ := item.Bounds()
		if x <= prev {
			t.Errorf("item %d (%q) placed at x=%v, which is not right of the previous item's x=%v",
				i, item.(*Button).Text(), x, prev)
		}
		prev = x
	}
}

// TestToolBarFitWidthPlacesEveryItem stands where the 1000 px fixture cannot:
// on a bar exactly as wide as the run instead of one with some 700 px to spare.
// Only here does the pre-check earn its place — delete the run total and the
// `total > avail` test in front of the fit loop, a tidy-up that reads as
// harmless because the loop appears to measure the same thing, and the loop's
// `- ovW - spacing` reservation comes out of a budget with nothing to give, so
// a toolbar that fits loses its tail to a chevron nobody needed.
func TestToolBarFitWidthPlacesEveryItem(t *testing.T) {
	tb, _ := newLabelToolBar()
	fitW := toolBarFitWidth(t, tb)

	// Overflow first: with the chevron already on the bar, "no chevron" below
	// means it was taken away at this width, not that it was never built.
	tb.SetBounds(0, 0, 120, 32)
	tb.Layout()
	if !toolBarChevronShowing(tb) {
		t.Fatalf("120 px bar shows no chevron (visible %v); there is nothing for the fit width to take away",
			visibleToolBarTexts(tb))
	}

	tb.SetBounds(0, 0, fitW, 32)
	tb.Layout()

	if got := visibleToolBarTexts(tb); !sameTexts(got, toolBarLabels) {
		t.Errorf("at %v px, the width of the run itself, visible items = %v, want all of %v",
			fitW, got, toolBarLabels)
	}
	if got := overflowToolBarTexts(tb); len(got) != 0 {
		t.Errorf("at %v px, the width of the run itself, overflow = %v, want nothing held back",
			fitW, got)
	}
	if toolBarChevronShowing(tb) {
		t.Errorf("at %v px the chevron is still shown; it is taking space on a bar the whole run fits in", fitW)
	}
}

// TestToolBarOnePixelUnderTheFitWidthCutsTheRun is the far side of that
// boundary, and it is what keeps the test above from being a slack fixture: were
// the fit width even a pixel generous, this bar would place everything too and
// neither test would be standing on the boundary. A pixel short, the budget must
// fail — and failing it costs far more than the pixel, because the chevron and
// its spacing come off the end, so the tail moves into the menu.
func TestToolBarOnePixelUnderTheFitWidthCutsTheRun(t *testing.T) {
	tb, _ := newLabelToolBar()
	narrow := toolBarFitWidth(t, tb) - 1

	tb.SetBounds(0, 0, narrow, 32)
	tb.Layout()

	vis := visibleToolBarTexts(tb)
	if sameTexts(vis, toolBarLabels) {
		t.Fatalf("at %v px, one pixel short of the run, every item was still placed (%v); the run does not fit this budget",
			narrow, vis)
	}
	if !sameTexts(vis, toolBarLabels[:len(vis)]) {
		t.Fatalf("at %v px visible items = %v, want the leading items of %v in order",
			narrow, vis, toolBarLabels)
	}
	if got, want := overflowToolBarTexts(tb), toolBarLabels[len(vis):]; !sameTexts(got, want) {
		t.Errorf("at %v px overflow = %v, want exactly the items that were not placed, %v",
			narrow, got, want)
	}
	if !toolBarChevronShowing(tb) {
		t.Errorf("at %v px the bar held back %v but shows no chevron; those actions cannot be reached",
			narrow, overflowToolBarTexts(tb))
	}
}

// TestToolBarNarrowLayoutCutsTheRunAndReservesTheChevron is the bug itself: at
// 120 px the trailing actions used to be placed past the widget's right edge —
// invisible, unclickable, unreachable. It also pins the reservation: without
// `- ovW - spacing` in the budget the last placed item runs under the chevron
// and covers the only way to reach the ones that are hidden.
func TestToolBarNarrowLayoutCutsTheRunAndReservesTheChevron(t *testing.T) {
	tb, _ := newLabelToolBar()
	tb.SetBounds(0, 0, 120, 32)
	tb.Layout()

	vis := visibleToolBarTexts(tb)
	if len(vis) == 0 {
		t.Fatalf("120 px bar placed nothing; the fixture no longer exercises a split")
	}
	if !sameTexts(vis, toolBarLabels[:len(vis)]) {
		t.Fatalf("visible items = %v, want the first %d of %v; the layout dropped or reordered items",
			vis, len(vis), toolBarLabels)
	}
	if got, want := overflowToolBarTexts(tb), toolBarLabels[len(vis):]; !sameTexts(got, want) {
		t.Errorf("overflow items = %v, want exactly the items that were not placed, %v", got, want)
	}
	for _, item := range tb.OverflowItems() {
		if item.IsVisible() {
			t.Errorf("overflowed item %q is still visible; it would draw at its stale position",
				item.(*Button).Text())
		}
	}

	chevron := tb.overflowButton()
	if !chevron.IsVisible() {
		t.Fatal("items overflowed but the chevron is hidden; the hidden actions cannot be reached")
	}
	cx, _, cw, _ := chevron.Bounds()
	if cx+cw > 120 {
		t.Errorf("chevron ends at x=%v, past the 120 px bar; it is clipped", cx+cw)
	}
	last := tb.Items()[len(vis)-1]
	lx, _, lw, _ := last.Bounds()
	if lx+lw > cx {
		t.Errorf("last placed item %q ends at x=%v but the chevron starts at x=%v; the chevron sits on top of it",
			last.(*Button).Text(), lx+lw, cx)
	}
}

// TestToolBarOverflowShrinksAsTheBarGrows lays the same toolbar out at two
// widths, because one width cannot tell a width-driven split from a constant
// one: a layout that hides everything, or hides a fixed count, satisfies a
// single narrow fixture. 200 px must place strictly more than 120 px does and
// still hold something back.
func TestToolBarOverflowShrinksAsTheBarGrows(t *testing.T) {
	tb, _ := newLabelToolBar()

	tb.SetBounds(0, 0, 120, 32)
	tb.Layout()
	visNarrow := visibleToolBarTexts(tb)
	ovNarrow := overflowToolBarTexts(tb)
	if !sameTexts(visNarrow, toolBarLabels[:len(visNarrow)]) {
		t.Fatalf("at 120 px visible items = %v, want a prefix of %v", visNarrow, toolBarLabels)
	}

	tb.SetBounds(0, 0, 200, 32)
	tb.Layout()
	visWide := visibleToolBarTexts(tb)
	ovWide := overflowToolBarTexts(tb)
	if !sameTexts(visWide, toolBarLabels[:len(visWide)]) {
		t.Fatalf("at 200 px visible items = %v, want a prefix of %v", visWide, toolBarLabels)
	}
	if !sameTexts(ovWide, toolBarLabels[len(visWide):]) {
		t.Errorf("at 200 px overflow = %v, want the unplaced suffix %v", ovWide, toolBarLabels[len(visWide):])
	}
	if len(ovWide) == 0 {
		t.Fatalf("at 200 px nothing overflowed (visible %v); the fixture no longer exercises a split", visWide)
	}
	if len(ovWide) >= len(ovNarrow) {
		t.Errorf("overflow at 200 px = %v (%d items), at 120 px = %v (%d items); a wider bar must place strictly more",
			ovWide, len(ovWide), ovNarrow, len(ovNarrow))
	}
}

// TestToolBarWideningUnhidesEveryItem covers the other half of the split: the
// items were hidden by a previous layout, so a re-layout that only places the
// prefix leaves the rest hidden forever — the window is wide again and half the
// toolbar is still blank.
func TestToolBarWideningUnhidesEveryItem(t *testing.T) {
	tb, _ := newLabelToolBar()
	tb.SetBounds(0, 0, 120, 32)
	tb.Layout()
	if len(tb.OverflowItems()) == 0 {
		t.Fatalf("120 px bar overflowed nothing; there is nothing to bring back")
	}

	tb.SetBounds(0, 0, 1000, 32)
	tb.Layout()
	if got := len(tb.OverflowItems()); got != 0 {
		t.Errorf("after widening, %d item(s) are still in the overflow: %v", got, overflowToolBarTexts(tb))
	}
	if got := visibleToolBarTexts(tb); !sameTexts(got, toolBarLabels) {
		t.Errorf("after widening, visible items = %v, want all of %v; hidden items were never shown again",
			got, toolBarLabels)
	}
	if tb.overflowButton().IsVisible() {
		t.Error("the chevron is still visible on a bar where everything fits")
	}
}

// TestToolBarChevronMenuTriggersTheOriginalActions drives the chevron the way a
// user does — the button's own key activation, through Button.emit and the
// binding installed by overflowButton — and inspects the menu that reached the
// popup step. The menu is what the hidden actions are reached by, so both its
// payload (one entry per hidden button, same order) and its wiring (the entry
// runs the ORIGINAL button's action) are the point; an entry carrying the right
// label and a dead handler is the failure this catches.
func TestToolBarChevronMenuTriggersTheOriginalActions(t *testing.T) {
	tb, fired := newLabelToolBar()
	tb.SetBounds(0, 0, 120, 32)
	tb.Layout()

	hidden := overflowToolBarTexts(tb)
	if len(hidden) < 2 {
		t.Fatalf("120 px bar overflowed %v; the menu assertions need at least two entries", hidden)
	}

	// Swap only the OS-window step: ShowAsPopup makes an NSWindow, which a test
	// goroutine may not do. Everything above it — the action binding, the menu
	// build, the positioning — is the production code path.
	var shown *Menu
	saved := showToolBarPopup
	showToolBarPopup = func(menu *Menu, xg, yg float64) {
		menu.setVisible(true) // Model the successful native Show step too.
		shown = menu
	}
	t.Cleanup(func() { showToolBarPopup = saved })

	tb.overflowButton().OnKeyDown(KeyEnter, false)

	if shown == nil {
		t.Fatal("activating the chevron showed no menu; the hidden actions are unreachable")
	}
	var entries []string
	for _, item := range shown.Items() {
		entries = append(entries, item.(*Button).Text())
	}
	if !sameTexts(entries, hidden) {
		t.Fatalf("chevron menu entries = %v, want one per hidden item in order: %v", entries, hidden)
	}

	// The second entry, not the first: an off-by-one in the binding — or a
	// handler bound to whichever button the loop ended on — survives triggering
	// entry zero.
	target := hidden[1]
	want := -1
	for i, s := range toolBarLabels {
		if s == target {
			want = i
		}
	}
	shown.Items()[1].(*Button).OnKeyDown(KeyEnter, false)
	if fired[want] != 1 {
		t.Errorf("menu entry %q fired the original button's action %d time(s), want 1", target, fired[want])
	}
	for i, n := range fired {
		if i != want && n != 0 {
			t.Errorf("menu entry %q also fired %q's action %d time(s)", target, toolBarLabels[i], n)
		}
	}
}

func TestToolBarMeasuresOnlyVisibleButtonText(t *testing.T) {
	tb := NewToolBar()
	icon := LoadIcon("document-save")
	iconOnly := tb.AddAction("", icon, func() {})
	hiddenText := tb.AddAction("Very long descriptive action which belongs only in its tooltip", icon, func() {})
	if hiddenText.IsTextVisible() {
		t.Fatal("fixture did not hide action text")
	}
	short, hidden := tb.itemSizeHints(iconOnly), tb.itemSizeHints(hiddenText)
	if short.Width != hidden.Width || short.Height != hidden.Height {
		t.Fatalf("hidden label consumes space: icon-only %+v, hidden-text %+v", short, hidden)
	}
	hiddenText.SetTextVisible(true)
	visible := tb.itemSizeHints(hiddenText)
	if visible.Width <= hidden.Width {
		t.Fatal("making the label visible did not change the width")
	}
	hints := tb.SizeHints()
	tb.SetSize(hints.Width, hints.Height)
	tb.Layout()
	if len(tb.OverflowItems()) != 0 {
		t.Fatal("the size advertised by SizeHints overflows")
	}
	last := tb.Items()[1]
	if got := last.X() + last.Width() + Theme().ButtonMargin.R; math.Abs(got-hints.Width) > 1e-9 {
		t.Fatalf("layout width %g differs from hints %g", got, hints.Width)
	}
}

func TestToolBarSizeHintsContainTallTextAndMixedItems(t *testing.T) {
	old := Theme().Font
	Theme().Font = paint.NewFont(old.Family(), 30, false, false)
	t.Cleanup(func() { Theme().Font = old })
	tb := NewToolBar()
	tb.AddAction("Design mode", nil, func() {})
	both := tb.AddAction("Preview", LoadIcon("preview"), func() {})
	both.SetTextVisible(true)
	tb.AddSeparator()
	tb.AddAction("", LoadIcon("document-save"), func() {})
	hints := tb.SizeHints()
	tb.SetSize(hints.Width, hints.Height)
	tb.Layout()
	if len(tb.OverflowItems()) != 0 {
		t.Fatal("mixed-content preferred width overflows")
	}
	if hints.Height < Theme().Font.FontExtents().Height+Theme().ButtonMargin.T+Theme().ButtonMargin.B {
		t.Fatalf("font height was omitted from toolbar hints: %+v", hints)
	}
	for _, item := range tb.Items() {
		if item.Y() < 0 || item.Y()+item.Height() > hints.Height+1e-9 {
			t.Fatalf("item %T covers the neighboring dock: bounds %v, bar height %g", item, item.Bounds1(), hints.Height)
		}
	}
}

func TestToolBarConstrainedHeightNeverEscapesTheBar(t *testing.T) {
	tb := NewToolBar()
	tb.AddAction("Design", nil, func() {})
	tb.AddSeparator()
	tb.AddWidget(NewEdit())
	for _, height := range []float64{0, 1, 4, 9, 22} {
		tb.SetSize(1000, height)
		tb.Layout()
		for _, item := range tb.Items() {
			if item.Y() < 0 || item.Height() < 0 || item.Y()+item.Height() > height+1e-9 {
				t.Fatalf("%T overflows height %g: %v", item, height, item.Bounds1())
			}
		}
	}
}

func TestToolBarChevronFitsAnExtremelyNarrowBar(t *testing.T) {
	tb, _ := newLabelToolBar()
	for _, size := range [][2]float64{{0, 0}, {1, 1}, {3, 2}, {10, 6}, {20, 12}} {
		tb.SetSize(size[0], size[1])
		tb.Layout()
		b := tb.overflowButton()
		if b.X() < 0 || b.Y() < 0 || b.Width() < 0 || b.Height() < 0 || b.X()+b.Width() > size[0] || b.Y()+b.Height() > size[1] {
			t.Fatalf("chevron escapes %v toolbar: %v", size, b.Bounds1())
		}
		if GetToolTip(b) != "更多工具" {
			t.Fatal("chevron has no readable Chinese tooltip")
		}
	}
}

func TestToolBarVerticalSwitchRestoresHorizontalOverflow(t *testing.T) {
	tb, _ := newLabelToolBar()
	tb.SetSize(120, 32)
	tb.Layout()
	if len(tb.OverflowItems()) == 0 {
		t.Fatal("fixture never overflowed horizontally")
	}
	tb.SetOrientation(1)
	hints := tb.SizeHints()
	tb.SetSize(hints.Width, hints.Height)
	tb.Layout()
	if len(tb.OverflowItems()) != 0 || toolBarChevronShowing(tb) {
		t.Fatal("horizontal overflow state survived the orientation change")
	}
	for _, item := range tb.Items() {
		if !item.IsVisible() || item.X() < 0 || item.X()+item.Width() > hints.Width || item.Y()+item.Height() > hints.Height {
			t.Fatalf("vertical item remains hidden or exceeds hints: %v", item.Bounds1())
		}
	}
	last := tb.Items()[len(tb.Items())-1]
	if got := last.Y() + last.Height() + Theme().ButtonMargin.B; math.Abs(got-hints.Height) > 1e-9 {
		t.Fatalf("vertical layout %g differs from hints %g", got, hints.Height)
	}
}

func TestToolBarOverflowKeepsNameStateAndLateDisableGuard(t *testing.T) {
	tb := NewToolBar()
	fired := 0
	source := tb.AddAction("", LoadIcon("document-save"), func() { fired++ })
	SetToolTip(source, "保存设计 (⌘S)")
	source.Action().SetChecked(true)
	tb.SetSize(20, 32)
	tb.Layout()
	var shown *Menu
	old := showToolBarPopup
	showToolBarPopup = func(m *Menu, _, _ float64) {
		m.setVisible(true)
		shown = m
	}
	t.Cleanup(func() { showToolBarPopup = old })
	tb.showOverflow()
	entry := shown.Items()[0].(*Button)
	if entry.Text() != GetToolTip(source) || !entry.IsChecked() || !entry.IsEnabled() {
		t.Fatal("overflow lost icon-only action's name or state")
	}
	source.Action().SetChecked(false)
	source.Action().SetEnabled(false)
	if entry.IsChecked() || entry.IsEnabled() {
		t.Fatal("open menu kept stale original action state")
	}
	// The proxy must preserve both action- and original widget-level
	// availability even for an already queued menu activation.
	entry.Action().Trigger(entry)
	if fired != 0 {
		t.Fatal("stale overflow activation triggered a disabled action")
	}
	source.Action().SetEnabled(true)
	source.SetEnabled(false)
	entry.Action().Trigger(entry)
	if fired != 0 || entry.IsEnabled() {
		t.Fatal("overflow ignored the button-level disabled state")
	}
	source.SetEnabled(true)
	entry.OnKeyDown(KeyEnter, false)
	if fired != 1 || source.Text() != "" {
		t.Fatal("overflow trigger did not reach the original action exactly once, or rewrote its text")
	}
}

func TestToolBarNormalizesInvalidSizingOptions(t *testing.T) {
	tb := NewToolBar()
	tb.SetIconSize(-20)
	if tb.IconSize() != 1 {
		t.Fatal("negative icon size was not clamped")
	}
	for _, spacing := range []float64{-5, math.NaN(), math.Inf(1)} {
		tb.SetSpacing(spacing)
		if tb.Spacing() != 0 {
			t.Fatalf("invalid spacing %g was accepted", spacing)
		}
	}
}
