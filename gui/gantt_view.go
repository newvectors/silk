package gui

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/uk0/silk/geom"
	"github.com/uk0/silk/i18n"
	"github.com/uk0/silk/paint"
)

type ganttLayout struct{ w, h, split, top, bottom, row, timelineW, bodyH float64 }

func (c *GanttChart) geometry() ganttLayout {
	w, h := c.Size()
	w = math.Max(0, w)
	h = math.Max(0, h)
	fh := Theme().Font.FontExtents().Height
	top := math.Min(h, math.Max(48, 2*(fh+7)))
	bottom := math.Max(top, h-math.Max(24, fh+8)-10)
	// Reserve a real timeline even at small bounds. The table clips individual
	// cells rather than letting long labels cover the calendar.
	split := math.Min(c.tableWidth, math.Max(0, w*.62))
	if w >= 400 {
		split = math.Min(c.tableWidth, w-180)
	}
	return ganttLayout{w, h, split, top, bottom, math.Max(c.rowHeight, fh+10), math.Max(0, w-split-10), math.Max(0, bottom-top)}
}
func (c *GanttChart) clampScroll() {
	l := c.geometry()
	c.scrollY = ganttClamp(c.scrollY, 0, math.Max(0, float64(len(c.rows))*l.row-l.bodyH))
	c.scrollX = ganttClamp(c.scrollX, 0, math.Max(0, c.contentWidth()-l.timelineW))
}
func (c *GanttChart) contentWidth() float64 {
	return math.Max(1, float64(GanttDaysBetween(c.start, c.end))) * c.dayWidth
}
func (c *GanttChart) visibleRange(l ganttLayout) (int, int) {
	first := int(c.scrollY / l.row)
	first = max(0, min(first, len(c.rows)))
	last := min(len(c.rows), int(math.Ceil((c.scrollY+l.bodyH)/l.row)))
	return first, max(first, last)
}
func (c *GanttChart) taskRect(t GanttTask, row int, l ganttLayout) geom.Rect {
	x := l.split + float64(GanttDaysBetween(c.start, t.Start))*c.dayWidth - c.scrollX
	y := l.top + float64(row)*l.row - c.scrollY + 6
	w := float64(GanttDaysBetween(t.Start, t.End)) * c.dayWidth
	if t.Milestone {
		w = l.row - 14
		x -= w / 2
	}
	return geom.Rect{X: x, Y: y, Width: math.Max(2, w), Height: l.row - 12}
}

type ganttColors struct{ bg, header, text, muted, grid, weekend, selection, summary, leaf, barText, progress, milestone, dependency, today paint.Color }

func ganttThemeColors() ganttColors {
	t := Theme()
	p := paint.CurrentIconPalette()
	c := ganttColors{bg: t.ViewBGColor, header: t.FormColor, text: t.TextColor, muted: t.TabTextColor, grid: t.BorderColor, summary: p.Success, milestone: p.Violet, dependency: p.Accent, today: p.Danger}
	if CurrentThemeMode() == ThemeDark {
		c.weekend = paint.Color{R: 37, G: 44, B: 57, A: 255}
		c.selection = paint.Color{R: 37, G: 57, B: 81, A: 255}
		c.leaf = paint.Color{R: 34, G: 211, B: 238, A: 255}
		c.barText = paint.Color{R: 8, G: 32, B: 40, A: 255}
		c.progress = paint.Color{R: 14, G: 116, B: 144, A: 255}
	} else {
		c.weekend = paint.Color{R: 247, G: 248, B: 251, A: 255}
		c.selection = paint.Color{R: 230, G: 240, B: 255, A: 255}
		c.leaf = paint.Color{R: 14, G: 116, B: 144, A: 255}
		c.barText = paint.Color{R: 255, G: 255, B: 255, A: 255}
		c.progress = paint.Color{R: 8, G: 81, B: 101, A: 255}
	}
	return c
}
func ganttFill(g paint.Painter, col paint.Color, x, y, w, h float64) {
	if w <= 0 || h <= 0 {
		return
	}
	g.SetBrush1(col)
	g.Rectangle(x, y, w, h)
	g.Fill()
}
func ganttLine(g paint.Painter, col paint.Color, width, x1, y1, x2, y2 float64) {
	g.SetPen1(col, width)
	g.MoveTo(x1, y1)
	g.LineTo(x2, y2)
	g.Stroke()
}
func ganttClip(g paint.Painter, x, y, w, h float64) int {
	s := g.Save()
	g.Rectangle(x, y, math.Max(0, w), math.Max(0, h))
	g.Clip()
	return s
}

