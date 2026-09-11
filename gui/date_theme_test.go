package gui

import (
	"fmt"
	"image"
	"testing"
	"time"

	"github.com/uk0/silk/paint"
)

// Capture the text ink while still forwarding every operation to the real
// Cairo painter. Pixel checks then verify the surface actually beneath it.
type dateTextPaint struct {
	paint.Painter
	brush paint.Color
	texts []datePaintedText
}

type datePaintedText struct {
	text string
	ink  paint.Color
}

func (r *dateTextPaint) SetBrush1(c paint.Color) {
	r.brush = c
	r.Painter.SetBrush1(c)
}
func (r *dateTextPaint) DrawText(text string) {
	r.texts = append(r.texts, datePaintedText{text, r.brush})
	r.Painter.DrawText(text)
}
func (r *dateTextPaint) DrawText1(x, y float64, text string) {
	r.texts = append(r.texts, datePaintedText{text, r.brush})
	r.Painter.DrawText1(x, y, text)
}

func renderDateWidget(t *testing.T, widget IWidget, scale int) (image.Image, *dateTextPaint) {
	t.Helper()
	w, h := widget.Size()
	pix := paint.NewPixmap(int(w)*scale, int(h)*scale)
	g := pix.NewPainter()
	g.Scale(float64(scale), float64(scale))
	rec := &dateTextPaint{Painter: g}
	widget.Draw(rec)
	img, err := pix.Image()
	if err != nil {
		t.Fatal(err)
	}
	return img, rec
}

func assertDatePixel(t *testing.T, img image.Image, scale, x, y int, want paint.Color) paint.Color {
	t.Helper()
	got := paint.ColorModel.Convert(img.At(x*scale, y*scale)).(paint.Color)
	if got != want {
		t.Errorf("pixel (%d,%d) at %dx = %v, want exact token %v", x, y, scale, got, want)
	}
	return got
}

func assertDateText(t *testing.T, rec *dateTextPaint, index int, text string, ink, actualBG paint.Color) {
	t.Helper()
	if index >= len(rec.texts) {
		t.Fatalf("missing text %q at index %d: %v", text, index, rec.texts)
	}
	got := rec.texts[index]
	if got.text != text || got.ink != ink {
		t.Errorf("text[%d] = %q / %v, want %q / exact token %v", index, got.text, got.ink, text, ink)
	}
	if ratio := contrastRatio(got.ink, actualBG); ratio < 4.5 {
		t.Errorf("rendered text %q contrast %.2f:1 < 4.5 against actual pixel %v", got.text, ratio, actualBG)
	}
}

func TestDatePickerDrawUsesReadableThemeColors(t *testing.T) {
	previousMode, previousPalette, previousHover := CurrentThemeMode(), paint.CurrentIconPalette(), mouseHoverWidget
	t.Cleanup(func() {
		SetThemeMode(previousMode)
		paint.SetIconPalette(previousPalette)
		mouseHoverWidget = previousHover
	})
	for _, mode := range []ThemeMode{ThemeLight, ThemeDark} {
		SetThemeMode(mode)
		for _, scale := range []int{1, 2} {
			for _, state := range []string{"normal", "hover", "pressed"} {
				t.Run(fmt.Sprintf("theme-%v/%dx/%s", mode, scale, state), func(t *testing.T) {
					d := NewDatePicker()
					d.SetDate(2026, 3, 15)
					d.SetSize(150, 28)
					mouseHoverWidget = nil
					if state == "hover" {
						mouseHoverWidget = d
					}
					d.pushed = state == "pressed"
					wantBG := Theme().ViewBGColor
					if state != "normal" {
						wantBG = Theme().ButtonHoverBGColor
					}
					img, rec := renderDateWidget(t, d, scale)
					bg := assertDatePixel(t, img, scale, 100, 3, wantBG)
					assertDateText(t, rec, 0, "2026-03-15", Theme().TextColor, bg)
					assertDatePixel(t, img, scale, 137, 13, Theme().TextColor)
				})
			}
		}
	}
}

func TestDateCalendarsDrawUsesReadableThemeColors(t *testing.T) {
	previousMode, previousPalette := CurrentThemeMode(), paint.CurrentIconPalette()
	t.Cleanup(func() { SetThemeMode(previousMode); paint.SetIconPalette(previousPalette) })
	for _, mode := range []ThemeMode{ThemeLight, ThemeDark} {
		SetThemeMode(mode)
		th := Theme()
		accent := paint.CurrentIconPalette().Accent
		selectedInk := paint.CurrentIconPalette().Paper
		if mode == ThemeDark {
			selectedInk = th.ViewBGColor
		}
		for _, scale := range []int{1, 2} {
			t.Run(fmt.Sprintf("popup/theme-%v/%dx", mode, scale), func(t *testing.T) {
				owner := NewDatePicker()
				owner.SetDate(2026, 3, 15)
				popup := newDateCalendarPopup(owner)
				popup.SetSize(calCellW*calCols+8, calHeaderH+calDayRowH+calCellH*calRows+8)
				popup.hoverDay = 16
				img, rec := renderDateWidget(t, popup, scale)
				bg := assertDatePixel(t, img, scale, 230, 195, th.ViewBGColor)
				selectedBG := assertDatePixel(t, img, scale, 200, 100, accent)
				hoverBG := assertDatePixel(t, img, scale, 8, 124, th.ButtonHoverBGColor)
				assertDateText(t, rec, 0, "2026-03", th.TextColor, bg)
				assertDateText(t, rec, 1, "Mo", th.TabTextColor, bg)
				assertDateText(t, rec, 8, "1", th.TextColor, bg)
				assertDateText(t, rec, 22, "15", selectedInk, selectedBG)
				assertDateText(t, rec, 23, "16", th.TextColor, hoverBG)
			})
			t.Run(fmt.Sprintf("inline/theme-%v/%dx", mode, scale), func(t *testing.T) {
				cal := NewCalendar()
				cal.SetSelectedDate(time.Date(2026, time.March, 15, 0, 0, 0, 0, time.Local))
				cal.SetSize(232, 236)
				// Hover an adjacent-month cell too: its subdued weekday ink
				// must not fall below 4.5:1 on the hover wash.
				cal.hoverRow, cal.hoverCol = 5, 2 // April 1
				img, rec := renderDateWidget(t, cal, scale)
				bg := assertDatePixel(t, img, scale, 230, 234, th.ViewBGColor)
				selectedBG := assertDatePixel(t, img, scale, 203, 117, accent)
				hoverBG := assertDatePixel(t, img, scale, 75, 207, th.ButtonHoverBGColor)
				assertDateText(t, rec, 0, "2026年3月", th.TextColor, bg)
				assertDateText(t, rec, 1, "一", th.TabTextColor, bg)
				assertDateText(t, rec, 8, "23", th.TabTextColor, bg)
				assertDateText(t, rec, 14, "1", th.TextColor, bg)
				assertDateText(t, rec, 28, "15", selectedInk, selectedBG)
				assertDateText(t, rec, 45, "1", th.TextColor, hoverBG)
			})
		}
	}
}
