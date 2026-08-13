package ged

import (
	"testing"

	"github.com/uk0/silk/graph"
	"github.com/uk0/silk/gui"
)

// keyInspector builds the tree the keyboard tests walk: a container holding one
// widget, plus a top-level sibling. Rows are [root, leftPane, btnSave,
// btnCancel], so a nested row sits between two others and an off-by-one in the
// walk lands on a different widget rather than on nothing.
func keyInspector(t *testing.T) (view *GedView, insp *ObjectInspector, box, save, cancel *FakeWidget) {
	t.Helper()
	view = NewGedView()
	scene := view.GedScene()
	box = sceneWidget(t, scene, "gui.VBox", "leftPane", 10, 10, 60, 40)
	save = nestChild(t, box, "btnSave")
	cancel = sceneWidget(t, scene, "gui.Button", "btnCancel", 10, 60, 40, 8)
	insp = boundInspector(view)
	if got := insp.RowNames(); !eqStrings(got, []string{"Form", "leftPane", "btnSave", "btnCancel"}) {
		t.Fatalf("fixture rows = %v, want [Form leftPane btnSave btnCancel]", got)
	}
	return
}

// itemLabel names an inspector payload for a failure message.
func itemLabel(it graph.IItem) string {
	if it == nil {
		return "<nothing>"
	}
	if fake, ok := it.(*FakeWidget); ok {
		return fake.WidgetName()
	}
	return "<form root>"
}

// itemLabels names a list of payloads, for the canvas selection.
func itemLabels(items []graph.IItem) []string {
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, itemLabel(it))
	}
	return names
}

// childNames lists the widget names of an item's children, top to bottom.
func childNames(parent graph.IItem) []string {
	names := make([]string, 0, len(parent.Children()))
	for _, ch := range parent.Children() {
		names = append(names, itemLabel(ch))
	}
	return names
}

// TestInspectorArrowsWalkRowsAndClamp: Up/Down walk the tree one row at a time
// and stop at its ends. Asserted on the payload of each row, not its index — an
// index walk that skipped the nested row would still count to four.
func TestInspectorArrowsWalkRowsAndClamp(t *testing.T) {
	view, insp, box, save, cancel := keyInspector(t)
	root := graph.IItem(view.GedScene())

	// Down from a fresh panel (nothing highlighted) lands on the first row.
	walk := []graph.IItem{root, graph.IItem(box), graph.IItem(save), graph.IItem(cancel)}
	for i, want := range walk {
		insp.OnKeyDown(gui.KeyDown, false)
		if got := insp.SelectedItem(); got != want {
			t.Fatalf("KeyDown #%d selected %q, want %q", i+1, itemLabel(got), itemLabel(want))
		}
	}
	insp.OnKeyDown(gui.KeyDown, false)
	if got := insp.SelectedItem(); got != graph.IItem(cancel) {
		t.Fatalf("KeyDown past the last row selected %q, want it to stay on btnCancel", itemLabel(got))
	}

	for i := len(walk) - 2; i >= 0; i-- {
		insp.OnKeyDown(gui.KeyUp, false)
		if got := insp.SelectedItem(); got != walk[i] {
			t.Fatalf("KeyUp back to row %d selected %q, want %q", i, itemLabel(got), itemLabel(walk[i]))
		}
	}
	insp.OnKeyDown(gui.KeyUp, false)
	if got := insp.SelectedItem(); got != root {
		t.Fatalf("KeyUp past the first row selected %q, want it to stay on the form root", itemLabel(got))
	}
}

// TestInspectorArrowSelectsOnCanvas is what makes the arrows worth having: the
// keyboard goes through selectRow, so the canvas selection — and every panel
// that follows it — lands on the widget the highlight names. A bare index move
// leaves the canvas wherever the last mouse press left it.
func TestInspectorArrowSelectsOnCanvas(t *testing.T) {
	view, insp, _, save, _ := keyInspector(t)

	insp.OnKeyDown(gui.KeyDown, false) // root
	insp.OnKeyDown(gui.KeyDown, false) // leftPane
	insp.OnKeyDown(gui.KeyDown, false) // btnSave

	got := view.Selection().ItemList()
	if len(got) != 1 || got[0] != graph.IItem(save) {
		t.Fatalf("after arrowing to btnSave the canvas selection is %v, want exactly [btnSave]", itemLabels(got))
	}
}

