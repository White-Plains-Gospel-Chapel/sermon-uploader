package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sermon-uploader/services"
)

func TestMinIOServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx := context.Background()
	
	// Setup test environment
	env, err := SetupTestEnvironment(ctx)
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Create MinIO service with test configuration
	cfg := env.GetAppConfig()
	minioService := services.NewMinIOService(cfg)

	t.Run("TestConnection", func(t *testing.T) {
		err := minioService.TestConnection()
		assert.NoError(t, err, "MinIO connection should succeed")
	})

	t.Run("EnsureBucketExists", func(t *testing.T) {
		err := minioService.EnsureBucketExists()
		assert.NoError(t, err, "Bucket creation should succeed")
		
		// Test idempotency - calling again should not fail
		err = minioService.EnsureBucketExists()
		assert.NoError(t, err, "Bucket creation should be idempotent")
	})

	t.Run("UploadAndDownloadFile", func(t *testing.T) {
		// Prepare test data
		originalFilename := "test-sermon.wav"
		testData, err := CreateTestFile(originalFilename, 1024*1024) // 1MB test file
		require.NoError(t, err, "Failed to create test file")
		
		// Upload file
		metadata, err := minioService.UploadFile(testData, originalFilename)
		require.NoError(t, err, "File upload should succeed")
		
		// Verify metadata
		assert.Equal(t, originalFilename, metadata.OriginalFilename)
		assert.Equal(t, "test-sermon_raw.wav", metadata.RenamedFilename)
		assert.Equal(t, int64(len(testData)), metadata.FileSize)
		assert.Equal(t, "uploaded", metadata.ProcessingStatus)
		assert.NotEmpty(t, metadata.FileHash)
		
		// Verify hash calculation
		expectedHash := fmt.Sprintf("%x", sha256.Sum256(testData))
		assert.Equal(t, expectedHash, metadata.FileHash)
		
		// Download and verify file data
		downloadedData, err := minioService.DownloadFileData(metadata.RenamedFilename)
		require.NoError(t, err, "File download should succeed")
		
		assert.Equal(t, testData, downloadedData, "Downloaded data should match uploaded data")
	})

	t.Run("StreamingUpload", func(t *testing.T) {
		// Prepare larger test data for streaming
		originalFilename := "test-large-sermon.wav"
		testData, err := CreateTestFile(originalFilename, 5*1024*1024) // 5MB test file
		require.NoError(t, err, "Failed to create large test file")
		
		// Calculate hash
		expectedHash := fmt.Sprintf("%x", sha256.Sum256(testData))
		
		// Create reader
		reader := bytes.NewReader(testData)
		
		// Upload via streaming
		metadata, err := minioService.UploadFileStreaming(reader, originalFilename, int64(len(testData)), expectedHash)
		require.NoError(t, err, "Streaming upload should succeed")
		
		// Verify metadata
		assert.Equal(t, originalFilename, metadata.OriginalFilename)
		assert.Equal(t, int64(len(testData)), metadata.FileSize)
		assert.Equal(t, expectedHash, metadata.FileHash)
		
		// Verify file exists and data is correct
		downloadedData, err := minioService.DownloadFileData(metadata.RenamedFilename)
		require.NoError(t, err, "File download should succeed")
		assert.Equal(t, testData, downloadedData, "Downloaded data should match uploaded data")
	})

	t.Run("StreamingUploadWithProgress", func(t *testing.T) {
		originalFilename := "test-progress-sermon.wav"
		testData, err := CreateTestFile(originalFilename, 2*1024*1024) // 2MB test file
		require.NoError(t, err, "Failed to create test file")
		
		expectedHash := fmt.Sprintf("%x", sha256.Sum256(testData))
		reader := bytes.NewReader(testData)
		
		// Track progress
		var progressCallbacks []int64
		progressCallback := func(bytesTransferred int64) {
			progressCallbacks = append(progressCallbacks, bytesTransferred)
		}
		
		metadata, err := minioService.UploadFileStreamingWithProgress(reader, originalFilename, int64(len(testData)), expectedHash, progressCallback)
		require.NoError(t, err, "Streaming upload with progress should succeed")
		
		// Verify progress tracking
		assert.NotEmpty(t, progressCallbacks, "Progress callbacks should have been called")
		assert.Equal(t, int64(len(testData)), progressCallbacks[len(progressCallbacks)-1], "Final progress should equal file size")
		
		// Verify upload
		assert.Equal(t, originalFilename, metadata.OriginalFilename)
		assert.Equal(t, expectedHash, metadata.FileHash)
	})

	t.Run("UploadIntegrityVerification", func(t *testing.T) {
		originalFilename := "test-integrity.wav"
		testData, err := CreateTestFile(originalFilename, 512*1024) // 512KB test file
		require.NoError(t, err, "Failed to create test file")
		
		expectedHash := fmt.Sprintf("%x", sha256.Sum256(testData))
		
		// Upload file
		metadata, err := minioService.UploadFile(testData, originalFilename)
		require.NoError(t, err, "File upload should succeed")
		
		// Verify integrity
		result, err := minioService.VerifyUploadIntegrity(metadata.RenamedFilename, expectedHash)
		require.NoError(t, err, "Integrity verification should succeed")
		
		assert.True(t, result.IntegrityPassed, "Integrity check should pass")
		assert.Equal(t, expectedHash, result.ExpectedHash)
		assert.Equal(t, expectedHash, result.StoredHash)
		assert.Equal(t, metadata.RenamedFilename, result.Filename)
		assert.Empty(t, result.ErrorMessage)
	})

	t.Run("UploadIntegrityVerificationFailure", func(t *testing.T) {
		originalFilename := "test-integrity-fail.wav"
		testData, err := CreateTestFile(originalFilename, 512*1024)
		require.NoError(t, err, "Failed to create test file")
		
		// Upload file
		metadata, err := minioService.UploadFile(testData, originalFilename)
		require.NoError(t, err, "File upload should succeed")
		
		// Verify with wrong hash
		wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
		result, err := minioService.VerifyUploadIntegrity(metadata.RenamedFilename, wrongHash)
		require.NoError(t, err, "Integrity verification should complete")
		
		assert.False(t, result.IntegrityPassed, "Integrity check should fail with wrong hash")
		assert.Equal(t, wrongHash, result.ExpectedHash)
		assert.NotEqual(t, wrongHash, result.StoredHash)
		assert.NotEmpty(t, result.ErrorMessage)
	})

	t.Run("DuplicateDetection", func(t *testing.T) {
		originalFilename := "duplicate-test.wav"
		testData, err := CreateTestFile(originalFilename, 256*1024)
		require.NoError(t, err, "Failed to create test file")
		
		// Upload file first time
		metadata1, err := minioService.UploadFile(testData, originalFilename)
		require.NoError(t, err, "First upload should succeed")
		
		// Get existing hashes
		existingHashes, err := minioService.GetExistingHashes()
		require.NoError(t, err, "Getting existing hashes should succeed")
		
		// Check if our file hash is in the existing hashes
		assert.True(t, existingHashes[metadata1.FileHash], "Uploaded file hash should be in existing hashes")
		
		// Upload same file again (would be a duplicate in real scenario)
		metadata2, err := minioService.UploadFile(testData, "duplicate-test-2.wav")
		require.NoError(t, err, "Second upload should succeed")
		
		// Hashes should be the same
		assert.Equal(t, metadata1.FileHash, metadata2.FileHash, "Duplicate files should have same hash")
	})

	t.Run("ListFiles", func(t *testing.T) {
		// Upload several test files
		testFiles := []string{"list-test-1.wav", "list-test-2.wav", "list-test-3.wav"}
		uploadedFiles := make(map[string]*services.FileMetadata)
		
		for _, filename := range testFiles {
			testData, err := CreateTestFile(filename, 100*1024) // 100KB each
			require.NoError(t, err, "Failed to create test file")
			
			metadata, err := minioService.UploadFile(testData, filename)
			require.NoError(t, err, "Upload should succeed")
			uploadedFiles[metadata.RenamedFilename] = metadata
		}
		
		// List files
		files, err := minioService.ListFiles()
		require.NoError(t, err, "Listing files should succeed")
		
		// Verify we have at least our uploaded files
		assert.GreaterOrEqual(t, len(files), len(testFiles), "Should have at least the uploaded files")
		
		// Check if our files are in the list
		fileNames := make(map[string]bool)
		for _, file := range files {
			fileNames[file["name"].(string)] = true
		}
		
		for renamedFilename := range uploadedFiles {
			assert.True(t, fileNames[renamedFilename], "Uploaded file should be in list: %s", renamedFilename)
		}
	})

	t.Run("GetFileCount", func(t *testing.T) {
		// Get initial count
		initialCount, err := minioService.GetFileCount()
		require.NoError(t, err, "Getting file count should succeed")
		
		// Upload a new file
		testData, err := CreateTestFile("count-test.wav", 50*1024)
		require.NoError(t, err, "Failed to create test file")
		
		_, err = minioService.UploadFile(testData, "count-test.wav")
		require.NoError(t, err, "Upload should succeed")
		
		// Get new count
		newCount, err := minioService.GetFileCount()
		require.NoError(t, err, "Getting file count should succeed")
		
		assert.Equal(t, initialCount+1, newCount, "File count should increase by 1")
	})

	t.Run("CompressionStats", func(t *testing.T) {
		// Upload a file with streaming (should be bit-perfect)
		originalFilename := "compression-test.wav"
		testData, err := CreateTestFile(originalFilename, 1024*1024)
		require.NoError(t, err, "Failed to create test file")
		
		expectedHash := fmt.Sprintf("%x", sha256.Sum256(testData))
		reader := bytes.NewReader(testData)
		
		metadata, err := minioService.UploadFileStreaming(reader, originalFilename, int64(len(testData)), expectedHash)
		require.NoError(t, err, "Streaming upload should succeed")
		
		// Get compression stats
		stats, err := minioService.GetZeroCompressionStats()
		require.NoError(t, err, "Getting compression stats should succeed")
		
		assert.Greater(t, stats.TotalFiles, 0, "Should have at least one file")
		
		// Find our file in the stats
		var ourFile *services.FileCompressionInfo
		for i := range stats.Files {
			if stats.Files[i].Filename == metadata.RenamedFilename {
				ourFile = &stats.Files[i]
				break
			}
		}
		
		require.NotNil(t, ourFile, "Our uploaded file should be in compression stats")
		assert.True(t, ourFile.IsZeroCompression, "Streaming uploaded file should be zero compression")
		assert.True(t, ourFile.IsBitPerfect, "Streaming uploaded file should be bit perfect")
		assert.Equal(t, expectedHash, ourFile.Hash)
	})

	t.Run("PresignedURLGeneration", func(t *testing.T) {
		filename := "presigned-test.wav"
		
		// Generate presigned PUT URL
		putURL, err := minioService.GeneratePresignedPutURL(filename, 60) // 60 minutes
		require.NoError(t, err, "Generating presigned PUT URL should succeed")
		assert.NotEmpty(t, putURL, "Presigned PUT URL should not be empty")
		assert.Contains(t, putURL, "presigned-test_raw.wav", "URL should contain renamed filename")
		
		// Upload a file first for GET URL test
		testData, err := CreateTestFile(filename, 64*1024)
		require.NoError(t, err, "Failed to create test file")
		
		metadata, err := minioService.UploadFile(testData, filename)
		require.NoError(t, err, "File upload should succeed")
		
		// Generate presigned GET URL
		getURL, err := minioService.GeneratePresignedGetURL(metadata.RenamedFilename, 24) // 24 hours
		require.NoError(t, err, "Generating presigned GET URL should succeed")
		assert.NotEmpty(t, getURL, "Presigned GET URL should not be empty")
		assert.Contains(t, getURL, metadata.RenamedFilename, "URL should contain filename")
	})

	t.Run("MultipartUploadWorkflow", func(t *testing.T) {
		filename := "multipart-test.wav"
		parts := 3
		
		// Generate multipart upload URLs
		multipartURLs, err := minioService.GeneratePresignedMultipartURLs(filename, parts, 60)
		require.NoError(t, err, "Generating multipart URLs should succeed")
		
		assert.Equal(t, parts, len(multipartURLs.PartURLs), "Should have correct number of part URLs")
		assert.NotEmpty(t, multipartURLs.UploadID, "Upload ID should not be empty")
		assert.Equal(t, "multipart-test_raw.wav", multipartURLs.ObjectName)
		assert.Equal(t, filename, multipartURLs.OriginalName)
		
		// Verify each part URL
		for i, partURL := range multipartURLs.PartURLs {
			assert.Equal(t, i+1, partURL.PartNumber, "Part number should be correct")
			assert.NotEmpty(t, partURL.URL, "Part URL should not be empty")
			assert.Contains(t, partURL.URL, fmt.Sprintf("partNumber=%d", i+1), "URL should contain part number")
			assert.Contains(t, partURL.URL, "uploadId=", "URL should contain upload ID")
		}
	})

	t.Run("ConnectionPoolStats", func(t *testing.T) {
		stats := minioService.GetConnectionPoolStats()
		
		assert.Contains(t, stats, "active", "Stats should contain active connections")
		assert.Contains(t, stats, "idle", "Stats should contain idle connections") 
		assert.Contains(t, stats, "total", "Stats should contain total connections")
		
		// Values should be non-negative
		assert.GreaterOrEqual(t, stats["active"], int64(0), "Active connections should be non-negative")
		assert.GreaterOrEqual(t, stats["idle"], int64(0), "Idle connections should be non-negative")
		assert.GreaterOrEqual(t, stats["total"], int64(0), "Total connections should be non-negative")
	})

	t.Run("ServiceMetrics", func(t *testing.T) {
		// Upload a file to generate some metrics
		testData, err := CreateTestFile("metrics-test.wav", 256*1024)
		require.NoError(t, err, "Failed to create test file")
		
		_, err = minioService.UploadFile(testData, "metrics-test.wav")
		require.NoError(t, err, "File upload should succeed")
		
		// Get metrics
		metrics := minioService.GetMetrics()
		require.NotNil(t, metrics, "Metrics should not be nil")
		
		// Check that metrics are being tracked
		assert.GreaterOrEqual(t, metrics.UploadLatency, time.Duration(0), "Upload latency should be non-negative")
		assert.GreaterOrEqual(t, metrics.ConnectionErrors, int64(0), "Connection errors should be non-negative")
		assert.GreaterOrEqual(t, metrics.RetryCount, int64(0), "Retry count should be non-negative")
	})
}

