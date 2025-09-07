# TDD Developer Onboarding Guide

## Welcome to Sermon Uploader Development Team

This guide will get you productive with Test-Driven Development (TDD) in the sermon uploader project within your first week.

---

## 🎯 Learning Path Overview

### Day 1: Environment Setup & First Test
### Day 2-3: TDD Practice & Patterns  
### Day 4-5: Integration Testing & Mocking
### Week 2+: Advanced Topics & Production Code

---

## 📋 Day 1: Environment Setup & First Test

### Prerequisites Checklist

```bash
# Verify Go installation
go version  # Should be 1.23+

# Verify Git setup
git --version
git config --global user.name "Your Name"
git config --global user.email "your.email@example.com"

# Verify Docker (for integration tests)
docker --version
docker run hello-world
```

### Project Setup

```bash
# Clone the repository
git clone <repository-url>
cd sermon-uploader/backend

# Install dependencies
go mod download
go mod verify

# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Setup git hooks
git config core.hooksPath .githooks
chmod +x .githooks/pre-commit
```

### First Test Run

```bash
# Run all tests
go test ./... -v

# Check coverage
./coverage.sh

# Should see output like:
# ✅ SUCCESS: Coverage 100% meets the required 100% threshold!
```

### Your First TDD Cycle

Let's implement a simple filename validator using TDD:

#### 🔴 Step 1: Write Failing Test

Create `utils/validator_test.go`:
```go
package utils

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestValidateSermonFilename_ValidWAV_ShouldReturnTrue(t *testing.T) {
    // This test will fail because ValidateSermonFilename doesn't exist yet
    isValid, err := ValidateSermonFilename("sunday_sermon_2023.wav")
    
    assert.NoError(t, err)
    assert.True(t, isValid, "Valid WAV file should be accepted")
}

func TestValidateSermonFilename_EmptyString_ShouldReturnError(t *testing.T) {
    isValid, err := ValidateSermonFilename("")
    
    assert.Error(t, err)
    assert.False(t, isValid)
    assert.Contains(t, err.Error(), "filename cannot be empty")
}
```

Run the test - it should fail:
```bash
go test ./utils -v
# Should show compilation error because function doesn't exist
```

#### 🟢 Step 2: Make Test Pass

Create `utils/validator.go`:
```go
package utils

import (
    "errors"
    "strings"
)

func ValidateSermonFilename(filename string) (bool, error) {
    // Minimal implementation to make tests pass
    if filename == "" {
        return false, errors.New("filename cannot be empty")
    }
    
    if strings.HasSuffix(filename, ".wav") {
        return true, nil
    }
    
    return false, nil
}
```

Run tests again:
```bash
go test ./utils -v
# Both tests should pass now
```

#### 🔵 Step 3: Refactor

Add more test cases and improve implementation:

```go
// Add to validator_test.go
func TestValidateSermonFilename_InvalidExtension_ShouldReturnFalse(t *testing.T) {
    isValid, err := ValidateSermonFilename("sermon.txt")
    
    assert.NoError(t, err)
    assert.False(t, isValid, "Non-audio file should be rejected")
}

func TestValidateSermonFilename_ValidMP3_ShouldReturnTrue(t *testing.T) {
    isValid, err := ValidateSermonFilename("sermon.mp3")
    
    assert.NoError(t, err)
    assert.True(t, isValid, "Valid MP3 file should be accepted")
}
```

Refactor the implementation:
```go
package utils

import (
    "errors"
    "path/filepath"
    "strings"
)

func ValidateSermonFilename(filename string) (bool, error) {
    if filename == "" {
        return false, errors.New("filename cannot be empty")
    }
    
    // Get file extension
    ext := strings.ToLower(filepath.Ext(filename))
    
    // Check if it's a supported audio format
    supportedFormats := []string{".wav", ".mp3", ".aac", ".m4a"}
    for _, format := range supportedFormats {
        if ext == format {
            return true, nil
        }
    }
    
    return false, nil
}
```

Verify all tests still pass:
```bash
go test ./utils -v
./coverage.sh
```

