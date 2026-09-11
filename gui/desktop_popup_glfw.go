//go:build !windows

package gui

import (
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/uk0/silk/geom"
)

func platformPopupWorkArea(x, y float64) geom.Rect {
	if !glfwReady {
		return fallbackPopupWorkArea()
	}
	var areas []popupMonitorArea
	for _, monitor := range glfw.GetMonitors() {
		if monitor == nil {
			continue
		}
		wx, wy, width, height := monitor.GetWorkarea()
		work := geom.Rect{X: float64(wx), Y: float64(wy), Width: float64(width), Height: float64(height)}
		bounds := work
		if mode := monitor.GetVideoMode(); mode != nil {
			mx, my := monitor.GetPos()
			bounds = geom.Rect{X: float64(mx), Y: float64(my), Width: float64(mode.Width), Height: float64(mode.Height)}
		}
		if !validPopupArea(work) {
			work = bounds
		}
		areas = append(areas, popupMonitorArea{bounds: bounds, workArea: work})
	}
	if area, ok := selectPopupWorkArea(areas, x, y); ok {
		return area
	}
	return fallbackPopupWorkArea()
}