// Benchmark tests for MinIO operations
func BenchmarkMinIOOperations(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmarks in short mode")
	}

	ctx := context.Background()
	env, err := SetupTestEnvironment(ctx)
	if err != nil {
		b.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	cfg := env.GetAppConfig()
	minioService := services.NewMinIOService(cfg)

	b.Run("Upload1KB", func(b *testing.B) {
		testData, _ := CreateTestFile("bench-1kb.wav", 1024)
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			filename := fmt.Sprintf("bench-1kb-%d.wav", i)
			_, err := minioService.UploadFile(testData, filename)
			if err != nil {
				b.Fatalf("Upload failed: %v", err)
			}
		}
	})

	b.Run("Upload1MB", func(b *testing.B) {
		testData, _ := CreateTestFile("bench-1mb.wav", 1024*1024)
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			filename := fmt.Sprintf("bench-1mb-%d.wav", i)
			reader := bytes.NewReader(testData)
			expectedHash := fmt.Sprintf("%x", sha256.Sum256(testData))
			_, err := minioService.UploadFileStreaming(reader, filename, int64(len(testData)), expectedHash)
			if err != nil {
				b.Fatalf("Streaming upload failed: %v", err)
			}
		}
	})

	b.Run("Download1MB", func(b *testing.B) {
		// Setup: upload a file first
		testData, _ := CreateTestFile("bench-download.wav", 1024*1024)
		metadata, err := minioService.UploadFile(testData, "bench-download.wav")
		if err != nil {
			b.Fatalf("Setup upload failed: %v", err)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := minioService.DownloadFileData(metadata.RenamedFilename)
			if err != nil {
				b.Fatalf("Download failed: %v", err)
			}
		}
	})

	b.Run("HashCalculation", func(b *testing.B) {
		testData, _ := CreateTestFile("bench-hash.wav", 1024*1024)
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			_ = minioService.CalculateFileHash(testData)
		}
	})
}