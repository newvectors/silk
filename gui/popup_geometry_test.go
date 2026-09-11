package gui

import (
	"math"
	"testing"

	"github.com/uk0/silk/geom"
)

func popupTestMonitors() []popupMonitorArea {
	return []popupMonitorArea{
		{bounds: geom.Rect{Width: 1920, Height: 1080}, workArea: geom.Rect{Y: 24, Width: 1920, Height: 1016}},
		{bounds: geom.Rect{X: -1280, Width: 1280, Height: 1024}, workArea: geom.Rect{X: -1280, Y: 24, Width: 1280, Height: 1000}},
		{bounds: geom.Rect{X: 1920, Width: 2560, Height: 1440}, workArea: geom.Rect{X: 1920, Y: 40, Width: 2560, Height: 1360}},
		{bounds: geom.Rect{Y: -1200, Width: 1920, Height: 1200}, workArea: geom.Rect{Y: -1176, Width: 1920, Height: 1176}},
	}
}

func TestPopupWorkAreaSelectsAnchorMonitor(t *testing.T) {
	monitors := popupTestMonitors()
	for _, tc := range []struct {
		name string
		x, y float64
		want int
	}{
		{"negative-left", -400, 500, 1},
		{"right", 2300, 500, 2},
		{"above", 300, -500, 3},
		{"shared-right-edge", 1920, 500, 2},
		{"shared-origin-corner", 0, 0, 0},
		{"left-before-edge", -0.25, 500, 1},
		{"right-taskbar-anchor", 2000, 20, 2},
		{"nearest-outside-desktop", 4600, 1500, 2},
		{"nearest-in-monitor-gap", -100, -500, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := selectPopupWorkArea(monitors, tc.x, tc.y)
			if !ok || got != monitors[tc.want].workArea {
				t.Fatalf("anchor (%g,%g): got %+v, want %+v", tc.x, tc.y, got, monitors[tc.want].workArea)
			}
		})
	}
	// Invalid/disconnected monitor reports do not become an accidental
	// primary screen, and callers can fall back when no area is usable.
	bad := []popupMonitorArea{{workArea: geom.Rect{Width: -1, Height: 10}}, {workArea: geom.Rect{X: math.NaN(), Width: 10, Height: 10}}}
	if _, ok := selectPopupWorkArea(bad, 0, 0); ok {
		t.Fatal("invalid monitor geometry was accepted")
	}
	if _, ok := selectPopupWorkArea(nil, 0, 0); ok {
		t.Fatal("empty monitor list selected a work area")
	}
}

func TestPopupPlacementOnNegativeRightAndUpperScreens(t *testing.T) {
	monitors := popupTestMonitors()
	for _, tc := range []struct {
		name                 string
		x, y, rw, rh, pw, ph float64
		vertical             bool
		wantX, wantY         float64
	}{
		{name: "left-context-menu", x: -20, y: 400, pw: 200, ph: 150, wantX: -220, wantY: 400},
		{name: "right-context-menu", x: 2500, y: 500, pw: 200, ph: 150, wantX: 2500, wantY: 500},
		{name: "upper-screen", x: 200, y: -500, pw: 200, ph: 150, wantX: 200, wantY: -500},
		{name: "boundary-right-taskbar", x: 1920, y: 0, pw: 200, ph: 150, wantX: 1920, wantY: 40},
		{name: "right-submenu-flips-left", x: 4330, y: 300, rw: 100, rh: 24, pw: 240, ph: 150, wantX: 4090, wantY: 300},
		{name: "dropdown-flips-up", x: 2400, y: 1300, rw: 100, rh: 26, pw: 240, ph: 200, vertical: true, wantX: 2400, wantY: 1100},
		{name: "oversized-right", x: 2400, y: 500, pw: 6000, ph: 2000, wantX: 1920, wantY: 40},
		{name: "oversized-negative", x: -400, y: 500, pw: 2000, ph: 2000, wantX: -1280, wantY: 24},
	} {
		t.Run(tc.name, func(t *testing.T) {
			area, ok := selectPopupWorkArea(monitors, tc.x, tc.y)
			if !ok {
				t.Fatal("missing fixture screen")
			}
			x, y := popupPositionInWorkArea(area, tc.x, tc.y, tc.rw, tc.rh, tc.pw, tc.ph, tc.vertical, 0)
			if x != tc.wantX || y != tc.wantY {
				t.Fatalf("got (%g,%g), want (%g,%g)", x, y, tc.wantX, tc.wantY)
			}
		})
	}
}

func TestLayoutPopupEntryPointsUseGlobalAnchorWorkArea(t *testing.T) {
	previous := popupWorkAreaAt
	t.Cleanup(func() { popupWorkAreaAt = previous })
	var anchors [][2]float64
	popupWorkAreaAt = func(x, y float64) geom.Rect {
		anchors = append(anchors, [2]float64{x, y})
		area, _ := selectPopupWorkArea(popupTestMonitors(), x, y)
		return area
	}
	popup := NewLabel("menu")
	popup.SetSize(200, 150)
	LayoutPopup1(popup, -20, 400)
	if popup.X() != -220 || popup.Y() != 400 {
		t.Fatalf("LayoutPopup1 did not retain negative-screen placement: %v", popup.Bounds1())
	}
	LayoutPopup(popup, 2400, 1300, 100, 26, true, 0)
	if popup.X() != 2400 || popup.Y() != 1150 {
		t.Fatalf("LayoutPopup did not use right-screen work area: %v", popup.Bounds1())
	}
	if len(anchors) != 2 || anchors[0] != [2]float64{-20, 400} || anchors[1] != [2]float64{2400, 1300} {
		t.Fatalf("monitor provider received non-global/offset anchor: %v", anchors)
	}
	if popup.Window() != nil {
		t.Fatal("headless placement created a native window")
	}
}

func TestTooltipPlacementUsesCursorScreenBeforeOffset(t *testing.T) {
	for _, tc := range []struct {
		name         string
		x, y, w, h   float64
		wantX, wantY float64
	}{
		{"near-left-screen-edge", -4, 400, 200, 60, -200, 418},
		{"right-bottom-flips-up", 4400, 1380, 200, 60, 4280, 1316},
		{"upper-screen", 200, -500, 200, 60, 212, -482},
		{"oversized", -400, 400, 2000, 2000, -1280, 24},
	} {
		t.Run(tc.name, func(t *testing.T) {
			area, _ := selectPopupWorkArea(popupTestMonitors(), tc.x, tc.y)
			x, y := tooltipPositionInWorkArea(area, tc.x, tc.y, tc.w, tc.h)
			if x != tc.wantX || y != tc.wantY {
				t.Fatalf("got (%g,%g), want (%g,%g)", x, y, tc.wantX, tc.wantY)
			}
		})
	}
}
