package gui

import (
	"testing"
	"time"
)

func isolateFramePolicy(t *testing.T) {
	t.Helper()
	policy, input := frameRatePolicy, lastUIInteraction
	lastUIInteraction = time.Time{}
	frameRatePolicy = FrameRatePolicy{25, 60, 300 * time.Millisecond}
	t.Cleanup(func() { frameRatePolicy, lastUIInteraction = policy, input })
}
func TestFramePolicyQuietInteractionAndCooldown(t *testing.T) {
	isolateFramePolicy(t)
	now := time.Unix(100, 0)
	if got := frameInterval(now); got != 40*time.Millisecond {
		t.Fatal(got)
	}
	noteUIInteraction(now)
	if got := frameInterval(now); got != time.Second/60 {
		t.Fatal(got)
	}
	if got := frameInterval(now.Add(300 * time.Millisecond)); got != 40*time.Millisecond {
		t.Fatal(got)
	}
	SetFrameRatePolicy(FrameRatePolicy{IdleFPS: 999, InteractionFPS: 999, BoostDuration: time.Hour})
	if frameRatePolicy != (FrameRatePolicy{25, 120, 2 * time.Second}) {
		t.Fatal(frameRatePolicy)
	}
	SetFrameRatePolicy(FrameRatePolicy{})
	if frameRatePolicy.IdleFPS != 1 || frameRatePolicy.InteractionFPS != 25 {
		t.Fatal(frameRatePolicy)
	}
}
func TestFramePacerExpensivePaintCannotFeedBackImmediately(t *testing.T) {
	isolateFramePolicy(t)
	now := time.Unix(100, 0)
	p := framePacer{}
	if p.delay(now) != 0 {
		t.Fatal("first paint delayed")
	}
	// Old WM_PAINT -> serviceUIFrame ticked immediately because 50ms > 16ms.
	p.record(now, now.Add(50*time.Millisecond))
	if got := p.delay(now.Add(50 * time.Millisecond)); got != 150*time.Millisecond {
		t.Fatal(got)
	}
	noteUIInteraction(now.Add(50 * time.Millisecond))
	if got := p.delay(now.Add(50 * time.Millisecond)); got != 50*time.Millisecond {
		t.Fatal(got)
	}
	if p.delay(now.Add(101*time.Millisecond)) != 0 {
		t.Fatal("deadline did not become ready")
	}
}
func TestFramePacerFastPaintRespectsFPSCeiling(t *testing.T) {
	isolateFramePolicy(t)
	now := time.Unix(100, 0)
	p := framePacer{}
	p.record(now, now.Add(time.Millisecond))
	if got := p.delay(now.Add(time.Millisecond)); got != 39*time.Millisecond {
		t.Fatal(got)
	}
	noteUIInteraction(now.Add(time.Millisecond))
	if got := p.delay(now.Add(time.Millisecond)); got != time.Second/60-time.Millisecond {
		t.Fatal(got)
	}
	if got := ceilTimerMillis(time.Second / 60); got != 17 {
		t.Fatal(got)
	}
	if got := ceilTimerMillis(0); got != 1 {
		t.Fatal(got)
	}
}

func TestAnimationTickModalReentryCannotRecurseOrDemandContinuousFrames(t *testing.T) {
	oldManager, oldActive, oldPerf := animManager, animationTickActive, GlobalPerfStats
	animManager = &animationManager{}
	animationTickActive = false
	GlobalPerfStats = &PerfStats{}
	defer func() { animManager, animationTickActive, GlobalPerfStats = oldManager, oldActive, oldPerf }()
	calls := 0
	a := NewAnimation(0, 1, time.Second).OnUpdate(func(float64) {
		calls++
		if continuousFrameWanted() {
			t.Error("callback's nested modal demanded another animation tick")
		}
		AnimationTick()
	})
	a.Start()
	AnimationTick()
	if calls != 1 || animationTickActive || !continuousFrameWanted() {
		t.Fatalf("calls=%d active=%v", calls, animationTickActive)
	}
}

func TestFramePacerModalLifetimeDoesNotBecomePaintCost(t *testing.T) {
	isolateFramePolicy(t)
	start := time.Unix(100, 0)
	p := framePacer{}
	before := p.finished
	p.record(start.Add(10*time.Second), start.Add(10*time.Second+time.Millisecond))
	nested := p
	p.recordUnlessNestedPaint(before, start, start.Add(30*time.Second))
	if p != nested {
		t.Fatal("modal wall time overwrote actual paint cost")
	}
	p.recordUnlessNestedPaint(p.finished, start.Add(31*time.Second), start.Add(31*time.Second+time.Millisecond))
	if p.cost != time.Millisecond {
		t.Fatal(p.cost)
	}
}
