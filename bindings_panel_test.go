package main

import (
	"strings"
	"testing"

	"github.com/uk0/silk/ged"
)

// TestBindingsPanelFollowsTheActiveDocument covers the re-aim: one 信号/槽 panel
// is shared by every open design, so switching tabs has to point it at the new
// document. Left bound to the first it lists the bindings of a canvas the user
// is no longer looking at.
func TestBindingsPanelFollowsTheActiveDocument(t *testing.T) {
	bindingsPanel = ged.NewBindingsPanel()
	defer func() { bindingsPanel = nil }()

	first := ged.NewGedView()
	second := ged.NewGedView()
	dropWidget(t, first, "gui.Button")
	dropWidget(t, second, "gui.Edit")
	dropWidget(t, second, "gui.CheckBox")

	bindBindingsPanelTo(first)
	if got := len(bindingsPanel.Rows()); got != 1 {
		t.Errorf("bound to the first document the panel lists %d row(s), want 1 (the Button's OnClick)", got)
	}

	bindBindingsPanelTo(second)
	if got := len(bindingsPanel.Rows()); got != 2 {
		t.Errorf("after the tab change the panel lists %d row(s), want 2 — it is still on the first document", got)
	}

	// A tab with no canvas behind it must leave the panel where it was rather
	// than blanking it or panicking.
	bindBindingsPanelTo(nil)
	if got := len(bindingsPanel.Rows()); got != 2 {
		t.Errorf("binding nil left the panel listing %d row(s), want the second document's 2", got)
	}
}

// TestBindingsPanelIsDockedAndRebound reads createPanels because it needs a
// live frame to run. The panel has to be created and docked, and
// bindBindingsPanelTo has to be called both at startup and from the tab-change
// callback — one call site alone leaves the panel either empty or stuck on the
// first document.
func TestBindingsPanelIsDockedAndRebound(t *testing.T) {
	body := funcBody(t, "design.go", "func createPanels(mainFrame *gui.Frame) {")
	for _, want := range []string{
		"bindingsPanel = ged.NewBindingsPanel()",
		"rightDockI.AddView(bindingsPanel)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("createPanels no longer contains %q", want)
		}
	}
	if n := strings.Count(body, "bindBindingsPanelTo(gv)"); n < 2 {
		t.Errorf("createPanels binds the 信号/槽 panel %d time(s); it must bind at startup and again on every tab change", n)
	}
}
