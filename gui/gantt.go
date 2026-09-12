package gui

import (
	"math"
	"reflect"
	"time"

	"github.com/uk0/silk/core"
)

// GanttChart is a tree/table and calendar timeline backed by a validated task
// model. Dates are calendar days; End is exclusive (the table shows End-1 day).
// All setters and callbacks, like other widgets, run on the UI thread.
type GanttChart struct {
	Widget
	model                           *GanttModel
	rows                            []GanttRow
	rowIndex                        map[string]int
	taskIndex                       map[string]GanttTask
	start, end                      time.Time
	dayWidth, rowHeight, tableWidth float64
	scrollX, scrollY                float64
	readOnly                        bool
	cascadeDependencies             bool
	selectedID                      string
	lastError                       error
	cbTasksChanged                  func()
	cbEditCommitted                 func(before, after []GanttTask)
	cbSelectionChanged              func(id string)
	drag                            *ganttGesture
	hover                           ganttHit
}

func init() { core.RegisterFactory("gui.GanttChart", core.TypeOf((*GanttChart)(nil))) }

func NewGanttChart() *GanttChart { p := new(GanttChart); p.Init(p); return p }

// Init also serves core.New, the designer palette and generated SDK forms.
func (c *GanttChart) Init(self IWidget) {
	c.Widget.Init(self)
	c.dayWidth, c.rowHeight, c.tableWidth = 30, 30, 430
	c.cascadeDependencies = true
	c.model, _ = NewGanttModel(ganttDemoTasks(time.Now()))
	c.rebuildRows(true)
	c.SetSize(900, 390)
}

func ganttDemoTasks(now time.Time) []GanttTask {
	d := GanttDate(now).AddDate(0, 0, -3)
	return []GanttTask{
		{ID: "release", Name: "Silk UI release", Start: d, End: d.AddDate(0, 0, 19)},
		{ID: "design", ParentID: "release", Name: "Design", Start: d, End: d.AddDate(0, 0, 5), Owner: "Design", Progress: .8},
		{ID: "build", ParentID: "release", Name: "Implementation", Start: d.AddDate(0, 0, 5), End: d.AddDate(0, 0, 14)},
		{ID: "widgets", ParentID: "build", Name: "Widgets and layout", Start: d.AddDate(0, 0, 5), End: d.AddDate(0, 0, 10), Owner: "UI", Progress: .45, Dependencies: []string{"design"}},
		{ID: "sdk", ParentID: "build", Name: "SDK integration", Start: d.AddDate(0, 0, 10), End: d.AddDate(0, 0, 14), Owner: "SDK", Progress: .1, Dependencies: []string{"widgets"}},
		{ID: "verify", ParentID: "release", Name: "Cross-platform verification", Start: d.AddDate(0, 0, 14), End: d.AddDate(0, 0, 19), Owner: "QA", Dependencies: []string{"sdk"}},
		{ID: "ship", ParentID: "release", Name: "Release", Start: d.AddDate(0, 0, 19), End: d.AddDate(0, 0, 19), Milestone: true, Dependencies: []string{"verify"}},
	}
}

func (c *GanttChart) Tasks() []GanttTask { return c.model.Tasks() }
func (c *GanttChart) SetTasks(tasks []GanttTask) error {
	m, err := NewGanttModel(tasks)
	if err != nil {
		c.lastError = err
		c.Update()
		return err
	}
	c.CancelInteraction()
	c.model = m
	c.lastError = nil
	c.rebuildRows(true)
	c.changed()
	return nil
}
func (c *GanttChart) TasksJSON() string { return c.model.JSON() }

