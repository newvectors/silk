//go:build !windows

package gui

import (
	"github.com/go-gl/glfw/v3.3/glfw"
	"time"
)

var glfwAnimationPacer framePacer

func (win *Window) canRenderFrame() bool {
	return win != nil && win.widget != nil && validRenderSize(win.widget.Width(), win.widget.Height()) && win.glfwWin != nil && !win.iconified && win.IsVisible()
}
func hasRenderableGLFWWindow() bool {
	for _, win := range winMap {
		if win.canRenderFrame() {
			return true
		}
	}
	return false
}
func waitAdaptiveGLFWFrame() {
	now := time.Now()
	// Timer/hover heartbeat still wakes at most 21Hz when static; it does not
	// paint. Input and PostEmptyEvent interrupt this wait immediately.
	delay := 47 * time.Millisecond
	drawable := false
	for _, win := range winMap {
		if !win.canRenderFrame() {
			continue
		}
		drawable = true
		if win.dirty {
			delay = min(delay, win.framePacer.delay(now))
		}
	}
	if drawable && continuousFrameWanted() {
		delay = min(delay, glfwAnimationPacer.delay(now))
	}
	glfw.WaitEventsTimeout(max(time.Millisecond, delay).Seconds())
}
