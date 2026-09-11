package ged

import (
	"math"
	"testing"

	"github.com/uk0/silk/geom"
	"github.com/uk0/silk/graph"
)

func TestContainerTopLeftResizeCarriesAbsoluteCoordinateChildren(t *testing.T) {
	view := NewGedView()
	view.GedScene().SetSize(200, 150)
	box := sceneWidget(t, view.GedScene(), "gui.VBox", "box", 10, 10, 70, 60)
	button := sceneWidget(t, view.GedScene(), "gui.Button", "button", 20, 20, 25, 7)
	button.SetParent(box)
	inner := sceneWidget(t, view.GedScene(), "gui.GroupBox", "inner", 30, 40, 35, 20)
	inner.SetParent(box)
	leaf := sceneWidget(t, view.GedScene(), "gui.Button", "leaf", 35, 45, 10, 7)
	leaf.SetParent(inner)
	view.Selection().Add(box)
	before := []geom.Rect{box.Bounds1(), button.Bounds1(), inner.Bounds1(), leaf.Bounds1()}
	// This is the user's real resize-handle path, not direct SetBounds.
	dragOnCanvas(view, 10.5, 10.5, 40, 35)
	if math.Abs(box.X()-40) > 1e-9 || math.Abs(box.Y()-35) > 1e-9 {
		t.Fatalf("fixture did not move the origin: %v", box.Bounds1())
	}
	for i, child := range []*FakeWidget{button, inner, leaf} {
		want := before[i+1]
		want.X += 30
		want.Y += 25
		if child.Bounds1() != want {
			t.Fatalf("resizing container origin stranded %s: %v, want %v", child.WidgetName(), child.Bounds1(), want)
		}
	}
	if button.Parent() != graph.IItem(box) || leaf.Parent() != graph.IItem(inner) {
		t.Fatal("resize changed nesting")
	}
	if drawnRect(button).IsEmpty() {
		t.Fatal("resize left the child completely clipped out")
	}
	if view.Scene().UndoStack().Count() != 1 {
		t.Fatal("resize and child carry must be one undo step")
	}
	view.Undo()
	for i, item := range []*FakeWidget{box, button, inner, leaf} {
		if item.Bounds1() != before[i] {
			t.Fatalf("undo did not restore %s: %v, want %v", item.WidgetName(), item.Bounds1(), before[i])
		}
	}
	view.Redo()
	if button.X() != 50 || button.Y() != 45 || leaf.X() != 65 || leaf.Y() != 70 {
		t.Fatal("redo did not carry descendants again")
	}
}

func TestManagedContainerTopLeftResizeUsesLayoutWithoutDoubleCarry(t *testing.T) {
	view, box, button := managedGeometryFixture(t)
	view.GedScene().SetSize(200, 150)
	box.Layout()
	view.Selection().Add(box)
	before := []geom.Rect{box.Bounds1(), button.Bounds1()}
	dragOnCanvas(view, 10.5, 10.5, 40, 35)
	if math.Abs(box.X()-40) > 1e-9 || math.Abs(box.Y()-35) > 1e-9 {
		t.Fatalf("fixture did not resize the layout origin: %v", box.Bounds1())
	}
	after := button.Bounds1()
	box.Layout()
	if button.Bounds1() != after {
		t.Fatalf("resize moved a layout child twice: %v, runtime layout wants %v", after, button.Bounds1())
	}
	if drawnRect(button).IsEmpty() || view.Scene().UndoStack().Count() != 1 {
		t.Fatal("managed resize clipped its child or created extra history")
	}
	view.Undo()
	if box.Bounds1() != before[0] || button.Bounds1() != before[1] {
		t.Fatalf("undo did not restore managed geometry: %v, %v", box.Bounds1(), button.Bounds1())
	}
	view.Redo()
	if button.Bounds1() != after {
		t.Fatal("redo did not restore the same runtime-managed geometry")
	}
}
