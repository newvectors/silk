package gui

import (
	"testing"

	"github.com/uk0/silk/paint"
)

func toastTestOwner(t *testing.T, width, height float64) *Form {
	t.Helper()
	owner := NewForm()
	owner.SetSize(width, height)
	t.Cleanup(func() { dismissOwnedToasts(owner) })
	return owner
}

func TestToastIsOwnerLocalAndNeverAWindowOrLayoutChild(t *testing.T) {
	owner := NewHBox()
	owner.SetSize(640, 400)
	child := NewButton()
	owner.AddWidget(child)
	before := child.Bounds1()
	mgr := new(ToastManager)
	t.Cleanup(func() { dismissOwnedToasts(owner) })
	mgr.show(child, "已保存 floating.silkui", 2500, ToastSuccess)
	entry := mgr.toasts[0]
	if entry.Parent() != owner || entry.Window() != nil || entry.IsPopup() {
		t.Fatal("toast must belong to its owner without a native popup")
	}
	if children := owner.Children(); len(children) != 1 || children[0] != child {
		t.Fatal("toast leaked into document/layout children")
	}
	owner.Layout()
	if child.Bounds1() != before {
		t.Fatal("toast changed an automatic layout")
	}
	if entry.X() != 160 || entry.Y() != 12 {
		t.Fatalf("toast is not in owner-local coordinates: %v", entry.Bounds1())
	}
	owner.SetPos(250, 350)
	if entry.X() != 160 || entry.Y() != 12 {
		t.Fatal("owner movement must not introduce global coordinate offsets")
	}
	if entry.FocusPolicy() != NoFocus {
		t.Fatal("notifications must not steal editor keyboard focus")
	}
}

func TestToastGroupsResizeAndDismissAreOwnerIsolated(t *testing.T) {
	one, two := toastTestOwner(t, 640, 400), toastTestOwner(t, 900, 600)
	mgr := new(ToastManager)
	mgr.show(one, "one", 2500, ToastSuccess)
	first := mgr.toasts[0]
	mgr.show(two, "two", 2500, ToastInfo)
	second := mgr.toasts[1]
	secondBounds := second.Bounds1()
	mgr.show(one, "newest", 2500, ToastWarning)
	newest := mgr.toasts[2]
	if newest.Y() != 12 || first.Y() != 68 || second.Bounds1() != secondBounds {
		t.Fatal("stacking in one window moved another window's notification")
	}
	one.SetSize(200, 100)
	if !newest.IsVisible() || first.IsVisible() {
		t.Fatal("small owner must keep the newest result, not overflow old toasts")
	}
	if newest.X() < 0 || newest.X()+newest.Width() > one.Width() || newest.Y()+newest.Height() > one.Height() {
		t.Fatal("toast escaped the owner's content rectangle")
	}
	if second.Bounds1() != secondBounds {
		t.Fatal("resizing one window moved another window's notification")
	}
	one.Hide()
	if one.toastManager != nil || len(mgr.toasts) != 1 || mgr.toasts[0] != second {
		t.Fatal("owner hide failed to dismiss exactly its own notifications")
	}
	for _, entry := range []*toastEntry{first, newest} {
		if entry.timer != 0 || entry.Parent() != nil || entry.manager != nil || entry.IsVisible() {
			t.Fatal("dismissed toast retained timer, owner or visibility")
		}
		mgr.dismiss(entry) // idempotent; must not affect the other owner
	}
	if !second.IsVisible() {
		t.Fatal("dismissing another owner hid this toast")
	}
}

func TestToastCloseHitTestWinsWithoutConsumingOwnerContent(t *testing.T) {
	owner := toastTestOwner(t, 640, 400)
	underlay := NewButton()
	underlay.SetParent(owner)
	underlay.SetSize(owner.Size())
	mgr := new(ToastManager)
	mgr.show(owner, "已保存", 2500, ToastSuccess)
	entry := mgr.toasts[0]
	x, y := entry.X()+toastCloseX(entry.Width()), entry.Y()+24
	if got := owner.FindWidgetAt(x, y); got != entry {
		t.Fatalf("close hit landed on %T rather than toast overlay", got)
	}
	if got := owner.FindWidgetAt(10, 200); got != underlay {
		t.Fatal("overlay blocked input outside its rectangle")
	}
	entry.OnLeftDown(60, 24)
	entry.OnLeftDown(-1, 24)
	if len(mgr.toasts) != 1 {
		t.Fatal("body/outside click closed the notification")
	}
	entry.OnLeftDown(toastCloseX(entry.Width()), 24)
	if len(mgr.toasts) != 0 || owner.FindWidgetAt(x, y) != underlay {
		t.Fatal("close did not dismiss the toast and return input to owner")
	}
}

func TestToastSmallOwnersAndHiddenOwners(t *testing.T) {
	owner := toastTestOwner(t, 20, 20)
	mgr := new(ToastManager)
	mgr.show(nil, "nil", 2500, ToastInfo)
	owner.Hide()
	mgr.show(owner, "hidden", 2500, ToastInfo)
	if len(mgr.toasts) != 0 {
		t.Fatal("nil/hidden owner should not produce a toast")
	}
	owner.Show()
	mgr.show(owner, "small", 2500, ToastInfo)
	entry := mgr.toasts[0]
	if entry.IsVisible() {
		t.Fatal("too-small owner must not draw overlapping/clipped toast controls")
	}
	owner.SetSize(48, 100)
	if !entry.IsVisible() {
		t.Fatal("toast should reappear when enough space becomes available")
	}
	entry.OnLeftDown(toastCloseX(entry.Width()), 24)
	if len(mgr.toasts) != 0 {
		t.Fatal("narrow toast close control is not usable")
	}
}

func TestToastActuallyPaintsAboveContentWithExactThemeColors(t *testing.T) {
	restoreToastTestTheme(t)
	SetThemeMode(ThemeDark)
	owner := toastTestOwner(t, 640, 240)
	mgr := new(ToastManager)
	mgr.show(owner, "已保存 floating.silkui", 2500, ToastSuccess)
	entry := mgr.toasts[0]
	// Add content after the toast: overlay ordering must not depend on child
	// insertion order, otherwise opening a dock covers existing notifications.
	underlay := NewForm()
	underlay.SetParent(owner)
	underlay.SetSize(owner.Size())
	pix := paint.NewPixmap(640, 240)
	g := pix.NewPainter()
	DrawWidgetAll(owner, g, 0, 0, 0, 0, 640, 240)
	img, err := pix.Image()
	if err != nil {
		t.Fatal(err)
	}
	bg, _ := toastColors(ToastSuccess)
	for _, point := range []struct {
		x, y int
		want paint.Color
	}{
		{int(entry.X()) + 80, int(entry.Y()) + 8, bg},
		{int(entry.X()) + 2, int(entry.Y()) + 24, toastAccentColor(ToastSuccess)},
	} {
		got := paint.ColorModel.Convert(img.At(point.x, point.y)).(paint.Color)
		if got != point.want {
			t.Fatalf("rendered overlay pixel (%d,%d) = %v, want %v", point.x, point.y, got, point.want)
		}
	}
}
