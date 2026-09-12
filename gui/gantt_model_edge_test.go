package gui

import (
	"math/rand/v2"
	"reflect"
	"testing"
	"time"
)

func TestGanttDependencyDiamondIsIndependentOfTaskOrder(t *testing.T) {
	z := ganttTestLeaf("z", "", 1, 3)
	a, b := ganttTestLeaf("a", "group", 3, 5), ganttTestLeaf("b", "group", 6, 9)
	x, y := ganttTestLeaf("x", "", 9, 10), ganttTestLeaf("y", "", 10, 12)
	x.Dependencies, y.Dependencies = []string{"group"}, []string{"a", "x"}
	base := []GanttTask{{ID: "group", Dependencies: []string{"z"}, Collapsed: true}, z, a, b, x, y}
	for seed := uint64(0); seed < 24; seed++ {
		tasks := cloneGanttTasks(base)
		rng := rand.New(rand.NewPCG(seed, 37))
		rng.Shuffle(len(tasks), func(i, j int) { tasks[i], tasks[j] = tasks[j], tasks[i] })
		m := ganttTestModel(t, tasks)
		before := m.Snapshot()
		if err := m.MoveTask("z", 2, true); err != nil {
			t.Fatalf("permutation %d cascade failed: %v", seed, err)
		}
		for _, want := range []struct {
			id         string
			start, end int
		}{{"z", 3, 5}, {"group", 5, 11}, {"a", 5, 7}, {"b", 8, 11}, {"x", 11, 12}, {"y", 12, 14}} {
			ganttTestTask(t, m, want.id, want.start, want.end)
		}
		after := m.Snapshot()
		if !m.Undo() || m.CanUndo() || !reflect.DeepEqual(before, m.Snapshot()) {
			t.Fatalf("permutation %d cascade was not one undo atom", seed)
		}
		if !m.Redo() || !reflect.DeepEqual(after, m.Snapshot()) {
			t.Fatalf("permutation %d redo differed", seed)
		}
	}
}

func TestGanttEmptyMilestoneSummaryPreservesDatesAndHistory(t *testing.T) {
	for _, operation := range []string{"remove", "reparent", "update-parent"} {
		t.Run(operation, func(t *testing.T) {
			marker := ganttTestLeaf("marker", "group", 7, 7)
			marker.Milestone, marker.Progress = true, 0.75
			m := ganttTestModel(t, []GanttTask{{ID: "group", Name: "Empty after edit", Collapsed: true}, marker, ganttTestLeaf("other", "", 10, 12)})
			before := m.Snapshot()
			var err error
			switch operation {
			case "remove":
				err = m.RemoveTask("marker")
			case "reparent":
				err = m.ReparentTask("marker", "other")
			case "update-parent":
				task, _ := m.Task("marker")
				task.ParentID = "other"
				err = m.UpdateTask(task, false)
			}
			if err != nil {
				t.Fatalf("valid last-child operation rejected: %v", err)
			}
			group := ganttTestTask(t, m, "group", 7, 7)
			if !group.Milestone || group.Progress != 0.75 || group.Name != "Empty after edit" || !group.Collapsed {
				t.Fatalf("empty summary did not retain its date/metadata as a milestone: %+v", group)
			}
			after := m.Snapshot()
			if _, err := NewGanttModel(after.Tasks); err != nil {
				t.Fatalf("edit produced a non-round-trippable state: %v", err)
			}
			var reloaded GanttModel
			if err := reloaded.SetJSON(m.JSON()); err != nil || !reflect.DeepEqual(reloaded.Snapshot(), after) {
				t.Fatalf("empty-summary JSON reload changed state: %v", err)
			}
			if !m.Undo() || m.CanUndo() || !reflect.DeepEqual(m.Snapshot(), before) {
				t.Fatal("last-child edit did not undo as one atom")
			}
			if !m.Redo() || !reflect.DeepEqual(m.Snapshot(), after) {
				t.Fatal("last-child edit did not redo exactly")
			}
		})
	}
}

func TestGanttNonemptyMilestoneSummaryRemainsAGroup(t *testing.T) {
	a, b := ganttTestLeaf("a", "group", 7, 7), ganttTestLeaf("b", "group", 7, 7)
	a.Milestone, b.Milestone = true, true
	m := ganttTestModel(t, []GanttTask{{ID: "root"}, {ID: "group", ParentID: "root"}, a, b})
	if err := m.RemoveTask("a"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"root", "group"} {
		if task, _ := m.Task(id); task.Milestone {
			t.Fatalf("nonempty summary %s became a milestone", id)
		}
	}
	if err := m.MoveTask("root", 2, false); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"root", "group", "b"} {
		ganttTestTask(t, m, id, 9, 9)
	}
	if err := m.RemoveTask("b"); err != nil {
		t.Fatal(err)
	}
	if task, _ := m.Task("root"); task.Milestone {
		t.Fatal("ancestor with the remaining empty-group child became a milestone")
	}
	if task, _ := m.Task("group"); !task.Milestone {
		t.Fatal("directly emptied summary did not become a milestone")
	}
}

func TestGanttCascadeDateOverflowRejectsWithoutDestroyingRedo(t *testing.T) {
	a := GanttTask{ID: "a", Start: time.Date(9999, 12, 26, 0, 0, 0, 0, time.UTC), End: time.Date(9999, 12, 28, 0, 0, 0, 0, time.UTC)}
	b := GanttTask{ID: "b", Start: a.End, End: time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC), Dependencies: []string{"a"}}
	m := ganttTestModel(t, []GanttTask{a, b})
	if err := m.SetProgress("a", 0.5); err != nil {
		t.Fatal(err)
	}
	m.Undo()
	before, revision := m.Snapshot(), m.Revision()
	if err := m.MoveTask("a", 1, true); err == nil {
		t.Fatal("cascade pushed a successor beyond year 9999")
	}
	if !reflect.DeepEqual(m.Snapshot(), before) || m.Revision() != revision || m.CanUndo() || !m.CanRedo() {
		t.Fatal("rejected overflow partially modified dates/history")
	}
	if !m.Redo() {
		t.Fatal("rejected overflow destroyed a prior redo")
	}
}

func TestGanttLeapDayCalendarMoveAndJSONRoundTrip(t *testing.T) {
	task := GanttTask{ID: "leap", Start: time.Date(2024, 2, 28, 23, 0, 0, 0, time.FixedZone("+14", 14*3600)), End: time.Date(2024, 3, 1, 2, 0, 0, 0, time.FixedZone("-12", -12*3600))}
	m := ganttTestModel(t, []GanttTask{task})
	if err := m.MoveTask("leap", 1, false); err != nil {
		t.Fatal(err)
	}
	got, _ := m.Task("leap")
	if got.Start.Format(time.DateOnly) != "2024-02-29" || got.End.Format(time.DateOnly) != "2024-03-02" || GanttDaysBetween(got.Start, got.End) != 2 {
		t.Fatalf("calendar move changed leap-day duration: %v / %v", got.Start, got.End)
	}
	var copy GanttModel
	if err := copy.SetJSON(m.JSON()); err != nil || !reflect.DeepEqual(copy.Tasks(), m.Tasks()) {
		t.Fatalf("leap-day JSON round trip failed: %v", err)
	}
	before := m.Snapshot()
	if err := m.SetJSON(`[{"id":"invalid","start":"2026-02-29","end":"2026-03-02"}]`); err == nil {
		t.Fatal("non-leap-year February 29 was accepted")
	}
	if !reflect.DeepEqual(m.Snapshot(), before) {
		t.Fatal("invalid leap date changed live state")
	}
}
