package main

import (
	"os"
	"sync"
	"testing"
)

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
				command:       "false",
				successCodes:  map[int]bool{0: true},
				filterOnCodes: true,
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
				quiet:   false,
			}
			code := runCommand(cfg, tmpfile.Name())
			if code != tt.expectedCode {
				t.Errorf("expected exit code %d, got %d", tt.expectedCode, code)
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

func TestProcessFile_NewExitCodeHandling(t *testing.T) {
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
		expectNil bool
	}{
		{
			name: "success codes only - success result included",
			cfg: Config{
				command:       "true", // exits with 0
				successCodes:  map[int]bool{0: true},
				filterOnCodes: true,
			},
			expectNil: false,
		},
		{
			name: "success codes only - error result filtered out",
			cfg: Config{
				command:       "false", // exits with 1
				successCodes:  map[int]bool{0: true},
				filterOnCodes: true,
			},
			expectNil: true,
		},
		{
			name: "error codes only - error result included",
			cfg: Config{
				command:       "false", // exits with 1
				errorCodes:    map[int]bool{1: true},
				filterOnCodes: true,
			},
			expectNil: false,
		},
		{
			name: "error codes only - success result filtered out",
			cfg: Config{
				command:       "true", // exits with 0
				errorCodes:    map[int]bool{1: true},
				filterOnCodes: true,
			},
			expectNil: true,
		},
		{
			name: "both success and error codes - success included",
			cfg: Config{
				command:       "true", // exits with 0
				successCodes:  map[int]bool{0: true},
				errorCodes:    map[int]bool{1: true},
				filterOnCodes: true,
			},
			expectNil: false,
		},
		{
			name: "both success and error codes - error included",
			cfg: Config{
				command:       "false", // exits with 1
				successCodes:  map[int]bool{0: true},
				errorCodes:    map[int]bool{1: true},
				filterOnCodes: true,
			},
			expectNil: false,
		},
		{
			name: "both success and error codes - other code filtered out",
			cfg: Config{
				command:       "exit 2", // exits with 2
				successCodes:  map[int]bool{0: true},
				errorCodes:    map[int]bool{1: true},
				filterOnCodes: true,
			},
			expectNil: true,
		},
		{
			name: "no filtering - all results included",
			cfg: Config{
				command:       "false",
				filterOnCodes: false,
			},
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processFile(tmpfile.Name(), tt.cfg, nil)
			if tt.expectNil && result != nil {
				t.Errorf("expected nil result, got result with exit code %d", result.ExitCode)
			}
			if !tt.expectNil && result == nil {
				t.Error("expected non-nil result, got nil")
			}
		})
	}
}

func TestProcessFile_ErrorExitCodeMinus1(t *testing.T) {
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
		name             string
		cfg              Config
		expectNil        bool
		expectedExitCode int
	}{
		{
			name: "command not found with 127 not in error codes - filtered out",
			cfg: Config{
				command:       "nonexistentcommand12345", // Shell returns 127 for command not found
				errorCodes:    map[int]bool{1: true},     // 127 not included
				filterOnCodes: true,
				quiet:         false,
			},
			expectNil:        true,
			expectedExitCode: 127,
		},
		{
			name: "command not found with 127 in error codes - included",
			cfg: Config{
				command:       "nonexistentcommand12345", // Shell returns 127 for command not found
				errorCodes:    map[int]bool{127: true, 1: true},
				filterOnCodes: true,
				quiet:         false,
			},
			expectNil:        false,
			expectedExitCode: 127,
		},
		{
			name: "command not found in quiet mode",
			cfg: Config{
				command:       "nonexistentcommand12345",
				errorCodes:    map[int]bool{1: true},
				filterOnCodes: true,
				quiet:         true,
			},
			expectNil:        true,
			expectedExitCode: 127,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processFile(tmpfile.Name(), tt.cfg, nil)
			if tt.expectNil && result != nil {
				t.Errorf("expected nil result, got result with exit code %d", result.ExitCode)
			}
			if !tt.expectNil && result == nil {
				t.Error("expected non-nil result, got nil")
			}
			if result != nil && result.ExitCode != tt.expectedExitCode {
				t.Errorf("expected exit code %d, got %d", tt.expectedExitCode, result.ExitCode)
			}
		})
	}
}
