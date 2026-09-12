package gui

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/uk0/silk/paint"
)

type printPDFTestWidget struct {
	Widget
	draws int
}

func (w *printPDFTestWidget) Draw(p paint.Painter) {
	w.draws++
	p.SetBrush1(paint.Color{R: 19, G: 166, B: 227, A: 255})
	p.Rectangle(0, 0, w.Width(), w.Height())
	p.Fill()
	p.SetFont(Theme().Font)
	p.SetBrush1(Theme().TextColor)
	p.DrawText1(5, 20, "打印 / Gantt")
}

func TestRenderWidgetPDFUsesPhysicalUnitsAndEmbeddedRaster(t *testing.T) {
	w := new(printPDFTestWidget)
	w.Init(w)
	w.SetBounds(19, 27, 96, 48)
	before := w.Bounds1()
	var out bytes.Buffer
	if err := RenderWidgetPDF(&out, w); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, part := range []string{"%PDF-", "/MediaBox [0 0 72 36]", "/Subtype /Image", "/Width 288", "/Height 144", "%%EOF"} {
		if !strings.Contains(s, part) {
			t.Fatalf("PDF missing %q", part)
		}
	}
	if w.draws != 1 || w.Bounds1() != before {
		t.Fatal("printing changed widget geometry or skipped its Draw")
	}
}

type failingPDFWriter struct{}

func (failingPDFWriter) Write([]byte) (int, error) { return 0, errors.New("print output unavailable") }
func TestRenderWidgetPDFRejectsBadInputsAndWriterErrors(t *testing.T) {
	w := new(printPDFTestWidget)
	w.Init(w)
	w.SetSize(20, 20)
	if err := RenderWidgetPDF(failingPDFWriter{}, w); err == nil {
		t.Fatal("output failure hidden")
	}
	for _, size := range []float64{0, -1, math.NaN(), math.Inf(1), 1e9} {
		w.SetSize(size, 20)
		if err := RenderWidgetPDF(&bytes.Buffer{}, w); err == nil {
			t.Fatalf("invalid size %v accepted", size)
		}
	}
	var nilWidget *printPDFTestWidget
	if RenderWidgetPDF(&bytes.Buffer{}, nilWidget) == nil || RenderWidgetPDF(nil, w) == nil {
		t.Fatal("nil input accepted")
	}
}
