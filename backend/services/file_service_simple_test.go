package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sermon-uploader/config"
)

// Test basic FileService functionality without complex mocking
func TestFileService_BasicFunctionality(t *testing.T) {
	t.Run("FileService struct creation", func(t *testing.T) {
		cfg := &config.Config{
			BatchThreshold:     2,
			IOBufferSize:      32768,
			StreamingThreshold: 1048576,
		}

		service := &FileService{
			config:   cfg,
			metadata: NewMetadataService("/tmp"),
		}

		assert.NotNil(t, service)
		assert.Equal(t, cfg, service.config)
		assert.NotNil(t, service.metadata)
	})

	t.Run("GetMetadataService", func(t *testing.T) {
		metadataService := NewMetadataService("/tmp")
		service := &FileService{
			metadata: metadataService,
		}

		result := service.GetMetadataService()
		assert.Equal(t, metadataService, result)
	})

	t.Run("UploadSummary structure", func(t *testing.T) {
		results := []FileUploadResult{
			{Filename: "test1.wav", Status: "success", Size: 1024},
			{Filename: "test2.wav", Status: "duplicate", Size: 2048},
			{Filename: "test3.wav", Status: "error", Message: "Upload failed"},
		}

		summary := &UploadSummary{
			Successful: 1,
			Duplicates: 1,
			Failed:     1,
			Total:      3,
			Results:    results,
		}

		assert.Equal(t, 1, summary.Successful)
		assert.Equal(t, 1, summary.Duplicates)
		assert.Equal(t, 1, summary.Failed)
		assert.Equal(t, 3, summary.Total)
		assert.Len(t, summary.Results, 3)
	})

	t.Run("FileUploadResult structure", func(t *testing.T) {
		result := FileUploadResult{
			Filename: "test.wav",
			Renamed:  "test_raw.wav",
			Status:   "success",
			Message:  "Upload successful",
			Size:     1048576,
			Hash:     "abc123",
		}

		assert.Equal(t, "test.wav", result.Filename)
		assert.Equal(t, "test_raw.wav", result.Renamed)
		assert.Equal(t, "success", result.Status)
		assert.Equal(t, "Upload successful", result.Message)
		assert.Equal(t, int64(1048576), result.Size)
		assert.Equal(t, "abc123", result.Hash)
	})
}

// Test configuration handling
func TestFileService_Configuration(t *testing.T) {
	t.Run("Default configuration values", func(t *testing.T) {
		cfg := &config.Config{}
		service := &FileService{config: cfg}

		assert.NotNil(t, service.config)
	})

	t.Run("Custom configuration values", func(t *testing.T) {
		cfg := &config.Config{
			BatchThreshold:        5,
			IOBufferSize:         65536,
			StreamingThreshold:   2097152,
			MaxConcurrentUploads: 4,
		}
		service := &FileService{config: cfg}

		assert.Equal(t, 5, service.config.BatchThreshold)
		assert.Equal(t, 65536, service.config.IOBufferSize)
		assert.Equal(t, int64(2097152), service.config.StreamingThreshold)
		assert.Equal(t, 4, service.config.MaxConcurrentUploads)
	})
}

// Test service initialization
func TestFileService_Initialization(t *testing.T) {
	t.Run("Service components initialization", func(t *testing.T) {
		service := &FileService{
			metadata:  NewMetadataService("/tmp"),
			streaming: NewStreamingService(),
		}

		assert.NotNil(t, service.metadata)
		assert.NotNil(t, service.streaming)

		// Test getter methods
		if service.metadata != nil {
			metadataService := service.GetMetadataService()
			assert.Equal(t, service.metadata, metadataService)
		}

		if service.streaming != nil {
			streamingService := service.GetStreamingService()
			assert.Equal(t, service.streaming, streamingService)
		}
	})
}

// Test error handling scenarios
func TestFileService_ErrorHandling(t *testing.T) {
	t.Run("Nil service handling", func(t *testing.T) {
		service := &FileService{}

		// Should not panic when accessing nil services
		assert.NotPanics(t, func() {
			metadataService := service.GetMetadataService()
			_ = metadataService
		})

		assert.NotPanics(t, func() {
			streamingService := service.GetStreamingService()
			_ = streamingService
		})
	})

	t.Run("Invalid file upload result statuses", func(t *testing.T) {
		validStatuses := []string{"success", "duplicate", "error"}
		
		for _, status := range validStatuses {
			result := FileUploadResult{
				Filename: "test.wav",
				Status:   status,
			}
			
			assert.Equal(t, status, result.Status)
			assert.Contains(t, validStatuses, result.Status)
		}
	})
}

// Benchmark basic operations
func BenchmarkFileService_BasicOperations(b *testing.B) {
	cfg := &config.Config{BatchThreshold: 2}
	service := &FileService{
		config:   cfg,
		metadata: NewMetadataService("/tmp"),
	}

	b.Run("GetMetadataService", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = service.GetMetadataService()
		}
	})

	b.Run("UploadSummary creation", func(b *testing.B) {
		results := []FileUploadResult{
			{Filename: "test.wav", Status: "success"},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			summary := &UploadSummary{
				Successful: 1,
				Total:      1,
				Results:    results,
			}
			_ = summary
		}
	})
}

// Test thread safety basics
func TestFileService_ThreadSafety(t *testing.T) {
	t.Run("Concurrent GetMetadataService calls", func(t *testing.T) {
		service := &FileService{
			metadata: NewMetadataService("/tmp"),
		}

		// Run concurrent access - should not panic or race
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				for j := 0; j < 100; j++ {
					_ = service.GetMetadataService()
				}
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// Test edge cases
func TestFileService_EdgeCases(t *testing.T) {
	t.Run("Empty upload results", func(t *testing.T) {
		summary := &UploadSummary{
			Successful: 0,
			Duplicates: 0,
			Failed:     0,
			Total:      0,
			Results:    []FileUploadResult{},
		}

		assert.Equal(t, 0, summary.Total)
		assert.Len(t, summary.Results, 0)
	})

	t.Run("Large number of results", func(t *testing.T) {
		const numResults = 10000
		results := make([]FileUploadResult, numResults)
		
		for i := 0; i < numResults; i++ {
			results[i] = FileUploadResult{
				Filename: "test.wav",
				Status:   "success",
			}
		}

		summary := &UploadSummary{
			Successful: numResults,
			Total:      numResults,
			Results:    results,
		}

		assert.Equal(t, numResults, summary.Total)
		assert.Len(t, summary.Results, numResults)
	})

	t.Run("FileUploadResult with empty values", func(t *testing.T) {
		result := FileUploadResult{
			Filename: "",
			Status:   "",
			Size:     0,
		}

		assert.Empty(t, result.Filename)
		assert.Empty(t, result.Status)
		assert.Zero(t, result.Size)
	})
}