package gui

import "math"

const dialogButtonBarHeight = 56.0

// Dialog buttons are not a left-aligned menu. Keep a comfortable click target
// and a trailing inset; overriding Layout also survives child relayout requests.
type dialogButtonBar struct{ ButtonBox }

func (bar *dialogButtonBar) requiredWidth() float64 {
	w, count := 48.0, 0
	for _, child := range bar.Children() {
		if child.IsVisible() {
			w += math.Max(88, child.SizeHints().Width)
			count++
		}
	}
	return w + float64(max(0, count-1))*8
}

func (bar *dialogButtonBar) Layout() {
	x := math.Max(0, bar.Width()-24)
	children := bar.Children()
	for i := len(children) - 1; i >= 0; i-- {
		child := children[i]
		if !child.IsVisible() {
			continue
		}
		hints := child.SizeHints()
		w := math.Max(88, hints.Width)
		h := math.Min(math.Max(32, hints.Height), math.Max(0, bar.Height()-16))
		x -= w
		child.SetBounds(math.Max(0, x), math.Max(0, (bar.Height()-h)/2), w, h)
		x -= 8
	}
}
