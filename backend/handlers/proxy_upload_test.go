package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ProxyUploadTestSuite struct {
	suite.Suite
	app          *fiber.App
	handlers     *Handlers
	mockMinIO    *MockMinIOService
}

func (suite *ProxyUploadTestSuite) SetupTest() {
	suite.app = fiber.New()
	suite.mockMinIO = new(MockMinIOService)
	
	suite.handlers = &Handlers{
		minioService: suite.mockMinIO,
	}
	
	// Setup routes
	api := suite.app.Group("/api")
	api.Post("/upload/proxy", suite.handlers.ProxyUpload)
	api.Put("/upload/proxy", suite.handlers.ProxyUpload)
	api.Post("/upload/proxy-url", suite.handlers.GetProxyUploadURL)
	api.Put("/upload/stream", suite.handlers.StreamProxyUpload)
}

// Test 1: RED - Test proxy URL generation (should fail initially)
func (suite *ProxyUploadTestSuite) TestGetProxyUploadURL_ShouldReturnProxyURL() {
	// Arrange
	requestBody := map[string]interface{}{
		"filename": "test-sermon.wav",
		"fileSize": int64(500 * 1024 * 1024), // 500MB
	}
	body, _ := json.Marshal(requestBody)
	
	// Mock MinIO to say file doesn't exist
	suite.mockMinIO.On("FileExists", "test-sermon.wav").Return(false, nil)
	suite.mockMinIO.On("GetLargeFileThreshold").Return(int64(100 * 1024 * 1024)) // 100MB
	
	// Act
	req := httptest.NewRequest("POST", "/api/upload/proxy-url", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", "sermons.wpgc.church")
	
	resp, err := suite.app.Test(req)
	
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	
	assert.True(suite.T(), result["success"].(bool))
	assert.Equal(suite.T(), "backend_proxy", result["uploadMethod"])
	assert.Contains(suite.T(), result["uploadUrl"].(string), "https://sermons.wpgc.church/api/upload/proxy")
	assert.Contains(suite.T(), result["uploadUrl"].(string), "filename=test-sermon.wav")
	assert.True(suite.T(), result["isLargeFile"].(bool))
	assert.Contains(suite.T(), result["message"].(string), "backend proxy to bypass browser restrictions")
}

// Test 2: RED - Test multipart form upload through proxy
func (suite *ProxyUploadTestSuite) TestProxyUpload_MultipartForm_ShouldUploadToMinIO() {
	// Arrange
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	
	// Add file field
	fw, err := w.CreateFormFile("file", "test-sermon.wav")
	assert.NoError(suite.T(), err)
	
	// Write fake WAV data
	wavData := []byte("RIFF....WAVEfmt ....data....")
	_, err = fw.Write(wavData)
	assert.NoError(suite.T(), err)
	
	// Add filename field
	fw, err = w.CreateFormField("filename")
	assert.NoError(suite.T(), err)
	fw.Write([]byte("test-sermon.wav"))
	
	w.Close()
	
	// Mock MinIO upload
	uploadInfo := minio.UploadInfo{
		Size: int64(len(wavData)),
		ETag: "abc123",
	}
	suite.mockMinIO.On("Client").Return(&minio.Client{})
	suite.mockMinIO.On("BucketName").Return("sermons")
	// Note: In real implementation, we'd need to mock the Client.PutObject method
	
	// Act
	req := httptest.NewRequest("POST", "/api/upload/proxy?filename=test-sermon.wav", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	
	resp, err := suite.app.Test(req)
	
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	
	assert.True(suite.T(), result["success"].(bool))
	assert.Equal(suite.T(), "test-sermon.wav", result["filename"])
	assert.Equal(suite.T(), "File uploaded successfully via proxy", result["message"])
}

// Test 3: RED - Test raw body upload (streaming)
func (suite *ProxyUploadTestSuite) TestProxyUpload_RawBody_ShouldStreamToMinIO() {
	// Arrange
	wavData := bytes.Repeat([]byte("WAVE"), 1024*1024) // 4MB of fake data
	
	// Mock MinIO upload
	suite.mockMinIO.On("Client").Return(&minio.Client{})
	suite.mockMinIO.On("BucketName").Return("sermons")
	
	// Act
	req := httptest.NewRequest("PUT", "/api/upload/proxy?filename=large-sermon.wav", bytes.NewReader(wavData))
	req.Header.Set("Content-Type", "audio/wav")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(wavData)))
	
	resp, err := suite.app.Test(req)
	
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	
	assert.True(suite.T(), result["success"].(bool))
	assert.Equal(suite.T(), "large-sermon.wav", result["filename"])
}

