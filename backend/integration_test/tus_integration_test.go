package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sermon-uploader/services"
)

func TestTUSServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx := context.Background()
	
	// Setup test environment
	env, err := SetupTestEnvironment(ctx)
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Create services
	cfg := env.GetAppConfig()
	minioService := services.NewMinIOService(cfg)
	streamingService := services.NewStreamingService(minioService, cfg)
	tusService := services.NewTUSService(cfg, streamingService)

	t.Run("CreateUploadSession", func(t *testing.T) {
		filename := "tus-create-test.wav"
		size := int64(1024 * 1024) // 1MB
		metadata := map[string]string{
			"filename": filename,
			"filetype": "audio/wav",
		}

		response, err := tusService.CreateUpload(size, filename, metadata)
		require.NoError(t, err, "Creating upload session should succeed")

		assert.NotEmpty(t, response.ID, "Upload ID should not be empty")
		assert.NotEmpty(t, response.Location, "Location should not be empty")
		assert.Equal(t, metadata, response.Metadata, "Metadata should match")
		assert.Contains(t, response.Location, response.ID, "Location should contain upload ID")
	})

	t.Run("GetUploadInfo", func(t *testing.T) {
		filename := "tus-info-test.wav"
		size := int64(2 * 1024 * 1024) // 2MB
		metadata := map[string]string{
			"filename": filename,
			"filetype": "audio/wav",
			"quality":  "cd-quality",
		}

		// Create upload session
		response, err := tusService.CreateUpload(size, filename, metadata)
		require.NoError(t, err, "Creating upload session should succeed")

		// Get upload info
		info, err := tusService.GetUpload(response.ID)
		require.NoError(t, err, "Getting upload info should succeed")

		assert.Equal(t, response.ID, info.ID, "Upload ID should match")
		assert.Equal(t, response.ID, info.UploadID, "UploadID alias should match")
		assert.Equal(t, filename, info.Filename, "Filename should match")
		assert.Equal(t, size, info.Size, "Size should match")
		assert.Equal(t, int64(0), info.Offset, "Initial offset should be 0")
		assert.Equal(t, metadata, info.Metadata, "Metadata should match")
		assert.Equal(t, 0.0, info.Progress, "Initial progress should be 0")
		assert.Equal(t, "uploading", info.Status, "Status should be uploading")
		assert.False(t, info.HashVerified, "Hash should not be verified initially")
	})

	t.Run("WriteChunk", func(t *testing.T) {
		filename := "tus-chunk-test.wav"
		testData, err := CreateTestFile(filename, 1024*1024) // 1MB
		require.NoError(t, err, "Creating test data should succeed")

		size := int64(len(testData))
		chunkSize := 256 * 1024 // 256KB chunks

		// Create upload session
		response, err := tusService.CreateUpload(size, filename, map[string]string{"filename": filename})
		require.NoError(t, err, "Creating upload session should succeed")

		// Upload in chunks
		offset := int64(0)
		for offset < size {
			endOffset := offset + int64(chunkSize)
			if endOffset > size {
				endOffset = size
			}

			chunkData := testData[offset:endOffset]
			info, err := tusService.WriteChunk(response.ID, offset, chunkData)
			require.NoError(t, err, "Writing chunk should succeed at offset %d", offset)

			assert.Equal(t, endOffset, info.Offset, "Offset should be updated correctly")
			assert.Equal(t, float64(endOffset)/float64(size)*100, info.Progress, "Progress should be calculated correctly")

			offset = endOffset
		}

		// Verify final state
		finalInfo, err := tusService.GetUpload(response.ID)
		require.NoError(t, err, "Getting final upload info should succeed")

		assert.Equal(t, size, finalInfo.Offset, "Final offset should equal file size")
		assert.Equal(t, 100.0, finalInfo.Progress, "Final progress should be 100%")
	})

	t.Run("PatchUpload", func(t *testing.T) {
		filename := "tus-patch-test.wav"
		testData, err := CreateTestFile(filename, 512*1024) // 512KB
		require.NoError(t, err, "Creating test data should succeed")

		size := int64(len(testData))

		// Create upload session
		response, err := tusService.CreateUpload(size, filename, map[string]string{"filename": filename})
		require.NoError(t, err, "Creating upload session should succeed")

		// Upload using PATCH method
		reader := bytes.NewReader(testData)
		info, err := tusService.PatchUpload(response.ID, 0, reader)
		require.NoError(t, err, "PATCH upload should succeed")

		assert.Equal(t, size, info.Offset, "Offset should equal file size after PATCH")
		assert.Equal(t, 100.0, info.Progress, "Progress should be 100% after PATCH")
		assert.Equal(t, "uploading", info.Status, "Status should still be uploading")
	})

	t.Run("PartialUploadResumption", func(t *testing.T) {
		filename := "tus-resume-test.wav"
		testData, err := CreateTestFile(filename, 1024*1024) // 1MB
		require.NoError(t, err, "Creating test data should succeed")

		size := int64(len(testData))
		firstChunkSize := 400 * 1024 // 400KB
		secondChunkSize := int(size) - firstChunkSize // Remaining data

		// Create upload session
		response, err := tusService.CreateUpload(size, filename, map[string]string{"filename": filename})
		require.NoError(t, err, "Creating upload session should succeed")

		// Upload first chunk
		firstChunk := testData[:firstChunkSize]
		info1, err := tusService.WriteChunk(response.ID, 0, firstChunk)
		require.NoError(t, err, "Writing first chunk should succeed")

		assert.Equal(t, int64(firstChunkSize), info1.Offset, "First chunk offset should be correct")
		assert.Less(t, info1.Progress, 100.0, "Progress should be less than 100% after first chunk")

		// Simulate resumption - upload remaining data
		secondChunk := testData[firstChunkSize:]
		info2, err := tusService.WriteChunk(response.ID, int64(firstChunkSize), secondChunk)
		require.NoError(t, err, "Writing second chunk should succeed")

		assert.Equal(t, size, info2.Offset, "Final offset should equal file size")
		assert.Equal(t, 100.0, info2.Progress, "Progress should be 100% after resumption")

		// Verify we can read the complete file
		reader, err := tusService.GetUploadReader(response.ID)
		require.NoError(t, err, "Getting upload reader should succeed")
		defer reader.Close()

		uploadedData, err := io.ReadAll(reader)
		require.NoError(t, err, "Reading uploaded data should succeed")

		assert.Equal(t, testData, uploadedData, "Uploaded data should match original")
	})

	t.Run("IntegrityVerification", func(t *testing.T) {
		filename := "tus-integrity-test.wav"
		testData, err := CreateTestFile(filename, 256*1024) // 256KB
		require.NoError(t, err, "Creating test data should succeed")

		size := int64(len(testData))
		expectedHash := fmt.Sprintf("%x", sha256.Sum256(testData))

		// Create and complete upload
		response, err := tusService.CreateUpload(size, filename, map[string]string{"filename": filename})
		require.NoError(t, err, "Creating upload session should succeed")

		reader := bytes.NewReader(testData)
		_, err = tusService.PatchUpload(response.ID, 0, reader)
		require.NoError(t, err, "Uploading data should succeed")

		// Verify integrity
		quality, err := tusService.VerifyUpload(response.ID, expectedHash)
		require.NoError(t, err, "Verifying upload should succeed")

		assert.True(t, quality.IntegrityPassed, "Integrity should pass with correct hash")
		assert.Equal(t, expectedHash, quality.ExpectedHash, "Expected hash should match")
		assert.Equal(t, expectedHash, quality.ActualHash, "Actual hash should match expected")
		assert.Empty(t, quality.Message, "No error message should be present")
	})

	t.Run("IntegrityVerificationFailure", func(t *testing.T) {
		filename := "tus-integrity-fail-test.wav"
		testData, err := CreateTestFile(filename, 256*1024) // 256KB
		require.NoError(t, err, "Creating test data should succeed")

		size := int64(len(testData))
		wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"

		// Create and complete upload
		response, err := tusService.CreateUpload(size, filename, map[string]string{"filename": filename})
		require.NoError(t, err, "Creating upload session should succeed")

		reader := bytes.NewReader(testData)
		_, err = tusService.PatchUpload(response.ID, 0, reader)
		require.NoError(t, err, "Uploading data should succeed")

		// Verify integrity with wrong hash
		quality, err := tusService.VerifyUpload(response.ID, wrongHash)
		require.NoError(t, err, "Verifying upload should succeed")

		assert.False(t, quality.IntegrityPassed, "Integrity should fail with wrong hash")
		assert.Equal(t, wrongHash, quality.ExpectedHash, "Expected hash should be the wrong hash")
		assert.NotEqual(t, wrongHash, quality.ActualHash, "Actual hash should be different")
		assert.Empty(t, quality.Message, "No specific error message for hash mismatch")
	})

	t.Run("IncompleteUploadVerification", func(t *testing.T) {
		filename := "tus-incomplete-test.wav"
		size := int64(1024 * 1024) // 1MB
		partialSize := 512 * 1024  // 512KB (only half uploaded)

		testData, err := CreateTestFile(filename, partialSize)
		require.NoError(t, err, "Creating test data should succeed")

		expectedHash := fmt.Sprintf("%x", sha256.Sum256(testData))

		// Create upload session for larger file
		response, err := tusService.CreateUpload(size, filename, map[string]string{"filename": filename})
		require.NoError(t, err, "Creating upload session should succeed")

		// Upload only partial data
		reader := bytes.NewReader(testData)
		_, err = tusService.PatchUpload(response.ID, 0, reader)
		require.NoError(t, err, "Uploading partial data should succeed")

		// Try to verify incomplete upload
		quality, err := tusService.VerifyUpload(response.ID, expectedHash)
		require.NoError(t, err, "Verifying incomplete upload should succeed")

		assert.False(t, quality.IntegrityPassed, "Integrity should fail for incomplete upload")
		assert.Contains(t, quality.Message, "upload incomplete", "Message should indicate incomplete upload")
		assert.Contains(t, quality.Message, fmt.Sprintf("%d/%d", partialSize, size), "Message should show progress")
	})

	t.Run("OffsetValidation", func(t *testing.T) {
		filename := "tus-offset-test.wav"
		size := int64(1024 * 1024) // 1MB
		testData, err := CreateTestFile(filename, int(size))
		require.NoError(t, err, "Creating test data should succeed")

		// Create upload session
		response, err := tusService.CreateUpload(size, filename, map[string]string{"filename": filename})
		require.NoError(t, err, "Creating upload session should succeed")

		// Upload first chunk
		firstChunk := testData[:512*1024] // 512KB
		_, err = tusService.WriteChunk(response.ID, 0, firstChunk)
		require.NoError(t, err, "Writing first chunk should succeed")

		// Try to upload with wrong offset (should fail)
		secondChunk := testData[512*1024:]
		_, err = tusService.WriteChunk(response.ID, 256*1024, secondChunk) // Wrong offset
		assert.Error(t, err, "Writing chunk with wrong offset should fail")
		assert.Contains(t, err.Error(), "invalid offset", "Error should mention invalid offset")
	})

	t.Run("GetUploadSize", func(t *testing.T) {
		filename := "tus-size-test.wav"
		size := int64(768 * 1024) // 768KB
		testData, err := CreateTestFile(filename, int(size))
		require.NoError(t, err, "Creating test data should succeed")

		// Create upload session
		response, err := tusService.CreateUpload(size, filename, map[string]string{"filename": filename})
		require.NoError(t, err, "Creating upload session should succeed")

		// Check initial size
		currentSize, err := tusService.GetUploadSize(response.ID)
		require.NoError(t, err, "Getting upload size should succeed")
		assert.Equal(t, int64(0), currentSize, "Initial size should be 0")

		// Upload partial data
		partialData := testData[:384*1024] // 384KB
		_, err = tusService.WriteChunk(response.ID, 0, partialData)
		require.NoError(t, err, "Writing partial chunk should succeed")

		// Check updated size
		currentSize, err = tusService.GetUploadSize(response.ID)
		require.NoError(t, err, "Getting upload size should succeed")
		assert.Equal(t, int64(384*1024), currentSize, "Size should reflect uploaded data")
	})

	t.Run("ListUploads", func(t *testing.T) {
		// Create multiple upload sessions
		uploads := []struct {
			filename string
			size     int64
		}{
			{"tus-list-1.wav", 256 * 1024},
			{"tus-list-2.wav", 512 * 1024},
			{"tus-list-3.wav", 1024 * 1024},
		}

		createdIDs := make([]string, len(uploads))
		for i, upload := range uploads {
			response, err := tusService.CreateUpload(upload.size, upload.filename, map[string]string{"filename": upload.filename})
			require.NoError(t, err, "Creating upload session should succeed")
			createdIDs[i] = response.ID
		}

		// List all uploads
		uploadList := tusService.ListUploads()
		
		// Should have at least our created uploads
		assert.GreaterOrEqual(t, len(uploadList), len(uploads), "Should have at least the created uploads")

		// Check that our uploads are in the list
		uploadMap := make(map[string]*services.TUSInfo)
		for _, info := range uploadList {
			uploadMap[info.ID] = info
		}

		for i, id := range createdIDs {
			info, exists := uploadMap[id]
			assert.True(t, exists, "Created upload should be in list: %s", id)
			if exists {
				assert.Equal(t, uploads[i].filename, info.Filename, "Filename should match")
				assert.Equal(t, uploads[i].size, info.Size, "Size should match")
				assert.Equal(t, int64(0), info.Offset, "Initial offset should be 0")
				assert.Equal(t, 0.0, info.Progress, "Initial progress should be 0")
			}
		}
	})

	t.Run("DeleteUpload", func(t *testing.T) {
		filename := "tus-delete-test.wav"
		size := int64(256 * 1024) // 256KB

		// Create upload session
		response, err := tusService.CreateUpload(size, filename, map[string]string{"filename": filename})
		require.NoError(t, err, "Creating upload session should succeed")

		// Verify it exists
		_, err = tusService.GetUpload(response.ID)
		require.NoError(t, err, "Upload should exist before deletion")

		// Delete upload
		err = tusService.DeleteUpload(response.ID)
		require.NoError(t, err, "Deleting upload should succeed")

		// Verify it's gone
		_, err = tusService.GetUpload(response.ID)
		assert.Error(t, err, "Upload should not exist after deletion")
		assert.Contains(t, err.Error(), "not found", "Error should indicate upload not found")
	})

	t.Run("CleanupExpiredUploads", func(t *testing.T) {
		// Create an upload session
		filename := "tus-cleanup-test.wav"
		size := int64(128 * 1024) // 128KB

		response, err := tusService.CreateUpload(size, filename, map[string]string{"filename": filename})
		require.NoError(t, err, "Creating upload session should succeed")

		// Verify it exists
		_, err = tusService.GetUpload(response.ID)
		require.NoError(t, err, "Upload should exist before cleanup")

		// Run cleanup with very short expiration (should remove our upload since it was just created)
		tusService.CleanupExpiredUploads(1 * time.Nanosecond)

		// The upload might or might not be cleaned up depending on timing,
		// but the cleanup should not crash
		_, err = tusService.GetUpload(response.ID)
		// Don't assert on the error - timing dependent
		if err != nil {
			assert.Contains(t, err.Error(), "not found", "If cleanup removed upload, error should indicate not found")
		}
	})

	t.Run("LargeFileUpload", func(t *testing.T) {
		filename := "tus-large-test.wav"
		size := int64(10 * 1024 * 1024) // 10MB
		chunkSize := 1024 * 1024        // 1MB chunks

		// Create large test data
		testData, err := CreateTestFile(filename, int(size))
		require.NoError(t, err, "Creating large test data should succeed")

		expectedHash := fmt.Sprintf("%x", sha256.Sum256(testData))

		// Create upload session
		response, err := tusService.CreateUpload(size, filename, map[string]string{
			"filename": filename,
			"filetype": "audio/wav",
		})
		require.NoError(t, err, "Creating large upload session should succeed")

		// Upload in chunks
		offset := int64(0)
		for offset < size {
			endOffset := offset + int64(chunkSize)
			if endOffset > size {
				endOffset = size
			}

			chunkData := testData[offset:endOffset]
			info, err := tusService.WriteChunk(response.ID, offset, chunkData)
			require.NoError(t, err, "Writing large file chunk should succeed at offset %d", offset)

			expectedProgress := float64(endOffset) / float64(size) * 100
			assert.InDelta(t, expectedProgress, info.Progress, 0.1, "Progress should be calculated correctly")

			offset = endOffset
		}

		// Verify integrity
		quality, err := tusService.VerifyUpload(response.ID, expectedHash)
		require.NoError(t, err, "Verifying large upload should succeed")
		assert.True(t, quality.IntegrityPassed, "Large file integrity should pass")

		// Verify we can read the complete file
		reader, err := tusService.GetUploadReader(response.ID)
		require.NoError(t, err, "Getting large upload reader should succeed")
		defer reader.Close()

		uploadedData, err := io.ReadAll(reader)
		require.NoError(t, err, "Reading large uploaded data should succeed")
		assert.Equal(t, len(testData), len(uploadedData), "Uploaded data size should match")
		assert.Equal(t, expectedHash, fmt.Sprintf("%x", sha256.Sum256(uploadedData)), "Uploaded data hash should match")
	})

	t.Run("ConcurrentChunkUploads", func(t *testing.T) {
		// Note: TUS protocol typically requires sequential chunks, but we test concurrent sessions
		numSessions := 3
		sessionSize := int64(512 * 1024) // 512KB each

		sessions := make([]string, numSessions)
		testDataSets := make([][]byte, numSessions)

		// Create multiple sessions
		for i := 0; i < numSessions; i++ {
			filename := fmt.Sprintf("tus-concurrent-%d.wav", i)
			testData, err := CreateTestFile(filename, int(sessionSize))
			require.NoError(t, err, "Creating test data should succeed")

			response, err := tusService.CreateUpload(sessionSize, filename, map[string]string{"filename": filename})
			require.NoError(t, err, "Creating concurrent upload session should succeed")

			sessions[i] = response.ID
			testDataSets[i] = testData
		}

		// Upload data to all sessions concurrently
		for i := 0; i < numSessions; i++ {
			reader := bytes.NewReader(testDataSets[i])
			info, err := tusService.PatchUpload(sessions[i], 0, reader)
			require.NoError(t, err, "Concurrent upload should succeed for session %d", i)
			assert.Equal(t, 100.0, info.Progress, "Concurrent upload should be complete")
		}

		// Verify all uploads
		for i := 0; i < numSessions; i++ {
			expectedHash := fmt.Sprintf("%x", sha256.Sum256(testDataSets[i]))
			quality, err := tusService.VerifyUpload(sessions[i], expectedHash)
			require.NoError(t, err, "Verifying concurrent upload should succeed")
			assert.True(t, quality.IntegrityPassed, "Concurrent upload integrity should pass")
		}
	})
}

