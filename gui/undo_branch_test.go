package gui

import "testing"

// A reversible edit with optional grouping exercises both ordinary Push and
// the merge protocol without depending on a particular designer widget.
type branchTestEdit struct {
	value        *string
	before, next string
	group        string
	mergeCalls   int
}

func (edit *branchTestEdit) Redo()        { *edit.value = edit.next }
func (edit *branchTestEdit) Undo()        { *edit.value = edit.before }
func (edit *branchTestEdit) Text() string { return edit.next }
func (edit *branchTestEdit) MergeWidth(next ICommand) bool {
	edit.mergeCalls++
	other, ok := next.(*branchTestEdit)
	if !ok || edit.group == "" || edit.group != other.group || edit.value != other.value {
		return false
	}
	edit.next = other.next
	return true
}

func pushBranchEdit(stack *UndoStack, value *string, next, group string) *branchTestEdit {
	edit := &branchTestEdit{value: value, before: *value, next: next, group: group}
	stack.Push(edit)
	return edit
}

func TestUndoBranchInvalidatesDiscardedCleanPoint(t *testing.T) {
	stack := NewUndoStack("branch")
	value := "original"
	pushBranchEdit(stack, &value, "saved A", "")
	stack.SetClean()
	stack.Undo()
	pushBranchEdit(stack, &value, "unsaved B", "")
	if stack.IsClean() || value != "unsaved B" || stack.CanRedo() {
		t.Fatal("new branch reused the discarded saved-A index as a clean point")
	}
	for i := 0; i < 2; i++ {
		stack.Undo()
		if value != "original" || stack.IsClean() {
			t.Fatal("Undo falsely returned to unreachable saved A")
		}
		stack.Redo()
		if value != "unsaved B" || stack.IsClean() {
			t.Fatal("Redo falsely marked branch B as saved A")
		}
	}
	stack.SetClean() // Explicitly saving B establishes a new reachable point.
	stack.Undo()
	if stack.IsClean() {
		t.Fatal("Undo from saved B should be dirty")
	}
	stack.Redo()
	if !stack.IsClean() {
		t.Fatal("Redo to newly saved B should be clean")
	}
}

func TestUndoBranchRetainsCleanPointInSurvivingHistory(t *testing.T) {
	stack := NewUndoStack("branch")
	value := "original"
	pushBranchEdit(stack, &value, "A", "")
	pushBranchEdit(stack, &value, "B", "")
	stack.Undo()
	pushBranchEdit(stack, &value, "C", "")
	stack.Undo()
	if value != "A" || stack.IsClean() {
		t.Fatal("wrong intermediate branch state")
	}
	stack.Undo()
	if value != "original" || !stack.IsClean() {
		t.Fatal("branch incorrectly discarded an earlier reachable clean point")
	}
}

func TestUndoBranchAtCleanPointKeepsSavedBoundary(t *testing.T) {
	stack := NewUndoStack("branch")
	value := "original"
	pushBranchEdit(stack, &value, "saved A", "typing")
	stack.SetClean()
	pushBranchEdit(stack, &value, "B", "typing")
	stack.Undo()
	if !stack.IsClean() {
		t.Fatal("test requires branching exactly at saved A")
	}
	pushBranchEdit(stack, &value, "C", "typing")
	if stack.IsClean() || stack.Count() != 2 || stack.CanRedo() {
		t.Fatal("branch crossed or merged the saved boundary")
	}
	stack.Undo()
	if value != "saved A" || !stack.IsClean() {
		t.Fatal("saved branch point must remain reachable")
	}
}

func TestUndoPushDoesNotMergeAcrossARedoBranch(t *testing.T) {
	stack := NewUndoStack("branch")
	value := "original"
	first := pushBranchEdit(stack, &value, "A", "gesture")
	pushBranchEdit(stack, &value, "B", "other-gesture")
	stack.SetClean()
	stack.Undo()
	beforeMergeCalls := first.mergeCalls
	pushBranchEdit(stack, &value, "C", "gesture")
	if first.mergeCalls != beforeMergeCalls {
		t.Fatal("Push after Undo tried to merge across the branch boundary")
	}
	if stack.Count() != 2 || stack.Current() != 2 || stack.CanRedo() || stack.IsClean() {
		t.Fatal("merge left the old redo branch reachable or reused its clean point")
	}
	for _, step := range []struct {
		undo bool
		want string
	}{{true, "A"}, {true, "original"}, {false, "A"}, {false, "C"}} {
		if step.undo {
			stack.Undo()
		} else {
			stack.Redo()
		}
		if value != step.want || stack.IsClean() {
			t.Fatalf("branched history = %q clean=%v, want %q dirty", value, stack.IsClean(), step.want)
		}
	}
}

func TestUndoTipMergeStillWorksWithoutCrossingSave(t *testing.T) {
	stack := NewUndoStack("branch")
	value := "original"
	pushBranchEdit(stack, &value, "A", "typing")
	pushBranchEdit(stack, &value, "AB", "typing")
	if stack.Count() != 1 || value != "AB" {
		t.Fatal("ordinary tip edits stopped merging")
	}
	stack.Undo()
	if value != "original" || !stack.IsClean() {
		t.Fatal("merged edit lost its original clean baseline")
	}
	stack.Redo()
	stack.SetClean()
	pushBranchEdit(stack, &value, "ABC", "typing")
	if stack.Count() != 2 || stack.IsClean() {
		t.Fatal("tip merge overwrote a saved state")
	}
	stack.Undo()
	if value != "AB" || !stack.IsClean() {
		t.Fatal("saved state is no longer reachable after tip edit")
	}
}

func TestUndoNilPushDoesNotDiscardRedoOrItsCleanPoint(t *testing.T) {
	stack := NewUndoStack("branch")
	value := "original"
	pushBranchEdit(stack, &value, "saved", "")
	stack.SetClean()
	stack.Undo()
	stack.Push(nil)
	if !stack.CanRedo() || stack.Count() != 1 {
		t.Fatal("nil command discarded the redo branch")
	}
	stack.Redo()
	if !stack.IsClean() || value != "saved" {
		t.Fatal("nil command invalidated a valid saved point")
	}
}
