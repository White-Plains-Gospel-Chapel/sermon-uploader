package services

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	
	"sermon-uploader/config"
)

// Test MinIO service creation
func TestNewMinIOService(t *testing.T) {
	cfg := &config.Config{
		MinIOEndpoint:  "localhost:9000",
		MinIOAccessKey: "testkey",
		MinIOSecretKey: "testsecret",
		MinIOSecure:    false,
		MinioBucket:    "test-bucket",
		IOBufferSize:   32768,
	}

	service := NewMinIOService(cfg)

	assert.NotNil(t, service)
	assert.NotNil(t, service.client)
	assert.Equal(t, cfg, service.config)
	assert.NotNil(t, service.pools)
	assert.NotNil(t, service.copier)
	assert.NotNil(t, service.metrics)
}

// Test file hash calculation
func TestCalculateFileHash(t *testing.T) {
	cfg := &config.Config{
		MinIOEndpoint:  "localhost:9000",
		MinIOAccessKey: "testkey",
		MinIOSecretKey: "testsecret",
		MinIOSecure:    false,
		MinioBucket:    "test-bucket",
		IOBufferSize:   32768,
	}

	service := NewMinIOService(cfg)
	testData := []byte("test data for hashing")

	hash1 := service.CalculateFileHash(testData)
	hash2 := service.CalculateFileHash(testData)

	// Hash should be consistent
	assert.Equal(t, hash1, hash2)
	assert.NotEmpty(t, hash1)
	assert.Len(t, hash1, 64) // SHA256 produces 64 character hex string
}

