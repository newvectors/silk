package ged

import (
	"reflect"
	"strings"
	"testing"

	"github.com/uk0/silk/core"
	"github.com/uk0/silk/gui"
)

const workflowTasksJSON = `[
 {"id":"project","name":"发布项目","collapsed":false},
 {"id":"design","parentId":"project","name":"设计 \\\"控件\\\"","start":"2026-04-06","end":"2026-04-09","progress":0.5,"owner":"张三","description":"line1\n@ref:literal"},
 {"id":"build","parentId":"project","name":"实现","start":"2026-04-09","end":"2026-04-13","dependencies":["design"]},
 {"id":"ship","parentId":"project","name":"发布","start":"2026-04-13","end":"2026-04-13","milestone":true,"dependencies":["build"]}
]`

func TestWorkflowWidgetsReachPaletteAndRealCanvasDrop(t *testing.T) {
	palette := NewWidgetList()
	for _, factory := range []string{"gui.GanttChart", "gui.PrintButton"} {
		found := false
		for i := 0; i < palette.Count(); i++ {
			if palette.Item(i).Data == factory {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s not offered by widget palette", factory)
		}
		view := NewGedView()
		view.GedScene().SetSize(240, 160)
		x, y := view.MapFromScene(10, 10)
		ctx := &dropCtx{pa: gui.DndCopy, payload: factory}
		view.OnDrop(x, y, ctx)
		if len(view.Scene().Children()) != 1 {
			t.Fatalf("%s did not drop", factory)
		}
		item := view.Scene().Children()[0].(*FakeWidget)
		if item.WidgetFactoryName() != factory {
			t.Fatalf("dropped %s instead of %s", item.WidgetFactoryName(), factory)
		}
		w, h := defaultSizeForWidget(factory)
		if item.Width() != w || item.Height() != h {
			t.Fatalf("%s has unusable drop geometry %v", factory, item.Bounds1())
		}
		if factory == "gui.GanttChart" && len(item.Widget().(*gui.GanttChart).Tasks()) == 0 {
			t.Fatal("factory-built Gantt is empty")
		}
	}
}

func TestWorkflowWidgetPropertiesSurviveSaveLoadAndPreview(t *testing.T) {
	for _, factory := range []string{"gui.GanttChart", "gui.PrintButton"} {
		src, err := NewFakeWidgetFromFactory(factory)
		if err != nil {
			t.Fatal(err)
		}
		switch w := src.Widget().(type) {
		case *gui.GanttChart:
			if err := w.LoadTasksJSON(workflowTasksJSON); err != nil {
				t.Fatal(err)
			}
			w.SetDayWidth(42.5)
			w.SetRowHeight(37)
			w.SetTableWidth(510)
			w.SetReadOnly(true)
			w.SetCascadeDependencies(false)
		case *gui.PrintButton:
			w.SetDocumentPath("reports/排期 \\\"v2\\\".pdf")
			w.SetPrinterName("office-printer")
			w.SetCopies(3)
			w.SetJobTitle("中文排期")
			w.SetText("打印排期")
		}
		want := captureWidgetProperties(src.Widget().(core.IEnumProperties))
		doc := src.SaveDesign()
		for round := 0; round < 3; round++ {
			parsed, err := core.LoadTDocStr(doc.String())
			if err != nil {
				t.Fatal(err)
			}
			dst, err := NewFakeWidgetFromFactory(factory)
			if err != nil {
				t.Fatal(err)
			}
			if err := dst.LoadDesign(parsed); err != nil {
				t.Fatal(err)
			}
			for phase, w := range map[string]gui.IWidget{"designer": dst.Widget(), "preview": dst.Generate()} {
				if got := captureWidgetProperties(w.(core.IEnumProperties)); !reflect.DeepEqual(got, want) {
					t.Fatalf("%s round%d %s properties lost:\n got=%v\nwant=%v", factory, round, phase, got, want)
				}
			}
			doc = dst.SaveDesign()
		}
	}
}

func TestWorkflowWidgetsGenerateConcreteTypesPropertiesAndEvents(t *testing.T) {
	scene := NewGedScene()
	scene.SetSize(240, 160)
	chart := addNamedWidget(t, scene, "gui.GanttChart", "Schedule", 5)
	g := chart.Widget().(*gui.GanttChart)
	if err := g.LoadTasksJSON(workflowTasksJSON); err != nil {
		t.Fatal(err)
	}
	g.SetDayWidth(42.5)
	g.SetTableWidth(510)
	g.SetReadOnly(true)
	g.SetCascadeDependencies(false)
	chart.SetEventHandler("OnTasksChanged", "onTasksChanged")
	button := addNamedWidget(t, scene, "gui.PrintButton", "PrintSchedule", 115)
	p := button.Widget().(*gui.PrintButton)
	p.SetDocumentPath("reports/schedule.pdf")
	p.SetPrinterName("office")
	p.SetCopies(3)
	p.SetJobTitle("Project report")
	button.SetEventHandler("OnPrintFinished", "onPrintFinished")
	generated := scene.GenerateCodeIndexed(CodeGenOptions{PackageName: "ui", TypeName: "WorkflowUI"})
	if generated.Err != nil {
		t.Fatal(generated.Err)
	}
	for _, want := range []string{"*gui.GanttChart", "gui.NewGanttChart()", ".SetTasksJSON(", ".SetDayWidth(42.5)", ".SetTableWidth(510)", ".SetReadOnly(true)", ".SetCascadeDependencies(false)", ".SigTasksChanged(", "*gui.PrintButton", "gui.NewPrintButton()", ".SetDocumentPath(", ".SetPrinterName(", ".SetCopies(3)", ".SetJobTitle(", ".SigPrintFinished("} {
		if !strings.Contains(generated.Code, want) {
			t.Errorf("generated workflow missing %q", want)
		}
	}
	if !testing.Short() {
		vetGeneratedFiles(t, map[string]string{"main.go": generated.Code, "handlers.go": "package ui\nfunc onTasksChanged() {}\nfunc onPrintFinished(jobID, errorText string) {}\n"})
	}
}
