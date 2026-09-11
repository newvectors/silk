package gui

import (
	"github.com/uk0/silk/paint"
	"testing"
)

func TestThemeIconsHaveReadableSemanticColors(t *testing.T) {
	previous := CurrentThemeMode()
	defer SetThemeMode(previous)
	for _, mode := range []ThemeMode{ThemeLight, ThemeDark} {
		SetThemeMode(mode)
		palette := paint.CurrentIconPalette()
		want := paint.LightIconPalette()
		if mode == ThemeDark {
			want = paint.DarkIconPalette()
		}
		if palette != want {
			t.Fatal("workbench theme and icons diverged")
		}
		for _, ink := range []paint.Color{palette.Foreground, palette.Accent, palette.Success, palette.Warning, palette.Danger, palette.Violet} {
			for _, bg := range []paint.Color{Theme().FormColor, Theme().ViewBGColor, Theme().FormLightColor} {
				if ratio := contrastRatio(ink, bg); ratio < 3 {
					t.Errorf("mode %v: %v on %v contrast %.2f < 3", mode, ink, bg, ratio)
				}
			}
		}
	}
}

func TestDisabledButtonFaceUsesActiveTheme(t *testing.T) {
	previous := CurrentThemeMode()
	defer SetThemeMode(previous)
	for _, mode := range []ThemeMode{ThemeLight, ThemeDark} {
		SetThemeMode(mode)
		button := NewButton1("", nil)
		button.SetEnabled(false)
		button.SetSize(80, 32)
		pix := paint.NewPixmap(80, 32)
		painter := pix.NewPainter()
		button.Draw(painter)
		img, err := pix.Image()
		if err != nil {
			t.Fatal(err)
		}
		if got := paint.ColorModel.Convert(img.At(40, 16)).(paint.Color); got != Theme().FormLightColor {
			t.Fatalf("disabled button in mode %v: %v want %v", mode, got, Theme().FormLightColor)
		}
	}
}
