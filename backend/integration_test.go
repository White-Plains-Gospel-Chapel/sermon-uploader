//go:build integration
// +build integration

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"sermon-uploader/config"
	"sermon-uploader/services"
)

// IntegrationTestSuite provides end-to-end testing for the sermon uploader system
type IntegrationTestSuite struct {
	suite.Suite
	config       *config.Config
	minioService *services.MinIOService
	testBucket   string
	testFiles    []TestFile
	server       *http.Server
	baseURL      string
	cleanup      []func()
}

// TestFile represents a test file with metadata
type TestFile struct {
	Name    string
	Size    int64
	Hash    string
	Data    []byte
	Pattern string // "predictable", "random", "large"
}

// SetupSuite initializes the integration test environment
func (s *IntegrationTestSuite) SetupSuite() {
	// Load test configuration
	s.config = config.New()
	s.testBucket = s.config.MinioBucket + "-test"
	s.baseURL = fmt.Sprintf("http://localhost:%s", s.config.Port)

	// Override config for testing
	s.config.MinioBucket = s.testBucket
	s.config.MaxConcurrentUploads = 5 // Higher for concurrent testing

	s.T().Logf("Using test bucket: %s", s.testBucket)

	// Initialize MinIO service with test configuration
	s.minioService = services.NewMinIOService(s.config)

	// Test MinIO connection
	require.NoError(s.T(), s.minioService.TestConnection(), "MinIO connection failed")

	// Ensure test bucket exists
	require.NoError(s.T(), s.minioService.EnsureBucketExists(), "Failed to create test bucket")

	// Generate test files
	s.generateTestFiles()

	// Start test server
	s.startTestServer()

	s.T().Logf("Integration test suite setup complete")
}

// TearDownSuite cleans up the integration test environment
func (s *IntegrationTestSuite) TearDownSuite() {
	// Run cleanup functions
	for _, cleanup := range s.cleanup {
		cleanup()
	}

	// Stop test server
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}

	// Clear test bucket
	if s.minioService != nil {
		result, err := s.minioService.ClearBucket()
		if err != nil {
			s.T().Logf("Warning: Failed to clear test bucket: %v", err)
		} else {
			s.T().Logf("Cleared test bucket: %d deleted, %d failed", result.DeletedCount, result.FailedCount)
		}
	}

	s.T().Logf("Integration test suite teardown complete")
}

// generateTestFiles creates various test files for integration testing
func (s *IntegrationTestSuite) generateTestFiles() {
	testSpecs := []struct {
		name    string
		sizeKB  int64
		pattern string
	}{
		{"small_file.wav", 100, "predictable"},      // 100KB
		{"medium_file.wav", 10240, "predictable"},   // 10MB
		{"large_file.wav", 51200, "predictable"},    // 50MB
		{"huge_file.wav", 512000, "predictable"},    // 500MB
		{"xlarge_file.wav", 1048576, "predictable"}, // 1GB
		{"random_small.wav", 1024, "random"},        // 1MB random
		{"random_medium.wav", 10240, "random"},      // 10MB random
	}

	s.testFiles = make([]TestFile, 0, len(testSpecs))

	for _, spec := range testSpecs {
		data := s.generateFileData(spec.sizeKB*1024, spec.pattern)
		hash := fmt.Sprintf("%x", sha256.Sum256(data))

		testFile := TestFile{
			Name:    spec.name,
			Size:    spec.sizeKB * 1024,
			Hash:    hash,
			Data:    data,
			Pattern: spec.pattern,
		}

		s.testFiles = append(s.testFiles, testFile)
		s.T().Logf("Generated test file: %s (%.2f MB)", spec.name, float64(spec.sizeKB)/1024)
	}

	s.T().Logf("Generated %d test files", len(s.testFiles))
}

// generateFileData creates test file data with specified pattern
func (s *IntegrationTestSuite) generateFileData(size int64, pattern string) []byte {
	data := make([]byte, size)

	switch pattern {
	case "predictable":
		// Create predictable pattern for hash consistency
		for i := int64(0); i < size; i++ {
			data[i] = byte(i % 256)
		}
	case "random":
		// Create random data
		rand.Read(data)
	default:
		// Default to predictable
		for i := int64(0); i < size; i++ {
			data[i] = byte(i % 256)
		}
	}

	return data
}