### Day 1 Assignment

Implement a file size validator using TDD:
- Function: `ValidateFileSize(size int64) (bool, error)`
- Requirements:
  - Files over 1GB should be rejected
  - Files under 1KB should be rejected  
  - Valid range: 1KB - 1GB
  - Must have 100% test coverage

---

## 📚 Day 2-3: TDD Practice & Patterns

### Table-Driven Tests

Learn the most efficient testing pattern in Go:

```go
func TestValidateSermonFilename_MultipleScenarios(t *testing.T) {
    tests := []struct {
        name        string
        filename    string
        expectValid bool
        expectError bool
        errorMsg    string
    }{
        {
            name:        "valid WAV file",
            filename:    "sermon.wav",
            expectValid: true,
            expectError: false,
        },
        {
            name:        "valid MP3 file", 
            filename:    "message.mp3",
            expectValid: true,
            expectError: false,
        },
        {
            name:        "invalid extension",
            filename:    "document.pdf",
            expectValid: false,
            expectError: false,
        },
        {
            name:        "empty filename",
            filename:    "",
            expectValid: false,
            expectError: true,
            errorMsg:    "filename cannot be empty",
        },
        {
            name:        "filename without extension",
            filename:    "sermon",
            expectValid: false,
            expectError: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            isValid, err := ValidateSermonFilename(tt.filename)
            
            assert.Equal(t, tt.expectValid, isValid)
            
            if tt.expectError {
                assert.Error(t, err)
                if tt.errorMsg != "" {
                    assert.Contains(t, err.Error(), tt.errorMsg)
                }
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Test Helpers and Utilities

Create reusable test components:

```go
// testutil/helpers.go
package testutil

import (
    "bytes"
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/require"
)

// TestFileData represents test file information
type TestFileData struct {
    Filename string
    Content  []byte
    Size     int64
}

// CreateTestWAVFile generates a valid WAV file for testing
func CreateTestWAVFile(t *testing.T, name string, size int64) TestFileData {
    content := make([]byte, size)
    
    // Add basic WAV header (simplified)
    if size >= 44 {
        copy(content[0:4], []byte("RIFF"))
        copy(content[8:12], []byte("WAVE"))
        copy(content[12:16], []byte("fmt "))
    }
    
    return TestFileData{
        Filename: name,
        Content:  content,
        Size:     size,
    }
}

// CreateTestContext creates a context with reasonable timeout for tests
func CreateTestContext(t *testing.T) (context.Context, context.CancelFunc) {
    return context.WithTimeout(context.Background(), 30*time.Second)
}

