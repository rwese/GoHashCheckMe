package main

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestProgressReporter(t *testing.T) {
	tests := []struct {
		name         string
		total        int
		showProgress bool
		quiet        bool
	}{
		{
			name:         "with progress",
			total:        100,
			showProgress: true,
			quiet:        false,
		},
		{
			name:         "without progress",
			total:        100,
			showProgress: false,
			quiet:        false,
		},
		{
			name:         "quiet mode",
			total:        100,
			showProgress: false,
			quiet:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			progress := NewProgressReporter(tt.total, tt.showProgress, tt.quiet)

			// Simulate processing
			for i := range 10 {
				progress.Update(i%3 == 0, i%5 == 0)
			}

			// Check counters
			if progress.processed != 10 {
				t.Errorf("expected 10 processed, got %d", progress.processed)
			}

			// Finish should not panic
			progress.Finish()
		})
	}
}

func TestProgressReporterAtomicCounters(t *testing.T) {
	progress := NewProgressReporter(100, false, false)

	// Test Update increments processed
	progress.Update(true, false)
	if atomic.LoadInt32(&progress.processed) != 1 {
		t.Errorf("expected processed to be 1")
	}

	// Test Update increments changed
	if atomic.LoadInt32(&progress.changed) != 1 {
		t.Errorf("expected changed to be 1")
	}

	// Test Update increments errors
	progress.Update(false, true)
	if atomic.LoadInt32(&progress.errors) != 1 {
		t.Errorf("expected errors to be 1")
	}
}

func TestProgressReporterFinishWithNoProgress(t *testing.T) {
	// Test that Finish doesn't panic with 0 processed
	progress := NewProgressReporter(100, true, false)
	progress.Finish()
}

func TestProgressReporterConcurrent(t *testing.T) {
	progress := NewProgressReporter(1000, false, false)

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 10 {
				progress.Update(j%2 == 0, j%3 == 0)
			}
		}()
	}
	wg.Wait()

	// 100 goroutines * 10 updates each = 1000 processed
	if atomic.LoadInt32(&progress.processed) != 1000 {
		t.Errorf("expected 1000 processed, got %d", progress.processed)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{time.Second * 45, "45s"},
		{time.Minute*2 + time.Second*30, "2m30s"},
		{time.Hour*1 + time.Minute*30 + time.Second*45, "1h30m45s"},
		{time.Duration(0), "0s"},
		{time.Duration(-1), "0s"},
		{time.Second * 30, "30s"},
		{time.Minute * 5, "5m0s"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
