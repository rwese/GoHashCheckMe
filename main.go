package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	command      string
	hashesFile   string
	audit        bool
	update       bool
	exitCodes    map[int]bool
	storeOnCodes bool
	workers      int
	showProgress bool
	quiet        bool
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

func showUsage() {
	fmt.Fprintf(os.Stderr, `Usage: %[1]s [OPTIONS] [FILES...]

GoHashCheckMe - Run commands on files and store their hashes based on exit codes.
Avoids expensive operations on unchanged files when using audit mode.

OPTIONS:
  -c, --check-command COMMAND    Command to run on each file
  -a, --audit                   Enable audit mode (only run commands on changed/unknown files)
  -f, --hashes-file FILE        File with known hashes for audit mode (JSONL format)
  -u, --update                  Update hashes file with new successful file hashes
  --store-exit-code CODES       Comma-separated exit codes to filter output
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

  # Filter output to only show files where command returned specific exit codes
  %[1]s -c "test-command" --store-exit-code "0,2" files/

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

`, os.Args[0])
}

func parseFlags() Config {
	var cfg Config
	var exitCodeStr string
	var showHelp bool
	
	flag.StringVar(&cfg.command, "c", "", "Command to run on each file")
	flag.StringVar(&cfg.command, "check-command", "", "Command to run on each file")
	flag.BoolVar(&cfg.audit, "a", false, "Enable audit mode (only run commands on changed/unknown files)")
	flag.BoolVar(&cfg.audit, "audit", false, "Enable audit mode (only run commands on changed/unknown files)")
	flag.StringVar(&cfg.hashesFile, "f", "", "File with known hashes for audit mode (JSONL format)")
	flag.StringVar(&cfg.hashesFile, "hashes-file", "", "File with known hashes for audit mode (JSONL format)")
	flag.BoolVar(&cfg.update, "u", false, "Update hashes file with new successful file hashes")
	flag.BoolVar(&cfg.update, "update", false, "Update hashes file with new successful file hashes")
	flag.StringVar(&exitCodeStr, "store-exit-code", "", "Comma-separated exit codes to filter output")
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
	
	cfg.exitCodes = parseExitCodes(exitCodeStr)
	cfg.storeOnCodes = len(cfg.exitCodes) > 0
	
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

func processFiles(files []string, cfg Config, auditMap map[string]string, output io.Writer) {
	jobs := make(chan string, len(files))
	results := make(chan *Result, len(files))
	
	// Initialize progress reporter
	progress := NewProgressReporter(len(files), cfg.showProgress, cfg.quiet)
	
	// Start workers
	var wg sync.WaitGroup
	for range cfg.workers {
		wg.Add(1)
		go worker(&wg, jobs, results, cfg, auditMap, progress)
	}
	
	// Send jobs
	for _, file := range files {
		jobs <- file
	}
	close(jobs)
	
	// Start result writer
	done := make(chan bool)
	go writeResults(results, output, done, cfg)
	
	// Wait for workers
	wg.Wait()
	close(results)
	
	// Wait for writer
	<-done
	
	// Show final progress
	progress.Finish()
}

func worker(wg *sync.WaitGroup, jobs <-chan string, results chan<- *Result, cfg Config, auditMap map[string]string, progress *ProgressReporter) {
	defer wg.Done()
	
	for filename := range jobs {
		result := processFile(filename, cfg, auditMap)
		
		// Update progress
		changed := result != nil && result.Changed
		errored := result == nil
		progress.Update(changed, errored)
		
		if result != nil {
			results <- result
		}
	}
}

func writeResults(results <-chan *Result, output io.Writer, done chan<- bool, cfg Config) {
	encoder := json.NewEncoder(output)
	
	// Open .new file for successful hashes if update mode is enabled
	var newFile *os.File
	var newEncoder *json.Encoder
	if cfg.update && cfg.hashesFile != "" {
		var err error
		newFile, err = os.Create(cfg.hashesFile + ".new")
		if err != nil {
			if !cfg.quiet {
				logError("Error creating .new file: %v\n", err)
			}
		} else {
			newEncoder = json.NewEncoder(newFile)
		}
	}
	
	for result := range results {
		// Write to main output
		if err := encoder.Encode(result); err != nil {
			if !cfg.quiet {
				logError("Error encoding result: %v\n", err)
			}
		}
		
		// Write successful results to .new file if update mode is enabled
		if newEncoder != nil && result.ExitCode == 0 {
			entry := AuditEntry{Filename: result.Filename, Hash: result.Hash}
			if err := newEncoder.Encode(entry); err != nil {
				if !cfg.quiet {
					logError("Error writing to .new file: %v\n", err)
				}
			}
		}
	}
	
	if newFile != nil {
		newFile.Close()
	}
	
	done <- true
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

var errMutex sync.Mutex

// Buffer pool for file reading - reuse buffers to reduce GC pressure
var bufferPool = sync.Pool{
	New: func() any {
		return make([]byte, 64*1024) // 64KB buffer
	},
}

func logError(format string, args ...any) {
	errMutex.Lock()
	fmt.Fprintf(os.Stderr, format, args...)
	errMutex.Unlock()
}

type ProgressReporter struct {
	total      int
	processed  int32
	errors     int32
	changed    int32
	startTime  time.Time
	showProgress bool
	quiet      bool
	mu         sync.Mutex
}

func NewProgressReporter(total int, showProgress, quiet bool) *ProgressReporter {
	return &ProgressReporter{
		total:        total,
		startTime:    time.Now(),
		showProgress: showProgress,
		quiet:        quiet,
	}
}

func (p *ProgressReporter) Update(changed, errored bool) {
	atomic.AddInt32(&p.processed, 1)
	if changed {
		atomic.AddInt32(&p.changed, 1)
	}
	if errored {
		atomic.AddInt32(&p.errors, 1)
	}
	
	if p.showProgress {
		p.displayProgress()
	}
}

func (p *ProgressReporter) displayProgress() {
	processed := atomic.LoadInt32(&p.processed)
	errors := atomic.LoadInt32(&p.errors)
	changed := atomic.LoadInt32(&p.changed)
	elapsed := time.Since(p.startTime)
	
	// Calculate rate
	rate := float64(processed) / elapsed.Seconds()
	
	// Estimate remaining time
	remaining := time.Duration(0)
	if rate > 0 {
		remainingFiles := float64(p.total - int(processed))
		remaining = time.Duration(remainingFiles/rate) * time.Second
	}
	
	// Clear line and print progress
	percentage := float64(processed) / float64(p.total) * 100
	fmt.Fprintf(os.Stderr, "\r\033[K[%[1]d/%[2]d] %.1[3]f%% | Changed: %[4]d | Errors: %[5]d | Rate: %.1[6]f/s | ETA: %[7]s",
		processed, p.total, percentage, changed, errors, rate, formatDuration(remaining))
}

func (p *ProgressReporter) Finish() {
	if !p.showProgress {
		return
	}
	
	processed := atomic.LoadInt32(&p.processed)
	errors := atomic.LoadInt32(&p.errors)
	changed := atomic.LoadInt32(&p.changed)
	elapsed := time.Since(p.startTime)
	
	fmt.Fprintf(os.Stderr, "\r\033[K")
	filesPerSecond := float64(processed) / elapsed.Seconds()
	fmt.Fprintf(os.Stderr, "Completed: %[1]d files in %[2]s (%.1[3]f files/s)\n", 
		processed, formatDuration(elapsed), filesPerSecond)
	
	if changed > 0 {
		fmt.Fprintf(os.Stderr, "Changed: %d files\n", changed)
	}
	if errors > 0 {
		fmt.Fprintf(os.Stderr, "Errors: %d files\n", errors)
	}
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "0s"
	}
	
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	
	if h > 0 {
		return fmt.Sprintf("%[1]dh%[2]dm%[3]ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%[1]dm%[2]ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
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
		
		if cfg.storeOnCodes && !cfg.exitCodes[result.ExitCode] {
			return nil
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

