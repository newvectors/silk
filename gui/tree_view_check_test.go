package gui

import (
	"fmt"
	"testing"

	"github.com/uk0/silk/geom"
	"github.com/uk0/silk/paint"
)

// checkTreeModel reuses the keyboard fixture's model (tree_view_keyboard_test.go)
// and replaces Flags only: every row is user-checkable except the ones named in
// noCheck, which stand in for a model that opts a row out of the check column.
// Index / Parent / RowCount / Data stay the shared fixture, so the tree shape is
// the one the keyboard tests already pin.
type checkTreeModel struct {
	*kbTreeModel
	noCheck map[*kbTreeNode]bool
}

func (m *checkTreeModel) Flags(idx ModelIndex) ItemFlags {
	f := m.kbTreeModel.Flags(idx)
	if idx.IsNil() || m.noCheck[idx.Param.(*kbTreeNode)] {
		return f
	}
	return f | ItemIsUserCheckable
}

// checkTreeRows is the fixture's visible row order once A0 is expanded.
var checkTreeRows = []string{"A", "A0", "A0a", "A0b", "A1", "B"}

type checkTree struct {
	tv    *TreeView
	m     *checkTreeModel
	nodes map[string]*kbTreeNode
}

// newCheckTree builds the keyboard fixture's tree (A > [A0 > [A0a, A0b], A1], B),
// gives the view a size — the header only lays its sections out on a resize, and
// hit testing needs them — turns the check column on and expands A0, so every
// level of the hierarchy is visible: [A, A0, A0a, A0b, A1, B]. The row list is
// asserted here so every case below starts from a pinned fixture.
// noCheck names rows whose Flags omit ItemIsUserCheckable.
func newCheckTree(t *testing.T, noCheck ...string) *checkTree {
	t.Helper()

	nodes := map[string]*kbTreeNode{
		"A0a": leaf("A0a"),
		"A0b": leaf("A0b"),
		"A1":  leaf("A1"),
		"B":   leaf("B"),
	}
	nodes["A0"] = branch("A0", nodes["A0a"], nodes["A0b"])
	nodes["A"] = branch("A", nodes["A0"], nodes["A1"])

	m := &checkTreeModel{
		kbTreeModel: newKbTreeModel(nodes["A"], nodes["B"]),
		noCheck:     make(map[*kbTreeNode]bool),
	}
	for _, name := range noCheck {
		if nodes[name] == nil {
			t.Fatalf("newCheckTree: no fixture node named %q", name)
		}
		m.noCheck[nodes[name]] = true
	}

	tv := NewTreeView()
	tv.SetModel(m)
	tv.SetSize(400, 300)
	tv.SetCheckBoxVisible(true)

	f := &checkTree{tv: tv, m: m, nodes: nodes}
	tv.Expand(f.mi("A0"))

	if got := rowLabels(tv); !eqStrings(got, checkTreeRows) {
		t.Fatalf("fixture visible rows = %v, want %v", got, checkTreeRows)
	}
	return f
}

// mi returns the column-0 ModelIndex of a fixture node by label.
func (f *checkTree) mi(name string) ModelIndex {
	return f.m.col0Index(f.nodes[name])
}

func (f *checkTree) label(mi ModelIndex) string {
	s, _ := f.m.Data(mi, DisplayRole).(string)
	if s == "" {
		return "<none>"
	}
	return s
}

func (f *checkTree) state(name string) int {
	return f.tv.CheckState(f.mi(name))
}

// row returns the visible row object behind a fixture label.
func (f *checkTree) row(t *testing.T, name string) *TreeViewRow {
	t.Helper()
	row := f.tv.getRow(f.mi(name), false)
	if row == nil {
		t.Fatalf("row %q is not in the view", name)
	}
	return row
}

// Column 0 is split into three bands, left to right: the expander, the check
// box, then the icon+text content. Each band is one expanderSize() wide and they
// start after the row's own indent — the same two primitives the view uses, so
// the harness is not free to invent its own layout.
func (f *checkTree) expanderX(t *testing.T, name string) float64 {
	t.Helper()
	return f.tv.rowIndentPx(f.row(t, name))
}

func (f *checkTree) checkX(t *testing.T, name string) float64 {
	t.Helper()
	return f.tv.rowIndentPx(f.row(t, name)) + f.tv.expanderSize()
}

func (f *checkTree) contentX(t *testing.T, name string) float64 {
	t.Helper()
	return f.tv.rowIndentPx(f.row(t, name)) + 2*f.tv.expanderSize()
}

