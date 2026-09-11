package gui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/uk0/silk/core"
	"github.com/uk0/silk/geom"
)

func sdkDesignNode(factory string, x, y, w, h float64, children ...*core.TDoc) *core.TDoc {
	doc := core.NewTDoc()
	_ = doc.SetValue(factory)
	_ = doc.WriteAttr("bounds", geom.Rect{X: x * 25.4 / 96, Y: y * 25.4 / 96, Width: w * 25.4 / 96, Height: h * 25.4 / 96})
	if len(children) > 0 {
		sub := core.NewTDoc()
		sub.SetKey("children")
		for _, child := range children {
			sub.AddChild(child)
		}
		doc.AddChild(sub)
	}
	return doc
}

func TestSDKLoaderLocalizesDesignerBoundsOnceAndAppliesProperties(t *testing.T) {
	button := sdkDesignNode("gui.Button", 55, 48, 80, 26)
	props := core.NewTDoc()
	props.SetKey("props")
	_ = props.WriteAttr("文本", `"@ref:literal"`)
	_ = props.WriteAttr("可用", "false")
	button.AddChild(props)
	inner := sdkDesignNode("gui.HBox", 50, 42, 150, 80, button)
	outer := sdkDesignNode("gui.VBox", 40, 30, 200, 160, inner)
	doc := sdkDesignNode("form", 0, 0, 400, 280, outer)
	before := doc.String()
	form, err := LoadFormFromDoc(doc)
	if err != nil {
		t.Fatal(err)
	}
	box := form.Children()[0].(*VBox)
	nested := box.Children()[0].(*HBox)
	b := nested.Children()[0].(*Button)
	if box.X() != 40 || box.Y() != 30 || nested.X() != 10 || nested.Y() != 12 || b.X() != 5 || b.Y() != 6 {
		t.Fatalf("nested absolute origins applied as local: box=%v inner=%v button=%v", box.Bounds1(), nested.Bounds1(), b.Bounds1())
	}
	if b.Text() != "@ref:literal" || b.IsEnabled() {
		t.Fatalf("persisted props lost/evaluated: text=%q enabled=%v", b.Text(), b.IsEnabled())
	}
	if box.LayoutEnabled() || nested.LayoutEnabled() {
		t.Fatal("unmanaged designer containers unexpectedly reflowed")
	}
	if doc.String() != before {
		t.Fatal("SDK load mutated source/embedded document")
	}
}

type sdkNotAWidget struct{}

func init() { core.RegisterFactory("test.sdkNotAWidget", core.TypeOf((*sdkNotAWidget)(nil))) }

func TestSDKLoaderRejectsMalformedOrUnknownNestedWidgets(t *testing.T) {
	for _, kind := range []string{"missing.Widget", "test.sdkNotAWidget", ""} {
		doc := sdkDesignNode("form", 0, 0, 400, 280, sdkDesignNode("gui.VBox", 40, 30, 200, 160, sdkDesignNode(kind, 50, 40, 80, 26)))
		if form, err := LoadFormFromDoc(doc); err == nil || form != nil {
			t.Errorf("silently accepted nested factory %q: form=%v err=%v", kind, form, err)
		}
	}
	child := sdkDesignNode("gui.Button", 0, 0, 20, 20)
	_ = child.WriteAttr("bounds", "invalid rectangle")
	if form, err := LoadFormFromDoc(sdkDesignNode("form", 0, 0, 400, 280, child)); err == nil || form != nil {
		t.Fatal("malformed bounds were silently dropped")
	}
}

