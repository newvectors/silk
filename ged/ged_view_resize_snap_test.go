package ged

// Mouse resize has to land on the grid the way every move already does, and
// nothing else may. OnLeftUp used to snap only the position, so a widget
// dragged in by hand was pixel-tidy while the same widget resized by a handle
// kept whatever fraction the cursor left behind — invisible on screen, because
// the status bar prints the size with %.0f. The first fix then ran the size
// snap on every canvas press: a selection click, a move drag and a click on
// empty canvas (which selects the form) all rewrote sizes nobody had touched,
// off the undo stack and out of sight.
//
// So the cases here come in two halves: the gestures that must snap, and the
// gestures that must not. Every one of them drives the real thing (FindHandleAt
// on the bottom-right handle, then OnLeftDown / OnMouseMove / OnLeftUp through
// the view) rather than calling snapSelectionSizeToGrid directly: the snap is
// only worth anything at the OnLeftUp call site, and a direct call would stay
// green with that line deleted or fired on the wrong gesture.

import (
	"fmt"
	"testing"
	"time"

	"github.com/uk0/silk/graph"
)

// handleBottomRight is ResizeDecor's index for the bottom-right grip (graph
// direction dirBottomRight), the one every case below drags.
const handleBottomRight = 8

// newResizeSnapView builds the fixture the cases share: a 200x150mm scene on a
// grid of the given pitch, with snap in the requested state.
func newResizeSnapView(t *testing.T, pitch float64, snap bool) (*GedView, *GedScene) {
	t.Helper()
	view := NewGedView()
	scene := view.GedScene()
	scene.SetSize(200, 150)
	view.SetGridStep(pitch)
	view.SetSnapToGrid(snap)
	return view, scene
}

// dragScene runs one gesture: press at scene point (sx, sy), move the pointer
// to (ex, ey), release there. It returns the release point in view pixels, so a
// case that needs sub-pixel control can press again from the same place.
func dragScene(t *testing.T, view *GedView, sx, sy, ex, ey float64) (px, py float64) {
	t.Helper()
	lastClickTime = time.Time{}
	dx, dy := view.MapFromScene(sx, sy)
	px, py = view.MapFromScene(ex, ey)
	view.OnLeftDown(dx, dy)
	view.OnMouseMove(px, py)
	view.OnLeftUp(px, py)
	return px, py
}

// clickScene presses and releases at one scene point without moving the
// pointer — the plainest gesture the canvas has, and the one the size snap used
// to fire on.
func clickScene(t *testing.T, view *GedView, sx, sy float64) {
	t.Helper()
	lastClickTime = time.Time{}
	px, py := view.MapFromScene(sx, sy)
	view.OnLeftDown(px, py)
	view.OnLeftUp(px, py)
}

// dragBottomRightHandle runs one real resize gesture: press the bottom-right
// handle at scene point (sx, sy), drag to (ex, ey), release. It refuses to run
// unless the press really is on that handle, and returns the scene point the
// release landed on, read back through the view's own MapToScene — the same
// conversion the gesture ran — so a case can pin the unsnapped size exactly
// without baking in a screen DPI.
func dragBottomRightHandle(t *testing.T, view *GedView, sx, sy, ex, ey float64) (endX, endY float64) {
	t.Helper()
	if decor, handle := view.FindHandleAt(sx, sy); decor == nil || handle != handleBottomRight {
		t.Fatalf("press point (%g, %g) is not on the bottom-right handle: decor=%v handle=%d",
			sx, sy, decor, handle)
	}
	px, py := dragScene(t, view, sx, sy, ex, ey)
	return view.MapToScene(px, py)
}

// The shared gesture: a button parked off-grid at (10.4, 12.7) sized 25x7, its
// bottom-right handle pressed at (34.9, 19.2) and released at (43.6, 24.0).
// That leaves a raw 33.2 x 11.3.
const (
	snapBtnX, snapBtnY = 10.4, 12.7
	snapBtnW, snapBtnH = 25.0, 7.0
	snapPressX         = 34.9
	snapPressY         = 19.2
	snapEndX           = 43.6
	snapEndY           = 24.0
)