// Bounded measured ellipsis, preserving Unicode and never painting into the
// next cell. A text run need not shape megabytes to display one short cell.
func ganttElide(f paint.Font, s string, w float64) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > 512 {
		r = r[:512]
		s = string(r) + "…"
	}
	if f.TextExtents(s).Width <= w {
		return s
	}
	mark := "…"
	if f.TextExtents(mark).Width > w {
		return ""
	}
	lo, hi := 0, len(r)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if f.TextExtents(string(r[:mid])+mark).Width <= w {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(r[:lo]) + mark
}
func ganttText(g paint.Painter, text string, col paint.Color, f paint.Font, x, y, w, h float64) {
	if w < 4 || h < 4 {
		return
	}
	s := ganttClip(g, x, y, w, h)
	defer g.RestoreTo(s)
	text = ganttElide(f, text, w-8)
	if text == "" {
		return
	}
	fe := f.FontExtents()
	te := f.TextExtents(text)
	g.SetFont(f)
	g.SetBrush1(col)
	g.DrawText1(x+4-te.XBearing, y+(h-fe.Height)/2+fe.Ascent, text)
}
func (c *GanttChart) tableColumns(l ganttLayout) []float64 {
	// Keep calendar dates readable, including bold summary rows. Proportional
	// shrinking used to turn both dates into indistinguishable "2026-0…".
	// Drop optional trailing columns instead of truncating essential dates.
	f := Theme().Font
	b := f.Dup()
	b.SetBold(true)
	measure := func(s string) float64 {
		return math.Ceil(math.Max(f.TextExtents(s).Width, b.TextExtents(s).Width)) + 10
	}
	nameMin := math.Max(108, measure(i18n.T("Task"))+20)
	date := math.Max(measure("8888-88-88"), math.Max(measure(i18n.T("Start")), measure(i18n.T("Finish"))))
	progress := math.Max(measure("100%"), measure(i18n.T("Progress")))
	owner := math.Max(64, measure(i18n.T("Owner")))
	if l.split < nameMin+2*date {
		return []float64{l.split}
	}
	cols := []float64{0, date, date}
	remaining := l.split - 2*date
	if remaining >= nameMin+progress {
		cols = append(cols, progress)
		remaining -= progress
		if remaining >= nameMin+owner {
			cols = append(cols, owner)
			remaining -= owner
		}
	}
	cols[0] = remaining
	return cols
}

