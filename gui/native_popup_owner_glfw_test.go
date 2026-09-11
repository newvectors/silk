//go:build !windows

package gui

import (
	"testing"

	"github.com/go-gl/glfw/v3.3/glfw"
)

func TestNativePopupOwnerNilAndSelfAreNoOps(t *testing.T) {
	// Do not initialize GLFW or create desktop windows. Invalid/nil ownership
	// requests must be rejected before any GetCocoaWindow/native API call.
	uninitialized := new(glfw.Window)
	for _, pair := range [][2]*glfw.Window{{nil, nil}, {nil, uninitialized}, {uninitialized, nil}, {uninitialized, uninitialized}} {
		if bindNativePopupOwner(pair[0], pair[1]) {
			t.Fatal("invalid popup ownership request was accepted")
		}
	}
	unbindNativePopupOwner(nil)
}
