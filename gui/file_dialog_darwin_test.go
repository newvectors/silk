//go:build darwin && cgo

package gui

import "testing"

func TestCocoaFileDialogRejectsNonUIThreadBeforeNativeCalls(t *testing.T) {
	previous := uiThreadOwner
	uiThreadOwner = func() bool { return false }
	defer func() { uiThreadOwner = previous }()
	for _, kind := range []nativeFileDialogKind{nativeFileDialogOpen, nativeFileDialogFolder, nativeFileDialogSave} {
		if path := runCocoaFileDialog(kind); path != "" {
			t.Fatalf("off-thread native chooser returned a path: %q", path)
		}
	}
}
