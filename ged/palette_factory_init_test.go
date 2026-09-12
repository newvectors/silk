package ged

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"testing"

	"github.com/uk0/silk/gui"
	"github.com/uk0/silk/paint"
)

// The designer never calls a widget's NewXxx constructor. It asks the factory
// registry for the type and the factory constructs it by reflection and calls
// Init — nothing else. So every widget whose setup lives in its constructor
// instead of in Init reaches the canvas half-built: gui.Table came out with a
// nil scrollArea and panicked in Layout on the very first SetBounds, and the
// quieter members of the family come out with a sentinel at 0 where the
// constructor would have written -1 (row 0 selected, column 0 being resized).
//
// The three tests below cover that family from three sides: the whole palette
// is driven through the real drop path (a crash), a designer-built widget is
// compared field by field with the one its constructor returns (a divergence
// between the two paths), and a designer-built Table is checked against the
// state it is supposed to start in (a divergence both paths share).

// TestPaletteWidgetsSurviveTheCanvas drops every widget the palette offers onto
// a real GedView and then handles it the way the canvas does — a drag that
// moves and resizes it, then a repaint. This is the guard that would have
// caught the gui.Table crash: OnDrop -> SetBounds -> Item.OnMove ->
// FakeWidget.Layout -> Table.Layout dereferenced the nil scrollArea.
//
// It walks paletteNames(), which reads categoryDefs, so a widget added to the
// palette later is covered without touching this file.
func TestPaletteWidgetsSurviveTheCanvas(t *testing.T) {
	for _, name := range paletteNames() {
		dropOnCanvas(t, name)
	}
}

// dropOnCanvas runs one widget's drop-and-handle cycle, turning a panic into a
// failure that names the widget and keeps the loop going, so one broken widget
// reports every broken widget instead of only the first.
func dropOnCanvas(t *testing.T, name string) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s: dropping it on the designer canvas panicked: %v\n%s", name, r, debug.Stack())
		}
	}()

	view := NewGedView()
	scene := view.GedScene()
	// The palette now includes large timeline controls. Exercise creation on
	// a form large enough for each default; oversized-drop rejection is tested
	// separately by the drop-plan tests, not a factory initialization failure.
	width, height := defaultSizeForWidget(name)
	scene.SetSize(max(scene.Width(), width+20), max(scene.Height(), height+20))

	ctx := &dropCtx{pa: gui.DndCopy, payload: name}
	ctx.SetAction(gui.DndIgnore)
	view.OnDrop(20, 20, ctx)

	kids := scene.Children()
	if len(kids) != 1 {
		t.Errorf("%s: scene children after the drop = %d, want 1", name, len(kids))
		return
	}
	fw, ok := kids[0].(*FakeWidget)
	if !ok {
		t.Errorf("%s: the drop put a %T on the canvas, want *FakeWidget", name, kids[0])
		return
	}

	// A canvas drag: moving and resizing the item re-runs the widget's Layout.
	fw.SetBounds(30, 25, 60, 40)

	// Repaint. The canvas draws through FakeWidget.DrawSelf, which recovers
	// around the widget's Draw and paints an error cross instead, so a Draw
	// that blows up on half-built state would never reach a test through that
	// path — call the widget's own Draw, the way the gui package's Draw tests
	// do, on a real painter.
	w := fw.Widget()
	pm := paint.NewPixmap(int(w.Width())+1, int(w.Height())+1)
	w.Draw(pm.NewPainter())
}

// factoryParityCases pairs a palette factory name with the constructor that is
// supposed to be equivalent to it. Add a row when a widget is found to set
// state its Init does not; the comparison itself is generic, so a row is all
// it takes. If NewFakeWidgetFromFactory gives the widget default content
// (setDefaultContent), the constructor side of the row has to apply the same
// content, or the difference reported is that, not the initialisation.
var factoryParityCases = []struct {
	factory  string
	ctorName string
	ctor     func() gui.IWidget
}{
	{"gui.Table", "gui.NewTable", func() gui.IWidget { return gui.NewTable() }},
}

// TestFactoryBuiltWidgetMatchesItsConstructor compares a widget built the way
// the designer builds it against the same widget built by its constructor,
// field by field. It catches the regression that put the crash here in the
// first place: setup added to NewXxx and not to Init, which leaves the
// designer's copy of the widget in a state no hand-written program can produce.
func TestFactoryBuiltWidgetMatchesItsConstructor(t *testing.T) {
	for _, tc := range factoryParityCases {
		fw, err := NewFakeWidgetFromFactory(tc.factory)
		if err != nil {
			t.Fatalf("create %s through the factory: %v", tc.factory, err)
		}
		got, want := fw.Widget(), tc.ctor()
		if reflect.TypeOf(got) != reflect.TypeOf(want) {
			t.Fatalf("%s builds a %T, %s() builds a %T", tc.factory, got, tc.ctorName, want)
		}
		compareWidgetState(t, tc.factory, tc.ctorName, "",
			reflect.ValueOf(got).Elem(), reflect.ValueOf(want).Elem())
	}
}

