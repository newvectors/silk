//go:build !windows

package gui

import (
	"image"
	"testing"

	"github.com/uk0/silk/paint"
)

type nativeIconFixture struct{ calls int }

func (*nativeIconFixture) AvailableSize() []int      { return []int{32} }
func (*nativeIconFixture) IsAir() bool               { return false }
func (i *nativeIconFixture) Pixmap(int) paint.Pixmap { i.calls++; return paint.NewPixmap(32, 32) }

func TestWindowIconLinuxDeliveryAndMacOSBundleBoundary(t *testing.T) {
	ico := new(nativeIconFixture)
	deliveries := 0
	set := func(images []image.Image) {
		deliveries++
		if len(images) != 1 || images[0].Bounds().Dx() != 32 {
			t.Fatal("duplicate or invalid native image candidates")
		}
	}
	applyGLFWWindowIcon("darwin", ico, set)
	if deliveries != 0 || ico.calls != 0 {
		t.Fatal("macOS called unsupported GLFW icon API")
	}
	applyGLFWWindowIcon("linux", ico, set)
	if deliveries != 1 || ico.calls != 4 {
		t.Fatal("Linux did not deliver its decoded window icon")
	}
	applyGLFWWindowIcon("linux", nil, set)
	applyGLFWWindowIcon("linux", paint.AirIcon(), set)
	if deliveries != 1 {
		t.Fatal("empty icon replaced native identity")
	}
}
