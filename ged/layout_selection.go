package ged

import (
	"sort"

	"github.com/uk0/silk/geom"
	"github.com/uk0/silk/graph"
)

// LayoutKind describes a real runtime geometry manager, not an alignment action.
type LayoutKind string

const (
	LayoutHorizontal LayoutKind = "gui.HBox"
	LayoutVertical   LayoutKind = "gui.VBox"
	LayoutGrid       LayoutKind = "gui.GridLayout"
	LayoutForm       LayoutKind = "gui.FormLayout"
)

func isDesignerLayoutFactory(name string) bool {
	switch LayoutKind(name) {
	case LayoutHorizontal, LayoutVertical, LayoutGrid, LayoutForm:
		return true
	}
	return false
}

func (v *GedView) layoutCandidates() []*FakeWidget {
	raw := v.Selection().ItemList()
	selected := make(map[graph.IItem]bool, len(raw))
	for _, it := range raw {
		if _, ok := it.(*FakeWidget); ok {
			selected[it] = true
		}
	}
	var items []*FakeWidget
	for _, it := range raw {
		f, ok := it.(*FakeWidget)
		if !ok || f.IsLockPos() || hasSelectedAncestor(it, selected) {
			continue
		}
		items = append(items, f)
	}
	return items
}

func (v *GedView) CanLayoutSelection(kind LayoutKind) bool {
	if !isDesignerLayoutFactory(string(kind)) {
		return false
	}
	items := v.layoutCandidates()
	if len(items) < 2 {
		return false
	}
	// Group only siblings: stealing children out of unrelated containers is not
	// a layout operation and would make the generated hierarchy surprising.
	parent := items[0].Parent()
	for _, f := range items[1:] {
		if f.Parent() != parent {
			return false
		}
	}
	return true
}

func (v *GedView) CanBreakLayout() bool {
	for _, it := range v.Selection().ItemList() {
		if f, ok := it.(*FakeWidget); ok && isDesignerLayoutFactory(f.WidgetFactoryName()) && !f.IsLockPos() && f.HasChildren() && f.Scene() == v.Scene() {
			return true
		}
	}
	return false
}

type layoutSnapshot struct {
	item      *FakeWidget
	bounds    geom.Rect
	placement layoutPlacement
}
type layoutSelectionCommand struct {
	view              *GedView
	originalSelection []graph.IItem
	move              *graph.ReparentCommand
	container         *FakeWidget
	parentLayout      *FakeWidget
	before            []layoutSnapshot
	placements        []layoutPlacement
}

func (c *layoutSelectionCommand) Text() string { return "Lay Out" }
func (c *layoutSelectionCommand) Redo() {
	c.move.Redo()
	for i, s := range c.before {
		s.item.layoutCell = c.placements[i]
	}
	c.container.layoutManaged = true
	c.container.Layout()
	setLayoutSelection(c.view, []graph.IItem{c.container})
	if c.parentLayout != nil && c.parentLayout.layoutManaged {
		c.parentLayout.Layout()
	}
}
func (c *layoutSelectionCommand) Undo() {
	c.move.Undo()
	for _, s := range c.before {
		s.item.layoutCell = s.placement
		s.item.SetBounds1(s.bounds)
	}
	if c.parentLayout != nil && c.parentLayout.layoutManaged {
		c.parentLayout.Layout()
	}
	setLayoutSelection(c.view, c.originalSelection)
}

