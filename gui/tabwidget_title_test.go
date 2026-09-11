package gui

import "testing"

func TestTabTitlePreservesPageAndHistory(t *testing.T) {
	tabs := NewTabWidget()
	ed := NewCodeEditor()
	ed.SetText("before")
	ed.MarkSaved()
	ed.ReplaceAllText("after")
	tabs.AddTab(ed, "a.go", nil)
	tabs.SetTabTitle(0, "a.go *")
	if tabs.TabTitle(0) != "a.go *" || tabs.Stack().Page(0) != IWidget(ed) {
		t.Fatal("metadata replaced page")
	}
	ed.Undo()
	if ed.Text() != "before" || ed.Modified() {
		t.Fatal("lost clean undo state")
	}
	ed.Redo()
	if ed.Text() != "after" || !ed.Modified() {
		t.Fatal("lost redo state")
	}
	tabs.SetTabTitle(-1, "invalid")
	tabs.SetTabTitle(9, "invalid")
	if tabs.TabTitle(9) != "" {
		t.Fatal("invalid tab title")
	}
}
