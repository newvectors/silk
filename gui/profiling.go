package gui

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

type cpuProfileSession struct {
	mu    sync.Mutex
	file  *os.File
	start func(io.Writer) error
	stop  func()
}

var cpuProfiler = cpuProfileSession{start: pprof.StartCPUProfile, stop: pprof.StopCPUProfile}

// StartCPUProfile begins recording a Go pprof CPU profile to filename.
// Call StopCPUProfile to flush and close it. A repeated Start is rejected
// before opening/truncating any file. Start/Stop are safe across goroutines.
// Only one process-wide pprof CPU recorder can run at a time.
func StartCPUProfile(filename string) error { return cpuProfiler.begin(filename) }

func (s *cpuProfileSession) begin(filename string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file != nil {
		return fmt.Errorf("CPU profile already running")
	}
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if err := s.start(f); err != nil {
		f.Close()
		return err
	}
	s.file = f
	return nil
}

// StopCPUProfile flushes and closes the profile started by StartCPUProfile.
// With no owned session it is a no-op (it does not stop another pprof owner).
func StopCPUProfile() {
	if err := cpuProfiler.finish(); err != nil {
		fmt.Fprintf(os.Stderr, "silk CPU profile close: %v\n", err)
	}
}

func (s *cpuProfileSession) finish() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	// Keep ownership until pprof has joined its writer and the descriptor is
	// closed; otherwise a concurrent Start could truncate a still-flushing file.
	s.stop()
	err := s.file.Close()
	s.file = nil
	return err
}

// WriteHeapProfile triggers a GC and writes the current heap profile to
// filename. Use for one-shot allocation snapshots.
func WriteHeapProfile(filename string) error {
	runtime.GC()
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return pprof.WriteHeapProfile(f)
}

// MemStats returns a one-line human-readable summary of the current process
// memory state (live allocation, lifetime allocation, OS-reserved, GC count,
// goroutine count). Useful for inline logging or perf overlays.
func MemStats() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return fmt.Sprintf("Alloc=%dKB TotalAlloc=%dMB Sys=%dMB NumGC=%d Goroutines=%d",
		m.Alloc/1024, m.TotalAlloc/1024/1024, m.Sys/1024/1024, m.NumGC,
		runtime.NumGoroutine())
}

// MemStatsDetailed returns a multi-line dump with additional fields useful
// when chasing leaks: heap objects, mspan/mcache, GC pause stats.
func MemStatsDetailed() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	var lastGCMs uint64
	if m.LastGC != 0 {
		lastGCMs = (uint64(time.Now().UnixNano()) - m.LastGC) / 1_000_000
	}
	return fmt.Sprintf(
		"Alloc=%dKB\nTotalAlloc=%dMB\nSys=%dMB\nHeapObjects=%d\nHeapInuse=%dKB\nStackInuse=%dKB\nNumGC=%d\nLastGC=%dms ago\nGoroutines=%d",
		m.Alloc/1024,
		m.TotalAlloc/1024/1024,
		m.Sys/1024/1024,
		m.HeapObjects,
		m.HeapInuse/1024,
		m.StackInuse/1024,
		m.NumGC,
		lastGCMs,
		runtime.NumGoroutine(),
	)
}
