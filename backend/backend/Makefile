# Makefile for Sermon Uploader Go Backend
# Enforces TDD by running tests before builds

# Configuration Variables
GO_VERSION := 1.23.0
MODULE_NAME := sermon-uploader
BINARY_NAME := sermon-uploader
COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html
LINT_TIMEOUT := 5m
TEST_TIMEOUT := 10m
DOCKER_IMAGE := sermon-uploader/backend
DOCKER_TAG := latest

# Build Configuration
BUILD_DIR := ./bin
LDFLAGS := -s -w -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
CGO_ENABLED := 0

# Platform-specific configurations
LINUX_ARCH := amd64
DARWIN_ARCH := amd64
PI_ARCH := arm64

# Security Tools
GOSEC_VERSION := latest
NANCY_VERSION := latest

# PHONY declarations for all targets
.PHONY: help install-deps clean
.PHONY: test test-coverage test-race test-integration test-unit benchmark test-watch
.PHONY: lint lint-fix format
.PHONY: build build-linux build-darwin build-pi build-all
.PHONY: docker-build docker-run docker-push docker-clean
.PHONY: dev watch run
.PHONY: security-scan vuln-check security-all
.PHONY: pre-commit setup-hooks
.PHONY: deps-update deps-check deps-tidy
.PHONY: release debug profile version env tools

# Default target
.DEFAULT_GOAL := help

## Help: Display this help message
help:
	@echo "Sermon Uploader Backend - Makefile Commands"
	@echo ""
	@echo "Development Workflow:"
	@echo "  make dev           - Start development server with hot reload"
	@echo "  make test          - Run all tests (enforces TDD)"
	@echo "  make build         - Build binary (tests run first)"
	@echo "  make lint          - Run linting and formatting checks"
	@echo ""
	@echo "Test Targets:"
	@echo "  make test              - Run all tests"
	@echo "  make test-unit         - Run unit tests only"
	@echo "  make test-integration  - Run integration tests"
	@echo "  make test-coverage     - Run tests with coverage report"
	@echo "  make test-race         - Run tests with race detection"
	@echo "  make test-watch        - Watch files and run tests on change"
	@echo "  make benchmark         - Run benchmark tests"
	@echo ""
	@echo "Lint Targets:"
	@echo "  make lint              - Run all linting checks"
	@echo "  make lint-fix          - Auto-fix linting issues"
	@echo "  make format            - Format code (gofmt + goimports)"
	@echo ""
	@echo "Build Targets:"
	@echo "  make build             - Build for current platform"
	@echo "  make build-linux       - Build for Linux amd64"
	@echo "  make build-darwin      - Build for macOS"
	@echo "  make build-pi          - Build for Raspberry Pi (ARM64)"
	@echo "  make build-all         - Build for all platforms"
	@echo ""
	@echo "Docker Targets:"
	@echo "  make docker-build      - Build Docker image"
	@echo "  make docker-run        - Run Docker container"
	@echo "  make docker-clean      - Clean Docker images"
	@echo ""
	@echo "Security Targets:"
	@echo "  make security-scan     - Run security scans"
	@echo "  make vuln-check        - Check for vulnerabilities"
	@echo "  make security-all      - Run all security checks"
	@echo ""
	@echo "Utility Targets:"
	@echo "  make clean             - Clean build artifacts"
	@echo "  make install-deps      - Install required tools"
	@echo "  make setup-hooks       - Setup git hooks"
	@echo "  make deps-update       - Update dependencies"
	@echo ""

## Install Dependencies: Install required development tools
install-deps:
	@echo ">>> Installing development dependencies..."
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.55.2; \
	}
	@command -v gosec >/dev/null 2>&1 || { \
		echo "Installing gosec..."; \
		go install github.com/securecodewarrior/gosec/v2/cmd/gosec@$(GOSEC_VERSION); \
	}
	@command -v nancy >/dev/null 2>&1 || { \
		echo "Installing nancy..."; \
		go install github.com/sonatypecommunity/nancy@$(NANCY_VERSION); \
	}
	@command -v goimports >/dev/null 2>&1 || { \
		echo "Installing goimports..."; \
		go install golang.org/x/tools/cmd/goimports@latest; \
	}
	@command -v air >/dev/null 2>&1 || { \
		echo "Installing air for hot reload..."; \
		go install github.com/cosmtrek/air@latest; \
	}
	@echo "✓ All development dependencies installed"

## Clean: Remove build artifacts and temporary files
clean:
	@echo ">>> Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)
	@rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	@rm -f *.test
	@rm -rf .tmp/
	@go clean -cache -testcache -modcache
	@echo "✓ Clean completed"

