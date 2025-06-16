# GoHashCheckMe

A fast, concurrent tool that runs commands on files and tracks their hashes to avoid expensive operations on unchanged files.

## What it does

GoHashCheckMe helps you optimize CI/CD pipelines and development workflows by:
- Running commands only on files that have actually changed
- Tracking file hashes to detect changes efficiently  
- Supporting concurrent processing for better performance
- Providing audit trails of what was processed and when
- Filtering results by success/error exit codes

## Quick Start

```bash
# Build the tool
make build

# Run a linter on all Go files
./build/ghc -c "golint" *.go

# Only run checks on changed files (audit mode)
./build/ghc -a -f hashes.jsonl -c "mycheck" *.txt

# Update hash tracking with successful results
./build/ghc -u -f hashes.jsonl -c "test" src/*.js

# Combine audit and update for efficient workflows
./build/ghc -a -u -f hashes.jsonl -c "mycheck" *.go

# Filter results by exit codes
./build/ghc -c "lint" --success-exit-codes "0" --error-exit-codes "1,2" *.go
```

## Key Features

- **🚀 Fast**: Concurrent processing with configurable worker count
- **📝 Hash Tracking**: SHA256 hashing to detect file changes
- **🎯 Audit Mode**: Only process files that have changed since last run
- **📊 Progress Display**: Real-time progress reporting with ETA
- **🔄 Auto-Update**: Update hash database with successful results
- **🤐 Quiet Mode**: Silent operation for CI/CD integration
- **📋 JSONL Output**: Machine-readable results
- **🎛️ Exit Code Filtering**: Include only specific success/error codes
- **💡 Smart Error Handling**: Helpful messages for unexpected command failures

## Use Cases

### CI/CD Optimization
```bash
# Only run expensive tests on changed files
./build/ghc -a -u -q -f .hashes -c "npm test" src/**/*.js

# Run linter and only report actual errors (not warnings)
./build/ghc -c "eslint" --error-exit-codes "1,2" --success-exit-codes "0" src/*.js
```

### Development Workflow
```bash
# Run linter with progress display
find . -name "*.go" | ./build/ghc -c "gofmt -l" -p

# Run formatter and track what gets changed
./build/ghc -c "prettier --write" --success-exit-codes "0" src/*.ts
```

### Batch Processing
```bash
# Process files and include both successful validations and specific error types
./build/ghc -c "validate" --success-exit-codes "0" --error-exit-codes "2,127" data/*.json

# Handle command-not-found errors gracefully
./build/ghc -c "optional-tool" --error-exit-codes "127" --success-exit-codes "0" files/*
```

### File Monitoring
```bash
# Monitor file changes and run commands only on modified files
./build/ghc -a -f monitor.jsonl -c "process-file" --success-exit-codes "0" watch_dir/*
```

## Installation

```bash
git clone <repository>
cd GoHashCheckMe
make build
```

## Command Reference

```
Usage: ./build/ghc [OPTIONS] [FILES...]

GoHashCheckMe - Run commands on files and store their hashes based on exit codes.
Avoids expensive operations on unchanged files when using audit mode.

OPTIONS:
  -c, --check-command COMMAND      Command to run on each file
  -a, --audit                     Enable audit mode (only run commands on changed/unknown files)
  -f, --hashes-file FILE          File with known hashes for audit mode (JSONL format)
  -u, --update                    Update hashes file with new successful file hashes
  --success-exit-codes CODES      Comma-separated success exit codes to include in output
  --error-exit-codes CODES        Comma-separated error exit codes to include in output
  -w, --workers N                 Number of concurrent workers (default: CPU count)
  -p, --progress                  Show progress bar
  -q, --quiet                     Quiet mode (no error output, suppresses stdout if -f given)
  -h, --help                      Show this help message

EXAMPLES:
  # Run linter on all Go files and store results
  ./build/ghc -c "golint" *.go

  # Enable audit mode with hashes file, only run command on changed/unknown files
  ./build/ghc -a -f audit.jsonl -c "mycheck" *.txt

  # Update hashes file with successful results from new files
  ./build/ghc -u -f hashes.jsonl -c "mycheck" *.txt

  # Combine audit and update: only check changed files and update successful ones
  ./build/ghc -a -u -f hashes.jsonl -c "mycheck" *.txt

  # Filter output to only show files where command returned success exit codes
  ./build/ghc -c "test-command" --success-exit-codes "0" --error-exit-codes "1,2" files/

  # Read filenames from stdin with progress display
  find . -name "*.go" | ./build/ghc -c "gofmt -l" -p

  # Use $FILE placeholder in command for more control
  ./build/ghc -c "diff $FILE expected.txt" test_files/

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
```

## Exit Code Filtering

The tool supports sophisticated exit code filtering to help you focus on relevant results:

### Success Exit Codes (`--success-exit-codes`)
Include only results where the command succeeded with specific exit codes:
```bash
# Only include successful operations
./build/ghc -c "lint" --success-exit-codes "0" *.js

# Include multiple success scenarios
./build/ghc -c "compile" --success-exit-codes "0,2" src/*.c
```

### Error Exit Codes (`--error-exit-codes`)  
Include only results where the command failed with specific exit codes:
```bash
# Only include specific error types
./build/ghc -c "validate" --error-exit-codes "1,2" data/*.json

# Include command-not-found errors
./build/ghc -c "optional-tool" --error-exit-codes "127" files/*
```

### Combined Filtering
Use both flags to include specific success and error scenarios:
```bash
# Include successes and specific errors, filter out warnings
./build/ghc -c "checker" --success-exit-codes "0" --error-exit-codes "1" --quiet files/*
```

### Special Error Handling
When a command fails to execute (exit code -1), the tool provides helpful guidance:
```
Command failed to run with exit code -1 for file.txt. If expected, add -1 to the error exit codes with --error-exit-codes
```

## Output Format

Results are output in JSONL format:
```json
{"filename":"src/main.go","hash":"abc123...","exit_code":0,"audited":true,"changed":false}
{"filename":"src/util.go","hash":"def456...","exit_code":1,"audited":true,"changed":true}
```

Fields:
- `filename`: Path to the processed file
- `hash`: SHA256 hash of the file content
- `exit_code`: Exit code returned by the command
- `audited`: Whether this file was checked against audit history (if using -f)
- `changed`: Whether the file changed since last audit (only present if audited)

## Performance Tips

1. **Use Audit Mode**: Skip processing unchanged files with `-a -f hashes.jsonl`
2. **Tune Workers**: Adjust `-w N` based on your CPU and I/O characteristics
3. **Filter Early**: Use exit code filtering to reduce output processing
4. **Quiet Mode**: Use `-q` in CI/CD to reduce noise and improve performance
5. **Batch Updates**: Use `-u` to efficiently update hash databases

## Development

```bash
# Run tests
make test

# Run with coverage
make test-coverage

# Run linter (if available)
make lint

# Clean build artifacts
make clean
```

## License

[Add your license here]