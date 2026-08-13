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

// TestBindingsPanelKeepsFollowingAfterTheReAim crosses the two halves the suite
// covered apart: the live refresh (one panel, one design, a command pushed) and
// the re-aim (the panel pointed at a second design, its rows read). The crossing
// is the ordinary session — switch tabs, then edit — and it is what tells a
// panel that follows the active document from one that subscribes to the
// design-changed list once and then lists the first document opened forever.
//
// The second half switches back and edits there, which the tab-change callback
// does on every return visit: the re-aim has to leave exactly the current
// document driving the panel, whether or not the panel has shown it before.
func TestBindingsPanelKeepsFollowingAfterTheReAim(t *testing.T) {
	bindingsPanel = ged.NewBindingsPanel()
	defer func() { bindingsPanel = nil }()

	first := ged.NewGedView()
	second := ged.NewGedView()
	dropWidget(t, first, "gui.Button")

	bindBindingsPanelTo(first)
	bindBindingsPanelTo(second)

	// The tab change happened, and now the user edits the document they
	// switched to.
	dropWidget(t, second, "gui.Edit")
	if got := bindingRowKeys(); len(got) != 1 || got[0] != "gui.Edit/OnChanged" {
		t.Fatalf("after the drop on the re-aimed document the panel lists %v, want [gui.Edit/OnChanged] — it never heard the command", got)
	}

	// Back to the first tab, and an edit there.
	bindBindingsPanelTo(first)
	if got := bindingRowKeys(); len(got) != 1 || got[0] != "gui.Button/OnClick" {
		t.Fatalf("re-aimed back the panel lists %v, want the first document's [gui.Button/OnClick]", got)
	}
	dropWidget(t, first, "gui.CheckBox")
	want := []string{"gui.Button/OnClick", "gui.CheckBox/OnToggled"}
	got := bindingRowKeys()
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("after the drop on the first document the panel lists %v, want %v — the re-aim back left it deaf", got, want)
	}
}

// bindingRowKeys is the panel's content as "factory/event" strings, the shape
// the assertions above compare.
func bindingRowKeys() []string {
	rows := bindingsPanel.Rows()
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.Factory+"/"+row.Event)
	}
	return keys
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