// TestResizeGestureSnapsSizeToGrid: the handle drag ends on a fractional size,
// and OnLeftUp must round it onto the grid. Run on two different grids, because
// the raw 33.2 x 11.3 rounds to a different multiple on each — a pitch baked
// into the snap (5, 10, anything) can satisfy at most one row. And on each grid
// one extent rounds up while the other rounds down, so neither a floor nor a
// ceil in place of the rounding can satisfy even one row.
//
// The position assertions are the other half: the button started off-grid at
// (10.4, 12.7) and must still be pulled onto the grid, so replacing
// snapSelectionToGrid with the new size snap instead of adding beside it turns
// this red.
func TestResizeGestureSnapsSizeToGrid(t *testing.T) {
	cases := []struct {
		pitch        float64
		wantW, wantH float64 // the raw 33.2 x 11.3 rounded onto this pitch
		wantX, wantY float64 // the off-grid (10.4, 12.7) rounded onto it
	}{
		{pitch: 5, wantW: 35, wantH: 10, wantX: 10, wantY: 15}, // 33.2 rounds up, 11.3 down
		{pitch: 4, wantW: 32, wantH: 12, wantX: 12, wantY: 12}, // 33.2 rounds down, 11.3 up
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%gmm grid", c.pitch), func(t *testing.T) {
			view, scene := newResizeSnapView(t, c.pitch, true)
			btn := addFakeAt(t, scene, "btn", snapBtnX, snapBtnY, snapBtnW, snapBtnH)
			view.Selection().Add(btn)

			dragBottomRightHandle(t, view, snapPressX, snapPressY, snapEndX, snapEndY)

			if w, h := btn.Width(), btn.Height(); w != c.wantW || h != c.wantH {
				t.Errorf("resize gesture on a %gmm grid: size = (%v, %v), want (%v, %v) — "+
					"the raw 33.2 x 11.3 the handle committed must round onto this grid's pitch",
					c.pitch, w, h, c.wantW, c.wantH)
			}
			if x, y := btn.Pos(); x != c.wantX || y != c.wantY {
				t.Errorf("resize gesture on a %gmm grid: pos = (%v, %v), want (%v, %v) — "+
					"the position snap must survive alongside the size snap", c.pitch, x, y, c.wantX, c.wantY)
			}
		})
	}
}

// TestResizeGestureKeepsSizeWhenSnapOff is the gate: with 表单设置 snap turned
// off the identical gesture must leave the fractional size exactly as the
// handle drag committed it. The expectation is derived from the release point
// the view itself computed, so it is the raw width to the last bit — a snap
// that ignored grid.Snap and rounded to 35 x 10 shows up here.
func TestResizeGestureKeepsSizeWhenSnapOff(t *testing.T) {
	view, scene := newResizeSnapView(t, 5, false)
	btn := addFakeAt(t, scene, "btn", snapBtnX, snapBtnY, snapBtnW, snapBtnH)
	view.Selection().Add(btn)

	endX, endY := dragBottomRightHandle(t, view, snapPressX, snapPressY, snapEndX, snapEndY)

	// The bottom-right handle drags the right and bottom edges to the cursor,
	// so the committed extent is exactly cursor minus the pinned top-left.
	wantW, wantH := endX-snapBtnX, endY-snapBtnY
	if w, h := btn.Width(), btn.Height(); w != wantW || h != wantH {
		t.Errorf("with snap off: size = (%v, %v), want the raw (%v, %v) untouched", w, h, wantW, wantH)
	}
	if x, y := btn.Pos(); x != snapBtnX || y != snapBtnY {
		t.Errorf("with snap off: pos = (%v, %v), want (%v, %v) untouched", x, y, snapBtnX, snapBtnY)
	}
}

// TestSelectionClickLeavesSizeAlone is the defect this repair exists for: a
// press that is not a resize at all — no handle, no movement — must not touch
// a single dimension. The sizes are the two the reviewer measured: a freshly
// dropped Button carries defaultSizeForWidget's 7mm height, which is off-grid
// on the default 5mm grid, and a deliberately sub-pitch widget is smaller than
// the grid can express at all. The click is asserted to have selected the
// widget, so a case cannot pass by the press never reaching the canvas.
func TestSelectionClickLeavesSizeAlone(t *testing.T) {
	cases := []struct {
		name string
		w, h float64
	}{
		{"a freshly dropped Button keeps its off-grid 7mm height", 25, 7},
		{"a deliberately sub-pitch widget keeps both extents", 3, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			view, scene := newResizeSnapView(t, 5, true)
			btn := addFakeAt(t, scene, "btn", 20, 20, c.w, c.h)

			clickScene(t, view, 20+c.w*0.5, 20+c.h*0.5)

			if !view.Selection().Contains(btn) {
				t.Fatalf("the click did not select the widget, so this case proves nothing")
			}
			if w, h := btn.Width(), btn.Height(); w != c.w || h != c.h {
				t.Errorf("after a plain selection click: size = (%v, %v), want (%v, %v) — "+
					"a click that resized nothing must not resize anything; the size snap "+
					"belongs to the handle drag alone", w, h, c.w, c.h)
			}
		})
	}
}