// clickAt sends a real left-button press to the view. x is measured from the
// left edge of column 0 in scrolled coordinates — the space hitTestScrolled
// works in — and is mapped back to widget coordinates through the production
// MapFromScrolled, so OnLeftDown runs its own MapToScrolled on the way in.
func (f *checkTree) clickAt(t *testing.T, name string, x float64) {
	t.Helper()
	row := f.row(t, name)
	sec := f.tv.header.LogicSection(0)
	y := row.ypos + 0.5*f.tv.rowHeight(row.ri)
	wx, wy := f.tv.MapFromScrolled(sec.Offset+x, y)
	f.tv.OnLeftDown(wx, wy)
}

// wantStates asserts the check state of every fixture row at once, so a change
// that leaks into a row the case does not mention is still caught. A row left
// out of want is asserted unchecked (TVC_UNCHECKED is the zero value).
func (f *checkTree) wantStates(t *testing.T, when string, want map[string]int) {
	t.Helper()
	for _, name := range checkTreeRows {
		if got := f.state(name); got != want[name] {
			t.Errorf("%s: CheckState(%s) = %s, want %s",
				when, name, checkStateName(got), checkStateName(want[name]))
		}
	}
}

func checkStateName(s int) string {
	switch s {
	case TVC_UNCHECKED:
		return "TVC_UNCHECKED"
	case TVC_CHECKED:
		return "TVC_CHECKED"
	case TVC_PARTIAL:
		return "TVC_PARTIAL"
	}
	return fmt.Sprintf("check state %d", s)
}

// TestTreeViewCheckFixture pins the fixture the rest of the file builds on: A0
// expanded, six visible rows in a known order, the check column on and every row
// starting unchecked.
func TestTreeViewCheckFixture(t *testing.T) {
	f := newCheckTree(t)

	if !f.tv.IsCheckBoxVisible() {
		t.Fatalf("SetCheckBoxVisible(true) did not stick: IsCheckBoxVisible() = false")
	}
	f.wantStates(t, "fresh tree", map[string]int{})
}

// TestTreeViewCheckBandsAreDisjoint drives three x positions on one row — the
// left edge of the expander band, of the check band and of the content band —
// and asserts each does its own job. It is what stops the check band being
// carved out of the expander's pixels (clicks stop expanding), overlapping it
// (clicks toggle instead of expanding), or swallowing the content area (rows
// stop selecting).
func TestTreeViewCheckBandsAreDisjoint(t *testing.T) {
	f := newCheckTree(t)

	var activated []string
	f.tv.SetActivatedCallback(func(o interface{}, mi ModelIndex) {
		activated = append(activated, f.label(mi))
	})

	// 1) the expander band still expands and collapses A0.
	f.clickAt(t, "A0", f.expanderX(t, "A0"))
	if got, want := rowLabels(f.tv), []string{"A", "A0", "A1", "B"}; !eqStrings(got, want) {
		t.Fatalf("click at the expander band: rows = %v, want %v (the band must still collapse)", got, want)
	}
	f.clickAt(t, "A0", f.expanderX(t, "A0"))
	if got := rowLabels(f.tv); !eqStrings(got, checkTreeRows) {
		t.Fatalf("second click at the expander band: rows = %v, want %v", got, checkTreeRows)
	}
	if f.state("A0") != TVC_UNCHECKED {
		t.Errorf("expanding/collapsing changed A0's check state to %s", checkStateName(f.state("A0")))
	}
	if f.tv.CurrentRow() != -1 {
		t.Errorf("click at the expander band moved the current row to %d, want -1", f.tv.CurrentRow())
	}

	// 2) the check band toggles without touching the current row or activating.
	f.clickAt(t, "A0", f.checkX(t, "A0"))
	if got := f.state("A0"); got != TVC_CHECKED {
		t.Fatalf("click at the check band: CheckState(A0) = %s, want TVC_CHECKED", checkStateName(got))
	}
	if got := rowLabels(f.tv); !eqStrings(got, checkTreeRows) {
		t.Errorf("click at the check band changed the layout: rows = %v, want %v", got, checkTreeRows)
	}
	if f.tv.CurrentRow() != -1 {
		t.Errorf("click at the check band moved the current row to %d, want -1 (unchanged)", f.tv.CurrentRow())
	}
	if len(activated) != 0 {
		t.Errorf("click at the check band activated %v, want no activation", activated)
	}

	// 3) the content band selects and activates the row, check state untouched.
	f.clickAt(t, "A0", f.contentX(t, "A0"))
	if got, want := f.tv.CurrentIndex(), f.mi("A0"); got != want {
		t.Errorf("click at the content band: current row = %q, want %q", f.label(got), f.label(want))
	}
	if want := []string{"A0"}; !eqStrings(activated, want) {
		t.Errorf("click at the content band: activations = %v, want %v", activated, want)
	}
	if got := f.state("A0"); got != TVC_CHECKED {
		t.Errorf("click at the content band changed CheckState(A0) to %s, want TVC_CHECKED (unchanged)",
			checkStateName(got))
	}
}

