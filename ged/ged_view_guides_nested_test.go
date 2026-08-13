package ged

import "testing"

// nestedGuideFixture builds the form the nesting cases share: a VBox holding a
// button, a free button outside it, and a second top-level button. The scene is
// 400x300 mm so neither canvas centre line (x=200, y=150) lands near any widget
// and every guide in these tests can be traced back to a widget edge.
//
//	box    (60,10)-(140,70)   left 60,  right 140, centre 100 / centre-y 40
//	nested (70,20)-(95,27)    left 70,  right 95,  centre 82.5   (inside box)
//	free   (10,40)-(35,47)    left 10,  right 35,  centre 22.5
//	top    (120,90)-(145,97)  left 120, right 145, centre 132.5
func nestedGuideFixture(t *testing.T) (view *GedView, box, free *FakeWidget) {
	t.Helper()
	view = NewGedView()
	scene := view.GedScene()
	scene.SetSize(400, 300)

	box = newAlignBox(t, scene, "box", 60, 10, 80, 60)
	nested := addFakeAt(t, scene, "nested", 70, 20, 25, 7)
	nested.SetParent(box)
	free = addFakeAt(t, scene, "free", 10, 40, 25, 7)
	addFakeAt(t, scene, "top", 120, 90, 25, 7)
	return
}

// dragGuides selects one widget and asks for the guides a live drag of (dx, dy)
// would draw — the same selection-driven call OnMouseMove makes.
func dragGuides(view *GedView, item *FakeWidget, dx, dy float64) []alignGuide {
	view.Selection().Clear()
	view.Selection().Add(item)
	view.computeAlignGuides(view.Selection().ItemList(), dx, dy)
	return view.alignGuides
}

// TestAlignGuidesSeeInsideContainer is the headline case: the target collection
// walked scene.Children() only, so every widget inside a container was invisible
// to the snap lines. A left edge sitting exactly on a nested button's left edge
// produced nothing at all, while the same edge against a top-level sibling
// produced three guides.
func TestAlignGuidesSeeInsideContainer(t *testing.T) {
	view, _, free := nestedGuideFixture(t)

	// free.Left is 10; +60 puts it exactly on the nested button's left edge.
	gs := dragGuides(view, free, 60, 0)

	if !hasVGuide(gs, 70) {
		t.Fatalf("no vertical guide at x=70 for an edge sitting exactly on a button inside a VBox; got %+v", gs)
	}
	// The dragged rect matches all three of the nested button's x targets:
	// left 70, centre 82.5 and right 95. Nothing else is within alignTolerance.
	if n := countVertical(gs); n != 3 {
		t.Errorf("%d vertical guides, want 3 (left 70, centre 82.5, right 95 of the nested button): %+v", n, gs)
	}
}

// TestAlignGuidesNestedRespectsTolerance is the negative control for the case
// above: the same fixture with the edge parked outside alignTolerance produces
// no guide at 70, so that assertion cannot be true by construction.
func TestAlignGuidesNestedRespectsTolerance(t *testing.T) {
	view, _, free := nestedGuideFixture(t)

	// Left edge lands at 73 — 3mm off the nested button, twice alignTolerance.
	gs := dragGuides(view, free, 63, 0)

	if hasVGuide(gs, 70) {
		t.Fatalf("vertical guide at x=70 with the edge 3mm away (alignTolerance %g): %+v", alignTolerance, gs)
	}
	if n := countVertical(gs); n != 0 {
		t.Errorf("%d vertical guides for a drag that lines up with no x target: %+v", n, gs)
	}
}

// TestAlignGuidesSkipDraggedContainersOwnChild pins the other half of the rule.
// A dragged container carries its subtree (Selection.GenerateMoveCommand moves
// the whole subtree), so a child keeps its offset and can never be lined up
// against: collecting every descendant would draw a snap line the drag can
// never satisfy.
func TestAlignGuidesSkipDraggedContainersOwnChild(t *testing.T) {
	view, box, _ := nestedGuideFixture(t)

	// box.Left is 60; +10 lands it exactly on its own child's left edge (70).
	gs := dragGuides(view, box, 10, 0)

	if hasVGuide(gs, 70) {
		t.Fatalf("vertical guide at x=70: the dragged VBox lined up against the button it carries: %+v", gs)
	}
	if n := countVertical(gs); n != 0 {
		t.Errorf("%d vertical guides, want 0 — nothing outside the dragged container is within alignTolerance: %+v", n, gs)
	}
}

