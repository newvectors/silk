package ged

import (
	"github.com/uk0/silk/gui"
	"github.com/uk0/silk/paint"
	"testing"
)

// TestWidgetListSCADACategory verifies the industrial/SCADA widgets are bucketed
// into their own palette category (组态 (SCADA)) rather than falling into the
// generic "其他 (Other)" group, so an HMI builder finds them grouped together.
func TestWidgetListSCADACategory(t *testing.T) {
	const catName = "组态 (SCADA)"
	want := []string{"gui.Tank", "gui.Indicator", "gui.DigitalDisplay", "gui.Valve", "gui.Pipe"}

	var members []string
	found := false
	for _, cd := range categoryDefs {
		if cd.name == catName {
			found = true
			members = cd.names
			break
		}
	}
	if !found {
		t.Fatalf("palette category %q not found in categoryDefs", catName)
	}

	member := map[string]bool{}
	for _, n := range members {
		member[n] = true
	}
	for _, w := range want {
		if !member[w] {
			t.Errorf("widget %q not mapped to category %q", w, catName)
		}
	}
}

// Observe the real paint path, not just a detached color helper.
type widgetListBrushRecorder struct {
	paint.Painter
	brushes []paint.Color
}

func (r *widgetListBrushRecorder) SetBrush1(c paint.Color) {
	r.brushes = append(r.brushes, c)
	r.Painter.SetBrush1(c)
}
func TestWidgetListUsesWorkbenchTheme(t *testing.T) {
	previous := gui.CurrentThemeMode()
	defer gui.SetThemeMode(previous)
	for _, mode := range []gui.ThemeMode{gui.ThemeLight, gui.ThemeDark} {
		gui.SetThemeMode(mode)
		w := NewWidgetList()
		w.SetBounds(0, 0, 220, 140)
		w.SetActiveIndex(1)
		pix := paint.NewPixmap(220, 140)
		r := &widgetListBrushRecorder{Painter: pix.NewPainter()}
		w.Draw(r)
		theme := gui.Theme()
		for _, want := range []paint.Color{theme.ViewBGColor, theme.FormLightColor, theme.TextColor, theme.MenuActiveBGColor, theme.MenuActiveTextColor} {
			found := false
			for _, got := range r.brushes {
				if got == want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("mode %v: palette did not use theme color %v", mode, want)
			}
		}
	}
}
