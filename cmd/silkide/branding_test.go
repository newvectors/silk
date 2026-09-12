package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/uk0/silk/ged"
	"github.com/uk0/silk/gui"
	"github.com/uk0/silk/paint"
)

func TestIDEBrandingIsEmbeddedAndUsesBoundedNativeSizes(t *testing.T) {
	oldIcon, oldApp := ideLogo, gui.AppIcon()
	t.Cleanup(func() { ideLogo = oldIcon; gui.SetAppIcon(oldApp) })
	t.Chdir(t.TempDir()) // no install resources/icon directory or working tree
	ideLogo = nil
	ico := silkIDEIcon()
	if ico == nil || ico.IsAir() {
		t.Fatal("standalone executable lost its embedded identity")
	}
	info := silkIDEAboutInfo()
	if info.Name != "Silk IDE" || info.Version != ideVersion || info.Logo != ico {
		t.Fatal("welcome/about identity drifted from IDE")
	}
	gui.SetAppIcon(ico)
	if gui.AppIcon() != ico {
		t.Fatal("native frame app-icon override not applied")
	}
	for _, size := range []int{16, 24, 32, 48, 64, 128, 256} {
		p := ico.Pixmap(size)
		if p == nil || p.Width() != size || p.Height() != size {
			t.Fatalf("native icon size %d incorrect", size)
		}
		img, err := p.Image()
		if err != nil {
			t.Fatal(err)
		}
		_, _, _, a := img.At(0, 0).RGBA()
		if a != 0 {
			t.Fatalf("icon %d lost transparent corner", size)
		}
		if ico.Pixmap(size) != p {
			t.Fatal("repeated icon request was not cached")
		}
		if size == 256 {
			// The embedded asset is already 256px: opaque sRGB bytes must
			// survive PNG -> RGBA -> Cairo -> image without a channel swap.
			source := ico.(*ideBrandIcon).source
			checked := 0
			for y := 0; y < 256; y += 8 {
				for x := 0; x < 256; x += 8 {
					r, g, b, a := source.At(x, y).RGBA()
					if a != 65535 {
						continue
					}
					pr, pg, pb, pa := img.At(x, y).RGBA()
					if r != pr || g != pg || b != pb || a != pa {
						t.Fatalf("sRGB logo pixel changed at %d,%d", x, y)
					}
					checked++
				}
			}
			if checked == 0 {
				t.Fatal("logo color verification did not inspect opaque pixels")
			}
		}
	}
	for size := 1; size <= 1024; size++ {
		ico.Pixmap(size)
	}
	if len(ico.(*ideBrandIcon).cache) != 7 {
		t.Fatal("raster cache grew with arbitrary caller sizes")
	}
	files, err := filepath.Glob("*")
	if err != nil || len(files) != 0 {
		t.Fatalf("branding extracted writable files: %v %v", files, err)
	}
	if _, err := os.Stat("icon"); !os.IsNotExist(err) {
		t.Fatal("fixture unexpectedly depends on icon directory")
	}
}

func TestIDEBrandingActualWelcomeCairoPaintUsesWrittenPixels(t *testing.T) {
	old := ideLogo
	t.Cleanup(func() { ideLogo = old })
	ideLogo = nil
	w := ged.NewWelcomeScreen()
	w.SetBranding(silkIDEAboutInfo())
	w.SetSize(900, 600)
	target := paint.NewPixmap(900, 600)
	w.Draw(target.NewPainter())
	img, err := target.Image()
	if err != nil {
		t.Fatal(err)
	}
	// Welcome's real logo rectangle at this size is (90,68)-(146,124).
	// Inspect its interior, never the shifted title or a fake fixture icon.
	colored := 0
	for y := 70; y < 122; y++ {
		for x := 92; x < 144; x++ {
			if paint.ColorModel.Convert(img.At(x, y)).(paint.Color) != gui.Theme().FormColor {
				colored++
			}
		}
	}
	if colored < 100 {
		t.Fatalf("embedded logo pixels never reached Welcome's Cairo target: %d colored pixels", colored)
	}
}
