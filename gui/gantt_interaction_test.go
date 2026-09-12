package gui

import (
	"math"
	"reflect"
	"testing"
)

func chartBarPoint(c *GanttChart, id, kind string) (float64, float64) {
	i := c.rowIndex[id]
	b := c.taskRect(c.rows[i].Task, i, c.geometry())
	x, y := b.X+b.Width/2, b.Y+b.Height/2
	switch kind {
	case "start":
		x = b.X + 2
	case "end":
		x = b.Right() - 2
	case "progress":
		y = b.Bottom() - 2
	}
	return x, y
}
func TestGanttMoveParentUsesPreviewAndCommitsSingleUndo(t *testing.T) {
	c := chartFixture(t)
	before := c.Tasks()
	var changes, commits int
	c.SigTasksChanged(func() { changes++ })
	c.SetEditCommittedCallback(func(a, b []GanttTask) {
		commits++
		if !reflect.DeepEqual(a, before) || reflect.DeepEqual(a, b) {
			t.Error("bad gesture snapshots")
		}
		a[0].Name = "must not alias"
		b[0].Name = "must not alias"
	})
	x, y := chartBarPoint(c, "project", "move")
	c.OnLeftDown(x, y)
	if !c.HasCapture() || c.drag == nil || c.drag.kind != "move" {
		t.Fatalf("no task capture: %+v", c.drag)
	}
	for _, days := range []float64{1, 2, 3} {
		c.OnMouseMove(x+days*c.DayWidth(), y)
	}
	if changes != 0 || commits != 0 || !reflect.DeepEqual(c.Tasks(), before) || c.CanUndo() {
		t.Fatal("preview leaked into model/history/host")
	}
	c.OnLeftUp(x+3*c.DayWidth(), y)
	if changes != 1 || commits != 1 || c.HasCapture() || !c.CanUndo() {
		t.Fatalf("commit counts changed=%d edits=%d capture=%v undo=%v", changes, commits, c.HasCapture(), c.CanUndo())
	}
	for i, task := range c.Tasks() {
		if GanttDaysBetween(before[i].Start, task.Start) != 3 || GanttDaysBetween(before[i].End, task.End) != 3 {
			t.Fatalf("descendant %s not moved", task.ID)
		}
	}
	if !c.Undo() || c.CanUndo() || !reflect.DeepEqual(c.Tasks(), before) {
		t.Fatal("one gesture not exactly one undo")
	}
	if !c.Redo() || c.Tasks()[0].Name == "must not alias" {
		t.Fatal("snapshot alias/redo failure")
	}
}
func TestGanttResizeBothEdgesCascadeAndProgress(t *testing.T) {
	for _, kind := range []string{"start", "end", "progress"} {
		t.Run(kind, func(t *testing.T) {
			c := chartFixture(t)
			before, _ := c.model.Task("a")
			nextBefore, _ := c.model.Task("b")
			x, y := chartBarPoint(c, "a", kind)
			c.OnLeftDown(x, y)
			if c.drag == nil || c.drag.kind != kind {
				t.Fatalf("hit=%+v want %s", c.drag, kind)
			}
			delta := c.DayWidth()
			if kind == "start" {
				delta = -c.DayWidth()
			}
			if kind == "progress" {
				delta = c.DayWidth()
			}
			c.OnMouseMove(x+delta, y)
			c.OnLeftUp(x+delta, y)
			after, _ := c.model.Task("a")
			switch kind {
			case "start":
				if GanttDaysBetween(before.Start, after.Start) != -1 || !before.End.Equal(after.End) {
					t.Fatal("left edge failed")
				}
			case "end":
				next, _ := c.model.Task("b")
				if GanttDaysBetween(before.End, after.End) != 1 || GanttDaysBetween(nextBefore.Start, next.Start) != 1 {
					t.Fatal("right edge did not cascade FS successor")
				}
			case "progress":
				if after.Progress != .75 {
					t.Fatalf("progress=%v", after.Progress)
				}
			}
			if !c.Undo() || c.CanUndo() {
				t.Fatal("resize/progress should be one undo")
			}
		})
	}
}
func TestGanttCancelPathsNeverChangeModelOrNotify(t *testing.T) {
	for _, kind := range []string{"escape", "hide", "disable", "readonly", "focus", "capture", "resize", "replace"} {
		t.Run(kind, func(t *testing.T) {
			c := chartFixture(t)
			before := c.Tasks()
			changes := 0
			c.SigTasksChanged(func() { changes++ })
			x, y := chartBarPoint(c, "project", "move")
			c.OnLeftDown(x, y)
			c.OnMouseMove(x+c.DayWidth()*2, y)
			switch kind {
			case "escape":
				c.OnKeyDown(KeyEsc, false)
			case "hide":
				c.SetVisible(false)
			case "disable":
				c.SetEnabled(false)
			case "readonly":
				c.SetReadOnly(true)
			case "focus":
				NewButton1("outside", nil).SetFocus()
			case "capture":
				other := NewLabel("other")
				other.PushCapture()
				c.OnMouseMove(x+c.DayWidth()*3, y)
				other.PopCapture()
			case "resize":
				c.SetSize(800, 300)
			case "replace":
				if err := c.SetTasks(before); err != nil {
					t.Fatal(err)
				}
				changes-- // explicit replacement is a normal change callback
			}
			c.OnLeftUp(x+c.DayWidth()*2, y)
			if c.drag != nil || c.HasCapture() || changes != 0 || c.CanUndo() || !reflect.DeepEqual(c.Tasks(), before) {
				t.Fatalf("%s failed cancellation: drag=%v capture=%v changes=%d", kind, c.drag, c.HasCapture(), changes)
			}
		})
	}
}
func TestGanttReadOnlyAndDisabledNeverScheduleEdits(t *testing.T) {
	for _, state := range []string{"readonly", "disabled", "ancestor-disabled", "hidden"} {
		t.Run(state, func(t *testing.T) {
			c := chartFixture(t)
			before := c.Tasks()
			switch state {
			case "readonly":
				c.SetReadOnly(true)
			case "disabled":
				c.SetEnabled(false)
			case "ancestor-disabled":
				p := NewVBox()
				c.SetParent(p)
				p.SetEnabled(false)
			case "hidden":
				c.SetVisible(false)
			}
			x, y := chartBarPoint(c, "a", "move")
			c.OnLeftDown(x, y)
			c.OnMouseMove(x+60, y)
			c.OnLeftUp(x+60, y)
			if c.HasCapture() || c.CanUndo() || !reflect.DeepEqual(c.Tasks(), before) || c.HandleShortcut('Z', ModAction) {
				t.Fatal("inert widget claimed editing")
			}
		})
	}
}
func TestGanttWheelContinuousAnchorZoomAndAxisIsolation(t *testing.T) {
	c := chartFixture(t)
	c.SetSize(760, 170)
	c.SetScrollOffsets(80, 30)
	l := c.geometry()
	x := l.split + 100
	beforeDay := (c.scrollX + x - l.split) / c.dayWidth
	oldWidth := c.dayWidth
	oldY := c.scrollY
	c.wheel(x, .37, true, false)
	if c.dayWidth == oldWidth || math.Abs((c.scrollX+x-l.split)/c.dayWidth-beforeDay) > 1e-9 || c.scrollY != oldY {
		t.Fatal("zoom loses cursor anchor/vertical position")
	}
	oldX := c.scrollX
	c.wheel(x, -.25, false, true)
	if c.scrollX <= oldX || c.scrollY != oldY {
		t.Fatal("shift wheel did not scroll horizontal only")
	}
	oldX = c.scrollX
	c.wheel(x, -.25, false, false)
	if c.scrollY <= oldY || c.scrollX != oldX {
		t.Fatal("wheel did not scroll vertical only")
	}
	c.wheel(x, 1e100, true, false)
	if c.dayWidth != 120 {
		t.Fatal("zoom upper limit")
	}
	c.wheel(x, -1e100, true, false)
	if c.dayWidth != 4 {
		t.Fatal("zoom lower limit")
	}
	c.SetScrollOffsets(math.Inf(1), math.NaN())
	if !ganttFinite(c.scrollX) || !ganttFinite(c.scrollY) {
		t.Fatal("scroll accepted nonfinite")
	}
}
func TestGanttShortcutClaimsOnlyLocalEditCommands(t *testing.T) {
	c := chartFixture(t)
	if !c.HandleShortcut('Z', ModAction) || !c.HandleShortcut('Y', ModAction) || !c.HandleShortcut('Z', ModAction|ModShift) {
		t.Fatal("empty history lets host undo run")
	}
	for _, v := range []struct {
		key  int
		mods uint8
	}{{'Z', 0}, {'S', ModAction}, {'Z', ModAction | ModAlt}} {
		if c.HandleShortcut(v.key, v.mods) {
			t.Fatal("claimed unrelated shortcut")
		}
	}
}
