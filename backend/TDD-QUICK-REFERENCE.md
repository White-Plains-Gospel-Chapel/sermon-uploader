# TDD Quick Reference Guide

## Sermon Uploader TDD Cheat Sheet

### 🚀 Quick Start Commands

```bash
# Run all tests with coverage
go test -race -coverprofile=coverage.out -covermode=atomic ./...

# Check coverage (must be 100%)
./coverage.sh

# Run fast integration tests (pre-commit)
./scripts/run-integration-tests.sh --fast

# Run full integration test suite
./scripts/run-integration-tests.sh --integration

# Run performance benchmarks
./scripts/run-integration-tests.sh --performance

# Run pre-commit checks
./.githooks/pre-commit
```

---

## 🔄 TDD Cycle Workflow

### 1. 🔴 RED: Write Failing Test
```go
func TestNewFeature_ShouldWork(t *testing.T) {
    result := NewFeature("input")
    assert.Equal(t, "expected", result)
}
```

### 2. 🟢 GREEN: Make Test Pass
```go
func NewFeature(input string) string {
    return "expected" // Minimal implementation
}
```

### 3. 🔵 REFACTOR: Improve Code
```go
func NewFeature(input string) string {
    // Proper implementation after refactoring
    return processInput(input)
}
```

---

## 📋 Test Types Checklist

### Unit Tests (70%)
- [ ] Test all public functions
- [ ] Test error conditions
- [ ] Test edge cases
- [ ] Use mocks for dependencies
- [ ] Fast execution (< 1ms each)

### Integration Tests (25%)
- [ ] Test component interactions
- [ ] Test database operations
- [ ] Test external service calls
- [ ] Test file system operations

### E2E Tests (5%)
- [ ] Test complete user workflows
- [ ] Test critical business processes
- [ ] Test system-wide functionality

---

## 🏗️ Test Structure Template

```go
func TestComponent_Action_ExpectedOutcome(t *testing.T) {
    // Arrange
    setup := createTestSetup(t)
    defer setup.cleanup()
    
    // Act
    result, err := setup.component.Action(input)
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

---

## 🎯 Mock Setup Pattern

```go
// 1. Define interface
type ServiceInterface interface {
    Method(input string) (string, error)
}

// 2. Create mock
type MockService struct {
    mock.Mock
}

func (m *MockService) Method(input string) (string, error) {
    args := m.Called(input)
    return args.String(0), args.Error(1)
}

// 3. Use in test
func TestWithMock(t *testing.T) {
    mockService := &MockService{}
    mockService.On("Method", "input").Return("output", nil)
    defer mockService.AssertExpectations(t)
    
    // Test logic
}
```

---

## 📊 Coverage Commands

```bash
# Generate coverage
go test -coverprofile=coverage.out ./...

# View coverage percentage
go tool cover -func=coverage.out | grep "total:"

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html

# Find uncovered code
go tool cover -func=coverage.out | grep -v "100.0%"
```

---

## 🚨 Common Anti-Patterns to Avoid

### ❌ Don't Test Implementation
```go
// BAD - tests private methods
func TestInternalMethod(t *testing.T) {
    result := obj.internalMethod()
    assert.Equal(t, "expected", result)
}
```

### ✅ Do Test Behavior
```go
// GOOD - tests public behavior
func TestPublicBehavior(t *testing.T) {
    result := obj.PublicMethod()
    assert.Equal(t, "expected", result)
}
```

### ❌ Don't Write Brittle Tests
```go
// BAD - tests specific implementation details
assert.Equal(t, 3, len(response.Data.Items))
```

### ✅ Do Write Robust Tests
```go
// GOOD - tests essential behavior
assert.NotEmpty(t, response.Data.Items)
assert.True(t, response.Success)
```

---

## 🔧 Debugging Test Issues

### Test Failures
```bash
# Run with verbose output
go test -v ./...

# Run specific test
go test -run TestSpecificFunction ./...

# Run with race detection
go test -race ./...
```

### Mock Issues
```go
// Debug mock calls
mockService.On("Method", mock.Anything).Run(func(args mock.Arguments) {
    t.Logf("Mock called with: %v", args)
}).Return("result", nil)
```

### Coverage Issues
```bash
# Find missing coverage
go tool cover -func=coverage.out | sort -k3 -n

# Generate visual coverage report
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```

---

## 🎨 Table-Driven Test Pattern

```go
func TestValidation(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected bool
        wantErr  bool
    }{
        {"valid input", "valid.wav", true, false},
        {"empty input", "", false, true},
        {"invalid format", "file.txt", false, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := Validate(tt.input)
            
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expected, result)
            }
        })
    }
}
```

---

## 🏃‍♂️ Performance Testing

```go
func BenchmarkOperation(b *testing.B) {
    service := setupService()
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        service.Operation("input")
    }
}

// Run benchmarks
go test -bench=. -benchmem ./...
```

---

## 🔄 Integration Test Setup

```go
// +build integration

