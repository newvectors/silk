package ged

import (
	"github.com/uk0/silk/geom"
	"github.com/uk0/silk/graph"
	"github.com/uk0/silk/gui"
)

// SetContextPanelCallbacks lets the host activate its actual dock panels rather
// than merely changing a selection that is already selected by right-click.
func (v *GedView) SetContextPanelCallbacks(properties, code func()) {
	v.showPropertiesPanel, v.showCodePanel = properties, code
}

func (v *GedView) requestCodePanel() {
	if v.showCodePanel != nil {
		v.showCodePanel()
	} else if ShowCodePanelCallback != nil {
		ShowCodePanelCallback()
	}
}

func (v *GedView) gateContextAlignment(menu *gui.Menu) {
	modes := []AlignMode{AlignLeft, AlignRight, AlignHCenter, AlignTop, AlignBottom, AlignVCenter, AlignCenter, DistributeH, DistributeV}
	index := 0
	for _, child := range menu.Children() {
		if b, ok := child.(*gui.Button); ok && index < len(modes) {
			b.Action().SetEnabled(v.canContextAlign(modes[index]))
			index++
		}
	}
}

func (v *GedView) canContextAlign(mode AlignMode) bool {
	items := v.Selection().ItemList()
	set := make(map[graph.IItem]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	var movers []graph.IItem
	var rects []geom.Rect
	free := 0
	for _, item := range items {
		if hasSelectedAncestor(item, set) {
			continue
		}
		movers = append(movers, item)
		rects = append(rects, item.Bounds1())
		if !item.IsLockPos() {
			free++
		}
	}
	if free == 0 {
		return false
	}
	if mode == DistributeH || mode == DistributeV {
		if len(movers) < 3 {
			return false
		}
		aligned, _ := alignRects(rects, mode)
		for i, item := range movers {
			if item.IsLockPos() && (item.Bounds1().X != aligned[i].X || item.Bounds1().Y != aligned[i].Y) {
				return false
			}
		}
		return true
	}
	if mode == AlignCenter || len(items) == 1 {
		_, ok := alignFrame(items)
		return ok
	}
	if mode == AlignHCenter || mode == AlignVCenter {
		return free >= 2
	}
	return len(movers) >= 2
}
