package gui

import (
	"testing"

	"github.com/uk0/silk/geom"
)

func TestWidgetUpdateRectPreservesExtentThroughNestedOrigins(t *testing.T) {
	form := NewForm()
	form.SetBounds(400, 300, 800, 600)
	owner := &Window{}
	form.NakedWidget().win = owner // metadata only, no native window
	parent := new(Widget)
	parent.Init(parent)
	parent.SetBounds(40, 30, 200, 150)
	parent.SetParent(form)
	child := new(Widget)
	child.Init(child)
	child.SetBounds(12, 8, 120, 90)
	child.SetParent(parent)
	count := 0
	var got geom.Rect
	child.invalidateRectInWindow(2.5, 4.25, 3, 9, func(win *Window, x, y, w, h float64) {
		count++
		if win != owner {
			t.Fatal("wrong native owner")
		}
		got = geom.Rect{X: x, Y: y, Width: w, Height: h}
	})
	want := geom.Rect{X: 54.5, Y: 42.25, Width: 3, Height: 9}
	if count != 1 || got != want {
		t.Fatalf("dirty=%+v count=%d want=%+v", got, count, want)
	}
	// A root's desktop origin is not part of its own client dirty rectangle.
	form.invalidateRectInWindow(1, 2, 3, 4, func(_ *Window, x, y, w, h float64) { got = geom.Rect{X: x, Y: y, Width: w, Height: h} })
	if got != (geom.Rect{X: 1, Y: 2, Width: 3, Height: 4}) {
		t.Fatalf("root desktop origin leaked: %+v", got)
	}
}
func TestWidgetUpdateRectEmptyOrDetachedDoesNotInvalidate(t *testing.T) {
	w := new(Widget)
	w.Init(w)
	w.SetSize(100, 80)
	calls := 0
	sink := func(*Window, float64, float64, float64, float64) { calls++ }
	w.invalidateRectInWindow(0, 0, 3, 4, sink)
	w.NakedWidget().win = &Window{}
	for _, size := range [][2]float64{{0, 1}, {1, 0}, {-1, 2}, {2, -1}} {
		w.invalidateRectInWindow(0, 0, size[0], size[1], sink)
	}
	if calls != 0 {
		t.Fatalf("empty/detached invalidation calls=%d", calls)
	}
}