// AssertEventuallyTrue waits for a condition to become true
func AssertEventuallyTrue(t *testing.T, condition func() bool, timeout time.Duration, msg string) {
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()
    
    timeoutCh := time.After(timeout)
    
    for {
        select {
        case <-ticker.C:
            if condition() {
                return
            }
        case <-timeoutCh:
            t.Fatal(msg)
        }
    }
}
```

### Error Testing Patterns

Learn to test error conditions thoroughly:

```go
func TestFileProcessor_ProcessFile_ErrorScenarios(t *testing.T) {
    tests := []struct {
        name          string
        setupMock     func(*MockFileService)
        filename      string
        expectErrType error
        expectErrMsg  string
    }{
        {
            name: "file service error",
            setupMock: func(m *MockFileService) {
                m.On("ValidateFile", "test.wav").Return(false, errors.New("validation service error"))
            },
            filename:      "test.wav",
            expectErrType: &ServiceError{},
            expectErrMsg:  "validation service error",
        },
        {
            name: "file too large",
            setupMock: func(m *MockFileService) {
                m.On("ValidateFile", "large.wav").Return(false, &FileSizeError{Size: 2000000000})
            },
            filename:      "large.wav", 
            expectErrType: &FileSizeError{},
            expectErrMsg:  "file size 2000000000 exceeds maximum",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockService := &MockFileService{}
            tt.setupMock(mockService)
            
            processor := &FileProcessor{fileService: mockService}
            
            err := processor.ProcessFile(tt.filename, []byte("data"))
            
            assert.Error(t, err)
            assert.IsType(t, tt.expectErrType, err)
            assert.Contains(t, err.Error(), tt.expectErrMsg)
            
            mockService.AssertExpectations(t)
        })
    }
}
```

### Day 2-3 Assignment

Implement a metadata extractor for audio files using TDD:
- Function: `ExtractAudioMetadata(filename string, data []byte) (*AudioMetadata, error)`
- Requirements:
  - Extract file size, duration (mock for now), format
  - Handle different audio formats (WAV, MP3, AAC)
  - Return structured metadata
  - Handle invalid files gracefully
  - Use table-driven tests
  - Must achieve 100% coverage

---

## 🔗 Day 4-5: Integration Testing & Mocking

### Understanding Mocks vs Real Services

#### When to Use Mocks (Unit Tests)
```go
func TestPresignedURLHandler_Success_ShouldReturnURL(t *testing.T) {
    // Use mocks for fast, isolated unit tests
    mockMinio := &MockMinIOService{}
    mockConfig := &config.Config{}
    
    handler := &TestHandlers{
        minioService: mockMinio,
        config:       mockConfig,
    }
    
    // Set up mock expectations
    mockMinio.On("CheckDuplicateByFilename", "test.wav").Return(false, nil)
    mockMinio.On("GeneratePresignedUploadURL", "test.wav", mock.AnythingOfType("time.Duration")).Return("http://mocked-url", nil)
    
    // Execute handler
    response := callHandler(handler, "test.wav", 1024)
    
    // Verify behavior
    assert.True(t, response.Success)
    assert.Equal(t, "http://mocked-url", response.UploadURL)
    mockMinio.AssertExpectations(t)
}
```

#### When to Use Real Services (Integration Tests)
```go
// +build integration

func TestEndToEndUpload_RealServices_ShouldWork(t *testing.T) {
    // Use real services for integration tests
    env := setupIntegrationEnvironment(t)
    defer env.cleanup()
    
    // Test with actual MinIO instance
    filename := "integration_test.wav"
    fileData := testutil.CreateTestWAVFile(t, filename, 1024*1024)
    
    // Get real presigned URL
    presignedURL, err := env.fileService.GeneratePresignedUploadURL(filename, time.Hour)
    require.NoError(t, err)
    assert.True(t, strings.HasPrefix(presignedURL, "http"))
    
    // Perform actual upload
    err = uploadToPresignedURL(presignedURL, fileData.Content)
    require.NoError(t, err)
    
    // Verify file exists in MinIO
    exists, err := env.minioClient.StatObject(context.Background(), "sermons", filename, minio.StatObjectOptions{})
    assert.NoError(t, err)
    assert.Equal(t, fileData.Size, exists.Size)
}
```

### Advanced Mock Patterns

#### Conditional Mock Behavior
```go
func TestFileUpload_ConditionalBehavior(t *testing.T) {
    mockMinio := &MockMinIOService{}
    
    // Different behavior based on input
    mockMinio.On("GeneratePresignedUploadURL", 
        mock.MatchedBy(func(filename string) bool {
            return strings.Contains(filename, "large")
        }), 
        mock.AnythingOfType("time.Duration"),
    ).Return("http://large-file-url", nil)
    
    mockMinio.On("GeneratePresignedUploadURL", 
        mock.MatchedBy(func(filename string) bool {
            return !strings.Contains(filename, "large")
        }), 
        mock.AnythingOfType("time.Duration"),
    ).Return("http://regular-url", nil)
    
    service := &FileService{minioService: mockMinio}
    
    // Test large file
    url1, err := service.GetUploadURL("large_file.wav", 100*1024*1024)
    assert.NoError(t, err)
    assert.Contains(t, url1, "large-file-url")
    
    // Test regular file
    url2, err := service.GetUploadURL("regular.wav", 1024)
    assert.NoError(t, err)
    assert.Contains(t, url2, "regular-url")
    
    mockMinio.AssertExpectations(t)
}
```

#### Mock Side Effects
```go
func TestFileUpload_WithSideEffects(t *testing.T) {
    mockMinio := &MockMinIOService{}
    callCount := 0
    
    mockMinio.On("UploadFile", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
        callCount++
        filename := args.String(0)
        t.Logf("Upload called for file: %s (call #%d)", filename, callCount)
        
        // Simulate some side effect
        if callCount == 1 {
            // First call succeeds immediately
            return
        } else {
            // Subsequent calls take longer
            time.Sleep(100 * time.Millisecond)
        }
    }).Return(nil)
    
    service := &FileService{minioService: mockMinio}
    
    // Test multiple uploads
    err1 := service.UploadFile("file1.wav", []byte("data1"))
    err2 := service.UploadFile("file2.wav", []byte("data2"))
    
    assert.NoError(t, err1)
    assert.NoError(t, err2)
    assert.Equal(t, 2, callCount)
    
    mockMinio.AssertExpectations(t)
}
```

### Integration Test Environment Setup

```go
// integration/setup.go
package integration

