package gui

import (
	"math"
	"time"

	"github.com/uk0/silk/core"
	"github.com/uk0/silk/paint"
)

func init() {
	core.RegisterFactory("gui.Spinner", core.TypeOf((*Spinner)(nil)))
}

// Spinner is an animated busy indicator: eight dots arranged on a
// circle, with the "current" dot at full alpha and trailing dots
// fading toward transparent. The pattern rotates once per
// CycleDuration so the user perceives the widget as alive.
//
// Usage:
//
//	sp := gui.NewSpinner()
//	sp.SetSize(24, 24)
//	sp.SetParent(parent)
//	// later, when the long-running op finishes:
//	sp.SetBusy(false)
//
// Spinners draw nothing while !busy, so it's safe to leave one in
// the layout permanently and just toggle SetBusy(true/false) when an
// operation runs.
//
// Internally a single looping Animation drives redraws — its
// onUpdate is a no-op, but its presence makes
// gui.HasActiveAnimations() report true so the window event loop
// keeps ticking. That heartbeat exists only while the spinner could
// actually be seen (busy, attached, visible); see canDrive.
// The phase that Draw uses is computed straight from
// time.Since(startTime), so the visual is independent of the
// animation tick rate (it stays smooth even if AnimationTick lands
// on irregular intervals).
type Spinner struct {
	Widget

	color    paint.Color
	dotCount int
	cycle    time.Duration

	busy      bool
	startTime time.Time
	anim      *Animation
}

// NewSpinner creates a spinner with sensible defaults: theme accent
// colour, 8 dots, 1-second rotation cycle. The widget starts in the
// busy state so adding it to a layout immediately animates — call
// SetBusy(false) afterwards if you want to start hidden.
//
// No heartbeat is armed here. A spinner that has not been parented
// cannot be on screen, and arming at construction meant every spinner
// ever built — including ones dropped without being used — registered a
// looping Animation that nothing could ever evict. SetParent arms it.
func NewSpinner() *Spinner {
	s := new(Spinner)
	s.Init(s)
	return s
}

// Init carries the defaults, not NewSpinner: the designer and the .tdoc
// loader build widgets through the core factory, which reflects on Init and
// never sees the constructor. Draw bails below 3 dots, so a factory spinner
// left at zero could not render even once SetBusy(true) armed it.
//
// The busy flag and the phase origin belong here too — a factory spinner that
// starts !busy draws nothing at all. Arming the heartbeat does not: Init runs
// for every reflect-built widget, including throwaway instances the designer
// never shows, and each startAnim would leave a looping Animation behind. The
// heartbeat is started by NewSpinner and by SetBusy(true) instead.
func (this *Spinner) Init(self IWidget) {
	this.Widget.Init(self)
	this.color = Theme().HighLightColor
	this.dotCount = 8
	this.cycle = time.Second
	this.busy = true
	this.startTime = time.Now()
}

// SetColor overrides the dot colour. Defaults to the theme accent so
// most callers shouldn't need this — it exists for spinners on
// non-default surface colours (e.g. dark dialogs) where the theme
// accent provides poor contrast.
func (this *Spinner) SetColor(c paint.Color) {
	this.color = c
	this.Self().Update()
}

// Color returns the configured dot colour.
func (this *Spinner) Color() paint.Color {
	return this.color
}

// SetDotCount changes the number of dots on the circle. 8 is the
// default — fewer dots look chunkier, more dots look smoother. Below
// 3 the visual collapses to a single travelling dot.
func (this *Spinner) SetDotCount(n int) {
	if n < 3 {
		n = 3
	}
	if n == this.dotCount {
		return
	}
	this.dotCount = n
	this.Self().Update()
}

// DotCount returns the current dot count.
func (this *Spinner) DotCount() int {
	return this.dotCount
}

// SetCycleDuration sets how long one full rotation takes. Default
// 1 second; smaller values look more urgent, larger values calmer.
// Negative or zero falls back to 1 second so Draw never divides by
// zero when computing phase.
func (this *Spinner) SetCycleDuration(d time.Duration) {
	if d <= 0 {
		d = time.Second
	}
	this.cycle = d
}

// CycleDuration returns the rotation period.
func (this *Spinner) CycleDuration() time.Duration {
	return this.cycle
}

// IsBusy reports whether the spinner is currently animating.
func (this *Spinner) IsBusy() bool {
	return this.busy
}

// SetBusy toggles the animation. true starts the rotation and resets
// the phase so the spinner doesn't appear to jump when reactivated;
// false stops the underlying Animation so the window event loop can
// idle when nothing else needs ticking.
//
// A factory-built spinner starts busy but unarmed (Init sets the flag,
// not the heartbeat), so SetBusy(true) still has work to do there even
// though the flag already reads true.
func (this *Spinner) SetBusy(b bool) {
	if this.busy == b && (!b || this.anim != nil) {
		return
	}
	this.busy = b
	if b {
		this.startTime = time.Now()
	}
	this.syncAnim()
	this.Self().Update()
}

// SetParent arms or drops the heartbeat along with the attachment. A
// spinner only earns the 60fps tick once it is in a widget tree, and
// loses it the moment it is detached — so reparenting away, or dropping
// the last reference to a spinner that was never attached, leaves
// nothing registered with animManager.
func (this *Spinner) SetParent(parent IWidget) {
	this.Widget.SetParent(parent)
	this.syncAnim()
}

// Detach must route through Spinner.SetParent; Widget.Detach calls the
// embedded Widget.SetParent directly and would skip the heartbeat.
func (this *Spinner) Detach() {
	this.SetParent(nil)
}

