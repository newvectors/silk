package gui

import (
	"github.com/uk0/silk/core"
	"github.com/uk0/silk/paint"
	"math"
)

func init() {
	core.RegisterFactory("gui.ToolBar", core.TypeOf((*ToolBar)(nil)))
}

// ToolBar is a horizontal or vertical bar for tool buttons, separators,
// and arbitrary widgets. Similar to QToolBar.
type ToolBar struct {
	Widget
	items       []IWidget
	orientation int     // 0=horizontal, 1=vertical
	iconSize    int     // icon display size in pixels
	spacing     float64 // gap between items
	overflow    []IWidget
	overflowBtn *Button
}

func NewToolBar() *ToolBar {
	p := new(ToolBar)
	p.Init(p)
	p.iconSize = int(Theme().IconSize)
	p.spacing = Theme().Spacing
	return p
}

func (this *ToolBar) EnumProperties(list core.IPropertyList) {
	list.AddProperty("图标大小", this.IconSize, this.SetIconSize)
	list.AddProperty("间距", this.Spacing, this.SetSpacing)
}

// AddAction creates a tool button and appends it to the toolbar.
func (this *ToolBar) AddAction(text string, icon paint.Icon, callback func()) *Button {
	btn := NewButton1(text, icon)
	btn.Action().BindFunc0(callback)
	this.AddWidget(btn)
	return btn
}

// AddActionButton adds an existing action as a tool button.
func (this *ToolBar) AddActionButton(a IAction) *Button {
	btn := NewActionButton(a)
	this.AddWidget(btn)
	return btn
}

// AddSeparator inserts a visual separator into the toolbar.
func (this *ToolBar) AddSeparator() *Separator {
	sep := NewSeparator()
	this.AddWidget(sep)
	return sep
}

// AddWidget appends any widget (e.g. ComboBox, Edit) to the toolbar.
func (this *ToolBar) AddWidget(iw IWidget) {
	iw.SetParent(this)
	this.items = append(this.items, iw)
}

// RemoveWidget removes a widget from the toolbar.
func (this *ToolBar) RemoveWidget(iw IWidget) {
	iw.SetParent(nil)
	for i, v := range this.items {
		if v == iw {
			copy(this.items[i:], this.items[i+1:])
			this.items[len(this.items)-1] = nil
			this.items = this.items[:len(this.items)-1]
			break
		}
	}
	if i, ok := this.Self().(ILayout); ok {
		i.Layout()
	}
}

// Items returns all items in the toolbar.
func (this *ToolBar) Items() []IWidget {
	return this.items
}

func (this *ToolBar) IconSize() int {
	return this.iconSize
}

func (this *ToolBar) SetIconSize(size int) {
	if this.iconSize == size {
		return
	}
	this.iconSize = size
	if i, ok := this.Self().(ILayout); ok {
		i.Layout()
	}
	this.Self().Update()
}

func (this *ToolBar) Spacing() float64 {
	return this.spacing
}

func (this *ToolBar) SetSpacing(s float64) {
	if this.spacing == s {
		return
	}
	this.spacing = s
	if i, ok := this.Self().(ILayout); ok {
		i.Layout()
	}
	this.Self().Update()
}

func (this *ToolBar) Orientation() int {
	return this.orientation
}

func (this *ToolBar) SetOrientation(o int) {
	if this.orientation == o {
		return
	}
	this.orientation = o
	if i, ok := this.Self().(ILayout); ok {
		i.Layout()
	}
	this.Self().Update()
}

func (this *ToolBar) IsVertical() bool {
	return this.orientation == 1
}

// Layout arranges children horizontally or vertically.
func (this *ToolBar) Layout() {
	if this.IsVertical() {
		this.layoutVertical()
	} else {
		this.layoutHorizontal()
	}
}

