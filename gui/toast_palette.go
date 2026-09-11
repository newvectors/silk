package gui

import "github.com/uk0/silk/paint"

// Toasts are opaque neutral cards, not saturated severity-colored banners.
// Severity belongs to the accent bar/icon; the body uses the active theme so
// small notification text remains readable regardless of what is underneath.
func toastSurfaceColors(_ ToastLevel) (bg, text paint.Color) {
	bg, text = Theme().ViewBGColor, Theme().TextColor
	bg.A, text.A = 255, 255
	return
}

// Reuse the exact 8-bit sRGB semantic tokens used by the workbench icons.
// Read them at draw time so existing notifications follow a theme change.
func toastSemanticColor(level ToastLevel) paint.Color {
	palette := paint.CurrentIconPalette()
	switch level {
	case ToastInfo:
		return palette.Accent
	case ToastSuccess:
		return palette.Success
	case ToastWarning:
		return palette.Warning
	case ToastError:
		return palette.Danger
	default:
		return palette.Foreground
	}
}
