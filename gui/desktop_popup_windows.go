package gui

import (
	"math"
	"syscall"
	"unsafe"

	"github.com/uk0/silk/geom"
	"github.com/uk0/silk/win32"
)

var (
	popupUser32          = syscall.NewLazyDLL("user32.dll")
	popupMonitorFromRect = popupUser32.NewProc("MonitorFromRect")
	popupGetMonitorInfo  = popupUser32.NewProc("GetMonitorInfoW")
)

func platformPopupWorkArea(x, y float64) geom.Rect {
	// Use a one-pixel anchor rectangle, avoiding POINT-by-value ABI issues
	// when calling MonitorFromPoint through syscall on 32/64-bit Windows.
	left, top := int32(math.Floor(x)), int32(math.Floor(y))
	anchor := win32.RECT{Left: left, Top: top, Right: left + 1, Bottom: top + 1}
	monitor, _, _ := popupMonitorFromRect.Call(uintptr(unsafe.Pointer(&anchor)), win32.MONITOR_DEFAULTTONEAREST)
	info := win32.MONITORINFO{}
	info.CbSize = uint32(unsafe.Sizeof(info))
	ok, _, _ := popupGetMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info)))
	if monitor != 0 && ok != 0 {
		work := info.RcWork
		area := geom.Rect{X: float64(work.Left), Y: float64(work.Top), Width: float64(work.Right - work.Left), Height: float64(work.Bottom - work.Top)}
		if validPopupArea(area) {
			return area
		}
	}
	return fallbackPopupWorkArea()
}
