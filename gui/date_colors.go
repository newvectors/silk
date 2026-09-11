package gui

import "github.com/uk0/silk/paint"

// dateSelectionColors keeps the popup and inline calendar selection identical.
// The semantic accent is blue-600 in light mode and blue-400 in dark mode:
// white ink on the dark theme's lighter blue is not legible. Pair each exact
// 8-bit accent token with its contrasting surface ink instead.
func dateSelectionColors(t *defaultTheme) (background, text paint.Color) {
	palette := paint.CurrentIconPalette()
	background, text = palette.Accent, palette.Paper
	if CurrentThemeMode() == ThemeDark {
		text = t.ViewBGColor
	}
	return
}
