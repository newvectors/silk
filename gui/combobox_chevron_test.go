package gui

import (
	"testing"

	"github.com/uk0/silk/paint"
)

type comboChevronPoint struct{ x, y float64 }
type comboChevronStroke struct {
	points []comboChevronPoint
	ink    paint.Color
	width  float64
}
type comboChevronRecorder struct {
	*themeFillRecorder
	path    []comboChevronPoint
	strokes []comboChevronStroke
}

func (r *comboChevronRecorder) MoveTo(x, y float64) {
	r.path = []comboChevronPoint{{x, y}}
}
func (r *comboChevronRecorder) LineTo(x, y float64) {
	r.path = append(r.path, comboChevronPoint{x, y})
}
func (r *comboChevronRecorder) Stroke() {
	r.strokes = append(r.strokes, comboChevronStroke{append([]comboChevronPoint(nil), r.path...), r.pen, r.penW})
	r.path = nil
}

func TestComboBoxChevronStaysInReservedAreaAndFollowsTheme(t *testing.T) {
	previous := CurrentThemeMode()
	t.Cleanup(func() { SetThemeMode(previous) })
	for _, mode := range []ThemeMode{ThemeLight, ThemeDark} {
		SetThemeMode(mode)
		for _, enabled := range []bool{true, false} {
			combo := NewComboBox()
			combo.SetSize(160, 32)
			combo.SetEnabled(enabled)
			editBefore := combo.EditWidget().Bounds1()
			r := &comboChevronRecorder{themeFillRecorder: newThemeFillRecorder()}
			combo.Draw(r)
			var chevrons []comboChevronStroke
			for _, stroke := range r.strokes {
				if len(stroke.points) == 3 {
					chevrons = append(chevrons, stroke)
				}
			}
			if len(chevrons) != 1 {
				t.Fatalf("theme %v enabled=%v: %d chevrons, want 1", mode, enabled, len(chevrons))
			}
			arrow := chevrons[0]
			wantInk := Theme().TextColor
			if !enabled {
				wantInk = paint.CurrentIconPalette().Disabled
			}
			if arrow.ink != wantInk || arrow.width != 1.5 {
				t.Fatalf("theme %v enabled=%v: pen=%v width=%v, want %v/1.5", mode, enabled, arrow.ink, arrow.width, wantInk)
			}
			for _, point := range arrow.points {
				if point.x-arrow.width/2 < editBefore.Right() || point.x+arrow.width/2 > combo.Width()-Theme().EditPadding.R || point.y-arrow.width/2 < 0 || point.y+arrow.width/2 > combo.Height() {
					t.Fatalf("chevron intersects editor/border: %+v, edit=%v", arrow, editBefore)
				}
			}
			if arrow.points[0].y != arrow.points[2].y || arrow.points[1].y <= arrow.points[0].y {
				t.Fatal("chevron does not point downward")
			}
			if combo.EditWidget().Bounds1() != editBefore {
				t.Fatal("drawing altered the editor's layout")
			}
		}
	}
}

func TestComboBoxChevronHasRealThemeColoredPixels(t *testing.T) {
	previous := CurrentThemeMode()
	t.Cleanup(func() { SetThemeMode(previous) })
	for _, mode := range []ThemeMode{ThemeLight, ThemeDark} {
		SetThemeMode(mode)
		for _, enabled := range []bool{true, false} {
			combo := NewComboBox()
			combo.SetSize(160, 32)
			combo.SetEnabled(enabled)
			pix := paint.NewPixmap(160, 32)
			combo.Draw(pix.NewPainter())
			img, err := pix.Image()
			if err != nil {
				t.Fatal(err)
			}
			ink := Theme().TextColor
			if !enabled {
				ink = paint.CurrentIconPalette().Disabled
			}
			colored := 0
			for y := 8; y < 24; y++ {
				for x := int(combo.ClientRect().Right()) + 1; x < 153; x++ {
					if paint.ColorModel.Convert(img.At(x, y)).(paint.Color) == ink {
						colored++
					}
				}
			}
			if colored == 0 {
				t.Fatalf("theme %v enabled=%v: no real chevron pixels in reserved dropdown area", mode, enabled)
			}
			if got := paint.ColorModel.Convert(img.At(120, 16)).(paint.Color); got != Theme().ViewBGColor {
				t.Fatalf("chevron painted into editable area: %v", got)
			}
		}
	}
}

func TestComboBoxChevronDoesNotOverflowTinyBounds(t *testing.T) {
	for _, size := range [][2]float64{{8, 32}, {160, 7}, {0, 0}} {
		combo := NewComboBox()
		combo.SetSize(size[0], size[1])
		r := &comboChevronRecorder{themeFillRecorder: newThemeFillRecorder()}
		combo.Draw(r)
		for _, stroke := range r.strokes {
			if len(stroke.points) == 3 {
				t.Fatalf("unfittable chevron drawn outside tiny bounds %v", size)
			}
		}
	}
}
