# GoHashCheckMe

A fast, concurrent tool that runs commands on files and tracks their hashes to avoid expensive operations on unchanged files.

## What it does

GoHashCheckMe helps you optimize CI/CD pipelines and development workflows by:
- Running commands only on files that have actually changed
- Tracking file hashes to detect changes efficiently  
- Supporting concurrent processing for better performance
- Providing audit trails of what was processed and when

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
```

## Key Features

- **🚀 Fast**: Concurrent processing with configurable worker count
- **📝 Hash Tracking**: SHA256 hashing to detect file changes
- **🎯 Audit Mode**: Only process files that have changed since last run
- **📊 Progress Display**: Real-time progress reporting
- **🔄 Auto-Update**: Update hash database with successful results
- **🤐 Quiet Mode**: Silent operation for CI/CD integration
- **📋 JSONL Output**: Machine-readable results

## Use Cases

### CI/CD Optimization
```bash
# Only run expensive tests on changed files
./tool -a -u -q -f .hashes -c "npm test" src/**/*.js
```

### Development Workflow
```bash
# Run linter with progress display
find . -name "*.go" | ./tool -c "gofmt -l" -p
```

### Batch Processing
```bash
# Process files with specific exit code filtering
./tool -c "validate" --store-exit-code "0,2" data/*.json
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

OPTIONS:
  -c, --check-command COMMAND    Command to run on each file
  -a, --audit                   Enable audit mode (only run commands on changed files)
  -f, --hashes-file FILE        File with known hashes (JSONL format)
  -u, --update                  Update hashes file with successful results
  -q, --quiet                   Quiet mode (suppresses stdout if -f given)
  -p, --progress                Show progress bar
  -w, --workers N               Number of concurrent workers
  --store-exit-code CODES       Filter output by exit codes
```

## Output Format

Results are output in JSONL format:
```json
{"filename":"src/main.go","hash":"abc123...","exit_code":0,"audited":true,"changed":false}
```

## License

[Add your license here]
