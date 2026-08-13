package gui

import "testing"

// TestListWidgetSetActiveIndexAcrossClear: Clear wipes the selection along with
// the items, so a host that rebuilds its rows (every type-ahead list does) comes
// back with ActiveIndex() == -1 — nothing highlighted, and every ActiveIndex()
// reader dead — until it selects again. SetActiveIndex is the way back in.
func TestListWidgetSetActiveIndexAcrossClear(t *testing.T) {
	lw := newItemList("A", "B", "C")

	lw.SetActiveIndex(1)
	if got := lw.ActiveIndex(); got != 1 {
		t.Fatalf("SetActiveIndex(1): ActiveIndex = %d, want 1", got)
	}

	lw.Clear()
	if got := lw.ActiveIndex(); got != -1 {
		t.Fatalf("after Clear: ActiveIndex = %d, want -1", got)
	}

	for _, s := range []string{"D", "E", "F"} {
		lw.Append(ListItem{Text: s})
	}
	lw.SetActiveIndex(0)
	if got := lw.ActiveIndex(); got != 0 {
		t.Fatalf("SetActiveIndex(0) after refill: ActiveIndex = %d, want 0", got)
	}
	if got, want := lw.ActiveItem().Text, "D"; got != want {
		t.Errorf("ActiveItem = %q, want the first row %q", got, want)
	}
}

// TestListWidgetSetActiveIndexClamps: the public setter goes through the same
// clamp as the keyboard navigation, so an out-of-range index from a caller that
// is off by a row lands on a real row instead of one that Draw never paints and
// Item() panics on.
func TestListWidgetSetActiveIndexClamps(t *testing.T) {
	lw := newItemList("A", "B", "C")

	lw.SetActiveIndex(99)
	if got := lw.ActiveIndex(); got != 2 {
		t.Errorf("SetActiveIndex(99) on 3 rows: ActiveIndex = %d, want 2 (clamped to the last row)", got)
	}
	lw.SetActiveIndex(-5)
	if got := lw.ActiveIndex(); got != 0 {
		t.Errorf("SetActiveIndex(-5): ActiveIndex = %d, want 0 (clamped to the first row)", got)
	}

	// An empty list has no row to select: "nothing selected" has to survive, or
	// ActiveItem and every host indexing its own parallel slice reads row 0 of
	// a list that has none.
	empty := NewListWidget()
	empty.SetActiveIndex(0)
	if got := empty.ActiveIndex(); got != -1 {
		t.Errorf("SetActiveIndex(0) on an empty list: ActiveIndex = %d, want -1", got)
	}
}

// TestListWidgetSetActiveIndexEmitsOnlyOnChange: the selection-changed callback
// is how a host keeps a preview pane or a status line in step with the list, so
// SetActiveIndex has to report the row it moved to — and only when it moved,
// since a repopulate that re-selects the same row is not a selection change.
func TestListWidgetSetActiveIndexEmitsOnlyOnChange(t *testing.T) {
	lw := newItemList("A", "B", "C")
	lw.SetActiveIndex(0)

	var fired [][]int
	lw.SigSelectionChanged(func(o interface{}, idx []int) {
		fired = append(fired, idx)
	})

	lw.SetActiveIndex(1)
	if len(fired) != 1 {
		t.Fatalf("SetActiveIndex(1) fired the callback %d times, want 1", len(fired))
	}
	if len(fired[0]) != 1 || fired[0][0] != 1 {
		t.Errorf("callback got selection %v, want [1] (the row now selected)", fired[0])
	}

	lw.SetActiveIndex(1)
	if len(fired) != 1 {
		t.Errorf("re-selecting the row already selected fired the callback %d times in total, want 1", len(fired))
	}
}
