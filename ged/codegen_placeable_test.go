package ged

import (
	"strings"
	"testing"

	"github.com/uk0/silk/gui"
)

// TestGeneratedCodeCoversNewlyPlaceableWidgets is the guard for the four
// factoryMap rows that make a divider rule, a spring, a standard button row and
// a property grid placeable at all. Without a row, GenerateCodeIndexed reports
// the factory as unmapped and degrades the field to gui.IWidget behind a
// core.New(...) assertion — output that looks green and is not the design. The
// import assertion is separate on purpose: prop.PropertySheet is the only
// mapping outside gui/device, so its import path is the one nothing else adds.
func TestGeneratedCodeCoversNewlyPlaceableWidgets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compile test in short mode")
	}

	scene := NewGedScene()
	scene.SetFormTitle("Placeable")
	scene.SetSize(140, 200)

	y := 5.0
	for _, factory := range []string{"gui.Separator", "gui.Space", "gui.ButtonBox", "prop.PropertySheet"} {
		fake := addNamedWidget(t, scene, factory, "", y)
		y += 20
		// Configure the two widgets that have design properties, so vet below
		// type-checks the emitted setters against the concrete field type as
		// well as the constructors. A setter named in industrialDesignProps but
		// absent from the widget is a compile error nothing else here would see.
		switch w := fake.Widget().(type) {
		case *gui.ButtonBox:
			w.SetButtonsSpec("@yes,@no")
		case *gui.Space:
			w.SetExpand(true)
		}
	}

	gen := scene.GenerateCodeIndexed(CodeGenOptions{PackageName: "main", TypeName: "PlaceableUI"})
	if gen.Err != nil {
		t.Fatalf("generation reported an error: %v\n----\n%s", gen.Err, gen.Code)
	}
	for _, want := range []string{
		"*gui.Separator", "gui.NewSeparator()",
		"*gui.Space", "gui.NewSpace(false, false)",
		"*gui.ButtonBox", "gui.NewButtonBox()",
		"*prop.PropertySheet", "prop.NewPropertySheet()",
		`"github.com/uk0/silk/prop"`,
	} {
		if !strings.Contains(gen.Code, want) {
			t.Errorf("generated code missing %q\n----\n%s", want, gen.Code)
		}
	}

	// The constructor arguments only matter if they compile and match the
	// widget the designer drew: gui.NewSpace(false, false) is the factory
	// default a dropped Space carries, so canvas and program start identical.
	vetGeneratedCode(t, gen.Code)
}

// TestGeneratedCodeReproducesDesignedButtons is the two-sided assertion for the
// ButtonBox design property, made at the emitter rather than at the table: a
// designed button list must reach the generated screen, and the list a dropped
// ButtonBox already carries must not be written out again. The second half is
// what makes setDefaultContent's "buttonbox" case load-bearing — drop it and
// the reference default becomes the empty spec, so every untouched ButtonBox
// emits a SetButtonsSpec nobody asked for.
func TestGeneratedCodeReproducesDesignedButtons(t *testing.T) {
	scene := NewGedScene()
	scene.SetFormTitle("Buttons")
	scene.SetSize(120, 60)

	fake := addNamedWidget(t, scene, "gui.ButtonBox", "", 5)
	box, ok := fake.Widget().(*gui.ButtonBox)
	if !ok {
		t.Fatalf("dropped gui.ButtonBox produced a %T", fake.Widget())
	}
	if got := box.ButtonsSpec(); got != "@ok,@cancel" {
		t.Fatalf("as-dropped ButtonBox spec = %q, want %q", got, "@ok,@cancel")
	}

	box.SetButtonsSpec("@yes,@no")
	code := scene.GenerateCode(CodeGenOptions{PackageName: "main", TypeName: "ButtonsUI"})
	const want = `ui.ButtonBox1.SetButtonsSpec("@yes,@no")`
	if !strings.Contains(code, want) {
		t.Errorf("designed buttons not reproduced: missing %s\n----\n%s", want, code)
	}

	// Back to exactly what the drop produced: nothing was designed, so nothing
	// is emitted.
	box.SetButtonsSpec("@ok,@cancel")
	code = scene.GenerateCode(CodeGenOptions{PackageName: "main", TypeName: "ButtonsUI"})
	if strings.Contains(code, "SetButtonsSpec(") {
		t.Errorf("an untouched ButtonBox emitted a SetButtonsSpec call\n----\n%s", code)
	}
}

// TestGeneratedCodeReproducesDesignedSpace is the same two-sided assertion for
// the spring. A Space with no properties in the emitter table is inert once
// generated — it comes out as gui.NewSpace(false, false) whatever the designer
// set — and a Space whose defaults are emitted anyway fills every generated
// constructor with noise.
func TestGeneratedCodeReproducesDesignedSpace(t *testing.T) {
	scene := NewGedScene()
	scene.SetFormTitle("Spring")
	scene.SetSize(120, 60)

	fake := addNamedWidget(t, scene, "gui.Space", "", 5)
	space, ok := fake.Widget().(*gui.Space)
	if !ok {
		t.Fatalf("dropped gui.Space produced a %T", fake.Widget())
	}

	space.SetExpand(true)
	code := scene.GenerateCode(CodeGenOptions{PackageName: "main", TypeName: "SpringUI"})
	const want = "ui.Space1.SetExpand(true)"
	if !strings.Contains(code, want) {
		t.Errorf("designed stretch not reproduced: missing %s\n----\n%s", want, code)
	}

	// A second, untouched spring: both of its properties sit at the dropped
	// defaults, so neither setter may appear.
	plain := NewGedScene()
	plain.SetFormTitle("Plain")
	plain.SetSize(120, 60)
	addNamedWidget(t, plain, "gui.Space", "", 5)
	code = plain.GenerateCode(CodeGenOptions{PackageName: "main", TypeName: "PlainUI"})
	for _, unwanted := range []string{"SetExpand(", "SetVertical("} {
		if strings.Contains(code, unwanted) {
			t.Errorf("an untouched Space emitted %s\n----\n%s", unwanted, code)
		}
	}
}

// TestDropSizeForNewlyPlaceableWidgets drives the real canvas drop path, not
// defaultSizeForWidget on its own: the size table is only worth anything if
// OnDrop reads it. A property grid or a divider rule that lands at the 30x8 mm
// fallback is unreadable on the canvas — the grid shows one truncated row, the
// rule is eight times too thick.
func TestDropSizeForNewlyPlaceableWidgets(t *testing.T) {
	for _, tc := range []struct {
		factory string
		w, h    float64
	}{
		{"prop.PropertySheet", 60, 50},
		{"gui.Separator", 50, 2},
	} {
		view := NewGedView()
		scene := view.GedScene()

		ctx := &dropCtx{pa: gui.DndCopy, payload: tc.factory}
		ctx.SetAction(gui.DndIgnore)
		view.OnDrop(20, 20, ctx)

		if got := len(scene.Children()); got != 1 {
			t.Fatalf("%s: scene children after the drop = %d, want 1", tc.factory, got)
		}
		item := scene.Children()[0]
		if w, h := item.Width(), item.Height(); w != tc.w || h != tc.h {
			t.Errorf("%s dropped at %gx%g mm, want %gx%g", tc.factory, w, h, tc.w, tc.h)
		}
	}
}
