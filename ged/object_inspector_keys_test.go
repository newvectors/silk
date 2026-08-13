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
//
// BackSpace is pressed alongside Delete because the canvas deletes on either
// key: leaving it out of this panel's own switch hands it to the forward, and
// the guard above is bypassed by the key next to the one it guards.
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
	insp.OnKeyDown(gui.KeyBackSpace, false)
	insp.OnKeyDown(gui.KeyF2, false)

	if got := sceneOrder(scene); !eqStrings(got, []string{"leftPane", "btnCancel"}) {
		t.Errorf("Delete/BackSpace on the form root left the form holding %v, want [leftPane btnCancel]", got)
	}
	if got := childNames(box); !eqStrings(got, []string{"btnSave"}) {
		t.Errorf("Delete/BackSpace on the form root left the container holding %v, want [btnSave]", got)
	}
	if n := scene.UndoStack().Count(); n != 0 {
		t.Errorf("Delete/BackSpace on the form root pushed %d command(s), want none", n)
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
// renamed or mis-signed OnKeyDown means no key ever arrives. The release half
// is looked up the same way, and carries the forward's other end.
func TestInspectorSatisfiesKeyDownInterface(t *testing.T) {
	var w gui.IWidget = NewObjectInspector()
	if _, ok := w.(gui.IEventKeyDown); !ok {
		t.Fatal("ObjectInspector does not satisfy gui.IEventKeyDown; the window dispatches keys through that interface and the tree would never receive one")
	}
	if _, ok := w.(gui.IEventKeyUp); !ok {
		t.Fatal("ObjectInspector does not satisfy gui.IEventKeyUp; the window dispatches releases through that interface and a forwarded press would never be released")
	}
}

// filterTo types text into the panel's own filter box, the way the user does:
// SearchBox.SetText fires SigTextChanged, which is what Init wires to SetFilter.
func filterTo(insp *ObjectInspector, text string) {
	insp.filterBox.SetText(text)
}

// TestInspectorFilteredRowZeroDeletesThatWidget: with a filter on, Rebuild
// lists the matches flat and never emits the form-root row, so row 0 is an
// ordinary widget — usually the only one the user narrowed down to. Delete must
// take it.
//
// Catches: a root guard that tests the row INDEX (selectedIdx > 0) instead of
// the item. It reads the first match as the document and refuses the key, so
// filtering to a widget and pressing Delete does nothing at all.
func TestInspectorFilteredRowZeroDeletesThatWidget(t *testing.T) {
	view, insp, box, save, _ := keyInspector(t)
	scene := view.GedScene()

	filterTo(insp, "btnsave")
	if got := insp.RowNames(); !eqStrings(got, []string{"btnSave"}) {
		t.Fatalf("filter \"btnsave\" left rows %v, want [btnSave] — this test needs a widget at row 0", got)
	}

	insp.OnKeyDown(gui.KeyDown, false) // the only row, row 0
	if got := insp.SelectedItem(); got != graph.IItem(save) {
		t.Fatalf("KeyDown under the filter selected %q, want btnSave", itemLabel(got))
	}
	insp.OnKeyDown(gui.KeyDelete, false)

	if got := childNames(box); len(got) != 0 {
		t.Fatalf("Delete on the filtered row left the container holding %v, want btnSave gone", got)
	}
	if n := scene.UndoStack().Count(); n != 1 {
		t.Fatalf("Delete on the filtered row pushed %d command(s), want 1", n)
	}
	scene.UndoStack().Undo()
	if got := childNames(box); !eqStrings(got, []string{"btnSave"}) {
		t.Errorf("after undo the container holds %v, want [btnSave] back inside it", got)
	}
}

// TestInspectorFilteredRowZeroRenamesThatWidget is the same first-class path on
// the rename key: a filter is how a user reaches a widget buried in a deep
// tree, and F2 is the only keyboard route to its name.
//
// Catches: the index-based root guard on the F2 branch — the filtered row 0
// opens no prompt and the widget keeps its name.
func TestInspectorFilteredRowZeroRenamesThatWidget(t *testing.T) {
	_, insp, _, save, _ := keyInspector(t)
	calls := answerRename(t, "renamed", true)

	filterTo(insp, "btnsave")
	if got := insp.RowNames(); !eqStrings(got, []string{"btnSave"}) {
		t.Fatalf("filter \"btnsave\" left rows %v, want [btnSave] — this test needs a widget at row 0", got)
	}

	insp.OnKeyDown(gui.KeyDown, false) // the only row, row 0
	insp.OnKeyDown(gui.KeyF2, false)

	if *calls != 1 {
		t.Fatalf("F2 on the filtered row opened the name prompt %d times, want 1", *calls)
	}
	if got := save.WidgetName(); got != "renamed" {
		t.Errorf("btnSave is named %q after F2 on the filtered row, want renamed", got)
	}
}

// TestInspectorForwardsUndoToTheCanvas is the immediate follow-up to the Delete
// this panel just gained: the row is gone, the user presses Ctrl+Z. The tree
// holds focus by then and the window routes a key to the focused widget alone —
// nothing bubbles to a parent — so a key the tree does not answer has to be
// handed to the canvas or it is lost.
//
// Catches: an OnKeyDown that swallows what it does not handle. Ctrl+Z after a
// tree-driven Delete does nothing and the widget stays deleted.
func TestInspectorForwardsUndoToTheCanvas(t *testing.T) {
	_, insp, box, save, _ := keyInspector(t)

	insp.OnKeyDown(gui.KeyDown, false) // root
	insp.OnKeyDown(gui.KeyDown, false) // leftPane
	insp.OnKeyDown(gui.KeyDown, false) // btnSave
	insp.OnKeyDown(gui.KeyDelete, false)
	if got := childNames(box); len(got) != 0 {
		t.Fatalf("Delete left the container holding %v, want btnSave gone", got)
	}

	holdKeys(t, gui.KeyCtrl)
	insp.OnKeyDown('Z', false)

	if got := childNames(box); !eqStrings(got, []string{"btnSave"}) {
		t.Fatalf("Ctrl+Z sent to the tree left the container holding %v, want [btnSave] back", got)
	}
	if save.Parent() != graph.IItem(box) {
		t.Errorf("after Ctrl+Z btnSave is parented to %q, want the container it was deleted from", itemLabel(save.Parent()))
	}
}

// TestInspectorForwardsEscToTheCanvas: the forward is not a Ctrl-chord special
// case. Esc carries no modifier and is the designer's deselect key.
//
// Catches: a forward narrowed to chords (or to a hand-written list of keys),
// which leaves every bare designer key dead while the tree holds focus.
func TestInspectorForwardsEscToTheCanvas(t *testing.T) {
	view, insp, _, save, _ := keyInspector(t)

	insp.OnKeyDown(gui.KeyDown, false) // root
	insp.OnKeyDown(gui.KeyDown, false) // leftPane
	insp.OnKeyDown(gui.KeyDown, false) // btnSave
	if got := view.Selection().ItemList(); len(got) != 1 || got[0] != graph.IItem(save) {
		t.Fatalf("the walk left the canvas holding %v, want exactly [btnSave]", itemLabels(got))
	}

	insp.OnKeyDown(gui.KeyEsc, false)

	if got := view.Selection().ItemList(); len(got) != 0 {
		t.Errorf("Esc sent to the tree left the canvas holding %v, want the selection cleared", itemLabels(got))
	}
}

// TestInspectorCtrlArrowResizesOnTheCanvas: the tree walks on the BARE arrows.
// Ctrl+arrow is the canvas's resize chord and the tree has no meaning for it,
// so it goes on with everything else.
//
// Catches: an arrow case that claims the key whatever modifier is held — the
// chord walks the tree instead of resizing, and Ctrl+arrow resize is dead from
// the moment a row is clicked.
func TestInspectorCtrlArrowResizesOnTheCanvas(t *testing.T) {
	_, insp, _, save, _ := keyInspector(t)
	save.SetBounds(12, 12, 30, 8) // a real box, so the grown height is a real number

	insp.OnKeyDown(gui.KeyDown, false) // root
	insp.OnKeyDown(gui.KeyDown, false) // leftPane
	insp.OnKeyDown(gui.KeyDown, false) // btnSave
	h0 := save.Height()

	holdKeys(t, gui.KeyCtrl)
	insp.OnKeyDown(gui.KeyDown, false)

	if got := save.Height(); got != h0+1 {
		t.Errorf("Ctrl+Down on the tree left btnSave %.0fmm tall, want %.0f — the chord must reach the canvas resize", got, h0+1)
	}
	if got := insp.SelectedItem(); got != graph.IItem(save) {
		t.Errorf("Ctrl+Down walked the highlight to %q; the chord is not the tree's key", itemLabel(got))
	}
}

// TestInspectorSpaceReleaseReachesTheCanvas: the window sends the release to
// the focused widget too, so a press forwarded to the canvas must have its
// release follow it. Space is where it shows — the canvas arms Space+drag
// panning on the press and disarms it on the release.
//
// Catches: forwarding OnKeyDown and not OnKeyUp. The canvas is left armed for
// good, and the next click on it drags the page instead of hitting the widget
// under the cursor.
func TestInspectorSpaceReleaseReachesTheCanvas(t *testing.T) {
	view, insp, _, _, cancel := keyInspector(t)

	insp.OnKeyDown(gui.KeySpace, false)
	insp.OnKeyUp(gui.KeySpace)

	px, py := view.MapFromScene(cancel.X()+1, cancel.Y()+1)
	view.OnLeftDown(px, py)
	if view.isPanning {
		t.Error("a click on the canvas started a pan: the Space release never reached it, so spacePanReady is still armed")
	}
	view.OnLeftUp(px, py)
}

// TestInspectorArrowUpFromNothingEntersTheList: with no row highlighted yet
// (selectedIdx == -1, a freshly bound panel) either arrow must land on the
// first row — that is the keyboard's way into the list, and Up is the half no
// other test presses first.
//
// Catches: dropping the lower clamp in moveSelection. Up computes -2, selectRow
// bounds-checks it away, and the panel stays unselected however many times the
// key is pressed.
func TestInspectorArrowUpFromNothingEntersTheList(t *testing.T) {
	view, insp, _, _, _ := keyInspector(t)
	root := graph.IItem(view.GedScene())

	if insp.SelectedItem() != nil {
		t.Fatalf("a freshly bound panel already highlights %q; this test needs nothing selected", itemLabel(insp.SelectedItem()))
	}

	insp.OnKeyDown(gui.KeyUp, false)
	if got := insp.SelectedItem(); got != root {
		t.Fatalf("KeyUp on a panel with nothing highlighted selected %q, want the first row", itemLabel(got))
	}
	insp.OnKeyDown(gui.KeyUp, false)
	if got := insp.SelectedItem(); got != root {
		t.Errorf("a second KeyUp at the first row selected %q, want it to stay on the first row", itemLabel(got))
	}
}

// TestInspectorDownAtTheLastRowSelectsItAgain: at the end of the list the walk
// clamps onto the last row and selects it — the press is answered, not dropped.
// SigSelect is the observable: it is how the host (property sheet, code panel)
// learns which row the tree is on, and the highlight index reads the same
// either way.
//
// Catches: the upper clamp off by one (idx > len(items) instead of
// len(items)-1). It hands selectRow an index one past the end, where its own
// bounds check silently swallows the press: no canvas re-select, no report, and
// the next off-by-one in the same expression has nothing standing in its way.
func TestInspectorDownAtTheLastRowSelectsItAgain(t *testing.T) {
	_, insp, _, _, cancel := keyInspector(t)

	for i := 0; i < 4; i++ {
		insp.OnKeyDown(gui.KeyDown, false) // root -> leftPane -> btnSave -> btnCancel
	}
	if got := insp.SelectedItem(); got != graph.IItem(cancel) {
		t.Fatalf("the walk stopped on %q, want the last row btnCancel", itemLabel(got))
	}

	var seen []graph.IItem
	insp.SigSelect(func(it graph.IItem) { seen = append(seen, it) })
	insp.OnKeyDown(gui.KeyDown, false)

	if len(seen) != 1 || seen[0] != graph.IItem(cancel) {
		t.Fatalf("KeyDown at the last row reported %v, want it to answer with [btnCancel]", itemLabels(seen))
	}
}
