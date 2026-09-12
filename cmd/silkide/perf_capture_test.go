package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPerformanceCaptureDisabledDoesNoWork(t *testing.T) {
	// Every dependency is nil: disabled mode must not access any of them.
	stop := startBoundedCPUProfile("", "30", perfCaptureHooks{})
	stop()
	stop()
	t.Setenv("SILKIDE_CPU_PROFILE", "")
	t.Setenv("SILKIDE_PROFILE_SECONDS", "invalid")
	stop = startPerformanceCapture()
	stop()
}

func TestPerformanceCaptureDurationBounds(t *testing.T) {
	for _, tc := range []struct {
		value   string
		seconds int
		warning bool
	}{
		{"", 30, false}, {"5", 5, false}, {"30", 30, false}, {"120", 120, false}, {"0", 5, false}, {"-99", 5, false}, {"121", 120, false}, {"9999", 120, false}, {" 10 ", 10, false}, {"not-a-number", 30, true}, {"999999999999999999999999999999", 30, true},
	} {
		d, err := profileCaptureDuration(tc.value)
		if d != time.Duration(tc.seconds)*time.Second || (err != nil) != tc.warning {
			t.Errorf("%q=%v,%v", tc.value, d, err)
		}
	}
}

func TestPerformanceCaptureBoundedStopAndMetadata(t *testing.T) {
	for _, finish := range []string{"deadline", "shutdown", "concurrent"} {
		t.Run(finish, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "capture.pprof")
			var callback func()
			var duration time.Duration
			var starts, stops, cancels, memReads atomic.Int32
			var logsMu sync.Mutex
			var logs []string
			h := perfCaptureHooks{
				start: func(got string) error {
					starts.Add(1)
					if got != path {
						t.Errorf("path=%q want=%q", got, path)
					}
					return nil
				},
				stop: func() { stops.Add(1) },
				after: func(d time.Duration, fn func()) func() bool {
					duration = d
					callback = fn
					return func() bool { cancels.Add(1); return true }
				},
				memory:   func() string { memReads.Add(1); return "Alloc=12KB NumGC=0" },
				metadata: func() string { return "GOOS=windows GOARCH=amd64 IDE=test" },
				logf: func(format string, args ...any) {
					logsMu.Lock()
					defer logsMu.Unlock()
					logs = append(logs, fmt.Sprintf(format, args...))
				},
			}
			stop := startBoundedCPUProfile(path, "", h)
			if starts.Load() != 1 || stops.Load() != 0 || duration != 30*time.Second || callback == nil || memReads.Load() != 1 {
				t.Fatal("capture did not arm exactly once")
			}
			switch finish {
			case "deadline":
				callback()
			case "shutdown":
				stop()
			case "concurrent":
				var wg sync.WaitGroup
				wg.Add(2)
				go func() { defer wg.Done(); callback() }()
				go func() { defer wg.Done(); stop() }()
				wg.Wait()
			}
			callback()
			stop()
			stop()
			if stops.Load() != 1 || cancels.Load() != 1 || memReads.Load() != 2 {
				t.Fatalf("flush ownership stops=%d cancel=%d mem=%d", stops.Load(), cancels.Load(), memReads.Load())
			}
			joined := strings.Join(logs, "\n")
			for _, want := range []string{"GOOS=windows GOARCH=amd64 IDE=test", "duration=30s", "memory start", "memory end", fmt.Sprintf("%q", path), "finished reason="} {
				if !strings.Contains(joined, want) {
					t.Errorf("missing diagnostic %q in %s", want, joined)
				}
			}
		})
	}
}

func TestPerformanceCaptureStartFailureNeverArmsOrStops(t *testing.T) {
	logs := ""
	h := perfCaptureHooks{
		start: func(string) error { return errors.New("output denied") },
		logf:  func(f string, args ...any) { logs += fmt.Sprintf(f, args...) },
		// Other hooks are nil: failure must not stop another owner, read
		// memory, inspect build data or schedule any timer.
	}
	stop := startBoundedCPUProfile(filepath.Join(t.TempDir(), "capture.pprof"), "30", h)
	stop()
	if !strings.Contains(logs, "cannot start") || !strings.Contains(logs, "output denied") {
		t.Fatal("failure silently swallowed")
	}
}

func TestPerformanceBuildMetadataIsSmallAndPlatformAccurate(t *testing.T) {
	got := performanceBuildMetadata()
	for _, want := range []string{"GOOS=" + runtime.GOOS, "GOARCH=" + runtime.GOARCH, "IDE=" + ideVersion, "Go=" + runtime.Version(), "build="} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if len(got) > 512 || strings.Contains(got, "\n") {
		t.Fatal("unexpected unbounded build/environment dump")
	}
}
