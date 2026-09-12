package gui

import (
	"math"
	"time"
)

// FrameRatePolicy limits rendering, not input dispatch or timers. Static UI
// draws zero frames; IdleFPS is the ceiling while background visuals change.
// Configure on the UI thread before entering the event loop.
type FrameRatePolicy struct {
	IdleFPS        int
	InteractionFPS int
	BoostDuration  time.Duration
}

var frameRatePolicy = FrameRatePolicy{25, 60, 300 * time.Millisecond}
var lastUIInteraction time.Time

// SetFrameRatePolicy clamps the quiet rate to 1..25 and interactive rate to
// 25..120 FPS. It never requests a repaint merely to meet a minimum FPS.
func SetFrameRatePolicy(policy FrameRatePolicy) {
	policy.IdleFPS = max(1, min(25, policy.IdleFPS))
	policy.InteractionFPS = max(25, min(120, policy.InteractionFPS))
	if policy.BoostDuration <= 0 {
		policy.BoostDuration = 300 * time.Millisecond
	}
	policy.BoostDuration = min(policy.BoostDuration, 2*time.Second)
	frameRatePolicy = policy
}

func UIFrameRatePolicy() FrameRatePolicy { return frameRatePolicy }

func noteUIInteraction(now time.Time) { lastUIInteraction = now }
func interactionActive(now time.Time) bool {
	return !lastUIInteraction.IsZero() && now.Sub(lastUIInteraction) < frameRatePolicy.BoostDuration
}
func frameInterval(now time.Time) time.Duration {
	fps := frameRatePolicy.IdleFPS
	if interactionActive(now) {
		fps = frameRatePolicy.InteractionFPS
	}
	return time.Second / time.Duration(fps)
}

// framePacer also reserves CPU headroom. If one expensive Cairo frame takes
// longer than the nominal interval, do NOT immediately start another frame:
// rest at least 3x its cost at idle (25% rendering duty), 1x while interacting.
// This is a per-window budget, not a promise about total process CPU usage.
type framePacer struct {
	started, finished time.Time
	cost              time.Duration
}

func (p *framePacer) delay(now time.Time) time.Duration {
	if p.finished.IsZero() {
		return 0
	}
	rest := p.cost * 3
	if interactionActive(now) {
		rest = p.cost
	}
	due := p.started.Add(frameInterval(now))
	if recovery := p.finished.Add(rest); recovery.After(due) {
		due = recovery
	}
	return max(0, due.Sub(now))
}
func (p *framePacer) record(start, end time.Time) {
	p.started, p.finished = start, end
	p.cost = max(0, end.Sub(start))
}

// ceilTimerMillis avoids truncating a 16.67 ms frame deadline to 16 ms.
func ceilTimerMillis(delay time.Duration) uint32 {
	return uint32(max(1, min(time.Duration(^uint32(0)), (delay+time.Millisecond-1)/time.Millisecond)))
}

// Degenerate or non-finite client areas have no native update region and
// cannot promise a WM_PAINT completion to the scheduler.
func validRenderSize(w, h float64) bool {
	return w > 0 && h > 0 && w <= 32768 && h <= 32768 && !math.IsNaN(w+h) && !math.IsInf(w+h, 0)
}

// A callback may enter a nested modal and paint there. Do not charge the
// modal's entire lifetime as render cost or overwrite its newer paint timing.
func (p *framePacer) recordUnlessNestedPaint(previous time.Time, start, end time.Time) {
	if p.finished == previous {
		p.record(start, end)
	}
}
