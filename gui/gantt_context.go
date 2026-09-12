package gui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/uk0/silk/i18n"
	"github.com/uk0/silk/paint"
)

var showGanttDialog = (*Dialog).ShowModal
var showGanttContextMenu = (*Menu).ShowAsPopup

func (c *GanttChart) OnRightUp(x, y float64) {
	if !c.interactive() || !ganttFinite(x) || !ganttFinite(y) {
		return
	}
	c.CancelInteraction()
	c.SetFocus()
	if !c.interactive() || !c.HasFocus() {
		return
	}
	if hit := c.hitTest(x, y); hit.row >= 0 {
		id := c.rows[hit.row].Task.ID
		model, revision := c.model, c.model.Revision()
		c.SetSelectedTaskID(id)
		if c.model != model || model.Revision() != revision || !c.interactive() || !c.HasFocus() || c.selectedID != id || c.hitTest(x, y) != hit {
			return
		}
	}
	m := c.buildGanttContextMenu()
	m.SetParent(c)
	gx, gy := c.MapToGlobal(x, y)
	showGanttContextMenu(m, gx, gy, true)
}

func (c *GanttChart) buildGanttContextMenu() *Menu {
	m := NewPopupMenu()
	model, revision := c.model, c.model.Revision()
	add := func(id, label, icon string, enabled bool, run func()) {
		b := m.AddButton1(i18n.T(label), paint.LoadIcon(icon))
		b.Action().SetObjName(id)
		b.Action().BindFunc0(func() {
			if c.model != model || c.model.Revision() != revision || !c.interactive() {
				return
			}
			if id != "gantt.collapse" && c.readOnly {
				return
			}
			run()
		})
		b.SetEnabled(enabled)
		b.Action().SetEnabled(enabled)
	}
	task, selected := c.model.Task(c.selectedID)
	canEdit := !c.readOnly && c.interactive()
	add("gantt.add", "Add task...", "plus", canEdit, func() { c.editGanttTask("", "") })
	add("gantt.child", "Add child task...", "plus", canEdit && selected && !task.Milestone, func() { c.editGanttTask("", task.ID) })
	add("gantt.edit", "Edit task...", "edit", canEdit && selected, func() { c.editGanttTask(task.ID, "") })
	m.AddSeparator()
	previous := c.previousGanttSibling(task.ID)
	add("gantt.indent", "Indent task", "layout-horizontal", canEdit && selected && previous != "", func() { c.reparentGanttTask(task.ID, previous) })
	add("gantt.outdent", "Outdent task", "layout-horizontal", canEdit && selected && task.ParentID != "", func() {
		parent, ok := c.model.Task(task.ParentID)
		if ok {
			c.reparentGanttTask(task.ID, parent.ParentID)
		}
	})
	hasChildren := false
	for _, t := range c.Tasks() {
		if t.ParentID == task.ID {
			hasChildren = true
			break
		}
	}
	label := "Collapse branch"
	if task.Collapsed {
		label = "Expand branch"
	}
	add("gantt.collapse", label, "tree-view", selected && hasChildren, func() { _ = c.SetCollapsed(task.ID, !task.Collapsed) })
	m.AddSeparator()
	add("gantt.undo", "Undo", "edit-undo", canEdit && c.CanUndo(), func() { c.Undo() })
	add("gantt.redo", "Redo", "edit-redo", canEdit && c.CanRedo(), func() { c.Redo() })
	add("gantt.delete", "Delete task subtree...", "close", canEdit && selected, func() {
		dlg := NewDialog(i18n.T("Delete task"), c)
		dlg.SetContent(NewLabel(i18n.T("Delete this task and its children? Undo can restore them.")))
		dlg.AddButton(i18n.T("Cancel"), DialogCancel)
		dlg.AddButton(i18n.T("Delete"), DialogOK)
		dlg.SetSize(560, 180)
		if showGanttDialog(dlg) == DialogOK && c.model == model && c.model.Revision() == revision {
			c.applyGanttContextEdit(func() error { return c.model.RemoveTask(task.ID) })
		}
	})
	return m
}

func (c *GanttChart) previousGanttSibling(id string) string {
	task, ok := c.model.Task(id)
	if !ok {
		return ""
	}
	previous := ""
	for _, t := range c.Tasks() {
		if t.ID == id {
			return previous
		}
		if t.ParentID == task.ParentID && !t.Milestone {
			previous = t.ID
		}
	}
	return ""
}

func (c *GanttChart) applyGanttContextEdit(edit func() error) error {
	if c.readOnly || !c.interactive() {
		return fmt.Errorf("%s", i18n.T("Gantt chart is read-only"))
	}
	c.CancelInteraction()
	before := c.Tasks()
	if err := edit(); err != nil {
		c.lastError = err
		c.Update()
		ShowToast(c, err.Error(), 4000, ToastWarning)
		return err
	}
	c.rebuildRows(false)
	if _, ok := c.model.Task(c.selectedID); !ok {
		c.SetSelectedTaskID("")
	}
	c.commitNotification(before)
	return nil
}
func (c *GanttChart) reparentGanttTask(id, parent string) {
	_ = c.applyGanttContextEdit(func() error { return c.model.ReparentTask(id, parent) })
}

func (c *GanttChart) nextGanttTaskID() string {
	for n := 1; ; n++ {
		id := fmt.Sprintf("task%d", n)
		if _, ok := c.model.Task(id); !ok {
			return id
		}
	}
}