// SetTasksJSON is a property-sheet compatible setter. Invalid input leaves the
// previous document and history intact; inspect LastError or use LoadTasksJSON.
func (c *GanttChart) SetTasksJSON(s string) { _ = c.LoadTasksJSON(s) }
func (c *GanttChart) LoadTasksJSON(s string) error {
	m, _ := NewGanttModel(nil)
	if err := m.SetJSON(s); err != nil {
		c.lastError = err
		c.Update()
		return err
	}
	return c.SetTasks(m.Tasks())
}
func (c *GanttChart) LastError() error              { return c.lastError }
func (c *GanttChart) ReadOnly() bool                { return c.readOnly }
func (c *GanttChart) IsReadOnly() bool              { return c.readOnly }
func (c *GanttChart) CascadeDependencies() bool     { return c.cascadeDependencies }
func (c *GanttChart) SetCascadeDependencies(v bool) { c.CancelInteraction(); c.cascadeDependencies = v }
func (c *GanttChart) SetReadOnly(v bool) {
	if v {
		c.CancelInteraction()
	}
	c.readOnly = v
	c.Update()
}
func (c *GanttChart) SetEnabled(v bool) {
	if !v {
		c.CancelInteraction()
	}
	c.Widget.SetEnabled(v)
}
func (c *GanttChart) DayWidth() float64 { return c.dayWidth }
func (c *GanttChart) SetDayWidth(v float64) {
	if !ganttFinite(v) {
		return
	}
	c.CancelInteraction()
	c.dayWidth = ganttClamp(v, 4, 120)
	c.clampScroll()
	c.Update()
}
func (c *GanttChart) RowHeight() float64 { return c.rowHeight }
func (c *GanttChart) SetRowHeight(v float64) {
	if !ganttFinite(v) {
		return
	}
	c.CancelInteraction()
	c.rowHeight = ganttClamp(v, 22, 96)
	c.clampScroll()
	c.Update()
}
func (c *GanttChart) TableWidth() float64 { return c.tableWidth }
func (c *GanttChart) SetTableWidth(v float64) {
	if !ganttFinite(v) {
		return
	}
	c.CancelInteraction()
	c.tableWidth = ganttClamp(v, 100, 1200)
	c.clampScroll()
	c.Update()
}
func (c *GanttChart) SelectedTaskID() string { return c.selectedID }
func (c *GanttChart) SetSelectedTaskID(id string) {
	if id != "" {
		if _, ok := c.taskIndex[id]; !ok {
			return
		}
	}
	if c.selectedID == id {
		return
	}
	c.selectedID = id
	c.Update()
	if c.cbSelectionChanged != nil {
		c.cbSelectionChanged(id)
	}
}
func (c *GanttChart) SigTasksChanged(fn func()) { c.cbTasksChanged = fn }

