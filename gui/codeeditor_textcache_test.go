package gui

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func assertEditorTextCache(t *testing.T, e *CodeEditor) {
	t.Helper()
	want := strings.Join(e.lines, "\n")
	// Modified is intentionally the first getter: title polling must see a
	// splice even when nobody explicitly requested Text beforehand.
	if got := e.Modified(); got != (want != e.savedText) {
		t.Fatalf("Modified() = %v for text %q, saved %q", got, want, e.savedText)
	}
	if got := e.Text(); got != want {
		t.Fatalf("Text() = %q, want buffer %q", got, want)
	}
	if got := e.textRuneCount; got != utf8.RuneCountInString(want) {
		t.Fatalf("cached rune count = %d, want %d", got, utf8.RuneCountInString(want))
	}
}

func TestCodeEditorTextCacheExactSavedContentAndRevision(t *testing.T) {
	e := NewCodeEditor()
	const original = "package p\n// 中文🙂\nvar aa = 1\n"
	e.SetText(original)
	e.MarkSaved()
	base := e.TextRevision()
	if e.Modified() {
		t.Fatal("freshly saved editor is dirty")
	}
	changed := strings.Replace(original, "aa", "bb", 1)
	e.ReplaceAllText(changed) // same byte count, distinct content
	if !e.Modified() || e.TextRevision() <= base {
		t.Fatal("same-length replacement did not change dirty state/revision")
	}
	changedRev := e.TextRevision()
	e.Undo()
	if e.Modified() || e.Text() != original || e.TextRevision() <= changedRev {
		t.Fatal("undo to identical saved bytes must be clean at a new revision")
	}
	e.Redo()
	if !e.Modified() || e.Text() != changed {
		t.Fatal("redo did not restore the dirty buffer")
	}
	e.MarkSaved()
	savedRev := e.TextRevision()
	e.Undo()
	if !e.Modified() {
		t.Fatal("undo away from a newly saved state must become dirty")
	}
	e.Redo()
	if e.Modified() || e.TextRevision() <= savedRev {
		t.Fatal("redo to newly saved content must become clean")
	}
	// Restoring content without following undo history has the same semantics.
	e.SetText(original)
	if !e.Modified() {
		t.Fatal("SetText away from the save snapshot must be dirty")
	}
	e.SetText(changed)
	if e.Modified() {
		t.Fatal("SetText restoring saved bytes must be clean")
	}
	assertEditorTextCache(t, e)
}

func TestCodeEditorTextCacheNoopAndUIStateKeepRevision(t *testing.T) {
	e := NewCodeEditor()
	e.SetText("alpha\nbeta")
	e.MarkSaved()
	rev := e.TextRevision()
	e.SetText(e.Text())
	e.ReplaceAllText(e.Text())
	e.rebuildText()
	e.cursorCol = 2
	e.editorSelectAll()
	e.clearSelection()
	e.SetScrollY(20)
	e.MarkSaved()
	if e.TextRevision() != rev || e.Modified() {
		t.Fatal("no-op content, cursor, selection, scroll or save changed revision")
	}
}