import (
    "context"
    "fmt"
    "testing"
    "time"
    
    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
    "github.com/stretchr/testify/require"
    
    "sermon-uploader/config"
    "sermon-uploader/services"
)

type IntegrationEnv struct {
    MinioClient    *minio.Client
    FileService    *services.FileService
    Config         *config.Config
    BucketName     string
    Context        context.Context
    cleanup        func()
}

func SetupIntegrationTest(t *testing.T) *IntegrationEnv {
    // Skip if no integration environment available
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    ctx := context.Background()
    
    // Setup MinIO client
    endpoint := "localhost:9000"
    accessKey := "testuser"
    secretKey := "testpass123"
    
    minioClient, err := minio.New(endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
        Secure: false,
    })
    require.NoError(t, err, "Failed to create MinIO client")
    
    // Test connection
    _, err = minioClient.ListBuckets(ctx)
    if err != nil {
        t.Skipf("MinIO not available at %s: %v", endpoint, err)
    }
    
    // Create unique test bucket
    bucketName := fmt.Sprintf("test-sermons-%d", time.Now().UnixNano())
    err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
    require.NoError(t, err, "Failed to create test bucket")
    
    // Setup services
    cfg := &config.Config{
        MinioBucket:    bucketName,
        MinioEndpoint:  endpoint,
        MinioAccessKey: accessKey,
        MinioSecretKey: secretKey,
        MinioSecure:    false,
    }
    
    fileService := services.NewFileService(minioClient, cfg)
    
    // Cleanup function
    cleanup := func() {
        t.Log("Cleaning up integration test environment")
        
        // Remove all objects from bucket
        objectsCh := minioClient.ListObjects(ctx, bucketName, minio.ListObjectsOptions{Recursive: true})
        for object := range objectsCh {
            err := minioClient.RemoveObject(ctx, bucketName, object.Key, minio.RemoveObjectOptions{})
            if err != nil {
                t.Logf("Failed to remove object %s: %v", object.Key, err)
            }
        }
        
        // Remove bucket
        err := minioClient.RemoveBucket(ctx, bucketName)
        if err != nil {
            t.Logf("Failed to remove bucket %s: %v", bucketName, err)
        }
    }
    
    return &IntegrationEnv{
        MinioClient: minioClient,
        FileService: fileService,
        Config:      cfg,
        BucketName:  bucketName,
        Context:     ctx,
        cleanup:     cleanup,
    }
}

