package ged

import (
	"runtime"
	"testing"

	"github.com/uk0/silk/gui"
)

func TestDesignerActionModifierMatchesPlatform(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		for _, held := range []int{0, gui.KeyCtrl, gui.KeyLWin, gui.KeyRWin, gui.KeyShift, gui.KeyMenu} {
			got := designerActionModifier(goos, func(key int) bool { return held != 0 && key == held })
			want := held == gui.KeyCtrl || (goos == "darwin" && (held == gui.KeyLWin || held == gui.KeyRWin))
			if got != want {
				t.Fatalf("%s key %x action modifier = %v, want %v", goos, held, got, want)
			}
		}
	}
}

func TestDesignerCommandSelectCopyPasteDuplicateAndHistory(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("actual Command dispatch is macOS-specific; the platform predicate is tested everywhere")
	}
	for _, commandKey := range []int{gui.KeyLWin, gui.KeyRWin} {
		t.Run(map[int]string{gui.KeyLWin: "left command", gui.KeyRWin: "right command"}[commandKey], func(t *testing.T) {
			view := NewGedView()
			one := addFakeAt(t, view.GedScene(), "one", 10, 10, 25, 7)
			two := addFakeAt(t, view.GedScene(), "two", 10, 30, 25, 7)
			previousClipboard := clipboard
			t.Cleanup(func() { clipboard = previousClipboard })
			holdKeys(t, commandKey)
			view.OnKeyDown('A', false)
			if view.Selection().Count() != 2 || !view.Selection().Contains(one) || !view.Selection().Contains(two) || view.Selection().Contains(view.Scene()) {
				t.Fatal("Command+A did not select exactly the design widgets")
			}
			view.OnKeyDown('C', false)
			if len(clipboard) != 2 {
				t.Fatal("Command+C did not copy selection")
			}
			view.OnKeyDown('V', false)
			if len(view.Scene().Children()) != 4 {
				t.Fatal("Command+V did not paste")
			}
			view.OnKeyDown('Z', false)
			if len(view.Scene().Children()) != 2 {
				t.Fatal("Command+Z did not undo paste")
			}
			keyDown = func(key int) bool { return key == commandKey || key == gui.KeyShift }
			view.OnKeyDown('Z', false)
			if len(view.Scene().Children()) != 4 {
				t.Fatal("Command+Shift+Z did not redo paste")
			}
			keyDown = func(key int) bool { return key == commandKey }
			view.Selection().Clear()
			view.Selection().Add(one)
			view.OnKeyDown('D', false)
			if len(view.Scene().Children()) != 5 {
				t.Fatal("Command+D did not duplicate")
			}
		})
	}
}

func TestDesignerBareActionLettersDoNotEditTheDesign(t *testing.T) {
	view := NewGedView()
	one := addFakeAt(t, view.GedScene(), "one", 10, 10, 25, 7)
	addFakeAt(t, view.GedScene(), "two", 10, 30, 25, 7)
	view.Selection().Add(one)
	previousClipboard := clipboard
	clipboard = nil
	t.Cleanup(func() { clipboard = previousClipboard })
	holdKeys(t)
	for _, key := range []int{'A', 'C', 'V', 'D', 'X', 'Z', 'Y'} {
		view.OnKeyDown(key, false)
	}
	if view.Selection().Count() != 1 || !view.Selection().Contains(one) || len(view.Scene().Children()) != 2 || len(clipboard) != 0 || view.Scene().UndoStack().CanUndo() {
		t.Fatal("plain letter activated a designer edit command")
	}
}
