package paint

import "github.com/uk0/silk/cairo"

func init() {
	proceduralFallbacks["printer"] = drawPrinter
	proceduralFallbacks["gantt"] = drawGantt
}

// Workflow icons use the same semantic sRGB palette and device-transform
// rasterization as the rest of the workbench, including disabled rendering.
func drawPrinter(size int, cc *cairo.Context) {
	w := float64(size)
	setIconStroke(cc, size)
	cc.Rectangle(w*.25, w*.09, w*.5, w*.25)
	cc.Stroke()
	cc.MoveTo(w*.25, w*.73)
	cc.LineTo(w*.11, w*.73)
	cc.LineTo(w*.11, w*.35)
	cc.LineTo(w*.89, w*.35)
	cc.LineTo(w*.89, w*.73)
	cc.LineTo(w*.75, w*.73)
	cc.Stroke()
	setIconColor(cc, currentIconPalette.Accent)
	cc.Rectangle(w*.25, w*.59, w*.5, w*.3)
	cc.Stroke()
	setIconColor(cc, currentIconPalette.Success)
	cc.Arc(w*.75, w*.47, w*.042, 0, 6.283185307179586)
	cc.Fill()
}

func drawGantt(size int, cc *cairo.Context) {
	w := float64(size)
	setIconStroke(cc, size)
	cc.MoveTo(w*.12, w*.12)
	cc.LineTo(w*.12, w*.87)
	cc.LineTo(w*.91, w*.87)
	cc.Stroke()
	setIconColor(cc, currentIconPalette.Success)
	cc.Rectangle(w*.24, w*.16, w*.61, w*.15)
	cc.Fill()
	setIconColor(cc, currentIconPalette.Accent)
	cc.Rectangle(w*.3, w*.4, w*.29, w*.15)
	cc.Fill()
	cc.Rectangle(w*.54, w*.64, w*.35, w*.15)
	cc.Fill()
}
