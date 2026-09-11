package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uk0/silk/ged"
	"github.com/uk0/silk/gui"
	"github.com/uk0/silk/paint"
)

// Use the real designer serializer, not decl.ToTDoc: these are two different
// dialects. Compiling alone misses the old bug (an empty, unbuildable "form").
func TestRegenerateDesignerGoCompilesAndBuilds(t *testing.T) {
	dir := t.TempDir()
	scene := ged.NewGedScene()
	scene.SetSize(100, 100)
	scene.SetFormTitle("精确设计")
	scene.SetFilename(filepath.Join(dir, "design.silkui"))

	button, err := ged.NewFakeWidgetFromFactory("gui.Button")
	if err != nil {
		t.Fatal(err)
	}
	button.SetWidgetName("action")
	button.SetBounds(25, 15, 35, 10)
	button.Widget().(*gui.Button).SetText("按钮 \"确定\"\n@ref:literal")
	button.Widget().SetEnabled(false)
	button.SetParent(scene)

	box, err := ged.NewFakeWidgetFromFactory("gui.VBox")
	if err != nil {
		t.Fatal(err)
	}
	box.SetWidgetName("container")
	box.SetBounds(5, 40, 50, 35)
	box.SetParent(scene)
	label, err := ged.NewFakeWidgetFromFactory("gui.Label")
	if err != nil {
		t.Fatal(err)
	}
	label.SetWidgetName("emptyLabel")
	label.Widget().(*gui.Label).SetText("")
	label.SetParent(box)

	tank, err := ged.NewFakeWidgetFromFactory("gui.Tank")
	if err != nil {
		t.Fatal(err)
	}
	tank.SetWidgetName("tank")
	tank.SetBounds(70, 40, 15, 35)
	tank.Widget().(*gui.Tank).SetColor(paint.Color{R: 19, G: 166, B: 227, A: 128})
	tank.Widget().(*gui.Tank).SetMax(250)
	tank.SetParent(scene)
	if !scene.Save() {
		t.Fatal("save actual designer fixture failed")
	}
	regenerateGoForSilkui(scene.Filename())
	generated := filepath.Join(dir, "design.silk.go")
	first, err := os.ReadFile(generated)
	if err != nil {
		t.Fatal(err)
	}
	regenerateGoForSilkui(scene.Filename())
	second, err := os.ReadFile(generated)
	if err != nil || string(first) != string(second) {
		t.Fatalf("generation is not byte stable: %v", err)
	}
	if strings.Contains(string(first), `decl.New("form"`) {
		t.Fatalf("unregistered designer root leaked into runtime:\n%s", first)
	}

	// Compile and execute the generated function in a separate process against
	// the real GUI factories. Nothing is mocked and no native window is opened.
	check := filepath.Join(dir, "generated_runtime_test.go")
	if err := os.WriteFile(check, []byte(`package ui
import (
    "testing"
    "github.com/uk0/silk/gui"
    "github.com/uk0/silk/paint"
)
func TestGeneratedRuntime(t *testing.T) {
    formObject, index, err := BuildDesign().BuildWithIndex()
    if err != nil { t.Fatal(err) }
    form := formObject.(*gui.Form)
    if form.Title() != "精确设计" || form.Width() != 378 || form.Height() != 378 {
        t.Fatalf("form mismatch: %q %v x %v", form.Title(), form.Width(), form.Height())
    }
    button, ok := index["action"].(*gui.Button)
    if !ok { t.Fatalf("missing real button: %#v", index) }
    if button.Text() != "按钮 \"确定\"\n@ref:literal" || button.IsEnabled() {
        t.Fatalf("button state mismatch: %q, enabled=%v", button.Text(), button.IsEnabled())
    }
    x, y, w, h := button.Bounds()
    if x != 94 || y != 57 || w != 132 || h != 38 {
        t.Fatalf("bounds mismatch: %v %v %v %v", x, y, w, h)
    }
    label := index["emptyLabel"].(*gui.Label)
    if label.Text() != "" || label.Parent() != index["container"] {
        t.Fatal("nested child or intentionally empty caption lost")
    }
    tank := index["tank"].(*gui.Tank)
    if tank.Color() != (paint.Color{R:19, G:166, B:227, A:128}) || tank.Max() != 250 {
        t.Fatalf("exact RGBA or scalar lost: %v max=%v", tank.Color(), tank.Max())
    }
}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", generated, check)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated code must build the actual design: %v\n%s\n--- source ---\n%s", err, output, first)
	}
}