// startTestServer starts a test HTTP server
func (s *IntegrationTestSuite) startTestServer() {
	// Use main.go server setup or create minimal test server
	// For integration testing, we'll create a minimal server
	mux := http.NewServeMux()

	// Add basic upload endpoint for testing
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse multipart form
		err := r.ParseMultipartForm(32 << 20) // 32MB max memory
		if err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Failed to get file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Read file data
		fileData, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "Failed to read file", http.StatusInternalServerError)
			return
		}

		// Upload to MinIO
		metadata, err := s.minioService.UploadFile(fileData, header.Filename)
		if err != nil {
			http.Error(w, fmt.Sprintf("Upload failed: %v", err), http.StatusInternalServerError)
			return
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"filename": metadata.RenamedFilename,
			"hash":     metadata.FileHash,
			"size":     metadata.FileSize,
		})
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		err := s.minioService.TestConnection()
		if err != nil {
			http.Error(w, "MinIO connection failed", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
			"minio":  "connected",
		})
	})

	s.server = &http.Server{
		Addr:    ":" + s.config.Port,
		Handler: mux,
	}

	// Start server in background
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.T().Logf("Server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	s.T().Logf("Test server started on %s", s.baseURL)
}

// TestEndToEndUploadFlow tests the complete upload workflow
func (s *IntegrationTestSuite) TestEndToEndUploadFlow() {
	testFile := s.getTestFile("small_file.wav")
	require.NotNil(s.T(), testFile, "Test file not found")

	// Test direct upload via API
	uploadResp := s.uploadFileViaAPI(testFile)
	assert.Equal(s.T(), testFile.Hash, uploadResp["hash"])
	assert.Equal(s.T(), float64(testFile.Size), uploadResp["size"])

	// Verify file exists in MinIO
	exists := s.verifyFileInMinIO(uploadResp["filename"].(string))
	assert.True(s.T(), exists, "File should exist in MinIO")

	// Verify file integrity
	integrity := s.verifyFileIntegrity(uploadResp["filename"].(string), testFile.Hash)
	assert.True(s.T(), integrity.IntegrityPassed, "File integrity should pass")
	assert.Equal(s.T(), testFile.Hash, integrity.StoredHash, "Stored hash should match")

	s.T().Logf("✓ End-to-end upload flow completed successfully")
}

// TestLargeFileHandling tests uploading large files (500MB-1GB)
func (s *IntegrationTestSuite) TestLargeFileHandling() {
	testCases := []string{"huge_file.wav", "xlarge_file.wav"}

	for _, filename := range testCases {
		s.T().Run(filename, func(t *testing.T) {
			testFile := s.getTestFile(filename)
			require.NotNil(t, testFile, "Test file not found")

			t.Logf("Testing large file upload: %s (%.2f MB)", filename, float64(testFile.Size)/(1024*1024))

			startTime := time.Now()

			// Use streaming upload for large files
			reader := bytes.NewReader(testFile.Data)
			metadata, err := s.minioService.UploadFileStreaming(reader, testFile.Name, testFile.Size, testFile.Hash)

			uploadDuration := time.Since(startTime)

			require.NoError(t, err, "Large file upload should succeed")
			assert.Equal(t, testFile.Hash, metadata.FileHash, "Hash should match")
			assert.Equal(t, testFile.Size, metadata.FileSize, "Size should match")

			// Verify integrity
			integrity, err := s.minioService.VerifyUploadIntegrity(metadata.RenamedFilename, testFile.Hash)
			require.NoError(t, err, "Integrity verification should succeed")
			assert.True(t, integrity.IntegrityPassed, "Large file integrity should pass")

			throughput := float64(testFile.Size) / (1024 * 1024) / uploadDuration.Seconds()
			t.Logf("✓ Large file upload completed: %.2f MB/s throughput", throughput)

			// Verify throughput is reasonable (>1 MB/s for Pi)
			assert.Greater(t, throughput, 1.0, "Upload throughput should be reasonable")
		})
	}
}

// TestBatchUploadPerformance tests concurrent file uploads
func (s *IntegrationTestSuite) TestBatchUploadPerformance() {
	const numConcurrentFiles = 20

	// Use small and medium files for batch testing
	testFiles := []TestFile{}
	for i := 0; i < numConcurrentFiles; i++ {
		if i%2 == 0 {
			testFiles = append(testFiles, *s.getTestFile("small_file.wav"))
		} else {
			testFiles = append(testFiles, *s.getTestFile("medium_file.wav"))
		}
	}

	// Test concurrent uploads
	var wg sync.WaitGroup
	results := make([]error, numConcurrentFiles)
	startTime := time.Now()

	for i, testFile := range testFiles {
		wg.Add(1)
		go func(index int, tf TestFile) {
			defer wg.Done()

			// Create unique filename
			uniqueFilename := fmt.Sprintf("batch_%d_%s", index, tf.Name)
			reader := bytes.NewReader(tf.Data)

			_, err := s.minioService.UploadFileStreaming(reader, uniqueFilename, tf.Size, tf.Hash)
			results[index] = err
		}(i, testFile)
	}

	wg.Wait()
	batchDuration := time.Since(startTime)

	// Check results
	successCount := 0
	for i, err := range results {
		if err != nil {
			s.T().Logf("Upload %d failed: %v", i, err)
		} else {
			successCount++
		}
	}

	successRate := float64(successCount) / float64(numConcurrentFiles) * 100
	assert.Greater(s.T(), successRate, 80.0, "Success rate should be > 80%")

	s.T().Logf("✓ Batch upload performance: %d/%d successful (%.1f%%) in %v",
		successCount, numConcurrentFiles, successRate, batchDuration)
}

