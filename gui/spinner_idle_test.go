package gui

import (
	"math"
	"testing"
	"time"

	"github.com/uk0/silk/paint"
)

// These tests cover the cost side of the animation engine: what keeps
// MainLoop off the idle wait. HasActiveAnimations() is the production
// predicate the loop polls — while it holds, the loop runs at 1/60 and
// marks every visible window dirty every frame — so the assertions go
// through it and through the manager's own activeCount(), never through
// a private re-count of animManager.animations.

// spinnerIsolateAnims empties the process-wide animation manager for the
// duration of one test and puts it back afterwards, so a heartbeat left
// registered by another test cannot mask (or fake) these results. Same
// save/restore shape as TestHasActiveAnimationsEmpty.
func spinnerIsolateAnims(t *testing.T) {
	t.Helper()
	origAnims := animManager.animations
	origGroups := animManager.groups
	animManager.animations = nil
	animManager.groups = nil
	t.Cleanup(func() {
		animManager.animations = origAnims
		animManager.groups = origGroups
	})
}

// TestSpinnerConstructedAndDroppedDoesNotDriveLoop: a spinner that was
// never attached cannot be on screen, so it must register nothing.
// Arming at construction meant every spinner ever built — including the
// ones a dialog constructor makes and throws away — left a looping
// Animation behind that no code path could evict, because the only
// eviction was SetBusy(false) and nobody was left holding the pointer.
func TestSpinnerConstructedAndDroppedDoesNotDriveLoop(t *testing.T) {
	spinnerIsolateAnims(t)

	for i := 0; i < 100; i++ {
		_ = NewSpinner() // built, never parented, reference dropped
	}

	if HasActiveAnimations() {
		t.Fatalf("HasActiveAnimations() = true after building and dropping 100 spinners "+
			"(activeCount() = %d): an unattached spinner is on no screen, yet each one "+
			"pins MainLoop at the 1/60 tick and dirties every visible window",
			animManager.activeCount())
	}
}

// TestSpinnerHiddenDoesNotDriveLoop: Hide() must drop the heartbeat, and
// Show() must bring it back. A hidden spinner draws nothing (Draw bails
// on !busy is not the point — the widget is not painted at all), so the
// measured 63 wakes/s it used to cost bought exactly zero pixels.
func TestSpinnerHiddenDoesNotDriveLoop(t *testing.T) {
	spinnerIsolateAnims(t)

	s := NewSpinner()
	s.SetParent(NewForm())
	if !HasActiveAnimations() {
		t.Fatalf("attached, visible, busy spinner registered no animation: "+
			"activeCount() = %d, want 1", animManager.activeCount())
	}

	s.Hide()
	if HasActiveAnimations() {
		t.Fatalf("HasActiveAnimations() = true after Hide() (activeCount() = %d): "+
			"a hidden spinner paints nothing but still holds MainLoop at the 1/60 tick",
			animManager.activeCount())
	}

	s.Show()
	if !HasActiveAnimations() {
		t.Fatalf("Show() did not re-arm the heartbeat: activeCount() = %d, want 1 — "+
			"a re-shown spinner would freeze because the loop stays on the idle wait",
			animManager.activeCount())
	}
}

// TestSpinnerDetachedDoesNotDriveLoop: reparenting a spinner out of the
// tree (SetParent(nil), or the Detach() alias) must drop the heartbeat.
// This is the "its dialog went away" path — the spinner is unreachable
// from any window, so nothing it could redraw is on screen.
func TestSpinnerDetachedDoesNotDriveLoop(t *testing.T) {
	for _, tc := range []struct {
		name   string
		detach func(*Spinner)
	}{
		{"SetParent(nil)", func(s *Spinner) { s.SetParent(nil) }},
		{"Detach()", func(s *Spinner) { s.Detach() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spinnerIsolateAnims(t)

			s := NewSpinner()
			s.SetParent(NewForm())
			if n := animManager.activeCount(); n != 1 {
				t.Fatalf("attached busy spinner: activeCount() = %d, want 1", n)
			}

			tc.detach(s)
			if HasActiveAnimations() {
				t.Fatalf("HasActiveAnimations() = true after %s (activeCount() = %d): "+
					"a detached spinner is in no window's tree and can never repaint, "+
					"but it still drives the loop at 1/60", tc.name, animManager.activeCount())
			}
		})
	}
}

// TestSpinnerVisibleKeepsAnimating is the other half of the property: a
// fix that just stopped every spinner would satisfy every assertion
// above. A busy spinner that is attached and visible must hold the loop
// awake, must still be holding it after the loop has ticked many times
// (a looping heartbeat is never "done", so it must never be evicted),
// and must still paint dots that move as the clock advances.
func TestSpinnerVisibleKeepsAnimating(t *testing.T) {
	spinnerIsolateAnims(t)

	s := NewSpinner()
	s.SetParent(NewForm())
	s.SetSize(24, 24)

	if n := animManager.activeCount(); n != 1 {
		t.Fatalf("attached, visible, busy spinner: activeCount() = %d, want 1", n)
	}

	for i := 0; i < 500; i++ {
		AnimationTick()
	}
	if !HasActiveAnimations() {
		t.Fatalf("HasActiveAnimations() = false after 500 ticks (activeCount() = %d): "+
			"the heartbeat that keeps a visible spinner repainting was dropped, so the "+
			"spinner freezes on screen", animManager.activeCount())
	}
	if s.anim == nil || s.anim.State() != AnimRunning {
		t.Fatalf("visible spinner's own heartbeat is %v after 500 ticks, want AnimRunning",
			animState(s))
	}

	// The visual has to move too, not just the registration. Draw takes
	// its phase from time.Since(startTime), so rewinding startTime by a
	// quarter cycle is the same as letting a quarter cycle elapse.
	before := drawSpinnerDots(s)
	s.startTime = s.startTime.Add(-s.CycleDuration() / 4)
	after := drawSpinnerDots(s)

	if len(before) != 8 || len(after) != 8 {
		t.Fatalf("busy spinner painted %d and %d dots, want 8 each", len(before), len(after))
	}
	if math.Hypot(after[0].x-before[0].x, after[0].y-before[0].y) < 1 {
		t.Fatalf("lead dot did not move over a quarter cycle: %v -> %v — the spinner is "+
			"registered but frozen", before[0], after[0])
	}
}

