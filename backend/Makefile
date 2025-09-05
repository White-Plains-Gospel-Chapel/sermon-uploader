# Makefile for Sermon Uploader Backend
# Provides convenient commands for testing, building, and deployment

.PHONY: help test test-fast test-integration test-performance test-health benchmark clean build run docker-build docker-run deps lint

# Default target
help: ## Show this help message
	@echo "Sermon Uploader Backend - Available Commands"
	@echo "==========================================="
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Test targets
test: ## Run all tests (unit + integration)
	@echo "Running all tests..."
	go test -v ./...
	@if command -v curl >/dev/null && curl -f -s --max-time 2 "http://localhost:9000/minio/health/live" >/dev/null 2>&1; then \
		echo "MinIO available, running integration tests..."; \
		./scripts/run-integration-tests.sh --integration; \
	else \
		echo "MinIO not available, skipping integration tests"; \
	fi

test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	go test -v -short ./...

test-fast: ## Run fast integration tests (< 30s)
	@echo "Running fast integration tests..."
	./scripts/run-integration-tests.sh --fast

test-integration: ## Run full integration test suite
	@echo "Running full integration test suite..."
	./scripts/run-integration-tests.sh --integration

test-performance: ## Run performance tests and benchmarks
	@echo "Running performance tests..."
	./scripts/run-integration-tests.sh --performance

test-health: ## Run health check tests
	@echo "Running health check tests..."
	./scripts/run-integration-tests.sh --health

test-ci: ## Run tests in CI mode with containers
	@echo "Running tests in CI mode..."
	./scripts/run-integration-tests.sh --ci

benchmark: ## Run performance benchmarks
	@echo "Running benchmarks..."
	go test -tags=integration -bench=. -benchmem -timeout=1800s ./...

# Pre-commit hooks
pre-commit: test-fast lint ## Run pre-commit checks (fast tests + linting)
	@echo "Pre-commit checks completed successfully!"

# Build targets
build: ## Build the application
	@echo "Building sermon-uploader..."
	go build -o sermon-uploader ./...

build-pi: ## Build for Raspberry Pi (ARM64)
	@echo "Building for Raspberry Pi (ARM64)..."
	GOOS=linux GOARCH=arm64 go build -o sermon-uploader-pi ./...

# Run targets  
run: ## Run the application locally
	@echo "Starting sermon-uploader..."
	go run ./...

run-dev: ## Run with development settings
	@echo "Starting sermon-uploader in development mode..."
	ENV=development go run ./...

# Docker targets
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t sermon-uploader:latest .

docker-run: ## Run in Docker container
	@echo "Running Docker container..."
	docker run -p 8000:8000 --env-file .env sermon-uploader:latest

docker-test: ## Run tests in Docker container
	@echo "Running tests in Docker..."
	docker build -f Dockerfile.test -t sermon-uploader:test .
	docker run --rm -v $(PWD):/app -w /app sermon-uploader:test

# Development targets
deps: ## Download and verify dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod verify
	go mod tidy

deps-update: ## Update all dependencies
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

lint: ## Run linters (if available)
	@if command -v golangci-lint >/dev/null; then \
		echo "Running golangci-lint..."; \
		golangci-lint run; \
	elif command -v staticcheck >/dev/null; then \
		echo "Running staticcheck..."; \
		staticcheck ./...; \
	else \
		echo "Running go vet..."; \
		go vet ./...; \
		echo "Running go fmt check..."; \
		test -z "$$(go fmt ./...)"; \
	fi

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...

# Cleanup targets
clean: ## Clean build artifacts and test files
	@echo "Cleaning up..."
	go clean ./...
	rm -f sermon-uploader sermon-uploader-pi
	rm -rf /tmp/sermon-test /tmp/sermon-uploads
	docker system prune -f --volumes --filter label=com.sermon-uploader.test=true || true

clean-cache: ## Clean Go module cache
	@echo "Cleaning Go module cache..."
	go clean -modcache

