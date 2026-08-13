package ged

import (
	"testing"

	"github.com/uk0/silk/graph"
	"github.com/uk0/silk/gui"
)

// dropCtx is a minimal gui.IDndContext carrying a single text/plain payload,
// so the canvas drop path can be driven without a live drag loop. noFormat
// makes it carry a payload the canvas cannot read — a file list, a dragged
// selection from another view — which is the refusal branch of OnDragMove.
type dropCtx struct {
	pa       gui.DndAction
	act      gui.DndAction
	payload  interface{}
	noFormat bool
}

func (this *dropCtx) PosibleActions() gui.DndAction { return this.pa }

func (this *dropCtx) Action() gui.DndAction { return this.act }

func (this *dropCtx) SetAction(act gui.DndAction) {
	if this.pa&act == 0 {
		this.act = gui.DndIgnore
		return
	}
	this.act = act
}

func (this *dropCtx) From() interface{} { return nil }

func (this *dropCtx) Formats() []string {
	if this.noFormat {
		return nil
	}
	return []string{"text/plain"}
}

func (this *dropCtx) HasFormat(format string) bool {
	return !this.noFormat && format == "text/plain"
}

func (this *dropCtx) Data(format string) interface{} {
	if format == "text/plain" {
		return this.payload
	}
	return nil
}

// TestGedViewOnDropClaimsTheDrop pins the action the canvas leaves behind after
// it has consumed a palette drop. Both backends now offer the drop to the
// widget under the cursor and then to its ancestors, stopping at the first one
// that sets an action; a canvas that creates the widget but leaves the action
// at DndIgnore is treated as having refused, so the drop is offered again to
// the enclosing Dock and Frame and DoDragDrop reports DndIgnore to the palette.
func TestGedViewOnDropClaimsTheDrop(t *testing.T) {
	view := NewGedView()
	scene := view.GedScene()

	ctx := &dropCtx{pa: gui.DndCopy, payload: "gui.Button"}
	// The walk resets the action before offering the drop to each target.
	ctx.SetAction(gui.DndIgnore)
	view.OnDrop(20, 20, ctx)

	if got := len(scene.Children()); got != 1 {
		t.Fatalf("scene children after the drop = %d, want 1", got)
	}
	if ctx.Action() != gui.DndCopy {
		t.Errorf("action after a drop that created a widget = %v, want DndCopy", ctx.Action())
	}
}

// TestGedViewOnDropRefusesUnknownPayload keeps the refusal path intact: a drop
// whose payload is not a factory name must leave DndIgnore so the walk carries
// on to the ancestors instead of the canvas swallowing it.
func TestGedViewOnDropRefusesUnknownPayload(t *testing.T) {
	view := NewGedView()
	scene := view.GedScene()

	// Not a string: the canvas cannot read a factory name out of it, which is
	// the refusal that does not reach the modal error box.
	ctx := &dropCtx{pa: gui.DndCopy, payload: 42}
	ctx.SetAction(gui.DndIgnore)
	view.OnDrop(20, 20, ctx)

	if got := len(scene.Children()); got != 0 {
		t.Fatalf("scene children after a refused drop = %d, want 0", got)
	}
	if ctx.Action() != gui.DndIgnore {
		t.Errorf("action after a refused drop = %v, want DndIgnore", ctx.Action())
	}
}

