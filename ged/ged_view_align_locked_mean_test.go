package ged

import "testing"

// centresAfterMeanAlign builds the same three ungrouped widgets every test in
// this file works on — leftmost at centre 10, then 100, then 130, all three
// selected — optionally pins the leftmost one, runs mode, and returns the three
// centres along the axis mode aligns. Different widths on purpose: a centre
// mode has to place each widget by its own half-width, and equal widths would
// let an X-only fix pass.
func centresAfterMeanAlign(t *testing.T, mode AlignMode, lock bool) [3]float64 {
	t.Helper()

	view := NewGedView()
	scene := view.GedScene()
	scene.SetSize(400, 400)

	var a, b, c *FakeWidget
	if mode == AlignVCenter {
		a = addFakeAt(t, scene, "a", 5, 5, 4, 10)
		b = addFakeAt(t, scene, "b", 5, 90, 4, 20)
		c = addFakeAt(t, scene, "c", 5, 115, 4, 30)
	} else {
		a = addFakeAt(t, scene, "a", 5, 5, 10, 4)
		b = addFakeAt(t, scene, "b", 90, 5, 20, 4)
		c = addFakeAt(t, scene, "c", 115, 5, 30, 4)
	}
	a.SetLockPos(lock)

	view.Selection().Clear()
	for _, w := range []*FakeWidget{a, b, c} {
		view.Selection().Add(w)
	}
	view.AlignSelection(mode)

	var out [3]float64
	for i, w := range []*FakeWidget{a, b, c} {
		x, y := w.Pos()
		cw, ch := w.Size()
		if mode == AlignVCenter {
			out[i] = y + ch/2
		} else {
			out[i] = x + cw/2
		}
	}
	return out
}

// TestAlignHCenterMeanExcludesLockedWidget: 水平居中对齐 puts the selection on
// the mean of its centres, and the mean only means anything over the widgets
// that swing onto it. A 位置锁定 widget does not — it stays where it is — so
// letting it vote picked a line nobody ends up on: pinned centre 10 with free
// centres 100 and 130 dragged the two free widgets to 80, past each other's
// midpoint at 115 and towards a widget that never moved. This is the mirror of
// the carried-child rule (TestAlignHCenterMeanExcludesCarriedChild); both ask
// whether the rect will move, not why it will not.
//
// The control run in the same test is the point of the arithmetic: 115 alone
// is also what a fix that drops locked rects from EVERY mode produces, and what
// a test with the answer hardcoded into it produces. Only the unlocked run
// landing on 80 says the lock is what changed the line.
func TestAlignHCenterMeanExcludesLockedWidget(t *testing.T) {
	locked := centresAfterMeanAlign(t, AlignHCenter, true)
	if want := [3]float64{10, 115, 115}; locked != want {
		t.Errorf("水平居中对齐 with the leftmost widget 位置锁定 gave centres %v, want %v — "+
			"the pinned widget stays at 10 and the two free ones meet at their own mean", locked, want)
	}

	free := centresAfterMeanAlign(t, AlignHCenter, false)
	if want := [3]float64{80, 80, 80}; free != want {
		t.Errorf("水平居中对齐 with nothing locked gave centres %v, want %v — "+
			"the mean over all three; the lock is what may drop a widget out of it, "+
			"not the align mode", free, want)
	}
}

// TestAlignVCenterMeanExcludesLockedWidget is the vertical twin, and a separate
// test rather than a table row for the same reason the 分布 refusals are: the
// filter is written as one condition over both centre modes, so narrowing it to
// AlignHCenter alone would leave 垂直居中对齐 averaging over a widget that never
// moves and the whole suite green.
func TestAlignVCenterMeanExcludesLockedWidget(t *testing.T) {
	locked := centresAfterMeanAlign(t, AlignVCenter, true)
	if want := [3]float64{10, 115, 115}; locked != want {
		t.Errorf("垂直居中对齐 with the topmost widget 位置锁定 gave centres %v, want %v", locked, want)
	}

	free := centresAfterMeanAlign(t, AlignVCenter, false)
	if want := [3]float64{80, 80, 80}; free != want {
		t.Errorf("垂直居中对齐 with nothing locked gave centres %v, want %v", free, want)
	}
}