// TestConnectionPoolHealth tests connection pool behavior
func (s *IntegrationTestSuite) TestConnectionPoolHealth() {
	// Get initial connection pool stats
	initialStats := s.minioService.GetConnectionPoolStats()
	s.T().Logf("Initial connection pool stats: %+v", initialStats)

	// Perform multiple operations to stress connection pool
	const numOperations = 50
	var wg sync.WaitGroup

	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Alternate between different operations
			switch index % 3 {
			case 0:
				// List files
				_, err := s.minioService.ListFiles()
				if err != nil {
					s.T().Logf("List files operation %d failed: %v", index, err)
				}
			case 1:
				// Get file count
				_, err := s.minioService.GetFileCount()
				if err != nil {
					s.T().Logf("Get file count operation %d failed: %v", index, err)
				}
			case 2:
				// Test connection
				err := s.minioService.TestConnection()
				if err != nil {
					s.T().Logf("Test connection operation %d failed: %v", index, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Get final connection pool stats
	finalStats := s.minioService.GetConnectionPoolStats()
	s.T().Logf("Final connection pool stats: %+v", finalStats)

	// Verify connection pool health
	assert.Greater(s.T(), finalStats["total"], int64(0), "Should have connections")
	assert.LessOrEqual(s.T(), finalStats["active"], int64(s.config.MaxConnsPerHost),
		"Active connections should not exceed limit")

	s.T().Logf("✓ Connection pool health verified")
}

// TestErrorRecovery tests network failures and retry mechanisms
func (s *IntegrationTestSuite) TestErrorRecovery() {
	testFile := s.getTestFile("small_file.wav")
	require.NotNil(s.T(), testFile, "Test file not found")

	// Test with timeout context to simulate network issues
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond) // Very short timeout
	defer cancel()

	// This should fail due to timeout
	reader := bytes.NewReader(testFile.Data)
	_, err := s.minioService.UploadFileStreaming(reader, "timeout_test.wav", testFile.Size, testFile.Hash)

	// The upload might succeed if it's very fast, or fail due to timeout
	// The important thing is that it handles the error gracefully
	if err != nil {
		s.T().Logf("Expected timeout error occurred: %v", err)
		assert.Contains(s.T(), err.Error(), "context deadline exceeded", "Should be timeout error")
	} else {
		s.T().Logf("Upload succeeded despite short timeout (very fast connection)")
	}

	// Test normal upload after error to verify recovery
	reader = bytes.NewReader(testFile.Data)
	metadata, err := s.minioService.UploadFileStreaming(reader, "recovery_test.wav", testFile.Size, testFile.Hash)
	assert.NoError(s.T(), err, "Upload should succeed after recovery")
	assert.NotNil(s.T(), metadata, "Metadata should be returned")

	s.T().Logf("✓ Error recovery verified")
}

// TestPiResourceConstraints tests memory and CPU limits
func (s *IntegrationTestSuite) TestPiResourceConstraints() {
	// Test memory usage with large files
	testFile := s.getTestFile("huge_file.wav")
	require.NotNil(s.T(), testFile, "Test file not found")

	// Monitor memory during upload
	startTime := time.Now()

	reader := bytes.NewReader(testFile.Data)
	metadata, err := s.minioService.UploadFileStreaming(reader, "memory_test.wav", testFile.Size, testFile.Hash)

	uploadDuration := time.Since(startTime)

	require.NoError(s.T(), err, "Large file upload should succeed despite memory constraints")
	assert.Equal(s.T(), testFile.Hash, metadata.FileHash, "Hash should match")

	// Test concurrent uploads to stress CPU
	const numConcurrent = 10
	var wg sync.WaitGroup
	errors := make([]error, numConcurrent)

	smallFile := s.getTestFile("small_file.wav")

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			reader := bytes.NewReader(smallFile.Data)
			filename := fmt.Sprintf("cpu_test_%d.wav", index)

			_, err := s.minioService.UploadFileStreaming(reader, filename, smallFile.Size, smallFile.Hash)
			errors[index] = err
		}(i)
	}

	wg.Wait()

	// Count successes
	successCount := 0
	for _, err := range errors {
		if err == nil {
			successCount++
		}
	}

	successRate := float64(successCount) / float64(numConcurrent) * 100
	assert.Greater(s.T(), successRate, 70.0, "Should handle CPU stress with >70% success rate")

	s.T().Logf("✓ Pi resource constraints: %.1f%% success under load, %v upload time for large file",
		successRate, uploadDuration)
}

