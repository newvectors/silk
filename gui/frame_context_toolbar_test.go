package gui

import "testing"

func TestFrameContextToolbarReservesDockSpace(t *testing.T) {
	f := NewFrame()
	f.SetSize(800, 600)
	primary, context := NewToolBar(), NewToolBar()
	primary.AddAction("Design", nil, func() {})
	context.AddAction("Grid Layout", LoadIcon("layout-grid"), func() {}).SetTextVisible(true)
	f.SetToolBar(primary)
	f.SetContextToolBar(context)
	f.Layout()
	dockTop := func() float64 { _, y, _, _ := f.root_.Bounds(); return y }
	if context.Y() < primary.Y()+primary.Height() || dockTop() < context.Y()+context.Height() {
		t.Fatal("toolbar overlaps another row or the dock")
	}
	oldTop := dockTop()
	context.SetVisible(false)
	f.Layout()
	if dockTop() != oldTop-context.Height() {
		t.Fatal("hidden context row still reserves space")
	}
	replacement := NewToolBar()
	f.SetContextToolBar(replacement)
	if context.Parent() != nil || f.ContextToolBar() != replacement {
		t.Fatal("replacing context row left an overlay child")
	}
}