func (c *GanttChart) Draw(g paint.Painter) {
	if c.drag != nil && !c.HasCapture() {
		c.CancelInteraction()
	}
	l := c.geometry()
	if l.w <= 0 || l.h <= 0 {
		return
	}
	s := ganttClip(g, 0, 0, l.w, l.h)
	defer g.RestoreTo(s)
	p := ganttThemeColors()
	f := Theme().Font
	ganttFill(g, p.bg, 0, 0, l.w, l.h)
	body := ganttClip(g, 0, l.top, l.w-10, l.bodyH)
	first, last := c.visibleRange(l)
	// Date culling is independent of the total project span (including decades).
	dayFirst := int(math.Floor(c.scrollX / c.dayWidth))
	dayLast := dayFirst + int(math.Ceil(l.timelineW/c.dayWidth)) + 1
	for d := dayFirst; d <= dayLast; d++ {
		date := c.start.AddDate(0, 0, d)
		x := l.split + float64(d)*c.dayWidth - c.scrollX
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			ganttFill(g, p.weekend, math.Max(l.split, x), l.top, math.Min(x+c.dayWidth, l.w-10)-math.Max(l.split, x), l.bodyH)
		}
	}
	for i := first; i < last; i++ {
		y := l.top + float64(i)*l.row - c.scrollY
		if c.rows[i].Task.ID == c.selectedID {
			ganttFill(g, p.selection, 0, y, l.w-10, l.row)
		}
		ganttLine(g, p.grid, 1, 0, math.Floor(y+l.row)+.5, l.w-10, math.Floor(y+l.row)+.5)
	}
	cal := ganttClip(g, l.split, l.top, l.timelineW, l.bodyH)
	for d := dayFirst; d <= dayLast; d++ {
		x := math.Floor(l.split+float64(d)*c.dayWidth-c.scrollX) + .5
		ganttLine(g, p.grid, 1, x, l.top, x, l.bottom)
	}
	// Dependencies lie behind bars. Only incoming edges of visible rows are
	// visited; an offscreen endpoint is clipped to this viewport, not omitted.
	for i := first; i < last; i++ {
		c.drawDependencies(g, i, l, p)
	}
	for i := first; i < last; i++ {
		c.drawBar(g, i, l, p, f)
	}
	todayX := l.split + float64(GanttDaysBetween(c.start, GanttDate(time.Now())))*c.dayWidth - c.scrollX
	if todayX >= l.split && todayX < l.w-10 {
		ganttLine(g, p.today, 1.5, todayX, l.top, todayX, l.bottom)
	}
	g.RestoreTo(cal)
	left := ganttClip(g, 0, l.top, l.split, l.bodyH)
	cols := c.tableColumns(l)
	for i := first; i < last; i++ {
		r := c.rows[i]
		y := l.top + float64(i)*l.row - c.scrollY
		ff := f
		if r.HasChildren {
			ff = f.Dup()
			ff.SetBold(true)
		}
		end := r.Task.End
		if !r.Task.Milestone {
			end = end.AddDate(0, 0, -1)
		}
		values := []string{r.Task.Name, r.Task.Start.Format("2006-01-02"), end.Format("2006-01-02"), fmt.Sprintf("%.0f%%", r.Task.Progress*100), r.Task.Owner}
		x := 0.0
		for j, w := range cols {
			tx, tw := x, w
			if j == 0 {
				indent := math.Min(float64(r.Depth)*14, math.Max(0, w-44))
				tx += indent + 20
				tw -= indent + 20
				if r.HasChildren {
					c.drawExpander(g, indent+9, y+l.row/2, r.Task.Collapsed, p.muted)
				}
			}
			ganttText(g, values[j], p.text, ff, tx, y, tw, l.row)
			x += w
			ganttLine(g, p.grid, 1, math.Floor(x)+.5, y, math.Floor(x)+.5, y+l.row)
		}
	}
	g.RestoreTo(left)
	g.RestoreTo(body)
	// Header is painted after the scrolling body, so no bar/text can cover it.
	ganttFill(g, p.header, 0, 0, l.w, l.top)
	x := 0.0
	labels := []string{i18n.T("Task"), i18n.T("Start"), i18n.T("Finish"), i18n.T("Progress"), i18n.T("Owner")}
	for i, w := range cols {
		ganttText(g, labels[i], p.muted, f, x, 0, w, l.top)
		x += w
		ganttLine(g, p.grid, 1, x, 0, x, l.top)
	}
	c.drawCalendarHeader(g, l, p, f, dayFirst, dayLast, todayX)
	ganttLine(g, p.grid, 1, 0, l.top-.5, l.w, l.top-.5)
	ganttLine(g, p.grid, 1.5, l.split, 0, l.split, l.bottom)
	c.drawScrollbars(g, l, p)
	footer := i18n.T("Drag task | edges: resize | bottom: progress | Ctrl/Cmd + wheel: zoom")
	if c.readOnly {
		footer = i18n.T("Read-only | wheel: scroll | Ctrl/Cmd + wheel: zoom")
	}
	if c.selectedID != "" {
		if t, ok := c.taskIndex[c.selectedID]; ok {
			footer = t.Name + " | " + t.Start.Format("2006-01-02") + " -> " + t.End.Format("2006-01-02") + " (" + i18n.T("end exclusive") + ")"
		}
	}
	ink := p.muted
	if c.lastError != nil {
		footer = c.lastError.Error()
		ink = p.today
	}
	ganttFill(g, p.header, 0, l.bottom+10, l.w, l.h-l.bottom-10)
	ganttText(g, footer, ink, f, 4, l.bottom+10, l.w-8, l.h-l.bottom-10)
	g.SetPen1(p.grid, 1)
	g.Rectangle(.5, .5, math.Max(0, l.w-1), math.Max(0, l.h-1))
	g.Stroke()
}
func (c *GanttChart) drawExpander(g paint.Painter, x, y float64, collapsed bool, col paint.Color) {
	g.SetPen1(col, 1.5)
	if collapsed {
		g.MoveTo(x-2, y-4)
		g.LineTo(x+2, y)
		g.LineTo(x-2, y+4)
	} else {
		g.MoveTo(x-4, y-2)
		g.LineTo(x, y+2)
		g.LineTo(x+4, y-2)
	}
	g.Stroke()
}
func (c *GanttChart) drawBar(g paint.Painter, i int, l ganttLayout, p ganttColors, f paint.Font) {
	r := c.rows[i]
	b := c.taskRect(r.Task, i, l)
	if b.Right() < l.split || b.X > l.w {
		return
	}
	col := p.leaf
	if r.HasChildren {
		col = p.summary
	}
	if !c.IsEnabled() {
		col = paint.CurrentIconPalette().Disabled
	}
	if r.Task.Milestone {
		cx, cy := b.X+b.Width/2, b.Y+b.Height/2
		radius := math.Min(b.Width, b.Height) / 2
		g.SetBrush1(p.milestone)
		g.MoveTo(cx, cy-radius)
		g.LineTo(cx+radius, cy)
		g.LineTo(cx, cy+radius)
		g.LineTo(cx-radius, cy)
		g.LineTo(cx, cy-radius)
		g.Fill()
		ganttText(g, r.Task.Name, p.text, f, cx+radius+3, b.Y, 150, b.Height)
		return
	}
	ganttFill(g, col, b.X, b.Y, b.Width, b.Height)
	if r.HasChildren {
		// Summary end caps are distinct from leaf resize handles.
		for _, x := range []float64{b.X, b.Right() - 5} {
			ganttFill(g, col, x, b.Bottom()-1, 5, 4)
		}
	}
	ganttFill(g, p.progress, b.X, b.Bottom()-4, b.Width*r.Task.Progress, 4)
	barFont := f
	if available := b.Height - 6; f.FontExtents().Height > available {
		barFont = f.Dup()
		barFont.SetSize(max(8, int(float64(f.Size())*available/f.FontExtents().Height)))
	}
	ganttText(g, r.Task.Name, p.barText, barFont, math.Max(b.X, l.split), b.Y, math.Min(b.Right(), l.w-10)-math.Max(b.X, l.split), b.Height-3)
	if r.Task.ID == c.selectedID {
		g.SetPen1(p.dependency, 2)
		g.Rectangle(b.X-1, b.Y-1, b.Width+2, b.Height+2)
		g.Stroke()
		if !c.readOnly && !r.HasChildren {
			for _, x := range []float64{b.X + 2, b.Right() - 3} {
				ganttLine(g, p.barText, 1, x, b.Y+3, x, b.Bottom()-6)
			}
			px := b.X + b.Width*r.Task.Progress
			ganttFill(g, p.barText, px-2, b.Bottom()-5, 4, 5)
		}
	}
}
func (c *GanttChart) drawDependencies(g paint.Painter, i int, l ganttLayout, p ganttColors) {
	target := c.rows[i].Task
	to := c.taskRect(target, i, l)
	for _, id := range target.Dependencies {
		j, ok := c.rowIndex[id]
		if !ok {
			continue
		}
		from := c.taskRect(c.rows[j].Task, j, l)
		x1, y1 := from.Right()+1, from.Y+from.Height/2
		x2, y2 := to.X-2, to.Y+to.Height/2
		if x1 < l.split && x2 < l.split || x1 > l.w && x2 > l.w {
			continue
		}
		// Bound offscreen coordinates before sending them to the rasterizer.
		x1 = ganttClamp(x1, l.split-16, l.w+16)
		x2 = ganttClamp(x2, l.split-16, l.w+16)
		y1 = ganttClamp(y1, l.top-16, l.bottom+16)
		g.SetPen1(p.dependency, 1.25)
		g.MoveTo(x1, y1)
		if x2 >= x1+8 {
			mid := x1 + math.Min(12, (x2-x1)/2)
			g.LineTo(mid, y1)
			g.LineTo(mid, y2)
		} else {
			midY := y2 - l.row/2
			g.LineTo(x1+7, y1)
			g.LineTo(x1+7, midY)
			g.LineTo(x2-7, midY)
			g.LineTo(x2-7, y2)
		}
		g.LineTo(x2, y2)
		g.Stroke()
		g.SetBrush1(p.dependency)
		g.MoveTo(x2, y2)
		g.LineTo(x2-5, y2-3)
		g.LineTo(x2-5, y2+3)
		g.LineTo(x2, y2)
		g.Fill()
	}
}
func (c *GanttChart) drawCalendarHeader(g paint.Painter, l ganttLayout, p ganttColors, f paint.Font, first, last int, todayX float64) {
	s := ganttClip(g, l.split, 0, l.timelineW, l.top)
	defer g.RestoreTo(s)
	half := l.top / 2
	monthStart := c.start.AddDate(0, 0, first)
	monthStart = time.Date(monthStart.Year(), monthStart.Month(), 1, 0, 0, 0, 0, time.UTC)
	// At the 4px minimum zoom this loops months visible on screen, not years
	// preceding the first viewport date.
	for date := monthStart; !date.After(c.start.AddDate(0, 0, last)); date = date.AddDate(0, 1, 0) {
		next := date.AddDate(0, 1, 0)
		x := l.split + float64(GanttDaysBetween(c.start, date))*c.dayWidth - c.scrollX
		right := l.split + float64(GanttDaysBetween(c.start, next))*c.dayWidth - c.scrollX
		ganttText(g, date.Format("2006-01"), p.text, f, math.Max(l.split, x), 0, math.Min(l.w-10, right)-math.Max(l.split, x), half)
		ganttLine(g, p.grid, 1, right, 0, right, half)
	}
	labelStride := max(1, int(math.Ceil(23/c.dayWidth)))
	for d := first; d <= last; d++ {
		date := c.start.AddDate(0, 0, d)
		x := l.split + float64(d)*c.dayWidth - c.scrollX
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			ganttFill(g, p.weekend, x, half, c.dayWidth, half)
		}
		if d%labelStride == 0 {
			ganttText(g, strconv.Itoa(date.Day()), p.muted, f, x, half, c.dayWidth*float64(labelStride), half)
		}
		ganttLine(g, p.grid, 1, x, half, x, l.top)
	}
	if todayX >= l.split && todayX < l.w-10 {
		ganttFill(g, p.today, todayX, half, 2, half)
	}
	ganttLine(g, p.grid, 1, l.split, half, l.w, half)
}
func (c *GanttChart) scrollThumbs(l ganttLayout) (geom.Rect, geom.Rect) {
	var h, v geom.Rect
	if total := c.contentWidth(); total > l.timelineW && l.timelineW > 0 {
		width := math.Min(l.timelineW, math.Max(24, l.timelineW*l.timelineW/total))
		x := l.split + c.scrollX/(total-l.timelineW)*(l.timelineW-width)
		h = geom.Rect{X: x, Y: l.bottom + 1, Width: width, Height: 8}
	}
	if total := float64(len(c.rows)) * l.row; total > l.bodyH && l.bodyH > 0 {
		height := math.Min(l.bodyH, math.Max(24, l.bodyH*l.bodyH/total))
		y := l.top + c.scrollY/(total-l.bodyH)*(l.bodyH-height)
		v = geom.Rect{X: l.w - 9, Y: y, Width: 8, Height: height}
	}
	return h, v
}
func (c *GanttChart) drawScrollbars(g paint.Painter, l ganttLayout, p ganttColors) {
	ganttFill(g, p.header, 0, l.bottom, l.w, 10)
	ganttFill(g, p.header, l.w-10, l.top, 10, l.bodyH)
	h, v := c.scrollThumbs(l)
	ganttFill(g, p.muted, h.X, h.Y, h.Width, h.Height)
	ganttFill(g, p.muted, v.X, v.Y, v.Width, v.Height)
}
