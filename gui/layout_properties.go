package gui

import (
	"github.com/uk0/silk/core"
	"math"
)

func layoutDimension(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0
	}
	return v
}
func enumLayoutPadding(list core.IPropertyList, get func() Padding, set func(Padding)) {
	list.AddProperty("左边距", func() float64 { return get().L }, func(v float64) { p := get(); p.L = layoutDimension(v); set(p) })
	list.AddProperty("右边距", func() float64 { return get().R }, func(v float64) { p := get(); p.R = layoutDimension(v); set(p) })
	list.AddProperty("上边距", func() float64 { return get().T }, func(v float64) { p := get(); p.T = layoutDimension(v); set(p) })
	list.AddProperty("下边距", func() float64 { return get().B }, func(v float64) { p := get(); p.B = layoutDimension(v); set(p) })
}

// A container smaller than its insets has no interior, not a reflected
// rectangle outside its bounds. This keeps tiny layouts from painting over
// their siblings when margins exceed the available width/height.
func applyLayoutPadding(p Padding, w, h float64) (float64, float64, float64, float64) {
	left, top := math.Min(layoutDimension(p.L), w), math.Min(layoutDimension(p.T), h)
	return left, top, math.Max(0, w-layoutDimension(p.L)-layoutDimension(p.R)), math.Max(0, h-layoutDimension(p.T)-layoutDimension(p.B))
}
