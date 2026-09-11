package ged

import (
	"runtime"

	"github.com/uk0/silk/gui"
)

// Keep Ctrl compatibility on every platform, adding macOS Command without
// treating Windows/Linux Super as an editor modifier or consuming bare keys.
func designerActionModifier(goos string, down func(int) bool) bool {
	return down(gui.KeyCtrl) || (goos == "darwin" && (down(gui.KeyLWin) || down(gui.KeyRWin)))
}

func designerActionKeyDown() bool { return designerActionModifier(runtime.GOOS, keyDown) }
