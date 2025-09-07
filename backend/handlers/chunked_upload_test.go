package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ChunkedUploadTestSuite struct {
	suite.Suite
	app      *fiber.App
	handlers *Handlers
	mockMinIO *MockMinIOService
}

func (suite *ChunkedUploadTestSuite) SetupTest() {
	suite.app = fiber.New(fiber.Config{
		BodyLimit: 100 * 1024 * 1024, // 100MB limit to simulate CloudFlare
	})
	suite.mockMinIO = new(MockMinIOService)
	
	suite.handlers = &Handlers{
		minioService: suite.mockMinIO,
		chunkStore:   make(map[string][]byte), // In-memory chunk storage for testing
	}
	
	// Setup routes
	api := suite.app.Group("/api")
	api.Put("/upload/chunk", suite.handlers.UploadChunk)
	api.Post("/upload/complete-chunks", suite.handlers.CompleteChunkedUpload)
}

// Test 1: RED - Upload single chunk (should fail initially)
func (suite *ChunkedUploadTestSuite) TestUploadSingleChunk_ShouldStore() {
	// Arrange
	chunkData := bytes.Repeat([]byte("A"), 50*1024*1024) // 50MB chunk
	filename := "test-sermon.wav"
	
	// Act
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/upload/chunk?filename=%s&chunk=0&totalChunks=3", filename), 
		bytes.NewReader(chunkData))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Chunk-Index", "0")
	req.Header.Set("X-Total-Chunks", "3")
	
	resp, err := suite.app.Test(req)
	
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	
	// Verify chunk was stored
	assert.Contains(suite.T(), suite.handlers.chunkStore, fmt.Sprintf("%s_chunk_0", filename))
}

// Test 2: RED - Upload multiple chunks
func (suite *ChunkedUploadTestSuite) TestUploadMultipleChunks_ShouldStoreAll() {
	// Arrange
	filename := "large-sermon.wav"
	totalChunks := 3
	chunkSize := 50 * 1024 * 1024 // 50MB per chunk
	
	// Act - Upload 3 chunks
	for i := 0; i < totalChunks; i++ {
		chunkData := bytes.Repeat([]byte(fmt.Sprintf("%d", i)), chunkSize)
		
		req := httptest.NewRequest("PUT", 
			fmt.Sprintf("/api/upload/chunk?filename=%s&chunk=%d&totalChunks=%d", filename, i, totalChunks),
			bytes.NewReader(chunkData))
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("X-Chunk-Index", fmt.Sprintf("%d", i))
		req.Header.Set("X-Total-Chunks", fmt.Sprintf("%d", totalChunks))
		
		resp, err := suite.app.Test(req)
		
		// Assert each chunk uploads successfully
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	}
	
	// Verify all chunks were stored
	for i := 0; i < totalChunks; i++ {
		assert.Contains(suite.T(), suite.handlers.chunkStore, fmt.Sprintf("%s_chunk_%d", filename, i))
	}
}

// Test 3: RED - Complete chunked upload (reassemble chunks)
func (suite *ChunkedUploadTestSuite) TestCompleteChunkedUpload_ShouldReassemble() {
	// Arrange
	filename := "complete-test.wav"
	totalChunks := 3
	chunkSize := 50 * 1024 * 1024
	
	// Upload chunks first
	var expectedData []byte
	for i := 0; i < totalChunks; i++ {
		chunkData := bytes.Repeat([]byte(fmt.Sprintf("%d", i)), chunkSize)
		expectedData = append(expectedData, chunkData...)
		
		suite.handlers.chunkStore[fmt.Sprintf("%s_chunk_%d", filename, i)] = chunkData
	}
	
	// Mock MinIO upload expectation
	suite.mockMinIO.On("ProxyUploadFile", mock.Anything, filename, mock.Anything, int64(len(expectedData)), "audio/wav").
		Return(minio.UploadInfo{Size: int64(len(expectedData))}, nil)
	
	// Act - Complete the upload
	req := httptest.NewRequest("POST", 
		fmt.Sprintf("/api/upload/complete-chunks?filename=%s", filename),
		strings.NewReader(fmt.Sprintf(`{"totalChunks": %d}`, totalChunks)))
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := suite.app.Test(req)
	
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	
	// Verify chunks were cleaned up after assembly
	for i := 0; i < totalChunks; i++ {
		assert.NotContains(suite.T(), suite.handlers.chunkStore, fmt.Sprintf("%s_chunk_%d", filename, i))
	}
	
	// Verify MinIO upload was called
	suite.mockMinIO.AssertExpectations(suite.T())
}

