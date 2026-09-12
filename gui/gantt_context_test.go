package gui

import (
	"reflect"
	"testing"
)

func ganttContextFixture(t *testing.T) *GanttChart {
	t.Helper()
	c := NewGanttChart()
	err := c.SetTasks([]GanttTask{
		{ID: "project", Name: "Project"},
		ganttTestLeaf("a", "project", 1, 3),
		ganttTestLeaf("b", "project", 4, 6),
		ganttTestLeaf("c", "project", 7, 9),
	})
	if err != nil {
		t.Fatal(err)
	}
	oldFocus, oldHover := focusWidget, mouseHoverWidget
	t.Cleanup(func() {
		c.CancelInteraction()
		dismissOwnedToasts(c)
		focusWidget, mouseHoverWidget = oldFocus, oldHover
	})
	return c
}

func ganttContextAction(t *testing.T, menu *Menu, id string) *Button {
	t.Helper()
	for _, child := range menu.Children() {
		if b, ok := child.(*Button); ok && b.Action().ObjName() == id {
			return b
		}
	}
	t.Fatalf("missing context action %q", id)
	return nil
}

func ganttContextRun(t *testing.T, c *GanttChart, id string) {
	t.Helper()
	m := c.buildGanttContextMenu()
	m.SetParent(c)
	m.setVisible(true) // logical popup only; never attach a native window
	b := ganttContextAction(t, m, id)
	if !b.IsEnabled() {
		t.Fatalf("context action %q unexpectedly disabled", id)
	}
	b.OnKeyDown(KeyEnter, false)
}

func ganttContextPresent(t *testing.T, fn func(*Dialog) DialogResult) {
	t.Helper()
	old := showGanttDialog
	showGanttDialog = func(d *Dialog) DialogResult {
		defer dismissOwnedToasts(d)
		return fn(d)
	}
	t.Cleanup(func() { showGanttDialog = old })
}

func TestGanttContextIndentOutdentNotifyOnceAndUndoExactState(t *testing.T) {
	c := ganttContextFixture(t)
	c.SetSelectedTaskID("b")
	before := c.Tasks()
	changes, commits := 0, 0
	var reportedBefore, reportedAfter []GanttTask
	c.SigTasksChanged(func() { changes++ })
	c.SetEditCommittedCallback(func(before, after []GanttTask) { commits++; reportedBefore, reportedAfter = before, after })
	ganttContextRun(t, c, "gantt.indent")
	task, _ := c.model.Task("b")
	if task.ParentID != "a" || !task.Start.Equal(ganttTestDate(4)) || !task.End.Equal(ganttTestDate(6)) {
		t.Fatal("indent changed dates or failed to move selected subtree")
	}
	if changes != 1 || commits != 1 || !reflect.DeepEqual(before, reportedBefore) || !reflect.DeepEqual(c.Tasks(), reportedAfter) {
		t.Fatal("indent did not produce exactly one complete notification")
	}
	ganttContextRun(t, c, "gantt.undo")
	if !reflect.DeepEqual(before, c.Tasks()) || c.CanUndo() || changes != 2 {
		t.Fatal("menu Undo did not restore the one-edit baseline")
	}
	ganttContextRun(t, c, "gantt.redo")
	if changes != 3 || !reflect.DeepEqual(reportedAfter, c.Tasks()) {
		t.Fatal("menu Redo failed")
	}
	ganttContextRun(t, c, "gantt.outdent")
	task, _ = c.model.Task("b")
	if task.ParentID != "project" || changes != 4 || commits != 2 {
		t.Fatal("outdent did not restore the parent's parent with one notification")
	}
}

