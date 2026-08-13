package ged

import (
	"strings"
	"testing"

	"github.com/uk0/silk/graph"
	"github.com/uk0/silk/gui"
)

// addSetpointBox drops an unnamed SetpointBox into scene (so it takes the
// generator's derived field name, SetpointBox1) and hands the live designed
// widget to configure — the same object codegen reflects the design-time
// property values off.
func addSetpointBox(t *testing.T, scene *GedScene, configure func(sp *gui.SetpointBox)) {
	t.Helper()
	w, err := NewFakeWidgetFromFactory("gui.SetpointBox")
	if err != nil {
		t.Fatalf("create gui.SetpointBox: %v", err)
	}
	w.SetBounds(5, 5, 30, 20)
	sp, ok := w.Widget().(*gui.SetpointBox)
	if !ok {
		t.Fatalf("gui.SetpointBox factory produced %T", w.Widget())
	}
	if configure != nil {
		configure(sp)
	}
	cmd := graph.NewAddCommand()
	cmd.AddItem(w, scene)
	scene.PushCommand(cmd)
}

// TestGenerateCodeSetpointBox: a designed setpoint must come out of the
// generator as a concrete *gui.SetpointBox carrying BOTH halves of its design —
// the tag (emitted by BindServices, which is what lets scada.BindScreen resolve
// it and install the write side) and the range the operator is allowed to type
// into. A factoryMap entry typed gui.IWidget, or a missing industrialDesignProps
// row, drops one of the two silently and the generated screen still compiles.
func TestGenerateCodeSetpointBox(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compile test in short mode")
	}
	scene := NewGedScene()
	scene.SetFormTitle("HMI")
	scene.SetSize(200, 150)
	addSetpointBox(t, scene, func(sp *gui.SetpointBox) {
		sp.SetTagName("sp")
		sp.SetMax(200)
	})

	gen := scene.GenerateCodeIndexed(CodeGenOptions{PackageName: "main", TypeName: "HMIUI"})
	if gen.Err != nil {
		t.Fatalf("GenerateCodeIndexed: %v", gen.Err)
	}
	code := gen.Code

	for _, want := range []string{
		"SetpointBox1 *gui.SetpointBox",
		`ui.SetpointBox1.SetTagName("sp")`,
		"ui.SetpointBox1.SetMax(200)",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated code missing:\n  %s\n----\n%s", want, code)
		}
	}

	// The emitted calls must type-check against the real gui + scada API.
	vetGeneratedCode(t, code)
}

// TestGenerateCodeSetpointBoxUntouchedEmitsNoRange: emitDesignProperties writes
// only what the designer changed. A setpoint dragged onto the form and left
// alone must emit no range call at all — hard-coding a "default" range into the
// output freezes it, so a later change to the widget's own defaults never
// reaches an already-generated screen.
func TestGenerateCodeSetpointBoxUntouchedEmitsNoRange(t *testing.T) {
	scene := NewGedScene()
	scene.SetFormTitle("HMI")
	scene.SetSize(200, 150)
	addSetpointBox(t, scene, nil)

	code := scene.GenerateCode(CodeGenOptions{PackageName: "main", TypeName: "HMIUI"})
	for _, bad := range []string{"SetMax(", "SetMin(", "SetDecimals("} {
		if strings.Contains(code, bad) {
			t.Errorf("untouched SetpointBox emitted %s\n----\n%s", bad, code)
		}
	}
}