func (this *ToolBar) layoutHorizontal() {
	t := Theme()
	m := t.ButtonMargin
	w, h := this.Self().Size()

	// Measure every item before placing any: where the run has to be cut can
	// only be known once the whole run is known.
	type itemGeom struct {
		y, w, h float64
	}
	geom := make([]itemGeom, len(this.items))
	for i, item := range this.items {
		hints := item.SizeHints()
		// separators get a small fixed width
		if _, ok := item.(*Separator); ok {
			sz := t.SeparatorSize + 6
			geom[i] = itemGeom{4, sz, h - 8}
			continue
		}
		// buttons: size depends on whether they have an icon, text, or both
		if btn, ok := item.(*Button); ok {
			hasIcon := btn.Icon() != nil && !btn.Icon().IsAir()
			text := btn.Text()

			var btnW, btnH float64
			if hasIcon && text == "" {
				// Icon-only button (standard Qt Creator toolbar style)
				btnW = float64(this.iconSize) + m.L + m.R + 4
				btnH = float64(this.iconSize) + m.T + m.B + 4
			} else if hasIcon && text != "" {
				// Icon + text button
				textW := t.Font.TextExtents(text).Width
				btnW = float64(this.iconSize) + textW + m.L + m.R + 10
				btnH = float64(this.iconSize) + m.T + m.B + 4
			} else {
				// Text-only button — size by text content
				textW := t.Font.TextExtents(text).Width
				fe := t.Font.FontExtents()
				btnW = textW + m.L + m.R + 12
				btnH = fe.Height + m.T + m.B + 6
			}
			y := (h - btnH) * 0.5
			if y < 0 {
				y = 0
			}
			geom[i] = itemGeom{y, btnW, btnH}
			continue
		}
		// custom widgets use their SizeHints
		iw := hints.Width
		ih := hints.Height
		if (hints.Policy & (GrowVertical | ExpandVertical)) != 0 {
			ih = h
		}
		y := (h - ih) * 0.5
		if y < 0 {
			y = 0
		}
		geom[i] = itemGeom{y, iw, ih}
	}

	// The chevron takes an icon-only button's width; reserving it (plus the
	// spacing before it) is what keeps the last placed item from ending up
	// underneath it.
	ovW := float64(this.iconSize) + m.L + m.R + 4
	avail := w - m.R

	total := m.L
	for i, g := range geom {
		if i > 0 {
			total += this.spacing
		}
		total += g.w
	}

	shown := len(geom)
	if total > avail {
		limit := avail - ovW - this.spacing
		x := m.L
		shown = 0
		for i, g := range geom {
			if i > 0 {
				x += this.spacing
			}
			if x+g.w > limit {
				break
			}
			x += g.w
			shown = i + 1
		}
	}

	this.overflow = nil
	x := m.L
	for i, item := range this.items {
		if i >= shown {
			// Off the end of the bar: reachable through the chevron menu only.
			item.SetVisible(false)
			this.overflow = append(this.overflow, item)
			continue
		}
		item.SetVisible(true)
		item.SetBounds(x, geom[i].y, geom[i].w, geom[i].h)
		x += geom[i].w + this.spacing
	}

	if len(this.overflow) == 0 {
		if this.overflowBtn != nil {
			this.overflowBtn.SetVisible(false)
		}
		return
	}
	btn := this.overflowButton()
	btnH := float64(this.iconSize) + m.T + m.B + 4
	y := (h - btnH) * 0.5
	if y < 0 {
		y = 0
	}
	btn.SetVisible(true)
	btn.SetBounds(avail-ovW, y, ovW, btnH)
}

// OverflowItems returns the items the last horizontal layout could not fit,
// in their original order. They are hidden and reachable through the chevron.
func (this *ToolBar) OverflowItems() []IWidget {
	return this.overflow
}

// overflowButton returns the chevron, creating it on first use. It is a child
// of the toolbar so it draws and takes clicks like any other widget, but it is
// deliberately kept out of this.items: it is not part of the run it cuts.
func (this *ToolBar) overflowButton() *Button {
	if this.overflowBtn == nil {
		btn := NewButton1("»", nil)
		btn.SetParent(this)
		btn.Action().BindFunc0(func() {
			this.showOverflow()
		})
		this.overflowBtn = btn
	}
	return this.overflowBtn
}

// showToolBarPopup pops the overflow menu up at a global position. It is a
// variable rather than a direct ShowAsPopup call because ShowAsPopup creates an
// OS window, which AppKit allows on the main thread alone; tests replace it to
// drive the chevron's real action without one.
var showToolBarPopup = func(menu *Menu, xg, yg float64) {
	menu.ShowAsPopup(xg, yg, true)
}