func TestGanttContextInventoryReadOnlyAndStaleActions(t *testing.T) {
	c := ganttContextFixture(t)
	c.SetSelectedTaskID("b")
	m := c.buildGanttContextMenu()
	for _, id := range []string{"gantt.add", "gantt.child", "gantt.edit", "gantt.indent", "gantt.outdent", "gantt.collapse", "gantt.undo", "gantt.redo", "gantt.delete"} {
		ganttContextAction(t, m, id)
	}
	before := c.Tasks()
	c.SetReadOnly(true)
	readOnly := c.buildGanttContextMenu()
	for _, id := range []string{"gantt.add", "gantt.child", "gantt.edit", "gantt.indent", "gantt.outdent", "gantt.undo", "gantt.redo", "gantt.delete"} {
		b := ganttContextAction(t, readOnly, id)
		if b.IsEnabled() || b.Action().IsEnabled() {
			t.Errorf("read-only command %s enabled", id)
		}
		b.Action().Trigger(b)
	}
	// A menu that was open before read-only changed must also recheck at run.
	ganttContextAction(t, m, "gantt.indent").Action().Trigger(nil)
	if !reflect.DeepEqual(before, c.Tasks()) || c.CanUndo() {
		t.Fatal("read-only action mutated task data")
	}
	c.SetReadOnly(false)
	for _, mode := range []string{"replacement", "revision"} {
		c.SetSelectedTaskID("b")
		stale := c.buildGanttContextMenu()
		if mode == "replacement" {
			if err := c.SetTasks(c.Tasks()); err != nil {
				t.Fatal(err)
			}
		} else if err := c.model.SetProgress("a", .6); err != nil {
			t.Fatal(err)
		}
		before, revision := c.Tasks(), c.model.Revision()
		ganttContextAction(t, stale, "gantt.indent").Action().Trigger(nil)
		if !reflect.DeepEqual(before, c.Tasks()) || c.model.Revision() != revision {
			t.Fatalf("%s: stale menu mutated newer model", mode)
		}
	}
}

func TestGanttContextCollapseIsViewStateAndNoopHasNoNotification(t *testing.T) {
	c := ganttContextFixture(t)
	c.SetSelectedTaskID("project")
	c.SetReadOnly(true) // Read-only data can still be expanded/collapsed.
	changes, commits := 0, 0
	c.SigTasksChanged(func() { changes++ })
	c.SetEditCommittedCallback(func(_, _ []GanttTask) { commits++ })
	ganttContextRun(t, c, "gantt.collapse")
	if len(c.rows) != 1 || changes != 1 || commits != 1 {
		t.Fatal("collapse did not update only one view-state edit")
	}
	if err := c.SetCollapsed("project", true); err != nil {
		t.Fatal(err)
	}
	if changes != 1 || commits != 1 {
		t.Fatal("no-op collapse emitted a fake edit")
	}
	ganttContextRun(t, c, "gantt.collapse")
	if len(c.rows) != 4 {
		t.Fatal("expand failed")
	}
}

func TestGanttContextDeleteConfirmationCancelAcceptAndStaleModel(t *testing.T) {
	for _, scenario := range []string{"cancel", "accept", "replace-while-confirming", "readonly-while-confirming"} {
		t.Run(scenario, func(t *testing.T) {
			c := ganttContextFixture(t)
			c.SetSelectedTaskID("project")
			before := c.Tasks()
			changes, commits := 0, 0
			c.SigTasksChanged(func() { changes++ })
			c.SetEditCommittedCallback(func(_, _ []GanttTask) { commits++ })
			presented := 0
			ganttContextPresent(t, func(d *Dialog) DialogResult {
				presented++
				if d.Parent() != c {
					t.Fatal("confirmation lost its chart owner")
				}
				switch scenario {
				case "cancel":
					return DialogCancel
				case "replace-while-confirming":
					replacement := c.Tasks()
					replacement[0].Name = "new model"
					if err := c.SetTasks(replacement); err != nil {
						t.Fatal(err)
					}
					before = c.Tasks()
					changes = 0
				case "readonly-while-confirming":
					c.SetReadOnly(true)
				}
				return DialogOK
			})
			ganttContextRun(t, c, "gantt.delete")
			if presented != 1 {
				t.Fatal("delete did not request exactly one confirmation")
			}
			if scenario == "accept" {
				if len(c.Tasks()) != 0 || changes != 1 || commits != 1 || c.SelectedTaskID() != "" {
					t.Fatal("delete did not remove subtree and selection in one edit")
				}
				if !c.Undo() || !reflect.DeepEqual(before, c.Tasks()) || c.CanUndo() {
					t.Fatal("delete undo did not restore complete subtree")
				}
			} else if !reflect.DeepEqual(before, c.Tasks()) || changes != 0 || commits != 0 {
				t.Fatal("cancelled/stale/read-only confirmation deleted task data")
			}
		})
	}
}

func ganttContextForm(t *testing.T, d *Dialog) *FormLayout {
	t.Helper()
	form, ok := d.Content().(*FormLayout)
	if !ok || len(form.rows) != 9 {
		t.Fatalf("unexpected task form %T", d.Content())
	}
	return form
}

