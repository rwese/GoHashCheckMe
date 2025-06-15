package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

func showUsage() {
	fmt.Fprintf(os.Stderr, `Usage: %[1]s [OPTIONS] [FILES...]

GoHashCheckMe - Run commands on files and store their hashes based on exit codes.
Avoids expensive operations on unchanged files when using audit mode.

OPTIONS:
  -c, --check-command COMMAND    Command to run on each file
  -a, --audit                   Enable audit mode (only run commands on changed/unknown files)
  -f, --hashes-file FILE        File with known hashes for audit mode (JSONL format)
  -u, --update                  Update hashes file with new successful file hashes
  --success-exit-codes CODES   Comma-separated success exit codes to include in output
  --error-exit-codes CODES     Comma-separated error exit codes to include in output
  -w, --workers N               Number of concurrent workers (default: CPU count)
  -p, --progress                Show progress bar
  -q, --quiet                   Quiet mode (no error output, suppresses stdout if -f given)
  -h, --help                    Show this help message

EXAMPLES:
  # Run linter on all Go files and store results
  %[1]s -c "golint" *.go

  # Enable audit mode with hashes file, only run command on changed/unknown files
  %[1]s -a -f audit.jsonl -c "mycheck" *.txt

  # Update hashes file with successful results from new files
  %[1]s -u -f hashes.jsonl -c "mycheck" *.txt

  # Combine audit and update: only check changed files and update successful ones
  %[1]s -a -u -f hashes.jsonl -c "mycheck" *.txt

  # Filter output to only show files where command returned success exit codes
  %[1]s -c "test-command" --success-exit-codes "0" --error-exit-codes "1,2" files/

  # Read filenames from stdin with progress display
  find . -name "*.go" | %[1]s -c "gofmt -l" -p

  # Use $FILE placeholder in command for more control
  %[1]s -c "diff $FILE expected.txt" test_files/

MODES:
  - Normal mode: Run command on all given files
  - Audit mode (-a): Only run command on changed/unknown files (requires -f)
  - Update mode (-u): Write successful file hashes to .new file and merge into hashes file

NOTES:
  - Files can be specified as arguments or read from stdin
  - Output is to stdout in JSONL format with filename, hash, exit_code, and audit info
  - Quiet mode (-q) with hashes file (-f) suppresses stdout output
  - Use $FILE in command to specify exact placement of filename
  - Update mode creates a .new file with successful hashes and merges into existing file
  - Only one hash per filename is maintained (new hashes overwrite existing ones)
  - Exit code filtering: specify --success-exit-codes and/or --error-exit-codes to filter results

`, os.Args[0])
}

func parseFlags() Config {
	var cfg Config
	var successCodeStr, errorCodeStr string
	var showHelp bool

	flag.StringVar(&cfg.command, "c", "", "Command to run on each file")
	flag.StringVar(&cfg.command, "check-command", "", "Command to run on each file")
	flag.BoolVar(&cfg.audit, "a", false, "Enable audit mode (only run commands on changed/unknown files)")
	flag.BoolVar(&cfg.audit, "audit", false, "Enable audit mode (only run commands on changed/unknown files)")
	flag.StringVar(&cfg.hashesFile, "f", "", "File with known hashes for audit mode (JSONL format)")
	flag.StringVar(&cfg.hashesFile, "hashes-file", "", "File with known hashes for audit mode (JSONL format)")
	flag.BoolVar(&cfg.update, "u", false, "Update hashes file with new successful file hashes")
	flag.BoolVar(&cfg.update, "update", false, "Update hashes file with new successful file hashes")
	flag.StringVar(&successCodeStr, "success-exit-codes", "", "Comma-separated success exit codes to include in output")
	flag.StringVar(&errorCodeStr, "error-exit-codes", "", "Comma-separated error exit codes to include in output")
	flag.IntVar(&cfg.workers, "w", 0, "Number of concurrent workers (default: CPU count)")
	flag.IntVar(&cfg.workers, "workers", 0, "Number of concurrent workers (default: CPU count)")
	flag.BoolVar(&cfg.showProgress, "p", false, "Show progress bar")
	flag.BoolVar(&cfg.showProgress, "progress", false, "Show progress bar")
	flag.BoolVar(&cfg.quiet, "q", false, "Quiet mode (no error output)")
	flag.BoolVar(&cfg.quiet, "quiet", false, "Quiet mode (no error output)")
	flag.BoolVar(&showHelp, "h", false, "Show help message")
	flag.BoolVar(&showHelp, "help", false, "Show help message")

	flag.Usage = showUsage
	flag.Parse()

	if showHelp {
		showUsage()
		os.Exit(0)
	}

	if cfg.command == "" && !cfg.audit {
		fmt.Fprintln(os.Stderr, "Error: Either command (-c) or audit mode (--audit) is required")
		fmt.Fprintln(os.Stderr)
		showUsage()
		os.Exit(1)
	}

	if cfg.audit && cfg.hashesFile == "" {
		fmt.Fprintln(os.Stderr, "Error: Audit mode requires -f (hashes file) to be specified")
		fmt.Fprintln(os.Stderr)
		showUsage()
		os.Exit(1)
	}

	if cfg.update && cfg.hashesFile == "" {
		fmt.Fprintln(os.Stderr, "Error: Update mode requires -f (hashes file) to be specified")
		fmt.Fprintln(os.Stderr)
		showUsage()
		os.Exit(1)
	}

	if cfg.workers <= 0 {
		cfg.workers = runtime.NumCPU()
	}

	cfg.successCodes = parseExitCodes(successCodeStr)
	cfg.errorCodes = parseExitCodes(errorCodeStr)
	cfg.filterOnCodes = len(cfg.successCodes) > 0 || len(cfg.errorCodes) > 0

	return cfg
}

func parseExitCodes(s string) map[int]bool {
	if s == "" {
		return nil
	}

	codes := make(map[int]bool)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		code, err := strconv.Atoi(part)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid exit code '%s'\n", part)
			continue
		}
		codes[code] = true
	}
	return codes
}

func getFiles() []string {
	args := flag.Args()
	if len(args) > 0 {
		return args
	}

	// Read from stdin
	var files []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		file := strings.TrimSpace(scanner.Text())
		if file != "" {
			files = append(files, file)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading filenames from stdin: %v\n", err)
		os.Exit(1)
	}

	return files
}