func TestIntegration(t *testing.T) {
    env := setupIntegrationTest(t)
    defer env.cleanup()
    
    // Test with real services
    result, err := env.service.RealOperation()
    
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

---

## 📋 Pre-Commit Checklist

Before every commit:
- [ ] ✅ All tests pass: `go test ./...`
- [ ] ✅ 100% coverage: `./coverage.sh`
- [ ] ✅ Code formatted: `gofmt -w .`
- [ ] ✅ No lint issues: `golangci-lint run`
- [ ] ✅ No race conditions: `go test -race ./...`

---

## 🚀 CI/CD Test Pipeline

```yaml
# Quick reference for GitHub Actions
jobs:
  test:
    steps:
    - name: Unit Tests
      run: go test -race -coverprofile=coverage.out ./...
    
    - name: Coverage Check
      run: ./coverage.sh
    
    - name: Integration Tests
      run: ./scripts/run-integration-tests.sh --ci
    
    - name: Lint
      run: golangci-lint run
```

---

## 📚 Testing Principles

1. **F.I.R.S.T.**
   - **Fast**: Tests should run quickly
   - **Independent**: Tests should not depend on each other
   - **Repeatable**: Tests should produce same results every time
   - **Self-Validating**: Tests should have clear pass/fail result
   - **Timely**: Tests should be written at the right time

2. **AAA Pattern**
   - **Arrange**: Set up test data and conditions
   - **Act**: Execute the code under test
   - **Assert**: Verify the results

3. **Test Names Should Be Descriptive**
   - `TestComponent_Action_ExpectedOutcome`
   - `TestFileUpload_LargeFile_ShouldUseDirectMinIO`
   - `TestDuplicateCheck_ExistingFile_ShouldReturnTrue`

---

## 🎯 Sermon Uploader Specific Examples

### File Upload Test
```go
func TestPresignedURL_ValidFile_ShouldReturnURL(t *testing.T) {
    mockMinio := &MockMinIOService{}
    handler := &TestHandlers{minioService: mockMinio}
    
    mockMinio.On("CheckDuplicateByFilename", "sermon.wav").Return(false, nil)
    mockMinio.On("GeneratePresignedUploadURL", "sermon.wav", mock.AnythingOfType("time.Duration")).Return("http://presigned-url", nil)
    
    response := callHandler(handler, "sermon.wav", 1048576)
    
    assert.True(t, response.Success)
    assert.Equal(t, "http://presigned-url", response.UploadURL)
    mockMinio.AssertExpectations(t)
}
```

### Duplicate Detection Test
```go
func TestDuplicateCheck_ExistingFile_ShouldReturnConflict(t *testing.T) {
    mockMinio := &MockMinIOService{}
    handler := &TestHandlers{minioService: mockMinio}
    
    mockMinio.On("CheckDuplicateByFilename", "duplicate.wav").Return(true, nil)
    
    response := callHandler(handler, "duplicate.wav", 1024)
    
    assert.Equal(t, 409, response.StatusCode)
    assert.True(t, response.IsDuplicate)
}
```

### Large File Test
```go
func TestLargeFile_Over100MB_ShouldUseDirectMinIO(t *testing.T) {
    mockMinio := &MockMinIOService{}
    handler := &TestHandlers{minioService: mockMinio}
    
    largeFileSize := int64(200 * 1024 * 1024) // 200MB
    mockMinio.On("GeneratePresignedUploadURLSmart", "large.wav", largeFileSize, mock.AnythingOfType("time.Duration")).Return("http://direct-url", true, nil)
    
    response := callHandler(handler, "large.wav", largeFileSize)
    
    assert.True(t, response.IsLargeFile)
    assert.Equal(t, "direct_minio", response.UploadMethod)
}
```

---

## 🚑 Emergency Test Fixes

### Flaky Test Fix
```go
func TestWithRetry(t *testing.T) {
    var result string
    var err error
    
    // Retry flaky operations
    for i := 0; i < 3; i++ {
        result, err = flakyOperation()
        if err == nil {
            break
        }
        time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
    }
    
    assert.NoError(t, err)
    assert.NotEmpty(t, result)
}
```

### Timeout Fix
```go
func TestWithTimeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    result, err := operationWithContext(ctx)
    
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

### Memory Leak Fix
```go
func TestWithProperCleanup(t *testing.T) {
    resource := createResource()
    defer func() {
        if err := resource.Close(); err != nil {
            t.Errorf("Failed to close resource: %v", err)
        }
    }()
    
    // Test logic
}
```

---

## 📞 Emergency Contacts

When tests fail in production:
1. Check CI/CD pipeline logs
2. Review recent commits
3. Run local reproduction
4. Check monitoring dashboards
5. Escalate to team lead if needed

---

This quick reference provides immediate access to the most commonly needed TDD patterns and commands for the sermon uploader project.