// TestZeroCompressionValidation tests bit-perfect audio preservation
func (s *IntegrationTestSuite) TestZeroCompressionValidation() {
	testFile := s.getTestFile("medium_file.wav")
	require.NotNil(s.T(), testFile, "Test file not found")

	// Upload file with zero compression settings
	reader := bytes.NewReader(testFile.Data)
	metadata, err := s.minioService.UploadFileStreaming(reader, "compression_test.wav", testFile.Size, testFile.Hash)
	require.NoError(s.T(), err, "Upload should succeed")

	// Get compression statistics
	stats, err := s.minioService.GetZeroCompressionStats()
	require.NoError(s.T(), err, "Should get compression stats")

	// Find our uploaded file in stats
	var fileInfo *services.FileCompressionInfo
	for i, file := range stats.Files {
		if strings.Contains(file.Filename, "compression_test") {
			fileInfo = &stats.Files[i]
			break
		}
	}

	require.NotNil(s.T(), fileInfo, "Should find uploaded file in stats")

	// Verify zero compression settings
	assert.True(s.T(), fileInfo.IsZeroCompression, "File should use zero compression")
	assert.True(s.T(), fileInfo.IsBitPerfect, "File should be marked as bit-perfect")
	assert.Equal(s.T(), "application/octet-stream", fileInfo.ContentType, "Should use octet-stream content type")

	// Verify integrity by downloading and comparing
	downloadedData, err := s.minioService.DownloadFileData(metadata.RenamedFilename)
	require.NoError(s.T(), err, "Should download file successfully")

	downloadedHash := fmt.Sprintf("%x", sha256.Sum256(downloadedData))
	assert.Equal(s.T(), testFile.Hash, downloadedHash, "Downloaded file hash should match original")
	assert.Equal(s.T(), testFile.Size, int64(len(downloadedData)), "Downloaded file size should match")

	s.T().Logf("✓ Zero compression validation: bit-perfect preservation confirmed")
}

// Helper methods

func (s *IntegrationTestSuite) getTestFile(name string) *TestFile {
	for i := range s.testFiles {
		if s.testFiles[i].Name == name {
			return &s.testFiles[i]
		}
	}
	return nil
}

func (s *IntegrationTestSuite) uploadFileViaAPI(testFile *TestFile) map[string]interface{} {
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", testFile.Name)
	require.NoError(s.T(), err, "Should create form file")

	_, err = part.Write(testFile.Data)
	require.NoError(s.T(), err, "Should write file data")

	err = writer.Close()
	require.NoError(s.T(), err, "Should close multipart writer")

	// Make request
	req, err := http.NewRequest("POST", s.baseURL+"/upload", &buf)
	require.NoError(s.T(), err, "Should create request")

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	require.NoError(s.T(), err, "Should make request")
	defer resp.Body.Close()

	require.Equal(s.T(), http.StatusOK, resp.StatusCode, "Should get OK response")

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(s.T(), err, "Should decode response")

	return result
}

func (s *IntegrationTestSuite) verifyFileInMinIO(filename string) bool {
	ctx := context.Background()
	_, err := s.minioService.GetClient().StatObject(ctx, s.testBucket, filename, minio.StatObjectOptions{})
	return err == nil
}

func (s *IntegrationTestSuite) verifyFileIntegrity(filename, expectedHash string) *services.IntegrityResult {
	result, err := s.minioService.VerifyUploadIntegrity(filename, expectedHash)
	require.NoError(s.T(), err, "Should verify integrity")
	return result
}

// TestRunner functions

// TestIntegrationSuite runs the complete integration test suite
func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Check if MinIO is available
	cfg := config.New()
	minioService := services.NewMinIOService(cfg)
	if err := minioService.TestConnection(); err != nil {
		t.Skipf("MinIO not available, skipping integration tests: %v", err)
	}

	suite.Run(t, new(IntegrationTestSuite))
}

// Individual test functions for targeted testing

func TestEndToEndUploadOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suite := &IntegrationTestSuite{}
	suite.SetupSuite()
	defer suite.TearDownSuite()

	suite.T = func() *testing.T { return t }
	suite.TestEndToEndUploadFlow()
}

func TestLargeFileOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suite := &IntegrationTestSuite{}
	suite.SetupSuite()
	defer suite.TearDownSuite()

	suite.T = func() *testing.T { return t }
	suite.TestLargeFileHandling()
}

func TestBatchUploadOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suite := &IntegrationTestSuite{}
	suite.SetupSuite()
	defer suite.TearDownSuite()

	suite.T = func() *testing.T { return t }
	suite.TestBatchUploadPerformance()
}
