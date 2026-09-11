package core

import (
	"fmt"
	"os"
	"path/filepath"
)

// Directory permissions are permission bits, not os.ModeDir (a file-type bit).
// Do not chmod existing paths: they may be shared or explicitly configured.
func preparePrivateDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, err
	}
	if err := os.MkdirAll(abs, 0700); err != nil {
		return abs, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return abs, err
	}
	if !info.IsDir() {
		return abs, fmt.Errorf("%s is not a directory", abs)
	}
	return abs, nil
}

func resolveResourceDir(install, cwd, gopath, override string) string {
	if override != "" {
		if abs, err := filepath.Abs(override); err == nil {
			return abs
		}
		return override
	}
	// Standard macOS bundles keep immutable assets outside Contents/MacOS.
	if filepath.Base(install) == "MacOS" && filepath.Base(filepath.Dir(install)) == "Contents" {
		bundle := filepath.Join(filepath.Dir(install), "Resources")
		if info, err := os.Stat(bundle); err == nil && info.IsDir() {
			return bundle
		}
	}
	candidates := []string{install}
	if cwd != "" {
		candidates = append(candidates, cwd)
	}
	for _, entry := range filepath.SplitList(gopath) {
		if entry != "" {
			candidates = append(candidates, filepath.Join(entry, "bin"))
		}
	}
	for _, path := range candidates {
		if info, err := os.Stat(filepath.Join(path, "icon")); err == nil && info.IsDir() {
			return path
		}
	}
	return install
}
