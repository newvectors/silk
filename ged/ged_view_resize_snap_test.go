package ged

// Mouse resize has to land on the grid the way every move already does.
// OnLeftUp used to snap only the position, so a widget dragged in by hand was
// pixel-tidy while the same widget resized by a handle kept whatever fraction
// the cursor left behind — invisible on screen, because the status bar prints
// the size with %.0f.
//
// Every case here drives the real gesture (FindHandleAt on the bottom-right
// handle, then OnLeftDown / OnMouseMove / OnLeftUp through the view) rather
// than calling snapSelectionSizeToGrid directly: the snap is only worth
// anything at the OnLeftUp call site, and a direct call would stay green with
// that line deleted.

import (
	"testing"
	"time"

	"github.com/uk0/silk/graph"
)

// handleBottomRight is ResizeDecor's index for the bottom-right grip (graph
// direction dirBottomRight), the one every case below drags.
const handleBottomRight = 8

// newResizeSnapView builds the fixture the cases share: a 200x150mm scene on a
// 5mm grid, with snap in the requested state.
func newResizeSnapView(t *testing.T, snap bool) (*GedView, *GedScene) {
	t.Helper()
	view := NewGedView()
	scene := view.GedScene()
	scene.SetSize(200, 150)
	view.SetGridStep(5)
	view.SetSnapToGrid(snap)
	return view, scene
}

// dragBottomRightHandle runs one real resize gesture: press the bottom-right
// handle at scene point (sx, sy), drag to (ex, ey), release. It returns the
// scene point the release landed on, read back through the view's own
// MapToScene — the same conversion the gesture ran — so a case can pin the
// unsnapped size exactly without baking in a screen DPI.
func dragBottomRightHandle(t *testing.T, view *GedView, sx, sy, ex, ey float64) (endX, endY float64) {
	t.Helper()
	if decor, handle := view.FindHandleAt(sx, sy); decor == nil || handle != handleBottomRight {
		t.Fatalf("press point (%g, %g) is not on the bottom-right handle: decor=%v handle=%d",
			sx, sy, decor, handle)
	}
	lastClickTime = time.Time{}
	px, py := view.MapFromScene(sx, sy)
	qx, qy := view.MapFromScene(ex, ey)
	view.OnLeftDown(px, py)
	view.OnMouseMove(qx, qy)
	view.OnLeftUp(qx, qy)
	return view.MapToScene(qx, qy)
}

// The shared gesture: a button parked off-grid at (10.4, 12.7) sized 25x7, its
// bottom-right handle pressed at (34.9, 19.2) and released at (48.3, 23.9).
// That leaves a raw 37.9 x 11.2 — one fraction that rounds up and one that
// rounds down, so a floor (35 x 10) or a ceil (40 x 15) implementation cannot
// pass the snapped case by accident.
const (
	snapBtnX, snapBtnY = 10.4, 12.7
	snapBtnW, snapBtnH = 25.0, 7.0
	snapPressX         = 34.9
	snapPressY         = 19.2
	snapEndX           = 48.3
	snapEndY           = 23.9
)

// TestResizeGestureSnapsSizeToGrid: the handle drag ends on a fractional size,
// and OnLeftUp must round it onto the grid — 37.9 x 11.2 becomes 40 x 10. The
// position assertions are the other half: the button started off-grid at
// (10.4, 12.7) and must still be pulled to (10, 15), so replacing
// snapSelectionToGrid with the new size snap instead of adding beside it turns
// this red.
func TestResizeGestureSnapsSizeToGrid(t *testing.T) {
	view, scene := newResizeSnapView(t, true)
	btn := addFakeAt(t, scene, "btn", snapBtnX, snapBtnY, snapBtnW, snapBtnH)
	view.Selection().Add(btn)

	dragBottomRightHandle(t, view, snapPressX, snapPressY, snapEndX, snapEndY)

	if w, h := btn.Width(), btn.Height(); w != 40 || h != 10 {
		t.Errorf("after the resize gesture: size = (%v, %v), want (40, 10)", w, h)
	}
	if x, y := btn.Pos(); x != 10 || y != 15 {
		t.Errorf("after the resize gesture: pos = (%v, %v), want (10, 15) — "+
			"the position snap must survive alongside the size snap", x, y)
	}
}

// TestResizeGestureKeepsSizeWhenSnapOff is the gate: with 表单设置 snap turned
// off the identical gesture must leave the fractional size exactly as the
// handle drag committed it. The expectation is derived from the release point
// the view itself computed, so it is the raw width to the last bit — a snap
// that ignored grid.Snap and rounded to 40 x 10 shows up here.
func TestResizeGestureKeepsSizeWhenSnapOff(t *testing.T) {
	view, scene := newResizeSnapView(t, false)
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

// TestResizeGestureSkipsLockedSize: a size-locked widget is dropped from the
// resize command (ResizeDecor.OnEndMoveHandle skips it), so the snap must drop
// it too — otherwise a lock the user set is undone by the grid on a gesture
// aimed at another widget. The locked widget cannot be the dragged one:
// GenDecors hands a locked item no handles at all, so it rides along as a
// second selected item. The dragged button's own snap is asserted in the same
// run, so "locked size unchanged" cannot pass by the snap never running.
func TestResizeGestureSkipsLockedSize(t *testing.T) {
	view, scene := newResizeSnapView(t, true)
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
	if w, h := btn.Width(), btn.Height(); w != 40 || h != 10 {
		t.Fatalf("dragged widget: size = (%v, %v), want (40, 10) — the snap did not run, "+
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
	view, scene := newResizeSnapView(t, snap)

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
	view, scene := newResizeSnapView(t, true)
	btn := addFakeAt(t, scene, "btn", snapBtnX, snapBtnY, snapBtnW, snapBtnH)
	view.Selection().Add(btn)

	dragBottomRightHandle(t, view, snapPressX, snapPressY, 12.7, 14.1)

	if w, h := btn.Width(), btn.Height(); w != 5 || h != 5 {
		t.Errorf("after shrinking below half a pitch: size = (%v, %v), want (5, 5)", w, h)
	}
}
