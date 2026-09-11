package ged

import (
	"testing"

	"github.com/uk0/silk/gui"
)

func contextButton(menu *gui.Menu, text string) *gui.Button {
	for _, child := range menu.Children() {
		b, ok := child.(*gui.Button)
		if !ok {
			continue
		}
		if b.Text() == text {
			return b
		}
		if sub, ok := b.SubPopup().(*gui.Menu); ok {
			if found := contextButton(sub, text); found != nil {
				return found
			}
		}
	}
	return nil
}

func TestContextPanelsAndSingleSelectionGuards(t *testing.T) {
	v, widgets := layoutFixture(t, LayoutGrid)
	v.Selection().Clear()
	v.Selection().Add(widgets[0])
	properties, code := 0, 0
	v.SetContextPanelCallbacks(func() { properties++ }, func() { code++ })
	x, y := v.MapFromScene(15, 15)
	m := v.buildContextMenu(x, y)
	for _, text := range []string{"属性...", "查看代码"} {
		b := contextButton(m, text)
		if b == nil {
			t.Fatal("missing", text)
		}
		b.Action().Trigger(b)
	}
	if properties != 1 || code != 1 {
		t.Fatal("selected widget did not activate host panels")
	}
	for _, text := range []string{"水平分布", "垂直分布", "垂直布局 (VBox)", "水平布局 (HBox)", "网格布局 (Grid)", "表单布局 (Form)", "解散布局 (Break)"} {
		b := contextButton(m, text)
		if b == nil || b.IsEnabled() {
			t.Fatal("unavailable command is not disabled:", text)
		}
	}
	if !contextButton(m, "左对齐").IsEnabled() {
		t.Fatal("single-item alignment to container incorrectly disabled")
	}
}

func TestContextGridActionCreatesRealLayout(t *testing.T) {
	v, _ := layoutFixture(t, LayoutGrid)
	x, y := v.MapFromScene(15, 15)
	b := contextButton(v.buildContextMenu(x, y), "网格布局 (Grid)")
	if b == nil || !b.IsEnabled() {
		t.Fatal("grid context action unavailable")
	}
	b.Action().Trigger(b)
	children := v.Scene().Children()
	if len(children) != 1 {
		t.Fatalf("children=%d", len(children))
	}
	container, ok := children[0].(*FakeWidget)
	if !ok || container.WidgetFactoryName() != string(LayoutGrid) || len(container.Children()) != 4 {
		t.Fatal("context command did not create the requested nested grid")
	}
}
