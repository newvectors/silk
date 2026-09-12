package gui

// IShortcutOverride is an opt-in, focused-widget boundary for self-contained
// document controls. Return false for every shortcut the widget does not own;
// the application's normal global handler then keeps its existing priority.
// Gantt previews use this so Undo cannot modify the IDE document behind them.
type IShortcutOverride interface {
	HandleShortcut(key int, mods uint8) bool
}

func (win *Window) dispatchShortcutOverride(key int) bool {
	target := win.keyTarget()
	if target == nil {
		return false
	}
	for w := target; w != nil; w = w.Parent() {
		if !w.IsEnabled() || !w.IsVisible() {
			return false
		}
	}
	if handler, ok := target.(IShortcutOverride); ok {
		return handler.HandleShortcut(key, currentMods())
	}
	return false
}