// TestAlignLeftStillAveragesNothingAndKeepsTheLockedEdge is the boundary of the
// filter above: an edge mode names a line one of the rects already owns, and a
// locked widget owns its edge more firmly than anyone — it is still there when
// the move is over. So 左对齐 keeps counting it, and pinning the leftmost widget
// is how a user says "line the rest up on this one". Extending the locked-rect
// filter to the edge modes would compute min-left over {90, 115} and send the
// group sailing off to the right of the widget that was pinned.
func TestAlignLeftStillAveragesNothingAndKeepsTheLockedEdge(t *testing.T) {
	view := NewGedView()
	scene := view.GedScene()
	scene.SetSize(400, 400)

	// The fixture the centre tests use, where the pinned widget is also the
	// leftmost: same selection, opposite expectation.
	a := addFakeAt(t, scene, "a", 5, 5, 10, 4)
	b := addFakeAt(t, scene, "b", 90, 5, 20, 4)
	c := addFakeAt(t, scene, "c", 115, 5, 30, 4)
	a.SetLockPos(true)

	view.Selection().Clear()
	for _, w := range []*FakeWidget{a, b, c} {
		view.Selection().Add(w)
	}

	view.AlignSelection(AlignLeft)

	if x, y := a.Pos(); x != 5 || y != 5 {
		t.Errorf("位置锁定 widget moved to (%g,%g) on 左对齐, want (5,5)", x, y)
	}
	for _, w := range []*FakeWidget{b, c} {
		if x, _ := w.Pos(); x != 5 {
			t.Errorf("%s.X = %g after 左对齐, want 5 — the locked widget's own left edge is the line",
				w.WidgetName(), x)
		}
	}
}

// TestAlignHCenterWithOneFreeMoverIsANoOp is the degenerate end of the filter,
// decided rather than fallen into: with everything but one widget pinned, the
// mean over what moves is that widget's own centre, so 水平居中对齐 has nothing
// to say and must do nothing at all. Snapping the free widget onto the locked
// one's centre would be the obvious wrong answer — it is not an average of
// anything, and it would move the selection on a command the user cannot even
// see a second widget participating in.
//
// The undo stack is checked as well as the position: a command that moves
// nobody but still lands on the stack costs the user a Ctrl+Z that undoes
// nothing.
func TestAlignHCenterWithOneFreeMoverIsANoOp(t *testing.T) {
	view := NewGedView()
	scene := view.GedScene()
	scene.SetSize(400, 400)

	locked := addFakeAt(t, scene, "locked", 5, 5, 10, 4)
	free := addFakeAt(t, scene, "free", 90, 5, 20, 4)
	locked.SetLockPos(true)

	view.Selection().Clear()
	view.Selection().Add(locked)
	view.Selection().Add(free)

	before := view.Scene().UndoStack().Count()
	couldUndo := view.Scene().UndoStack().CanUndo()
	view.AlignSelection(AlignHCenter)

	if x, y := free.Pos(); x != 90 || y != 5 {
		t.Errorf("the only free widget moved to (%g,%g) on 水平居中对齐, want (90,5) — "+
			"the mean over what moves is its own centre of 100", x, y)
	}
	if x, _ := locked.Pos(); x != 5 {
		t.Errorf("位置锁定 widget moved to %g, want 5", x)
	}
	if got := view.Scene().UndoStack().Count() - before; got != 0 {
		t.Errorf("no-op 水平居中对齐 pushed %d command(s); Ctrl+Z would burn a step on nothing", got)
	}
	if view.Scene().UndoStack().CanUndo() != couldUndo {
		t.Errorf("no-op 水平居中对齐 changed CanUndo() from %v to %v",
			couldUndo, view.Scene().UndoStack().CanUndo())
	}
}