func TestGanttContextEditorInclusiveFinishValidationAndDefaultEnter(t *testing.T) {
	c := ganttContextFixture(t)
	c.SetSelectedTaskID("b")
	before := c.Tasks()
	changes, commits := 0, 0
	c.SigTasksChanged(func() { changes++ })
	c.SetEditCommittedCallback(func(_, _ []GanttTask) { commits++ })
	ganttContextPresent(t, func(d *Dialog) DialogResult {
		form := ganttContextForm(t, d)
		start, finish := form.rows[2].widget.(*Edit), form.rows[3].widget.(*Edit)
		if start.Text() != "2026-03-04" || finish.Text() != "2026-03-05" {
			t.Fatalf("exclusive End not converted to inclusive Finish: %q/%q", start.Text(), finish.Text())
		}
		start.SetText("invalid")
		d.OnKeyDown(KeyEnter, false)
		if d.closed || !reflect.DeepEqual(before, c.Tasks()) || changes != 0 {
			t.Fatal("default Enter bypassed date validation or closed invalid form")
		}
		start.SetText("2026-03-04")
		finish.SetText("2026-03-03")
		d.OnKeyDown(KeyEnter, false)
		if d.closed || !reflect.DeepEqual(before, c.Tasks()) {
			t.Fatal("invalid finish closed or partly committed form")
		}
		finish.SetText("2026-03-08")
		form.rows[1].widget.(*Edit).SetText("  edited task  ")
		form.rows[4].widget.(*SpinBox).SetValue(75)
		form.rows[5].widget.(*Edit).SetText("SDK")
		d.OnKeyDown(KeyEnter, false)
		if !d.closed || d.result != DialogOK {
			t.Fatal("valid default Enter did not save and close")
		}
		return d.result
	})
	ganttContextRun(t, c, "gantt.edit")
	task, _ := c.model.Task("b")
	if task.Name != "edited task" || task.Owner != "SDK" || task.Progress != .75 || !task.End.Equal(ganttTestDate(9)) {
		t.Fatalf("edited fields not committed: %+v", task)
	}
	if changes != 1 || commits != 1 {
		t.Fatalf("Save notification count=%d/%d", changes, commits)
	}
	if !c.Undo() || !reflect.DeepEqual(before, c.Tasks()) || c.CanUndo() {
		t.Fatal("one Save was not one undo atom")
	}
}

func TestGanttContextEditorProtectsNewerModelAndSummaryDates(t *testing.T) {
	for _, scenario := range []string{"summary", "stale", "milestone"} {
		t.Run(scenario, func(t *testing.T) {
			c := ganttContextFixture(t)
			id := "b"
			if scenario == "summary" {
				id = "project"
			}
			c.SetSelectedTaskID(id)
			ganttContextPresent(t, func(d *Dialog) DialogResult {
				form := ganttContextForm(t, d)
				if scenario == "summary" {
					for _, i := range []int{2, 3, 4, 8} {
						if form.rows[i].widget.IsEnabled() {
							t.Fatalf("summary geometry/type field %d editable", i)
						}
					}
					form.rows[1].widget.(*Edit).SetText("Renamed summary")
				} else if scenario == "stale" {
					if err := c.model.MoveTask("a", 1, false); err != nil {
						t.Fatal(err)
					}
					before := c.Tasks()
					d.OnKeyDown(KeyEnter, false)
					if d.closed || !reflect.DeepEqual(before, c.Tasks()) {
						t.Fatal("stale task editor overwrote newer model")
					}
					return DialogCancel
				} else {
					form.rows[8].widget.(*CheckBox).SetChecked(true)
				}
				d.OnKeyDown(KeyEnter, false)
				if !d.closed {
					t.Fatal("valid editor stayed open")
				}
				return d.result
			})
			ganttContextRun(t, c, "gantt.edit")
			task, _ := c.model.Task(id)
			if scenario == "summary" && (task.Name != "Renamed summary" || !task.Start.Equal(ganttTestDate(1)) || !task.End.Equal(ganttTestDate(9))) {
				t.Fatal("summary name edit altered derived dates")
			}
			if scenario == "milestone" && (!task.Milestone || !task.Start.Equal(task.End)) {
				t.Fatal("milestone edit retained an inclusive duration")
			}
		})
	}
}
