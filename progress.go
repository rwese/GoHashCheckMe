package main

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type ProgressReporter struct {
	total        int
	processed    int32
	errors       int32
	changed      int32
	startTime    time.Time
	showProgress bool
	quiet        bool
	mu           sync.Mutex
}

func NewProgressReporter(total int, showProgress, quiet bool) *ProgressReporter {
	return &ProgressReporter{
		total:        total,
		startTime:    time.Now(),
		showProgress: showProgress,
		quiet:        quiet,
	}
}

func (p *ProgressReporter) Update(changed, errored bool) {
	atomic.AddInt32(&p.processed, 1)
	if changed {
		atomic.AddInt32(&p.changed, 1)
	}
	if errored {
		atomic.AddInt32(&p.errors, 1)
	}

	if p.showProgress {
		p.displayProgress()
	}
}

func (p *ProgressReporter) displayProgress() {
	processed := atomic.LoadInt32(&p.processed)
	errors := atomic.LoadInt32(&p.errors)
	changed := atomic.LoadInt32(&p.changed)
	elapsed := time.Since(p.startTime)

	// Calculate rate
	rate := float64(processed) / elapsed.Seconds()

	// Estimate remaining time
	remaining := time.Duration(0)
	if rate > 0 {
		remainingFiles := float64(p.total - int(processed))
		remaining = time.Duration(remainingFiles/rate) * time.Second
	}

	// Clear line and print progress
	percentage := float64(processed) / float64(p.total) * 100
	fmt.Fprintf(os.Stderr, "\r\033[K[%[1]d/%[2]d] %.1[3]f%% | Changed: %[4]d | Errors: %[5]d | Rate: %.1[6]f/s | ETA: %[7]s",
		processed, p.total, percentage, changed, errors, rate, formatDuration(remaining))
}

func (p *ProgressReporter) Finish() {
	if !p.showProgress {
		return
	}

	processed := atomic.LoadInt32(&p.processed)
	errors := atomic.LoadInt32(&p.errors)
	changed := atomic.LoadInt32(&p.changed)
	elapsed := time.Since(p.startTime)

	fmt.Fprintf(os.Stderr, "\r\033[K")
	filesPerSecond := float64(processed) / elapsed.Seconds()
	fmt.Fprintf(os.Stderr, "Completed: %[1]d files in %[2]s (%.1[3]f files/s)\n",
		processed, formatDuration(elapsed), filesPerSecond)

	if changed > 0 {
		fmt.Fprintf(os.Stderr, "Changed: %d files\n", changed)
	}
	if errors > 0 {
		fmt.Fprintf(os.Stderr, "Errors: %d files\n", errors)
	}
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "0s"
	}

	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%[1]dh%[2]dm%[3]ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%[1]dm%[2]ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
