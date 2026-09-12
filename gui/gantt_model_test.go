package gui

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
	"time"
)

func ganttTestDate(day int) time.Time { return time.Date(2026, 3, day, 0, 0, 0, 0, time.UTC) }
func ganttTestLeaf(id, parent string, start, end int) GanttTask {
	return GanttTask{ID: id, ParentID: parent, Name: id, Start: ganttTestDate(start), End: ganttTestDate(end)}
}
func ganttTestModel(t *testing.T, tasks []GanttTask) *GanttModel {
	t.Helper()
	m, err := NewGanttModel(tasks)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func ganttTestTask(t *testing.T, m *GanttModel, id string, start, end int) GanttTask {
	t.Helper()
	task, ok := m.Task(id)
	if !ok || !task.Start.Equal(ganttTestDate(start)) || !task.End.Equal(ganttTestDate(end)) {
		t.Fatalf("task %s dates=%v/%v want=%d/%d", id, task.Start, task.End, start, end)
	}
	return task
}

func TestGanttNestedParentMoveIsAtomicAndUndoable(t *testing.T) {
	m := ganttTestModel(t, []GanttTask{{ID: "project", Collapsed: true}, {ID: "group", ParentID: "project"}, ganttTestLeaf("a", "group", 3, 5), ganttTestLeaf("b", "project", 7, 10)})
	before := m.Snapshot()
	if err := m.MoveTask("project", 4, false); err != nil {
		t.Fatal(err)
	}
	ganttTestTask(t, m, "project", 7, 14)
	ganttTestTask(t, m, "group", 7, 9)
	ganttTestTask(t, m, "a", 7, 9)
	ganttTestTask(t, m, "b", 11, 14)
	if len(m.VisibleRows()) != 1 {
		t.Fatal("moving expanded a collapsed subtree")
	}
	after := m.Snapshot()
	if !m.Undo() || !reflect.DeepEqual(m.Snapshot(), before) || m.CanUndo() {
		t.Fatal("parent drag was not one undo atom")
	}
	if !m.Redo() || !reflect.DeepEqual(m.Snapshot(), after) || m.CanRedo() {
		t.Fatal("redo lost descendant dates")
	}
}

func TestGanttLeafResizeSummarizesAncestorsAndProgress(t *testing.T) {
	a := ganttTestLeaf("a", "group", 3, 5)
	a.Progress = 1
	b := ganttTestLeaf("b", "project", 7, 10)
	m := ganttTestModel(t, []GanttTask{{ID: "project"}, {ID: "group", ParentID: "project"}, a, b})
	parent := ganttTestTask(t, m, "project", 3, 10)
	if math.Abs(parent.Progress-0.4) > 1e-12 {
		t.Fatalf("parent progress=%v", parent.Progress)
	}
	if err := m.ResizeTask("a", ganttTestDate(1), ganttTestDate(6), false); err != nil {
		t.Fatal(err)
	}
	ganttTestTask(t, m, "group", 1, 6)
	parent = ganttTestTask(t, m, "project", 1, 10)
	if math.Abs(parent.Progress-0.625) > 1e-12 {
		t.Fatalf("duration-weighted progress=%v", parent.Progress)
	}
	if err := m.SetProgress("project", 0.5); err == nil {
		t.Fatal("independent summary progress accepted")
	}
	if err := m.ResizeTask("project", ganttTestDate(1), ganttTestDate(12), false); err == nil {
		t.Fatal("summary resize silently broke descendant bounds")
	}
}

func TestGanttDependencyCascadeIncludesGroupsAndAncestors(t *testing.T) {
	a := ganttTestLeaf("a", "", 1, 3)
	b := ganttTestLeaf("b", "group", 3, 5)
	c := ganttTestLeaf("c", "group", 6, 7)
	d := ganttTestLeaf("d", "", 7, 9)
	d.Dependencies = []string{"group"}
	// Reverse dependency order forces more than one propagation pass.
	m := ganttTestModel(t, []GanttTask{d, {ID: "group", Dependencies: []string{"a"}}, b, c, a})
	before := m.Snapshot()
	if err := m.MoveTask("a", 4, false); err == nil {
		t.Fatal("non-cascade move violated FS constraints")
	}
	if !reflect.DeepEqual(m.Snapshot(), before) || m.CanUndo() {
		t.Fatal("rejected edit partly changed dates/history")
	}
	if err := m.MoveTask("a", 4, true); err != nil {
		t.Fatal(err)
	}
	ganttTestTask(t, m, "a", 5, 7)
	ganttTestTask(t, m, "b", 7, 9)
	ganttTestTask(t, m, "c", 10, 11)
	ganttTestTask(t, m, "group", 7, 11)
	ganttTestTask(t, m, "d", 11, 13)
	if !m.Undo() || !reflect.DeepEqual(m.Snapshot(), before) || m.CanUndo() {
		t.Fatal("cascade produced multiple history entries")
	}
	if !m.Redo() {
		t.Fatal("cascade cannot be redone")
	}
	if err := m.MoveTask("a", -2, true); err != nil {
		t.Fatal(err)
	}
	ganttTestTask(t, m, "group", 7, 11)
	ganttTestTask(t, m, "d", 11, 13) // never pull successors earlier
}

func TestGanttInvalidGraphsAndEditsPreserveRedo(t *testing.T) {
	base := []GanttTask{ganttTestLeaf("a", "", 1, 3), ganttTestLeaf("b", "", 3, 5)}
	m := ganttTestModel(t, base)
	if err := m.MoveTask("a", 1, false); err != nil {
		t.Fatal(err)
	}
	m.Undo()
	before, revision := m.Snapshot(), m.Revision()
	cases := map[string][]GanttTask{
		"duplicate":               {base[0], base[0]},
		"missing-parent":          {{ID: "a", ParentID: "missing", Start: ganttTestDate(1), End: ganttTestDate(3)}},
		"parent-cycle":            {{ID: "a", ParentID: "b"}, {ID: "b", ParentID: "a"}},
		"dependency-cycle":        {func() GanttTask { x := base[0]; x.Dependencies = []string{"b"}; return x }(), func() GanttTask { x := base[1]; x.Dependencies = []string{"a"}; return x }()},
		"missing-predecessor":     {func() GanttTask { x := base[0]; x.Dependencies = []string{"missing"}; return x }()},
		"parent-child-dependency": {{ID: "p"}, func() GanttTask { x := base[0]; x.ParentID = "p"; x.Dependencies = []string{"p"}; return x }()},
		"group-feedback":          {{ID: "p", Dependencies: []string{"b"}}, func() GanttTask { x := base[0]; x.ParentID = "p"; return x }(), func() GanttTask { x := base[1]; x.Dependencies = []string{"a"}; return x }()},
		"zero-duration":           {{ID: "a", Start: ganttTestDate(1), End: ganttTestDate(1)}},
		"nan-progress":            {func() GanttTask { x := base[0]; x.Progress = math.NaN(); return x }()},
		"milestone-parent":        {{ID: "p", Milestone: true}, func() GanttTask { x := base[0]; x.ParentID = "p"; return x }()},
	}
	for name, tasks := range cases {
		t.Run(name, func(t *testing.T) {
			if err := m.SetTasks(tasks); err == nil {
				t.Fatal("invalid state accepted")
			}
			if !reflect.DeepEqual(m.Snapshot(), before) || m.Revision() != revision || !m.CanRedo() || m.CanUndo() {
				t.Fatal("failed edit mutated model/history")
			}
		})
	}
	if err := m.MoveTask("a", 0, false); err != nil {
		t.Fatal(err)
	}
	if !m.CanRedo() || m.Revision() != revision {
		t.Fatal("no-op invalidated redo")
	}
	if err := m.MoveTask("a", int(^uint(0)>>1), true); err == nil {
		t.Fatal("overflowing shift accepted")
	}
	if !m.Redo() {
		t.Fatal("invalid operations destroyed redo")
	}
}

func TestGanttMilestonesAndCalendarDaysAcrossDST(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	task := GanttTask{ID: "dst", Start: time.Date(2026, 3, 7, 13, 0, 0, 0, zone), End: time.Date(2026, 3, 10, 13, 0, 0, 0, zone)}
	marker := ganttTestLeaf("milestone", "", 10, 10)
	marker.Milestone = true
	marker.Dependencies = []string{"dst"}
	m := ganttTestModel(t, []GanttTask{task, marker})
	if err := m.MoveTask("dst", 1, true); err != nil {
		t.Fatal(err)
	}
	x := ganttTestTask(t, m, "dst", 8, 11)
	ganttTestTask(t, m, "milestone", 11, 11)
	if GanttDaysBetween(x.Start, x.End) != 3 || x.Start.Location() != time.UTC {
		t.Fatal("DST altered calendar-day duration")
	}
	if err := m.ResizeTask("milestone", ganttTestDate(12), ganttTestDate(13), false); err == nil {
		t.Fatal("milestone became a duration without explicit type edit")
	}
}

func TestGanttUnlimitedHierarchyVisibilityAndDetachedSnapshots(t *testing.T) {
	const depth = 1200
	tasks := make([]GanttTask, depth)
	for i := range tasks {
		tasks[i] = GanttTask{ID: fmt.Sprint(i)}
		if i > 0 {
			tasks[i].ParentID = fmt.Sprint(i - 1)
		}
	}
	tasks[depth-1].Start, tasks[depth-1].End = ganttTestDate(1), ganttTestDate(2)
	m := ganttTestModel(t, tasks)
	rows := m.VisibleRows()
	if len(rows) != depth || rows[depth-1].Depth != depth-1 {
		t.Fatal("hierarchy depth was truncated")
	}
	if err := m.SetCollapsed("3", true); err != nil {
		t.Fatal(err)
	}
	if len(m.VisibleRows()) != 4 {
		t.Fatal("collapse does not hide only descendants")
	}
	if err := m.MoveTask("0", 2, false); err != nil {
		t.Fatal(err)
	}
	ganttTestTask(t, m, "1199", 3, 4)
	snapshot := m.Snapshot()
	snapshot.Tasks[0].Name = "mutated snapshot"
	if task, _ := m.Task("0"); task.Name == "mutated snapshot" {
		t.Fatal("snapshot aliases live data")
	}
	if err := m.Restore(snapshot); err != nil {
		t.Fatal(err)
	}
	if !m.Undo() {
		t.Fatal("restore is not an atomic edit")
	}
	if task, _ := m.Task("0"); task.Name == "mutated snapshot" {
		t.Fatal("restore mutated its undo snapshot")
	}
}

func TestGanttTaskMetadataAndDependencyCopies(t *testing.T) {
	a, b := ganttTestLeaf("a", "", 1, 2), ganttTestLeaf("b", "", 2, 4)
	b.Dependencies = []string{"a"}
	m := ganttTestModel(t, []GanttTask{a, b})
	b.Dependencies[0] = "broken-input"
	copy, _ := m.Task("b")
	copy.Dependencies[0] = "broken-copy"
	copy, _ = m.Task("b")
	copy.Name = "设计稿"
	copy.Owner = "工程组"
	copy.Description = "支持中文和\n换行"
	copy.Progress = 0.75
	if err := m.UpdateTask(copy, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := m.Task("b"); got.Dependencies[0] != "a" || got.Owner != copy.Owner || got.Progress != 0.75 {
		t.Fatal("metadata/dependency ownership broken")
	}
	if err := m.RemoveTask("a"); err != nil {
		t.Fatal(err)
	}
	if got, _ := m.Task("b"); len(got.Dependencies) != 0 {
		t.Fatal("remove left dangling dependency")
	}
	if !m.Undo() {
		t.Fatal("remove cannot be undone")
	}
	ganttTestTask(t, m, "a", 1, 2)
}

func TestGanttJSONStrictAtomicAndCalendarRoundTrip(t *testing.T) {
	var empty GanttModel
	if err := empty.SetJSON("[]"); err != nil || empty.CanUndo() {
		t.Fatal("empty JSON changed an already empty model's history")
	}
	m := ganttTestModel(t, []GanttTask{ganttTestLeaf("existing", "", 1, 2)})
	valid := `[{"id":"p","name":"工程","collapsed":true},{"id":"a","parentId":"p","name":"叶子","start":"2026-03-07T23:30:00-08:00","end":"2026-03-10","progress":0.5,"owner":"团队","description":"备注"}]`
	if err := m.SetJSON(valid); err != nil {
		t.Fatal(err)
	}
	ganttTestTask(t, m, "a", 7, 10)
	canonical := m.JSON()
	if strings.Contains(canonical, "T23:") || !strings.Contains(canonical, `"start":"2026-03-07"`) {
		t.Fatal("JSON changed civil dates")
	}
	var copy GanttModel
	if err := json.Unmarshal([]byte(canonical), &copy); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(copy.Tasks(), m.Tasks()) {
		t.Fatal("JSON failed to preserve hierarchy/metadata")
	}
	before, revision := m.Snapshot(), m.Revision()
	for _, bad := range []string{`null`, `[null]`, `[] []`, `[{"id":"a","id":"b"}]`, `[{"id":"a","ID":"b"}]`, `[{"id":"a","unknown":1}]`, `[{"id":"a","start":"not-a-date","end":"2026-01-02"}]`, `[{"id":"a","start":"2026-01-01","end":"2026-01-02","progress":2}]`, strings.Repeat("[", 20) + strings.Repeat("]", 20)} {
		if err := m.SetJSON(bad); err == nil {
			t.Errorf("accepted invalid JSON %s", bad)
		}
		if !reflect.DeepEqual(m.Snapshot(), before) || m.Revision() != revision {
			t.Fatal("invalid JSON mutated model")
		}
	}
	if err := m.SetJSON(strings.Repeat(" ", ganttMaxJSONBytes+1)); err == nil {
		t.Fatal("unbounded JSON accepted")
	}
}

func TestGanttAddRemoveAndReparentAreSingleAtomicEdits(t *testing.T) {
	m := ganttTestModel(t, []GanttTask{{ID: "p"}, {ID: "q"}, ganttTestLeaf("a", "p", 1, 3), ganttTestLeaf("b", "p", 10, 12), ganttTestLeaf("c", "q", 20, 22)})
	before := m.Snapshot()
	if err := m.ReparentTask("b", "q"); err != nil {
		t.Fatal(err)
	}
	ganttTestTask(t, m, "p", 1, 3)
	ganttTestTask(t, m, "q", 10, 22)
	ganttTestTask(t, m, "b", 10, 12)
	after := m.Snapshot()
	if !m.Undo() || !reflect.DeepEqual(before, m.Snapshot()) || m.CanUndo() {
		t.Fatal("reparent was not one atomic undo")
	}
	if !m.Redo() || !reflect.DeepEqual(after, m.Snapshot()) {
		t.Fatal("reparent redo differs")
	}
	if err := m.ReparentTask("p", "q"); err != nil {
		t.Fatal(err)
	}
	ganttTestTask(t, m, "q", 1, 22)
	for _, parent := range []string{"p", "a", "missing"} {
		snapshot, revision := m.Snapshot(), m.Revision()
		if err := m.ReparentTask("p", parent); err == nil {
			t.Fatalf("accepted invalid parent %q", parent)
		}
		if !reflect.DeepEqual(snapshot, m.Snapshot()) || revision != m.Revision() {
			t.Fatal("invalid reparent changed state/history")
		}
	}
	if err := m.ReparentTask("p", ""); err != nil {
		t.Fatal(err)
	}
	ganttTestTask(t, m, "q", 10, 22)
	added := ganttTestLeaf("dependent", "", 24, 25)
	added.Dependencies = []string{"p", "a", "b"}
	if err := m.AddTask(added); err != nil {
		t.Fatal(err)
	}
	before = m.Snapshot()
	if err := m.AddTask(added); err == nil {
		t.Fatal("duplicate AddTask accepted")
	}
	if !reflect.DeepEqual(before, m.Snapshot()) {
		t.Fatal("rejected add changed state")
	}
	if err := m.RemoveTask("p"); err != nil {
		t.Fatal(err)
	}
	if _, exists := m.Task("a"); exists {
		t.Fatal("remove did not delete full subtree")
	}
	dependent, _ := m.Task("dependent")
	if !reflect.DeepEqual(dependent.Dependencies, []string{"b"}) {
		t.Fatalf("removed subtree left stale dependencies: %v", dependent.Dependencies)
	}
	if !m.Undo() || !reflect.DeepEqual(before, m.Snapshot()) {
		t.Fatal("remove did not restore subtree and dependencies atomically")
	}
}

func TestGanttReparentRejectsNewCombinedDependencyCycle(t *testing.T) {
	a, b := ganttTestLeaf("a", "", 1, 3), ganttTestLeaf("b", "group", 3, 5)
	b.Dependencies = []string{"a"}
	m := ganttTestModel(t, []GanttTask{a, {ID: "group"}, b})
	before := m.Snapshot()
	// Turning a into b's ancestor would make b depend on its own summary end.
	if err := m.ReparentTask("group", "a"); err == nil {
		t.Fatal("reparent introduced an implicit dependency cycle")
	}
	if !reflect.DeepEqual(before, m.Snapshot()) || m.CanUndo() {
		t.Fatal("rejected dependency reparent changed model")
	}
}

func TestGanttBoundedRandomEditsKeepTreeScheduleAndHistoryInvariants(t *testing.T) {
	rng := rand.New(rand.NewPCG(23, 91))
	m := ganttTestModel(t, []GanttTask{{ID: "p"}, {ID: "q"}, ganttTestLeaf("a", "p", 1, 3), ganttTestLeaf("b", "p", 5, 8), ganttTestLeaf("c", "q", 11, 15)})
	for step := 0; step < 300; step++ {
		tasks := m.Tasks()
		if len(tasks) == 0 {
			if err := m.AddTask(ganttTestLeaf(fmt.Sprintf("seed-%d", step), "", 1, 3)); err != nil {
				t.Fatal(err)
			}
			tasks = m.Tasks()
		}
		id := tasks[rng.IntN(len(tasks))].ID
		before, revision := m.Snapshot(), m.Revision()
		undoLen, redoLen := len(m.undo), len(m.redo)
		var err error
		switch rng.IntN(7) {
		case 0:
			err = m.MoveTask(id, rng.IntN(11)-4, true)
		case 1:
			parent := ""
			if rng.IntN(2) == 0 {
				parent = tasks[rng.IntN(len(tasks))].ID
			}
			err = m.ReparentTask(id, parent)
		case 2:
			err = m.RemoveTask(id)
		case 3:
			added := ganttTestLeaf(fmt.Sprintf("added-%d", step), "", 20, 23)
			if rng.IntN(2) == 0 {
				added.ParentID = id
			}
			if rng.IntN(2) == 0 {
				added.Dependencies = []string{id}
			}
			err = m.AddTask(added)
		case 4:
			err = m.SetProgress(id, rng.Float64())
		case 5:
			err = m.SetCollapsed(id, rng.IntN(2) == 0)
		case 6:
			task, _ := m.Task(id)
			err = m.ResizeTask(id, task.Start.AddDate(0, 0, rng.IntN(5)-2), task.End.AddDate(0, 0, rng.IntN(5)-2), true)
		}
		if err != nil {
			if !reflect.DeepEqual(before, m.Snapshot()) || m.Revision() != revision || len(m.undo) != undoLen || len(m.redo) != redoLen {
				t.Fatalf("step %d rejected operation partly committed: %v", step, err)
			}
			continue
		}
		after := m.Snapshot()
		if _, err := NewGanttModel(after.Tasks); err != nil {
			t.Fatalf("step %d produced invalid tree/schedule: %v", step, err)
		}
		if m.Revision() != revision {
			if !m.Undo() || !reflect.DeepEqual(before, m.Snapshot()) {
				t.Fatalf("step %d did not undo exactly", step)
			}
			if !m.Redo() || !reflect.DeepEqual(after, m.Snapshot()) {
				t.Fatalf("step %d did not redo exactly", step)
			}
		} else if len(m.undo) != undoLen || len(m.redo) != redoLen {
			t.Fatalf("step %d no-op changed history", step)
		}
	}
}
