package gui

import (
	"math"
	"reflect"
	"testing"
)

func TestGanttContextAddRootAndChildAreSingleUndoEdits(t *testing.T) {
	for _, command := range []string{"gantt.add", "gantt.child"} {
		t.Run(command, func(t *testing.T) {
			c := ganttContextFixture(t)
			c.SetSelectedTaskID("project")
			before := c.Tasks()
			changes, commits := 0, 0
			c.SigTasksChanged(func() { changes++ })
			c.SetEditCommittedCallback(func(a, b []GanttTask) {
				commits++
				if !reflect.DeepEqual(a, before) || len(b) != len(before)+1 {
					t.Error("add reported wrong edit snapshots")
				}
			})
			ganttContextPresent(t, func(d *Dialog) DialogResult {
				form := ganttContextForm(t, d)
				form.rows[1].widget.(*Edit).SetText("  中文任务  ")
				form.rows[2].widget.(*Edit).SetText("2026-03-10")
				form.rows[3].widget.(*Edit).SetText("2026-03-12")
				form.rows[6].widget.(*Edit).SetText(" a, b ")
				d.OnKeyDown(KeyEnter, false)
				if !d.closed || d.result != DialogOK {
					t.Fatal("new task Save did not close valid form")
				}
				return d.result
			})
			ganttContextRun(t, c, command)
			task, ok := c.model.Task(c.SelectedTaskID())
			if !ok || task.ID != "task1" || task.Name != "中文任务" || !task.End.Equal(ganttTestDate(13)) || !reflect.DeepEqual(task.Dependencies, []string{"a", "b"}) {
				t.Fatalf("new task fields/selection mismatch: %+v", task)
			}
			parent := ""
			if command == "gantt.child" {
				parent = "project"
			}
			if task.ParentID != parent || changes != 1 || commits != 1 {
				t.Fatalf("new task parent/notifications: parent=%q changes=%d commits=%d", task.ParentID, changes, commits)
			}
			if !c.Undo() || c.CanUndo() || !reflect.DeepEqual(c.Tasks(), before) {
				t.Fatal("new task was not one undo atom")
			}
		})
	}
}

type ganttFocusLeaveWidget struct {
	Widget
	onLeave func()
}

func (w *ganttFocusLeaveWidget) OnFocusChanged(next, old IWidget) {
	if next != w && w.onLeave != nil {
		w.onLeave()
	}
}

func TestGanttRightClickOwnsFocusAndRejectsReentrantState(t *testing.T) {
	for _, state := range []string{"normal", "focus-disabled", "selection-empty", "selection-collapsed", "selection-hidden", "selection-readonly", "selection-other-focus", "selection-geometry", "invalid-pointer"} {
		t.Run(state, func(t *testing.T) {
			c := chartFixture(t)
			otherWindow := NewFrame()
			otherWindow.win = &Window{widget: otherWindow}
			other := new(ganttFocusLeaveWidget)
			other.Init(other)
			other.SetParent(otherWindow)
			other.SetFocus()
			if other.OwnerWindow() == c.OwnerWindow() {
				t.Fatal("fixture did not start in a different logical window")
			}
			if state == "focus-disabled" {
				other.onLeave = func() { c.SetEnabled(false) }
			}
			c.SetSelectionChangedCallback(func(id string) {
				c.SetSelectionChangedCallback(nil)
				switch state {
				case "selection-empty":
					if err := c.SetTasks(nil); err != nil {
						t.Fatal(err)
					}
				case "selection-collapsed":
					if err := c.SetCollapsed("group", true); err != nil {
						t.Fatal(err)
					}
				case "selection-hidden":
					c.SetVisible(false)
				case "selection-readonly":
					c.SetReadOnly(true)
				case "selection-other-focus":
					other.SetFocus()
				case "selection-geometry":
					c.SetTableWidth(c.TableWidth() + 100)
				}
			})
			shown := 0
			oldShow := showGanttContextMenu
			showGanttContextMenu = func(menu *Menu, x, y float64, autoClose bool) {
				shown++
				if !c.HasFocus() || c.SelectedTaskID() != "a" || menu.Parent() != c || !autoClose {
					t.Fatal("menu did not capture the chart's own focused selection")
				}
				if !ganttContextAction(t, menu, "gantt.edit").IsEnabled() {
					t.Fatal("valid selected task cannot be edited")
				}
			}
			t.Cleanup(func() { showGanttContextMenu = oldShow })
			x, y := chartBarPoint(c, "a", "move")
			if state == "invalid-pointer" {
				x = math.NaN()
			}
			c.OnRightUp(x, y)
			want := 0
			if state == "normal" {
				want = 1
			}
			if shown != want || c.HasCapture() {
				t.Fatalf("right-click produced stale popup/capture: shown=%d want=%d", shown, want)
			}
		})
	}
}