// Test 4: RED - Test streaming proxy for very large files
func (suite *ProxyUploadTestSuite) TestStreamProxyUpload_ShouldHandleLargeStreams() {
	// Arrange
	// Simulate a 500MB file
	largeData := strings.NewReader(strings.Repeat("A", 500*1024*1024))
	
	// Mock MinIO streaming upload
	suite.mockMinIO.On("Client").Return(&minio.Client{})
	suite.mockMinIO.On("BucketName").Return("sermons")
	
	// Act
	req := httptest.NewRequest("PUT", "/api/upload/stream?filename=huge-sermon.wav", largeData)
	req.Header.Set("Content-Type", "audio/wav")
	
	resp, err := suite.app.Test(req, -1) // No timeout for large files
	
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	
	assert.True(suite.T(), result["success"].(bool))
	assert.Equal(suite.T(), "huge-sermon.wav", result["filename"])
	assert.Equal(suite.T(), "File streamed successfully via proxy", result["message"])
}

// Test 5: RED - Test duplicate file detection
func (suite *ProxyUploadTestSuite) TestGetProxyUploadURL_DuplicateFile_ShouldReturn409() {
	// Arrange
	requestBody := map[string]interface{}{
		"filename": "existing-sermon.wav",
		"fileSize": int64(100 * 1024 * 1024),
	}
	body, _ := json.Marshal(requestBody)
	
	// Mock MinIO to say file exists
	suite.mockMinIO.On("FileExists", "existing-sermon.wav").Return(true, nil)
	
	// Act
	req := httptest.NewRequest("POST", "/api/upload/proxy-url", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := suite.app.Test(req)
	
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusConflict, resp.StatusCode)
	
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	
	assert.True(suite.T(), result["error"].(bool))
	assert.True(suite.T(), result["isDuplicate"].(bool))
	assert.Equal(suite.T(), "File already exists", result["message"])
}

// Test 6: RED - Test error handling when MinIO is down
func (suite *ProxyUploadTestSuite) TestProxyUpload_MinIOError_ShouldReturn500() {
	// Arrange
	wavData := []byte("RIFF....WAVEfmt ....data....")
	
	// Mock MinIO to return error
	suite.mockMinIO.On("Client").Return(&minio.Client{})
	suite.mockMinIO.On("BucketName").Return("sermons")
	// Simulate MinIO error (would need proper mocking in real implementation)
	
	// Act
	req := httptest.NewRequest("PUT", "/api/upload/proxy?filename=error-test.wav", bytes.NewReader(wavData))
	req.Header.Set("Content-Type", "audio/wav")
	
	resp, err := suite.app.Test(req)
	
	// Assert
	assert.NoError(suite.T(), err)
	// Should return 500 when MinIO fails
	assert.Equal(suite.T(), http.StatusInternalServerError, resp.StatusCode)
}

// Test 7: RED - Test missing filename parameter
func (suite *ProxyUploadTestSuite) TestProxyUpload_MissingFilename_ShouldReturn400() {
	// Arrange
	wavData := []byte("RIFF....WAVEfmt ....data....")
	
	// Act - No filename in query or form
	req := httptest.NewRequest("PUT", "/api/upload/proxy", bytes.NewReader(wavData))
	req.Header.Set("Content-Type", "audio/wav")
	
	resp, err := suite.app.Test(req)
	
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)
	
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	
	assert.True(suite.T(), result["error"].(bool))
	assert.Equal(suite.T(), "Filename is required", result["message"])
}

// Test 8: RED - Test browser compatibility with different protocols
func (suite *ProxyUploadTestSuite) TestGetProxyUploadURL_HTTPProtocol_ShouldReturnHTTPURL() {
	// Arrange
	requestBody := map[string]interface{}{
		"filename": "test.wav",
		"fileSize": int64(50 * 1024 * 1024),
	}
	body, _ := json.Marshal(requestBody)
	
	suite.mockMinIO.On("FileExists", "test.wav").Return(false, nil)
	suite.mockMinIO.On("GetLargeFileThreshold").Return(int64(100 * 1024 * 1024))
	
	// Act - Simulate HTTP request
	req := httptest.NewRequest("POST", "/api/upload/proxy-url", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", "localhost:8000")
	req.Header.Set("X-Forwarded-Proto", "http")
	
	resp, err := suite.app.Test(req)
	
	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	
	// Should return HTTP URL when request is HTTP
	assert.Contains(suite.T(), result["uploadUrl"].(string), "http://")
	assert.NotContains(suite.T(), result["uploadUrl"].(string), "https://")
}

func TestProxyUploadTestSuite(t *testing.T) {
	suite.Run(t, new(ProxyUploadTestSuite))
}

// Use the existing MockMinIOService from presigned_test.go
// Additional proxy-specific mock methods can be added here if needed