// showOverflow builds a menu mirroring the hidden items and drops it under the
// chevron. The menu entries trigger the original items' actions, so an action
// does the same thing whether the window was wide enough for it or not.
func (this *ToolBar) showOverflow() {
	menu := NewPopupMenu()
	for _, item := range this.overflow {
		switch v := item.(type) {
		case *Button:
			src := v
			menu.AddButton1(src.Text(), src.Icon()).Action().BindFunc0(func() {
				src.Action().Trigger(src)
			})
		case *Separator:
			menu.AddSeparator()
		}
	}
	if len(menu.Items()) == 0 {
		return
	}
	btn := this.overflowButton()
	xg, yg := btn.MapToGlobal(0, 0)
	_, bh := btn.Size()
	showToolBarPopup(menu, xg, yg+bh)
}

func (this *ToolBar) layoutVertical() {
	t := Theme()
	m := t.ButtonMargin
	w, h := this.Self().Size()
	_ = h

	y := m.T
	for _, item := range this.items {
		hints := item.SizeHints()
		if _, ok := item.(*Separator); ok {
			sz := t.SeparatorSize
			item.SetBounds(0, y, w, sz)
			y += sz + this.spacing
			continue
		}
		if _, ok := item.(*Button); ok {
			btnW := float64(this.iconSize) + m.L + m.R
			btnH := float64(this.iconSize) + m.T + m.B
			x := (w - btnW) * 0.5
			if x < 0 {
				x = 0
			}
			item.SetBounds(x, y, btnW, btnH)
			y += btnH + this.spacing
			continue
		}
		iw := hints.Width
		ih := hints.Height
		if (hints.Policy & (GrowHorizontal | ExpandHorizontal)) != 0 {
			iw = w
		}
		x := (w - iw) * 0.5
		if x < 0 {
			x = 0
		}
		item.SetBounds(x, y, iw, ih)
		y += ih + this.spacing
	}
}

func (this *ToolBar) Draw(g paint.Painter) {
	t := Theme()
	w, h := this.Self().Size()

	// background: use MenuBGColor for subtle differentiation from form
	g.Rectangle(0, 0, w, h)
	g.SetBrush1(t.MenuBGColor)
	g.Fill()

	// bottom border (horizontal) or right border (vertical)
	if this.IsVertical() {
		g.Line(w-0.5, 0, w-0.5, h)
	} else {
		g.Line(0, h-0.5, w, h-0.5)
	}
	g.SetPen1(t.BorderColor, 1)
	g.Stroke()
}

func (this *ToolBar) SizeHints() SizeHints {
	t := Theme()
	m := t.ButtonMargin
	if this.IsVertical() {
		var maxW, totalH float64
		for i, item := range this.items {
			if _, ok := item.(*Separator); ok {
				totalH += t.SeparatorSize
			} else if _, ok := item.(*Button); ok {
				bw := float64(this.iconSize) + m.L + m.R
				bh := float64(this.iconSize) + m.T + m.B
				maxW = math.Max(maxW, bw)
				totalH += bh
			} else {
				hints := item.SizeHints()
				maxW = math.Max(maxW, hints.Width)
				totalH += hints.Height
			}
			if i > 0 {
				totalH += this.spacing
			}
		}
		return SizeHints{Width: maxW, Height: totalH + m.T + m.B, Policy: GrowVertical}
	}

	// horizontal
	var totalW, maxH float64
	for i, item := range this.items {
		if _, ok := item.(*Separator); ok {
			totalW += t.SeparatorSize
		} else if _, ok := item.(*Button); ok {
			bw := float64(this.iconSize) + m.L + m.R
			bh := float64(this.iconSize) + m.T + m.B
			totalW += bw
			maxH = math.Max(maxH, bh)
		} else {
			hints := item.SizeHints()
			totalW += hints.Width
			maxH = math.Max(maxH, hints.Height)
		}
		if i > 0 {
			totalW += this.spacing
		}
	}
	return SizeHints{Width: totalW + m.L + m.R, Height: maxH, Policy: GrowHorizontal}
}

func (this *ToolBar) OnIdle() {
	for _, v := range this.items {
		if v.IsVisible() {
			if im, ok := v.(interface {
				OnIdle()
			}); ok {
				im.OnIdle()
			}
		}
	}
}