## Dependencies: Update and tidy dependencies
deps-update:
	@echo ">>> Updating dependencies..."
	@go get -u ./...
	@go mod tidy
	@go mod verify
	@echo "✓ Dependencies updated"

deps-check:
	@echo ">>> Checking dependencies..."
	@go mod verify
	@go list -m -u all
	@echo "✓ Dependencies checked"

deps-tidy:
	@echo ">>> Tidying dependencies..."
	@go mod tidy
	@go mod verify
	@echo "✓ Dependencies tidied"

# ==============================================================================
# TEST TARGETS (Core of TDD workflow)
# ==============================================================================

## Test: Run all tests (enforces TDD)
test:
	@echo ">>> Running all tests (TDD enforcement)..."
	@go test -v -timeout $(TEST_TIMEOUT) ./...
	@echo "✓ All tests passed ✅"

## Test Unit: Run unit tests only
test-unit:
	@echo ">>> Running unit tests..."
	@go test -v -timeout $(TEST_TIMEOUT) -short ./...
	@echo "✓ Unit tests passed ✅"

## Test Integration: Run integration tests with MinIO container
test-integration:
	@echo ">>> Starting test infrastructure..."
	@docker-compose -f docker-compose.test.yml up -d
	@echo ">>> Waiting for MinIO to be ready..."
	@sleep 5
	@echo ">>> Running integration tests..."
	@RUN_INTEGRATION_TESTS=true TEST_MINIO_ENDPOINT=localhost:9000 go test -v -tags=integration -timeout=$(TEST_TIMEOUT) ./...
	@echo ">>> Stopping test infrastructure..."
	@docker-compose -f docker-compose.test.yml down -v
	@echo "✓ Integration tests passed ✅"

## Test Coverage: Run tests with coverage report
test-coverage:
	@echo ">>> Running tests with coverage analysis..."
	@go test -v -timeout $(TEST_TIMEOUT) -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	@go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@go tool cover -func=$(COVERAGE_FILE) | tail -1
	@echo "✓ Coverage report generated: $(COVERAGE_HTML)"

## Test Race: Run tests with race detection
test-race:
	@echo ">>> Running tests with race detection..."
	@go test -v -timeout $(TEST_TIMEOUT) -race ./...
	@echo "✓ Race tests passed ✅"

## Test Watch: Watch files and run tests on change
test-watch:
	@echo ">>> Starting test watcher..."
	@command -v air >/dev/null 2>&1 || { echo "✗ air not installed. Run: make install-deps"; exit 1; }
	@echo "# Air configuration for test watching" > .air.toml
	@echo 'root = "."' >> .air.toml
	@echo 'testdata_dir = "testdata"' >> .air.toml
	@echo 'tmp_dir = ".tmp"' >> .air.toml
	@echo '[build]' >> .air.toml
	@echo '  cmd = "go test -v ./..."' >> .air.toml
	@echo '  bin = ".tmp/test"' >> .air.toml
	@echo '  full_bin = "echo \"Tests completed\""' >> .air.toml
	@echo '  include_ext = ["go"]' >> .air.toml
	@echo '  exclude_dir = [".tmp", "vendor", ".git"]' >> .air.toml
	@air -c .air.toml
	@rm -f .air.toml

## Benchmark: Run benchmark tests
benchmark:
	@echo ">>> Running benchmark tests..."
	@go test -v -timeout $(TEST_TIMEOUT) -bench=. -benchmem ./...
	@echo "✓ Benchmarks completed ✅"

# ==============================================================================
# LINT TARGETS
# ==============================================================================

## Lint: Run all linting checks
lint:
	@echo ">>> Running linting checks..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "✗ golangci-lint not installed. Run: make install-deps"; exit 1; }
	@golangci-lint run --timeout $(LINT_TIMEOUT) ./...
	@go vet ./...
	@go fmt -n ./... | (! grep .) || { echo "✗ Code not formatted. Run: make format"; exit 1; }
	@echo "✓ Linting passed ✅"

## Lint Fix: Auto-fix linting issues
lint-fix:
	@echo ">>> Auto-fixing linting issues..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "✗ golangci-lint not installed. Run: make install-deps"; exit 1; }
	@golangci-lint run --fix --timeout $(LINT_TIMEOUT) ./...
	@$(MAKE) format
	@echo "✓ Linting issues fixed ✅"

## Format: Format code using gofmt and goimports
format:
	@echo ">>> Formatting code..."
	@go fmt ./...
	@command -v goimports >/dev/null 2>&1 && goimports -w . || echo "goimports not available, using go fmt only"
	@echo "✓ Code formatted ✅"