// TestInspectorArrowFiresSelectCallback: SigSelect is how the host learns a row
// was chosen (the property sheet and code panel hang off it), and a keyboard
// move must announce itself exactly as a press does.
func TestInspectorArrowFiresSelectCallback(t *testing.T) {
	_, insp, box, save, _ := keyInspector(t)

	var seen []graph.IItem
	insp.SigSelect(func(it graph.IItem) { seen = append(seen, it) })

	insp.OnKeyDown(gui.KeyDown, false) // root
	insp.OnKeyDown(gui.KeyDown, false) // leftPane
	insp.OnKeyDown(gui.KeyDown, false) // btnSave

	if len(seen) != 3 {
		t.Fatalf("three arrow presses fired SigSelect %d times (%v), want 3", len(seen), itemLabels(seen))
	}
	if seen[1] != graph.IItem(box) || seen[2] != graph.IItem(save) {
		t.Errorf("SigSelect reported %v, want the walked items [<form root> leftPane btnSave]", itemLabels(seen))
	}
}

// TestInspectorF2RenamesSelectedRow: F2 renames the widget the tree highlights,
// reusing the canvas's own prompt. The row is nested, which is the case the
// keyboard exists for — reaching it on the canvas needs the mouse first.
func TestInspectorF2RenamesSelectedRow(t *testing.T) {
	_, insp, _, save, _ := keyInspector(t)
	calls := answerRename(t, "renamed", true)

	insp.OnKeyDown(gui.KeyDown, false) // root
	insp.OnKeyDown(gui.KeyDown, false) // leftPane
	insp.OnKeyDown(gui.KeyDown, false) // btnSave
	insp.OnKeyDown(gui.KeyF2, false)

	if *calls != 1 {
		t.Fatalf("F2 on the tree opened the name prompt %d times, want 1", *calls)
	}
	if got := save.WidgetName(); got != "renamed" {
		t.Errorf("btnSave is named %q after F2, want renamed", got)
	}
}

// TestInspectorDeleteGoesThroughTheCommand: Delete removes the highlighted
// widget AND one Ctrl+Z puts it back where it was. A raw detach would pass the
// first half and lose the widget for good on the second.
func TestInspectorDeleteGoesThroughTheCommand(t *testing.T) {
	view, insp, box, save, _ := keyInspector(t)
	scene := view.GedScene()

	insp.OnKeyDown(gui.KeyDown, false) // root
	insp.OnKeyDown(gui.KeyDown, false) // leftPane
	insp.OnKeyDown(gui.KeyDown, false) // btnSave
	insp.OnKeyDown(gui.KeyDelete, false)

	if got := childNames(box); len(got) != 0 {
		t.Fatalf("after Delete the container still holds %v, want btnSave gone", got)
	}

	scene.UndoStack().Undo()
	if got := childNames(box); !eqStrings(got, []string{"btnSave"}) {
		t.Fatalf("after undo the container holds %v, want [btnSave] back inside it", got)
	}
	if save.Parent() != graph.IItem(box) {
		t.Errorf("after undo btnSave is parented to %q, want the container it was deleted from", itemLabel(save.Parent()))
	}
}

// TestInspectorRootRowRefusesDeleteAndRename: row 0 is the form itself, not a
// widget. The walk first goes down to a widget and back up, so the canvas
// selection is still holding leftPane when the keys arrive — a Delete that read
// the canvas instead of the highlighted row would take that whole subtree out.
func TestInspectorRootRowRefusesDeleteAndRename(t *testing.T) {
	view, insp, box, save, _ := keyInspector(t)
	scene := view.GedScene()
	calls := answerRename(t, "renamed", true)

	for i := 0; i < 3; i++ {
		insp.OnKeyDown(gui.KeyDown, false) // root -> leftPane -> btnSave
	}
	for i := 0; i < 3; i++ {
		insp.OnKeyDown(gui.KeyUp, false) // back to the form root
	}
	if insp.selectedIdx != 0 {
		t.Fatalf("walking back up left the highlight on row %d, want the form root", insp.selectedIdx)
	}
	if view.Selection().IsEmpty() {
		t.Fatal("the canvas selection emptied itself; this test needs it left on a widget")
	}

	insp.OnKeyDown(gui.KeyDelete, false)
	insp.OnKeyDown(gui.KeyF2, false)

	if got := sceneOrder(scene); !eqStrings(got, []string{"leftPane", "btnCancel"}) {
		t.Errorf("Delete on the form root left the form holding %v, want [leftPane btnCancel]", got)
	}
	if got := childNames(box); !eqStrings(got, []string{"btnSave"}) {
		t.Errorf("Delete on the form root left the container holding %v, want [btnSave]", got)
	}
	if n := scene.UndoStack().Count(); n != 0 {
		t.Errorf("Delete on the form root pushed %d command(s), want none", n)
	}
	if *calls != 0 {
		t.Errorf("F2 on the form root opened the name prompt %d times, want none", *calls)
	}
	if got := save.WidgetName(); got != "btnSave" {
		t.Errorf("F2 on the form root renamed btnSave to %q", got)
	}
}

