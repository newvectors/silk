package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/uk0/silk/decl"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/uk0/silk/core"
	"github.com/uk0/silk/geom"
)

// Exercise the actual CLI and then compile/run its output from a separate Go
// module. Parsing generated text alone misses factory registration, imports,
// clamping during property restoration and native SDK consumer compatibility.
func TestSDKConsumerCompilesAndBuildsGeneratedDesignerForm(t *testing.T) {
	if testing.Short() {
		t.Skip("external SDK consumer compilation")
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "silkgen")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	runSDKCommandEnv(t, repo, []string{"CGO_ENABLED=0"}, "go", "build", "-o", binary, "./cmd/silkgen")
	form := sdkGeneratorNode("form", "main", 0, 0, 400, 280)
	box := sdkGeneratorNode("gui.VBox", "box", 40, 30, 220, 180)
	button := sdkGeneratorNode("gui.Button", "button", 45, 36, 100, 26)
	sdkGeneratorProps(button, [][2]string{{"文本", `"@ref:literal"`}, {"可用", "false"}})
	date := sdkGeneratorNode("gui.DatePicker", "date", 45, 72, 150, 28)
	sdkGeneratorProps(date, [][2]string{{"Day", "31"}, {"Month", "1"}, {"Year", "2024"}})
	slider := sdkGeneratorNode("gui.Slider", "slider", 45, 110, 160, 24)
	sdkGeneratorProps(slider, [][2]string{{"值", "-250"}, {"最小值", "-300"}, {"最大值", "-200"}})
	combo := sdkGeneratorNode("gui.ComboBox", "combo", 45, 150, 150, 28)
	items := []string{"", "引号 \"第二项\"\n下一行", "@ref:literal"}
	rawItems, err := core.PersistString(items)
	if err != nil {
		t.Fatal(err)
	}
	sdkGeneratorProps(combo, [][2]string{{"selectedIndex", "1"}, {"items", rawItems}})
	sdkGeneratorChildren(box, button, date, slider, combo)
	sdkGeneratorChildren(form, box)
	input := filepath.Join(dir, "main-window.silkui")
	if err := form.SaveFile(input); err != nil {
		t.Fatal(err)
	}
	assertSDKStdoutContract(t, dir, binary, input)
	output := filepath.Join(dir, "ui_generated.go")
	runSDKCommand(t, repo, binary, "-package", "consumer", "-o", output, input)
	generated, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	runSDKCommand(t, repo, binary, "-package", "consumer", "-o", output, input)
	again, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(generated) != string(again) {
		t.Fatal("CLI changed output for identical source")
	}
	mod := "module sdk-consumer\n\ngo 1.26.0\n\nrequire github.com/uk0/silk v0.0.0\nreplace github.com/uk0/silk => " + strconv.Quote(repo) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0644); err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile(filepath.Join(repo, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), sum, 0644); err != nil {
		t.Fatal(err)
	}
	literal, err := generateSource("choices.silkui", "consumer", "BuildChoices", decl.ComboBox(decl.P("selectedIndex", 2), decl.P("items", items)))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "choices_generated.go"), literal, 0644); err != nil {
		t.Fatal(err)
	}
	copySDKReadmeExamples(t, repo, dir)
	consumer := `package consumer
import (
 "testing"
 "github.com/uk0/silk/gui"
)
func TestGeneratedSDKForm(t *testing.T) {
 root,index,err:=BuildMainWindow().BuildWithIndex()
 if err!=nil {t.Fatal(err)}
 if root.(*gui.Form).Width()!=400 {t.Fatal("root geometry lost")}
 box:=index["box"].(*gui.VBox)
 button:=index["button"].(*gui.Button)
 if box.X()!=40 || box.Y()!=30 || button.Parent()!=box || button.X()!=5 || button.Y()!=6 {t.Fatal("nested designer geometry changed in generated SDK consumer")}
 if box.LayoutEnabled() || button.Text()!="@ref:literal" || button.IsEnabled() {t.Fatal("unmanaged layout or explicit property state lost")}
 d:=index["date"].(*gui.DatePicker)
 if d.Year()!=2024 || d.Month()!=1 || d.Day()!=31 {t.Fatal("date was clamped or not restored")}
 s:=index["slider"].(*gui.Slider)
 if s.Min()!=-300 || s.Max()!=-200 || s.Value()!=-250 {t.Fatal("value was clamped before its final range")}
 c:=index["combo"].(*gui.ComboBox)
 if c.Count()!=3 || c.ActiveIndex()!=1 || c.Text()!="引号 \"第二项\"\n下一行" {t.Fatalf("combo string list/selection lost: %v %d %q",c.Items(),c.ActiveIndex(),c.Text())}
 literal,err:=BuildChoices().Build();if err!=nil {t.Fatal(err)}
 choices:=literal.(*gui.ComboBox)
 if choices.Count()!=3 || choices.ActiveIndex()!=2 || choices.Text()!="@ref:literal" {t.Fatal("typed []string Go literal did not build its final selection")}
}
`
	if err := os.WriteFile(filepath.Join(dir, "consumer_test.go"), []byte(consumer), 0644); err != nil {
		t.Fatal(err)
	}
	runSDKCommand(t, dir, "go", "test", "-mod=mod", "-count=1", "./...")
}