func TestGanttContextDateEditHonorsCascadeAndRejectedEditKeepsRedo(t *testing.T) {
	for _, cascade := range []bool{false, true} {
		t.Run(map[bool]string{false: "strict", true: "cascade"}[cascade], func(t *testing.T) {
			c := chartFixture(t)
			dismissOwnedToasts(c)
			t.Cleanup(func() { dismissOwnedToasts(c) })
			x, y := chartBarPoint(c, "project", "move")
			c.OnLeftDown(x, y)
			c.OnLeftUp(x+c.DayWidth(), y)
			redo := c.Tasks()
			c.Undo()
			before := c.Tasks()
			c.SetCascadeDependencies(cascade)
			c.SetSelectedTaskID("a")
			changes, commits := 0, 0
			c.SigTasksChanged(func() { changes++ })
			c.SetEditCommittedCallback(func(_, _ []GanttTask) { commits++ })
			ganttContextPresent(t, func(d *Dialog) DialogResult {
				form := ganttContextForm(t, d)
				task, _ := c.model.Task("a")
				form.rows[3].widget.(*Edit).SetText(task.End.AddDate(0, 0, 1).Format("2006-01-02")) // extend exclusive End by 2 days
				d.OnKeyDown(KeyEnter, false)
				if cascade && !d.closed {
					t.Fatal("cascade editor rejected a valid downstream push")
				}
				if !cascade && d.closed {
					t.Fatal("strict editor closed after dependency failure")
				}
				return d.result
			})
			ganttContextRun(t, c, "gantt.edit")
			if cascade {
				if changes != 1 || commits != 1 || c.CanRedo() {
					t.Fatal("cascade Save did not commit one new branch")
				}
				for _, id := range []string{"b", "m"} {
					var old GanttTask
					for _, task := range before {
						if task.ID == id {
							old = task
						}
					}
					after, _ := c.model.Task(id)
					if GanttDaysBetween(old.Start, after.Start) != 2 {
						t.Fatalf("successor %s was not pushed two days", id)
					}
				}
				if !c.Undo() || c.CanUndo() || !reflect.DeepEqual(before, c.Tasks()) {
					t.Fatal("cascade Save was not exactly one undo")
				}
			} else {
				if changes != 0 || commits != 0 || c.CanUndo() || !c.CanRedo() || c.LastError() == nil || !reflect.DeepEqual(before, c.Tasks()) {
					t.Fatal("strict rejection mutated history/state or hid error")
				}
				if !c.Redo() || !reflect.DeepEqual(redo, c.Tasks()) {
					t.Fatal("strict rejected Save lost existing redo")
				}
			}
		})
	}
}

func TestGanttContextEditorCancelAndPermissionChangesPreserveTasks(t *testing.T) {
	for _, state := range []string{"escape", "readonly", "disabled", "hidden", "ancestor-disabled", "ancestor-hidden"} {
		t.Run(state, func(t *testing.T) {
			c := ganttContextFixture(t)
			c.SetSelectedTaskID("b")
			before := c.Tasks()
			changes, commits := 0, 0
			c.SigTasksChanged(func() { changes++ })
			c.SetEditCommittedCallback(func(_, _ []GanttTask) { commits++ })
			ganttContextPresent(t, func(d *Dialog) DialogResult {
				ganttContextForm(t, d).rows[1].widget.(*Edit).SetText("must not save")
				switch state {
				case "escape":
					d.OnKeyDown(KeyEsc, false)
					if !d.closed || d.result != DialogCancel {
						t.Fatal("Escape did not cancel editor")
					}
					return d.result
				case "readonly":
					c.SetReadOnly(true)
				case "disabled":
					c.SetEnabled(false)
				case "hidden":
					c.SetVisible(false)
				default:
					p := NewVBox()
					c.SetParent(p)
					if state == "ancestor-disabled" {
						p.SetEnabled(false)
					} else {
						p.SetVisible(false)
					}
				}
				d.OnKeyDown(KeyEnter, false)
				return DialogCancel
			})
			ganttContextRun(t, c, "gantt.edit")
			if changes != 0 || commits != 0 || c.CanUndo() || !reflect.DeepEqual(c.Tasks(), before) {
				t.Fatal("cancelled/inert chart editor changed tasks")
			}
		})
	}
}
