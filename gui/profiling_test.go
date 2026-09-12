package gui

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestCPUProfileRepeatedStartDoesNotTruncateAndStopCloses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capture.pprof")
	other := filepath.Join(dir, "existing.pprof")
	if err := os.WriteFile(other, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	var file *os.File
	stops := 0
	s := cpuProfileSession{
		start: func(w io.Writer) error { file = w.(*os.File); _, err := io.WriteString(w, "header"); return err },
		stop: func() {
			stops++
			if _, err := file.WriteString("-flushed"); err != nil {
				t.Errorf("file closed before flush: %v", err)
			}
		},
	}
	if err := s.begin(path); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{path, other} {
		if err := s.begin(target); err == nil {
			t.Fatal("duplicate profile accepted")
		}
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "header" {
		t.Fatalf("active output truncated: %q %v", got, err)
	}
	if got, err := os.ReadFile(other); err != nil || string(got) != "keep" {
		t.Fatalf("rejected output touched: %q %v", got, err)
	}
	if err := s.finish(); err != nil {
		t.Fatal(err)
	}
	if stops != 1 || s.file != nil {
		t.Fatal("stop did not release session")
	}
	if _, err := file.WriteString("late"); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("file remains open after stop: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "header-flushed" {
		t.Fatalf("flush lost: %q %v", got, err)
	}
	if err := s.finish(); err != nil || stops != 1 {
		t.Fatal("empty Stop affected another owner")
	}
	if err := s.begin(path); err != nil {
		t.Fatal("valid restart failed", err)
	}
	if err := s.finish(); err != nil {
		t.Fatal(err)
	}
}

func TestCPUProfileFailedStartClosesFileAndCanRetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.pprof")
	var file *os.File
	stopped := false
	want := errors.New("profiler busy outside Silk")
	s := cpuProfileSession{start: func(w io.Writer) error { file = w.(*os.File); return want }, stop: func() { stopped = true }}
	if err := s.begin(path); !errors.Is(err, want) || s.file != nil {
		t.Fatalf("start failure lost: %v", err)
	}
	if _, err := file.WriteString("late"); !errors.Is(err, os.ErrClosed) {
		t.Fatal("failed Start leaked file", err)
	}
	if err := s.finish(); err != nil || stopped {
		t.Fatal("Stop interrupted an unowned profiler")
	}
	s.start = func(io.Writer) error { return nil }
	if err := s.begin(path); err != nil {
		t.Fatal(err)
	}
	if err := s.finish(); err != nil {
		t.Fatal(err)
	}
}

func TestCPUProfileConcurrentStartStopOwnsOneDescriptor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.pprof")
	var starts, stops atomic.Int32
	var file *os.File
	s := cpuProfileSession{start: func(w io.Writer) error {
		starts.Add(1)
		file = w.(*os.File)
		_, err := io.WriteString(w, "active")
		return err
	}, stop: func() { stops.Add(1) }}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = s.begin(path) }()
	}
	wg.Wait()
	if starts.Load() != 1 {
		t.Fatalf("starts=%d", starts.Load())
	}
	if got, _ := os.ReadFile(path); !strings.EqualFold(string(got), "active") {
		t.Fatalf("repeated start truncated: %q", got)
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.finish(); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if stops.Load() != 1 {
		t.Fatalf("stops=%d", stops.Load())
	}
	if _, err := file.WriteString("late"); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("descriptor leaked: %v", err)
	}
}
