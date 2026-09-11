package main

import (
	"fmt"
	"github.com/uk0/silk/ged"
	"github.com/uk0/silk/gui"
	"github.com/uk0/silk/paint"
	"testing"
)

func TestDesignActionsSelectionAndModeGuards(t *testing.T) {
	w := testWorkbench(t)
	buildToolBar(w.frame, w.tabs, w.canvas)
	w.activate(modeDesign)
	if !w.designToolbar.IsVisible() || len(w.designActions) != 6 {
		t.Fatal("design row missing")
	}
	if w.designActions[0].button.IsEnabled() {
		t.Fatal("layout enabled for empty selection")
	}
	for i := 0; i < 2; i++ {
		widget, err := ged.NewFakeWidgetFromFactory("gui.Button")
		if err != nil {
			t.Fatal(err)
		}
		widget.SetBounds(float64(10+i*40), 10, 30, 10)
		widget.SetParent(w.canvas.GedScene())
		w.canvas.Selection().Add(widget)
	}
	w.syncMode() // Same mode: selection state must still update.
	grid := w.designActions[2]
	if !grid.button.IsEnabled() || gui.GetToolTip(grid.button) == "" {
		t.Fatal("grid action not available or undocumented")
	}
	grid.button.Action().Trigger(nil)
	box, ok := w.canvas.GedScene().Children()[0].(*ged.FakeWidget)
	if !ok || box.WidgetFactoryName() != string(ged.LayoutGrid) {
		t.Fatal("toolbar did not apply real GridLayout")
	}
	w.syncMode()
	if !w.designActions[4].button.IsEnabled() {
		t.Fatal("break action unavailable after layout")
	}
	w.activate(modeEdit)
	if w.designToolbar.IsVisible() {
		t.Fatal("design toolbar remains over editor")
	}
	w.designActions[4].spec.run() // Palette uses this same guarded handler.
	if len(w.canvas.GedScene().Children()) != 1 {
		t.Fatal("hidden canvas mutated")
	}
	w.activate(modeDesign)
	w.designActions[4].button.Action().Trigger(nil)
	if len(w.canvas.GedScene().Children()) != 2 {
		t.Fatal("break layout lost children")
	}
}

func TestCommandPaletteNamesAreUniqueAndRegistrationIdempotent(t *testing.T) {
	w := testWorkbench(t)
	old := paletteCommands
	t.Cleanup(func() { paletteCommands = old })
	registerPaletteCommands(w.tabs, w.canvas)
	n := len(paletteCommands)
	registerPaletteCommands(w.tabs, w.canvas)
	if len(paletteCommands) != n {
		t.Fatal("registration appends stale commands")
	}
	counts := map[string]int{}
	for _, cmd := range paletteCommands {
		counts[cmd.Name]++
	}
	for name, count := range counts {
		if count != 1 {
			t.Errorf("duplicate command %q (%d)", name, count)
		}
	}
	for _, spec := range designerActionSpecs(w.canvas) {
		if counts[spec.label] != 1 {
			t.Errorf("missing layout command %q", spec.label)
		}
	}
}

func TestDesignActionDescriptorsHaveNamedIconsAndHints(t *testing.T) {
	for _, spec := range designerActionSpecs(nil) {
		if spec.label == "" || spec.hint == "" {
			t.Fatal("anonymous designer action")
		}
		if icon := paint.LoadIcon(spec.icon); icon == nil || icon.IsAir() || fmt.Sprint(icon) != spec.icon {
			t.Errorf("action %q resolves missing icon %q", spec.label, spec.icon)
		}
	}
}