func runSDKCommand(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	runSDKCommandEnv(t, dir, nil, name, args...)
}

func runSDKCommandEnv(t *testing.T, dir string, env []string, name string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v in %s: %v\n%s", name, args, dir, err, output)
	}
}

func sdkGeneratorNode(factory, id string, x, y, w, h float64) *core.TDoc {
	d := core.NewTDoc()
	_ = d.SetValue(factory)
	_ = d.WriteAttr("name", id)
	_ = d.WriteAttr("bounds", geom.Rect{X: x * 25.4 / 96, Y: y * 25.4 / 96, Width: w * 25.4 / 96, Height: h * 25.4 / 96})
	return d
}
func sdkGeneratorChildren(parent *core.TDoc, children ...*core.TDoc) {
	d := core.NewTDoc()
	d.SetKey("children")
	for _, child := range children {
		d.AddChild(child)
	}
	parent.AddChild(d)
}
func sdkGeneratorProps(parent *core.TDoc, props [][2]string) {
	d := core.NewTDoc()
	d.SetKey("props")
	for _, p := range props {
		_ = d.WriteAttr(p[0], p[1])
	}
	parent.AddChild(d)
}

// Compile the README snippets verbatim, as independent main packages in the
// consumer module. Running main is intentionally not part of this headless
// SDK contract test (it would open windows and enter the event loop).
func copySDKReadmeExamples(t *testing.T, repo, dir string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readme := string(raw)
	start := strings.Index(readme, "## Your First App")
	if start < 0 {
		t.Fatal("SDK README section missing")
	}
	end := strings.Index(readme[start:], "## Using the Designer")
	if end < 0 {
		t.Fatal("SDK README boundary missing")
	}
	matches := regexp.MustCompile("(?s)```go\n(.*?)```").FindAllStringSubmatch(readme[start:start+end], -1)
	if len(matches) != 2 {
		t.Fatalf("got %d SDK README examples, want 2", len(matches))
	}
	for i, match := range matches {
		sub := filepath.Join(dir, fmt.Sprintf("readme_app%d", i+1))
		if err := os.MkdirAll(sub, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "main.go"), []byte(match[1]), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func assertSDKStdoutContract(t *testing.T, dir, binary, input string) {
	t.Helper()
	state := filepath.Join(dir, "cli-state")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(state, "_"+filepath.Base(binary)+"_.runlock")
	var previous []byte
	for _, stale := range []bool{false, false, true} {
		if stale {
			if err := os.WriteFile(marker, []byte("stale"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command(binary, input)
		cmd.Env = append(os.Environ(), "SILK_DATA_DIR="+state)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("CLI stdout generation: %v\n%s", err, stderr.String())
		}
		if !bytes.HasPrefix(output, []byte("// Code generated by silkgen")) {
			t.Fatalf("startup diagnostics contaminated generated Go stdout:\n%s", output)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), "stdout.go", output, 0); err != nil {
			t.Fatalf("stdout isn't compilable Go syntax: %v\n%s", err, output)
		}
		if stale && !strings.Contains(stderr.String(), "Warning:") {
			t.Fatalf("stale-run diagnostic was lost instead of redirected to stderr: %s", stderr.String())
		}
		if previous != nil && !bytes.Equal(previous, output) {
			t.Fatal("repeated CLI stdout generation is not byte-identical")
		}
		previous = output
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("normal CLI exit retained run marker: %v", err)
		}
	}
	// Help and validation errors also run cleanup; flag.ExitOnError would
	// bypass defers and make a harmless invocation look like a crashed SDK app.
	for _, args := range [][]string{{"-h"}, {"-unknown"}, {"-package", "bad-name", input}, {"missing.silkui"}} {
		cmd := exec.Command(binary, args...)
		cmd.Env = append(os.Environ(), "SILK_DATA_DIR="+state)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, _ := cmd.Output()
		if len(output) != 0 {
			t.Fatalf("non-generation invocation wrote stdout: %s", output)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("CLI arguments %v retained run marker: %v", args, err)
		}
	}
}
