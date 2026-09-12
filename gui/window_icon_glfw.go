//go:build !windows

package gui

import (
	"image"

	"github.com/uk0/silk/paint"
)

// macOS application icons are owned by the .app bundle (CFBundleIconFile),
// not GLFW windows. X11 accepts image candidates and selects the best size.
func applyGLFWWindowIcon(goos string, icon paint.Icon, set func([]image.Image)) {
	if goos == "darwin" || icon == nil || icon.IsAir() {
		return
	}
	var images []image.Image
	seen := make(map[image.Point]bool)
	for _, size := range []int{16, 32, 48, 256} {
		pm := icon.Pixmap(size)
		if pm == nil {
			continue
		}
		img, err := pm.Image()
		if err != nil || img == nil {
			continue
		}
		sz := img.Bounds().Size()
		if sz.X <= 0 || sz.Y <= 0 || seen[sz] {
			continue
		}
		seen[sz] = true
		images = append(images, img)
	}
	if len(images) > 0 {
		set(images)
	}
}
