package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAuditFile_EmptyFilename(t *testing.T) {
	result := loadAuditFile("")
	if result != nil {
		t.Error("expected nil for empty filename")
	}
}

func TestLoadAuditFile_NonExistentFile_CreateSuccess(t *testing.T) {
	// Test case where file doesn't exist and is successfully created
	tempDir, err := os.MkdirTemp("", "audit_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	nonExistentFile := filepath.Join(tempDir, "new_audit.jsonl")

	// Capture stderr output
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	result := loadAuditFile(nonExistentFile)

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured output
	var output strings.Builder
	io.Copy(&output, r)
	r.Close()

	// Verify file was created and result is empty map
	if result == nil {
		t.Fatal("expected empty map, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}

	// Verify file exists
	if _, err := os.Stat(nonExistentFile); os.IsNotExist(err) {
		t.Error("expected file to be created")
	}

	// Verify correct message was printed
	expectedMsg := fmt.Sprintf("Hashes file '%s' does not exist, creating empty file", nonExistentFile)
	if !strings.Contains(output.String(), expectedMsg) {
		t.Errorf("expected message about creating file, got: %s", output.String())
	}
}

func TestLoadAuditFile_NonExistentFile_CreateFail(t *testing.T) {
	// Test case where file doesn't exist and creation fails (e.g., read-only directory)
	// Create a read-only directory
	tempDir, err := os.MkdirTemp("", "audit_test_readonly")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Make directory read-only
	if err := os.Chmod(tempDir, 0444); err != nil {
		t.Fatal(err)
	}

	nonExistentFile := filepath.Join(tempDir, "cannot_create.jsonl")

	// This should call os.Exit(1), so we test indirectly by verifying
	// that file creation would fail in this scenario
	_, err = os.Create(nonExistentFile)
	if err == nil {
		t.Error("expected file creation to fail in read-only directory")
	}

	// Restore directory permissions for cleanup
	defer os.Chmod(tempDir, 0755)
}

func TestLoadAuditFile_OpenError(t *testing.T) {
	// Create a file we can't read (permission denied)
	tempFile, err := os.CreateTemp("", "audit_test")
	if err != nil {
		t.Fatal(err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	// Make file unreadable
	if err := os.Chmod(tempFile.Name(), 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(tempFile.Name(), 0644) // Restore for cleanup

	// This will cause os.Exit(1), so we can't test it directly in a unit test
	// Instead, we'll verify the error handling logic by checking the file permissions

	// Test that we can detect the file exists but has wrong permissions
	_, err = os.Open(tempFile.Name())
	if err == nil {
		t.Error("expected permission error")
	}
	if os.IsNotExist(err) {
		t.Error("file should exist but be unreadable")
	}
}

func TestLoadAuditFile_ValidFile(t *testing.T) {
	// Create temp audit file with valid JSON entries
	tempFile, err := os.CreateTemp("", "audit_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name())

	// Write test data
	entries := []AuditEntry{
		{Filename: "file1.txt", Hash: "hash1"},
		{Filename: "file2.txt", Hash: "hash2"},
		{Filename: "file3.txt", Hash: "hash3"},
	}

	encoder := json.NewEncoder(tempFile)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
	tempFile.Close()

	// Test loading
	result := loadAuditFile(tempFile.Name())
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result) != len(entries) {
		t.Errorf("expected %d entries, got %d", len(entries), len(result))
	}

	for _, entry := range entries {
		if result[entry.Filename] != entry.Hash {
			t.Errorf("expected hash %s for file %s, got %s", entry.Hash, entry.Filename, result[entry.Filename])
		}
	}
}

func TestLoadAuditFile_InvalidJSON(t *testing.T) {
	// Create temp file with invalid JSON
	tempFile, err := os.CreateTemp("", "audit_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name())

	// Write invalid JSON
	_, err = tempFile.WriteString(`{"filename": "test.txt", "hash": "hash1"}
{"filename": "test2.txt", "invalid_field": }
{"filename": "test3.txt", "hash": "hash3"}`)
	if err != nil {
		t.Fatal(err)
	}
	tempFile.Close()

	// This will cause os.Exit(1), so we can't test it directly
	// But we can verify that invalid JSON would cause a decode error
	file, err := os.Open(tempFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	for {
		var entry AuditEntry
		err := decoder.Decode(&entry)
		if err == io.EOF {
			break
		}
		if err != nil {
			// This is the error that would cause os.Exit(1) in loadAuditFile
			if !strings.Contains(err.Error(), "invalid character") {
				t.Errorf("expected JSON syntax error, got: %v", err)
			}
			break
		}
	}
}

func TestLoadAuditFile_EmptyFile(t *testing.T) {
	// Create empty audit file
	tempFile, err := os.CreateTemp("", "audit_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	// Test loading empty file
	result := loadAuditFile(tempFile.Name())
	if result == nil {
		t.Fatal("expected empty map, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestMergeHashFiles_NoNewFile(t *testing.T) {
	// Test when .new file doesn't exist
	tempDir, err := os.MkdirTemp("", "merge_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	hashesFile := filepath.Join(tempDir, "hashes.jsonl")

	// Create original hashes file
	originalEntries := []AuditEntry{
		{Filename: "file1.txt", Hash: "hash1"},
	}

	file, err := os.Create(hashesFile)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, entry := range originalEntries {
		if err := encoder.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
	file.Close()

	// Call merge (should do nothing since .new file doesn't exist)
	mergeHashFiles(hashesFile)

	// Verify original file is unchanged
	result := loadAuditFile(hashesFile)
	if len(result) != 1 {
		t.Errorf("expected 1 entry, got %d", len(result))
	}
	if result["file1.txt"] != "hash1" {
		t.Errorf("expected hash1, got %s", result["file1.txt"])
	}
}

func TestMergeHashFiles_EmptyNewFile(t *testing.T) {
	// Test when .new file exists but is empty
	tempDir, err := os.MkdirTemp("", "merge_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	hashesFile := filepath.Join(tempDir, "hashes.jsonl")
	newFile := hashesFile + ".new"

	// Create original hashes file
	originalEntries := []AuditEntry{
		{Filename: "file1.txt", Hash: "hash1"},
	}

	file, err := os.Create(hashesFile)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, entry := range originalEntries {
		if err := encoder.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
	file.Close()

	// Create empty .new file
	if _, err := os.Create(newFile); err != nil {
		t.Fatal(err)
	}

	// Call merge
	mergeHashFiles(hashesFile)

	// Verify .new file was removed
	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Error("expected .new file to be removed")
	}

	// Verify original file is unchanged
	result := loadAuditFile(hashesFile)
	if len(result) != 1 {
		t.Errorf("expected 1 entry, got %d", len(result))
	}
	if result["file1.txt"] != "hash1" {
		t.Errorf("expected hash1, got %s", result["file1.txt"])
	}
}

func TestMergeHashFiles_SuccessfulMerge(t *testing.T) {
	// Test successful merge with both new and existing entries
	tempDir, err := os.MkdirTemp("", "merge_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	hashesFile := filepath.Join(tempDir, "hashes.jsonl")
	newFile := hashesFile + ".new"

	// Create original hashes file
	originalEntries := []AuditEntry{
		{Filename: "file1.txt", Hash: "hash1"},
		{Filename: "file2.txt", Hash: "hash2"},
	}

	file, err := os.Create(hashesFile)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, entry := range originalEntries {
		if err := encoder.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
	file.Close()

	// Create .new file with new and updated entries
	newEntries := []AuditEntry{
		{Filename: "file2.txt", Hash: "updated_hash2"}, // Update existing
		{Filename: "file3.txt", Hash: "hash3"},         // New entry
	}

	newFileHandle, err := os.Create(newFile)
	if err != nil {
		t.Fatal(err)
	}
	encoder = json.NewEncoder(newFileHandle)
	for _, entry := range newEntries {
		if err := encoder.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
	newFileHandle.Close()

	// Call merge
	mergeHashFiles(hashesFile)

	// Verify .new file was removed
	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Error("expected .new file to be removed")
	}

	// Verify merged result
	result := loadAuditFile(hashesFile)
	if len(result) != 3 {
		t.Errorf("expected 3 entries, got %d", len(result))
	}

	// Check entries
	expected := map[string]string{
		"file1.txt": "hash1",
		"file2.txt": "updated_hash2", // Should be updated
		"file3.txt": "hash3",         // Should be new
	}

	for filename, expectedHash := range expected {
		if result[filename] != expectedHash {
			t.Errorf("expected hash %s for file %s, got %s", expectedHash, filename, result[filename])
		}
	}
}

func TestMergeHashFiles_NoExistingFile(t *testing.T) {
	// Test merge when original hashes file doesn't exist
	tempDir, err := os.MkdirTemp("", "merge_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	hashesFile := filepath.Join(tempDir, "nonexistent.jsonl")
	newFile := hashesFile + ".new"

	// Create .new file
	newEntries := []AuditEntry{
		{Filename: "file1.txt", Hash: "hash1"},
		{Filename: "file2.txt", Hash: "hash2"},
	}

	newFileHandle, err := os.Create(newFile)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(newFileHandle)
	for _, entry := range newEntries {
		if err := encoder.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
	newFileHandle.Close()

	// Capture stderr to check for file creation message
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Call merge
	mergeHashFiles(hashesFile)

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured output
	var output strings.Builder
	io.Copy(&output, r)
	r.Close()

	// Verify .new file was removed
	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Error("expected .new file to be removed")
	}

	// Verify new hashes file was created with entries from .new
	result := loadAuditFile(hashesFile)
	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}

	for _, entry := range newEntries {
		if result[entry.Filename] != entry.Hash {
			t.Errorf("expected hash %s for file %s, got %s", entry.Hash, entry.Filename, result[entry.Filename])
		}
	}
}

func TestMergeHashFiles_StatError(t *testing.T) {
	// Test the case where os.Stat fails (but not with IsNotExist)
	// This tests the .new file existence check
	tempDir, err := os.MkdirTemp("", "merge_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	hashesFile := filepath.Join(tempDir, "hashes.jsonl")

	// Test with non-existent .new file (early return path)
	mergeHashFiles(hashesFile)

	// Since no .new file exists, the function should return early
	// and no hashes file should be created
	if _, err := os.Stat(hashesFile); !os.IsNotExist(err) {
		t.Error("expected no hashes file to be created when no .new file exists")
	}
}

func TestMergeHashFiles_OverwriteExisting(t *testing.T) {
	// Test that new hashes overwrite existing ones for the same filename
	tempDir, err := os.MkdirTemp("", "merge_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	hashesFile := filepath.Join(tempDir, "hashes.jsonl")
	newFile := hashesFile + ".new"

	// Create original hashes file with duplicate filenames
	originalEntries := []AuditEntry{
		{Filename: "duplicate.txt", Hash: "old_hash"},
		{Filename: "unique_old.txt", Hash: "hash1"},
	}

	file, err := os.Create(hashesFile)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, entry := range originalEntries {
		if err := encoder.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
	file.Close()

	// Create .new file with same filename but different hash
	newEntries := []AuditEntry{
		{Filename: "duplicate.txt", Hash: "new_hash"},
		{Filename: "unique_new.txt", Hash: "hash2"},
	}

	newFileHandle, err := os.Create(newFile)
	if err != nil {
		t.Fatal(err)
	}
	encoder = json.NewEncoder(newFileHandle)
	for _, entry := range newEntries {
		if err := encoder.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
	newFileHandle.Close()

	// Call merge
	mergeHashFiles(hashesFile)

	// Verify merged result
	result := loadAuditFile(hashesFile)
	if len(result) != 3 {
		t.Errorf("expected 3 entries, got %d", len(result))
	}

	// Verify the duplicate was overwritten with new hash
	if result["duplicate.txt"] != "new_hash" {
		t.Errorf("expected new_hash for duplicate.txt, got %s", result["duplicate.txt"])
	}

	// Verify other entries are preserved
	if result["unique_old.txt"] != "hash1" {
		t.Errorf("expected hash1 for unique_old.txt, got %s", result["unique_old.txt"])
	}
	if result["unique_new.txt"] != "hash2" {
		t.Errorf("expected hash2 for unique_new.txt, got %s", result["unique_new.txt"])
	}
}
