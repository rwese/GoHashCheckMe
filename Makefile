# Binary name
BINARY := ghc
VERSION := 1.0.0

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOCLEAN := $(GOCMD) clean
GOGET := $(GOCMD) get

# Build flags
CGO_ENABLED := 0
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION)"
BUILD_FLAGS := -trimpath

# Directories
BUILD_DIR := build
DIST_DIR := dist

# Platforms
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build test clean run deps lint fmt vet test-verbose test-race test-coverage bench help

# Default target
all: deps lint fmt vet test test-race test-coverage build

# Build for current platform
build:
	@echo "Building $(BINARY)..."
	@CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(BUILD_FLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) .
	@echo "Build complete: $(BUILD_DIR)/$(BINARY)"

# Build for all platforms
build-all: clean
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} \
		CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(BUILD_FLAGS) $(LDFLAGS) \
		-o $(DIST_DIR)/$(BINARY)-$${platform%/*}-$${platform#*/}$$([ "$${platform%/*}" = "windows" ] && echo ".exe") .; \
		echo "Built: $(DIST_DIR)/$(BINARY)-$${platform%/*}-$${platform#*/}"; \
	done

# Run tests
test:
	@echo "Running tests..."
	@CGO_ENABLED=$(CGO_ENABLED) $(GOTEST) -short ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	@CGO_ENABLED=$(CGO_ENABLED) $(GOTEST) -v ./...

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	@CGO_ENABLED=1 $(GOTEST) -race -short ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@CGO_ENABLED=$(CGO_ENABLED) $(GOTEST) -cover -coverprofile=coverage.out ./...
	@$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	@CGO_ENABLED=$(CGO_ENABLED) $(GOTEST) -bench=. -benchmem ./...

# Run specific benchmark
bench-hash:
	@echo "Running hash benchmarks..."
	@CGO_ENABLED=$(CGO_ENABLED) $(GOTEST) -bench=BenchmarkHashFile -benchmem -benchtime=10s ./...

# Compare benchmarks (requires benchstat)
bench-compare: clean
	@echo "Running baseline benchmarks..."
	@git stash
	@CGO_ENABLED=$(CGO_ENABLED) $(GOTEST) -bench=. -benchmem -count=10 ./... > old.bench
	@git stash pop
	@echo "Running current benchmarks..."
	@CGO_ENABLED=$(CGO_ENABLED) $(GOTEST) -bench=. -benchmem -count=10 ./... > new.bench
	@if command -v benchstat >/dev/null 2>&1; then \
		benchstat old.bench new.bench; \
	else \
		echo "benchstat not installed. Install with: go install golang.org/x/perf/cmd/benchstat@latest"; \
	fi

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@$(GOCLEAN)
	@rm -rf $(BUILD_DIR) $(DIST_DIR) coverage.out coverage.html

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@$(GOMOD) download
	@$(GOMOD) tidy

# Format code
fmt:
	@echo "Formatting code..."
	@$(GOCMD) fmt ./...

# Run go vet
vet:
	@echo "Running go vet..."
	@$(GOCMD) vet ./...

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with:"; \
		echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Quick development build
dev: fmt vet test build

# Run the binary with example
run: build
	@echo "Running example..."
	@$(BUILD_DIR)/$(BINARY) -c "wc -l" Makefile

# Install binary to GOPATH/bin
install: build
	@echo "Installing $(BINARY)..."
	@cp $(BUILD_DIR)/$(BINARY) $(GOPATH)/bin/
	@echo "Installed to $(GOPATH)/bin/$(BINARY)"

# Create release archives
release: build-all
	@echo "Creating release archives..."
	@mkdir -p $(DIST_DIR)/archives
	@for platform in $(PLATFORMS); do \
		cd $(DIST_DIR) && \
		if [ "$${platform%/*}" = "windows" ]; then \
			zip archives/$(BINARY)-$(VERSION)-$${platform%/*}-$${platform#*/}.zip \
				$(BINARY)-$${platform%/*}-$${platform#*/}.exe; \
		else \
			tar czf archives/$(BINARY)-$(VERSION)-$${platform%/*}-$${platform#*/}.tar.gz \
				$(BINARY)-$${platform%/*}-$${platform#*/}; \
		fi && \
		cd ..; \
	done
	@echo "Release archives created in $(DIST_DIR)/archives/"

# Show help
help:
	@echo "Available targets:"
	@echo "  make              - Run tests and build"
	@echo "  make build        - Build binary for current platform"
	@echo "  make build-all    - Build for all platforms"
	@echo "  make test         - Run tests"
	@echo "  make test-verbose - Run tests with verbose output"
	@echo "  make test-race    - Run tests with race detector"
	@echo "  make test-coverage- Run tests with coverage report"
	@echo "  make bench        - Run benchmarks"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make deps         - Install/update dependencies"
	@echo "  make fmt          - Format code"
	@echo "  make vet          - Run go vet"
	@echo "  make lint         - Run linter (requires golangci-lint)"
	@echo "  make dev          - Format, vet, test, and build"
	@echo "  make run          - Build and run example"
	@echo "  make install      - Install binary to GOPATH/bin"
	@echo "  make release      - Create release archives for all platforms"
	@echo ""
	@echo "Build variables:"
	@echo "  CGO_ENABLED=$(CGO_ENABLED)"
	@echo "  VERSION=$(VERSION)"
