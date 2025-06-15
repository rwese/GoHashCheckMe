package main

import (
	"fmt"
	"io"
	"os"
)

type Config struct {
	command       string
	hashesFile    string
	audit         bool
	update        bool
	successCodes  map[int]bool
	errorCodes    map[int]bool
	filterOnCodes bool
	workers       int
	showProgress  bool
	quiet         bool
}

type Result struct {
	Filename string `json:"filename"`
	Hash     string `json:"hash"`
	ExitCode int    `json:"exit_code"`
	Audited  bool   `json:"audited,omitempty"`
	Changed  bool   `json:"changed,omitempty"`
}

type AuditEntry struct {
	Filename string `json:"filename"`
	Hash     string `json:"hash"`
}

func main() {
	cfg := parseFlags()

	files := getFiles()
	if len(files) == 0 && cfg.hashesFile == "" {
		fmt.Fprintln(os.Stderr, "No files to process")
		os.Exit(1)
	}

	auditMap := loadAuditFile(cfg.hashesFile)

	// If audit mode and no files specified, check all audit entries
	if cfg.hashesFile != "" && len(files) == 0 {
		for filename := range auditMap {
			files = append(files, filename)
		}
	}

	// Determine output writer: suppress stdout if quiet mode and hashes file are both enabled
	var output io.Writer = os.Stdout
	if cfg.quiet && cfg.hashesFile != "" {
		output = io.Discard
	}

	processFiles(files, cfg, auditMap, output)

	// Handle update mode: merge new hashes into existing file
	if cfg.update {
		mergeHashFiles(cfg.hashesFile)
	}
}