// SetEditCommittedCallback fires once per committed gesture, never during
// preview/cancel. Snapshots are independent copies suitable for host undo.
func (c *GanttChart) SetEditCommittedCallback(fn func(before, after []GanttTask)) {
	c.cbEditCommitted = fn
}
func (c *GanttChart) SetSelectionChangedCallback(fn func(id string)) { c.cbSelectionChanged = fn }
func (c *GanttChart) EnumProperties(p core.IPropertyList) {
	p.AddProperty("tasksJSON", c.TasksJSON, c.SetTasksJSON)
	p.AddProperty("readOnly", c.ReadOnly, c.SetReadOnly)
	p.AddProperty("dayWidth", c.DayWidth, c.SetDayWidth)
	p.AddProperty("rowHeight", c.RowHeight, c.SetRowHeight)
	p.AddProperty("tableWidth", c.TableWidth, c.SetTableWidth)
	p.AddProperty("cascadeDependencies", c.CascadeDependencies, c.SetCascadeDependencies)
}
func (c *GanttChart) CanUndo() bool { return c.model.CanUndo() }
func (c *GanttChart) CanRedo() bool { return c.model.CanRedo() }
func (c *GanttChart) Undo() bool {
	if c.readOnly {
		return false
	}
	c.CancelInteraction()
	if !c.model.Undo() {
		return false
	}
	c.rebuildRows(false)
	c.changed()
	return true
}
func (c *GanttChart) Redo() bool {
	if c.readOnly {
		return false
	}
	c.CancelInteraction()
	if !c.model.Redo() {
		return false
	}
	c.rebuildRows(false)
	c.changed()
	return true
}
func (c *GanttChart) SetCollapsed(id string, v bool) error {
	c.CancelInteraction()
	before := c.Tasks()
	if err := c.model.SetCollapsed(id, v); err != nil {
		c.lastError = err
		return err
	}
	c.rebuildRows(false)
	c.commitNotification(before)
	return nil
}
func (c *GanttChart) changed() {
	c.Update()
	if c.cbTasksChanged != nil {
		c.cbTasksChanged()
	}
}
func (c *GanttChart) commitNotification(before []GanttTask) {
	c.lastError = nil
	after := c.Tasks()
	c.Update()
	if reflect.DeepEqual(before, after) {
		return
	}
	// Keep snapshots isolated even when the normal changed callback edits data.
	cb := c.cbEditCommitted
	if c.cbTasksChanged != nil {
		c.cbTasksChanged()
	}
	if cb != nil {
		cb(before, after)
	}
}
func (c *GanttChart) displayedModel() *GanttModel {
	if c.drag != nil && c.drag.preview != nil {
		return c.drag.preview
	}
	return c.model
}
func (c *GanttChart) rebuildRows(reset bool) {
	oldStart := c.start
	m := c.displayedModel()
	c.rows = m.VisibleRows()
	c.rowIndex = make(map[string]int, len(c.rows))
	c.taskIndex = make(map[string]GanttTask)
	for i, r := range c.rows {
		c.rowIndex[r.Task.ID] = i
	}
	tasks := m.Tasks()
	if reset {
		c.start = GanttDate(time.Now())
		c.end = c.start.AddDate(0, 0, 30)
		c.scrollX = 0
		c.scrollY = 0
	}
	for i, t := range tasks {
		c.taskIndex[t.ID] = t
		if reset && i == 0 {
			c.start = t.Start.AddDate(0, 0, -3)
			c.end = t.End.AddDate(0, 0, 7)
		}
		if c.drag == nil && t.Start.Before(c.start) {
			c.start = t.Start.AddDate(0, 0, -3)
		}
		if c.drag == nil && t.End.After(c.end) {
			c.end = t.End.AddDate(0, 0, 7)
		}
	}
	if !reset && !oldStart.IsZero() && !oldStart.Equal(c.start) {
		c.scrollX += float64(GanttDaysBetween(c.start, oldStart)) * c.dayWidth
	}
	if _, ok := c.taskIndex[c.selectedID]; !ok {
		c.selectedID = ""
	}
	c.clampScroll()
}
func (c *GanttChart) ScrollOffsets() (float64, float64) { return c.scrollX, c.scrollY }
func (c *GanttChart) SetScrollOffsets(x, y float64) {
	if !ganttFinite(x) || !ganttFinite(y) {
		return
	}
	c.scrollX = x
	c.scrollY = y
	c.clampScroll()
	c.Update()
}
func (c *GanttChart) ScrollToTask(id string) {
	i, ok := c.rowIndex[id]
	if !ok {
		return
	}
	l := c.geometry()
	c.scrollY = float64(i) * l.row
	t := c.taskIndex[id]
	c.scrollX = float64(GanttDaysBetween(c.start, t.Start))*c.dayWidth - c.dayWidth
	c.clampScroll()
	c.Update()
}
func (c *GanttChart) OnResize() { c.CancelInteraction(); c.clampScroll(); c.Widget.OnResize() }
func (c *GanttChart) OnHide()   { c.CancelInteraction() }
func (c *GanttChart) OnFocusChanged(newFocus, oldFocus IWidget) {
	if newFocus != c.Self() {
		c.CancelInteraction()
	}
}
func ganttFinite(v float64) bool           { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func ganttClamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
