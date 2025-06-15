package main

import (
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
