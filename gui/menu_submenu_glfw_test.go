//go:build !windows

package gui

import "testing"

func TestEnterOpensSubmenuWithoutTriggeringHeaderAndTransfersFocus(t *testing.T) {
	isolatePopupState(t)
	owner := NewLabel("owner")
	_, ownerWin := testPopupWindow(owner, WtForm, nil)
	menu := NewPopupMenu()
	menu.SetParent(owner)
	_, menuWin := testPopupWindow(menu, WtPopup, ownerWin)
	menu.setVisible(true)
	sub, header := menu.AddSubMenu("submenu", nil, nil)
	sub.SetParent(header)
	testPopupWindow(sub, WtPopup, menuWin)
	sub.AddButton1("child", nil)
	headerCalls := 0
	header.Action().BindFunc0(func() { headerCalls++ })
	menu.setHighlight(0)
	menu.OnKeyDown(KeyEnter, false)
	if !sub.IsVisible() || focusWidget != sub || headerCalls != 0 {
		t.Fatalf("submenu visible=%v focus=%T header callbacks=%d", sub.IsVisible(), focusWidget, headerCalls)
	}
	sub.OnKeyDown(KeyDown, false)
	if sub.highlight != 0 {
		t.Fatal("new submenu did not accept keyboard navigation")
	}
	sub.OnKeyDown(KeyLeft, false)
	if sub.IsVisible() || focusWidget != menu {
		t.Fatal("Left did not close just the submenu and restore parent-menu focus")
	}
}

func TestDisabledSubmenuHoverCannotPrepareOrOpenPopup(t *testing.T) {
	isolatePopupState(t)
	menu := visibleActivationMenu()
	sub, header := menu.AddSubMenu("disabled submenu", nil, nil)
	prepared := 0
	header.SetSubPopupCallback(func(IButton) { prepared++ })
	header.SetEnabled(false)
	header.OnMouseStop(0, 0)
	header.ShowSubPopup()
	if prepared != 0 || sub.IsVisible() || sub.Window() != nil {
		t.Fatal("disabled header opened/prepared a native popup from hover")
	}
	header.SetSubPopup(nil) // Removing a popup must not dereference nil.
	if header.SubPopup() != nil {
		t.Fatal("submenu removal did not clear the popup")
	}
}

func TestCancelledSubmenuPreparationDoesNotTriggerHeaderCommand(t *testing.T) {
	isolatePopupState(t)
	menu := visibleActivationMenu()
	_, header := menu.AddSubMenu("dynamic", nil, nil)
	calls := 0
	header.Action().BindFunc0(func() { calls++ })
	header.SetSubPopupCallback(func(IButton) { header.SetSubPopup(nil) })
	header.emit()
	if calls != 0 || !menu.IsVisible() {
		t.Fatal("cancelled submenu preparation fell through to a leaf command")
	}
}

func TestUnavailableSubmenuDoesNotStealKeyboardFocus(t *testing.T) {
	isolatePopupState(t)
	oldReady := glfwReady
	glfwReady = false
	t.Cleanup(func() { glfwReady = oldReady })
	menu := visibleActivationMenu()
	sub, _ := menu.AddSubMenu("unavailable", nil, nil)
	sub.AddButton1("child", nil)
	menu.SetFocus()
	menu.setHighlight(0)
	menu.OnKeyDown(KeyRight, false)
	if sub.IsVisible() || sub.Window() != nil || focusWidget != menu {
		t.Fatal("failed native submenu creation moved focus into a hidden popup")
	}
}
