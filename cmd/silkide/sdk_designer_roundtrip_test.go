package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/uk0/silk/core"
	"github.com/uk0/silk/ged"
	"github.com/uk0/silk/gui"
)

// Pin the GUI-reported failure through the actual editor serializer, not the
// SDK's separate SaveForm path: load persisted props, save, regenerate from a
// hyphenated filename, then compile and Build in a real external SDK consumer.
func TestSDKDesignerLoadSaveRegenerateBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("external SDK compilation")
	}
	dir := t.TempDir()
	uiDir := filepath.Join(dir, "ui")
	if err := os.Mkdir(uiDir, 0o700); err != nil {
		t.Fatal(err)
	}
	doc, err := core.LoadTDocStr(`= "form"
bounds = [0 0 120 90]
title = "SDK cross-platform controls"
children =
	= "gui.VBox"
		bounds = [10 10 90 70]
		name = "platformPanel"
		children =
			= "gui.DatePicker"
				bounds = [15 15 65 10]
				name = "sdkDate"
				props =
					Day = "29"
					Format = "\"02/01/2006\""
					Month = "2"
					Year = "2024"
			= "gui.ComboBox"
				bounds = [15 35 65 10]
				name = "sdkPlatform"
				props =
					selectedIndex = "2"
					text = "\"macOS\""
					items = "[\"Windows\" \"Linux \\\"桌面\\\"\\n第二行\" \"macOS\" \"\" \"@ref:literal\"]"
			= "gui.Slider"
				bounds = [15 50 65 10]
				name = "sdkRange"
				props =
					值 = "-150"
					最小值 = "-200"
					最大值 = "-100"
`)
	if err != nil {
		t.Fatal(err)
	}
	items := []string{"Windows", "Linux \"桌面\"\n第二行", "macOS", "", "@ref:literal"}
	scene := ged.NewGedScene()
	scene.SetFilename(filepath.Join(uiDir, "sdk-platforms.silkui"))
	for round := 0; round < 2; round++ {
		if err := scene.LoadDesign(doc); err != nil {
			t.Fatal(err)
		}
		if len(scene.Children()) != 1 {
			t.Fatal("designer failed to load the nested container")
		}
		box := scene.Children()[0].(*ged.FakeWidget)
		if len(box.Children()) != 3 {
			t.Fatal("designer lost nested widgets")
		}
		combo := box.Children()[1].(*ged.FakeWidget).Widget().(*gui.ComboBox)
		if !reflect.DeepEqual(combo.Items(), items) || combo.ActiveIndex() != 2 || combo.Text() != "macOS" {
			t.Fatalf("round %d designer list/selection lost: %q index=%d text=%q", round, combo.Items(), combo.ActiveIndex(), combo.Text())
		}
		date := box.Children()[0].(*ged.FakeWidget).Widget().(*gui.DatePicker)
		if date.Year() != 2024 || date.Month() != 2 || date.Day() != 29 || date.Format() != "02/01/2006" {
			t.Fatalf("round %d designer date/format lost", round)
		}
		slider := box.Children()[2].(*ged.FakeWidget).Widget().(*gui.Slider)
		if slider.Min() != -200 || slider.Max() != -100 || slider.Value() != -150 {
			t.Fatalf("round %d designer range/value clamped", round)
		}
		if !scene.Save() { // Save calls the editor's SaveDesign, not gui.SaveForm.
			t.Fatal("designer save failed")
		}
		doc, err = core.LoadTDocFile(scene.Filename())
		if err != nil {
			t.Fatal(err)
		}
	}
	regenerateGoForSilkui(scene.Filename())
	generated := filepath.Join(uiDir, "sdk-platforms.silk.go")
	first, err := os.ReadFile(generated)
	if err != nil {
		t.Fatalf("IDE failed to generate from a hyphenated design filename: %v", err)
	}
	regenerateGoForSilkui(scene.Filename())
	second, err := os.ReadFile(generated)
	if err != nil || string(first) != string(second) {
		t.Fatalf("IDE generation is not byte-stable: %v", err)
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	mod := fmt.Sprintf("module sdk.designer.consumer\n\ngo 1.26.0\n\nrequire github.com/uk0/silk v0.0.0\nreplace github.com/uk0/silk => %q\n", repo)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uiDir, "sdk_runtime_test.go"), []byte(sdkDesignerRuntimeCheck), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-mod=mod", "-count=1", "./...")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("external SDK consumer failed to compile/Build: %v\n%s\n--- generated ---\n%s", err, output, first)
	}
}

const sdkDesignerRuntimeCheck = `package ui
import (
    "reflect"
    "testing"
    "github.com/uk0/silk/gui"
)
func TestSDKRuntime(t *testing.T) {
    object, index, err := BuildSdkPlatforms().BuildWithIndex()
    if err != nil { t.Fatal(err) }
    form := object.(*gui.Form)
    if form.Title() != "SDK cross-platform controls" || form.Width() != 454 || form.Height() != 340 {
        t.Fatalf("form geometry/title changed: %q %v x %v", form.Title(), form.Width(), form.Height())
    }
    items := []string{"Windows", "Linux \"桌面\"\n第二行", "macOS", "", "@ref:literal"}
    combo := index["sdkPlatform"].(*gui.ComboBox)
    if !reflect.DeepEqual(combo.Items(), items) || combo.ActiveIndex() != 2 || combo.Text() != "macOS" {
        t.Fatalf("SDK list/selection changed: %q index=%d text=%q", combo.Items(), combo.ActiveIndex(), combo.Text())
    }
    if combo.Parent() != index["platformPanel"] { t.Fatal("SDK nesting lost") }
    x, y, w, h := combo.Bounds()
    if x != 19 || y != 94 || w != 246 || h != 38 {
        t.Fatalf("nested coordinates double-offset: %v %v %v %v", x, y, w, h)
    }
    date := index["sdkDate"].(*gui.DatePicker)
    if date.Year() != 2024 || date.Month() != 2 || date.Day() != 29 || date.Format() != "02/01/2006" {
        t.Fatal("SDK leap date/format lost")
    }
    slider := index["sdkRange"].(*gui.Slider)
    if slider.Min() != -200 || slider.Max() != -100 || slider.Value() != -150 {
        t.Fatalf("SDK negative range/value changed: [%v,%v] value=%v", slider.Min(), slider.Max(), slider.Value())
    }
}
`
