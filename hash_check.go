package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

var bufferPool = sync.Pool{
	New: func() any {
		return make([]byte, 64*1024) // 64KB buffer
	},
}

func processFile(filename string, cfg Config, auditMap map[string]string) *Result {
	hash, err := hashFile(filename)
	if err != nil {
		if !cfg.quiet {
			logError("Error hashing %s: %v\n", filename, err)
		}
		return nil
	}

	result := &Result{
		Filename: filename,
		Hash:     hash,
	}

	// Check audit if available
	if auditMap != nil {
		expectedHash, exists := auditMap[filename]
		if exists {
			result.Audited = true
			result.Changed = hash != expectedHash
		}
	}

	// Run command if specified
	// In audit mode, only run if file changed
	shouldRunCommand := cfg.command != "" && (!cfg.audit || result.Changed)

	if shouldRunCommand {
		result.ExitCode = runCommand(cfg, filename)

		// Handle -1 exit code (command execution error) specially
		if result.ExitCode == -1 && cfg.filterOnCodes && !cfg.errorCodes[-1] {
			if !cfg.quiet {
				logError("Command failed to run with exit code -1 for %s. If expected, add -1 to the error exit codes with --error-exit-codes\n", filename)
			}
			return nil
		}

		// Filter based on success/error codes
		if cfg.filterOnCodes {
			isSuccess := cfg.successCodes[result.ExitCode]
			isError := cfg.errorCodes[result.ExitCode]
			if !isSuccess && !isError {
				return nil
			}
		}
	}

	return result
}

func hashFile(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()

	// Get buffer from pool
	buf := bufferPool.Get().([]byte)
	defer bufferPool.Put(buf)

	// Use CopyBuffer for efficient streaming with reused buffer
	if _, err := io.CopyBuffer(h, f, buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func runCommand(cfg Config, filename string) int {
	// Replace $FILE placeholder with filename, or append filename if no placeholder
	command := cfg.command
	if strings.Contains(command, "$FILE") {
		command = strings.ReplaceAll(command, "$FILE", "'"+filename+"'")
	} else {
		// For standalone commands like "exit", "true", "false", don't append filename
		// For commands that process files, append the filename
		standaloneCommands := []string{"exit", "true", "false"}
		isStandalone := false
		for _, standalone := range standaloneCommands {
			if command == standalone || strings.HasPrefix(command, standalone+" ") {
				isStandalone = true
				break
			}
		}
		if !isStandalone {
			command = command + " '" + filename + "'"
		}
	}

	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err == nil {
		return 0
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		if !cfg.quiet {
			logError("Error running command for %s: %v\n", filename, err)
		}
		return -1
	}

	return exitErr.ExitCode()
}