// TestInspectorKeysWithoutViewAreInert: a panel bound to a scene nobody shows
// has rows but no canvas to act on — reparentInto already guards this way. The
// keys must walk the tree and then do nothing, rather than call through a nil
// view. A panel that was never bound at all takes the same keys.
func TestInspectorKeysWithoutViewAreInert(t *testing.T) {
	scene := NewGedScene()
	box := sceneWidget(t, scene, "gui.VBox", "leftPane", 10, 10, 60, 40)
	nestChild(t, box, "btnSave")

	insp := NewObjectInspector()
	insp.SetScene(scene)
	if insp.view != nil {
		t.Fatal("a scene with no GedView still handed the inspector a view; this test needs view == nil")
	}
	answerRename(t, "renamed", true)

	insp.OnKeyDown(gui.KeyDown, false) // root
	insp.OnKeyDown(gui.KeyDown, false) // leftPane
	insp.OnKeyDown(gui.KeyDown, false) // btnSave
	insp.OnKeyDown(gui.KeyDelete, false)
	insp.OnKeyDown(gui.KeyF2, false)

	if got := childNames(box); !eqStrings(got, []string{"btnSave"}) {
		t.Errorf("keys on a view-less panel changed the tree: container holds %v, want [btnSave]", got)
	}

	// The same keys on a panel whose SetScene was never called.
	fresh := NewObjectInspector()
	fresh.OnKeyDown(gui.KeyDown, false)
	fresh.OnKeyDown(gui.KeyUp, false)
	fresh.OnKeyDown(gui.KeyDelete, false)
	fresh.OnKeyDown(gui.KeyF2, false)
	if got := fresh.SelectedItem(); got != nil {
		t.Errorf("an unbound panel highlighted %q, want nothing", itemLabel(got))
	}
}

// TestInspectorPressTakesFocus: keys only reach a widget that holds focus, so
// the press that selects a row must claim it — the sibling panels (code outline,
// TODO) do the same. A press in the header band leaves focus alone, because that
// band belongs to the filter box.
func TestInspectorPressTakesFocus(t *testing.T) {
	view, insp, _, _, _ := keyInspector(t)

	other := gui.NewButton()
	other.SetFocus()

	insp.OnLeftDown(10, 4) // header band, above contentTop
	if insp.HasFocus() {
		t.Error("a press in the header band stole focus from the filter box")
	}

	insp.OnLeftDown(10, rowMidY(insp, 2))
	if !insp.HasFocus() {
		t.Fatal("a row press left focus elsewhere; the window dispatches keys to the focused widget, so the tree would stay deaf")
	}
	// The press still selects, and the keys that follow it now land here.
	insp.OnKeyDown(gui.KeyDown, false)
	if got := insp.SelectedItem(); got != graph.IItem(view.GedScene().Children()[1]) {
		t.Errorf("KeyDown after a press on btnSave selected %q, want btnCancel", itemLabel(got))
	}
}

// TestInspectorSatisfiesKeyDownInterface pins the dispatch contract on a live
// instance: gui's window looks up IEventKeyDown on the focused widget, so a
// renamed or mis-signed OnKeyDown means no key ever arrives.
func TestInspectorSatisfiesKeyDownInterface(t *testing.T) {
	var w gui.IWidget = NewObjectInspector()
	if _, ok := w.(gui.IEventKeyDown); !ok {
		t.Fatal("ObjectInspector does not satisfy gui.IEventKeyDown; the window dispatches keys through that interface and the tree would never receive one")
	}
}