# ==============================================================================
# BUILD TARGETS (TDD enforced - tests run first)
# ==============================================================================

## Build: Build binary for current platform (tests run first)
build: test
	@echo ">>> Building $(BINARY_NAME) for current platform..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=$(CGO_ENABLED) go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "✓ Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

## Build Linux: Build for Linux amd64 (tests run first)
build-linux: test
	@echo ">>> Building $(BINARY_NAME) for Linux amd64..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=$(LINUX_ARCH) go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-$(LINUX_ARCH) .
	@echo "✓ Linux build completed: $(BUILD_DIR)/$(BINARY_NAME)-linux-$(LINUX_ARCH)"

## Build Darwin: Build for macOS (tests run first)
build-darwin: test
	@echo ">>> Building $(BINARY_NAME) for macOS..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=$(DARWIN_ARCH) go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-$(DARWIN_ARCH) .
	@echo "✓ macOS build completed: $(BUILD_DIR)/$(BINARY_NAME)-darwin-$(DARWIN_ARCH)"

## Build Pi: Build for Raspberry Pi ARM64 (tests run first)
build-pi: test
	@echo ">>> Building $(BINARY_NAME) for Raspberry Pi ARM64..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=$(PI_ARCH) go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-pi-$(PI_ARCH) .
	@echo "✓ Raspberry Pi build completed: $(BUILD_DIR)/$(BINARY_NAME)-pi-$(PI_ARCH)"

## Build All: Build for all supported platforms (tests run first)
build-all: build-linux build-darwin build-pi
	@echo "✓ All platform builds completed ✅"
	@ls -la $(BUILD_DIR)/

# ==============================================================================
# DOCKER TARGETS
# ==============================================================================

## Docker Build: Build Docker image (tests run first)
docker-build: test
	@echo ">>> Building Docker image: $(DOCKER_IMAGE):$(DOCKER_TAG)"
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "✓ Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

## Docker Run: Run Docker container
docker-run:
	@echo ">>> Running Docker container..."
	@docker run --rm -p 8000:8000 --env-file .env $(DOCKER_IMAGE):$(DOCKER_TAG)

## Docker Push: Push Docker image to registry
docker-push: docker-build
	@echo ">>> Pushing Docker image to registry..."
	@docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	@echo "✓ Docker image pushed ✅"

## Docker Clean: Clean Docker images and containers
docker-clean:
	@echo ">>> Cleaning Docker artifacts..."
	@docker system prune -f
	@docker image prune -f
	@echo "✓ Docker cleanup completed ✅"

# ==============================================================================
# DEVELOPMENT TARGETS
# ==============================================================================

## Dev: Start development server with hot reload
dev: install-deps
	@echo ">>> Starting development server with hot reload..."
	@command -v air >/dev/null 2>&1 || { echo "✗ air not installed. Run: make install-deps"; exit 1; }
	@echo "# Air configuration for development" > .air.toml
	@echo 'root = "."' >> .air.toml
	@echo 'testdata_dir = "testdata"' >> .air.toml
	@echo 'tmp_dir = ".tmp"' >> .air.toml
	@echo '[build]' >> .air.toml
	@echo '  cmd = "go build -o .tmp/main ."' >> .air.toml
	@echo '  bin = ".tmp/main"' >> .air.toml
	@echo '  full_bin = "APP_ENV=development .tmp/main"' >> .air.toml
	@echo '  include_ext = ["go", "tpl", "tmpl", "html"]' >> .air.toml
	@echo '  exclude_dir = [".tmp", "vendor", ".git", "frontend"]' >> .air.toml
	@echo '  include_dir = []' >> .air.toml
	@echo '  exclude_file = []' >> .air.toml
	@echo '  delay = 1000' >> .air.toml
	@echo '  stop_on_root = false' >> .air.toml
	@echo '  send_interrupt = false' >> .air.toml
	@echo '  kill_delay = "0s"' >> .air.toml
	@air -c .air.toml
	@rm -f .air.toml

## Watch: Watch files and rebuild on change
watch: dev

## Run: Run the application
run: build
	@echo ">>> Running $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME)

## Debug: Run with debug mode
debug:
	@echo ">>> Running in debug mode..."
	@ENV=development go run -race .

## Profile: Run with profiling enabled
profile:
	@echo ">>> Running with profiling enabled..."
	@ENV=development go run . &
	@sleep 2
	@echo "Profiling available at: http://localhost:8000/debug/pprof/"
	@wait

# ==============================================================================
# SECURITY TARGETS
# ==============================================================================

