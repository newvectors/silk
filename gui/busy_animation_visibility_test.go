package gui

import "testing"

// A logical owner window exercises the real widget-tree notification paths
// without creating an NSWindow/GLFW surface from testing's worker goroutine.
// Native presentation/occlusion remains the event-loop integration's job.
type busyAnimationTestHost struct {
	Widget
	logical *Window
}

func (h *busyAnimationTestHost) Window() *Window { return h.logical }
func newBusyAnimationTestHost(t *testing.T) *busyAnimationTestHost {
	t.Helper()
	h := &busyAnimationTestHost{}
	h.Init(h)
	h.logical = &Window{widget: h}
	t.Cleanup(func() { h.setVisible(false); h.logical = nil; notifyBusyAnimationVisibility(h) })
	return h
}

func TestBusyAnimationsRequireVisibleWindowTreeAndFollowAncestorLifecycle(t *testing.T) {
	for _, kind := range []string{"spinner", "progress"} {
		t.Run(kind, func(t *testing.T) {
			spinnerIsolateAnims(t)
			host := newBusyAnimationTestHost(t)
			outer := NewVBox()
			page := NewVBox()
			page.SetParent(outer)
			page.Hide()
			var w IWidget
			if kind == "spinner" {
				w = NewSpinner()
			} else {
				p := NewProgressBar()
				p.SetIndeterminate(true)
				w = p
			}
			w.SetParent(page)
			if HasActiveAnimations() {
				t.Fatal("unmounted subtree drives animations")
			}
			outer.SetParent(host)
			if HasActiveAnimations() {
				t.Fatal("hidden ancestor drives animations")
			}
			page.Show()
			if animManager.activeCount() != 1 {
				t.Fatal("ancestor Show did not rearm visible busy child")
			}
			first := animManager.animations[0]
			notifyBusyAnimationVisibility(host)
			if len(animManager.animations) != 1 || animManager.animations[0] != first {
				t.Fatal("unchanged visibility recreated heartbeat")
			}
			outer.Hide()
			if HasActiveAnimations() || len(animManager.animations) != 0 {
				t.Fatal("ancestor Hide retained heartbeat/closure")
			}
			outer.Show()
			if animManager.activeCount() != 1 {
				t.Fatal("ancestor Show failed after pause")
			}
			outer.Detach() // child still has two direct ancestors, but no window
			if HasActiveAnimations() || len(animManager.animations) != 0 {
				t.Fatal("detached whole subtree retained animation")
			}
			outer.SetParent(host)
			if animManager.activeCount() != 1 {
				t.Fatal("reattached subtree failed to animate")
			}
			host.Hide()
			if HasActiveAnimations() {
				t.Fatal("hidden owner drives animation")
			}
			host.Show()
			if animManager.activeCount() != 1 {
				t.Fatal("shown owner did not restart animation")
			}
			w.Hide()
			if HasActiveAnimations() {
				t.Fatal("hidden child retained animation")
			}
			w.Show()
			if animManager.activeCount() != 1 {
				t.Fatal("shown child did not restart animation")
			}
			w.Detach()
			if HasActiveAnimations() {
				t.Fatal("detached child retained animation")
			}
		})
	}
}

func TestBusyProgressDrawDoesNotArmUnattachedOrHiddenHeartbeat(t *testing.T) {
	spinnerIsolateAnims(t)
	p := NewProgressBar()
	p.SetIndeterminate(true)
	p.SetSize(120, 20)
	p.Draw(newPBRecorder())
	if p.anim != nil || HasActiveAnimations() {
		t.Fatal("unattached busy Draw armed timer")
	}
	host := newBusyAnimationTestHost(t)
	host.Hide()
	p.SetParent(host)
	p.Draw(newPBRecorder())
	if p.anim != nil || HasActiveAnimations() {
		t.Fatal("hidden busy Draw armed timer")
	}
	host.Show()
	if p.anim == nil {
		t.Fatal("visible attached progress failed to arm")
	}
	p.SetIndeterminate(false)
	if p.anim != nil || HasActiveAnimations() {
		t.Fatal("determinate state retained animation")
	}
}

// Notifications must not masquerade as ordinary OnShow/OnHide events: many
// existing widgets use those to start modal windows or their own controllers.
type busyVisibilityLocalEvents struct {
	Widget
	shows, hides int
}

func (w *busyVisibilityLocalEvents) OnShow() { w.shows++ }
func (w *busyVisibilityLocalEvents) OnHide() { w.hides++ }
func TestBusyVisibilityPropagationDoesNotBroadcastLocalShowHide(t *testing.T) {
	host := newBusyAnimationTestHost(t)
	child := &busyVisibilityLocalEvents{}
	child.Init(child)
	child.SetParent(host)
	host.Hide()
	host.Show()
	if child.shows != 0 || child.hides != 0 {
		t.Fatal("ancestor change broadcast ordinary child visibility callbacks")
	}
}