# File generation targets
generate-test-files: ## Generate test WAV files
	@echo "Generating test files..."
	cd ../test-utils && go run wav-generator.go -testsuite -output ../backend/test-files

# Monitoring and analysis
profile-cpu: ## Run CPU profiling during tests
	@echo "Running CPU profiling..."
	go test -tags=integration -cpuprofile cpu.prof -run=TestPerformance ./...
	@echo "View profile with: go tool pprof cpu.prof"

profile-mem: ## Run memory profiling during tests
	@echo "Running memory profiling..."
	go test -tags=integration -memprofile mem.prof -run=TestPerformance ./...
	@echo "View profile with: go tool pprof mem.prof"

trace: ## Generate execution trace during tests
	@echo "Generating execution trace..."
	go test -tags=integration -trace trace.out -run=TestPerformance ./...
	@echo "View trace with: go tool trace trace.out"

# Coverage targets
coverage: ## Generate test coverage report
	@echo "Generating coverage report..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

coverage-integration: ## Generate integration test coverage
	@echo "Generating integration test coverage..."
	go test -tags=integration -coverprofile=coverage-integration.out ./...
	go tool cover -html=coverage-integration.out -o coverage-integration.html
	@echo "Integration coverage report generated: coverage-integration.html"

# Security targets
security: ## Run security checks (if gosec is available)
	@if command -v gosec >/dev/null; then \
		echo "Running security checks..."; \
		gosec ./...; \
	else \
		echo "gosec not available, skipping security checks"; \
		echo "Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi

# Pi-specific targets
pi-deploy: build-pi ## Build and prepare for Pi deployment
	@echo "Preparing Pi deployment..."
	mkdir -p dist/pi
	cp sermon-uploader-pi dist/pi/
	cp .env.example dist/pi/.env
	cp -r scripts dist/pi/
	@echo "Pi deployment ready in dist/pi/"

# Environment setup
setup-dev: ## Setup development environment
	@echo "Setting up development environment..."
	cp .env.example .env.test
	mkdir -p test-files logs
	@echo "Development environment setup complete!"
	@echo "Next steps:"
	@echo "1. Start MinIO: docker run -p 9000:9000 -p 9001:9001 minio/minio server /data --console-address ':9001'"
	@echo "2. Run tests: make test-fast"

# Documentation targets
docs: ## Generate documentation
	@echo "Generating documentation..."
	@if command -v godoc >/dev/null; then \
		echo "Starting godoc server on :6060"; \
		godoc -http=:6060; \
	else \
		echo "godoc not available, install with: go install golang.org/x/tools/cmd/godoc@latest"; \
	fi

# Database/Migration targets (for future use)
migrate: ## Run database migrations (placeholder)
	@echo "No migrations defined yet"

# Deployment targets
deploy-staging: ## Deploy to staging environment
	@echo "Deploying to staging..."
	@echo "Not implemented yet"

deploy-prod: ## Deploy to production environment
	@echo "Deploying to production..."
	@echo "Not implemented yet"

# Health check targets
health-check: ## Check system health
	@echo "Running health checks..."
	@if curl -f -s "http://localhost:8000/health" >/dev/null 2>&1; then \
		echo "✅ Application is healthy"; \
		curl -s "http://localhost:8000/health" | jq . || curl -s "http://localhost:8000/health"; \
	else \
		echo "❌ Application is not responding"; \
		exit 1; \
	fi

# Utility targets
version: ## Show version information
	@echo "Go version: $$(go version)"
	@echo "Module: $$(grep '^module' go.mod)"
	@echo "Git commit: $$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
	@echo "Build time: $$(date)"

# List all make targets
targets: ## List all available targets
	@$(MAKE) -pRrq -f $(firstword $(MAKEFILE_LIST)) : 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | sort | egrep -v -e '^[^[:alnum:]]' -e '^$@$$'