// TestIsContainerItem verifies the drop-into-container predicate: every
// layout/AddWidget/window container reports true, leaf widgets report false,
// and a non-FakeWidget (the scene root) reports false. This is the headless
// core of the drag-drop nesting decision in OnDrop.
func TestIsContainerItem(t *testing.T) {
	containers := []string{
		"gui.VBox", "gui.HBox", "gui.GridLayout", "gui.FormLayout",
		"gui.Card", "gui.GroupBox", "gui.Accordion", "gui.StackedWidget",
		"gui.TabWidget", "gui.Splitter", "gui.ScrollArea", "gui.Form", "gui.Dialog",
	}
	for _, name := range containers {
		fw, err := NewFakeWidgetFromFactory(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if !isContainerItem(fw) {
			t.Errorf("isContainerItem(%s) = false, want true", name)
		}
	}

	leaves := []string{"gui.Button", "gui.Label", "gui.Edit", "gui.CheckBox"}
	for _, name := range leaves {
		fw, err := NewFakeWidgetFromFactory(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if isContainerItem(fw) {
			t.Errorf("isContainerItem(%s) = true, want false", name)
		}
	}

	// The scene root is not a FakeWidget → never a container.
	if isContainerItem(NewGedScene()) {
		t.Error("isContainerItem(scene) = true, want false")
	}
}

// TestContainerUnderPoint builds a scene with a VBox holding a Button and
// asserts the container hit-test finds the VBox both when the drop lands on
// the nested Button (walk up from the leaf) and on the container's own body,
// and returns nil over empty canvas (so OnDrop falls back to the scene root).
func TestContainerUnderPoint(t *testing.T) {
	scene := NewGedScene()
	scene.SetSize(200, 150)

	vbox, err := NewFakeWidgetFromFactory("gui.VBox")
	if err != nil {
		t.Fatalf("create VBox: %v", err)
	}
	vbox.SetBounds(10, 10, 80, 60)
	vbox.SetParent(scene)

	btn, _ := NewFakeWidgetFromFactory("gui.Button")
	btn.SetBounds(20, 20, 25, 7)
	btn.SetParent(vbox)

	// Drop over the nested Button → walk up to the containing VBox.
	if got := nearestContainerAncestor(scene.FindItemAt(25, 24, nil)); got != graph.IItem(vbox) {
		t.Errorf("container under nested button = %v, want vbox", got)
	}
	// Drop over the VBox body (not on the Button) → the VBox itself.
	if got := nearestContainerAncestor(scene.FindItemAt(12, 12, nil)); got != graph.IItem(vbox) {
		t.Errorf("container under vbox body = %v, want vbox", got)
	}
	// Drop over empty canvas → nil (fall back to scene root).
	if got := nearestContainerAncestor(scene.FindItemAt(150, 130, nil)); got != nil {
		t.Errorf("container under empty canvas = %v, want nil", got)
	}
}

// newDropFixture builds the scene the palette-drag highlight tests drive: a
// VBox holding one Button, and a second Button loose on the canvas. The nested
// Button is what makes the walk-up to its container observable; the loose one
// is what separates "a widget is under the cursor" from "a container is".
func newDropFixture(t *testing.T) (view *GedView, scene *GedScene, vbox, nested, free *FakeWidget) {
	t.Helper()
	view = NewGedView()
	scene = view.GedScene()
	scene.SetSize(200, 150)

	vbox, err := NewFakeWidgetFromFactory("gui.VBox")
	if err != nil {
		t.Fatalf("create VBox: %v", err)
	}
	vbox.SetBounds(60, 10, 80, 60)
	vbox.SetParent(scene)

	nested, err = NewFakeWidgetFromFactory("gui.Button")
	if err != nil {
		t.Fatalf("create nested Button: %v", err)
	}
	nested.SetBounds(70, 20, 25, 7)
	nested.SetParent(vbox)

	free, err = NewFakeWidgetFromFactory("gui.Button")
	if err != nil {
		t.Fatalf("create free Button: %v", err)
	}
	free.SetBounds(10, 10, 25, 7)
	free.SetParent(scene)
	return
}

// dragMoveOverScene replays one OnDragMove at the view coordinates of a scene
// point. The action is reset first, exactly as dndDragOver resets it before
// offering the drag to each target, so a stale accept cannot be mistaken for a
// fresh one.
func dragMoveOverScene(view *GedView, ctx *dropCtx, sx, sy float64) {
	vx, vy := view.MapFromScene(sx, sy)
	ctx.SetAction(gui.DndIgnore)
	view.OnDragMove(vx, vy, ctx)
}

// collectItems appends every descendant of root, so a test can find the item a
// drop created without looking it up under the parent it is about to assert.
func collectItems(root graph.IItem, out []graph.IItem) []graph.IItem {
	for _, child := range root.Children() {
		out = append(out, child)
		out = collectItems(child, out)
	}
	return out
}

// itemsAddedTo returns the items present in the tree now but not in before.
func itemsAddedTo(root graph.IItem, before []graph.IItem) []graph.IItem {
	old := make(map[graph.IItem]bool, len(before))
	for _, it := range before {
		old[it] = true
	}
	var added []graph.IItem
	for _, it := range collectItems(root, nil) {
		if !old[it] {
			added = append(added, it)
		}
	}
	return added
}

// TestGedViewDragMoveHighlightsDropContainer pins the feedback a palette drag
// gets while it is still in the air: dropContainer names the very item that
// would receive the widget, by identity — not merely "a container of the right
// kind" — and is nil wherever the drop would fall back to the form root.
func TestGedViewDragMoveHighlightsDropContainer(t *testing.T) {
	view, _, vbox, _, _ := newDropFixture(t)
	ctx := &dropCtx{pa: gui.DndCopy, payload: "gui.Button"}

	// Over the VBox's own body.
	dragMoveOverScene(view, ctx, 100, 40)
	if view.dropContainer != graph.IItem(vbox) {
		t.Fatalf("dropContainer over the VBox body = %v, want the VBox itself", view.dropContainer)
	}
	if ctx.Action() != gui.DndCopy {
		t.Errorf("action over the canvas = %v, want DndCopy", ctx.Action())
	}

	// Over the Button inside it: a leaf takes no children, so the highlight
	// walks up to the container that would actually own the drop.
	dragMoveOverScene(view, ctx, 75, 23)
	if view.dropContainer != graph.IItem(vbox) {
		t.Errorf("dropContainer over the nested button = %v, want the VBox around it", view.dropContainer)
	}

	// Over the loose Button: its only ancestor is the scene root, so the drop
	// lands at the form root and there is nothing to outline. Reached from the
	// VBox, this also proves the previous highlight is cleared, not kept.
	dragMoveOverScene(view, ctx, 22.5, 13.5)
	if view.dropContainer != nil {
		t.Errorf("dropContainer over a free button = %v, want nil — a Button is not a container", view.dropContainer)
	}

	// Over bare canvas, re-armed over the VBox first so a highlight left
	// standing from the previous position would show up here too.
	dragMoveOverScene(view, ctx, 100, 40)
	dragMoveOverScene(view, ctx, 170, 120)
	if view.dropContainer != nil {
		t.Errorf("dropContainer over empty canvas = %v, want nil", view.dropContainer)
	}
}

// TestGedViewDragLeaveClearsDropContainer: dndDragOver hands the drag to an
// ancestor by calling OnDragLeave on the canvas and never calling OnDragMove
// again, so the leave is the only chance to take the outline down.
func TestGedViewDragLeaveClearsDropContainer(t *testing.T) {
	view, _, vbox, _, _ := newDropFixture(t)
	ctx := &dropCtx{pa: gui.DndCopy, payload: "gui.Button"}

	dragMoveOverScene(view, ctx, 100, 40)
	if view.dropContainer != graph.IItem(vbox) {
		t.Fatalf("dropContainer before the leave = %v, want the VBox", view.dropContainer)
	}

	view.OnDragLeave()
	if view.dropContainer != nil {
		t.Errorf("dropContainer after OnDragLeave = %v, want nil — the outline outlived the drag", view.dropContainer)
	}
}

// TestGedViewDragMoveClearsHighlightForRefusedPayload: a drag the canvas cannot
// read is passed on to the ancestors, and must not leave the canvas advertising
// a parent for a widget that will never be created.
func TestGedViewDragMoveClearsHighlightForRefusedPayload(t *testing.T) {
	view, _, vbox, _, _ := newDropFixture(t)

	ctx := &dropCtx{pa: gui.DndCopy, payload: "gui.Button"}
	dragMoveOverScene(view, ctx, 100, 40)
	if view.dropContainer != graph.IItem(vbox) {
		t.Fatalf("dropContainer before the refused drag = %v, want the VBox", view.dropContainer)
	}

	// Same spot over the same container, a payload with no text/plain.
	refused := &dropCtx{pa: gui.DndCopy, payload: "gui.Button", noFormat: true}
	refused.SetAction(gui.DndCopy) // an accept left over from another target
	vx, vy := view.MapFromScene(100, 40)
	view.OnDragMove(vx, vy, refused)

	if view.dropContainer != nil {
		t.Errorf("dropContainer after a refused drag = %v, want nil — the canvas keeps highlighting a drop it refuses", view.dropContainer)
	}
	if refused.Action() != gui.DndIgnore {
		t.Errorf("action for a payload the canvas cannot read = %v, want DndIgnore", refused.Action())
	}
}

// TestGedViewDropLandsInTheHighlightedContainer is the agreement between the
// promise and the result: the container OnDragMove outlined is the container
// OnDrop parents the new widget into, and the outline is gone once the drag is.
//
// The VBox is selected first, which is where the two container resolvers part
// company: nearestDropContainer excludes the moving (selected) items from their
// own drop target — right for a canvas drag, which must not nest a widget into
// itself — so a highlight computed with it would go blank here while OnDrop,
// which asks containerUnder, still nests into the VBox.
func TestGedViewDropLandsInTheHighlightedContainer(t *testing.T) {
	view, scene, vbox, _, _ := newDropFixture(t)
	view.Selection().Add(vbox)

	ctx := &dropCtx{pa: gui.DndCopy, payload: "gui.Button"}
	vx, vy := view.MapFromScene(100, 40)
	ctx.SetAction(gui.DndIgnore)
	view.OnDragMove(vx, vy, ctx)

	want := view.dropContainer
	if want == nil {
		t.Fatalf("no container highlighted over the VBox body — the drag promised the form root")
	}

	before := collectItems(scene, nil)
	ctx.SetAction(gui.DndIgnore)
	view.OnDrop(vx, vy, ctx)

	added := itemsAddedTo(scene, before)
	if len(added) != 1 {
		t.Fatalf("items created by the drop = %d, want 1", len(added))
	}
	if got := added[0].Parent(); got != want {
		t.Errorf("parent of the dropped widget = %v, want %v — the drop ignored the container the drag highlighted", got, want)
	}
	if view.dropContainer != nil {
		t.Errorf("dropContainer after the drop = %v, want nil — the outline stays on the canvas after the drag ended", view.dropContainer)
	}
}

// newEdgeDropFixture builds the one geometry newDropFixture cannot express: a
// VBox whose vertical edges sit at 61 and 141, off the 5mm grid, so a cursor
// within half a pitch of an edge snaps ACROSS it. At (61.5,40) the cursor is
// inside the box while the snapped placement (60,40) is outside; at (141.5,40)
// it is the other way round. Those are the samples that tell "resolved from the
// cursor" apart from "resolved from the placement" — at the box's centre, where
// the other drop tests aim, the two answers coincide and neither is under test.
func newEdgeDropFixture(t *testing.T) (*GedView, *GedScene, *FakeWidget) {
	t.Helper()
	view := NewGedView()
	scene := view.GedScene()
	scene.SetSize(200, 150)
	return view, scene, sceneWidget(t, scene, "gui.VBox", "box", 61, 10, 80, 60)
}

// checkSampleSeparatesCursorFromPlacement is the premise every edge sample
// below rests on: containerUnder answers differently for the cursor and for the
// grid-snapped placement. Without it a test at this point would hold for a
// resolver reading either one, which is exactly how the centre-of-box sample
// let the two decisions pass for one.
func checkSampleSeparatesCursorFromPlacement(t *testing.T, view *GedView, sx, sy float64) (float64, float64) {
	t.Helper()
	gx, gy := view.snapToGrid(sx, sy)
	if a, b := view.containerUnder(sx, sy), view.containerUnder(gx, gy); a == b {
		t.Fatalf("cursor (%v,%v) and snapped placement (%v,%v) both resolve to %v — this sample cannot tell the two apart", sx, sy, gx, gy, a)
	}
	return gx, gy
}

// TestGedViewDragHighlightAndDropFollowTheCursorAtAContainerEdge drops half a
// pitch inside the box's left edge, where the widget's snapped resting place
// lands outside the box. Both halves of the gesture have to answer from the
// cursor: the outline names the container, and OnDrop parents the new widget
// into that same container.
//
// The regression it catches is a highlight resolved at the point the widget is
// PLACED — `this.dropContainer = this.containerUnder(this.snapToGrid(sx, sy))`.
// That reads correctly at a container's centre and goes dark in a band half a
// pitch wide along every edge, while OnDrop still nests the widget into the
// container the outline stopped promising.
func TestGedViewDragHighlightAndDropFollowTheCursorAtAContainerEdge(t *testing.T) {
	view, scene, vbox := newEdgeDropFixture(t)
	const sx, sy = 61.5, 40.0
	gx, gy := checkSampleSeparatesCursorFromPlacement(t, view, sx, sy)

	ctx := &dropCtx{pa: gui.DndCopy, payload: "gui.Button"}
	dragMoveOverScene(view, ctx, sx, sy)
	if view.dropContainer != graph.IItem(vbox) {
		t.Fatalf("dropContainer %.1fmm inside the box's left edge = %v, want the box — the outline is being read off the snapped placement (%v,%v), which is outside it", sx-vbox.X(), view.dropContainer, gx, gy)
	}

	before := collectItems(scene, nil)
	vx, vy := view.MapFromScene(sx, sy)
	ctx.SetAction(gui.DndIgnore)
	view.OnDrop(vx, vy, ctx)

	added := itemsAddedTo(scene, before)
	if len(added) != 1 {
		t.Fatalf("items created by the drop = %d, want 1", len(added))
	}
	if got := added[0].Parent(); got != graph.IItem(vbox) {
		t.Errorf("parent of the widget dropped at (%v,%v) = %v, want the box the drag outlined", sx, sy, got)
	}
}

// TestGedViewDropWithNoHighlightLandsOnTheFormRoot ties the "no outline"
// promise to where the widget actually arrives: nothing outlined means the form
// root owns it. The cursor is half a pitch OUTSIDE the box's right edge, so the
// snapped placement (140,40) is inside the box — the mirror of the sample
// above, and the one that makes the nil answer a claim rather than a default.
//
// The regression it catches is the same misplaced resolver seen from the other
// side: a highlight taken from the snapped placement outlines a box the cursor
// never entered, and a parent taken from there nests the widget into it while
// the canvas showed no outline at all.
func TestGedViewDropWithNoHighlightLandsOnTheFormRoot(t *testing.T) {
	view, scene, vbox := newEdgeDropFixture(t)
	const sx, sy = 141.5, 40.0
	gx, gy := checkSampleSeparatesCursorFromPlacement(t, view, sx, sy)

	ctx := &dropCtx{pa: gui.DndCopy, payload: "gui.Button"}
	dragMoveOverScene(view, ctx, sx, sy)
	if view.dropContainer != nil {
		t.Fatalf("dropContainer %.1fmm outside the box's right edge (%v) = %v, want nil — the canvas outlines a box the cursor is not over, from the snapped placement (%v,%v)", sx-(vbox.X()+vbox.Width()), vbox.X()+vbox.Width(), view.dropContainer, gx, gy)
	}

	before := collectItems(scene, nil)
	vx, vy := view.MapFromScene(sx, sy)
	ctx.SetAction(gui.DndIgnore)
	view.OnDrop(vx, vy, ctx)

	added := itemsAddedTo(scene, before)
	if len(added) != 1 {
		t.Fatalf("items created by the drop = %d, want 1", len(added))
	}
	if got := added[0].Parent(); got != graph.IItem(scene) {
		t.Errorf("parent of the widget dropped with no container outlined = %v, want the form root — the drag promised the root and the drop nested it anyway", got)
	}
}