// Test 4: RED - Handle missing chunks
func (suite *ChunkedUploadTestSuite) TestCompleteChunkedUpload_MissingChunks_ShouldFail() {
	// Arrange
	filename := "incomplete-test.wav"
	totalChunks := 3
	
	// Only upload 2 out of 3 chunks
	for i := 0; i < 2; i++ {
		chunkData := bytes.Repeat([]byte("X"), 50*1024*1024)
		suite.handlers.chunkStore[fmt.Sprintf("%s_chunk_%d", filename, i)] = chunkData
	}
	
	// Act - Try to complete with missing chunk
	req := httptest.NewRequest("POST",
		fmt.Sprintf("/api/upload/complete-chunks?filename=%s", filename),
		strings.NewReader(fmt.Sprintf(`{"totalChunks": %d}`, totalChunks)))
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := suite.app.Test(req)
	
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)
	
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	assert.True(suite.T(), result["error"].(bool))
	assert.Contains(suite.T(), result["message"], "Missing chunk")
}

// Test 5: RED - Chunk size validation (must be under 100MB for CloudFlare)
func (suite *ChunkedUploadTestSuite) TestUploadChunk_TooLarge_ShouldReject() {
	// Arrange
	oversizedChunk := bytes.Repeat([]byte("X"), 101*1024*1024) // 101MB - over CloudFlare limit
	filename := "oversized-test.wav"
	
	// Act
	req := httptest.NewRequest("PUT",
		fmt.Sprintf("/api/upload/chunk?filename=%s&chunk=0&totalChunks=1", filename),
		bytes.NewReader(oversizedChunk))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(oversizedChunk)))
	
	resp, err := suite.app.Test(req)
	
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusRequestEntityTooLarge, resp.StatusCode)
}

// Test 6: RED - Concurrent chunk uploads
func (suite *ChunkedUploadTestSuite) TestConcurrentChunkUploads_ShouldHandleCorrectly() {
	// Arrange
	filename := "concurrent-test.wav"
	totalChunks := 10
	chunkSize := 40 * 1024 * 1024 // 40MB chunks
	
	// Act - Upload chunks concurrently
	var wg sync.WaitGroup
	errors := make(chan error, totalChunks)
	
	for i := 0; i < totalChunks; i++ {
		wg.Add(1)
		go func(chunkIndex int) {
			defer wg.Done()
			
			chunkData := bytes.Repeat([]byte(fmt.Sprintf("%d", chunkIndex)), chunkSize)
			req := httptest.NewRequest("PUT",
				fmt.Sprintf("/api/upload/chunk?filename=%s&chunk=%d&totalChunks=%d", filename, chunkIndex, totalChunks),
				bytes.NewReader(chunkData))
			req.Header.Set("Content-Type", "application/octet-stream")
			
			resp, err := suite.app.Test(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				errors <- fmt.Errorf("chunk %d failed", chunkIndex)
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Assert - Check no errors occurred
	var errorList []error
	for err := range errors {
		errorList = append(errorList, err)
	}
	assert.Empty(suite.T(), errorList, "Concurrent uploads should not fail")
	
	// Verify all chunks were stored
	for i := 0; i < totalChunks; i++ {
		assert.Contains(suite.T(), suite.handlers.chunkStore, fmt.Sprintf("%s_chunk_%d", filename, i))
	}
}

// Test 7: RED - Cleanup orphaned chunks after timeout
func (suite *ChunkedUploadTestSuite) TestOrphanedChunkCleanup_ShouldRemoveOldChunks() {
	// Arrange
	filename := "orphaned-test.wav"
	
	// Create an old chunk (simulate timeout)
	oldChunk := bytes.Repeat([]byte("O"), 50*1024*1024)
	suite.handlers.chunkStore[fmt.Sprintf("%s_chunk_0", filename)] = oldChunk
	
	// Set chunk timestamp to 1 hour ago
	suite.handlers.chunkTimestamps[fmt.Sprintf("%s_chunk_0", filename)] = time.Now().Add(-1 * time.Hour)
	
	// Act - Run cleanup
	suite.handlers.CleanupOrphanedChunks(30 * time.Minute)
	
	// Assert - Old chunk should be removed
	assert.NotContains(suite.T(), suite.handlers.chunkStore, fmt.Sprintf("%s_chunk_0", filename))
}

func TestChunkedUploadTestSuite(t *testing.T) {
	suite.Run(t, new(ChunkedUploadTestSuite))
}