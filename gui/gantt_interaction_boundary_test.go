package gui

import (
	"math"
	"reflect"
	"testing"
)

func TestGanttPointerSelectionCallbackCannotStartStaleGesture(t *testing.T) {
	for _, state := range []string{"replace-empty", "replace-same-id", "collapse", "readonly", "disabled", "hidden", "focus", "selection", "geometry"} {
		t.Run(state, func(t *testing.T) {
			c := chartFixture(t)
			x, y := chartBarPoint(c, "a", "move")
			var expected []GanttTask
			calls, commits := 0, 0
			c.SetEditCommittedCallback(func(_, _ []GanttTask) { commits++ })
			c.SetSelectionChangedCallback(func(id string) {
				calls++
				c.SetSelectionChangedCallback(nil)
				switch state {
				case "replace-empty":
					if err := c.SetTasks(nil); err != nil {
						t.Fatal(err)
					}
				case "replace-same-id":
					task, _ := c.model.Task(id)
					task.ParentID, task.Name = "", "replacement"
					if err := c.SetTasks([]GanttTask{task}); err != nil {
						t.Fatal(err)
					}
				case "collapse":
					if err := c.SetCollapsed("group", true); err != nil {
						t.Fatal(err)
					}
				case "readonly":
					c.SetReadOnly(true)
				case "disabled":
					c.SetEnabled(false)
				case "hidden":
					c.SetVisible(false)
				case "focus":
					NewButton1("outside", nil).SetFocus()
				case "selection":
					c.SetSelectedTaskID("b")
				case "geometry":
					c.SetTableWidth(c.TableWidth() + 100)
				}
				expected = c.Tasks()
				commits = 0 // An explicit host edit is not a pointer gesture.
			})
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("selection callback left stale row access: %v", r)
				}
			}()
			c.OnLeftDown(x, y)
			if c.drag != nil || c.HasCapture() {
				t.Errorf("%s callback still started drag/capture", state)
			}
			c.OnLeftUp(x+60, y)
			if calls != 1 || commits != 0 || !reflect.DeepEqual(expected, c.Tasks()) {
				t.Errorf("stale gesture changed replacement/state: calls=%d commits=%d", calls, commits)
			}
		})
	}
}

func TestGanttInvalidPointerReleaseCancelsPreview(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		c := chartFixture(t)
		before := c.Tasks()
		changes := 0
		c.SigTasksChanged(func() { changes++ })
		x, y := chartBarPoint(c, "project", "move")
		c.OnLeftDown(x, y)
		c.OnMouseMove(x+60, y)
		c.OnLeftUp(value, y)
		if c.HasCapture() || c.drag != nil || changes != 0 || c.CanUndo() || !reflect.DeepEqual(before, c.Tasks()) {
			t.Errorf("nonfinite release %v committed an old preview", value)
		}
		c.SetSelectedTaskID("")
		c.OnLeftDown(value, y)
		if c.SelectedTaskID() != "" || c.HasCapture() {
			t.Errorf("nonfinite press %v selected a task", value)
		}
	}
}

func TestGanttSubDayPointerMotionDoesNotRevalidateWholeModel(t *testing.T) {
	c := chartFixture(t)
	x, y := chartBarPoint(c, "project", "move")
	c.OnLeftDown(x, y)
	c.OnMouseMove(x+c.DayWidth(), y)
	preview := c.drag.preview
	allocs := testing.AllocsPerRun(10, func() { c.OnMouseMove(x+c.DayWidth()+.1, y) })
	if allocs != 0 || c.drag.preview != preview {
		t.Fatalf("unchanged snapped day rebuilt model: allocations=%v", allocs)
	}
}

func TestGanttNoopAndRejectedGesturePreserveRedo(t *testing.T) {
	for _, operation := range []string{"click", "return-to-origin", "rejected-resize"} {
		t.Run(operation, func(t *testing.T) {
			c := chartFixture(t)
			x, y := chartBarPoint(c, "project", "move")
			c.OnLeftDown(x, y)
			c.OnLeftUp(x+30, y)
			redo := c.Tasks()
			c.Undo()
			before := c.Tasks()
			changes, commits := 0, 0
			c.SigTasksChanged(func() { changes++ })
			c.SetEditCommittedCallback(func(_, _ []GanttTask) { commits++ })
			x, y = chartBarPoint(c, "a", "end")
			c.SetCascadeDependencies(false)
			c.OnLeftDown(x, y)
			if operation == "return-to-origin" {
				c.OnMouseMove(x-30, y)
			}
			if operation == "rejected-resize" {
				x += 30
			}
			c.OnLeftUp(x, y)
			if c.HasCapture() || c.CanUndo() || !c.CanRedo() || changes != 0 || commits != 0 || !reflect.DeepEqual(before, c.Tasks()) {
				t.Fatal("no-op/rejected gesture mutated history or notified host")
			}
			if operation == "rejected-resize" && c.LastError() == nil {
				t.Fatal("invalid dependency edit hid the error")
			}
			if !c.Redo() || !reflect.DeepEqual(redo, c.Tasks()) {
				t.Fatal("gesture lost prior redo branch")
			}
		})
	}
}

func TestGanttResizeAndProgressClampRemainOneUndo(t *testing.T) {
	for _, kind := range []string{"start", "end", "progress-zero", "progress-full"} {
		t.Run(kind, func(t *testing.T) {
			c := chartFixture(t)
			before := c.Tasks()
			hit := kind
			if kind == "progress-zero" || kind == "progress-full" {
				hit = "progress"
			}
			x, y := chartBarPoint(c, "a", hit)
			c.OnLeftDown(x, y)
			if c.drag == nil || c.drag.kind != hit {
				t.Fatalf("wrong clamp test hit: %+v", c.drag)
			}
			endX := x + 10000
			if kind == "end" || kind == "progress-zero" {
				endX = x - 10000
			}
			c.OnMouseMove(endX, y)
			c.OnLeftUp(endX, y)
			task, _ := c.model.Task("a")
			if hit == "progress" {
				want := 1.0
				if kind == "progress-zero" {
					want = 0
				}
				if task.Progress != want {
					t.Fatalf("progress=%v want %v", task.Progress, want)
				}
			} else if GanttDaysBetween(task.Start, task.End) != 1 {
				t.Fatal("resize crossed edges or lost minimum duration")
			}
			if c.HasCapture() || !c.Undo() || c.CanUndo() || !reflect.DeepEqual(c.Tasks(), before) {
				t.Fatal("clamped gesture did not restore in one undo")
			}
		})
	}
}

func TestGanttReadOnlyAndDisabledCursorCannotAdvertiseEditing(t *testing.T) {
	c := chartFixture(t)
	x, y := chartBarPoint(c, "a", "end")
	c.OnMouseMove(x, y)
	if c.Cursor() != cursorSizeWE {
		t.Fatal("resize hover has no resize cursor")
	}
	c.SetReadOnly(true)
	if c.Cursor() != DefaultCursor() {
		t.Fatal("read-only chart retained resize cursor")
	}
	c.SetReadOnly(false)
	p := NewVBox()
	c.SetParent(p)
	p.SetEnabled(false)
	if c.Cursor() != DefaultCursor() {
		t.Fatal("disabled ancestor retained resize cursor")
	}
}