func (env *IntegrationEnv) Cleanup() {
    if env.cleanup != nil {
        env.cleanup()
    }
}
```

### Day 4-5 Assignment

Create integration tests for the file upload pipeline:
1. Test complete upload workflow with real MinIO
2. Test duplicate detection with actual files
3. Test large file upload optimization
4. Test error recovery and cleanup
5. Use both mocked unit tests and real integration tests
6. Achieve 100% coverage across both test types

---

## 🏆 Week 2+: Advanced Topics & Production Code

### Performance Testing

```go
func BenchmarkFileUpload_SmallFiles(b *testing.B) {
    env := setupBenchmarkEnvironment(b)
    defer env.cleanup()
    
    // Create test data
    fileData := testutil.CreateTestWAVFile(nil, "bench.wav", 1024) // 1KB
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        filename := fmt.Sprintf("bench_%d.wav", i)
        err := env.fileService.UploadFile(filename, fileData.Content)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkFileUpload_LargeFiles(b *testing.B) {
    env := setupBenchmarkEnvironment(b)
    defer env.cleanup()
    
    // Create large test data
    fileData := testutil.CreateTestWAVFile(nil, "large.wav", 10*1024*1024) // 10MB
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        filename := fmt.Sprintf("large_%d.wav", i)
        err := env.fileService.UploadFile(filename, fileData.Content)
        if err != nil {
            b.Fatal(err)
        }
    }
}

// Run benchmarks
// go test -bench=. -benchmem -run=^$ ./...
```

### Concurrent Testing

```go
func TestConcurrentUploads_ShouldNotConflict(t *testing.T) {
    env := setupIntegrationTest(t)
    defer env.cleanup()
    
    const numGoroutines = 10
    const fileSize = 1024
    
    var wg sync.WaitGroup
    errors := make(chan error, numGoroutines)
    
    // Launch concurrent uploads
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            
            filename := fmt.Sprintf("concurrent_%d.wav", id)
            fileData := testutil.CreateTestWAVFile(t, filename, fileSize)
            
            err := env.fileService.UploadFile(filename, fileData.Content)
            errors <- err
        }(i)
    }
    
    wg.Wait()
    close(errors)
    
    // Check all uploads succeeded
    successCount := 0
    for err := range errors {
        if err == nil {
            successCount++
        } else {
            t.Logf("Upload failed: %v", err)
        }
    }
    
    assert.Equal(t, numGoroutines, successCount, "All concurrent uploads should succeed")
    
    // Verify all files exist
    objects := env.minioClient.ListObjects(env.Context, env.BucketName, minio.ListObjectsOptions{})
    objectCount := 0
    for range objects {
        objectCount++
    }
    assert.Equal(t, numGoroutines, objectCount, "All files should be uploaded")
}
```

### Production Code Patterns

```go
// handlers/upload.go - Real production handler
func (h *Handlers) GetPresignedURL(c *fiber.Ctx) error {
    // Parse request
    var req struct {
        Filename string `json:"filename" validate:"required"`
        FileSize int64  `json:"fileSize" validate:"min=1"`
    }
    
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{
            "error":   true,
            "message": "Invalid request format",
        })
    }
    
    // Validate input
    if err := h.validator.Validate(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{
            "error":   true,
            "message": fmt.Sprintf("Validation failed: %v", err),
        })
    }
    
    // Check for duplicates
    isDuplicate, err := h.fileService.CheckDuplicate(req.Filename)
    if err != nil {
        h.logger.WithError(err).WithField("filename", req.Filename).Error("Failed to check for duplicates")
        return c.Status(500).JSON(fiber.Map{
            "error":   true,
            "message": "Failed to check for duplicates",
        })
    }
    
    if isDuplicate {
        return c.Status(409).JSON(fiber.Map{
            "error":       true,
            "isDuplicate": true,
            "message":     "File already exists",
            "filename":    req.Filename,
        })
    }
    
    // Generate presigned URL with smart routing
    presignedURL, isLargeFile, err := h.fileService.GenerateSmartUploadURL(req.Filename, req.FileSize, time.Hour)
    if err != nil {
        h.logger.WithError(err).WithFields(logrus.Fields{
            "filename": req.Filename,
            "fileSize": req.FileSize,
        }).Error("Failed to generate presigned URL")
        
        return c.Status(500).JSON(fiber.Map{
            "error":   true,
            "message": "Failed to generate upload URL",
        })
    }
    
    // Build response
    response := fiber.Map{
        "success":      true,
        "isDuplicate":  false,
        "uploadUrl":    presignedURL,
        "filename":     req.Filename,
        "fileSize":     req.FileSize,
        "expires":      time.Now().Add(time.Hour).Unix(),
        "isLargeFile":  isLargeFile,
        "uploadMethod": map[bool]string{true: "direct_minio", false: "cloudflare"}[isLargeFile],
    }
    
    // Add debugging info for large files
    if isLargeFile {
        response["largeFileThreshold"] = h.config.LargeFileThreshold
        response["message"] = fmt.Sprintf("Large file (%.1f MB) will use direct MinIO upload", 
            float64(req.FileSize)/(1024*1024))
    }
    
    // Log successful request
    h.logger.WithFields(logrus.Fields{
        "filename":     req.Filename,
        "fileSize":     req.FileSize,
        "isLargeFile":  isLargeFile,
        "uploadMethod": response["uploadMethod"],
    }).Info("Generated presigned URL")
    
    return c.JSON(response)
}
```

### Advanced Test Organization

```go
// tests/suites/upload_test.go
package suites

