package paint

import "github.com/uk0/silk/cairo"

// IconPalette holds exact, unpremultiplied 8-bit sRGB colors. Hosts switch it
// on the UI thread with their theme. Resource PNGs keep their original colors;
// procedural icons (including already-loaded instances) use the current palette.
type IconPalette struct {
	Foreground, Accent, Success, Warning, Danger, Violet, Paper, Disabled Color
}

func LightIconPalette() IconPalette {
	return IconPalette{
		Foreground: Color{R: 71, G: 85, B: 105, A: 255},   // #475569
		Accent:     Color{R: 37, G: 99, B: 235, A: 255},   // #2563EB
		Success:    Color{R: 21, G: 128, B: 61, A: 255},   // #15803D
		Warning:    Color{R: 161, G: 98, B: 7, A: 255},    // #A16207
		Danger:     Color{R: 185, G: 28, B: 28, A: 255},   // #B91C1C
		Violet:     Color{R: 124, G: 58, B: 237, A: 255},  // #7C3AED
		Paper:      Color{R: 248, G: 250, B: 252, A: 255}, // #F8FAFC
		Disabled:   Color{R: 148, G: 163, B: 184, A: 255}, // #94A3B8
	}
}

func DarkIconPalette() IconPalette {
	return IconPalette{
		Foreground: Color{R: 203, G: 213, B: 225, A: 255}, // #CBD5E1
		Accent:     Color{R: 96, G: 165, B: 250, A: 255},  // #60A5FA
		Success:    Color{R: 74, G: 222, B: 128, A: 255},  // #4ADE80
		Warning:    Color{R: 251, G: 191, B: 36, A: 255},  // #FBBF24
		Danger:     Color{R: 248, G: 113, B: 113, A: 255}, // #F87171
		Violet:     Color{R: 196, G: 181, B: 253, A: 255}, // #C4B5FD
		Paper:      Color{R: 248, G: 250, B: 252, A: 255}, // #F8FAFC
		Disabled:   Color{R: 100, G: 116, B: 139, A: 255}, // #64748B
	}
}

var currentIconPalette = LightIconPalette()

func SetIconPalette(palette IconPalette) { currentIconPalette = palette }
func CurrentIconPalette() IconPalette    { return currentIconPalette }

func setIconColor(cc *cairo.Context, color Color) {
	cc.SetSourceRGBA(color.NRGBAf())
}

// Draw at logical size through Cairo's actual device transform. Retina and
// fractional scaling therefore rasterize vector paths, never upscale 16px PNGs.
func drawProceduralIcon(cc *cairo.Context, draw proceduralIconDrawer, size float64, disabled bool) {
	path := cc.CopyPath() // Cairo Save/Restore does not save the current path.
	defer path.Destroy()
	cc.Save()
	cc.NewPath()
	cc.Scale(size/16, size/16)
	if disabled {
		cc.PushGroup()
		draw(16, cc)
		mask := cc.PopGroup()
		setIconColor(cc, currentIconPalette.Disabled)
		cc.Mask(mask)
		mask.Destroy()
	} else {
		draw(16, cc)
	}
	cc.Restore()
	cc.NewPath()
	cc.AppendPath(path)
}

var preferVectorIcons bool

// SetPreferVectorIcons lets an IDE opt into crisp theme-aware built-ins even
// when an older resource pack supplies low-resolution PNGs for the same names.
// Custom/unknown resource icons are untouched. Existing Icon references are
// upgraded in place, including handles loaded during package initialization.
func SetPreferVectorIcons(prefer bool) {
	preferVectorIcons = prefer
	for name, cached := range iconCache {
		if cached.icon.resource {
			cached.icon.draw = nil
			if prefer {
				cached.icon.draw = proceduralFallbacks[name]
			}
		}
	}
}