// compareWidgetState walks two widgets of the same concrete type and reports
// every field where the factory-built one differs from the constructor-built
// one. Values are read with reflect's typed getters, which work on unexported
// fields; pointer-shaped fields are compared on nil-ness alone, since the two
// instances legitimately hold different addresses.
func compareWidgetState(t *testing.T, factory, ctorName, path string, got, want reflect.Value) {
	t.Helper()
	switch got.Kind() {
	case reflect.Struct:
		typ := got.Type()
		for i := 0; i < got.NumField(); i++ {
			name := typ.Field(i).Name
			if path != "" {
				name = path + "." + name
			}
			compareWidgetState(t, factory, ctorName, name, got.Field(i), want.Field(i))
		}
	case reflect.Array:
		for i := 0; i < got.Len(); i++ {
			compareWidgetState(t, factory, ctorName, fmt.Sprintf("%s[%d]", path, i), got.Index(i), want.Index(i))
		}
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		if got.IsNil() != want.IsNil() {
			t.Errorf("%s built through the designer's factory has %s = %s, %s() leaves it %s: "+
				"the factory calls Init and nothing else, so setup that lives in the constructor never runs",
				factory, path, nilness(got), ctorName, nilness(want))
		}
	case reflect.Bool:
		if got.Bool() != want.Bool() {
			reportFieldDiff(t, factory, ctorName, path, got.Bool(), want.Bool())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if got.Int() != want.Int() {
			reportFieldDiff(t, factory, ctorName, path, got.Int(), want.Int())
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if got.Uint() != want.Uint() {
			reportFieldDiff(t, factory, ctorName, path, got.Uint(), want.Uint())
		}
	case reflect.Float32, reflect.Float64:
		if got.Float() != want.Float() {
			reportFieldDiff(t, factory, ctorName, path, got.Float(), want.Float())
		}
	case reflect.String:
		if got.String() != want.String() {
			reportFieldDiff(t, factory, ctorName, path, got.String(), want.String())
		}
	}
}

func reportFieldDiff(t *testing.T, factory, ctorName, path string, got, want interface{}) {
	t.Helper()
	t.Errorf("%s built through the designer's factory has %s = %v, %s() gives %v: "+
		"the factory calls Init and nothing else, so setup that lives in the constructor never runs",
		factory, path, got, ctorName, want)
}

func nilness(v reflect.Value) string {
	if v.IsNil() {
		return "nil"
	}
	return "non-nil"
}

// TestDesignerBuiltTableIsFullyInitialised pins the state a Table placed by the
// designer starts in. The parity test above cannot see this one: once NewTable
// is `new + Init`, dropping the setup out of Init breaks both paths at once and
// they still match each other. These are absolute values — what "no selection,
// nothing hovered, not editing" means — read from the widget the canvas
// actually got.
func TestDesignerBuiltTableIsFullyInitialised(t *testing.T) {
	fw, err := NewFakeWidgetFromFactory("gui.Table")
	if err != nil {
		t.Fatalf("create gui.Table through the factory: %v", err)
	}
	tbl, ok := fw.Widget().(*gui.Table)
	if !ok {
		t.Fatalf("the gui.Table factory built a %T", fw.Widget())
	}

	if field(t, tbl, "scrollArea").IsNil() {
		t.Errorf("a designer-built gui.Table has a nil scrollArea: Layout dereferences it, " +
			"so the widget panics the moment the canvas gives it bounds")
	}

	// The two sentinels with public readers, checked through them: this is the
	// state the rest of the program asks the table about.
	if got := tbl.SelectedRow(); got != -1 {
		t.Errorf("a designer-built gui.Table reports SelectedRow() = %d, want -1: "+
			"row %d comes up highlighted and Enter activates it before anyone clicked", got, got)
	}
	if got := tbl.SortColumn(); got != -1 {
		t.Errorf("a designer-built gui.Table reports SortColumn() = %d, want -1: "+
			"the header draws a sort arrow on column %d that nobody asked for", got, got)
	}
	if got := tbl.SelectedRows(); len(got) != 0 {
		t.Errorf("a designer-built gui.Table reports SelectedRows() = %v, want none", got)
	}

	// The rest have no public reader; each 0 is a row or column the table
	// treats as live before the user has touched it.
	for _, s := range []struct {
		name   string
		effect string
	}{
		{"resizeCol", "the first mouse move over the table resizes column 0"},
		{"hoverBoundary", "the boundary of column 0 comes up hover-highlighted"},
		{"editRow", "cell (0,0) reads as the cell being edited"},
		{"editCol", "cell (0,0) reads as the cell being edited"},
		{"lastClickRow", "the first click on cell (0,0) counts as the second half of a double-click"},
		{"lastClickCol", "the first click on cell (0,0) counts as the second half of a double-click"},
		{"selectAnchor", "the first shift-click selects everything from row 0 down to it"},
	} {
		if got := field(t, tbl, s.name).Int(); got != -1 {
			t.Errorf("a designer-built gui.Table has %s = %d, want -1: %s", s.name, got, s.effect)
		}
	}
}

// field reads an unexported field of a widget by name. reflect can read one
// from another package (only Interface and the setters are blocked), which is
// what lets this test check state gui exposes no accessor for.
func field(t *testing.T, w gui.IWidget, name string) reflect.Value {
	t.Helper()
	v := reflect.ValueOf(w).Elem().FieldByName(name)
	if !v.IsValid() {
		t.Fatalf("%T has no field %s — renamed? update this test", w, name)
	}
	return v
}
