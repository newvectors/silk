package branding

import (
	"bytes"
	"image/png"
	"testing"
)

func TestRuntimeLogoIsSquareRGBAWithRealTransparency(t *testing.T) {
	data := PNG()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
		t.Fatal("runtime logo must be 256x256")
	}
	clear, opaque, edge := false, false, false
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			clear = clear || a == 0
			opaque = opaque || a == 65535
			edge = edge || (a > 0 && a < 65535)
		}
	}
	if !clear || !opaque || !edge {
		t.Fatal("logo must preserve transparent background, opaque color and antialiased alpha edges")
	}
	data[0] = 0
	if PNG()[0] != 137 {
		t.Fatal("caller modified shared embedded branding")
	}
}
