package gui

import (
	"testing"

	"github.com/uk0/silk/a11y"
)

func TestAccessibilityWalkIncludesRealWidgetChildren(t *testing.T) {
	root := NewFrame()
	box := NewVBox()
	box.SetParent(root)
	button := NewButton1("Menu action", nil)
	box.AddWidget(button)
	hidden := NewLabel("hidden")
	box.AddWidget(hidden)
	hidden.SetVisible(false)
	tree := a11y.Walk(root)
	var names []string
	var walk func(*a11y.Node)
	walk = func(n *a11y.Node) {
		if n == nil {
			return
		}
		names = append(names, n.Name)
		for _, child := range n.Children {
			walk(child)
		}
	}
	walk(tree)
	found := false
	for _, name := range names {
		if name == "hidden" {
			t.Fatal("hidden child exposed in visible tree")
		}
		if name == "Menu action" {
			found = true
		}
	}
	if !found {
		t.Fatalf("real child missing from tree: %q", names)
	}
}
