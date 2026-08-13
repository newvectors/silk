package main

import (
	"strings"
	"testing"

	"github.com/uk0/silk/ged"
	"github.com/uk0/silk/graph"
	"github.com/uk0/silk/gui"
)

// designerCanvas installs a headless canvas as the frame's current document,
// which is the only way the 布局 menu's handlers can reach one: they all open
// with requireSelection, and that walks gui.DefaultFrame().CurrentDocView().
func designerCanvas(t *testing.T) *ged.GedView {
	t.Helper()
	f := gui.NewFrame()
	gui.SetDefaultFrame(f)
	gv := ged.NewGedView()
	f.SuggestDocDock().AddView(gv)
	if cur := currentGedView(); cur != gv {
		t.Fatalf("the frame hands back %v as the current document, not the canvas", cur)
	}
	return gv
}

// placeWidget drops a widget through the same AddCommand the canvas's drop
// handler uses, then gives it a name and a row of its own so a VBox/HBox has
// something to sort the children by.
func placeWidget(t *testing.T, gv *ged.GedView, factory, name string, y float64) *ged.FakeWidget {
	t.Helper()
	dropWidget(t, gv, factory)
	kids := gv.Scene().Children()
	fw, ok := kids[len(kids)-1].(*ged.FakeWidget)
	if !ok {
		t.Fatalf("dropping %s put a %T on the canvas", factory, kids[len(kids)-1])
	}
	fw.SetWidgetName(name)
	fw.SetBounds(10, y, 30, 8)
	return fw
}

func selectWidgets(gv *ged.GedView, items ...*ged.FakeWidget) {
	sel := gv.Selection()
	sel.Clear()
	for _, it := range items {
		sel.Add(it)
	}
}

// namesOf labels a set of items for a failure message. The identity checks
// below are what the assertions turn on; this is only there to say what turned
// up instead.
func namesOf(items []graph.IItem) string {
	var out []string
	for _, it := range items {
		if fw, ok := it.(*ged.FakeWidget); ok {
			out = append(out, fw.WidgetName())
		} else {
			out = append(out, "?")
		}
	}
	return "[" + strings.Join(out, " ") + "]"
}

// hasItems checks membership by identity, never by count: a lay-out that
// dropped the selected widgets and built fresh ones in their place would hold
// the right number of children and still have thrown the user's work away.
func hasItems(t *testing.T, what string, got []graph.IItem, want ...*ged.FakeWidget) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s holds %d items %s, want %d", what, len(got), namesOf(got), len(want))
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == graph.IItem(w) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s does not hold the original %q; it holds %s", what, w.WidgetName(), namesOf(got))
		}
	}
}

// soleContainer returns the scene's single top-level child, which is what a
// lay-out leaves behind, and pins the container type it was asked for.
func soleContainer(t *testing.T, gv *ged.GedView, factory string) *ged.FakeWidget {
	t.Helper()
	top := gv.Scene().Children()
	if len(top) != 1 {
		t.Fatalf("scene top-level = %d %s, want 1 (the new container)", len(top), namesOf(top))
	}
	box, ok := top[0].(*ged.FakeWidget)
	if !ok {
		t.Fatalf("scene top-level item is a %T, not a widget", top[0])
	}
	if got := box.WidgetFactoryName(); got != factory {
		t.Fatalf("container is a %q, want %q", got, factory)
	}
	return box
}

// TestApplyVBoxLayoutWrapsSelectionUndoably is the menu half of the fix: 应用垂直
// 布局 used to detach every selected widget and push an AddCommand holding only
// the empty container, so the three widgets left the design and Ctrl+Z — which
// had recorded nothing but the container — could not bring them back. The
// handler now goes through the canvas's own undoable lay-out path.
func TestApplyVBoxLayoutWrapsSelectionUndoably(t *testing.T) {
	gv := designerCanvas(t)
	a := placeWidget(t, gv, "gui.Button", "a", 10)
	b := placeWidget(t, gv, "gui.Label", "b", 30)
	c := placeWidget(t, gv, "gui.Edit", "c", 50)
	selectWidgets(gv, a, b, c)

	applyVBoxLayout()

	box := soleContainer(t, gv, "gui.VBox")
	hasItems(t, "the VBox", box.Children(), a, b, c)
	hasItems(t, "the selection", gv.Selection().ItemList(), box)

	gv.Scene().UndoStack().Undo()
	hasItems(t, "the scene after undo", gv.Scene().Children(), a, b, c)
}

// TestApplyHBoxLayoutWrapsSelectionUndoably: 应用水平布局 shared the broken
// implementation, so it needs the same guard.
func TestApplyHBoxLayoutWrapsSelectionUndoably(t *testing.T) {
	gv := designerCanvas(t)
	a := placeWidget(t, gv, "gui.Button", "a", 10)
	b := placeWidget(t, gv, "gui.Label", "b", 30)
	c := placeWidget(t, gv, "gui.Edit", "c", 50)
	selectWidgets(gv, a, b, c)

	applyHBoxLayout()

	box := soleContainer(t, gv, "gui.HBox")
	hasItems(t, "the HBox", box.Children(), a, b, c)
	hasItems(t, "the selection", gv.Selection().ItemList(), box)

	gv.Scene().UndoStack().Undo()
	hasItems(t, "the scene after undo", gv.Scene().Children(), a, b, c)
}

// TestBreakLayoutLiftsChildrenUndoably: 打破布局 used to detach the container
// itself, which takes its children with it — the widgets vanished along with
// the box and nothing was pushed for Ctrl+Z to undo. The children must end up
// at the container's parent, and undo must re-nest them.
func TestBreakLayoutLiftsChildrenUndoably(t *testing.T) {
	gv := designerCanvas(t)
	box := placeWidget(t, gv, "gui.VBox", "box", 10)
	box.SetBounds(10, 10, 40, 40)

	a, err := ged.NewFakeWidgetFromFactory("gui.Button")
	if err != nil {
		t.Fatalf("create gui.Button: %v", err)
	}
	a.SetWidgetName("a")
	a.SetBounds(12, 14, 30, 8)
	a.SetParent(box)
	b, err := ged.NewFakeWidgetFromFactory("gui.Label")
	if err != nil {
		t.Fatalf("create gui.Label: %v", err)
	}
	b.SetWidgetName("b")
	b.SetBounds(12, 26, 30, 6)
	b.SetParent(box)

	selectWidgets(gv, box)

	breakLayout()

	// Both children lifted to the container's parent, the box gone.
	hasItems(t, "the scene after break", gv.Scene().Children(), a, b)

	gv.Scene().UndoStack().Undo()
	hasItems(t, "the scene after undo", gv.Scene().Children(), box)
	hasItems(t, "the restored container", box.Children(), a, b)
}