// TestAlignHCenterWithEveryMoverLockedIsANoOp is one widget past the case
// above, and the end of the filter's road: pin EVERY selected widget and the
// free-rect slice comes out empty, so the mean is asked of nothing at all.
// alignRects hands an empty slice straight back under its len < 2 guard,
// nobody gets a target, and the empty command is dropped — which is the only
// answer available, since the only widgets that could move are all pinned.
//
// It is spelled out because reaching that answer means NOT reading a rect the
// filter did not produce. Every other centre test here leaves one or two
// widgets free, so the slice always had something at index 0 and an unguarded
// read of it would go unseen.
func TestAlignHCenterWithEveryMoverLockedIsANoOp(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("水平居中对齐 panicked with every selected widget 位置锁定: %v — "+
				"the mean is taken over an empty free-rect slice, which has no rect to read", r)
		}
	}()

	view := NewGedView()
	scene := view.GedScene()
	scene.SetSize(400, 400)

	// The fixture the centre tests above use, with all three pinned instead of
	// just the leftmost.
	a := addFakeAt(t, scene, "a", 5, 5, 10, 4)
	b := addFakeAt(t, scene, "b", 90, 5, 20, 4)
	c := addFakeAt(t, scene, "c", 115, 5, 30, 4)
	for _, w := range []*FakeWidget{a, b, c} {
		w.SetLockPos(true)
	}

	view.Selection().Clear()
	for _, w := range []*FakeWidget{a, b, c} {
		view.Selection().Add(w)
	}

	before := view.Scene().UndoStack().Count()
	couldUndo := view.Scene().UndoStack().CanUndo()
	view.AlignSelection(AlignHCenter)

	want := []float64{5, 90, 115}
	for i, w := range []*FakeWidget{a, b, c} {
		if x, y := w.Pos(); x != want[i] || y != 5 {
			t.Errorf("%s sits at (%g,%g) after 水平居中对齐 with the whole selection 位置锁定, "+
				"want (%g,5) — there is no widget left to swing onto a line", w.WidgetName(), x, y, want[i])
		}
	}
	if got := view.Scene().UndoStack().Count() - before; got != 0 {
		t.Errorf("all-locked 水平居中对齐 pushed %d command(s); Ctrl+Z would burn a step on nothing", got)
	}
	if view.Scene().UndoStack().CanUndo() != couldUndo {
		t.Errorf("all-locked 水平居中对齐 changed CanUndo() from %v to %v",
			couldUndo, view.Scene().UndoStack().CanUndo())
	}
}

// TestAlignHCenterMeanExcludesLockedAndCarriedAtOnce puts both exclusions in
// one selection, which is the only place their order can be seen: the locked
// widget is filtered out of the mean here, the carried child was already out
// of the movers the filter reads, and a filter that reached back past the
// movers to the full selection would hand the child its vote back.
//
// TestAlignHCenterMeanExcludesCarriedChild has the child without a lock and
// the tests above have the lock without a child, so each of them passes on a
// mean that is wrong by the other rule.
func TestAlignHCenterMeanExcludesLockedAndCarriedAtOnce(t *testing.T) {
	view := NewGedView()
	scene := view.GedScene()
	scene.SetSize(400, 200)

	box := newAlignBox(t, scene, "box", 10, 20, 20, 30)
	inside := addFakeAt(t, scene, "inside", 12, 25, 4, 2)
	inside.SetParent(box)
	free := addFakeAt(t, scene, "free", 90, 60, 40, 10)
	locked := addFakeAt(t, scene, "locked", 200, 80, 40, 10)
	locked.SetLockPos(true)

	view.Selection().Clear()
	for _, w := range []*FakeWidget{box, inside, free, locked} {
		view.Selection().Add(w)
	}

	view.AlignSelection(AlignHCenter)

	// Two of the four rects survive both exclusions: the container's centre of
	// 20 and the free widget's of 110, so the line is 65. The child's 14 drags
	// it down to 48; the pinned widget's 220 pushes it up past 116.
	const wantCX = 65.0
	bx, _ := box.Pos()
	bw, _ := box.Size()
	if cx := bx + bw/2; cx != wantCX {
		t.Errorf("container centre = %g after 水平居中对齐, want %g — the mean is over the container "+
			"and the free widget, not over the child riding inside it or the pinned widget", cx, wantCX)
	}
	fx, _ := free.Pos()
	fw, _ := free.Size()
	if cx := fx + fw/2; cx != wantCX {
		t.Errorf("free widget centre = %g after 水平居中对齐, want %g — same line, placed by its own "+
			"half-width", cx, wantCX)
	}
	// Carried, at the offset it had: 12 - 10 = 2.
	if ix, _ := inside.Pos(); ix-bx != 2 {
		t.Errorf("widget inside the container sits at %g with the container at %g: offset %g, want 2",
			ix, bx, ix-bx)
	}
	if x, y := locked.Pos(); x != 200 || y != 80 {
		t.Errorf("位置锁定 widget moved to (%g,%g), want (200,80)", x, y)
	}
}
