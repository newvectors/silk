package gui

import "github.com/uk0/silk/paint"

// Called by both native renderers after normal children and widget overlays.
// Keeping toasts in this pass also guarantees that newly inserted toolbars or
// docks cannot paint on top of a notification.
func drawToastOverlay(owner IWidget, g paint.Painter) {
	mgr := owner.NakedWidget().toastManager
	if mgr == nil {
		return
	}
	cx, cy, cw, ch := g.ClipBounds()
	for _, entry := range mgr.toasts {
		if entry.parent != owner || !entry.IsVisible() {
			continue
		}
		x, y, w, h := entry.Bounds()
		cx1, cy1, cw1, ch1 := intersectRect(x, y, w, h, cx, cy, cw, ch)
		DrawWidgetAll(entry, g, x, y, cx1, cy1, cw1, ch1)
	}
}

func (mgr *ToastManager) hitTest(owner IWidget, x, y float64) IWidget {
	for i := len(mgr.toasts) - 1; i >= 0; i-- {
		entry := mgr.toasts[i]
		if entry.parent != owner || !entry.IsVisible() {
			continue
		}
		if hit := entry.FindWidgetAt(x-entry.X(), y-entry.Y()); hit != nil {
			return hit
		}
	}
	return nil
}

// Hiding/destroying an owner must stop callbacks and release the owner, not
// merely wait for a visually hidden toast's timer to expire.
func dismissOwnedToasts(owner IWidget) {
	if owner == nil {
		return
	}
	mgr := owner.NakedWidget().toastManager
	if mgr == nil {
		return
	}
	for i := len(mgr.toasts) - 1; i >= 0; i-- {
		if entry := mgr.toasts[i]; entry.parent == owner {
			mgr.dismiss(entry)
		}
	}
}
