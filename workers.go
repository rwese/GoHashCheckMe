package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

var errMutex sync.Mutex

func logError(format string, args ...any) {
	errMutex.Lock()
	fmt.Fprintf(os.Stderr, format, args...)
	errMutex.Unlock()
}

func processFiles(files []string, cfg Config, auditMap map[string]string, output io.Writer) {
	jobs := make(chan string, len(files))
	results := make(chan *Result, len(files))

	// Initialize progress reporter
	progress := NewProgressReporter(len(files), cfg.showProgress, cfg.quiet)

	// Start workers
	var wg sync.WaitGroup
	for range cfg.workers {
		wg.Add(1)
		go worker(&wg, jobs, results, cfg, auditMap, progress)
	}

	// Send jobs
	for _, file := range files {
		jobs <- file
	}
	close(jobs)

	// Start result writer
	done := make(chan bool)
	go writeResults(results, output, done, cfg)

	// Wait for workers
	wg.Wait()
	close(results)

	// Wait for writer
	<-done

	// Show final progress
	progress.Finish()
}

func worker(wg *sync.WaitGroup, jobs <-chan string, results chan<- *Result, cfg Config, auditMap map[string]string, progress *ProgressReporter) {
	defer wg.Done()

	for filename := range jobs {
		result := processFile(filename, cfg, auditMap)

		// Update progress
		changed := result != nil && result.Changed
		errored := result == nil
		progress.Update(changed, errored)

		if result != nil {
			results <- result
		}
	}
}

func writeResults(results <-chan *Result, output io.Writer, done chan<- bool, cfg Config) {
	encoder := json.NewEncoder(output)

	// Open .new file for successful hashes if update mode is enabled
	var newFile *os.File
	var newEncoder *json.Encoder
	if cfg.update && cfg.hashesFile != "" {
		var err error
		newFile, err = os.Create(cfg.hashesFile + ".new")
		if err != nil {
			if !cfg.quiet {
				logError("Error creating .new file: %v\n", err)
			}
		} else {
			newEncoder = json.NewEncoder(newFile)
		}
	}

	for result := range results {
		// Write to main output
		if err := encoder.Encode(result); err != nil {
			if !cfg.quiet {
				logError("Error encoding result: %v\n", err)
			}
		}

		// Write successful results to .new file if update mode is enabled
		if newEncoder != nil && result.ExitCode == 0 {
			entry := AuditEntry{Filename: result.Filename, Hash: result.Hash}
			if err := newEncoder.Encode(entry); err != nil {
				if !cfg.quiet {
					logError("Error writing to .new file: %v\n", err)
				}
			}
		}
	}

	if newFile != nil {
		newFile.Close()
	}

	done <- true
}