// nestGrandchild puts a Card inside box with a Button inside the Card, two
// levels below the scene root. The Button's left edge, 75, is a coordinate no
// other widget in the fixture owns — the button inside the VBox is at 70, the
// Card at 64 — so a guide there can only have come from the grandchild.
func nestGrandchild(t *testing.T, view *GedView, box *FakeWidget) {
	t.Helper()
	card, err := NewFakeWidgetFromFactory("gui.Card")
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	card.SetWidgetName("card")
	card.SetBounds(64, 24, 40, 25)
	card.SetParent(box)
	deep := addFakeAt(t, view.GedScene(), "deep", 75, 30, 25, 7)
	deep.SetParent(card)
}

// TestAlignGuidesSeeTwoLevelsDown: the walk goes all the way down, not one
// level. A container inside a container is ordinary — a Card of fields inside
// the form's VBox — and a walk that stops at the container's own children
// leaves everything below it unsnappable again.
func TestAlignGuidesSeeTwoLevelsDown(t *testing.T) {
	view, box, free := nestedGuideFixture(t)
	nestGrandchild(t, view, box)

	// free.Left is 10; +65 puts it exactly on the grandchild's left edge (75).
	gs := dragGuides(view, free, 65, 0)

	if !hasVGuide(gs, 75) {
		t.Fatalf("no vertical guide at x=75 for an edge on a button two containers deep; got %+v", gs)
	}
}

// TestAlignGuidesSkipDraggedContainersGrandchild repeats the exclusion two
// levels down: a Button inside a Card inside the dragged VBox. Skipping only
// the selection's direct children leaves this one collected, and the drag gets
// a snap line for a widget riding along with it.
func TestAlignGuidesSkipDraggedContainersGrandchild(t *testing.T) {
	view, box, _ := nestedGuideFixture(t)
	nestGrandchild(t, view, box)

	// box.Left is 60; +15 lands it exactly on the grandchild's left edge (75).
	gs := dragGuides(view, box, 15, 0)

	if hasVGuide(gs, 75) {
		t.Fatalf("vertical guide at x=75: the dragged VBox lined up against a button two levels inside it: %+v", gs)
	}
	if n := countVertical(gs); n != 0 {
		t.Errorf("%d vertical guides, want 0 — everything below the dragged container is excluded: %+v", n, gs)
	}
}

// TestAlignGuidesTopLevelSiblingUnchanged is the regression guard: the flat case
// the recursive walk replaced must still produce exactly the guides it did.
func TestAlignGuidesTopLevelSiblingUnchanged(t *testing.T) {
	view, _, free := nestedGuideFixture(t)

	// free.Left is 10; +110 puts it exactly on the top-level button's left edge.
	gs := dragGuides(view, free, 110, 0)

	if !hasVGuide(gs, 120) {
		t.Fatalf("no vertical guide at x=120 against a top-level sibling: %+v", gs)
	}
	// left 120, centre 132.5 and right 145 of the top-level button...
	if n := countVertical(gs); n != 3 {
		t.Errorf("%d vertical guides, want 3 (left 120, centre 132.5, right 145): %+v", n, gs)
	}
	// ...plus the one horizontal guide, the dragged top edge on the VBox's
	// centre-y. Four guides in total, the same set as before the walk.
	if !hasHGuide(gs, 40) {
		t.Errorf("no horizontal guide at y=40 (dragged top edge on the VBox centre): %+v", gs)
	}
	if len(gs) != 4 {
		t.Errorf("%d guides, want 4 (3 vertical + 1 horizontal): %+v", len(gs), gs)
	}
}
