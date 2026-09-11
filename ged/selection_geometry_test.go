package ged

import (
	"testing"
	"time"

	"github.com/uk0/silk/geom"
	"github.com/uk0/silk/graph"
)

func TestMarqueeKeepsFractionalLayoutAndFittedEdgeGeometry(t *testing.T) {
	view := NewGedView()
	view.SetGridStep(5)
	view.SetSnapToGrid(true)
	scene := view.GedScene()
	var widgets []*FakeWidget
	var before []geom.Rect
	for i, y := range []float64{15, 19.4449, 23.8899, 28.3349} {
		button := sceneWidget(t, scene, "gui.Button", string(rune('a'+i)), 15, y, 25, 4.4449)
		widgets = append(widgets, button)
		before = append(before, button.Bounds1())
	}
	// Exact placement produced by the palette's full-visibility clamp at the
	// form's right/bottom edge; snapping Y=93 to 95 clips it after a selection.
	edge := sceneWidget(t, scene, "gui.Button", "edge", 75, 93, 25, 7)
	widgets = append(widgets, edge)
	before = append(before, edge.Bounds1())
	view.Selection().Add(widgets[0])
	for _, band := range [][4]float64{{5, 5, 60, 60}, {60, 80, 102, 102}} {
		dragOnCanvas(view, band[0], band[1], band[2], band[3])
		for i, button := range widgets {
			if button.Bounds1() != before[i] || button.Parent() != graph.IItem(scene) {
				t.Fatalf("marquee %v rewrote %s: %v -> %v", band, button.WidgetName(), before[i], button.Bounds1())
			}
		}
		if scene.UndoStack().Count() != 0 || !scene.UndoStack().IsClean() {
			t.Fatal("selection gesture created history or dirtied the design")
		}
	}
	if !view.Selection().Contains(edge) {
		t.Fatal("edge band did not exercise real selection")
	}
}

func TestSelectionClickAndSubThresholdJitterDoNotSnapPositions(t *testing.T) {
	for _, jitter := range []float64{0, 0.5, 1} {
		view := NewGedView()
		view.SetGridStep(5)
		view.SetSnapToGrid(true)
		button := sceneWidget(t, view.GedScene(), "gui.Button", "fractional", 15.75, 19.4449, 25, 7)
		before := button.Bounds1()
		lastClickTime = time.Time{}
		x, y := view.MapFromScene(28, 23)
		view.OnLeftDown(x, y)
		if jitter != 0 {
			view.OnMouseMove(x+jitter, y+jitter)
		}
		view.OnLeftUp(x+jitter, y+jitter)
		if button.Bounds1() != before || view.Scene().UndoStack().CanUndo() {
			t.Fatalf("selection click with %g px jitter changed geometry/history: %v -> %v", jitter, before, button.Bounds1())
		}
		if !view.Selection().Contains(button) {
			t.Fatal("selection click stopped selecting the button")
		}
	}
}
