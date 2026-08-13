package main

import (
	"strings"
	"testing"

	"github.com/uk0/silk/ged"
	"github.com/uk0/silk/graph"
	"github.com/uk0/silk/gui"
)

// newStatusBarLabels installs fresh permanent indicator labels, standing in for
// createStatusBar which needs a live frame.
func newStatusBarLabels() {
	statusModeLabel = gui.NewLabel("")
	statusWidgetCountLabel = gui.NewLabel("")
	statusZoomLabel = gui.NewLabel("")
	statusInfoLabel = gui.NewLabel("")
}

// dropWidget replays what GedView.OnDrop does once the hit-test has picked a
// parent: a fresh FakeWidget is attached through an AddCommand pushed onto the
// scene's undo stack. Going through the command is the point — it is the path
// drop, paste and their undo/redo all share.
func dropWidget(t *testing.T, gv *ged.GedView, factory string) {
	t.Helper()
	item, err := ged.NewFakeWidgetFromFactory(factory)
	if err != nil {
		t.Fatalf("create %s: %v", factory, err)
	}
	cmd := graph.NewAddCommand()
	cmd.AddItem(item, gv.Scene())
	gv.Scene().PushCommand(cmd)
}

// TestStatusBarWidgetCountFollowsDrops guards the state the designer shipped
// in: three widgets on the canvas and a status bar still reading "0 widgets",
// because nothing refreshed the indicators when the scene gained or lost an
// item — the count only ever showed what was there at startup. Undo and redo
// replay the same AddCommand, so they ride on the same wiring.
func TestStatusBarWidgetCountFollowsDrops(t *testing.T) {
	newStatusBarLabels()
	gv := ged.NewGedView()
	bindStatusBarTo(gv)

	dropWidget(t, gv, "gui.Button")
	dropWidget(t, gv, "gui.Label")
	dropWidget(t, gv, "gui.Edit")
	if got := statusWidgetCountLabel.Text(); got != "3 widgets" {
		t.Errorf("after three drops the count cell reads %q, want %q", got, "3 widgets")
	}

	gv.Scene().UndoStack().Undo()
	if got := statusWidgetCountLabel.Text(); got != "2 widgets" {
		t.Errorf("after undo the count cell reads %q, want %q", got, "2 widgets")
	}

	gv.Scene().UndoStack().Redo()
	if got := statusWidgetCountLabel.Text(); got != "3 widgets" {
		t.Errorf("after redo the count cell reads %q, want %q", got, "3 widgets")
	}
}

// dropWidgetInto is dropWidget's other half: GedView.OnDrop hit-tests for a
// container under the cursor and hands that container to the AddCommand instead
// of the scene root, so a widget dropped on a VBox lands inside it. Same
// command, same PushCommand — drop, paste and their undo/redo all share it.
func dropWidgetInto(t *testing.T, gv *ged.GedView, parent graph.IItem, factory string) *ged.FakeWidget {
	t.Helper()
	item, err := ged.NewFakeWidgetFromFactory(factory)
	if err != nil {
		t.Fatalf("create %s: %v", factory, err)
	}
	cmd := graph.NewAddCommand()
	cmd.AddItem(item, parent)
	gv.Scene().PushCommand(cmd)
	return item
}

// TestStatusBarWidgetCountIncludesNestedWidgets guards the count against the
// user's own tidying: the cell counted the scene's direct children, so dragging
// three buttons into a VBox — what 布局 and drag-to-nest exist to encourage —
// took the design from "4 widgets" down to "1 widgets" while nothing was
// removed. Undo/redo ride the same AddCommand, one button at a time.
func TestStatusBarWidgetCountIncludesNestedWidgets(t *testing.T) {
	newStatusBarLabels()
	gv := ged.NewGedView()
	bindStatusBarTo(gv)

	vbox := dropWidgetInto(t, gv, gv.Scene(), "gui.VBox")
	if got := statusWidgetCountLabel.Text(); got != "1 widgets" {
		t.Fatalf("after the container drop the count cell reads %q, want %q", got, "1 widgets")
	}

	for i := 0; i < 3; i++ {
		dropWidgetInto(t, gv, vbox, "gui.Button")
	}
	if got := statusWidgetCountLabel.Text(); got != "4 widgets" {
		t.Errorf("with three buttons inside the VBox the count cell reads %q, want %q", got, "4 widgets")
	}

	gv.Scene().UndoStack().Undo()
	if got := statusWidgetCountLabel.Text(); got != "3 widgets" {
		t.Errorf("after undoing the third button the count cell reads %q, want %q", got, "3 widgets")
	}

	gv.Scene().UndoStack().Redo()
	if got := statusWidgetCountLabel.Text(); got != "4 widgets" {
		t.Errorf("after redoing the third button the count cell reads %q, want %q", got, "4 widgets")
	}
}

// TestStatusBarWidgetCountReachesEveryDepth is the same guard one level deeper.
// A container inside a container is ordinary — a Card of fields inside the
// form's VBox — and a count that descends only into the scene's own children
// reports 2 here while three widgets sit on the canvas.
func TestStatusBarWidgetCountReachesEveryDepth(t *testing.T) {
	newStatusBarLabels()
	gv := ged.NewGedView()
	bindStatusBarTo(gv)

	vbox := dropWidgetInto(t, gv, gv.Scene(), "gui.VBox")
	card := dropWidgetInto(t, gv, vbox, "gui.Card")
	dropWidgetInto(t, gv, card, "gui.Button")

	if got := statusWidgetCountLabel.Text(); got != "3 widgets" {
		t.Errorf("VBox → Card → Button reads %q, want %q", got, "3 widgets")
	}
}

