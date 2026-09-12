package gui

import (
	"testing"
	"time"
)

func TestAnimationRegistryRepeatedStartDoesNotMultiplyWork(t *testing.T) {
	spinnerIsolateAnims(t)
	calls := 0
	a := NewAnimation(0, 1, time.Second).OnUpdate(func(float64) { calls++ })
	for i := 0; i < 100; i++ {
		a.Start()
	}
	if len(animManager.animations) != 1 {
		t.Fatal("duplicate animation registrations")
	}
	AnimationTick()
	if calls != 1 {
		t.Fatal(calls)
	}
	a.Stop()
	if len(animManager.animations) != 0 {
		t.Fatal("Stop retained duplicate reference")
	}
}
func TestAnimationRegistryCallbackStartsNewAnimationWithoutLosingIt(t *testing.T) {
	spinnerIsolateAnims(t)
	next := NewAnimation(0, 1, time.Second)
	first := NewAnimation(0, 1, time.Millisecond).OnDone(func() { next.Start() })
	first.Start()
	first.start = time.Now().Add(-time.Second)
	AnimationTick()
	if len(animManager.animations) != 1 || animManager.animations[0] != next || !HasActiveAnimations() {
		t.Fatal("OnDone's new animation was lost")
	}
	next.Stop()
}
func TestAnimationRegistryCallbackCanStopSiblingAndRegisterReplacement(t *testing.T) {
	spinnerIsolateAnims(t)
	sibling := NewAnimation(0, 1, time.Second)
	replacement := NewAnimation(0, 1, time.Second)
	driver := NewAnimation(0, 1, time.Second).OnUpdate(func(float64) { sibling.Stop(); replacement.Start() })
	driver.Start()
	sibling.Start()
	AnimationTick()
	if len(animManager.animations) != 2 {
		t.Fatal(len(animManager.animations))
	}
	for _, a := range animManager.animations {
		if a == sibling {
			t.Fatal("stopped sibling resurrected")
		}
	}
	driver.Stop()
	replacement.Stop()
}

func TestBusyAnimationsDoNotRequestUnrelatedWindowRepaints(t *testing.T) {
	spinnerIsolateAnims(t)
	oldPerf := GlobalPerfStats
	GlobalPerfStats = &PerfStats{}
	defer func() { GlobalPerfStats = oldPerf }()
	host := newBusyAnimationTestHost(t)
	spinner := NewSpinner()
	spinner.SetParent(host)
	bar := NewProgressBar()
	bar.SetParent(host)
	bar.SetIndeterminate(true)
	defer spinner.SetBusy(false)
	defer bar.SetIndeterminate(false)
	if !HasActiveAnimations() || animationsRequireFullRepaint() {
		t.Fatal("busy widgets force whole-window repaint fallback")
	}
	if spinner.anim.onUpdate == nil || bar.anim.onUpdate == nil {
		t.Fatal("scoped animation cannot invalidate its pixels")
	}
	generic := NewAnimation(0, 1, time.Second)
	generic.Start()
	if !animationsRequireFullRepaint() {
		t.Fatal("generic geometry animation lost fallback")
	}
	generic.Stop()
	if animationsRequireFullRepaint() {
		t.Fatal("finished generic animation keeps full redraw alive")
	}
	GlobalPerfStats.SetVisible(true)
	if !animationsRequireFullRepaint() {
		t.Fatal("perf overlay no longer updates")
	}
}
