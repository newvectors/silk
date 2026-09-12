package ged

import (
	"strings"
	"testing"

	"github.com/uk0/silk/gui"
)

func TestOutlineEditorRevisionTracksUndoRedoWithoutHostHook(t *testing.T) {
	e := gui.NewCodeEditor()
	e.SetText("package p\nfunc alpha() {}\n")
	p := NewCodeOutlinePanel()
	p.SetEditor(e)
	e.ReplaceAllText("package p\nfunc omega() {}\n")
	if !p.RefreshIfStale() {
		t.Fatal("same-length edit did not invalidate automatically")
	}
	wantOutlineRows(t, p, []string{"0:omega"})
	e.Undo()
	if !p.RefreshIfStale() {
		t.Fatal("undo did not invalidate the outline")
	}
	wantOutlineRows(t, p, []string{"0:alpha"})
	e.Redo()
	if !p.RefreshIfStale() {
		t.Fatal("redo did not invalidate the outline")
	}
	wantOutlineRows(t, p, []string{"0:omega"})
	if p.RefreshIfStale() {
		t.Fatal("consumed revision re-parsed on the next frame")
	}
}

func TestOutlineRevisionCacheSwitchesEditorsAtSameRevision(t *testing.T) {
	a, b := gui.NewCodeEditor(), gui.NewCodeEditor()
	a.SetText("package p\nfunc alpha() {}\n")
	b.SetText("package p\nfunc omega() {}\n")
	if a.TextRevision() != b.TextRevision() {
		t.Fatal("fixture revisions must match")
	}
	p := NewCodeOutlinePanel()
	p.SetEditor(a)
	p.SetEditor(b)
	wantOutlineRows(t, p, []string{"0:omega"})
	if p.RefreshIfStale() {
		t.Fatal("binding did not seed the new editor revision")
	}
	p.SetEditor(nil)
	wantOutlineRows(t, p, nil)
	if p.RefreshIfStale() {
		t.Fatal("nil editor kept invalidating the outline")
	}
}

func TestOutlineRevisionCachePreservesUnchangedRows(t *testing.T) {
	e := gui.NewCodeEditor()
	e.SetText("package p\n" + strings.Repeat("// unchanged\n", 2048) + "func alpha() {}\n")
	p := NewCodeOutlinePanel()
	p.SetEditor(e)
	if len(p.flatList) != 1 {
		t.Fatalf("fixture rows = %d, want 1", len(p.flatList))
	}
	row := &p.flatList[0]
	if n := testing.AllocsPerRun(100, func() { p.RefreshIfStale() }); n != 0 {
		t.Fatalf("unchanged refresh allocated %v times", n)
	}
	if &p.flatList[0] != row {
		t.Fatal("unchanged content rebuilt the outline row storage")
	}
	e.SetText(e.Text())
	e.MarkSaved()
	if p.RefreshIfStale() {
		t.Fatal("same text/save marker unnecessarily invalidated the outline")
	}
}
