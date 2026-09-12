package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/uk0/silk/gui"
)

const defaultCPUProfileDuration = 30 * time.Second

type perfCaptureHooks struct {
	start    func(string) error
	stop     func()
	after    func(time.Duration, func()) func() bool
	memory   func() string
	metadata func() string
	logf     func(string, ...any)
}

// No environment opt-in means no timer, goroutine, profile file, memory scan,
// or diagnostic log. Collection never opens a network/pprof HTTP listener.
func startPerformanceCapture() func() {
	path := os.Getenv("SILKIDE_CPU_PROFILE")
	if strings.TrimSpace(path) == "" {
		return func() {}
	}
	return startBoundedCPUProfile(path, os.Getenv("SILKIDE_PROFILE_SECONDS"), perfCaptureHooks{
		start: gui.StartCPUProfile, stop: gui.StopCPUProfile,
		after:  func(d time.Duration, fn func()) func() bool { return time.AfterFunc(d, fn).Stop },
		memory: func() string { return gui.MemStats() + " " + gui.RenderStats() }, metadata: performanceBuildMetadata, logf: log.Printf,
	})
}

func profileCaptureDuration(value string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return defaultCPUProfileDuration, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return defaultCPUProfileDuration, fmt.Errorf("invalid SILKIDE_PROFILE_SECONDS; using 30 seconds")
	}
	n = max(5, min(120, n))
	return time.Duration(n) * time.Second, nil
}

// The only asynchronous work is stopping/flushing the Go profiler and logging
// process statistics. No widget, native window, or GUI-thread API is touched.
func startBoundedCPUProfile(path, seconds string, h perfCaptureHooks) func() {
	if strings.TrimSpace(path) == "" {
		return func() {}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		h.logf("silkide CPU profile: invalid output path: %v", err)
		return func() {}
	}
	duration, warning := profileCaptureDuration(seconds)
	if warning != nil {
		h.logf("silkide CPU profile: %v", warning)
	}
	if err := h.start(abs); err != nil {
		h.logf("silkide CPU profile: cannot start path=%q: %v", abs, err)
		return func() {}
	}
	h.logf("silkide CPU profile: started duration=%s path=%q %s", duration, abs, h.metadata())
	h.logf("silkide CPU profile: memory start %s", h.memory())
	var once sync.Once
	var timerMu sync.Mutex
	var cancel func() bool
	finish := func(reason string) {
		once.Do(func() {
			timerMu.Lock()
			stopTimer := cancel
			cancel = nil
			timerMu.Unlock()
			if stopTimer != nil {
				stopTimer()
			}
			h.stop() // pprof flushes its writer before gui closes the file
			h.logf("silkide CPU profile: finished reason=%s path=%q", reason, abs)
			h.logf("silkide CPU profile: memory end %s", h.memory())
		})
	}
	// AfterFunc callbacks are asynchronous. Hold the small registration lock
	// until cancel is installed, including a concurrent early app shutdown.
	timerMu.Lock()
	cancel = h.after(duration, func() { finish("deadline") })
	timerMu.Unlock()
	return func() { finish("shutdown") }
}

func performanceBuildMetadata() string {
	build := "development"
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			build = info.Main.Version
		}
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && setting.Value != "" {
				revision := setting.Value
				if len(revision) > 12 {
					revision = revision[:12]
				}
				build += " revision=" + revision
			}
			if setting.Key == "vcs.modified" && setting.Value == "true" {
				build += " modified=true"
			}
		}
	}
	return fmt.Sprintf("GOOS=%s GOARCH=%s IDE=%s Go=%s build=%s", runtime.GOOS, runtime.GOARCH, ideVersion, runtime.Version(), build)
}
