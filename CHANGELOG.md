# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Deprecated

### Removed

### Fixed

### Security

## [1.1.0] - 2025-03-21

### Added

- Exit code filtering with `--success-exit-codes` and `--error-exit-codes` flags
- `$FILE` placeholder support in commands for precise file placement
- Standalone command handling (exit, true, false) without filename appending
- Special handling for command-not-found errors (exit code 127) with helpful guidance

### Changed

- Improved README.md with comprehensive command reference and use cases
- Better error messages for command execution failures
- Refined .gitignore and struct field ordering

### Fixed

- Fixed license reference in README
- Updated copyright year to 2025

## [1.0.2] - 2025-02-20

### Changed

- Updated GitHub Actions workflow to use latest versions

## [1.0.1] - 2025-02-20

### Changed

- Fixed release workflow permissions
- Improved pipeline configuration

## [1.0.0] - 2025-02-20

### Added

- Concurrent file processing with configurable worker count
- SHA256 hash-based file change detection (audit mode)
- JSONL output format for results
- Progress bar display with ETA and rate metrics
- Auto-update mode for hash database management
- Quiet mode for CI/CD integration
- Cross-platform build support (Linux, macOS, Windows)
- Comprehensive test suite with race detector
- golangci-lint integration
- Complete documentation with README and examples

### Features

- `-c, --check-command`: Command to run on each file
- `-a, --audit`: Enable audit mode (only run on changed files)
- `-f, --hashes-file`: File for storing known hashes
- `-u, --update`: Update hashes with successful results
- `-w, --workers`: Number of concurrent workers
- `-p, --progress`: Show progress bar
- `-q, --quiet`: Quiet mode for CI/CD
