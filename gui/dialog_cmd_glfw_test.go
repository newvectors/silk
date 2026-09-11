//go:build !windows

package gui

import (
	"testing"
	"time"
)

func TestNativeDialogCommandServicesWorkAndHandlesFailure(t *testing.T) {
	previous, previousQuit := nativeDialogFrame, shouldQuit
	shouldQuit = false
	defer func() { nativeDialogFrame, shouldQuit = previous, previousQuit }()
	frames := 0
	nativeDialogFrame = func() { frames++; serviceUIFrame(); time.Sleep(time.Millisecond) }
	posted := false
	Post(func() { posted = true })
	got := runDialogCmd("/bin/sh", "-c", "sleep 0.04; printf '/tmp/chosen.go\\n'")
	if got != "/tmp/chosen.go" || frames == 0 || !posted {
		t.Fatalf("got %q frames %d posted %v", got, frames, posted)
	}
	if got := runDialogCmd("/bin/sh", "-c", "printf invalid; exit 1"); got != "" {
		t.Fatalf("failed chooser accepted stdout: %q", got)
	}
	if got := runDialogCmd("/nonexistent/silkide-chooser"); got != "" {
		t.Fatal(got)
	}
}
