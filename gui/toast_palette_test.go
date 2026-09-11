package gui

import (
	"fmt"
	"testing"

	"github.com/uk0/silk/paint"
)

func restoreToastTestTheme(t *testing.T) {
	t.Helper()
	mode, palette := CurrentThemeMode(), paint.CurrentIconPalette()
	t.Cleanup(func() {
		SetThemeMode(mode)
		paint.SetIconPalette(palette)
	})
}

func TestToastPaletteUsesExactThemeTokensAndReadableContrast(t *testing.T) {
	restoreToastTestTheme(t)
	for _, mode := range []ThemeMode{ThemeLight, ThemeDark} {
		SetThemeMode(mode)
		palette := paint.CurrentIconPalette()
		for _, tc := range []struct {
			level ToastLevel
			want  paint.Color
		}{
			{ToastInfo, palette.Accent},
			{ToastSuccess, palette.Success},
			{ToastWarning, palette.Warning},
			{ToastError, palette.Danger},
			{ToastLevel(-1), palette.Foreground},
		} {
			t.Run(fmt.Sprintf("theme-%v/level-%v", mode, tc.level), func(t *testing.T) {
				bg, text := toastSurfaceColors(tc.level)
				wantBG, wantText := Theme().ViewBGColor, Theme().TextColor
				wantBG.A, wantText.A = 255, 255
				if bg != wantBG || text != wantText {
					t.Fatalf("toast surface/text = %v/%v, want exact theme tokens %v/%v", bg, text, wantBG, wantText)
				}
				accent := toastSemanticColor(tc.level)
				if accent != tc.want {
					t.Fatalf("toast accent = %v, want exact icon token %v", accent, tc.want)
				}
				if ratio := contrastRatio(text, bg); ratio < 4.5 {
					t.Errorf("13px toast text contrast %.2f < 4.5", ratio)
				}
				if ratio := contrastRatio(accent, bg); ratio < 3 {
					t.Errorf("toast status accent contrast %.2f < 3", ratio)
				}
			})
		}
	}
}

func TestToastSurfaceForcesOpaqueColorsWithoutMutatingTheme(t *testing.T) {
	restoreToastTestTheme(t)
	SetThemeMode(ThemeDark)
	theme := Theme()
	previousBG, previousText := theme.ViewBGColor, theme.TextColor
	t.Cleanup(func() { theme.ViewBGColor, theme.TextColor = previousBG, previousText })
	theme.ViewBGColor = paint.Color{R: 24, G: 25, B: 27, A: 100}
	theme.TextColor = paint.Color{R: 228, G: 230, B: 231, A: 150}
	bg, text := toastSurfaceColors(ToastSuccess)
	if bg != (paint.Color{R: 24, G: 25, B: 27, A: 255}) || text != (paint.Color{R: 228, G: 230, B: 231, A: 255}) {
		t.Fatalf("toast colors are still translucent or changed RGB: %v/%v", bg, text)
	}
	if theme.ViewBGColor.A != 100 || theme.TextColor.A != 150 {
		t.Fatal("toast opacity mutated the shared theme")
	}
}

func TestToastSemanticColorReadsTheCurrentPalette(t *testing.T) {
	restoreToastTestTheme(t)
	palette := paint.CurrentIconPalette()
	palette.Accent = paint.Color{R: 12, G: 34, B: 56, A: 255}
	palette.Success = paint.Color{R: 23, G: 45, B: 67, A: 255}
	paint.SetIconPalette(palette)
	if toastSemanticColor(ToastInfo) != palette.Accent || toastSemanticColor(ToastSuccess) != palette.Success {
		t.Fatal("toast cached or hardcoded a semantic color instead of following the current palette")
	}
}

func TestToastPaletteRendersExactOpaquePixels(t *testing.T) {
	restoreToastTestTheme(t)
	for _, mode := range []ThemeMode{ThemeLight, ThemeDark} {
		SetThemeMode(mode)
		for _, level := range []ToastLevel{ToastInfo, ToastSuccess, ToastWarning, ToastError} {
			bg, text := toastSurfaceColors(level)
			accent := toastSemanticColor(level)
			pix := paint.NewPixmap(32, 24)
			g := pix.NewPainter()
			g.SetBrush1(bg)
			g.Rectangle(0, 0, 32, 24)
			g.Fill()
			g.SetBrush1(accent)
			g.Rectangle(0, 0, 4, 24)
			g.Fill()
			g.SetBrush1(text)
			g.Rectangle(10, 8, 4, 8)
			g.Fill()
			img, err := pix.Image()
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				x, y int
				want paint.Color
			}{{20, 12, bg}, {2, 12, accent}, {12, 12, text}} {
				if got := paint.ColorModel.Convert(img.At(tc.x, tc.y)).(paint.Color); got != tc.want {
					t.Fatalf("theme %v level %v: pixel (%d,%d)=%v, want exact opaque %v", mode, level, tc.x, tc.y, got, tc.want)
				}
			}
		}
	}
}
