package main

import (
	"github.com/uk0/silk/gui"
	"testing"
)

func TestPreviewWindowPreservesDesignedContentAfterDockAttachment(t *testing.T) {
	for _, size := range [][2]float64{{756, 756}, {600, 400}, {120, 80}} {
		frame := gui.NewFrame()
		form := gui.NewForm()
		form.SetSize(size[0], size[1])
		desiredW, desiredH := form.Size()
		dock := frame.SuggestDocDock().(*gui.Dock)
		dock.AddView(form)
		sizePreviewFrame(frame, form, desiredW, desiredH)
		if form.Width() < desiredW || form.Height() < desiredH {
			t.Fatalf("preview clipped designed %vx%v to %vx%v", desiredW, desiredH, form.Width(), form.Height())
		}
		if frame.Width() < 320 || frame.Height() < 240 {
			t.Fatal("tiny preview lost minimum window size")
		}
		child := gui.NewButton1("edge", nil)
		child.SetBounds(desiredW-40, desiredH-25, 40, 25)
		child.SetParent(form)
		if child.X()+child.Width() > form.Width() || child.Y()+child.Height() > form.Height() {
			t.Fatal("edge child clipped by window chrome")
		}
	}
}