// TestMoveDragLeavesSizeAlone: a widget dragged across the canvas is moved, not
// resized, so its extents must come out the far side untouched — while the
// position snap that shares the same OnLeftUp block still runs. Asserting both
// in one gesture is the point: it pins the size snap to the resize gesture
// specifically, not to "the drag block ran".
func TestMoveDragLeavesSizeAlone(t *testing.T) {
	view, scene := newResizeSnapView(t, 5, true)
	btn := addFakeAt(t, scene, "btn", snapBtnX, snapBtnY, snapBtnW, snapBtnH)

	// Press in the middle of the button, well clear of every handle disc, and
	// drag it 8mm down-right: MovePart claims the press and selects the widget.
	dragScene(t, view, snapBtnX+12, snapBtnY+3.5, snapBtnX+20, snapBtnY+11.5)

	if x, y := btn.Pos(); x != 20 || y != 20 {
		t.Fatalf("after the move drag: pos = (%v, %v), want (20, 20) — the drag or its "+
			"position snap did not run, so the size assertion below proves nothing", x, y)
	}
	if w, h := btn.Width(), btn.Height(); w != snapBtnW || h != snapBtnH {
		t.Errorf("after a move drag: size = (%v, %v), want (%v, %v) — moving a widget "+
			"must not resize it", w, h, snapBtnW, snapBtnH)
	}
}

// TestBlankCanvasClickLeavesFormSize: a click on empty canvas selects the scene
// — the form itself — so the size snap firing on any press rewrote the window's
// dimensions, which SetSize in the generated code then carries into the built
// program. 163x102 is off-grid on both axes, one rounding up and one down, and
// the reviewer measured it becoming 165x100.
func TestBlankCanvasClickLeavesFormSize(t *testing.T) {
	view, scene := newResizeSnapView(t, 5, true)
	scene.SetSize(163, 102)
	addFakeAt(t, scene, "btn", 10, 10, 25, 7)

	clickScene(t, view, 120, 80) // empty canvas, far from the button

	if !view.Selection().Contains(scene) {
		t.Fatalf("the click on empty canvas did not select the form, so this case proves nothing")
	}
	if w, h := scene.Width(), scene.Height(); w != 163 || h != 102 {
		t.Errorf("after a click on empty canvas: form size = (%v, %v), want (163, 102) — "+
			"deselecting must not resize the form", w, h)
	}
}

// TestFormHandleDragSnapsFormSize is the other side of the case above: the
// form is resized by dragging its own handle, and that gesture must snap like
// any other. The two share a fixture on purpose — the click that selects the
// form is the one that must leave it alone, and the handles it arms are the
// ones that must round it — so a gate written as "never touch the scene root"
// passes the click case and fails here.
func TestFormHandleDragSnapsFormSize(t *testing.T) {
	view, scene := newResizeSnapView(t, 5, true)
	scene.SetSize(163, 102)

	clickScene(t, view, 80, 60) // empty canvas: selects the form, arming its handles
	if !view.Selection().Contains(scene) {
		t.Fatalf("the click on empty canvas did not select the form")
	}

	// Press just inside the form's bottom-right corner and drag it to a raw
	// 150.4 x 93.2 — one extent rounding down onto the 5mm grid, one up.
	dragBottomRightHandle(t, view, 162.5, 101.5, 150.4, 93.2)

	if w, h := scene.Width(), scene.Height(); w != 150 || h != 95 {
		t.Errorf("after dragging the form's own handle: form size = (%v, %v), want (150, 95) — "+
			"a real handle drag on the form must snap the same as one on a widget", w, h)
	}
}

