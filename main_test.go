package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConcurrentProcessing(t *testing.T) {
	// Create multiple temp files
	var files []string
	for i := range 10 {
		tmpfile, err := os.CreateTemp("", "test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpfile.Name())

		content := []byte(strings.Repeat("x", i*100))
		if _, err := tmpfile.Write(content); err != nil {
			t.Fatal(err)
		}
		tmpfile.Close()
		files = append(files, tmpfile.Name())
	}

	// Process files concurrently
	cfg := Config{
		command:      "true",
		workers:      4,
		showProgress: false,
		quiet:        true,
	}

	var buf bytes.Buffer
	processFiles(files, cfg, nil, &buf)

	// Check results
	decoder := json.NewDecoder(&buf)
	results := make(map[string]bool)

	for {
		var result Result
		err := decoder.Decode(&result)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("error decoding result: %v", err)
		}
		results[result.Filename] = true
	}

	// Verify all files were processed
	if len(results) != len(files) {
		t.Errorf("expected %d results, got %d", len(files), len(results))
	}
	for _, file := range files {
		if !results[file] {
			t.Errorf("file %s not found in results", file)
		}
	}
}

func TestWriteResults(t *testing.T) {
	results := make(chan *Result, 3)
	results <- &Result{Filename: "file1.txt", Hash: "hash1", ExitCode: 0}
	results <- &Result{Filename: "file2.txt", Hash: "hash2", ExitCode: 1, Audited: true, Changed: true}
	results <- &Result{Filename: "file3.txt", Hash: "hash3", ExitCode: 0}
	close(results)

	var buf bytes.Buffer
	done := make(chan bool)
	cfg := Config{quiet: false}
	go writeResults(results, &buf, done, cfg)
	<-done

	// Check output
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}

	// Verify each line is valid JSON
	for i, line := range lines {
		var result Result
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}

func TestIntegration(t *testing.T) {
	// Create test directory
	testDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(testDir)

	// Create test files
	files := []string{"file1.txt", "file2.txt", "file3.txt"}
	for i, name := range files {
		path := filepath.Join(testDir, name)
		content := []byte(strings.Repeat("content", i+1))
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Test 1: Basic processing
	t.Run("basic processing", func(t *testing.T) {
		cfg := Config{
			command:      "true",
			workers:      2,
			showProgress: false,
			quiet:        true,
		}

		var buf bytes.Buffer
		fullPaths := make([]string, len(files))
		for i, f := range files {
			fullPaths[i] = filepath.Join(testDir, f)
		}

		processFiles(fullPaths, cfg, nil, &buf)

		// Verify results
		decoder := json.NewDecoder(&buf)
		count := 0
		for {
			var result Result
			if err := decoder.Decode(&result); err == io.EOF {
				break
			} else if err != nil {
				t.Fatal(err)
			}
			count++
			if result.ExitCode != 0 {
				t.Errorf("expected exit code 0, got %d", result.ExitCode)
			}
		}
		if count != len(files) {
			t.Errorf("expected %d results, got %d", len(files), count)
		}
	})

	// Test 2: Audit functionality
	t.Run("audit functionality", func(t *testing.T) {
		// Create audit file
		auditFile, err := os.CreateTemp("", "audit*.jsonl")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(auditFile.Name())

		// First pass: create baseline
		cfg := Config{
			command:      "true",
			workers:      1,
			showProgress: false,
			quiet:        true,
		}

		var buf1 bytes.Buffer
		fullPaths := make([]string, len(files))
		for i, f := range files {
			fullPaths[i] = filepath.Join(testDir, f)
		}

		processFiles(fullPaths, cfg, nil, &buf1)

		// Save to audit file
		encoder := json.NewEncoder(auditFile)
		decoder := json.NewDecoder(&buf1)
		for {
			var result Result
			if err := decoder.Decode(&result); err == io.EOF {
				break
			} else if err != nil {
				t.Fatal(err)
			}
			entry := AuditEntry{Filename: result.Filename, Hash: result.Hash}
			if err := encoder.Encode(entry); err != nil {
				t.Fatal(err)
			}
		}
		auditFile.Close()

		// Modify one file
		modifiedFile := filepath.Join(testDir, files[1])
		if err := os.WriteFile(modifiedFile, []byte("modified content"), 0644); err != nil {
			t.Fatal(err)
		}

		// Second pass: check against audit
		auditMap := loadAuditFile(auditFile.Name())
		cfg.command = "echo modified"
		cfg.showProgress = false
		cfg.quiet = true

		var buf2 bytes.Buffer
		processFiles(fullPaths, cfg, auditMap, &buf2)

		// Check results
		decoder2 := json.NewDecoder(&buf2)
		modifiedCount := 0
		for {
			var result Result
			if err := decoder2.Decode(&result); err == io.EOF {
				break
			} else if err != nil {
				t.Fatal(err)
			}
			if result.Changed {
				modifiedCount++
				// Command should have run on changed file, exit code depends on command
				// "echo modified" returns 0, so we expect 0
				if result.ExitCode != 0 {
					t.Errorf("expected command exit code 0 for changed file, got %d", result.ExitCode)
				}
			} else {
				// Command should not run on unchanged file, so exit code should be 0 (default)
				if result.ExitCode != 0 {
					t.Error("command should not run on unchanged file")
				}
			}
		}
		if modifiedCount != 1 {
			t.Errorf("expected 1 modified file, got %d", modifiedCount)
		}
	})
}
