package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func loadAuditFile(filename string) map[string]string {
	if filename == "" {
		return nil
	}

	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Hashes file '%s' does not exist, creating empty file\n", filename)
			// Create empty file
			if newFile, createErr := os.Create(filename); createErr == nil {
				newFile.Close()
				return make(map[string]string)
			} else {
				fmt.Fprintf(os.Stderr, "Error creating hashes file: %v\n", createErr)
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error opening hashes file: %v\n", err)
			os.Exit(1)
		}
	}
	defer f.Close()

	auditMap := make(map[string]string)
	decoder := json.NewDecoder(f)

	for {
		var entry AuditEntry
		err := decoder.Decode(&entry)
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading hashes file: %v\n", err)
			os.Exit(1)
		}
		auditMap[entry.Filename] = entry.Hash
	}

	return auditMap
}

func mergeHashFiles(hashesFile string) {
	newFile := hashesFile + ".new"

	// Check if .new file exists
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		return // No .new file to merge
	}

	// Load existing hashes
	existingHashes := loadAuditFile(hashesFile)
	if existingHashes == nil {
		existingHashes = make(map[string]string)
	}

	// Load new hashes
	newHashes := loadAuditFile(newFile)
	if newHashes == nil {
		os.Remove(newFile) // Clean up empty .new file
		return
	}

	// Merge new hashes into existing ones (overwrites existing entries for same filename)
	for filename, hash := range newHashes {
		existingHashes[filename] = hash
	}

	// Write merged hashes back to the original file
	f, err := os.Create(hashesFile)
	if err != nil {
		logError("Error creating merged hashes file: %v\n", err)
		return
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for filename, hash := range existingHashes {
		entry := AuditEntry{Filename: filename, Hash: hash}
		if err := encoder.Encode(entry); err != nil {
			logError("Error writing merged hash entry: %v\n", err)
		}
	}

	// Remove the .new file after successful merge
	os.Remove(newFile)
}