// TestStatusBarWidgetCountReachesEverySibling is the breadth half of the same
// guard. Every fixture above is a single chain, so they pin how deep the walk
// goes and never that it comes back up: a walk that descends into the first
// container it meets and returns from the loop with that branch's total gets
// all of them right. Here a header row sits beside a body column — the most
// ordinary form layout there is — and such a walk reports 3 while nine widgets
// sit on the canvas. Both levels branch, so a walk that gets the two roots
// right and still stops at the first nested container is caught as well.
func TestStatusBarWidgetCountReachesEverySibling(t *testing.T) {
	newStatusBarLabels()
	gv := ged.NewGedView()
	bindStatusBarTo(gv)

	header := dropWidgetInto(t, gv, gv.Scene(), "gui.HBox")
	dropWidgetInto(t, gv, header, "gui.Label")
	dropWidgetInto(t, gv, header, "gui.Button")
	if got := statusWidgetCountLabel.Text(); got != "3 widgets" {
		t.Fatalf("the header row alone reads %q, want %q", got, "3 widgets")
	}

	body := dropWidgetInto(t, gv, gv.Scene(), "gui.VBox")
	card := dropWidgetInto(t, gv, body, "gui.Card")
	dropWidgetInto(t, gv, card, "gui.Edit")
	dropWidgetInto(t, gv, card, "gui.Edit")
	group := dropWidgetInto(t, gv, body, "gui.GroupBox")
	dropWidgetInto(t, gv, group, "gui.CheckBox")

	// HBox(Label, Button) beside VBox(Card(Edit, Edit), GroupBox(CheckBox)).
	if got := statusWidgetCountLabel.Text(); got != "9 widgets" {
		t.Errorf("a header row beside a body column reads %q, want %q", got, "9 widgets")
	}
}

// TestStatusBarWidgetCountWithoutWidgets pins the two zero cases the recursive
// walk must keep: an empty scene counts nothing (not the scene item itself),
// and a view-less status bar still resets to zero rather than reaching through
// a nil canvas for its children.
func TestStatusBarWidgetCountWithoutWidgets(t *testing.T) {
	newStatusBarLabels()
	gv := ged.NewGedView()
	bindStatusBarTo(gv)

	updateStatusBarInfoFor(gv)
	if got := statusWidgetCountLabel.Text(); got != "0 widgets" {
		t.Errorf("empty scene: count cell = %q, want %q", got, "0 widgets")
	}

	dropWidgetInto(t, gv, gv.Scene(), "gui.Button")
	updateStatusBarInfoFor(nil)
	if got := statusWidgetCountLabel.Text(); got != "0 widgets" {
		t.Errorf("no canvas: count cell = %q, want %q", got, "0 widgets")
	}
}

// TestStatusBarCellsBesideTheCount keeps the other three cells honest while the
// count learns to recurse: they are written by the same function, and a nested
// widget is exactly the case where the selection cell has to report the item's
// own position rather than the container's.
func TestStatusBarCellsBesideTheCount(t *testing.T) {
	newStatusBarLabels()
	gv := ged.NewGedView()
	bindStatusBarTo(gv)

	vbox := dropWidgetInto(t, gv, gv.Scene(), "gui.VBox")
	btn := dropWidgetInto(t, gv, vbox, "gui.Button")
	btn.SetPos(40, 25)

	gv.Selection().Add(btn)
	updateStatusBarInfoFor(gv)

	if got := statusInfoLabel.Text(); got != "Button (40,25)" {
		t.Errorf("selection cell = %q, want %q", got, "Button (40,25)")
	}
	if got := statusZoomLabel.Text(); got != "100%" {
		t.Errorf("zoom cell = %q, want %q", got, "100%")
	}
	if got := statusWidgetCountLabel.Text(); got != "2 widgets" {
		t.Errorf("count cell = %q, want %q", got, "2 widgets")
	}
}

// TestStatusBarZoomFollowsTheView covers the other stale cell: the zoom
// percentage was written only where updateStatusBarInfo happened to be called,
// so Ctrl+wheel or a fit-to-view moved the canvas and left "100%" behind.
func TestStatusBarZoomFollowsTheView(t *testing.T) {
	newStatusBarLabels()
	gv := ged.NewGedView()
	bindStatusBarTo(gv)

	gv.SetZoomFactor(2)
	if got := statusZoomLabel.Text(); got != "200%" {
		t.Errorf("zoom cell = %q, want %q", got, "200%")
	}
}

// TestModeShortcutsAreRegisteredGlobally pins the fix for Ctrl+1 / Ctrl+2 doing
// nothing while the code editor had focus: GedView.OnKeyDown is reached only
// after focus routing, so a focused CodeEditor swallowed the keys before the
// canvas could see them. They have to go through the window-level registry,
// which runs first. There is no live window in a test process, so this reads
// the source — crude, but it fails the moment the registration is dropped.
func TestModeShortcutsAreRegisteredGlobally(t *testing.T) {
	body := funcBody(t, "design.go", "func main() {")
	for _, want := range []string{
		"gui.RegisterShortcut(gui.ModAction, '1'",
		"gui.RegisterShortcut(gui.ModAction, '2'",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("main() no longer contains %q; the mode shortcut only fires when the canvas has focus", want)
		}
	}
}