func TestCodeEditorTextCacheMutationPaths(t *testing.T) {
	selectPrefix := func(e *CodeEditor) {
		e.hasSelection = true
		e.selStartLine, e.selStartCol = 0, 0
		e.selEndLine, e.selEndCol = 0, 2
	}
	cases := []struct {
		name string
		edit func(*CodeEditor)
	}{
		{"typing", func(e *CodeEditor) { e.OnTextInput("中") }},
		{"auto-close", func(e *CodeEditor) { e.OnTextInput("(") }},
		{"set-text", func(e *CodeEditor) { e.SetText("different\ntext") }},
		{"replace-all-text", func(e *CodeEditor) { e.ReplaceAllText("replacement\n") }},
		{"replace-selection", func(e *CodeEditor) { selectPrefix(e); e.ReplaceSelection("X\nY") }},
		{"insert-selection", func(e *CodeEditor) { e.ReplaceSelection("X\nY") }},
		{"delete-selection", func(e *CodeEditor) { selectPrefix(e); e.DeleteSelection() }},
		{"duplicate-line", func(e *CodeEditor) { e.DuplicateLines() }},
		{"move-line", func(e *CodeEditor) { e.moveLineDown() }},
		{"join-lines", func(e *CodeEditor) { e.JoinLines() }},
		{"trim", func(e *CodeEditor) { e.TrimTrailingWhitespace() }},
		{"comment", func(e *CodeEditor) { e.ToggleLineComment() }},
		{"indent", func(e *CodeEditor) { selectPrefix(e); e.IndentSelection() }},
		{"replace-current", func(e *CodeEditor) {
			e.findText, e.replaceText = "alpha", "omega"
			e.findUpdateMatches()
			e.replaceCurrent()
		}},
		{"replace-all", func(e *CodeEditor) {
			e.findText, e.replaceText = "alpha", "omega"
			e.replaceAll()
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewCodeEditor()
			e.SetText("alpha  \nbeta\n")
			e.MarkSaved()
			before, rev := e.Text(), e.TextRevision()
			e.SigChanged(func(text string) {
				assertEditorTextCache(t, e)
				if e.Text() != text {
					t.Fatal("changed callback observed an unpublished buffer")
				}
			})
			tc.edit(e)
			assertEditorTextCache(t, e)
			if e.Text() == before || e.TextRevision() <= rev || !e.Modified() {
				t.Fatal("editing path left stale content/revision/dirty state")
			}
		})
	}
}

func TestCodeEditorTextCacheLowLevelSplicesBeforePublish(t *testing.T) {
	cases := []struct {
		name string
		edit func(*CodeEditor)
	}{
		{"raw-insert", func(e *CodeEditor) { e.insertRawText("X\nY") }},
		{"single-insert", func(e *CodeEditor) { e.insertTextAtCursor("X") }},
		{"multiline-insert", func(e *CodeEditor) { e.insertMultilineAtCursor([]string{"X", "Y"}) }},
		{"delete-range", func(e *CodeEditor) { e.deleteRange(0, 1, 4) }},
		{"multi-insert", func(e *CodeEditor) { e.insertAtAllCursors("X") }},
		{"multi-backspace", func(e *CodeEditor) { e.backspaceAtAllCursors() }},
		{"multi-delete", func(e *CodeEditor) { e.deleteAtAllCursors() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewCodeEditor()
			e.SetText("abc\ndef")
			e.MarkSaved()
			e.cursorCol = 1
			e.additionalCursors = []cursorPos{{line: 1, col: 1}}
			rev := e.TextRevision()
			tc.edit(e)
			assertEditorTextCache(t, e)
			if e.TextRevision() <= rev || !e.Modified() {
				t.Fatal("low-level splice was invisible before rebuildText")
			}
			rev = e.TextRevision()
			e.rebuildText()
			if e.TextRevision() != rev {
				t.Fatal("publishing already cached content changed revision twice")
			}
		})
	}
}

func TestCodeEditorTextCacheLinesSnapshotCannotMutateBuffer(t *testing.T) {
	e := NewCodeEditor()
	e.SetText("alpha\nbeta")
	e.MarkSaved()
	rev := e.TextRevision()
	lines := e.Lines()
	lines[0] = "omega"
	if e.Text() != "alpha\nbeta" || e.Modified() || e.TextRevision() != rev {
		t.Fatal("mutating a Lines snapshot changed the editor")
	}
	e.OnTextInput("X")
	if lines[1] != "beta" || lines[0] != "omega" {
		t.Fatal("an editor edit changed the retained snapshot")
	}
}

func TestCodeEditorTextCacheIdleReadsDoNotAllocate(t *testing.T) {
	e := NewCodeEditor()
	src := strings.Repeat("// 中文🙂 cached document\n", 2048)
	e.SetText(src)
	e.MarkSaved()
	for _, dirty := range []bool{false, true} {
		if dirty {
			e.ReplaceAllText(src + "// edit\n")
		}
		_ = e.Modified() // allow one exact comparison for a new revision
		if got := testing.AllocsPerRun(100, func() {
			_ = e.Text()
			_ = e.TextRevision()
			_ = e.Modified()
		}); got != 0 {
			t.Errorf("idle text/revision/dirty reads allocated %v times, dirty=%v", got, dirty)
		}
	}
}