## Security Scan: Run security scans
security-scan:
	@echo ">>> Running security scans..."
	@command -v gosec >/dev/null 2>&1 || { echo "✗ gosec not installed. Run: make install-deps"; exit 1; }
	@gosec -quiet ./...
	@echo "✓ Security scan completed ✅"

## Vulnerability Check: Check for vulnerabilities
vuln-check:
	@echo ">>> Checking for vulnerabilities..."
	@go list -json -m all | nancy sleuth
	@echo "✓ Vulnerability check completed ✅"

## Security All: Run all security checks
security-all: security-scan vuln-check
	@echo "✓ All security checks completed ✅"

# ==============================================================================
# PRE-COMMIT HOOKS AND AUTOMATION
# ==============================================================================

## Pre-commit: Run pre-commit checks (TDD workflow)
pre-commit: format lint test security-scan
	@echo "✓ Pre-commit checks passed ✅"

## Setup Hooks: Setup git hooks for TDD
setup-hooks:
	@echo ">>> Setting up git hooks..."
	@mkdir -p .git/hooks
	@echo '#!/bin/bash' > .git/hooks/pre-commit
	@echo 'set -e' >> .git/hooks/pre-commit
	@echo 'echo "🔍 Running pre-commit checks..."' >> .git/hooks/pre-commit
	@echo 'make pre-commit' >> .git/hooks/pre-commit
	@echo 'echo "✅ Pre-commit checks passed"' >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo '#!/bin/bash' > .git/hooks/pre-push
	@echo 'set -e' >> .git/hooks/pre-push
	@echo 'echo "🚀 Running pre-push checks..."' >> .git/hooks/pre-push
	@echo 'make test-coverage' >> .git/hooks/pre-push
	@echo 'make security-all' >> .git/hooks/pre-push
	@echo 'echo "✅ Pre-push checks passed"' >> .git/hooks/pre-push
	@chmod +x .git/hooks/pre-push
	@echo "✓ Git hooks setup completed ✅"

# ==============================================================================
# RELEASE AND CI/CD TARGETS
# ==============================================================================

## Release: Prepare release (comprehensive checks)
release: clean deps-tidy format lint test-coverage security-all build-all
	@echo ">>> Preparing release..."
	@git status --porcelain | (! grep .) || { echo "✗ Working directory not clean"; exit 1; }
	@echo "Version: $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")"
	@echo "Build artifacts:"
	@ls -la $(BUILD_DIR)/
	@echo "✓ Release preparation completed ✅"

## CI: Run CI pipeline checks
ci: install-deps clean format lint test-coverage security-all build
	@echo "✓ CI pipeline completed ✅"

# ==============================================================================
# UTILITY TARGETS
# ==============================================================================

## Version: Show version information
version:
	@echo "Go version: $(shell go version)"
	@echo "Module: $(MODULE_NAME)"
	@echo "Git commit: $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")"
	@echo "Git tag: $(shell git describe --tags --always 2>/dev/null || echo "dev")"
	@echo "Build time: $(shell date)"

## Env: Show environment information
env:
	@echo "GOPATH: $(shell go env GOPATH)"
	@echo "GOROOT: $(shell go env GOROOT)"
	@echo "GOOS: $(shell go env GOOS)"
	@echo "GOARCH: $(shell go env GOARCH)"
	@echo "CGO_ENABLED: $(CGO_ENABLED)"

## Tools: List installed tools
tools:
	@echo "Checking installed tools..."
	@command -v go >/dev/null 2>&1 && echo "✅ go: $(shell go version)" || echo "❌ go: not found"
	@command -v golangci-lint >/dev/null 2>&1 && echo "✅ golangci-lint: $(shell golangci-lint version 2>/dev/null || echo "installed")" || echo "❌ golangci-lint: not found"
	@command -v gosec >/dev/null 2>&1 && echo "✅ gosec: installed" || echo "❌ gosec: not found"
	@command -v nancy >/dev/null 2>&1 && echo "✅ nancy: installed" || echo "❌ nancy: not found"
	@command -v goimports >/dev/null 2>&1 && echo "✅ goimports: installed" || echo "❌ goimports: not found"
	@command -v air >/dev/null 2>&1 && echo "✅ air: installed" || echo "❌ air: not found"
	@command -v docker >/dev/null 2>&1 && echo "✅ docker: $(shell docker --version 2>/dev/null || echo "installed")" || echo "❌ docker: not found"

# ==============================================================================
# SPECIAL RULES
# ==============================================================================

# Ensure build directory exists
$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

# Auto-install dependencies if not present
.pre-check:
	@command -v go >/dev/null 2>&1 || { echo "✗ Go not installed"; exit 1; }

# Include pre-check in main targets
test lint build: .pre-check