// TestPausedAnimationIsNotActive: Pause() suspends progress, so a paused
// animation advances nothing and must not keep the loop at the animation
// tick. It must stay registered and resumable, though — evicting it
// would be a different bug, so Resume() is checked as well.
func TestPausedAnimationIsNotActive(t *testing.T) {
	spinnerIsolateAnims(t)

	a := NewAnimation(0, 1, time.Second).SetLoop(true).SetEase(EaseLinear)
	a.Start()
	if !HasActiveAnimations() {
		t.Fatalf("a started animation must be active: activeCount() = %d, want 1",
			animManager.activeCount())
	}

	a.Pause()
	for i := 0; i < 1000; i++ {
		AnimationTick()
	}
	if HasActiveAnimations() {
		t.Fatalf("HasActiveAnimations() = true after Pause() + 1000 ticks "+
			"(activeCount() = %d): a paused animation moves nothing, so it must not hold "+
			"MainLoop at the 1/60 tick", animManager.activeCount())
	}
	if a.State() != AnimPaused {
		t.Fatalf("after Pause() + 1000 ticks State() = %v, want AnimPaused — a paused "+
			"animation must stay registered and resumable, not be thrown away", a.State())
	}

	a.Resume()
	if !HasActiveAnimations() {
		t.Fatalf("Resume() did not make the animation active again: activeCount() = %d, "+
			"want 1", animManager.activeCount())
	}
	a.Stop()
}

// TestStopEvictsAnimationByIdentity: Stop() must unregister the
// animation on the spot. The manager only evicts on Tick, and the loop
// only Ticks while something is running, so an entry that waits for a
// Tick to be swept can wait forever — holding its onUpdate closure and
// whatever widget that closure captured.
func TestStopEvictsAnimationByIdentity(t *testing.T) {
	spinnerIsolateAnims(t)

	keep := NewAnimation(0, 1, time.Second).SetLoop(true)
	drop := NewAnimation(0, 1, time.Second).SetLoop(true)
	keep.Start()
	drop.Start()

	drop.Stop() // no AnimationTick(): Stop must stand on its own

	if len(animManager.animations) != 1 || animManager.animations[0] != keep {
		t.Fatalf("after Stop() the manager holds %d entries, want only the running one — "+
			"a stopped animation must be evicted by pointer identity, not left for a Tick "+
			"that may never come", len(animManager.animations))
	}
	keep.Stop()
}

// drawSpinnerDots runs Draw and returns the centre of every dot it
// painted, in paint order (dot 0 is the lead dot).
func drawSpinnerDots(s *Spinner) []spinnerDot {
	rec := &spinnerDotRecorder{Painter: nopPainter{}}
	s.Draw(rec)
	return rec.dots
}

type spinnerDot struct{ x, y float64 }

type spinnerDotRecorder struct {
	paint.Painter
	dots []spinnerDot
}

func (r *spinnerDotRecorder) Arc(xc, yc, radius, angle1, angle2 float64) {
	r.dots = append(r.dots, spinnerDot{xc, yc})
}
func (r *spinnerDotRecorder) SetBrush1(c paint.Color) {}
func (r *spinnerDotRecorder) Fill()                   {}

// TestSpinnerUnderAHiddenAncestorStillAnimates is the guard for the review
// that rejected the first version of this fix. canDrive() originally also
// required IsAllAncentorsVisible(). Nothing observes that input: setVisible
// dispatches OnShow/OnHide to the widget it was called on and never to its
// descendants, so a spinner that answered "no" because of an ancestor was
// never re-asked — and Draw only checks busy, so the user saw a busy
// indicator that never moved.
//
// The shape is not contrived. StackedWidget.AddPage calls SetParent then
// Hide on the page it was handed, and TabWidget.AddTab routes into it, so
// the ordinary "add the page, then fill it" order attaches the spinner under
// a hidden ancestor. Selecting that tab shows the page and the spinner is
// then fully visible, busy, and — with the clause in — driving nothing.
func TestSpinnerUnderAHiddenAncestorStillAnimates(t *testing.T) {
	spinnerIsolateAnims(t)

	host := NewForm()
	page := NewVBox()
	page.SetParent(host)
	page.Hide() // what StackedWidget.AddPage does to the page it takes

	sp := NewSpinner()
	sp.SetParent(page) // attached while the ancestor is hidden
	sp.SetBounds(0, 0, 28, 28)

	page.Show() // selecting the tab

	if !sp.IsVisible() {
		t.Fatal("the spinner hid itself; the fixture is wrong, not the code")
	}
	if !HasActiveAnimations() {
		t.Error("a busy, visible spinner on a shown page drives nothing: it paints but never turns")
	}
}
