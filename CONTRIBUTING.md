# Contributing to GoHashCheckMe

Thank you for your interest in contributing to GoHashCheckMe!

## Development Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/rwese/GoHashCheckMe.git
   cd GoHashCheckMe
   ```

2. Install development dependencies:
   ```bash
   # Using devbox (recommended)
   devbox shell

   # Or manually
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   ```

3. Build and test:
   ```bash
   make dev    # Format, vet, test, and build
   make test   # Run tests only
   make lint   # Run linter
   ```

## Development Workflow

1. **Before making changes**: Run `make test-race` to ensure tests pass
2. **After changes**: Run `make dev` to validate the full pipeline
3. **Before committing**: Ensure all tests pass and lint is clean

## Code Style

- Follow Go's standard formatting (`go fmt`)
- Run `golangci-lint run` to check for issues
- Write tests for new functionality
- Keep functions focused and small

## Testing

```bash
# Run all tests
make test

# Run tests with verbose output
make test-verbose

# Run tests with race detector
make test-race

# Run tests with coverage
make test-coverage
```

## Commit Messages

This project uses conventional commits:

- `feat:` New features
- `fix:` Bug fixes
- `docs:` Documentation changes
- `chore:` Maintenance tasks
- `refactor:` Code refactoring
- `test:` Test updates

Example:
```
feat: add support for custom exit code filtering
```

## Pull Requests

1. Create a feature branch: `git checkout -b feature/my-feature`
2. Make your changes and add tests
3. Ensure tests pass: `make dev`
4. Commit your changes
5. Push and create a PR

## Project Structure

```
GoHashCheckMe/
├── main.go          # Entry point and main logic
├── args.go         # Command-line argument parsing
├── hash_check.go   # File hashing and command execution
├── audit.go        # Hash audit/change detection
├── workers.go      # Concurrent worker management
├── progress.go     # Progress bar display
├── Makefile        # Build and test automation
└── *.go            # Test files (*_test.go)
```

## Getting Help

- Open an issue for bugs or feature requests
- Check the [README.md](README.md) for usage examples

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
