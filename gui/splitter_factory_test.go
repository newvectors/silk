package gui

import (
	"fmt"
	"testing"

	"github.com/uk0/silk/core"
)

// factorySplitter builds a Splitter the way the designer palette and the form
// loader do -- reflection plus Init, with NewSplitter nowhere in the picture --
// and gives it a size and the requested number of panes.
func factorySplitter(t *testing.T, kids int) *Splitter {
	t.Helper()
	s, ok := core.New("gui.Splitter").(*Splitter)
	if !ok {
		t.Fatal("gui.Splitter factory did not produce a *Splitter")
	}
	s.SetSize(200, 50)
	for i := 0; i < kids; i++ {
		s.AddWidget(NewLabel("pane"))
	}
	return s
}

// mouseMove drives OnMouseMove and turns a panic into a readable failure so the
// remaining child counts still get exercised.
func mouseMove(t *testing.T, s *Splitter, what string, x, y float64) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s: OnMouseMove(%v, %v) panicked with no drag in progress: %v", what, x, y, r)
		}
	}()
	s.OnMouseMove(x, y)
}

// sameSizes compares two proportional size vectors within splitterEps.
func sameSizes(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if d := a[i] - b[i]; d > splitterEps || d < -splitterEps {
			return false
		}
	}
	return true
}

// A factory-built splitter has never been clicked, so the very first mouse move
// must take the hover path, not the drag path. dragging defaults to 0 without
// the Init sentinel, which sends OnMouseMove into the drag branch and indexes
// sizes[dragging] and sizes[dragging+1] on a splitter that may have neither.
func TestFactorySplitterMouseMoveDoesNotDrag(t *testing.T) {
	for _, kids := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("kids=%d", kids), func(t *testing.T) {
			s := factorySplitter(t, kids)
			before := s.Sizes()

			mouseMove(t, s, fmt.Sprintf("factory splitter with %d children", kids), 100, 25)

			if s.dragging >= 0 {
				t.Errorf("after a bare mouse move the splitter reports dragging handle %d; want -1, no drag was ever started", s.dragging)
			}
			if after := s.Sizes(); !sameSizes(before, after) {
				t.Errorf("a bare mouse move resized the panes: sizes %v -> %v; a drag must start with a click on a handle", before, after)
			}
		})
	}
}

// The first mouse move over the gap between two panes must mark that handle
// hovered: it is the affordance Draw paints and the point OnLeftDown starts a
// drag from. A factory splitter with handleSize 0 has zero-width handles that no
// mouse position can hit, and one that starts with hoverHandle 0 paints a
// highlight before the mouse has arrived.
func TestFactorySplitterHandleIsGrabbable(t *testing.T) {
	s := factorySplitter(t, 2)

	if s.hoverHandle != -1 {
		t.Errorf("a factory splitter reports handle %d hovered before any mouse event; want -1", s.hoverHandle)
	}

	// 200px wide, handleSize 5 -> two 97.5px panes with the handle at [97.5, 102.5).
	s.OnMouseMove(100, 25)

	if s.hoverHandle != 0 {
		t.Errorf("mouse in the gap between the two panes: hovered handle %d, want 0 (handleSize %v leaves nothing to grab)", s.hoverHandle, s.HandleSize())
	}
}

// NewSplitter must still produce exactly the state it produced before the
// sentinels moved into Init: only vertical comes from the argument, everything
// else is an Init default.
func TestNewSplitterConstructorParity(t *testing.T) {
	for _, vertical := range []bool{false, true} {
		s := NewSplitter(vertical)

		if s.Self() != IWidget(s) {
			t.Errorf("NewSplitter(%v): Self() = %v, want the splitter itself", vertical, s.Self())
		}
		if s.vertical != vertical {
			t.Errorf("NewSplitter(%v): vertical = %v", vertical, s.vertical)
		}
		if s.handleSize != 5 {
			t.Errorf("NewSplitter(%v): handleSize = %v, want 5", vertical, s.handleSize)
		}
		if s.dragging != -1 {
			t.Errorf("NewSplitter(%v): dragging = %v, want -1", vertical, s.dragging)
		}
		if s.hoverHandle != -1 {
			t.Errorf("NewSplitter(%v): hoverHandle = %v, want -1", vertical, s.hoverHandle)
		}
		if s.lastClickHandle != -1 {
			t.Errorf("NewSplitter(%v): lastClickHandle = %v, want -1", vertical, s.lastClickHandle)
		}
		if s.dragStart != 0 {
			t.Errorf("NewSplitter(%v): dragStart = %v, want 0", vertical, s.dragStart)
		}
		if !s.lastClickTime.IsZero() {
			t.Errorf("NewSplitter(%v): lastClickTime = %v, want the zero time", vertical, s.lastClickTime)
		}
		if s.sizes != nil {
			t.Errorf("NewSplitter(%v): sizes = %v, want nil until a pane is added", vertical, s.sizes)
		}
		if s.collapsedSizes == nil {
			t.Errorf("NewSplitter(%v): collapsedSizes is nil", vertical)
		} else if len(s.collapsedSizes) != 0 {
			t.Errorf("NewSplitter(%v): collapsedSizes = %v, want empty", vertical, s.collapsedSizes)
		}
	}
}
