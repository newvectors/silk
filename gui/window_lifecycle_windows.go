package gui

import "github.com/uk0/silk/win32"

// GetKeyState returns a SHORT: bit 15 means down, bit 0 means toggled.
// Bit 8 is not a key-state flag (unlike the old mask used by this backend).
func decodeWin32KeyState(state int16) (down, checked bool) {
	return uint16(state)&0x8000 != 0, uint16(state)&0x0001 != 0
}

func (win *Window) prepareWindowDestruction() {
	win.dismissOwnedPopups()
	dismissOwnedToasts(win.widget)
	forgetWidgetsOfWindow(win)
	if win.widget != nil && win.widget.NakedWidget().win == win {
		win.widget.NakedWidget().setVisible(false)
		win.widget.NakedWidget().win = nil
	}
}

func (win *Window) refreshPopupOwner() {
	if win.wt != WtPopup || win.widget == nil {
		return
	}
	win.popupOwner = nil
	if parent := win.widget.Parent(); parent != nil {
		if owner := parent.OwnerWindow(); owner != win {
			win.popupOwner = owner
		}
	}
	if win.hWnd != 0 {
		var ownerHandle win32.HWND
		if win.popupOwner != nil {
			ownerHandle = win.popupOwner.hWnd
		}
		// A reusable popup can move between forms without recreating its HWND.
		win32.SetWindowLongPtr(win.hWnd, win32.GWLP_HWNDPARENT, uintptr(ownerHandle))
	}
}

func popupOwnedBy(popup, owner *Window) bool {
	for p := popup; p != nil && p.wt == WtPopup; p = p.popupOwner {
		if p.popupOwner == owner {
			return true
		}
		if p.popupOwner == p {
			return false
		}
	}
	return false
}

func widgetWithinPopups(widget IWidget, popups map[IWidget]bool) bool {
	for p := widget; p != nil; p = p.Parent() {
		if popups[p] {
			return true
		}
	}
	return false
}

// Windows automatically hides some owned HWNDs, but does not synchronize
// their widget visibility/capture state. Explicit Hide keeps reopening and
// keyboard routing consistent and also handles move/resize/deactivation.
func (win *Window) dismissOwnedPopups() {
	popups := make(map[IWidget]bool)
	for _, candidate := range winMap {
		if candidate != win && candidate.widget != nil && popupOwnedBy(candidate, win) {
			popups[candidate.widget] = true
		}
	}
	if len(popups) == 0 {
		return
	}
	hadFocus := widgetWithinPopups(focusWidget, popups)
	var restore IWidget
	if hadFocus {
		for p := focusWidget.OwnerWindow(); p != nil && popupOwnedBy(p, win); p = p.popupOwner {
			restore = p.popupRestoreFocus
		}
	}
	if currentTooltip != nil && popups[currentTooltip] {
		HideToolTip()
	}
	for popup := range popups {
		if popup.IsVisible() {
			popup.Hide()
		}
	}
	clearPopupInput(popups, restore, hadFocus)
}

func clearPopupInput(popups map[IWidget]bool, restore IWidget, hadFocus bool) {
	previousCapture := curCapture()
	kept := captureStack[:0]
	for _, captured := range captureStack {
		if !widgetWithinPopups(captured, popups) {
			kept = append(kept, captured)
		}
	}
	clear(captureStack[len(kept):])
	captureStack = kept
	if captured := curCapture(); captured != previousCapture {
		releaseCapture()
		if captured != nil {
			setCapture(captured)
		}
	}
	if widgetWithinPopups(lastMouseWidget, popups) {
		lastMouseWidget = nil
	}
	if widgetWithinPopups(mouseHoverWidget, popups) {
		setMouseHoverWidget(nil)
	}
	if hadFocus && (focusWidget == nil || widgetWithinPopups(focusWidget, popups)) {
		focusWidget = nil
		if restore != nil && !widgetWithinPopups(restore, popups) && restore.IsAllAncentorsVisible() {
			restore.SetFocus()
		}
	}
}

func (win *Window) queuePopupFocusCheck() {
	owner := win
	for owner.wt == WtPopup && owner.popupOwner != nil && owner.popupOwner != owner {
		owner = owner.popupOwner
	}
	if owner.popupFocusCheckPending {
		return
	}
	owner.popupFocusCheckPending = true
	Post(func() {
		owner.popupFocusCheckPending = false
		if owner.widget == nil {
			return
		}
		active := winMap[win32.GetActiveWindow()]
		if active != owner && !popupOwnedBy(active, owner) {
			owner.dismissOwnedPopups()
		}
	})
}

// Preserve the same API as GLFW. Win32's normal invalidation already clips
// the paint region; a conservative full redraw is correct for overlays too.
func (win *Window) MarkDirtyRect(x, y, width, height float64) {
	if width > 0 && height > 0 {
		win.MarkFullDirty()
	}
}

func (win *Window) MarkFullDirty() {
	if win.hWnd != 0 {
		win32.InvalidateRect(win.hWnd, nil, false)
	}
}