// TestTreeViewCheckMousePropagatesThroughHierarchy drives the mouse through the
// check band and pins the propagation in both directions:
//   - down: checking A reaches the grandchildren and stops at the sibling subtree
//     (a reversed direction, or one that stops at the direct children, is red);
//   - up: unchecking one grandchild makes both ancestors partial (recomputing a
//     parent as "any child checked -> checked" is red), and it does not drag the
//     sibling leaf along;
//   - and the partial chain resolves back to checked when the row returns.
func TestTreeViewCheckMousePropagatesThroughHierarchy(t *testing.T) {
	f := newCheckTree(t)

	// (a) A's check band: the whole A subtree follows, B does not.
	f.clickAt(t, "A", f.checkX(t, "A"))
	f.wantStates(t, "after checking A", map[string]int{
		"A":   TVC_CHECKED,
		"A0":  TVC_CHECKED,
		"A0a": TVC_CHECKED,
		"A0b": TVC_CHECKED,
		"A1":  TVC_CHECKED,
		"B":   TVC_UNCHECKED,
	})

	// (b) A0a's check band: only that leaf flips, both ancestors go partial.
	f.clickAt(t, "A0a", f.checkX(t, "A0a"))
	f.wantStates(t, "after unchecking A0a", map[string]int{
		"A":   TVC_PARTIAL,
		"A0":  TVC_PARTIAL,
		"A0a": TVC_UNCHECKED,
		"A0b": TVC_CHECKED,
		"A1":  TVC_CHECKED,
		"B":   TVC_UNCHECKED,
	})

	// (c) clicking it again resolves the partial chain back to checked.
	f.clickAt(t, "A0a", f.checkX(t, "A0a"))
	f.wantStates(t, "after re-checking A0a", map[string]int{
		"A":   TVC_CHECKED,
		"A0":  TVC_CHECKED,
		"A0a": TVC_CHECKED,
		"A0b": TVC_CHECKED,
		"A1":  TVC_CHECKED,
		"B":   TVC_UNCHECKED,
	})

	// (f) CheckedIndexes reports the fully checked rows in visible order.
	want := []ModelIndex{f.mi("A"), f.mi("A0"), f.mi("A0a"), f.mi("A0b"), f.mi("A1")}
	got := f.tv.CheckedIndexes()
	if len(got) != len(want) {
		t.Fatalf("CheckedIndexes() = %v, want %v", f.labels(got), f.labels(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("CheckedIndexes() = %v, want %v (position %d)", f.labels(got), f.labels(want), i)
		}
	}
}

// TestTreeViewCheckPartialIsNotSettableFromOutside: TVC_PARTIAL is derived from
// the children, so a caller passing it must neither plant a half-checked row no
// child state backs, nor be quietly read as "not checked" and clear the subtree.
// The call is made on an already checked subtree so both readings are visible.
func TestTreeViewCheckPartialIsNotSettableFromOutside(t *testing.T) {
	f := newCheckTree(t)
	f.clickAt(t, "A", f.checkX(t, "A"))

	f.tv.SetCheckState(f.mi("A0"), TVC_PARTIAL)
	f.wantStates(t, "after SetCheckState(A0, TVC_PARTIAL)", map[string]int{
		"A":   TVC_CHECKED,
		"A0":  TVC_CHECKED,
		"A0a": TVC_CHECKED,
		"A0b": TVC_CHECKED,
		"A1":  TVC_CHECKED,
		"B":   TVC_UNCHECKED,
	})
}

// TestTreeViewCheckSpaceTogglesThenFallsBackToActivate: with the check column on
// Space is a toggle for a checkable row and must not also activate it; with the
// column off the same key goes back to being the activation key.
func TestTreeViewCheckSpaceTogglesThenFallsBackToActivate(t *testing.T) {
	f := newCheckTree(t)

	var activated []string
	f.tv.SetActivatedCallback(func(o interface{}, mi ModelIndex) {
		activated = append(activated, f.label(mi))
	})

	f.tv.SetCurrentRow(f.row(t, "B").ri)
	if got, want := f.tv.CurrentIndex(), f.mi("B"); got != want {
		t.Fatalf("setup: current row = %q, want %q", f.label(got), f.label(want))
	}

	f.tv.OnKeyDown(KeySpace, false)
	if got := f.state("B"); got != TVC_CHECKED {
		t.Fatalf("Space with the check column on: CheckState(B) = %s, want TVC_CHECKED", checkStateName(got))
	}
	if len(activated) != 0 {
		t.Errorf("Space toggled the check box and activated the row as well: %v", activated)
	}

	f.tv.SetCheckBoxVisible(false)
	f.tv.OnKeyDown(KeySpace, false)
	if got := f.state("B"); got != TVC_CHECKED {
		t.Errorf("Space with the check column hidden changed CheckState(B) to %s, want TVC_CHECKED (unchanged)",
			checkStateName(got))
	}
	if want := []string{"B"}; !eqStrings(activated, want) {
		t.Errorf("Space with the check column hidden: activations = %v, want %v", activated, want)
	}
}

// TestTreeViewCheckUserCheckableGuardsUserInputOnly: ItemIsUserCheckable gates
// the mouse and Space only. The row still takes a programmatic SetCheckState,
// still counts in its parent's recompute, and still follows a parent that is
// checked from above.
func TestTreeViewCheckUserCheckableGuardsUserInputOnly(t *testing.T) {
	f := newCheckTree(t, "A1")

	// The mouse cannot toggle a row the model does not offer to the user.
	f.clickAt(t, "A1", f.checkX(t, "A1"))
	f.wantStates(t, "after clicking the check band of a non-checkable row", map[string]int{})

	// Code can, and the row still drives its parent's recompute.
	f.tv.SetCheckState(f.mi("A1"), TVC_CHECKED)
	f.wantStates(t, "after SetCheckState on the same row", map[string]int{
		"A":  TVC_PARTIAL,
		"A1": TVC_CHECKED,
	})

	// Space is guarded the same way as the mouse.
	f.tv.SetCurrentRow(f.row(t, "A1").ri)
	f.tv.OnKeyDown(KeySpace, false)
	if got := f.state("A1"); got != TVC_CHECKED {
		t.Errorf("Space on a non-checkable row changed CheckState(A1) to %s, want TVC_CHECKED (unchanged)",
			checkStateName(got))
	}

	// Checking the parent from the check band still reaches it: the flag is
	// about user input on that row, not about propagation.
	f.clickAt(t, "A", f.checkX(t, "A"))
	f.wantStates(t, "after checking the parent A", map[string]int{
		"A":   TVC_CHECKED,
		"A0":  TVC_CHECKED,
		"A0a": TVC_CHECKED,
		"A0b": TVC_CHECKED,
		"A1":  TVC_CHECKED,
		"B":   TVC_UNCHECKED,
	})
}

// TestTreeViewCheckDrawsBoxAndShiftsContent drives the real TreeView.Draw and
// inspects column 0: a box in the reserved band on every row (leaves included),
// the icon+text origin moved right by exactly one band, and a dash on top of the
// box only while the row is partially checked.
func TestTreeViewCheckDrawsBoxAndShiftsContent(t *testing.T) {
	f := newCheckTree(t)

	band := f.tv.expanderSize()

	on := f.draw()
	f.tv.SetCheckBoxVisible(false)
	off := f.draw()
	f.tv.SetCheckBoxVisible(true)

	for _, name := range checkTreeRows {
		row := f.row(t, name)
		x0, y0 := f.checkX(t, name), row.ypos
		y1 := y0 + f.tv.rowHeight(row.ri)

		if n := on.iconsIn(x0, x0+band, y0, y1); n != 1 {
			t.Errorf("row %s: %d icons painted in the check band [%v,%v), want exactly 1",
				name, n, x0, x0+band)
		}
		if n := off.iconsIn(x0, x0+band, y0, y1); n != 0 {
			t.Errorf("row %s: %d icons painted in the check band with the check column off, want 0",
				name, n)
		}

		xOn, okOn := on.textX(name)
		xOff, okOff := off.textX(name)
		if !okOn || !okOff {
			t.Fatalf("row %s: text drawn with check column on=%v off=%v; Draw skipped the row", name, okOn, okOff)
		}
		if xOn-xOff != band {
			t.Errorf("row %s: text origin = %v with the check column on and %v with it off; "+
				"want it shifted right by exactly one band (%v)", name, xOn, xOff, band)
		}
	}

	// A fully checked row draws the box and nothing else over it.
	f.clickAt(t, "A", f.checkX(t, "A"))
	if n := len(f.draw().lines); n != 0 {
		t.Errorf("fully checked tree: %d strokes over the boxes, want 0 (the dash is for partial only)", n)
	}

	// Unchecking one grandchild makes A0 and A partial: each gets a dash inside
	// its own check band.
	f.clickAt(t, "A0a", f.checkX(t, "A0a"))
	rec := f.draw()
	for _, name := range []string{"A0", "A"} {
		row := f.row(t, name)
		x0 := f.checkX(t, name)
		n := 0
		for _, l := range rec.lines {
			if l.x1 >= x0 && l.x2 <= x0+band && l.y1 >= row.ypos && l.y1 < row.ypos+f.tv.rowHeight(row.ri) {
				n++
			}
		}
		if n != 1 {
			t.Errorf("partially checked row %s: %d dashes inside its check band, want 1 (all strokes: %v)",
				name, n, rec.lines)
		}
	}
	if n := len(rec.lines); n != 2 {
		t.Errorf("%d dashes drawn in total, want 2 (only A0 and A are partial)", n)
	}
}

func (f *checkTree) labels(idxs []ModelIndex) []string {
	out := make([]string, len(idxs))
	for i, mi := range idxs {
		out[i] = f.label(mi)
	}
	return out
}

func (f *checkTree) draw() *checkDrawRecorder {
	rec := newCheckDrawRecorder()
	f.tv.Draw(rec)
	return rec
}

type drawnIcon struct{ x, y, size float64 }

type drawnText struct {
	x, y float64
	text string
}

type drawnLine struct{ x1, y1, x2, y2 float64 }

// checkDrawRecorder is a no-op paint.Painter (it embeds nopPainter, see
// spinner_test.go) that records the icons, text and strokes TreeView.Draw makes,
// so column 0 can be measured without a GLFW window. Translate is ignored, so
// the recorded coordinates are the ones the draw code computes, measured from
// the left edge of column 0.
type checkDrawRecorder struct {
	paint.Painter
	font  paint.Font
	icons []drawnIcon
	texts []drawnText
	lines []drawnLine
}

func newCheckDrawRecorder() *checkDrawRecorder {
	return &checkDrawRecorder{Painter: nopPainter{}, font: Theme().Font}
}

func (r *checkDrawRecorder) Save() int                                  { return 0 }
func (r *checkDrawRecorder) Restore() int                               { return 0 }
func (r *checkDrawRecorder) Clip()                                      {}
func (r *checkDrawRecorder) Rectangle(x, y, w, h float64)               {}
func (r *checkDrawRecorder) Rectangle1(rect geom.Rect)                  {}
func (r *checkDrawRecorder) Translate(tx, ty float64)                   {}
func (r *checkDrawRecorder) MoveTo(x, y float64)                        {}
func (r *checkDrawRecorder) LineTo(x, y float64)                        {}
func (r *checkDrawRecorder) Arc(xc, yc, radius, angle1, angle2 float64) {}
func (r *checkDrawRecorder) Stroke()                                    {}
func (r *checkDrawRecorder) Fill()                                      {}
func (r *checkDrawRecorder) FillPreserve()                              {}
func (r *checkDrawRecorder) SetPen1(c paint.Color, w float64)           {}
func (r *checkDrawRecorder) SetBrush1(c paint.Color)                    {}
func (r *checkDrawRecorder) SetFont(f paint.Font)                       { r.font = f }
func (r *checkDrawRecorder) Font() paint.Font                           { return r.font }

func (r *checkDrawRecorder) Line(x1, y1, x2, y2 float64) {
	r.lines = append(r.lines, drawnLine{x1, y1, x2, y2})
}

func (r *checkDrawRecorder) DrawText1(x, y float64, text string) {
	r.texts = append(r.texts, drawnText{x, y, text})
}

func (r *checkDrawRecorder) DrawIcon1(icon paint.Icon, x, y, size float64, gray bool) {
	r.icons = append(r.icons, drawnIcon{x, y, size})
}

// iconsIn counts the icons whose origin falls in the box [x0,x1) x [y0,y1) —
// one row's slice of one band, since neighbouring rows at the same indent draw
// into the same x range.
func (r *checkDrawRecorder) iconsIn(x0, x1, y0, y1 float64) int {
	n := 0
	for _, i := range r.icons {
		if i.x >= x0 && i.x < x1 && i.y >= y0 && i.y < y1 {
			n++
		}
	}
	return n
}

// textX returns the x origin the given cell text was drawn at.
func (r *checkDrawRecorder) textX(text string) (float64, bool) {
	for _, t := range r.texts {
		if t.text == text {
			return t.x, true
		}
	}
	return 0, false
}
