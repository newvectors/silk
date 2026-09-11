package gui

import (
	"reflect"
	"testing"
)

func TestSDKComboSelectionAndItems(t *testing.T) {
	combo := NewComboBox()
	if combo.Text() != "" || combo.ActiveIndex() != -1 {
		t.Fatal("new SDK control contains demo text or an implicit selection")
	}
	items := []string{"Windows", "Linux", "macOS"}
	combo.SetItems(items)
	items[0] = "mutated"
	copy := combo.Items()
	copy[1] = "mutated"
	if !reflect.DeepEqual(combo.Items(), []string{"Windows", "Linux", "macOS"}) {
		t.Fatal("items alias caller-owned storage")
	}
	var changes []int
	var submits int
	combo.SigSelectionChanged(func(_ interface{}, index int) { changes = append(changes, index) })
	combo.SigSubmit(func(_ interface{}) { submits++ })
	combo.SetActiveIndex(2)
	combo.SetActiveIndex(2)
	if combo.Text() != "macOS" || combo.ActiveItem().Text != "macOS" || combo.sub.ActiveIndex() != 2 {
		t.Fatal("selection, display and popup disagree")
	}
	// Moving the popup highlight is not a committed selection.
	combo.sub.activeIndex = 0
	if combo.ActiveItem().Text != "macOS" {
		t.Fatal("uncommitted popup highlight leaked through ActiveItem")
	}
	combo.SetActiveIndex(-1)
	combo.SetActiveIndex(500)
	if combo.ActiveIndex() != -1 || combo.Text() != "" || combo.ActiveItem().Text != "" {
		t.Fatal("invalid selection was not cleared")
	}
	if !reflect.DeepEqual(changes, []int{2, -1}) || submits != 0 {
		t.Fatalf("programmatic event contract: changes=%v submits=%d", changes, submits)
	}
}

func TestSDKComboMutationKeepsCommittedSelectionConsistent(t *testing.T) {
	combo := NewComboBox()
	combo.SetItems([]string{"A", "B", "C"})
	combo.SetActiveIndex(1)
	combo.Insert(0, ListItem{Text: "new"})
	if combo.ActiveIndex() != 2 || combo.Text() != "B" {
		t.Fatal("insert moved the selection to another item")
	}
	combo.Remove(0)
	if combo.ActiveIndex() != 1 || combo.Text() != "B" {
		t.Fatal("remove-before-selection moved to another item")
	}
	combo.SetItem(1, ListItem{Text: "renamed"})
	if combo.Text() != "renamed" {
		t.Fatal("replacing the selected item left stale text")
	}
	combo.Remove(1)
	if combo.ActiveIndex() != -1 || combo.Text() != "" {
		t.Fatal("removed selected item is still committed")
	}
	combo.SetActiveIndex(0)
	combo.Clear()
	if combo.Count() != 0 || combo.ActiveIndex() != -1 || combo.Text() != "" {
		t.Fatal("Clear left a stale index/text")
	}
	_ = combo.RemoveLast() // safe for an already empty list
}

func TestSDKComboReplacingEditorDetachesOldChild(t *testing.T) {
	combo := NewComboBox()
	old := combo.EditWidget()
	replacement := NewEdit()
	combo.SetEditWidget(replacement)
	if old.Parent() != nil || replacement.Parent() != combo {
		t.Fatal("old and new editors still render in the same combo")
	}
	for _, child := range combo.Children() {
		if child == old {
			t.Fatal("old editor remains in the rendering/hit-test tree")
		}
	}
	combo.SetEditWidget(replacement)
	if replacement.Parent() != combo {
		t.Fatal("setting the same editor detached it")
	}
}

func TestSDKDatePickerUsesConfiguredLayout(t *testing.T) {
	picker := NewDatePicker()
	picker.SetDate(2024, 2, 29)
	changes := 0
	picker.SigDateChanged(func(_, _, _ int) { changes++ })
	for _, tc := range []struct{ layout, want string }{
		{"02/01/2006", "29/02/2024"},
		{"2006年1月2日", "2024年2月29日"},
		{"", "2024-02-29"},
	} {
		picker.SetFormat(tc.layout)
		if got := picker.displayText(); got != tc.want {
			t.Fatalf("layout %q displayed %q, want %q", tc.layout, got, tc.want)
		}
	}
	if changes != 0 {
		t.Fatal("formatting emitted a date value change")
	}
	picker.SetEnabled(false)
	picker.OnLeftDown(0, 0)
	if picker.popup != nil || picker.pushed {
		t.Fatal("disabled date picker opened a popup")
	}
}
