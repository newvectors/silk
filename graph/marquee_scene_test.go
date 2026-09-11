package graph

import "testing"

// Reproduce the designer's selectable, resizable form surface without
// importing ged (which would make a graph -> ged -> graph import cycle).
func marqueeSceneFixture() (*GraphView, *SceneItem, *ArrowTool) {
	view := NewView()
	scene := new(SceneItem)
	scene.Init(scene)
	scene.SetBounds(0, 0, 100, 100)
	scene.SetSelectable(true)
	scene.SetLockPos(false)
	scene.SetLockSize(false)
	view.SetScene(scene)
	tool := NewArrowTool().(*ArrowTool)
	view.AddTool(tool)
	view.SetActiveTool(tool)
	return view, scene, tool
}

func TestMarqueeStartsInsideSelectableScene(t *testing.T) {
	view, scene, tool := marqueeSceneFixture()
	first, second := NewRectItem(), NewRectItem()
	first.SetBounds(20, 20, 10, 10)
	first.SetParent(scene)
	second.SetBounds(45, 45, 10, 10)
	second.SetParent(scene)
	before := scene.Bounds1()
	tool.OnLeftDown(5, 5)
	tool.OnLeftDragStart(60, 60)
	if _, ok := tool.ActivePart().(*MarqueePart); !ok {
		t.Fatalf("inside form activated %T, want marquee", tool.ActivePart())
	}
	tool.OnMouseMove(60, 60)
	tool.OnLeftUp(60, 60)
	if view.Selection().Count() != 2 || !view.Selection().Contains(first) || !view.Selection().Contains(second) {
		t.Fatalf("selected %v, want both children", view.Selection().ItemList())
	}
	if view.Selection().Contains(scene) {
		t.Fatal("marquee selected document root")
	}
	if scene.Bounds1() != before || scene.UndoStack().CanUndo() {
		t.Fatal("marquee moved the form or pushed history")
	}
}

func TestSceneClickAndResizeRemainAvailable(t *testing.T) {
	view, scene, tool := marqueeSceneFixture()
	tool.OnLeftDown(50, 50)
	tool.OnLeftClick(50, 50)
	tool.OnLeftUp(50, 50)
	if !view.Selection().Contains(scene) {
		t.Fatal("form click no longer opens property selection")
	}
	tool.OnLeftDown(100, 100)
	if _, ok := tool.ActivePart().(*DecorPart); !ok {
		t.Fatalf("resize handle activated %T, want decor", tool.ActivePart())
	}
	tool.OnLeftDragStart(110, 120)
	tool.OnMouseMove(110, 120)
	tool.OnLeftUp(110, 120)
	if scene.Width() != 110 || scene.Height() != 120 {
		t.Fatalf("form size=(%g,%g), want (110,120)", scene.Width(), scene.Height())
	}
}

func TestSceneChildStillDragsUndoably(t *testing.T) {
	view, scene, tool := marqueeSceneFixture()
	child := NewRectItem()
	child.SetBounds(20, 20, 10, 10)
	child.SetParent(scene)
	tool.OnLeftDown(25, 25)
	tool.OnLeftDragStart(35, 40)
	if _, ok := tool.ActivePart().(*MovePart); !ok {
		t.Fatalf("child drag activated %T", tool.ActivePart())
	}
	tool.OnMouseMove(35, 40)
	tool.OnLeftUp(35, 40)
	if !view.Selection().Contains(child) || child.X() != 30 || child.Y() != 35 {
		t.Fatalf("child not moved to (%g,%g)", child.X(), child.Y())
	}
	scene.UndoStack().Undo()
	if child.X() != 20 || child.Y() != 20 {
		t.Fatal("child move no longer undoable")
	}
}
