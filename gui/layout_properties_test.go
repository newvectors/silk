package gui

import "testing"

func TestLayoutInsetsDoNotReflectOutsideSmallContainer(t *testing.T) {
	cases := []struct {
		name      string
		newWidget func() IWidget
	}{{"HBox", func() IWidget { return NewHBox() }}, {"VBox", func() IWidget { return NewVBox() }}, {"Grid", func() IWidget { return NewGridLayout() }}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			container := tc.newWidget()
			container.(interface{ SetPadding(Padding) }).SetPadding(Padding{L: 20, R: 20, T: 20, B: 20})
			container.SetSize(10, 10)
			child := NewButton1("Test", nil)
			if grid, ok := container.(*GridLayout); ok {
				grid.AddWidget(child, 0, 0)
			} else {
				container.(interface{ AddWidget(IWidget) }).AddWidget(child)
			}
			container.(ILayout).Layout()
			if child.X() < 0 || child.Y() < 0 || child.X() > 10 || child.Y() > 10 {
				t.Fatalf("reflected inset: (%g,%g)", child.X(), child.Y())
			}
		})
	}
}

func TestFormLayoutDesignerUsesExistingLabels(t *testing.T) {
	f := NewFormLayout()
	f.SetSize(300, 100)
	label := NewLabel("Name")
	edit := NewEdit()
	f.AddLayoutWidget(label, 0, 0, 1, 1)
	f.AddLayoutWidget(edit, 0, 1, 1, 1)
	f.Layout()
	if len(f.Children()) != 2 {
		t.Fatalf("duplicate label: %d", len(f.Children()))
	}
	if edit.X() < label.X()+label.Width() {
		t.Fatal("field overlaps label")
	}
	f.SetSize(30, 20)
	if label.Width() > 30 || edit.Width() < 0 {
		t.Fatal("small form dimensions invalid")
	}
}

func TestLayoutEnabledDefaultsAndExplicitFreeGeometry(t *testing.T) {
	for _, tc := range []struct {
		name      string
		newWidget func() IWidget
	}{
		{"HBox", func() IWidget { return NewHBox() }}, {"VBox", func() IWidget { return NewVBox() }},
		{"Grid", func() IWidget { return NewGridLayout() }}, {"Form", func() IWidget { return NewFormLayout() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			widget := tc.newWidget()
			manager := widget.(interface {
				LayoutEnabled() bool
				SetLayoutEnabled(bool)
				Layout()
			})
			if !manager.LayoutEnabled() {
				t.Fatal("hand-written runtime layout must default enabled")
			}
			manager.SetLayoutEnabled(false)
			widget.SetSize(100, 100)
			child := NewButton1("Test", nil)
			child.SetBounds(12, 14, 30, 18)
			child.SetParent(widget)
			widget.SetSize(200, 180)
			manager.Layout()
			if child.X() != 12 || child.Y() != 14 || child.Width() != 30 || child.Height() != 18 {
				t.Fatal("explicit free geometry was rearranged")
			}
		})
	}
}