// Task form uses inclusive Finish for display; the SDK/JSON contract remains
// End-exclusive. Summary dates/progress come from children, not editable fields.
func (c *GanttChart) editGanttTask(id, parent string) {
	if c.readOnly || !c.interactive() {
		return
	}
	task, editing := c.model.Task(id)
	if id != "" && !editing {
		return
	}
	if !editing {
		start := GanttDate(time.Now())
		if p, ok := c.model.Task(parent); ok {
			start = p.Start
		}
		task = GanttTask{ID: c.nextGanttTaskID(), ParentID: parent, Name: i18n.T("New task"), Start: start, End: start.AddDate(0, 0, 1)}
	}
	summary := false
	for _, t := range c.Tasks() {
		if t.ParentID == task.ID {
			summary = true
			break
		}
	}
	model, revision := c.model, c.model.Revision()
	dlg := NewDialog(i18n.T("Edit task"), c)
	form := NewFormLayout()
	form.SetRowHeight(32)
	form.SetSpacing(6)
	form.SetLabelWidth(120)
	text := func(label, value string) *Edit {
		e := NewEdit()
		e.SetText(value)
		form.AddRow(i18n.T(label), e)
		return e
	}
	form.AddRow("ID", NewLabel(task.ID))
	name := text("Task", task.Name)
	start := text("Start (YYYY-MM-DD)", task.Start.Format(time.DateOnly))
	finishDate := task.End
	if !task.Milestone {
		finishDate = finishDate.AddDate(0, 0, -1)
	}
	finish := text("Finish (inclusive)", finishDate.Format(time.DateOnly))
	progress := NewSpinBox()
	progress.SetRange(0, 100)
	progress.SetValue(int(task.Progress*100 + .5))
	form.AddRow(i18n.T("Progress (%)"), progress)
	owner := text("Owner", task.Owner)
	deps := text("Predecessor IDs", strings.Join(task.Dependencies, ","))
	desc := text("Description", task.Description)
	milestone := NewCheckBox()
	milestone.SetText(i18n.T("Milestone"))
	milestone.SetChecked(task.Milestone)
	form.AddRow(i18n.T("Task type"), milestone)
	for _, field := range []IWidget{start, finish, progress, milestone} {
		field.SetEnabled(!summary)
	}
	dlg.SetContent(form)
	dlg.AddButton(i18n.T("Cancel"), DialogCancel)
	ok := dlg.AddButton(i18n.T("Save"), DialogOK)
	ok.Action().BindFunc0(func() {
		if c.model != model || c.model.Revision() != revision {
			ShowToast(dlg, i18n.T("Task data changed; reopen the editor"), 3500, ToastWarning)
			return
		}
		next := task
		next.Name = strings.TrimSpace(name.Text())
		next.Owner = owner.Text()
		next.Description = desc.Text()
		next.Dependencies = nil
		for _, id := range strings.Split(deps.Text(), ",") {
			if id = strings.TrimSpace(id); id != "" {
				next.Dependencies = append(next.Dependencies, id)
			}
		}
		if !summary {
			var err error
			next.Start, err = time.Parse(time.DateOnly, strings.TrimSpace(start.Text()))
			if err != nil {
				ShowToast(dlg, i18n.T("Use dates in YYYY-MM-DD format"), 3500, ToastWarning)
				return
			}
			next.End, err = time.Parse(time.DateOnly, strings.TrimSpace(finish.Text()))
			if err != nil {
				ShowToast(dlg, i18n.T("Use dates in YYYY-MM-DD format"), 3500, ToastWarning)
				return
			}
			next.Milestone = milestone.IsChecked()
			next.Progress = float64(progress.Value()) / 100
			if next.Milestone {
				next.End = next.Start
			} else {
				next.End = next.End.AddDate(0, 0, 1)
			}
		}
		taskSnapshot := c.Tasks()
		var err error
		if c.readOnly || !c.interactive() {
			err = fmt.Errorf("%s", i18n.T("Gantt chart is read-only"))
		} else if editing {
			err = c.model.UpdateTask(next, c.cascadeDependencies)
		} else {
			err = c.model.AddTask(next)
		}
		if err != nil {
			c.lastError = err
			ShowToast(dlg, err.Error(), 4000, ToastWarning)
			return
		}
		// Use the actual pre-edit snapshot for exactly one notification/history.
		c.rebuildRows(false)
		c.SetSelectedTaskID(next.ID)
		c.commitNotification(taskSnapshot)
		dlg.onBtnClick(dialogResultKey(DialogOK))
	})
	sizeGanttTaskDialog(dlg, form)
	_ = showGanttDialog(dlg)
}

// FormLayout right-aligns its labels, so a fixed label width crops long
// translated text on the left. Measure the actual label/field fonts at dialog
// construction and reserve both readable labels and usable date inputs.
func sizeGanttTaskDialog(dlg *Dialog, form *FormLayout) {
	labelWidth, rowHeight := 120.0, 32.0
	for _, row := range form.rows {
		label, field := formRowHints(row)
		labelWidth = math.Max(labelWidth, label.Width+8)
		rowHeight = math.Max(rowHeight, math.Max(label.Height, field.Height)+8)
	}
	form.SetLabelWidth(math.Ceil(labelWidth))
	form.SetRowHeight(math.Ceil(rowHeight))
	fieldWidth := math.Max(280, Theme().Font.TextExtents("0000-00-00").Width+24)
	width := form.LabelWidth() + form.Spacing() + fieldWidth + 48
	height := form.SizeHints().Height + 20 + dialogButtonBarHeight + 16
	dlg.SetSize(math.Max(590, math.Ceil(width)), math.Max(470, math.Ceil(height)))
}
