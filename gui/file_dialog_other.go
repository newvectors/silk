//go:build !windows && (!darwin || !cgo)

package gui

func platformOpenFileDialog() string { return linuxOpenFileDialog() }
func platformSaveFileDialog() string { return linuxSaveFileDialog() }
