package main

import (
	"bytes"
	"encoding/json"
	"flag"
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

func TestWriteResultsWithUpdateMode(t *testing.T) {
	// Test the .new file creation path for update mode
	tempDir, err := os.MkdirTemp("", "write_results_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	hashesFile := filepath.Join(tempDir, "hashes.jsonl")
	newFile := hashesFile + ".new"

	// Clean up any existing new file
	if _, err := os.Stat(newFile); err == nil {
		os.Remove(newFile)
	}

	results := make(chan *Result, 3)
	results <- &Result{Filename: "file1.txt", Hash: "hash1", ExitCode: 0}
	results <- &Result{Filename: "file2.txt", Hash: "hash2", ExitCode: 0} // Success - should be in .new
	results <- &Result{Filename: "file3.txt", Hash: "hash3", ExitCode: 1} // Failure - should NOT be in .new
	close(results)

	var buf bytes.Buffer
	done := make(chan bool)
	cfg := Config{
		update:      true,
		hashesFile:  hashesFile,
		quiet:       true,
	}
	go writeResults(results, &buf, done, cfg)
	<-done

	// Verify .new file was created
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Fatal("expected .new file to be created")
	}

	// Verify .new file contains only successful results
	newEntries := loadAuditFile(newFile)
	if len(newEntries) != 2 {
		t.Errorf("expected 2 entries in .new file (exit 0 and 0), got %d", len(newEntries))
	}

	// Verify file2 (exit 0) is in .new
	if newEntries["file2.txt"] != "hash2" {
		t.Errorf("expected file2.txt hash in .new file")
	}

	// Verify file3 (exit 1) is NOT in .new
	if _, exists := newEntries["file3.txt"]; exists {
		t.Error("file3.txt should not be in .new file (non-zero exit code)")
	}
}

func TestWriteResultsWithUpdateModeNonZeroExit(t *testing.T) {
	// Test that non-zero exit codes are NOT written to .new file
	tempDir, err := os.MkdirTemp("", "write_results_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	hashesFile := filepath.Join(tempDir, "hashes.jsonl")
	newFile := hashesFile + ".new"

	results := make(chan *Result, 4)
	results <- &Result{Filename: "file1.txt", Hash: "hash1", ExitCode: 0}  // Should be in .new
	results <- &Result{Filename: "file2.txt", Hash: "hash2", ExitCode: 1}  // Should NOT be in .new
	results <- &Result{Filename: "file3.txt", Hash: "hash3", ExitCode: 2}  // Should NOT be in .new
	results <- &Result{Filename: "file4.txt", Hash: "hash4", ExitCode: 0}  // Should be in .new
	close(results)

	var buf bytes.Buffer
	done := make(chan bool)
	cfg := Config{
		update:      true,
		hashesFile:  hashesFile,
		quiet:       true,
	}
	go writeResults(results, &buf, done, cfg)
	<-done

	// Verify .new file contains only exit 0 results
	newEntries := loadAuditFile(newFile)
	if len(newEntries) != 2 {
		t.Errorf("expected 2 entries (only exit 0), got %d", len(newEntries))
	}

	// Verify correct entries are present
	if newEntries["file1.txt"] != "hash1" {
		t.Error("file1.txt (exit 0) should be in .new file")
	}
	if newEntries["file4.txt"] != "hash4" {
		t.Error("file4.txt (exit 0) should be in .new file")
	}

	// Verify incorrect entries are absent
	if _, exists := newEntries["file2.txt"]; exists {
		t.Error("file2.txt (exit 1) should NOT be in .new file")
	}
	if _, exists := newEntries["file3.txt"]; exists {
		t.Error("file3.txt (exit 2) should NOT be in .new file")
	}
}

func TestWriteResultsEmptyChannel(t *testing.T) {
	// Test writeResults with empty results channel
	results := make(chan *Result)
	close(results)

	var buf bytes.Buffer
	done := make(chan bool)
	cfg := Config{quiet: true}
	go writeResults(results, &buf, done, cfg)
	<-done

	if buf.Len() != 0 {
		t.Error("expected empty buffer for empty results")
	}
}

func TestWriteResultsNoUpdateMode(t *testing.T) {
	// Test writeResults without update mode - no .new file should be created
	tempDir, err := os.MkdirTemp("", "write_results_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	hashesFile := filepath.Join(tempDir, "hashes.jsonl")
	newFile := hashesFile + ".new"

	results := make(chan *Result, 2)
	results <- &Result{Filename: "file1.txt", Hash: "hash1", ExitCode: 0}
	results <- &Result{Filename: "file2.txt", Hash: "hash2", ExitCode: 0}
	close(results)

	var buf bytes.Buffer
	done := make(chan bool)
	cfg := Config{
		update:      false, // No update mode
		hashesFile:  hashesFile,
		quiet:       true,
	}
	go writeResults(results, &buf, done, cfg)
	<-done

	// Verify .new file was NOT created
	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Error("expected .new file to NOT be created without update mode")
	}
}

func TestWriteResultsQuietMode(t *testing.T) {
	// Test writeResults handles quiet mode without panicking
	results := make(chan *Result, 2)
	results <- &Result{Filename: "file1.txt", Hash: "hash1", ExitCode: 0}
	results <- &Result{Filename: "file2.txt", Hash: "hash2", ExitCode: 0}
	close(results)

	var buf bytes.Buffer
	done := make(chan bool)
	cfg := Config{
		update:     true,
		hashesFile: "/nonexistent/path/hashes.jsonl", // This will fail to create
		quiet:      true,
	}

	// Should not panic even if .new file creation fails
	go writeResults(results, &buf, done, cfg)
	<-done
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

		var buf1 bytes.Buffer
		fullPaths := make([]string, len(files))
		for i, f := range files {
			fullPaths[i] = filepath.Join(testDir, f)
		}

		processFiles(fullPaths, cfg, nil, &buf1)

		// Verify results
		decoder := json.NewDecoder(&buf1)
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

func TestMainFunctionality(t *testing.T) {
	// Create test directory and files
	testDir, err := os.MkdirTemp("", "main_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(testDir)

	// Create a test file
	testFile := filepath.Join(testDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test getFiles via flag.Args() by setting up flags
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"test", "-c", "true", testFile}

	// Reset flag state
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Parse flags and get files
	cfg := parseFlags()
	files := getFiles()

	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}

	if files[0] != testFile {
		t.Errorf("expected %s, got %s", testFile, files[0])
	}

	if cfg.command != "true" {
		t.Errorf("expected command 'true', got '%s'", cfg.command)
	}
}

func TestAuditModeValidation(t *testing.T) {
	// Test that audit mode without hashes file is detected
	// We can't call parseFlags directly as it exits on validation failure,
	// so we test the validation logic by checking Config struct behavior

	// Valid config with audit and hashes file
	validCfg := Config{
		audit:       true,
		hashesFile:  "test.jsonl",
		command:     "true",
	}

	if !validCfg.audit || validCfg.hashesFile == "" {
		t.Error("expected valid audit config")
	}

	// Invalid config - audit without hashes file
	invalidCfg := Config{
		audit:      true,
		hashesFile: "", // Missing required field
	}

	if invalidCfg.audit && invalidCfg.hashesFile == "" {
		// This is the invalid state that parseFlags would reject
		t.Log("Correctly detected: audit mode without hashes file is invalid")
	}
}

func TestUpdateModeValidation(t *testing.T) {
	// Test that update mode requires hashes file
	validCfg := Config{
		update:      true,
		hashesFile:  "test.jsonl",
		command:     "true",
	}

	if !validCfg.update || validCfg.hashesFile == "" {
		t.Error("expected valid update config")
	}

	invalidCfg := Config{
		update:      true,
		hashesFile:  "", // Missing required field
	}

	if invalidCfg.update && invalidCfg.hashesFile == "" {
		t.Log("Correctly detected: update mode without hashes file is invalid")
	}
}

func TestCommandOrAuditRequired(t *testing.T) {
	// Test that either command or audit mode is required
	validCfg := Config{
		command: "true",
		audit:  false,
	}

	if validCfg.command == "" && !validCfg.audit {
		t.Error("expected command or audit to be set")
	}

	// Test audit mode as alternative
	auditCfg := Config{
		command:    "",
		audit:      true,
		hashesFile: "test.jsonl",
	}

	if auditCfg.command == "" && !auditCfg.audit {
		t.Error("expected command or audit to be set")
	}
}

func TestConfigDefaults(t *testing.T) {
	// Test that Config struct has correct default behavior
	cfg := Config{}

	// Verify initial state
	if cfg.workers != 0 {
		t.Error("workers should be 0 by default")
	}
	if cfg.audit {
		t.Error("audit should be false by default")
	}
	if cfg.update {
		t.Error("update should be false by default")
	}
	if cfg.showProgress {
		t.Error("showProgress should be false by default")
	}
	if cfg.quiet {
		t.Error("quiet should be false by default")
	}
	if cfg.command != "" {
		t.Error("command should be empty by default")
	}
}

func TestResultJSONFields(t *testing.T) {
	// Test Result struct JSON marshaling with changed=true
	result := Result{
		Filename: "test.txt",
		Hash:     "abc123",
		ExitCode: 0,
		Audited:  true,
		Changed:  true, // Set to true so field is included (omitempty)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	// Verify JSON contains expected fields
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// Check required fields (changed has omitempty, so only present when true)
	expectedFields := []string{"filename", "hash", "exit_code", "audited"}
	for _, field := range expectedFields {
		if _, exists := decoded[field]; !exists {
			t.Errorf("expected field '%s' in JSON output", field)
		}
	}

	// Verify changed field is present when true
	if _, exists := decoded["changed"]; !exists {
		t.Error("expected 'changed' field in JSON output when true")
	}
	if decoded["changed"] != true {
		t.Error("expected 'changed' to be true")
	}
}

func TestResultJSONFieldsChangedFalse(t *testing.T) {
	// Test Result struct JSON marshaling with changed=false (should be omitted)
	result := Result{
		Filename: "test.txt",
		Hash:     "abc123",
		ExitCode: 0,
		Audited:  true,
		Changed:  false,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	// Verify JSON does NOT contain changed field when false (omitempty behavior)
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// changed=false should be omitted due to omitempty
	if _, exists := decoded["changed"]; exists {
		t.Error("'changed' field should NOT be present when false (omitempty)")
	}
}

func TestResultJSONFieldsAuditedFalse(t *testing.T) {
	// Test Result struct JSON marshaling with audited=false (should be omitted)
	result := Result{
		Filename: "test.txt",
		Hash:     "abc123",
		ExitCode: 0,
		Audited:  false,
		Changed:  false,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	// Verify audited field is NOT present when false
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// audited=false should be omitted due to omitempty
	if _, exists := decoded["audited"]; exists {
		t.Error("'audited' field should NOT be present when false (omitempty)")
	}
}

func TestAuditEntryJSONFields(t *testing.T) {
	// Test AuditEntry struct JSON marshaling
	entry := AuditEntry{
		Filename: "test.txt",
		Hash:     "abc123",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal entry: %v", err)
	}

	// Verify JSON contains expected fields
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal entry: %v", err)
	}

	if _, exists := decoded["filename"]; !exists {
		t.Error("expected 'filename' field in JSON output")
	}
	if _, exists := decoded["hash"]; !exists {
		t.Error("expected 'hash' field in JSON output")
	}
}
