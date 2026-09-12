package ged

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/uk0/silk/core"
	"github.com/uk0/silk/decl"
	"github.com/uk0/silk/gui"
	"github.com/uk0/silk/internal/silkgen"
)

const ganttSDKTasksJSON = `[
 {"id":"project","name":"跨平台发布项目"},
 {"id":"phase","parentId":"project","name":"设计阶段","collapsed":true},
 {"id":"design","parentId":"phase","name":"设计 \"控件\" \\ SDK","start":"2028-02-28T23:30:00-08:00","end":"2028-03-01T23:30:00-08:00","progress":0.5,"owner":"张三","description":"第一行\n@ref:literal"},
 {"id":"build","parentId":"project","name":"实现与联调","start":"2028-03-01","end":"2028-03-05","progress":0.25,"owner":"李四","dependencies":["design"]},
 {"id":"ship","parentId":"project","name":"发布里程碑","start":"2028-03-05","end":"2028-03-05","milestone":true,"dependencies":["build"]}
]`

func configureGanttSDKFixture(t *testing.T, fake *FakeWidget) *gui.GanttChart {
	t.Helper()
	chart := fake.Widget().(*gui.GanttChart)
	if err := chart.LoadTasksJSON(ganttSDKTasksJSON); err != nil {
		t.Fatal(err)
	}
	chart.SetDayWidth(42.5)
	chart.SetRowHeight(37)
	chart.SetTableWidth(510)
	chart.SetReadOnly(true)
	chart.SetCascadeDependencies(false)
	fake.SetEventHandler("OnTasksChanged", "onGanttTasksChanged")
	return chart
}

// Optional native-QA fixture creation uses the real designer serializer, not a
// hand-written approximation of the .silkui schema. It never opens a window.
func TestGanttDesignerWriteQAFixture(t *testing.T) {
	dir := os.Getenv("SILK_GANTT_QA_DIR")
	if dir == "" {
		t.Skip("set SILK_GANTT_QA_DIR to write an opt-in native QA fixture")
	}
	path := filepath.Join(dir, "workspace", "gantt-qa.silkui")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	scene := NewGedScene()
	scene.SetSize(240, 160)
	chart := addNamedWidget(t, scene, "gui.GanttChart", "Schedule", 5)
	chart.SetBounds(5, 5, 180, 95)
	g := configureGanttSDKFixture(t, chart)
	g.SetReadOnly(false)
	g.SetCascadeDependencies(true)
	g.SetTableWidth(340)
	chart.SetEventHandler("OnTasksChanged", "")
	scene.SetFilename(path)
	if !scene.Save() {
		t.Fatal("saving QA fixture failed")
	}
	t.Logf("wrote real designer fixture: %s", path)
}

// This fixture crosses the actual palette command, disk serializer and loader
// before either generator sees it. In particular, nested controls must remain
// parented after palette undo/redo, not only retain a JSON property in memory.
func loadSavedGanttSDKFixture(t *testing.T) (*GedScene, string, string) {
	t.Helper()
	view := NewGedView()
	scene := view.GedScene()
	scene.SetSize(280, 180)
	container := placeFromPalette(t, view, "gui.VBox", 10, 10)
	container.SetWidgetName("Container")
	container.SetBounds(10, 10, 250, 150)
	chart := placeFromPalette(t, view, "gui.GanttChart", 20, 20)
	chart.SetWidgetName("Schedule")
	chart.SetBounds(20, 20, 220, 105)
	if chart.Parent() != container {
		t.Fatal("palette did not place Gantt inside the container")
	}
	g := configureGanttSDKFixture(t, chart)
	wantJSON := g.TasksJSON()
	wantProps := captureWidgetProperties(g)
	wantEvents := chart.EventHandlers()
	view.Undo()
	if len(allFakeWidgets(scene)) != 1 {
		t.Fatal("undo did not remove just the Gantt chart")
	}
	view.Redo()
	if chart.Parent() != container || g.TasksJSON() != wantJSON || len(allFakeWidgets(scene)) != 2 {
		t.Fatal("redo lost Gantt data or its nested parent")
	}
	path := filepath.Join(t.TempDir(), "gantt-排期-sdk.silkui")
	for round := 0; round < 2; round++ {
		scene.SetFilename(path)
		if !scene.Save() {
			t.Fatal("saving Gantt scene failed")
		}
		reloaded := NewGedScene()
		if err := reloaded.OpenFile(path); err != nil {
			t.Fatal(err)
		}
		if missing := reloaded.MissingWidgets(); len(missing) != 0 {
			t.Fatalf("missing factories: %v", missing)
		}
		widgets := allFakeWidgets(reloaded)
		if len(widgets) != 2 || widgets[0].WidgetName() != "Container" || widgets[1].WidgetName() != "Schedule" || widgets[1].Parent() != widgets[0] {
			t.Fatal("disk round trip changed the Gantt hierarchy")
		}
		chart = widgets[1]
		if got := chart.EventHandlers(); !reflect.DeepEqual(got, wantEvents) {
			t.Fatalf("events lost after disk reload: %v", got)
		}
		for phase, widget := range map[string]gui.IWidget{"designer": chart.Widget(), "preview": chart.Generate()} {
			if got := captureWidgetProperties(widget.(core.IEnumProperties)); !reflect.DeepEqual(got, wantProps) {
				t.Fatalf("round %d %s properties changed:\ngot %v\nwant %v", round, phase, got, wantProps)
			}
			g := widget.(*gui.GanttChart)
			if g.LastError() != nil || g.TasksJSON() != wantJSON {
				t.Fatalf("round %d %s task data changed: %v", round, phase, g.LastError())
			}
		}
		scene = reloaded
	}
	return scene, path, wantJSON
}

