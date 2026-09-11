//go:build !windows && (!darwin || !cgo)

package gui

import "github.com/go-gl/glfw/v3.3/glfw"

// Other GLFW platforms keep their existing native policy. Go-side popup
// ownership/dismissal remains active; this does not claim X11/Wayland owners.
func bindNativePopupOwner(popup, owner *glfw.Window) bool { return false }

func unbindNativePopupOwner(popup *glfw.Window) {}