// LayoutSelection wraps sibling widgets in one undoable layout. Grid and form
// placement is persisted explicitly, so preview and both code emitters agree.
func (v *GedView) LayoutSelection(kind LayoutKind) bool {
	if !v.CanLayoutSelection(kind) {
		return false
	}
	items := v.layoutCandidates()
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].Bounds1(), items[j].Bounds1()
		if kind == LayoutHorizontal {
			return a.X < b.X
		}
		if kind == LayoutVertical {
			return a.Y < b.Y
		}
		// Equal top edges share a visual row; use a strict total ordering.
		if a.Y == b.Y {
			return a.X < b.X
		}
		return a.Y < b.Y
	})
	rects := make([]geom.Rect, len(items))
	for i, f := range items {
		rects[i] = f.Bounds1()
	}
	box := boundingBoxOf(rects)
	container, err := NewFakeWidgetFromFactory(string(kind))
	if err != nil {
		return false
	}
	container.SetBounds1(box)
	cmd := &layoutSelectionCommand{view: v, originalSelection: append([]graph.IItem(nil), v.Selection().ItemList()...), move: graph.NewReparentCommand("Lay Out"), container: container}
	cmd.parentLayout, _ = items[0].Parent().(*FakeWidget)
	cmd.move.Add(container, nil, items[0].Parent())
	row, col := 0, 0
	for i, f := range items {
		p := layoutPlacement{RowSpan: 1, ColumnSpan: 1, Valid: true}
		switch kind {
		case LayoutHorizontal:
			p.Column = i
		case LayoutVertical:
			p.Row = i
		case LayoutForm:
			p.Row, p.Column = i/2, i%2
		case LayoutGrid:
			if i > 0 && f.Y() >= items[i-1].Y()+items[i-1].Height() {
				row++
				col = 0
			}
			p.Row, p.Column = row, col
			col++
		}
		cmd.before = append(cmd.before, layoutSnapshot{f, f.Bounds1(), f.layoutCell})
		cmd.placements = append(cmd.placements, p)
		cmd.move.AddMoved(f, f.Parent(), container, f.X(), f.Y())
	}
	v.Scene().PushCommand(cmd)
	sel := v.Selection()
	sel.Clear()
	sel.Add(container)
	v.refreshPropertyValues()
	v.GedScene().NotifyDesignChanged()
	v.Self().Update()
	return true
}

// The command, rather than a one-time toolbar callback, owns selection state:
// undo/redo can run through the keyboard, history pane, or raw scene stack.
func setLayoutSelection(view *GedView, items []graph.IItem) {
	if view == nil {
		return
	}
	selection := view.Selection()
	selection.Clear()
	for _, item := range items {
		if item != nil && item.Scene() == view.Scene() {
			selection.Add(item)
		}
	}
	view.refreshPropertyValues()
	view.Self().Update()
}

type breakLayoutCommand struct {
	move            *graph.ReparentCommand
	view            *GedView
	original, freed []graph.IItem
}

func (c *breakLayoutCommand) Text() string { return "Break Layout" }
func (c *breakLayoutCommand) Redo()        { c.move.Redo(); setLayoutSelection(c.view, c.freed) }
func (c *breakLayoutCommand) Undo()        { c.move.Undo(); setLayoutSelection(c.view, c.original) }

// BreakLayout dissolves only layout containers and preserves both geometry
// and selection through undo/redo. Ordinary Card/GroupBox widgets are retained.
func (v *GedView) BreakLayout() bool {
	if !v.CanBreakLayout() {
		return false
	}
	cmd := &breakLayoutCommand{move: graph.NewReparentCommand("Break Layout"), view: v, original: append([]graph.IItem(nil), v.Selection().ItemList()...)}
	for _, item := range cmd.original {
		box, ok := item.(*FakeWidget)
		if !ok || !isDesignerLayoutFactory(box.factoryName) || box.IsLockPos() || !box.HasChildren() || box.Parent() == nil {
			continue
		}
		for _, child := range box.Children() {
			x, y := graph.TranslateBetweenParents(box, box.Parent(), child.X(), child.Y())
			cmd.move.AddMoved(child, box, box.Parent(), x, y)
			cmd.freed = append(cmd.freed, child)
		}
		cmd.move.Add(box, box.Parent(), nil)
	}
	if cmd.move.Count() == 0 {
		return false
	}
	v.Scene().PushCommand(cmd)
	v.GedScene().NotifyDesignChanged()
	return true
}