func TestSDKLoaderRegistersManagedGridCells(t *testing.T) {
	one := sdkDesignNode("gui.Button", 20, 20, 60, 24)
	two := sdkDesignNode("gui.Button", 90, 60, 60, 24)
	_ = one.WriteAttr("layout_row", 0)
	_ = one.WriteAttr("layout_column", 0)
	_ = two.WriteAttr("layout_row", 1)
	_ = two.WriteAttr("layout_column", 2)
	_ = two.WriteAttr("layout_column_span", 2)
	gridDoc := sdkDesignNode("gui.GridLayout", 10, 10, 320, 200, one, two)
	_ = gridDoc.WriteAttr("layout_managed", true)
	form, err := LoadFormFromDoc(sdkDesignNode("form", 0, 0, 500, 300, gridDoc))
	if err != nil {
		t.Fatal(err)
	}
	grid := form.Children()[0].(*GridLayout)
	if !grid.LayoutEnabled() || len(grid.cells) != 2 {
		t.Fatalf("managed grid missing cell registrations: enabled=%v cells=%v", grid.LayoutEnabled(), grid.cells)
	}
	cell := grid.cells[1]
	if cell.row != 1 || cell.col != 2 || cell.colSpan != 2 {
		t.Fatalf("sparse grid metadata lost: %+v", cell)
	}
	before := cell.widget.Bounds1()
	grid.SetSize(480, 260)
	grid.Layout()
	if cell.widget.Bounds1() == before {
		t.Fatal("registered child did not follow grid resize")
	}
	path := filepath.Join(t.TempDir(), "grid.silkui")
	if err := SaveForm(form, path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadForm(path)
	if err != nil {
		t.Fatal(err)
	}
	round := loaded.Children()[0].(*GridLayout)
	if !round.LayoutEnabled() || len(round.cells) != 2 || round.cells[1].row != 1 || round.cells[1].col != 2 || round.cells[1].colSpan != 2 {
		t.Fatal("SaveForm/LoadForm lost managed sparse cell metadata")
	}
}

func TestSDKSaveLoadNestedFormKeepsGeometryStateAndOwnedChildren(t *testing.T) {
	form := NewForm()
	form.SetSize(400, 280)
	box := NewVBox()
	box.SetLayoutEnabled(false)
	box.SetParent(form)
	box.SetBounds(40, 30, 200, 180)
	date := NewDatePicker()
	date.SetParent(box)
	date.SetBounds(5, 6, 150, 28)
	date.SetDate(2024, 1, 31)
	date.SetFormat("02/01/2006")
	slider := NewSlider(-300, -200)
	slider.SetValue(-250)
	slider.SetParent(box)
	slider.SetBounds(5, 42, 150, 24)
	combo := NewComboBox()
	combo.SetParent(box)
	combo.SetBounds(5, 76, 150, 28)
	combo.SetEnabled(false)
	combo.SetItems([]string{"", "引号 \"第二项\"\n下一行", "@ref:literal"})
	combo.SetActiveIndex(1)
	path := filepath.Join(t.TempDir(), "nested.silkui")
	// SDK geometry is logical pixels, independent of the monitor on which
	// code generation or saving happens.
	oldDPI := screenDpi
	screenDpi = 144
	t.Cleanup(func() { screenDpi = oldDPI })
	if err := SaveForm(form, path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadForm(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Width() != 400 || loaded.Height() != 280 {
		t.Fatalf("SDK save/load depends on monitor DPI: %v", loaded.Bounds1())
	}
	b := loaded.Children()[0].(*VBox)
	if b.X() != 40 || b.Y() != 30 || len(b.Children()) != 3 {
		t.Fatalf("nested geometry/tree drift: %v children=%d", b.Bounds1(), len(b.Children()))
	}
	d := b.Children()[0].(*DatePicker)
	if d.X() != 5 || d.Y() != 6 || d.Year() != 2024 || d.Month() != 1 || d.Day() != 31 || d.Format() != date.Format() {
		t.Fatalf("date geometry/state drift: %v %d-%02d-%02d", d.Bounds1(), d.Year(), d.Month(), d.Day())
	}
	s := b.Children()[1].(*Slider)
	if s.Min() != -300 || s.Max() != -200 || s.Value() != -250 {
		t.Fatalf("range clamped during restore: %v/%v/%v", s.Min(), s.Max(), s.Value())
	}
	c := b.Children()[2].(*ComboBox)
	if c.Items()[1] != combo.Items()[1] || c.ActiveIndex() != 1 || c.Text() != combo.Text() {
		t.Fatal("SDK save/load lost combo items, escapes or selection")
	}
	if c.IsEnabled() || len(c.Children()) != len(combo.Children()) {
		t.Fatalf("combo state or owned widgets duplicated: enabled=%v children=%d want=%d", c.IsEnabled(), len(c.Children()), len(combo.Children()))
	}
}

func TestSDKLoaderKeepsLegacyScalarAliasesAndLocalBoundsOptIn(t *testing.T) {
	root := core.NewTDoc()
	_ = root.WriteAttr("form_title", "Aliases")
	_ = root.WriteAttr("form_w", 50.8)
	_ = root.WriteAttr("form_h", 25.4)
	child := core.NewTDoc()
	child.SetKey("widget")
	_ = child.WriteAttr("factory", "gui.Button")
	_ = child.WriteAttr("x", 2.54)
	_ = child.WriteAttr("y", 5.08)
	_ = child.WriteAttr("w", 25.4)
	_ = child.WriteAttr("h", 6.35)
	_ = child.WriteAttr("text", "")
	root.AddChild(child)
	before := root.String()
	form, err := LoadFormFromDoc(root)
	if err != nil {
		t.Fatal(err)
	}
	if form.Title() != "Aliases" || form.Width() != 192 || form.Height() != 96 {
		t.Fatalf("root aliases drifted: %q %v", form.Title(), form.Bounds1())
	}
	b := form.Children()[0].(*Button)
	if b.X() != 10 || b.Y() != 19 || b.Width() != 96 || b.Height() != 24 {
		t.Fatalf("scalar millimetre alias drifted: %v", b.Bounds1())
	}
	if root.String() != before {
		t.Fatal("root factory fallback mutated input")
	}
	local := sdkDesignNode("form", 0, 0, 400, 280, sdkDesignNode("gui.VBox", 40, 30, 200, 160, sdkDesignNode("gui.Button", 5, 6, 80, 26)))
	_ = local.WriteAttr("bounds_space", "local")
	f, err := LoadFormFromDoc(local)
	if err != nil {
		t.Fatal(err)
	}
	if b := f.Children()[0].Children()[0]; b.X() != 5 || b.Y() != 6 {
		t.Fatalf("explicit old SDK local bounds not honored: %v", b.Bounds1())
	}
	if strings.Contains(local.String(), "load-0") {
		t.Fatal("private validation IDs leaked to document")
	}
}
