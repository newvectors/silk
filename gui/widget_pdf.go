package gui

import (
	"fmt"
	"io"
	"math"
	"reflect"
	"runtime"

	"github.com/uk0/silk/paint"
	"github.com/uk0/silk/pdfexport"
)

// RenderWidgetPDF renders one current widget viewport into a single-page PDF.
// Call on the UI thread. It does not open a printer, change the widget geometry,
// scroll hidden rows into view, or modify task data. 96-DPI logical dimensions
// become 72-DPI PDF points; the display's Retina scale is deliberately ignored.
//
// The page embeds a lossless 288-DPI Cairo raster to retain the same resolved
// fonts (including CJK), icons, clipping and exact 8-bit sRGB colors as the UI.
// Text is not selectable, and a printer/driver may apply its own color profile.
func RenderWidgetPDF(out io.Writer, widget IWidget) error {
	if out == nil || widget == nil || (reflect.ValueOf(widget).Kind() == reflect.Pointer && reflect.ValueOf(widget).IsNil()) {
		return fmt.Errorf("print: widget and output are required")
	}
	w, h := widget.Width(), widget.Height()
	const scale = 3.0
	pw, ph := math.Ceil(w*scale), math.Ceil(h*scale)
	if math.IsNaN(w) || math.IsNaN(h) || math.IsInf(w, 0) || math.IsInf(h, 0) ||
		w <= 0 || h <= 0 || pw*ph > 32*1024*1024 || pw > 32768 || ph > 32768 {
		return fmt.Errorf("print: invalid or oversized widget viewport %g x %g", w, h)
	}
	pix := paint.NewPixmap(int(pw), int(ph))
	p := pix.NewPainter()
	p.SetBrush1(paint.Color{R: 255, G: 255, B: 255, A: 255})
	p.Rectangle(0, 0, pw, ph)
	p.Fill()
	p.Scale(scale, scale)
	DrawWidgetAll(widget, p, 0, 0, 0, 0, w, h)
	pix.Flush()
	if _, err := pix.Image(); err != nil {
		return fmt.Errorf("print: rasterize widget: %w", err)
	}
	pdf := pdfexport.New(w*72/96, h*72/96)
	pdf.SetCompression(true)
	pdf.DrawPixmap5(0, 0, w*72/96, h*72/96, pix)
	_, err := pdf.WriteTo(out)
	runtime.KeepAlive(p)
	runtime.KeepAlive(pix)
	return err
}
