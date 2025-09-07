# TDD Workflow Documentation

Comprehensive Test-Driven Development (TDD) workflow for the sermon uploader project, covering Go backend and React/TypeScript frontend with practical examples from the codebase.

## Table of Contents

1. [TDD Fundamentals](#tdd-fundamentals)
2. [Red-Green-Refactor Cycle](#red-green-refactor-cycle)
3. [100% Test Coverage Strategy](#100-test-coverage-strategy)
4. [Go Backend TDD Patterns](#go-backend-tdd-patterns)
5. [React/TypeScript Frontend TDD](#reacttypescript-frontend-tdd)
6. [CI/CD Integration](#cicd-integration)
7. [Developer Onboarding Guide](#developer-onboarding-guide)
8. [Troubleshooting Common Issues](#troubleshooting-common-issues)
9. [Testing Tools and Configuration](#testing-tools-and-configuration)
10. [Real Codebase Examples](#real-codebase-examples)

## TDD Fundamentals

### The Three Laws of TDD

1. **You must write a failing unit test before writing any production code**
2. **You must not write more of a unit test than is sufficient to fail**
3. **You must not write more production code than is sufficient to pass the currently failing test**

### Why TDD for This Project

The sermon uploader system benefits from TDD because:

- **File Upload Reliability**: Critical that uploads work flawlessly
- **Data Integrity**: Audio file processing must be bulletproof
- **Integration Complexity**: MinIO, Discord, and audio processing integration
- **Performance Requirements**: Raspberry Pi constraints require optimized code
- **Security**: File handling and validation must be secure

## Red-Green-Refactor Cycle

### The Cycle Explained

```
🔴 RED → 🟢 GREEN → 🔵 REFACTOR
  ↑                      ↓
  ←←←←←←←←←←←←←←←←←←←←←←←←←←
```

### 1. RED Phase: Write a Failing Test

**Example: Testing MinIO Service Connection**

```go
// services/minio_test.go
func TestMinIOService_Connect_FailsWithInvalidCredentials(t *testing.T) {
    // RED: This test will fail because we haven't implemented the validation
    cfg := &config.Config{
        MinioEndpoint:   "localhost:9000",
        MinioAccessKey:  "invalid",
        MinioSecretKey:  "invalid",
        MinioBucketName: "test-bucket",
    }
    
    service := NewMinIOService(cfg)
    err := service.Connect()
    
    // Test should fail here - we want proper error handling
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "authentication failed")
}
```

**Run the test:**
```bash
go test ./services -v -run TestMinIOService_Connect_FailsWithInvalidCredentials
```

**Expected Result:** ❌ Test fails (RED phase complete)

### 2. GREEN Phase: Write Minimal Code to Pass

```go
// services/minio.go
func (m *MinIOService) Connect() error {
    // GREEN: Minimal implementation to pass the test
    if m.config.MinioAccessKey == "invalid" {
        return fmt.Errorf("authentication failed: invalid credentials")
    }
    
    // Actual MinIO connection logic here
    client, err := minio.New(m.config.MinioEndpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(m.config.MinioAccessKey, m.config.MinioSecretKey, ""),
        Secure: m.config.MinioUseSSL,
    })
    
    if err != nil {
        return fmt.Errorf("failed to create MinIO client: %w", err)
    }
    
    m.client = client
    return nil
}
```

**Run the test again:**
```bash
go test ./services -v -run TestMinIOService_Connect_FailsWithInvalidCredentials
```

**Expected Result:** ✅ Test passes (GREEN phase complete)

### 3. REFACTOR Phase: Improve the Code

```go
// services/minio.go - Refactored version
func (m *MinIOService) Connect() error {
    if err := m.validateConfig(); err != nil {
        return fmt.Errorf("configuration validation failed: %w", err)
    }
    
    client, err := minio.New(m.config.MinioEndpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(m.config.MinioAccessKey, m.config.MinioSecretKey, ""),
        Secure: m.config.MinioUseSSL,
    })
    
    if err != nil {
        return fmt.Errorf("failed to create MinIO client: %w", err)
    }
    
    // Test connection with a lightweight operation
    if _, err := client.ListBuckets(context.Background()); err != nil {
        return fmt.Errorf("authentication failed: %w", err)
    }
    
    m.client = client
    return nil
}

func (m *MinIOService) validateConfig() error {
    if m.config.MinioAccessKey == "" || m.config.MinioSecretKey == "" {
        return errors.New("MinIO credentials cannot be empty")
    }
    if m.config.MinioEndpoint == "" {
        return errors.New("MinIO endpoint cannot be empty")
    }
    return nil
}
```

**Run all tests:**
```bash
go test ./services -v
```

**Expected Result:** ✅ All tests pass (REFACTOR phase complete)

## 100% Test Coverage Strategy

### Current Coverage Configuration

**Go Backend Coverage:**
```bash
# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Coverage thresholds enforced in CI/CD
COVERAGE_THRESHOLD=95
```

**React Frontend Coverage (vitest.config.ts):**
```typescript
coverage: {
  thresholds: {
    global: {
      branches: 100,
      functions: 100,
      lines: 100,
      statements: 100
    }
  },
  failOnCoverageThreshold: true,
  all: true
}
```

### Maintaining 100% Coverage

#### 1. Write Tests First

Always start with tests. Example workflow for a new feature:

```bash
# 1. Create the test file first
touch handlers/upload_test.go

# 2. Write failing tests
# 3. Run tests (they should fail)
go test ./handlers -v

# 4. Implement the handler
# 5. Run tests until they pass
# 6. Refactor while maintaining test coverage
```

#### 2. Test Coverage Patterns

**Table-Driven Tests for Multiple Scenarios:**

```go
func TestConfigurePiRuntime(t *testing.T) {
    tests := []struct {
        name               string
        cpuCount           int
        expectedMaxProcs   int
        gcTargetPercentage int
        maxMemoryLimitMB   int64
    }{
        {
            name:               "Single CPU",
            cpuCount:           1,
            expectedMaxProcs:   1,
            gcTargetPercentage: 50,
            maxMemoryLimitMB:   512,
        },
        {
            name:               "Dual CPU",
            cpuCount:           2,
            expectedMaxProcs:   2,
            gcTargetPercentage: 50,
            maxMemoryLimitMB:   1024,
        },
        {
            name:               "Quad CPU (Pi 4/5)",
            cpuCount:           4,
            expectedMaxProcs:   3,
            gcTargetPercentage: 50,
            maxMemoryLimitMB:   2048,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cfg := &config.Config{
                PiOptimization:     true,
                GCTargetPercentage: tt.gcTargetPercentage,
                MaxMemoryLimitMB:   tt.maxMemoryLimitMB,
            }

            configurePiRuntime(cfg)
            
            currentMaxProcs := runtime.GOMAXPROCS(0)
            if currentMaxProcs <= 0 {
                t.Error("GOMAXPROCS should be set to positive value")
            }
        })
    }
}
```

#### 3. Test All Code Paths

```go
func TestMemoryLimitValidation(t *testing.T) {
    tests := []struct {
        name         string
        memLimitMB   int64
        shouldSetMem bool
    }{
        {
            name:         "Valid memory limit",
            memLimitMB:   512,
            shouldSetMem: true,
        },
        {
            name:         "Zero memory limit",
            memLimitMB:   0,
            shouldSetMem: false,
        },
        {
            name:         "Negative memory limit",
            memLimitMB:   -100,
            shouldSetMem: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cfg := &config.Config{
                PiOptimization:     true,
                GCTargetPercentage: 50,
                MaxMemoryLimitMB:   tt.memLimitMB,
            }

            configurePiRuntime(cfg)

            if cfg.MaxMemoryLimitMB != tt.memLimitMB {
                t.Errorf("Memory limit modified during configuration")
            }
        })
    }
}
```

## Go Backend TDD Patterns

### 1. Handler Testing Pattern

```go
func TestHealthCheck_IncludesBuildCommit(t *testing.T) {
    // Arrange
    os.Setenv("IMAGE_REVISION", "test-commit-sha")
    defer os.Unsetenv("IMAGE_REVISION")

    cfg := config.New()
    h := New(nil, nil, nil, nil, cfg)

    app := fiber.New()
    app.Get("/api/health", h.HealthCheck)

    // Act
    req := httptest.NewRequest("GET", "/api/health", nil)
    resp, err := app.Test(req)
    if err != nil {
        t.Fatalf("app.Test error: %v", err)
    }

    var body map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
        t.Fatalf("failed to decode response: %v", err)
    }

    // Assert
    build, ok := body["build"].(map[string]interface{})
    if !ok {
        t.Fatalf("expected build field in response")
    }
    if commit, _ := build["commit"].(string); commit != "test-commit-sha" {
        t.Fatalf("expected build.commit to be 'test-commit-sha', got '%s'", commit)
    }
}
```

### 2. Service Testing with Mocks

```go
type MockMinIOService struct {
    shouldFail bool
}

func (m *MockMinIOService) Upload(ctx context.Context, filename string, reader io.Reader, size int64) error {
    if m.shouldFail {
        return errors.New("upload failed")
    }
    return nil
}

func TestFileUploadHandler_WithMinIOError(t *testing.T) {
    // Arrange
    mockMinIO := &MockMinIOService{shouldFail: true}
    handler := NewFileHandler(mockMinIO)

    // Act & Assert
    // Test that handler properly handles MinIO errors
}
```

### 3. Integration Testing

```go
// integration_test.go
// +build integration

func TestEndToEndUploadOnly(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    // Test requires actual MinIO instance
    minioClient := setupMinIOClient(t)
    
    // Create test file
    testData := []byte("test audio data")
    
    // Upload file
    err := minioClient.PutObject(context.Background(), "test-bucket", 
        "test.wav", bytes.NewReader(testData), int64(len(testData)), 
        minio.PutObjectOptions{})
    
    assert.NoError(t, err)
    
    // Cleanup
    defer func() {
        minioClient.RemoveObject(context.Background(), "test-bucket", "test.wav", minio.RemoveObjectOptions{})
    }()
}
```

### 4. Benchmark Testing

```go
func BenchmarkConfigurePiRuntime(b *testing.B) {
    cfg := &config.Config{
        PiOptimization:     true,
        GCTargetPercentage: 50,
        MaxMemoryLimitMB:   1024,
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        configurePiRuntime(cfg)
    }
}
```

## React/TypeScript Frontend TDD

### 1. Component Testing Pattern

```typescript
// components/UploadDropzone.test.tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { UploadDropzone } from './UploadDropzone'

describe('UploadDropzone', () => {
  it('should accept WAV files when dropped', async () => {
    // Arrange
    const onFilesAccepted = vi.fn()
    render(<UploadDropzone onFilesAccepted={onFilesAccepted} />)

    // Act
    const file = new File(['audio data'], 'test.wav', { type: 'audio/wav' })
    const input = screen.getByLabelText(/upload files/i)
    await userEvent.upload(input, file)

    // Assert
    await waitFor(() => {
      expect(onFilesAccepted).toHaveBeenCalledWith([file])
    })
  })

  it('should reject non-WAV files', async () => {
    // RED: Write failing test first
    const onFilesRejected = vi.fn()
    render(<UploadDropzone onFilesRejected={onFilesRejected} />)

    const file = new File(['text'], 'test.txt', { type: 'text/plain' })
    const input = screen.getByLabelText(/upload files/i)
    await userEvent.upload(input, file)

    await waitFor(() => {
      expect(onFilesRejected).toHaveBeenCalledWith([
        expect.objectContaining({
          file,
          errors: expect.arrayContaining([
            expect.objectContaining({
              code: 'file-invalid-type'
            })
          ])
        })
      ])
    })
  })
})
```

### 2. Hook Testing

```typescript
// hooks/useFileUpload.test.ts
import { renderHook, act } from '@testing-library/react'
import { useFileUpload } from './useFileUpload'

describe('useFileUpload', () => {
  it('should start upload when files are provided', async () => {
    // Arrange
    const { result } = renderHook(() => useFileUpload())

    // Act
    const file = new File(['audio'], 'test.wav', { type: 'audio/wav' })
    act(() => {
      result.current.startUpload([file])
    })

    // Assert
    expect(result.current.isUploading).toBe(true)
    expect(result.current.progress).toBe(0)
  })

  it('should handle upload errors gracefully', async () => {
    // Test error scenarios
  })
})
```

### 3. API Integration Testing

```typescript
// lib/api.test.ts
import { uploadFile } from './api'

// Mock fetch globally
global.fetch = vi.fn()

describe('API Functions', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('should upload file with progress tracking', async () => {
    // Arrange
    const mockResponse = { ok: true, json: () => Promise.resolve({ success: true }) }
    const mockFetch = vi.mocked(fetch).mockResolvedValue(mockResponse as Response)

    const file = new File(['audio'], 'test.wav')
    const onProgress = vi.fn()

    // Act
    await uploadFile(file, onProgress)

    // Assert
    expect(mockFetch).toHaveBeenCalledWith('/api/upload', expect.objectContaining({
      method: 'POST',
      body: expect.any(FormData)
    }))
  })
})
```

## CI/CD Integration

### Pre-Commit Hooks (.githooks/pre-commit)

The project uses a comprehensive pre-commit hook that enforces TDD practices:

```bash
#!/bin/bash
set -e

echo "🔍 Running pre-commit checks..."

# Go backend checks
if command_exists go; then
  cd backend
  
  # 1. Verify Go modules
  go mod verify || exit 1
  
  # 2. Build check (compilation errors)
  go build -o /tmp/sermon-uploader . || exit 1
  
  # 3. Run go vet (static analysis)
  go vet ./... || exit 1
  
  # 4. Check formatting
  UNFORMATTED=$(gofmt -l .)
  if [ -n "$UNFORMATTED" ]; then
    echo "❌ Go files need formatting:"
    echo "$UNFORMATTED"
    exit 1
  fi
  
  # 5. Run golangci-lint (comprehensive linting)
  if command_exists golangci-lint; then
    golangci-lint run --config .golangci.yml || exit 1
  fi
  
  cd ..
fi

echo "🎉 All pre-commit checks passed! Safe to commit."
```

### Fast Integration Tests (Pre-Commit)

```bash
# scripts/pre-commit-tests.sh
# Fast integration tests (max 30 seconds)

if curl -f -s --max-time 2 "http://localhost:9000/minio/health/live" > /dev/null 2>&1; then
    # Run fast integration tests
    go test -tags=integration -timeout=30s -run="TestEndToEndUploadOnly|TestHealthMinIOConnectivity" ./... -v
else
    echo "⚠️  MinIO not available, skipping integration tests"
fi
```

### Full Test Suite (CI/CD Pipeline)

```bash
# scripts/run-integration-tests.sh
# Full integration test suite for CI/CD

# 1. Start test dependencies
docker-compose -f docker-compose.test.yml up -d

# 2. Wait for services
wait_for_service "MinIO" "http://localhost:9000/minio/health/live"

# 3. Run full test suite
go test -tags=integration -timeout=300s ./... -v -coverprofile=coverage.out

# 4. Check coverage thresholds
go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//' | \
  awk '{if ($1 < 95) { print "Coverage too low: " $1 "%"; exit 1 }}'

# 5. Cleanup
docker-compose -f docker-compose.test.yml down
```

### Deployment Pipeline Integration

```yaml
# .github/workflows/deploy.yml (example)
name: Deploy
on:
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v3
        with:
          go-version: 1.23
      
      - name: Run Tests
        run: |
          go test -v -coverprofile=coverage.out ./...
          go tool cover -func=coverage.out
      
      - name: Lint
        run: |
          curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2
          golangci-lint run --config .golangci.yml
      
      - name: Integration Tests
        run: ./scripts/run-integration-tests.sh

  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - name: Deploy to Production
        run: echo "Deploying..."
```

## Developer Onboarding Guide

### 1. Environment Setup

**Prerequisites:**
```bash
# Go 1.23+
go version

# Node.js 18+
node --version
npm --version

# Docker & Docker Compose
docker --version
docker-compose --version

# golangci-lint
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2
```

**Project Setup:**
```bash
# 1. Clone and setup
git clone <repo-url>
cd sermon-uploader

# 2. Setup git hooks
git config core.hooksPath .githooks
chmod +x .githooks/*

# 3. Install dependencies
cd backend && go mod download
cd ../frontend && npm install

# 4. Setup test environment
cp .env.example .env.test
# Edit .env.test with test credentials
```

### 2. First TDD Exercise

**Goal:** Add a new health check endpoint that returns system memory usage.

**Step 1 - Write the Test (RED):**
```bash
cd backend
touch handlers/system_test.go
```

```go
// handlers/system_test.go
package handlers

import (
    "encoding/json"
    "net/http/httptest"
    "testing"

    "github.com/gofiber/fiber/v2"
    "github.com/stretchr/testify/assert"
)

func TestSystemMemoryHandler(t *testing.T) {
    // Arrange
    app := fiber.New()
    handler := &Handler{}
    app.Get("/api/system/memory", handler.SystemMemory)

    // Act
    req := httptest.NewRequest("GET", "/api/system/memory", nil)
    resp, err := app.Test(req)

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)

    var response map[string]interface{}
    err = json.NewDecoder(resp.Body).Decode(&response)
    assert.NoError(t, err)
    
    // Should have memory information
    assert.Contains(t, response, "totalMemory")
    assert.Contains(t, response, "usedMemory")
    assert.Contains(t, response, "freeMemory")
    assert.IsType(t, float64(0), response["totalMemory"])
}
```

**Step 2 - Run Test (Should Fail):**
```bash
go test ./handlers -v -run TestSystemMemoryHandler
# Expected: FAIL - method doesn't exist
```

**Step 3 - Write Minimal Code (GREEN):**
```go
// handlers/system.go
package handlers

import (
    "runtime"
    "github.com/gofiber/fiber/v2"
)

func (h *Handler) SystemMemory(c *fiber.Ctx) error {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    return c.JSON(fiber.Map{
        "totalMemory": float64(m.Sys),
        "usedMemory":  float64(m.Alloc),
        "freeMemory":  float64(m.Sys - m.Alloc),
    })
}
```

**Step 4 - Run Test (Should Pass):**
```bash
go test ./handlers -v -run TestSystemMemoryHandler
# Expected: PASS
```

**Step 5 - Refactor:**
```go
// handlers/system.go - Improved version
package handlers

import (
    "runtime"
    "github.com/gofiber/fiber/v2"
)

type MemoryInfo struct {
    TotalMemory uint64 `json:"totalMemory"`
    UsedMemory  uint64 `json:"usedMemory"`
    FreeMemory  uint64 `json:"freeMemory"`
}

func (h *Handler) SystemMemory(c *fiber.Ctx) error {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    info := MemoryInfo{
        TotalMemory: m.Sys,
        UsedMemory:  m.Alloc,
        FreeMemory:  m.Sys - m.Alloc,
    }

    return c.JSON(info)
}
```

### 3. TDD Best Practices Checklist

**Before Writing Any Code:**
- [ ] Is there a failing test for this feature?
- [ ] Does the test cover the happy path?
- [ ] Does the test cover error cases?
- [ ] Is the test name descriptive?

**While Writing Code:**
- [ ] Am I writing the minimal code to pass the test?
- [ ] Am I avoiding over-engineering?
- [ ] Am I focusing on one failing test at a time?

**After Code Works:**
- [ ] Can I refactor without breaking tests?
- [ ] Are there any code smells?
- [ ] Is the code readable and maintainable?
- [ ] Are all tests still passing?

## Troubleshooting Common Issues

### Go Testing Issues

#### 1. Import Cycle Errors
```bash
# Error: import cycle not allowed
```

**Solution:** Restructure packages to avoid circular dependencies:
```go
// Instead of this (circular):
// handlers → services → handlers

// Use this (hierarchical):
// handlers → services → repositories → models
```

#### 2. Race Conditions in Tests
```bash
# Error: race condition detected
```

**Solution:** Use proper synchronization:
```go
func TestConcurrentUploads(t *testing.T) {
    var wg sync.WaitGroup
    var mu sync.Mutex
    results := make([]error, 0)

    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            err := uploadFile("test.wav")
            
            mu.Lock()
            results = append(results, err)
            mu.Unlock()
        }()
    }

    wg.Wait()
    // Assert results
}
```

#### 3. Test Timeouts
```bash
# Error: test timed out
```

**Solution:** Use proper timeouts and contexts:
```go
func TestLongRunningOperation(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    err := longRunningOperation(ctx)
    assert.NoError(t, err)
}
```

### React/TypeScript Testing Issues

#### 1. Mock API Calls
```typescript
// Error: Real API calls in tests

// Solution: Use MSW (Mock Service Worker)
import { rest } from 'msw'
import { setupServer } from 'msw/node'

const server = setupServer(
  rest.post('/api/upload', (req, res, ctx) => {
    return res(ctx.json({ success: true }))
  })
)

beforeAll(() => server.listen())
afterEach(() => server.resetHandlers())
afterAll(() => server.close())
```

#### 2. Component State Testing
```typescript
// Error: Testing implementation details

// Instead of testing state directly:
// expect(component.state.isLoading).toBe(true)

// Test behavior:
expect(screen.getByText(/uploading/i)).toBeInTheDocument()
```

#### 3. Async Component Testing
```typescript
// Error: Tests finish before async operations

// Solution: Use waitFor
import { waitFor } from '@testing-library/react'

it('should show upload complete message', async () => {
  render(<UploadComponent />)
  
  fireEvent.click(screen.getByText(/upload/i))
  
  await waitFor(() => {
    expect(screen.getByText(/upload complete/i)).toBeInTheDocument()
  })
})
```

### Coverage Issues

#### 1. Untested Error Paths
```go
// Problem: Error paths not covered

func ProcessFile(filename string) error {
    if filename == "" {
        return errors.New("filename required") // This line not tested
    }
    
    // ... processing logic
    return nil
}

// Solution: Add test for error case
func TestProcessFile_EmptyFilename(t *testing.T) {
    err := ProcessFile("")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "filename required")
}
```

#### 2. Branch Coverage Issues
```go
// Problem: Not all branches tested

func CalculatePrice(items []Item, discount bool) float64 {
    total := 0.0
    for _, item := range items {
        total += item.Price
    }
    
    if discount && total > 100 { // Both conditions need testing
        return total * 0.9
    }
    
    return total
}

// Solution: Test all combinations
func TestCalculatePrice(t *testing.T) {
    tests := []struct {
        name     string
        items    []Item
        discount bool
        expected float64
    }{
        {"no discount", []Item{{Price: 50}}, false, 50},
        {"discount but under 100", []Item{{Price: 50}}, true, 50},
        {"discount over 100", []Item{{Price: 150}}, true, 135},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := CalculatePrice(tt.items, tt.discount)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

## Testing Tools and Configuration

### Go Testing Stack

**Core Tools:**
- `go test` - Built-in testing
- `testify` - Assertions and mocks
- `golangci-lint` - Comprehensive linting

**Configuration (.golangci.yml highlights):**
```yaml
linters:
  enable:
    - errcheck      # Check for unhandled errors
    - gosec         # Security issues
    - govet         # Suspicious constructs
    - staticcheck   # Static analysis
    - unused        # Unused code
    - ineffassign   # Ineffective assignments
    - typecheck     # Type checking
```

**Test Build Tags:**
```go
// +build integration
// integration_test.go

// Run with: go test -tags=integration
```

### React/TypeScript Testing Stack

**Core Tools:**
- `vitest` - Fast test runner
- `@testing-library/react` - Component testing
- `@testing-library/user-event` - User interaction
- `jsdom` - DOM environment

**Configuration (vitest.config.ts highlights):**
```typescript
export default defineConfig({
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    coverage: {
      thresholds: {
        global: {
          branches: 100,
          functions: 100,
          lines: 100,
          statements: 100
        }
      },
      failOnCoverageThreshold: true
    }
  }
})
```

### CI/CD Testing Pipeline

**Stages:**
1. **Linting** (Fast feedback)
2. **Unit Tests** (Business logic)
3. **Integration Tests** (Component interaction)
4. **E2E Tests** (Full system)
5. **Performance Tests** (Load/stress)

**Commands:**
```bash
# Local development
make test-quick    # Unit tests only
make test-all      # All tests including integration
make coverage      # Coverage report

# CI/CD pipeline
make ci-test       # Full CI test suite
make deploy-test   # Deployment verification
```

## Real Codebase Examples

### Example 1: MinIO Service TDD

**Business Requirement:** Upload files to MinIO with duplicate detection

**Test First Approach:**
```go
func TestMinIOService_UploadWithDuplicateDetection(t *testing.T) {
    // Setup
    service := NewMinIOService(testConfig)
    ctx := context.Background()
    
    file1 := bytes.NewReader([]byte("content"))
    file2 := bytes.NewReader([]byte("content")) // Same content
    
    // First upload should succeed
    err1 := service.UploadFile(ctx, "test1.wav", file1, int64(len("content")))
    assert.NoError(t, err1)
    
    // Second upload with same content should be detected as duplicate
    err2 := service.UploadFile(ctx, "test2.wav", file2, int64(len("content")))
    assert.Error(t, err2)
    assert.Contains(t, err2.Error(), "duplicate content detected")
}
```

**Implementation:**
```go
func (m *MinIOService) UploadFile(ctx context.Context, filename string, reader io.Reader, size int64) error {
    // Calculate hash for duplicate detection
    hasher := sha256.New()
    teeReader := io.TeeReader(reader, hasher)
    
    // Buffer the content for hash calculation
    content, err := io.ReadAll(teeReader)
    if err != nil {
        return fmt.Errorf("failed to read file content: %w", err)
    }
    
    hash := hex.EncodeToString(hasher.Sum(nil))
    
    // Check for duplicates
    if m.isDuplicate(ctx, hash) {
        return fmt.Errorf("duplicate content detected: %s", hash)
    }
    
    // Upload to MinIO
    _, err = m.client.PutObject(ctx, m.bucketName, filename, 
        bytes.NewReader(content), int64(len(content)), 
        minio.PutObjectOptions{
            ContentType: "audio/wav",
            UserMetadata: map[string]string{
                "hash": hash,
            },
        })
    
    return err
}
```

### Example 2: React Upload Component TDD

**Business Requirement:** Drag-and-drop file upload with progress tracking

**Test First:**
```typescript
describe('UploadComponent', () => {
  it('should track upload progress', async () => {
    // Arrange
    const mockUpload = vi.fn().mockImplementation((file, onProgress) => {
      // Simulate progress updates
      onProgress(25)
      onProgress(50)
      onProgress(75)
      onProgress(100)
      return Promise.resolve()
    })

    render(<UploadComponent uploadFn={mockUpload} />)

    // Act
    const file = new File(['audio'], 'test.wav')
    const input = screen.getByLabelText(/choose files/i)
    await userEvent.upload(input, file)

    fireEvent.click(screen.getByText(/upload/i))

    // Assert
    await waitFor(() => {
      expect(screen.getByText(/100%/)).toBeInTheDocument()
    })

    expect(screen.getByText(/upload complete/i)).toBeInTheDocument()
  })
})
```

**Implementation:**
```typescript
export function UploadComponent({ uploadFn }: { uploadFn: UploadFunction }) {
  const [progress, setProgress] = useState(0)
  const [isUploading, setIsUploading] = useState(false)
  const [isComplete, setIsComplete] = useState(false)

  const handleUpload = async (files: File[]) => {
    setIsUploading(true)
    setProgress(0)

    try {
      await uploadFn(files[0], (percent) => {
        setProgress(percent)
      })
      setIsComplete(true)
    } catch (error) {
      // Handle error
    } finally {
      setIsUploading(false)
    }
  }

  return (
    <div>
      {isUploading && <ProgressBar value={progress} />}
      {isComplete && <p>Upload complete!</p>}
      <FileDropzone onFilesAccepted={handleUpload} />
    </div>
  )
}
```

### Example 3: Discord Integration TDD

**Business Requirement:** Send live-updating Discord notifications

**Test Strategy:**
```go
func TestDiscordService_LiveNotifications(t *testing.T) {
    tests := []struct {
        name           string
        updates        []ProgressUpdate
        expectedCalls  int
        expectedMessage string
    }{
        {
            name: "Single file upload",
            updates: []ProgressUpdate{
                {Filename: "sermon.wav", Progress: 0, Status: "starting"},
                {Filename: "sermon.wav", Progress: 50, Status: "uploading"},
                {Filename: "sermon.wav", Progress: 100, Status: "complete"},
            },
            expectedCalls: 3,
            expectedMessage: "sermon.wav - Complete ✅",
        },
        {
            name: "Batch upload",
            updates: []ProgressUpdate{
                {Filename: "sermon1.wav", Progress: 0, Status: "starting"},
                {Filename: "sermon2.wav", Progress: 0, Status: "starting"},
                {Filename: "sermon1.wav", Progress: 100, Status: "complete"},
                {Filename: "sermon2.wav", Progress: 100, Status: "complete"},
            },
            expectedCalls: 1, // Single message updated multiple times
            expectedMessage: "Batch Upload Complete ✅\n├─ sermon1.wav ✅\n└─ sermon2.wav ✅",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockWebhook := &MockDiscordWebhook{}
            service := NewDiscordService(testConfig, mockWebhook)

            // Send updates
            for _, update := range tt.updates {
                service.UpdateProgress(update)
            }

            // Verify final state
            assert.Equal(t, tt.expectedCalls, len(mockWebhook.messages))
            lastMessage := mockWebhook.messages[len(mockWebhook.messages)-1]
            assert.Contains(t, lastMessage.Content, tt.expectedMessage)
        })
    }
}
```

This comprehensive TDD workflow ensures that every feature is thoroughly tested, maintainable, and follows best practices. The combination of automated testing, linting, and CI/CD integration creates a robust development environment that catches issues early and maintains high code quality.

---

## Quick Reference Commands

```bash
# Go Backend
go test ./...                              # Run all tests
go test -v -run TestSpecificFunction      # Run specific test
go test -coverprofile=coverage.out ./...  # Coverage report
go tool cover -html=coverage.out         # HTML coverage
golangci-lint run                         # Lint code

# React Frontend  
npm test                                  # Run all tests
npm run test:watch                        # Watch mode
npm run test:coverage                     # Coverage report
npm run lint                              # Lint code

# Integration
./scripts/pre-commit-tests.sh            # Fast integration tests
./scripts/run-integration-tests.sh       # Full integration suite
```

**Remember:** Tests are not just verification—they're documentation, design tools, and safety nets all in one. Write them first, keep them simple, and let them guide your implementation.