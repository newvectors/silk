package paint

import (
	"image"
	"testing"
)

func iconPixel(t *testing.T, pix Pixmap, x, y int) Color {
	t.Helper()
	img, err := pix.Image()
	if err != nil {
		t.Fatal(err)
	}
	return ColorModel.Convert(img.At(x, y)).(Color)
}

func TestProceduralIconPaletteUsesExactSRGBAndUpdatesLoadedIcons(t *testing.T) {
	previous := CurrentIconPalette()
	defer SetIconPalette(previous)
	icon := genProceduralIcon("run", drawPlayTriangle)
	for _, palette := range []IconPalette{LightIconPalette(), DarkIconPalette()} {
		SetIconPalette(palette)
		pix := icon.Pixmap(64)
		if got := iconPixel(t, pix, 32, 32); got != palette.Success {
			t.Fatalf("run RGB = %v, want exact %v", got, palette.Success)
		}
		if got := iconPixel(t, pix, 0, 0); got.A != 0 {
			t.Fatal("icon corners are not transparent")
		}
		for _, active := range []bool{false, true} {
			handle := genProceduralIcon("handle", drawResizeHandle(0, active))
			want := palette.Paper
			if active {
				want = palette.Accent
			}
			if got := iconPixel(t, handle.Pixmap(64), 32, 32); got != want {
				t.Fatalf("handle state RGB: %v, want %v", got, want)
			}
		}
	}
	if icon.Pixmap(0) != nil || icon.Pixmap(-1) != nil {
		t.Fatal("invalid dimensions accepted")
	}
}

func TestProceduralIconsRasterizeAtActualDeviceScale(t *testing.T) {
	for _, size := range []int{16, 20, 24, 32, 48, 64, 96, 128} {
		ico := genProceduralIcon("save", drawSaveDisk)
		device := NewPixmap(size, size)
		p := device.NewPainter().(*cairoPainter)
		p.Scale(float64(size)/16, float64(size)/16)
		p.DrawIcon(ico, 16, false)
		got, _ := device.Image()
		want, _ := ico.Pixmap(size).Image()
		assertIconImagesEqual(t, size, got, want)
	}
}

func assertIconImagesEqual(t *testing.T, size int, got, want image.Image) {
	t.Helper()
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if ColorModel.Convert(got.At(x, y)) != ColorModel.Convert(want.At(x, y)) {
				t.Fatalf("%dpx device icon was resampled rather than vector-rendered at (%d,%d)", size, x, y)
			}
		}
	}
}

func TestDisabledIconUsesExplicitPaletteAndPreservesPainterPath(t *testing.T) {
	previous := CurrentIconPalette()
	defer SetIconPalette(previous)
	for _, palette := range []IconPalette{LightIconPalette(), DarkIconPalette()} {
		SetIconPalette(palette)
		pix := NewPixmap(64, 64)
		p := pix.NewPainter().(*cairoPainter)
		p.MoveTo(2, 3)
		p.LineTo(7, 9)
		p.DrawIcon(genProceduralIcon("run", drawPlayTriangle), 64, true)
		if x, y := p.CurrentPoint(); x != 7 || y != 9 {
			t.Fatal("icon draw corrupted the caller path")
		}
		if got := iconPixel(t, pix, 32, 32); got != palette.Disabled {
			t.Fatalf("disabled RGB = %v, want %v", got, palette.Disabled)
		}
	}
}

func TestVectorPreferenceUpgradesExistingResourceIconWithoutLosingPNG(t *testing.T) {
	cache, sources, preference := iconCache, iconSrcCache, preferVectorIcons
	defer func() { iconCache, iconSrcCache, preferVectorIcons = cache, sources, preference }()
	legacy := NewPixmap(16, 16)
	p := legacy.NewPainter()
	red := Color{R: 255, A: 255}
	p.SetBrush1(red)
	p.Rectangle(0, 0, 16, 16)
	p.Fill()
	loaded := &icon{name: "run", resource: true, subs: []*subIcon{{size: 16, img: legacy}}}
	iconSrcCache = map[string]*iconSrc{}
	iconCache = map[string]*cachedIcon{"run": {icon: loaded}}
	SetPreferVectorIcons(true)
	if LoadIcon("run") != loaded {
		t.Fatal("existing references were replaced")
	}
	if got := iconPixel(t, loaded.Pixmap(64), 32, 32); got != CurrentIconPalette().Success {
		t.Fatalf("legacy PNG overrode vector palette: %v", got)
	}
	SetPreferVectorIcons(false)
	if got := iconPixel(t, loaded.Pixmap(16), 8, 8); got != red {
		t.Fatal("opting out did not preserve resource PNG")
	}
}