// Test filename renaming logic
func TestGetRenamedFilename(t *testing.T) {
	cfg := &config.Config{
		MinIOEndpoint:  "localhost:9000",
		MinIOAccessKey: "testkey",
		MinIOSecretKey: "testsecret",
		MinIOSecure:    false,
		MinioBucket:    "test-bucket",
		WAVSuffix:      "_raw",
		IOBufferSize:   32768,
	}

	service := NewMinIOService(cfg)

	tests := []struct {
		input    string
		expected string
	}{
		{"sermon.wav", "sermon_raw.wav"},
		{"test-file.wav", "test-file_raw.wav"},
		{"no-extension", "no-extension"},
		{"multiple.dots.wav", "multiple.dots_raw.wav"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := service.getRenamedFilename(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test metrics initialization and updates
func TestMetricsInitialization(t *testing.T) {
	cfg := &config.Config{
		MinIOEndpoint:  "localhost:9000",
		MinIOAccessKey: "testkey",
		MinIOSecretKey: "testsecret",
		MinIOSecure:    false,
		MinioBucket:    "test-bucket",
		IOBufferSize:   32768,
	}

	service := NewMinIOService(cfg)

	metrics := service.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, int64(0), metrics.ConnectionErrors)
	assert.Equal(t, int64(0), metrics.RetryCount)
	assert.Equal(t, int64(0), metrics.MultipartUploads)
}

// Test connection pool stats
func TestConnectionPoolStats(t *testing.T) {
	cfg := &config.Config{
		MinIOEndpoint:  "localhost:9000",
		MinIOAccessKey: "testkey",
		MinIOSecretKey: "testsecret",
		MinIOSecure:    false,
		MinioBucket:    "test-bucket",
		IOBufferSize:   32768,
	}

	service := NewMinIOService(cfg)

	stats := service.GetConnectionPoolStats()
	assert.NotNil(t, stats)
	
	// Initial stats should be zero or reasonable defaults
	assert.Contains(t, stats, "active")
	assert.Contains(t, stats, "idle")
	assert.Contains(t, stats, "total")
}

// Test retry configuration
func TestRetryConfiguration(t *testing.T) {
	cfg := &config.Config{
		MinIOEndpoint:  "localhost:9000",
		MinIOAccessKey: "testkey",
		MinIOSecretKey: "testsecret",
		MinIOSecure:    false,
		MinioBucket:    "test-bucket",
		IOBufferSize:   32768,
	}

	service := NewMinIOService(cfg)

	// Test retry config values (private, but we can test behavior)
	retryConfig := RetryConfig{
		MaxRetries:      3,
		InitialDelay:    1 * time.Second,
		MaxDelay:        30 * time.Second,
		BackoffFactor:   2.0,
		RetryableErrors: []string{"connection reset", "timeout", "temporary failure"},
	}

	assert.Equal(t, 3, retryConfig.MaxRetries)
	assert.Equal(t, 1*time.Second, retryConfig.InitialDelay)
	assert.Equal(t, 30*time.Second, retryConfig.MaxDelay)
	assert.Equal(t, 2.0, retryConfig.BackoffFactor)
	assert.Len(t, retryConfig.RetryableErrors, 3)
}

// Test progress reader functionality
func TestProgressReader(t *testing.T) {
	testData := []byte("test data for progress tracking")
	reader := bytes.NewReader(testData)
	
	var progressCalled bool
	var lastProgress int64
	
	progressReader := &ProgressReader{
		Reader: reader,
		Size:   int64(len(testData)),
		Callback: func(bytesTransferred int64) {
			progressCalled = true
			lastProgress = bytesTransferred
		},
	}

	// Read some data
	buf := make([]byte, 10)
	n, err := progressReader.Read(buf)

	assert.NoError(t, err)
	assert.Equal(t, 10, n)
	assert.True(t, progressCalled)
	assert.Equal(t, int64(10), lastProgress)
	assert.Equal(t, int64(10), progressReader.BytesRead)
}

// Test integrity result structure
func TestIntegrityResult(t *testing.T) {
	result := &IntegrityResult{
		Filename:        "test.wav",
		ExpectedHash:    "hash123",
		StoredHash:      "hash123",
		IntegrityPassed: true,
		FileSize:        1024,
		UploadTime:      time.Now(),
	}

	assert.Equal(t, "test.wav", result.Filename)
	assert.Equal(t, "hash123", result.ExpectedHash)
	assert.Equal(t, "hash123", result.StoredHash)
	assert.True(t, result.IntegrityPassed)
	assert.Equal(t, int64(1024), result.FileSize)
	assert.Empty(t, result.ErrorMessage)
}

// Test compression stats structure
func TestCompressionStats(t *testing.T) {
	stats := &CompressionStats{
		TotalFiles:           10,
		ZeroCompressionFiles: 8,
		BitPerfectFiles:      8,
		TotalSize:            1024 * 1024 * 100, // 100MB
		Files:                make([]FileCompressionInfo, 0),
	}

	assert.Equal(t, 10, stats.TotalFiles)
	assert.Equal(t, 8, stats.ZeroCompressionFiles)
	assert.Equal(t, 8, stats.BitPerfectFiles)
	assert.Equal(t, int64(1024*1024*100), stats.TotalSize)
	assert.Empty(t, stats.Files)
}

// Test file compression info
func TestFileCompressionInfo(t *testing.T) {
	info := FileCompressionInfo{
		Filename:          "test.wav",
		Size:              1024,
		ContentType:       "application/octet-stream",
		Compression:       "none",
		Quality:           "bit-perfect",
		IsZeroCompression: true,
		IsBitPerfect:      true,
		Hash:              "abcdef123456",
		UploadDate:        time.Now(),
	}

	assert.Equal(t, "test.wav", info.Filename)
	assert.Equal(t, int64(1024), info.Size)
	assert.Equal(t, "application/octet-stream", info.ContentType)
	assert.Equal(t, "none", info.Compression)
	assert.Equal(t, "bit-perfect", info.Quality)
	assert.True(t, info.IsZeroCompression)
	assert.True(t, info.IsBitPerfect)
	assert.Equal(t, "abcdef123456", info.Hash)
}

// Test multipart upload URLs structure
func TestMultipartUploadURLs(t *testing.T) {
	urls := &MultipartUploadURLs{
		UploadID:      "upload123",
		Bucket:        "test-bucket",
		ObjectName:    "test_raw.wav",
		OriginalName:  "test.wav",
		PartURLs:      make([]PartURL, 3),
		ExpiryMinutes: 60,
		CreatedAt:     time.Now(),
	}

	assert.Equal(t, "upload123", urls.UploadID)
	assert.Equal(t, "test-bucket", urls.Bucket)
	assert.Equal(t, "test_raw.wav", urls.ObjectName)
	assert.Equal(t, "test.wav", urls.OriginalName)
	assert.Len(t, urls.PartURLs, 3)
	assert.Equal(t, 60, urls.ExpiryMinutes)
}

// Test part URL structure
func TestPartURL(t *testing.T) {
	partURL := PartURL{
		PartNumber: 1,
		URL:        "https://minio.example.com/bucket/object?partNumber=1&uploadId=123",
	}

	assert.Equal(t, 1, partURL.PartNumber)
	assert.Contains(t, partURL.URL, "partNumber=1")
	assert.Contains(t, partURL.URL, "uploadId=123")
}

// Test completed part structure
func TestCompletedPart(t *testing.T) {
	part := CompletedPart{
		PartNumber: 1,
		ETag:       "\"etag123456\"",
	}

	assert.Equal(t, 1, part.PartNumber)
	assert.Equal(t, "\"etag123456\"", part.ETag)
}

// Benchmark hash calculation
func BenchmarkCalculateFileHash(b *testing.B) {
	cfg := &config.Config{
		MinIOEndpoint:  "localhost:9000",
		MinIOAccessKey: "testkey",
		MinIOSecretKey: "testsecret",
		MinIOSecure:    false,
		MinioBucket:    "test-bucket",
		IOBufferSize:   32768,
	}

	service := NewMinIOService(cfg)
	testData := make([]byte, 1024) // 1KB test data
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.CalculateFileHash(testData)
	}
}

// Benchmark progress reader
func BenchmarkProgressReader(b *testing.B) {
	testData := make([]byte, 1024) // 1KB test data
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(testData)
		progressReader := &ProgressReader{
			Reader: reader,
			Size:   int64(len(testData)),
			Callback: func(int64) {}, // No-op callback
		}
		
		buf := make([]byte, 256)
		for {
			_, err := progressReader.Read(buf)
			if err != nil {
				break
			}
		}
	}
}

// Test object path generation
func TestGetObjectPath(t *testing.T) {
	cfg := &config.Config{
		MinIOEndpoint:  "localhost:9000",
		MinIOAccessKey: "testkey",
		MinIOSecretKey: "testsecret",
		MinIOSecure:    false,
		MinioBucket:    "test-bucket",
		IOBufferSize:   32768,
	}

	service := NewMinIOService(cfg)
	
	// Test that object path is filename directly (no subfolders)
	filename := "test.wav"
	path := service.getObjectPath(filename)
	
	assert.Equal(t, filename, path)
}

// Test MinIO metrics thread safety
func TestMetricsThreadSafety(t *testing.T) {
	cfg := &config.Config{
		MinIOEndpoint:  "localhost:9000",
		MinIOAccessKey: "testkey",
		MinIOSecretKey: "testsecret",
		MinIOSecure:    false,
		MinioBucket:    "test-bucket",
		IOBufferSize:   32768,
	}

	service := NewMinIOService(cfg)
	
	done := make(chan bool, 10)
	
	// Run multiple goroutines accessing metrics
	for i := 0; i < 10; i++ {
		go func() {
			metrics := service.GetMetrics()
			assert.NotNil(t, metrics)
			done <- true
		}()
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Test upload metrics update
func TestUpdateUploadMetrics(t *testing.T) {
	cfg := &config.Config{
		MinIOEndpoint:  "localhost:9000",
		MinIOAccessKey: "testkey",
		MinIOSecretKey: "testsecret",
		MinIOSecure:    false,
		MinioBucket:    "test-bucket",
		IOBufferSize:   32768,
	}

	service := NewMinIOService(cfg)
	
	// Test updating upload metrics
	duration := 5 * time.Second
	service.updateUploadMetrics(duration, true)
	
	metrics := service.GetMetrics()
	assert.Equal(t, duration, metrics.UploadLatency)
	assert.Equal(t, int64(1), metrics.MultipartUploads)
	
	// Test non-multipart upload
	service.updateUploadMetrics(duration, false)
	metrics = service.GetMetrics()
	assert.Equal(t, int64(1), metrics.MultipartUploads) // Should remain 1
}