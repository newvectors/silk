package ged

import "github.com/uk0/silk/graph"

// runtimePosition converts graph's shared scene coordinates at a parent
// boundary. Native widget children always use parent-local coordinates, even
// when the designer's nested graph items deliberately share the same space.
func runtimePosition(f *FakeWidget) (float64, float64) {
	parent, ok := f.Parent().(*FakeWidget)
	if !ok {
		return f.Pos()
	}
	x, y := f.MapToScene(f.Pos())
	px, py := parent.MapToScene(parent.Pos())
	return x - px, y - py
}

// translateFreeChildren carries contents when an explicit geometry setter or
// an enclosing automatic layout changes a free-positioned container's origin.
// Graph move/resize commands already carry their own descendants, so this is
// deliberately not a generic OnMove hook (which would double-apply gestures).
func translateFreeChildren(parent graph.IItem, dx, dy float64) {
	if (dx == 0 && dy == 0) || parent.HasLocalCoord() {
		return
	}
	for _, child := range parent.Children() {
		child.SetPos(child.X()+dx, child.Y()+dy)
		if manager, ok := child.(interface{ ManagesChildGeometry() bool }); ok && manager.ManagesChildGeometry() {
			continue
		}
		translateFreeChildren(child, dx, dy)
	}
}

func (f *FakeWidget) setDesignX(x float64) {
	dx := x - f.X()
	f.SetX(x)
	if !f.ManagesChildGeometry() {
		translateFreeChildren(f, dx, 0)
	}
}
func (f *FakeWidget) setDesignY(y float64) {
	dy := y - f.Y()
	f.SetY(y)
	if !f.ManagesChildGeometry() {
		translateFreeChildren(f, 0, dy)
	}
}
