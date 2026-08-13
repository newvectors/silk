package ged

import (
	"regexp"
	"strconv"
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

// TestGeneratedCodeReproducesDesignedButtons checks the generated source against
// the box the designer drew, in both shapes a design produces: the OK/Cancel row
// a drop already carries, and a list typed into the property sheet afterwards.
// The first is the one that bites. The generated file builds the box with
// gui.NewButtonBox(), which carries no buttons at all, so a ButtonBox measured
// against the canvas default instead of that constructor reads as unchanged and
// the standard button row — the whole point of dropping one — exists only in the
// designer.
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

	// Exactly what the drop produced, untouched by the designer.
	assertGeneratedButtons(t, scene, box)

	box.SetButtonsSpec("@yes,@no")
	assertGeneratedButtons(t, scene, box)
}

// assertGeneratedButtons generates the scene and compares the button spec the
// source carries against the one the designed box holds. Both sides are read
// off the live objects — the emitter's output and the widget the canvas shows —
// so a hardcoded expectation cannot agree with a broken emitter.
func assertGeneratedButtons(t *testing.T, scene *GedScene, box *gui.ButtonBox) {
	t.Helper()
	code := scene.GenerateCode(CodeGenOptions{PackageName: "main", TypeName: "ButtonsUI"})
	got, ok := generatedButtonSpec(code)
	if !ok {
		t.Fatalf("generated program builds an empty ButtonBox: no SetButtonsSpec call, so the designed buttons %v never reach the screen\n----\n%s",
			buttonLabels(box), code)
	}
	if want := box.ButtonsSpec(); got != want {
		t.Errorf("generated ButtonBox spec = %q, design has %q (buttons %v)\n----\n%s", got, want, buttonLabels(box), code)
	}
}

var buttonSpecCall = regexp.MustCompile(`SetButtonsSpec\(("[^"]*")\)`)

// generatedButtonSpec reads the button list back out of generated source: the
// argument of the SetButtonsSpec call codegen writes for the scene's one box.
func generatedButtonSpec(code string) (string, bool) {
	m := buttonSpecCall.FindStringSubmatch(code)
	if m == nil {
		return "", false
	}
	spec, err := strconv.Unquote(m[1])
	if err != nil {
		return "", false
	}
	return spec, true
}

// buttonLabels is what the designed box puts on screen, so a failure names the
// buttons that went missing and not just a spec string.
func buttonLabels(box *gui.ButtonBox) []string {
	var out []string
	for _, item := range box.Items() {
		if b, ok := item.(*gui.Button); ok {
			out = append(out, b.Text())
		}
	}
	return out
}

// dropSpace drops one unnamed spring on the scene at the given y (mm) and
// returns the live widget codegen reads the designed values back off.
func dropSpace(t *testing.T, scene *GedScene, y float64) *gui.Space {
	t.Helper()
	fake := addNamedWidget(t, scene, "gui.Space", "", y)
	space, ok := fake.Widget().(*gui.Space)
	if !ok {
		t.Fatalf("dropped gui.Space produced a %T", fake.Widget())
	}
	return space
}

// TestGeneratedCodeReproducesDesignedSpace is the same two-sided assertion for
// the spring, held on both of its properties. A Space with no properties in the
// emitter table is inert once generated — it comes out as
// gui.NewSpace(false, false) whatever the designer set, so a spring flipped to
// 垂直 on the canvas builds a horizontal one in the program — and a Space whose
// defaults are emitted anyway fills every generated constructor with noise.
// The two flags are designed on separate springs on purpose: one spring
// carrying both would generate identically if the table's two entries read the
// same getter, since 垂直 and 弹性 are both bools defaulting to false.
func TestGeneratedCodeReproducesDesignedSpace(t *testing.T) {
	scene := NewGedScene()
	scene.SetFormTitle("Spring")
	scene.SetSize(120, 60)

	dropSpace(t, scene, 5).SetVertical(true)
	dropSpace(t, scene, 20).SetExpand(true)

	code := scene.GenerateCode(CodeGenOptions{PackageName: "main", TypeName: "SpringUI"})
	for _, want := range []string{
		"ui.Space1.SetVertical(true)",
		"ui.Space2.SetExpand(true)",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("designed spring not reproduced: missing %s\n----\n%s", want, code)
		}
	}
	// Each spring changed exactly one flag; the other is still what the drop
	// produced, so it must stay out of the output.
	for _, unwanted := range []string{"ui.Space1.SetExpand(", "ui.Space2.SetVertical("} {
		if strings.Contains(code, unwanted) {
			t.Errorf("a spring emitted %s for a flag left at its default\n----\n%s", unwanted, code)
		}
	}

	// A third, untouched spring: both of its properties sit at the dropped
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