// TestHandlePressWithoutARealDragLeavesSize covers the decision: a press that
// lands on a resize handle but never drags it does NOT snap. DecorPart commits
// no resize for such a press (it calls OnEndMoveHandle only once the pointer has
// crossed graph's 1px drag threshold and set the decor moving), so there is no
// new size for the snap to tidy — rounding one would be the same silent,
// un-undoable edit as snapping a plain click. Gating on "the press was on a
// handle" alone, or on "the pointer moved at all", turns one of these red.
func TestHandlePressWithoutARealDragLeavesSize(t *testing.T) {
	cases := []struct {
		name     string
		dpx, dpy float64 // pointer movement in view pixels, before release
	}{
		{"press and release without moving", 0, 0},
		{"jitter that stays inside the 1px drag threshold", 1, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			view, scene := newResizeSnapView(t, 5, true)
			btn := addFakeAt(t, scene, "btn", snapBtnX, snapBtnY, snapBtnW, snapBtnH)
			view.Selection().Add(btn)
			if decor, handle := view.FindHandleAt(snapPressX, snapPressY); decor == nil || handle != handleBottomRight {
				t.Fatalf("press point is not on the bottom-right handle: decor=%v handle=%d", decor, handle)
			}

			lastClickTime = time.Time{}
			px, py := view.MapFromScene(snapPressX, snapPressY)
			view.OnLeftDown(px, py)
			if c.dpx != 0 || c.dpy != 0 {
				view.OnMouseMove(px+c.dpx, py+c.dpy)
			}
			view.OnLeftUp(px+c.dpx, py+c.dpy)

			if w, h := btn.Width(), btn.Height(); w != snapBtnW || h != snapBtnH {
				t.Errorf("after a handle press that never resized anything: size = (%v, %v), "+
					"want (%v, %v) — no resize was committed, so there is nothing to snap",
					w, h, snapBtnW, snapBtnH)
			}
		})
	}

	// Control on the same fixture and the same handle: a drag that does cross
	// the threshold must snap. Without it the two cases above would also pass
	// with the whole feature deleted.
	view, scene := newResizeSnapView(t, 5, true)
	btn := addFakeAt(t, scene, "btn", snapBtnX, snapBtnY, snapBtnW, snapBtnH)
	view.Selection().Add(btn)

	dragBottomRightHandle(t, view, snapPressX, snapPressY, snapEndX, snapEndY)

	if w, h := btn.Width(), btn.Height(); w != 35 || h != 10 {
		t.Fatalf("control: a real drag of the same handle left size = (%v, %v), want (35, 10) — "+
			"the cases above pass only because the size snap never runs at all", w, h)
	}
}

// TestResizeGestureSkipsLockedSize: a size-locked widget is dropped from the
// resize command (ResizeDecor.OnEndMoveHandle skips it), so the snap must drop
// it too — otherwise a lock the user set is undone by the grid on a gesture
// aimed at another widget. The locked widget cannot be the dragged one:
// GenDecors hands a locked item no handles at all, so it rides along as a
// second selected item. The dragged button's own snap is asserted in the same
// run, so "locked size unchanged" cannot pass by the snap never running.
func TestResizeGestureSkipsLockedSize(t *testing.T) {
	view, scene := newResizeSnapView(t, 5, true)
	btn := addFakeAt(t, scene, "btn", snapBtnX, snapBtnY, snapBtnW, snapBtnH)
	locked := addFakeAt(t, scene, "locked", 60, 10, 12.3, 6.7)
	locked.SetLockSize(true)
	view.Selection().Add(btn)
	view.Selection().Add(locked)

	dragBottomRightHandle(t, view, snapPressX, snapPressY, snapEndX, snapEndY)

	if w, h := locked.Width(), locked.Height(); w != 12.3 || h != 6.7 {
		t.Errorf("size-locked widget: size = (%v, %v), want (12.3, 6.7) — the grid snap "+
			"must respect IsLockSize", w, h)
	}
	if w, h := btn.Width(), btn.Height(); w != 35 || h != 10 {
		t.Fatalf("dragged widget: size = (%v, %v), want (35, 10) — the snap did not run, "+
			"so the lock assertion above proves nothing", w, h)
	}
}

