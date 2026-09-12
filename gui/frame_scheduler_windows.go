package gui

import (
	"github.com/uk0/silk/geom"
	"github.com/uk0/silk/win32"
	"math"
	"time"
)

// Only a deadline timer may advance continuous animation. WM_PAINT never
// schedules another immediate full repaint merely because painting was slow.
var renderTimer Timer
var renderDeadline time.Time
var animationPacer framePacer
var servicingRenderTimer bool

// Narrow native seams let tests exercise scheduling without creating an HWND.
var renderWindowVisible = func(win *Window) bool { return win.IsVisible() }
var invalidateRenderWindow = win32.InvalidateRect

func (win *Window) canRender() bool {
	return win != nil && win.hWnd != 0 && win.widget != nil && validRenderSize(win.widget.Width(), win.widget.Height()) && !win.minimized && renderWindowVisible(win)
}

func (win *Window) requestRepaint(rect geom.Rect, full bool) {
	if win.hWnd == 0 || win.widget == nil {
		return
	}
	if !full && (rect.Width <= 0 || rect.Height <= 0 || math.IsNaN(rect.X+rect.Y+rect.Width+rect.Height) || math.IsInf(rect.X+rect.Y+rect.Width+rect.Height, 0)) {
		return
	}
	if !full {
		rect = rect.IntersectCopy(geom.Rect{Width: win.widget.Width(), Height: win.widget.Height()})
		if rect.Width <= 0 || rect.Height <= 0 {
			return
		}
	}
	win.repaintPending = true
	if full {
		win.repaintFull = true
		win.repaintRect = geom.Rect{}
	} else if !win.repaintFull {
		if win.repaintRect.Width <= 0 || win.repaintRect.Height <= 0 {
			win.repaintRect = rect
		} else {
			win.repaintRect = win.repaintRect.UniteCopy(rect)
		}
	}
	scheduleRenderFrame()
}

// scheduleRenderFrame coalesces invalidations and uses a one-shot deadline;
// no pending damage/animation means no render timer and no synthetic paints.
func scheduleRenderFrame() {
	if servicingRenderTimer {
		return
	}
	now := time.Now()
	delay, needed := nextRenderDelay(now)
	if !needed {
		renderTimer.Stop()
		renderDeadline = time.Time{}
		return
	}
	deadline := now.Add(delay)
	// Keeping an earlier already-armed timer is harmless; resetting it for
	// every mouse event would postpone it forever during a drag.
	if renderTimer != 0 && !deadline.Before(renderDeadline) {
		return
	}
	renderDeadline = deadline
	if !renderTimer.Start(ceilTimerMillis(delay), flushRenderFrame) {
		renderDeadline = time.Time{}
	}
}

func nextRenderDelay(now time.Time) (time.Duration, bool) {
	var delay time.Duration
	needed, drawable, awaitingPaint := false, false, false
	choose := func(d time.Duration) {
		if !needed || d < delay {
			delay = d
		}
		needed = true
	}
	for _, win := range winMap {
		if !win.canRender() {
			continue
		}
		drawable = true
		if win.paintQueued || win.painting {
			awaitingPaint = true
			continue
		}
		if win.repaintPending {
			choose(win.framePacer.delay(now))
		}
	}
	// Wait for submitted WM_PAINTs before advancing animation again. Their
	// cost becomes part of the next deadline, preventing a repaint feedback loop.
	if drawable && !awaitingPaint && continuousFrameWanted() {
		choose(animationPacer.delay(now))
	}
	return delay, needed
}

func flushRenderFrame() {
	renderTimer.Stop()
	renderDeadline = time.Time{}
	servicingRenderTimer = true
	defer func() { servicingRenderTimer = false; scheduleRenderFrame() }()
	now := time.Now()
	waiting, drawable := false, false
	for _, win := range winMap {
		if win.canRender() {
			drawable = true
			waiting = waiting || win.paintQueued || win.painting
		}
	}
	if drawable && !waiting && animationPacer.delay(now) == 0 && continuousFrameWanted() {
		fullRepaint := animationsRequireFullRepaint()
		pacerBefore := animationPacer.finished
		// Permit a synchronous modal opened by an animation callback to schedule
		// its own damage. AnimationTick itself prevents recursive animation ticks.
		servicingRenderTimer = false
		AnimationTick()
		servicingRenderTimer = true
		// Paint the FINAL animation state too: HasActiveAnimations may become
		// false in this tick, but the endpoint must still reach the screen.
		for _, win := range winMap {
			if fullRepaint && win.canRender() {
				win.requestRepaint(geom.Rect{}, true)
			}
		}
		animationPacer.recordUnlessNestedPaint(pacerBefore, now, time.Now())
	}
	for _, win := range winMap {
		if !win.canRender() || !win.repaintPending || win.paintQueued || win.painting || win.framePacer.delay(time.Now()) > 0 {
			continue
		}
		var rect *win32.RECT
		if !win.repaintFull {
			r := win.repaintRect
			rect = &win32.RECT{Left: int32(math.Floor(r.X)), Top: int32(math.Floor(r.Y)), Right: int32(math.Ceil(r.X + r.Width)), Bottom: int32(math.Ceil(r.Y + r.Height))}
		}
		if invalidateRenderWindow(win.hWnd, rect, false) {
			win.paintQueued = true
			win.repaintPending, win.repaintFull = false, false
			win.repaintRect = geom.Rect{}
		} else {
			// A destroyed/invalid HWND must not create a zero-delay retry loop.
			win.repaintPending = false
		}
	}
}

func (win *Window) finishScheduledPaint(start time.Time) {
	end := time.Now()
	win.painting, win.paintQueued = false, false
	win.framePacer.record(start, end)
	// Include actual paint cost, not just the cheap animation callback.
	animationPacer.record(start, end)
	GlobalPerfStats.RecordPaint(end.Sub(start))
	GlobalPerfStats.RecordFrame()
	scheduleRenderFrame()
}