import (
    "testing"
    "github.com/stretchr/testify/suite"
)

type UploadTestSuite struct {
    suite.Suite
    env *IntegrationEnv
}

func (s *UploadTestSuite) SetupSuite() {
    s.env = setupIntegrationTest(s.T())
}

func (s *UploadTestSuite) TearDownSuite() {
    s.env.cleanup()
}

func (s *UploadTestSuite) SetupTest() {
    // Reset state before each test
}

func (s *UploadTestSuite) TestUploadWorkflow() {
    // Test cases using suite setup
}

func (s *UploadTestSuite) TestDuplicateDetection() {
    // Another test case
}

func TestUploadSuite(t *testing.T) {
    suite.Run(t, new(UploadTestSuite))
}
```

### Final Assignment: Production Feature

Implement a complete feature using TDD from start to finish:

**Feature**: Batch file upload with progress tracking
- Support uploading multiple files in a single request
- Track upload progress for each file
- Handle partial failures gracefully
- Send real-time updates via WebSocket
- Include comprehensive error handling
- Must have 100% test coverage
- Must include unit, integration, and performance tests

**Success Criteria**:
- [ ] All tests pass
- [ ] 100% test coverage achieved
- [ ] No linting issues
- [ ] Performance benchmarks within acceptable range
- [ ] Code review approved
- [ ] Integration tests pass with real services
- [ ] Feature works end-to-end in staging environment

---

## 🎓 Graduation Checklist

You've successfully completed TDD onboarding when you can:

### Technical Skills
- [ ] Write failing tests before implementing code
- [ ] Create comprehensive test suites with 100% coverage  
- [ ] Use mocks effectively for unit testing
- [ ] Write integration tests with real services
- [ ] Debug failing tests efficiently
- [ ] Write performance benchmarks
- [ ] Use table-driven tests for multiple scenarios

### Process Skills
- [ ] Follow Red-Green-Refactor cycle consistently
- [ ] Write descriptive test names and documentation
- [ ] Maintain tests as living documentation
- [ ] Use TDD to drive better code design
- [ ] Balance unit vs integration vs E2E testing
- [ ] Apply TDD in team code reviews

### Project Skills  
- [ ] Contribute to sermon uploader with confidence
- [ ] Understand project testing patterns
- [ ] Use project testing infrastructure
- [ ] Meet project quality standards
- [ ] Mentor other developers in TDD practices

## 📞 Getting Help

### Daily Standup
- Share TDD progress and blockers
- Ask for pairing on difficult tests
- Discuss testing approaches with team

### Code Review
- Always include test improvements in PRs
- Ask for feedback on test quality
- Learn from reviewing others' tests

### Team Resources
- TDD mentor assigned for first month
- Weekly TDD practice sessions
- Internal testing best practices wiki
- Team Slack channel: #tdd-help

### External Resources
- [Go Testing Documentation](https://pkg.go.dev/testing)
- [Testify Framework](https://github.com/stretchr/testify)
- [Clean Code Testing](https://blog.cleancoder.com/uncle-bob/2017/05/05/TestDefinitions.html)

---

Welcome to the team! Your journey to TDD mastery starts now. 🚀