// TestResizeGestureSkipsLockedPosition is the same rule for the other lock:
// ResizeDecor.OnEndMoveHandle drops a position-locked item from the resize too
// (it would have to move an edge to scale it), so the gesture never changed its
// size and the snap must not either. Only IsLockSize was consulted before, so a
// pinned-but-resizable widget riding in the selection had its dimensions
// rewritten by a drag on someone else's handle.
func TestResizeGestureSkipsLockedPosition(t *testing.T) {
	view, scene := newResizeSnapView(t, 5, true)
	btn := addFakeAt(t, scene, "btn", snapBtnX, snapBtnY, snapBtnW, snapBtnH)
	pinned := addFakeAt(t, scene, "pinned", 60, 10, 12.3, 6.7)
	pinned.SetLockPos(true)
	view.Selection().Add(btn)
	view.Selection().Add(pinned)

	dragBottomRightHandle(t, view, snapPressX, snapPressY, snapEndX, snapEndY)

	if w, h := pinned.Width(), pinned.Height(); w != 12.3 || h != 6.7 {
		t.Errorf("position-locked widget: size = (%v, %v), want (12.3, 6.7) — the resize "+
			"command skipped it, so the snap has nothing of its to tidy", w, h)
	}
	if w, h := btn.Width(), btn.Height(); w != 35 || h != 10 {
		t.Fatalf("dragged widget: size = (%v, %v), want (35, 10) — the snap did not run, "+
			"so the lock assertion above proves nothing", w, h)
	}
}

// TestResizeGestureSkipsNestedSelection: a selected child of a selected
// container was already scaled proportionally by the resize command, so
// snapping it again would break the ratio the resize just gave it — the same
// skip snapSelectionToGrid makes. The child's expected size is read off the
// identical gesture with snap off rather than hard-coded, so the case pins
// "the snap left the child alone" and not the scale factor.
func TestResizeGestureSkipsNestedSelection(t *testing.T) {
	_, rawChild := nestedResizeGesture(t, false)
	rawW, rawH := rawChild.Width(), rawChild.Height()

	vbox, child := nestedResizeGesture(t, true)

	if w, h := vbox.Width(), vbox.Height(); w != 40 || h != 10 {
		t.Fatalf("container: size = (%v, %v), want (40, 10) — the snap did not run, "+
			"so the child assertion below proves nothing", w, h)
	}
	if w, h := child.Width(), child.Height(); w != rawW || h != rawH {
		t.Errorf("nested child: size = (%v, %v), want (%v, %v) — a child whose container "+
			"is selected too must be left to the container's scale", w, h, rawW, rawH)
	}
	if x, y := child.Pos(); x != 12 || y != 12 {
		t.Errorf("nested child: pos = (%v, %v), want (12, 12) untouched", x, y)
	}
}

// nestedResizeGesture selects a VBox and the button inside it, then drags the
// VBox's bottom-right handle from (34.5, 16.5) to (48.3, 21.2) — a raw
// 38.3 x 11.2 on the container, which scales the child to a size that is no
// multiple of the 5mm pitch.
func nestedResizeGesture(t *testing.T, snap bool) (vbox, child *FakeWidget) {
	t.Helper()
	view, scene := newResizeSnapView(t, 5, snap)

	vbox, err := NewFakeWidgetFromFactory("gui.VBox")
	if err != nil {
		t.Fatalf("create VBox: %v", err)
	}
	vbox.SetWidgetName("box")
	vbox.SetBounds(10, 10, 25, 7)
	vbox.SetParent(scene)

	child = addFakeAt(t, scene, "child", 12, 12, 8.3, 3.7)
	child.SetParent(vbox)
	if !hasSelectedAncestor(child, map[graph.IItem]bool{vbox: true}) {
		t.Fatalf("fixture is wrong: the child is not inside the VBox")
	}

	view.Selection().Add(vbox)
	view.Selection().Add(child)

	dragBottomRightHandle(t, view, 34.5, 16.5, 48.3, 21.2)
	return vbox, child
}

// TestResizeGestureFloorsSnappedSizeAtOnePitch: dragging the handle back
// inside the widget leaves a raw 2.3 x 1.4, and rounding that to the nearest
// grid multiple is zero. A zero-extent widget is one the user can neither see
// nor grab a handle on again, so the snap floors at one full pitch.
func TestResizeGestureFloorsSnappedSizeAtOnePitch(t *testing.T) {
	view, scene := newResizeSnapView(t, 5, true)
	btn := addFakeAt(t, scene, "btn", snapBtnX, snapBtnY, snapBtnW, snapBtnH)
	view.Selection().Add(btn)

	dragBottomRightHandle(t, view, snapPressX, snapPressY, 12.7, 14.1)

	if w, h := btn.Width(), btn.Height(); w != 5 || h != 5 {
		t.Errorf("after shrinking below half a pitch: size = (%v, %v), want (5, 5) — "+
			"rounding alone gives a 0 x 0 widget with nothing left to grab", w, h)
	}
}
