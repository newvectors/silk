package gui

import (
	"github.com/uk0/silk/paint"
)

// Row-painting colours shared by Table, TreeView and ListWidget. They live
// beside the row draw code rather than in theme.go because they are derived
// from the palette, not part of it — theme.go stays the list of raw colours.

// altRowNudge is how far the zebra stripe steps off the view surface, in 8-bit
// channel points. It matches the separation the old hardcoded {245,245,250}
// had against a white view, so light mode keeps the stripe it always had.
const altRowNudge = 10

// tableAltRowColor is the fill for odd (alternating) table rows. It used to be
// a hardcoded near-white, which silkide — dark by default — turned into a band
// of near-white rows carrying near-white TextColor, i.e. invisible text.
//
// The stripe is one small step off ViewBGColor, in the direction of the theme's
// own "one step lighter than the view" chrome: FormLightColor is zinc-800 over
// a zinc-900 view in dark mode. The light theme paints both of those pure
// white, so there the step is taken toward FormColor (the slightly grey chrome)
// instead — otherwise the stripe would vanish on white.
func tableAltRowColor(t *defaultTheme) paint.Color {
	ref := t.FormLightColor
	if ref == t.ViewBGColor {
		ref = t.FormColor
	}
	c := t.ViewBGColor
	c.R = stepToward(c.R, ref.R, altRowNudge)
	c.G = stepToward(c.G, ref.G, altRowNudge)
	c.B = stepToward(c.B, ref.B, altRowNudge)
	return c
}

// stepToward moves v at most n points toward target, stopping on target rather
// than overshooting it.
func stepToward(v, target, n uint8) uint8 {
	if target > v {
		if target-v < n {
			return target
		}
		return v + n
	}
	if v-target < n {
		return target
	}
	return v - n
}

// selectionTextColor is the ink for a row that may be selected. A selected row
// is filled with HighLightColor — blue in both modes — and TextColor is tuned
// for the view surface, not for that fill: in dark mode it is zinc-200, i.e.
// near-white on blue-400. MenuActiveTextColor is the theme's "ink on an accent
// fill" colour, which the menus and the calc / recipe / trend panels already
// switch to after the same HighLightColor fill.
func selectionTextColor(t *defaultTheme, selected bool) paint.Color {
	if selected {
		return t.MenuActiveTextColor
	}
	return t.TextColor
}
