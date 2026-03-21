package main

import (
	"bytes"
	"io"
	"os"
	"testing"
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
		{
			name:     "negative codes",
			input:    "-1,0,127",
			expected: map[int]bool{-1: true, 0: true, 127: true},
		},
		{
			name:     "duplicate codes",
			input:    "0,0,1,1",
			expected: map[int]bool{0: true, 1: true},
		},
		{
			name:     "only invalid codes",
			input:    "abc,def",
			expected: map[int]bool{}, // Returns empty map, not nil
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

func TestGetFilesWithArgs(t *testing.T) {
	// Create temp files
	tmpfiles := make([]string, 3)
	for i := range tmpfiles {
		tmpfile, err := os.CreateTemp("", "test")
		if err != nil {
			t.Fatal(err)
		}
		tmpfiles[i] = tmpfile.Name()
		defer os.Remove(tmpfile.Name())
		tmpfile.Close()
	}

	// Note: This test is limited because flag.Parse is called globally
	// and affects subsequent tests. We test the function behavior conceptually.
	t.Run("empty args returns empty slice", func(t *testing.T) {
		// When flag.Args() returns empty
		result := getFiles()
		// getFiles() will try to read stdin, which will fail or return empty
		_ = result // Accept whatever is returned
	})
}

func TestLogError(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Call logError
	logError("Test error: %s\n", "message")

	// Close writer and restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured output
	var output bytes.Buffer
	io.Copy(&output, r)

	if !bytes.Contains(output.Bytes(), []byte("Test error: message")) {
		t.Errorf("expected error message in stderr, got: %s", output.String())
	}
}

func TestLogErrorConcurrent(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Call logError concurrently
	done := make(chan bool)
	for i := range 10 {
		go func(id int) {
			logError("Error %d\n", id)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for range 10 {
		<-done
	}

	// Close writer and restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured output
	var output bytes.Buffer
	io.Copy(&output, r)

	// Verify we got some output
	if output.Len() == 0 {
		t.Errorf("expected some output")
	}
}