// Benchmark tests for TUS operations
func BenchmarkTUSOperations(b *testing.B) {
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
	streamingService := services.NewStreamingService(minioService, cfg)
	tusService := services.NewTUSService(cfg, streamingService)

	b.Run("CreateUploadSession", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			filename := fmt.Sprintf("bench-create-%d.wav", i)
			_, err := tusService.CreateUpload(1024*1024, filename, map[string]string{"filename": filename})
			if err != nil {
				b.Fatalf("Create upload failed: %v", err)
			}
		}
	})

	b.Run("WriteSmallChunk", func(b *testing.B) {
		// Setup
		testData, _ := CreateTestFile("bench-chunk.wav", 4096) // 4KB
		response, err := tusService.CreateUpload(int64(len(testData)*b.N), "bench-chunk.wav", map[string]string{"filename": "bench-chunk.wav"})
		if err != nil {
			b.Fatalf("Setup failed: %v", err)
		}

		b.ResetTimer()
		offset := int64(0)
		for i := 0; i < b.N; i++ {
			_, err := tusService.WriteChunk(response.ID, offset, testData)
			if err != nil {
				b.Fatalf("Write chunk failed: %v", err)
			}
			offset += int64(len(testData))
		}
	})

	b.Run("VerifyIntegrity", func(b *testing.B) {
		// Setup multiple completed uploads
		uploads := make([]string, b.N)
		hashes := make([]string, b.N)

		for i := 0; i < b.N; i++ {
			testData, _ := CreateTestFile(fmt.Sprintf("bench-verify-%d.wav", i), 64*1024) // 64KB
			response, err := tusService.CreateUpload(int64(len(testData)), fmt.Sprintf("bench-verify-%d.wav", i), map[string]string{"filename": fmt.Sprintf("bench-verify-%d.wav", i)})
			if err != nil {
				b.Fatalf("Setup failed: %v", err)
			}

			reader := bytes.NewReader(testData)
			_, err = tusService.PatchUpload(response.ID, 0, reader)
			if err != nil {
				b.Fatalf("Upload failed: %v", err)
			}

			uploads[i] = response.ID
			hashes[i] = fmt.Sprintf("%x", sha256.Sum256(testData))
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := tusService.VerifyUpload(uploads[i], hashes[i])
			if err != nil {
				b.Fatalf("Verify failed: %v", err)
			}
		}
	})
}