func TestGanttDesignerPaletteUndoDiskAndPreview(t *testing.T) {
	loadSavedGanttSDKFixture(t)
}

// Compile and execute both exported APIs in a separate consumer module. This
// deliberately goes beyond go/parser/go vet: factory initialization, property
// replay, parent attachment and concrete event wiring all execute, but no native
// window is shown and no printer or external service is contacted.
func TestGanttDesignerGeneratedSDKRuntime(t *testing.T) {
	if testing.Short() {
		t.Skip("external SDK consumer compilation")
	}
	scene, input, wantJSON := loadSavedGanttSDKFixture(t)
	concrete := scene.GenerateCodeIndexed(CodeGenOptions{PackageName: "ganttfixture", TypeName: "GanttFixtureUI"})
	if concrete.Err != nil {
		t.Fatal(concrete.Err)
	}
	doc, err := core.LoadTDocFile(input)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := decl.FromTDoc(doc)
	if err != nil {
		t.Fatal(err)
	}
	declarative, err := silkgen.GenerateSource(input, "ganttfixture", "", tree)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":          fmt.Sprintf("module ganttfixture\ngo 1.26.0\nrequire github.com/uk0/silk v0.0.0\nreplace github.com/uk0/silk => %s\n", strconv.Quote(findModuleRoot(t))),
		"concrete.go":     concrete.Code,
		"declarative.go":  string(declarative),
		"runtime_test.go": fmt.Sprintf(ganttSDKRuntimeTest, wantJSON, silkgen.DefaultBuildName(input)),
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-mod=mod", "-p", "1", "-count=1", "-run", "TestGanttGeneratedRuntime", "-v", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOMAXPROCS=2")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("external Gantt SDK build/runtime failed: %v\n%s", err, out)
	}
	t.Logf("external consumer compiled and ran both generated constructors:\n%s", out)
}

const ganttSDKRuntimeTest = `package ganttfixture
import (
 "testing"
 "github.com/uk0/silk/gui"
)
var changes int
func onGanttTasksChanged() { changes++ }
func assertGantt(t *testing.T, chart *gui.GanttChart, container *gui.VBox) {
 t.Helper()
 if chart == nil || container == nil { t.Fatal("missing concrete Gantt or container") }
 if chart.LastError() != nil || chart.TasksJSON() != %q { t.Fatal("generated task snapshot changed", chart.LastError()) }
 if chart.DayWidth()!=42.5 || chart.RowHeight()!=37 || chart.TableWidth()!=510 || !chart.IsReadOnly() || chart.CascadeDependencies() { t.Fatal("generated properties changed") }
 if chart.Parent()!=container || container.LayoutEnabled() { t.Fatal("generated parent or free-container layout changed") }
 before := chart.Bounds1()
 container.Layout()
 if chart.Bounds1()!=before || chart.X()!=38 || chart.Y()!=38 { t.Fatal("generated local geometry changed", chart.Bounds1()) }
 tasks:=chart.Tasks()
 if len(tasks)!=5 || tasks[2].Start.Format("2006-01-02")!="2028-02-28" || tasks[2].End.Format("2006-01-02")!="2028-03-01" || !tasks[1].Collapsed || !tasks[4].Milestone { t.Fatal("civil-date, hierarchy or milestone data changed") }
}
func TestGanttGeneratedRuntime(t *testing.T) {
 ui:=NewGanttFixtureUI()
 assertGantt(t,ui.Schedule,ui.Container)
 before:=changes
 ui.Schedule.SetTasksJSON(ui.Schedule.TasksJSON())
 if changes!=before+1 { t.Fatal("concrete OnTasksChanged handler was not wired") }
 root,index,err:=%s().BuildWithIndex()
 if err!=nil { t.Fatal("declarative build",err) }
 if _,ok:=root.(*gui.Form); !ok { t.Fatal("declarative root is not a Form") }
 chart,ok:=index["Schedule"].(*gui.GanttChart)
 if !ok { t.Fatal("declarative chart factory type") }
 container,ok:=index["Container"].(*gui.VBox)
 if !ok { t.Fatal("declarative container factory type") }
 assertGantt(t,chart,container)
 // decl intentionally leaves application event callbacks to its host.
 before=changes
 chart.SigTasksChanged(onGanttTasksChanged)
 chart.SetTasksJSON(chart.TasksJSON())
 if changes!=before+1 { t.Fatal("declarative host event hookup failed") }
}
`

// A newly dropped chart's sample tasks are date-relative. Unlike static widget
// defaults, their persisted value must remain in the generated constructor:
// omitting the setter silently moves the schedule when the program runs later.
func TestGanttCodegenKeepsDateRelativeDefaultTaskSnapshot(t *testing.T) {
	scene := NewGedScene()
	scene.SetSize(260, 160)
	chart := addNamedWidget(t, scene, "gui.GanttChart", "Schedule", 10)
	want := chart.Widget().(*gui.GanttChart).TasksJSON()
	generated := scene.GenerateCodeIndexed(CodeGenOptions{PackageName: "ganttfixture", TypeName: "GanttFixtureUI"})
	if generated.Err != nil {
		t.Fatal(generated.Err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "generated.go", generated.Code, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := ""
	calls := 0
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		method, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || method.Sel.Name != "SetTasksJSON" {
			return true
		}
		calls++
		if len(call.Args) != 1 {
			t.Fatal("invalid task setter arity")
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			t.Fatal("task setter does not contain a persisted literal")
		}
		got, err = strconv.Unquote(literal.Value)
		if err != nil {
			t.Fatal(err)
		}
		return true
	})
	if calls != 1 || got != want {
		t.Fatalf("default task snapshot omitted or changed: SetTasksJSON calls=%d; generated program would use its execution date instead of the saved schedule", calls)
	}
}