// OnShow / OnHide are the widget tree's visibility seam (see
// Widget.setVisible). A hidden spinner paints nothing, so it must not
// keep the event loop at the animation tick either — hiding one used to
// cost more CPU than showing it, because the loop kept ticking and
// dirtying windows for a widget that drew nothing.
//
// SetVisible does not repaint on its own, and the heartbeat we just
// dropped is what used to dirty the window on the next frame, so the
// vacated rect is invalidated here — same as Menu.OnHide.
func (this *Spinner) OnShow() {
	this.syncAnim()
	this.Self().Update()
}

func (this *Spinner) OnHide() {
	this.syncAnim()
	this.Self().Update()
}

// canDrive reports whether this spinner is in a position to be seen:
// busy, attached to a tree, and not hidden itself. Only such a spinner
// may hold the loop at the animation tick.
//
// Deliberately NOT gated on ancestor visibility. Widget.setVisible
// dispatches OnShow/OnHide to the widget it was called on and never to
// descendants, so a spinner attached under a hidden ancestor would answer
// "no" here and never be re-asked -- StackedWidget.AddPage hides the page
// it just took, so the ordinary "add the page, then fill it" order left a
// tab's loading spinner painted but frozen forever. The hidden-spinner
// case that costs a core (68% measured) is spinner.Hide(), which
// IsVisible() plus the OnHide seam already catches. Gating on ancestors
// needs a real propagation seam in Widget.setVisible; until that exists,
// a leak while an ancestor is hidden beats a spinner that never turns.
func (this *Spinner) canDrive() bool {
	self := this.Self()
	return this.busy && self.Parent() != nil && self.IsVisible()
}

// syncAnim makes the heartbeat match canDrive(). Every seam that can
// change the answer — SetBusy, SetParent, OnShow, OnHide — funnels
// through here, so the arming rule lives in exactly one place.
func (this *Spinner) syncAnim() {
	if this.canDrive() {
		this.startAnim()
	} else {
		this.stopAnim()
	}
}

// startAnim spins up a heartbeat Animation if none is active. The
// animation does no actual interpolation work — its sole purpose is
// to register with animManager so HasActiveAnimations() returns true
// and the window event loop stays in active-redraw mode while the
// spinner is busy.
func (this *Spinner) startAnim() {
	if this.anim != nil && this.anim.State() == AnimRunning {
		return
	}
	this.anim = NewAnimation(0, 1, this.cycle).SetLoop(true).SetEase(EaseLinear)
	this.anim.Start()
}

// stopAnim ends the heartbeat so HasActiveAnimations can drop to
// false (assuming no other widget is animating). Safe to call
// multiple times — each call is idempotent.
func (this *Spinner) stopAnim() {
	if this.anim == nil {
		return
	}
	this.anim.Stop()
	this.anim = nil
}

// EnumProperties exposes the user-tunable properties for the
// designer's property sheet. Cycle duration is left out because
// the property sheet doesn't render time.Duration cleanly yet.
func (this *Spinner) EnumProperties(list core.IPropertyList) {
	list.AddProperty("点数", this.DotCount, this.SetDotCount)
	list.AddProperty("运行中", this.IsBusy, this.SetBusy)
}

// SizeHints reports a default 24×24 with growth disabled — spinners
// are usually small icons next to a label, not stretched across a
// container. Callers that want a bigger spinner SetSize after add.
func (this *Spinner) SizeHints() SizeHints {
	return SizeHints{Width: 24, Height: 24}
}

// Draw paints the eight-dot rotating pattern when busy; renders
// nothing when not busy so a stopped spinner leaves no visual
// residue. The phase is computed straight from elapsed time so the
// rotation rate stays constant regardless of how often
// AnimationTick fires.
func (this *Spinner) Draw(g paint.Painter) {
	if !this.busy || this.dotCount < 3 {
		return
	}
	w, h := this.Self().Size()
	cx := w * 0.5
	cy := h * 0.5
	r := math.Min(w, h)*0.5 - 1
	if r <= 0 {
		return
	}
	dotR := r * 0.16
	if dotR < 1 {
		dotR = 1
	}

	// Phase is fractional (0..1) over one cycle. Multiplying by 2π
	// rotates the leading dot once per cycle.
	cycle := this.cycle
	if cycle <= 0 {
		cycle = time.Second
	}
	phase := math.Mod(time.Since(this.startTime).Seconds()/cycle.Seconds(), 1.0)
	leadAngle := phase * 2 * math.Pi

	// Draw each dot at its angular position with alpha that fades as
	// you move backwards around the circle. Dot 0 is at the lead
	// position (full alpha); dot 1 trails 1/N of a circle behind, etc.
	// The fade is exponential so the trailing tail decays smoothly
	// rather than popping at a hard edge.
	n := this.dotCount
	for i := 0; i < n; i++ {
		ang := leadAngle - 2*math.Pi*float64(i)/float64(n)
		x := cx + (r-dotR)*math.Cos(ang)
		y := cy + (r-dotR)*math.Sin(ang)
		// Alpha: dot 0 = 1.0, dot 1 = (1-1/n), … dot N-1 ≈ 0.
		alpha := 1.0 - float64(i)/float64(n)
		c := this.color
		c.A = uint8(float64(c.A) * alpha)
		g.Arc(x, y, dotR, 0, 2*math.Pi)
		g.SetBrush1(c)
		g.Fill()
	}
}
