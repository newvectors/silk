package gui

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/uk0/silk/geom"
	"github.com/uk0/silk/win32"
)

// These tests never create HWNDs, enqueue WM_PAINT, or arm a native timer.
// requestRepaint's scheduling is suppressed while checking state; flush cases
// leave every visible window queued (or inactive), so no follow-up timer is due.
func isolateWindowsRenderScheduler(t *testing.T) {
	t.Helper()
	oldMap, oldVisible, oldInvalidate := winMap, renderWindowVisible, invalidateRenderWindow
	oldTimer, oldDeadline, oldPacer, oldServicing := renderTimer, renderDeadline, animationPacer, servicingRenderTimer
	oldAnimations, oldPerf := animManager, GlobalPerfStats
	winMap = make(map[win32.HWND]*Window)
	renderWindowVisible = func(w *Window) bool { return w.widget != nil && w.widget.IsVisible() }
	invalidateRenderWindow = func(win32.HWND, *win32.RECT, bool) bool { t.Fatal("unexpected native invalidation path"); return false }
	renderTimer, renderDeadline, animationPacer, servicingRenderTimer = 0, time.Time{}, framePacer{}, true
	animManager, GlobalPerfStats = &animationManager{}, &PerfStats{}
	isolateFramePolicy(t)
	t.Cleanup(func() {
		winMap, renderWindowVisible, invalidateRenderWindow = oldMap, oldVisible, oldInvalidate
		renderTimer, renderDeadline, animationPacer, servicingRenderTimer = oldTimer, oldDeadline, oldPacer, oldServicing
		animManager, GlobalPerfStats = oldAnimations, oldPerf
	})
}
func windowsRenderFixture(id uintptr, width, height float64) *Window {
	widget := new(Widget)
	widget.Init(widget)
	widget.w, widget.h = width, height
	win := &Window{hWnd: win32.HWND(id), widget: widget}
	widget.win = win
	winMap[win.hWnd] = win
	return win
}
func TestWindowsSchedulerPartialRectClipsAndDoesNotUnionEmptyOrigin(t *testing.T) {
	isolateWindowsRenderScheduler(t)
	win := windowsRenderFixture(1, 300, 200)
	win.requestRepaint(geom.Rect{X: 120.25, Y: 60.5, Width: 10, Height: 8}, false)
	if want := (geom.Rect{X: 120.25, Y: 60.5, Width: 10, Height: 8}); win.repaintRect != want {
		t.Fatalf("partial dirtied origin: %v", win.repaintRect)
	}
	win.requestRepaint(geom.Rect{X: 290, Y: 190, Width: 40, Height: 40}, false)
	if want := (geom.Rect{X: 120.25, Y: 60.5, Width: 179.75, Height: 139.5}); win.repaintRect != want {
		t.Fatalf("clipped union=%v want=%v", win.repaintRect, want)
	}
	win.requestRepaint(geom.Rect{}, true)
	win.requestRepaint(geom.Rect{X: 10, Y: 10, Width: 2, Height: 2}, false)
	if !win.repaintFull || !win.repaintPending || win.repaintRect != (geom.Rect{}) {
		t.Fatalf("partial erased full request: %+v", win)
	}
}
func TestWindowsSchedulerInvalidOrOutsideRectDoesNotQueuePaint(t *testing.T) {
	isolateWindowsRenderScheduler(t)
	win := windowsRenderFixture(1, 300, 200)
	for _, r := range []geom.Rect{{X: 301, Width: 4, Height: 4}, {Y: -10, Width: 3, Height: 2}, {Width: 0, Height: 4}, {Width: -1, Height: 2}, {X: math.NaN(), Width: 2, Height: 2}, {Width: math.Inf(1), Height: 2}} {
		win.requestRepaint(r, false)
		if win.repaintPending || win.paintQueued {
			t.Fatalf("invalid/outside request scheduled: %v", r)
		}
	}
}
func TestWindowsSchedulerQuietHasNoTimerAndPendingUsesPaintCostDeadline(t *testing.T) {
	isolateWindowsRenderScheduler(t)
	now := time.Unix(100, 0)
	win := windowsRenderFixture(1, 300, 200)
	if _, needed := nextRenderDelay(now); needed {
		t.Fatal("quiet UI requested render deadline")
	}
	win.repaintPending = true
	win.framePacer.record(now.Add(-50*time.Millisecond), now)
	if delay, needed := nextRenderDelay(now); !needed || delay != 150*time.Millisecond {
		t.Fatalf("expensive frame recovery=%v %v", delay, needed)
	}
	noteUIInteraction(now)
	if delay, needed := nextRenderDelay(now); !needed || delay != 50*time.Millisecond {
		t.Fatalf("interactive recovery=%v %v", delay, needed)
	}
}
func TestWindowsSchedulerQueuedPaintBlocksAnimationButNotOtherDamage(t *testing.T) {
	isolateWindowsRenderScheduler(t)
	now := time.Unix(100, 0)
	a, b := windowsRenderFixture(1, 300, 200), windowsRenderFixture(2, 300, 200)
	animManager.animations = []*Animation{{state: AnimRunning}}
	a.paintQueued = true
	if _, needed := nextRenderDelay(now); needed {
		t.Fatal("animation advanced while waiting for WM_PAINT")
	}
	b.repaintPending = true
	if delay, needed := nextRenderDelay(now); !needed || delay != 0 {
		t.Fatalf("another damaged window starved: %v %v", delay, needed)
	}
	b.repaintPending = false
	a.paintQueued = false
	a.painting = true
	if _, needed := nextRenderDelay(now); needed {
		t.Fatal("animation advanced recursively during paint")
	}
}
func TestWindowsSchedulerHiddenMinimizedAndZeroSizeCannotFreezeOtherWindows(t *testing.T) {
	isolateWindowsRenderScheduler(t)
	now := time.Unix(100, 0)
	bad := windowsRenderFixture(1, 300, 200)
	windowsRenderFixture(2, 300, 200)
	animManager.animations = []*Animation{{state: AnimRunning}}
	bad.paintQueued = true
	cases := []func(){
		func() { bad.widget.NakedWidget().hidden = true },
		func() { bad.widget.NakedWidget().hidden = false; bad.minimized = true },
		func() { bad.minimized = false; bad.widget.NakedWidget().w = 0 },
		func() { bad.widget.NakedWidget().w = 300; bad.widget.NakedWidget().h = 0 },
		func() { bad.widget.NakedWidget().h = math.NaN() },
	}
	for i, change := range cases {
		change()
		if bad.canRender() {
			t.Fatalf("case %d remained drawable", i)
		}
		if _, needed := nextRenderDelay(now); !needed {
			t.Fatalf("case %d stale queued window froze animations", i)
		}
	}
}
func TestWindowsSchedulerFlushQueuesOneClippedPaintWithoutImmediateRequeue(t *testing.T) {
	isolateWindowsRenderScheduler(t)
	win := windowsRenderFixture(1, 300, 200)
	win.requestRepaint(geom.Rect{X: 120.25, Y: 60.5, Width: 10.25, Height: 8.25}, false)
	calls := 0
	invalidateRenderWindow = func(hwnd win32.HWND, r *win32.RECT, erase bool) bool {
		calls++
		if hwnd != win.hWnd || erase || r == nil || *r != (win32.RECT{Left: 120, Top: 60, Right: 131, Bottom: 69}) {
			t.Fatalf("native bounds=%+v erase=%v", r, erase)
		}
		return true
	}
	flushRenderFrame()
	if calls != 1 || !win.paintQueued || win.repaintPending || win.repaintFull || renderTimer != 0 {
		t.Fatalf("calls=%d queued=%v pending=%v timer=%v", calls, win.paintQueued, win.repaintPending, renderTimer)
	}
	// While the WM_PAINT is queued, more damage must remain pending; another
	// flush must NOT emit a duplicate WM_PAINT or reset the new damage.
	win.requestRepaint(geom.Rect{X: 20, Y: 20, Width: 3, Height: 4}, false)
	flushRenderFrame()
	if calls != 1 || !win.repaintPending || win.repaintRect.Width != 3 {
		t.Fatal("queued paint duplicated or later damage lost")
	}
}
func TestWindowsSchedulerInvalidateFailureDoesNotBusyRetry(t *testing.T) {
	isolateWindowsRenderScheduler(t)
	win := windowsRenderFixture(1, 300, 200)
	win.requestRepaint(geom.Rect{}, true)
	calls := 0
	invalidateRenderWindow = func(win32.HWND, *win32.RECT, bool) bool { calls++; return false }
	flushRenderFrame()
	if calls != 1 || win.repaintPending || win.paintQueued || renderTimer != 0 {
		t.Fatalf("failed invalidation loop: calls=%d pending=%v queued=%v timer=%v", calls, win.repaintPending, win.paintQueued, renderTimer)
	}
}
func TestWindowsSchedulerFinalAnimationEndpointStillGetsPaint(t *testing.T) {
	isolateWindowsRenderScheduler(t)
	win := windowsRenderFixture(1, 300, 200)
	a := NewAnimation(0, 1, time.Millisecond)
	a.state = AnimRunning
	a.start = time.Now().Add(-time.Second)
	animManager.animations = []*Animation{a}
	calls := 0
	invalidateRenderWindow = func(_ win32.HWND, r *win32.RECT, _ bool) bool {
		calls++
		if r != nil {
			t.Fatal("final frame not full")
		}
		return true
	}
	flushRenderFrame()
	if HasActiveAnimations() || calls != 1 || !win.paintQueued {
		t.Fatalf("final animation frame missing: active=%v calls=%d queued=%v", HasActiveAnimations(), calls, win.paintQueued)
	}
}
func TestWindowsSchedulerFinishKeepsDamageRaisedWhileDrawing(t *testing.T) {
	isolateWindowsRenderScheduler(t)
	win := windowsRenderFixture(1, 300, 200)
	win.paintQueued, win.painting = true, true
	win.requestRepaint(geom.Rect{X: 70, Y: 40, Width: 4, Height: 8}, false)
	win.finishScheduledPaint(time.Now().Add(-50 * time.Millisecond))
	if win.paintQueued || win.painting || !win.repaintPending || win.repaintRect.X != 70 || win.repaintRect.Width != 4 {
		t.Fatal("finish discarded damage made inside Draw")
	}
	if win.framePacer.delay(win.framePacer.finished) < 150*time.Millisecond {
		t.Fatal("slow paint immediately fed back into another frame")
	}
}
func TestWindowsScheduledPaintAlwaysValidatesNativeRegion(t *testing.T) {
	src, err := os.ReadFile("window_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	start := strings.Index(text, "func (win *Window) on_WM_PAINT(")
	if start < 0 {
		t.Fatal("WM_PAINT missing")
	}
	end := strings.Index(text[start:], "\nfunc ")
	if end < 0 {
		t.Fatal("WM_PAINT boundary missing")
	}
	body := text[start : start+end]
	for _, wire := range []string{"defer win32.EndPaint(", "defer win.finishScheduledPaint(", "defer winSurface.Finish()"} {
		if !strings.Contains(body, wire) {
			t.Fatalf("panic cleanup missing: %s", wire)
		}
	}
	if strings.Contains(body, "AnimationTick(") {
		t.Fatal("WM_PAINT advances animation and recursively feeds painting")
	}
}

func TestWindowsSchedulerVisibilityTransitionsRecoverQueuedState(t *testing.T) {
	src, err := os.ReadFile("window_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, method := range []string{"on_WM_SIZE", "on_WM_SHOWWINDOW"} {
		prefix := "func (win *Window) " + method + "("
		start := strings.Index(text, prefix)
		if start < 0 {
			t.Fatalf("%s missing", method)
		}
		end := strings.Index(text[start:], "\nfunc ")
		if end < 0 {
			t.Fatalf("%s boundary missing", method)
		}
		body := text[start : start+end]
		if !strings.Contains(body, "win.paintQueued = false") || !strings.Contains(body, "win.Update()") {
			t.Fatalf("%s can retain stale WM_PAINT or fail to repaint after restoring", method)
		}
	}
}

func TestWindowsSchedulerBusySpinnerInvalidatesOnlyItsOwnerRect(t *testing.T) {
	isolateWindowsRenderScheduler(t)
	owner := windowsRenderFixture(1, 1000, 600)
	other := windowsRenderFixture(2, 1000, 600)
	spinner := NewSpinner()
	spinner.SetBounds(20, 30, 24, 24)
	spinner.SetParent(owner.widget)
	defer spinner.SetBusy(false)
	owner.repaintPending, owner.repaintFull = false, false
	owner.repaintRect = geom.Rect{}
	calls := 0
	invalidateRenderWindow = func(hwnd win32.HWND, r *win32.RECT, erase bool) bool {
		calls++
		if hwnd != owner.hWnd || r == nil || *r != (win32.RECT{Left: 20, Top: 30, Right: 44, Bottom: 54}) || erase {
			t.Fatalf("busy widget invalidated unrelated pixels: %v %+v", hwnd, r)
		}
		return true
	}
	flushRenderFrame()
	if calls != 1 || !owner.paintQueued || other.paintQueued || other.repaintPending {
		t.Fatal("spinner repainted another window")
	}
}
