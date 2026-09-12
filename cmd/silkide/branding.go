package main

import (
	"bytes"
	"image"
	"image/png"

	"github.com/uk0/silk/assets/branding"
	"github.com/uk0/silk/core"
	"github.com/uk0/silk/ged"
	"github.com/uk0/silk/paint"
	"golang.org/x/image/draw"
)

var ideLogo paint.Icon

// Like paint's icon cache, this lazy raster cache is owned by the UI thread.
// It uses the existing Pixmap/Icon rendering path, not a second renderer or
// temporary extracted files. The seven cache sizes bound memory growth.
type ideBrandIcon struct {
	source image.Image
	cache  map[int]paint.Pixmap
}

func (i *ideBrandIcon) AvailableSize() []int { return []int{16, 24, 32, 48, 64, 128, 256} }
func (i *ideBrandIcon) IsAir() bool          { return false }
func (i *ideBrandIcon) Pixmap(requested int) paint.Pixmap {
	size := 256
	for _, candidate := range i.AvailableSize() {
		if candidate >= requested {
			size = candidate
			break
		}
	}
	if p := i.cache[size]; p != nil {
		return p
	}
	// Cairo ARGB32 consumes premultiplied pixels; use its supported RGBA
	// fast path rather than passing PNG's non-premultiplied NRGBA through.
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(img, img.Bounds(), i.source, i.source.Bounds(), draw.Src, nil)
	p := paint.NewPixmap(size, size)
	if err := p.SetImage(img); err != nil {
		core.Warn("Silk IDE logo: ", err)
		return nil
	}
	// SetImage writes the backing bytes directly. Inform Cairo that a newly
	// allocated (formerly clear) surface now has pixels before using it as a
	// pattern; reading the bytes with Image() alone cannot verify this path.
	p.MarkDirty()
	i.cache[size] = p
	return p
}

func silkIDEIcon() paint.Icon {
	if ideLogo == nil {
		img, err := png.Decode(bytes.NewReader(branding.PNG()))
		if err != nil {
			core.Warn("Silk IDE logo: ", err)
			return paint.AirIcon()
		}
		ideLogo = &ideBrandIcon{source: img, cache: make(map[int]paint.Pixmap)}
	}
	return ideLogo
}

func silkIDEAboutInfo() ged.AboutInfo {
	return ged.AboutInfo{Name: "Silk IDE", Version: ideVersion, Logo: silkIDEIcon()}
}
