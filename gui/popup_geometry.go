package gui

import (
	"math"

	"github.com/uk0/silk/geom"
)

// A monitor's bounds select the screen (including its taskbar); workArea is
// the usable rectangle in the same global screen-coordinate system.
type popupMonitorArea struct {
	bounds   geom.Rect
	workArea geom.Rect
}

// Kept separate from the placement math so tests never enumerate native
// monitors. The production provider is called on the UI thread at popup time,
// rather than caching monitor handles across display-configuration changes.
var popupWorkAreaAt = platformPopupWorkArea

func validPopupArea(r geom.Rect) bool {
	return r.Width > 0 && r.Height > 0 &&
		!math.IsNaN(r.X+r.Y+r.Width+r.Height) &&
		!math.IsInf(r.X+r.Y+r.Width+r.Height, 0)
}

func selectPopupWorkArea(monitors []popupMonitorArea, x, y float64) (geom.Rect, bool) {
	var nearest geom.Rect
	best := math.Inf(1)
	for _, monitor := range monitors {
		if !validPopupArea(monitor.workArea) {
			continue
		}
		bounds := monitor.bounds
		if !validPopupArea(bounds) {
			bounds = monitor.workArea
		}
		// Half-open bounds resolve a shared screen edge to the screen that
		// starts there, regardless of primary-monitor enumeration order.
		if x >= bounds.X && x < bounds.Right() && y >= bounds.Y && y < bounds.Bottom() {
			return monitor.workArea, true
		}
		dx := math.Max(math.Max(bounds.X-x, x-bounds.Right()), 0)
		dy := math.Max(math.Max(bounds.Y-y, y-bounds.Bottom()), 0)
		if distance := dx*dx + dy*dy; distance < best {
			nearest, best = monitor.workArea, distance
		}
	}
	return nearest, best < math.Inf(1)
}

func fallbackPopupWorkArea() geom.Rect {
	x, y, w, h := DesktopArea()
	return geom.Rect{X: x, Y: y, Width: w, Height: h}
}

func clampPopupToWorkArea(area geom.Rect, x, y, w, h float64) (float64, float64) {
	// An oversized popup cannot fit without changing its content/layout.
	// Keep its origin at the work-area edge, not on a different monitor.
	x = math.Max(area.X, math.Min(x, area.Right()-math.Max(0, w)))
	y = math.Max(area.Y, math.Min(y, area.Bottom()-math.Max(0, h)))
	return x, y
}

func popupPositionInWorkArea(area geom.Rect, xref, yref, wref, href, pw, ph float64, vertical bool, overlap float64) (float64, float64) {
	x0, y0 := xref, yref
	x1, y1 := xref+wref, yref+href
	var x, y float64
	if vertical {
		y0, y1 = y0+overlap, y1-overlap
		x, y = x0, y1
		if y+ph > area.Bottom() {
			y = y0 - ph
		}
	} else {
		x0, x1 = x0+overlap, x1-overlap
		x, y = x1, y0
		if x+pw > area.Right() {
			x = x0 - pw
		}
	}
	return clampPopupToWorkArea(area, x, y, pw, ph)
}

func tooltipPositionInWorkArea(area geom.Rect, xref, yref, w, h float64) (float64, float64) {
	x, y := xref+12, yref+18
	if y+h > area.Bottom() {
		y = yref - h - 4
	}
	return clampPopupToWorkArea(area, x, y, w, h)
}
