package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[int]bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single code",
			input:    "0",
			expected: map[int]bool{0: true},
		},
		{
			name:     "multiple codes",
			input:    "0,1,2",
			expected: map[int]bool{0: true, 1: true, 2: true},
		},
		{
			name:     "with spaces",
			input:    " 0 , 1 , 2 ",
			expected: map[int]bool{0: true, 1: true, 2: true},
		},
		{
			name:     "with invalid codes",
			input:    "0,abc,2",
			expected: map[int]bool{0: true, 2: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseExitCodes(tt.input)
			if tt.expected == nil && result != nil {
				t.Errorf("expected nil, got %v", result)
				return
			}
			if tt.expected != nil && result == nil {
				t.Errorf("expected %v, got nil", tt.expected)
				return
			}
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d codes, got %d", len(tt.expected), len(result))
				return
			}
			for code := range tt.expected {
				if !result[code] {
					t.Errorf("expected code %d to be present", code)
				}
			}
		})
	}
}

func TestHashFile(t *testing.T) {
	// Create temp file
	tmpfile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := []byte("hello world")
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test successful hash
	hash, err := hashFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected SHA256 of "hello world"
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if hash != expected {
		t.Errorf("expected hash %s, got %s", expected, hash)
	}

	// Test non-existent file
	_, err = hashFile("non-existent-file")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

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

func TestProcessFile(t *testing.T) {
	// Create temp file
	tmpfile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := []byte("test content")
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		cfg       Config
		auditMap  map[string]string
		expectNil bool
	}{
		{
			name: "no command, no audit",
			cfg: Config{
				command: "",
			},
			auditMap:  nil,
			expectNil: false,
		},
		{
			name: "with command",
			cfg: Config{
				command: "true",
			},
			auditMap:  nil,
			expectNil: false,
		},
		{
			name: "with audit - unchanged",
			cfg: Config{
				command: "true",
			},
			auditMap: map[string]string{
				tmpfile.Name(): "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
			},
			expectNil: false,
		},
		{
			name: "with exit code filter - filtered out",
			cfg: Config{
				command:      "false",
				exitCodes:    map[int]bool{0: true},
				storeOnCodes: true,
			},
			auditMap:  nil,
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processFile(tmpfile.Name(), tt.cfg, tt.auditMap)
			if tt.expectNil && result != nil {
				t.Error("expected nil result")
			}
			if !tt.expectNil && result == nil {
				t.Error("expected non-nil result")
			}
			if result != nil && result.Filename != tmpfile.Name() {
				t.Errorf("expected filename %s, got %s", tmpfile.Name(), result.Filename)
			}
		})
	}
}

func TestLoadAuditFile(t *testing.T) {
	// Create temp audit file
	tmpfile, err := os.CreateTemp("", "audit*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Write test data
	entries := []AuditEntry{
		{Filename: "file1.txt", Hash: "hash1"},
		{Filename: "file2.txt", Hash: "hash2"},
	}

	encoder := json.NewEncoder(tmpfile)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
	tmpfile.Close()

	// Test loading
	auditMap := loadAuditFile(tmpfile.Name())
	if len(auditMap) != 2 {
		t.Errorf("expected 2 entries, got %d", len(auditMap))
	}
	if auditMap["file1.txt"] != "hash1" {
		t.Errorf("expected hash1 for file1.txt, got %s", auditMap["file1.txt"])
	}
	if auditMap["file2.txt"] != "hash2" {
		t.Errorf("expected hash2 for file2.txt, got %s", auditMap["file2.txt"])
	}

	// Test empty filename
	result := loadAuditFile("")
	if result != nil {
		t.Error("expected nil for empty filename")
	}
}

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
		command: "true",
		workers: 4,
		showProgress: false,
		quiet: true,
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

func TestRunCommand(t *testing.T) {
	tests := []struct {
		name         string
		command      string
		expectedCode int
	}{
		{
			name:         "successful command",
			command:      "true",
			expectedCode: 0,
		},
		{
			name:         "failing command",
			command:      "false",
			expectedCode: 1,
		},
		{
			name:         "command with specific exit code",
			command:      "exit 42",
			expectedCode: 42,
		},
	}

	tmpfile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				command: tt.command,
				quiet: false,
			}
			code := runCommand(cfg, tmpfile.Name())
			if code != tt.expectedCode {
				t.Errorf("expected exit code %d, got %d", tt.expectedCode, code)
			}
		})
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
			command: "true",
			workers: 2,
			showProgress: false,
			quiet: true,
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
			command: "true",
			workers: 1,
			showProgress: false,
			quiet: true,
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

func BenchmarkHashFile(b *testing.B) {
	// Create test files of various sizes
	sizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
		{"10MB", 10 * 1024 * 1024},
		{"100MB", 100 * 1024 * 1024},
	}
	
	for _, tc := range sizes {
		b.Run(tc.name, func(b *testing.B) {
			// Create temp file with random data
			tmpfile, err := os.CreateTemp("", "bench")
			if err != nil {
				b.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())
			
			// Write random data
			data := make([]byte, tc.size)
			for i := range data {
				data[i] = byte(i % 256)
			}
			if _, err := tmpfile.Write(data); err != nil {
				b.Fatal(err)
			}
			tmpfile.Close()
			
			// Reset timer to exclude setup
			b.ResetTimer()
			b.SetBytes(int64(tc.size))
			
			// Benchmark hashing
			for i := 0; i < b.N; i++ {
				_, err := hashFile(tmpfile.Name())
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestBufferPool(t *testing.T) {
	// Test buffer pool functionality
	var wg sync.WaitGroup
	
	// Create multiple goroutines to test concurrent access
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			// Get buffer from pool
			buf := bufferPool.Get().([]byte)
			
			// Verify buffer size
			if len(buf) != 64*1024 {
				t.Errorf("expected buffer size 65536, got %d", len(buf))
			}
			
			// Use buffer
			copy(buf[:10], []byte("test data"))
			
			// Return to pool
			bufferPool.Put(buf)
		}()
	}
	
	wg.Wait()
}
