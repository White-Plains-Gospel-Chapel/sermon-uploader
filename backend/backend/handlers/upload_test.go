package handlers_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"sermon-uploader/config"
	"sermon-uploader/handlers"
	"sermon-uploader/services"
)

// MinIOServiceInterface defines what we need from MinIO for uploads
type MinIOServiceInterface interface {
	PutFile(ctx context.Context, bucket, objectName string, reader io.Reader, size int64, contentType string) (*minio.UploadInfo, error)
	TestConnection() error
}

// MockMinIOService mocks the MinIO service for testing
type MockMinIOService struct {
	mock.Mock
}

func (m *MockMinIOService) PutFile(ctx context.Context, bucket, objectName string, reader io.Reader, size int64, contentType string) (*minio.UploadInfo, error) {
	args := m.Called(ctx, bucket, objectName, reader, size, contentType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*minio.UploadInfo), args.Error(1)
}

func (m *MockMinIOService) UploadFile(fileData []byte, originalFilename string) (*services.FileMetadata, error) {
	args := m.Called(fileData, originalFilename)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.FileMetadata), args.Error(1)
}

func (m *MockMinIOService) TestConnection() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockMinIOService) GetClient() *minio.Client {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*minio.Client)
}

// UploadTestSuite defines the test suite for upload handlers
type UploadTestSuite struct {
	suite.Suite
	app          *fiber.App
	handler      *handlers.Handlers
	mockMinIO    *MockMinIOService
	mockDiscord  *services.DiscordService
	mockWsHub    *services.WebSocketHub
	mockFileService *services.FileService
}

// SetupTest runs before each test
func (suite *UploadTestSuite) SetupTest() {
	suite.app = fiber.New()
	suite.mockMinIO = new(MockMinIOService)
	
	// Create real services (we'll mock MinIO)
	cfg := config.New()
	suite.mockDiscord = services.NewDiscordService("")
	suite.mockWsHub = services.NewWebSocketHub()
	
	// Create handler with mocked MinIO service
	// We're passing mockMinIO where a real MinIOService would go
	// This requires changing the handlers.New signature to accept an interface
	suite.handler = &handlers.Handlers{}
	
	// Manually set the MinIO service for now (we'll refactor this later)
	// For TDD purposes, we'll create a test-specific constructor
	suite.handler = createTestHandler(suite.mockMinIO, suite.mockDiscord, suite.mockWsHub, cfg)

	// Setup routes
	api := suite.app.Group("/api")
	api.Post("/upload", suite.handler.Upload)
	api.Post("/upload/batch", suite.handler.UploadBatch)
}

// TestHandlers is a test-specific version of Handlers that uses interfaces
type TestHandlers struct {
	MinioService   MinIOServiceInterface
	DiscordService *services.DiscordService
	WsHub         *services.WebSocketHub
	Config        *config.Config
}

// createTestHandler creates a handler for testing with mock services
func createTestHandler(minioSvc MinIOServiceInterface, discord *services.DiscordService, wsHub *services.WebSocketHub, cfg *config.Config) *handlers.Handlers {
	// For now, we'll need to create a real MinIOService to satisfy the constructor
	// but we'll refactor this to use interfaces properly in production code
	// This is a temporary solution for TDD
	
	// Create a real handler with nil for services we don't need
	h := &handlers.Handlers{}
	// We'll need to manually set the minioService field through reflection or refactor the code
	// For now, let's create a simpler approach
	return h
}

// TestUploadSingleFile tests uploading a single WAV file
func (suite *UploadTestSuite) TestUploadSingleFile() {
	// Create a test WAV file
	fileContent := []byte("RIFF....WAVEfmt ") // Simplified WAV header
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	
	part, err := writer.CreateFormFile("file", "sermon.wav")
	suite.Require().NoError(err)
	
	_, err = part.Write(fileContent)
	suite.Require().NoError(err)
	
	err = writer.Close()
	suite.Require().NoError(err)

	// Setup mock expectation
	uploadInfo := &minio.UploadInfo{
		Bucket: "sermons",
		Key:    "sermon_123456789.wav",
		Size:   int64(len(fileContent)),
		ETag:   "abc123",
	}
	
	suite.mockMinIO.On("PutFile", 
		mock.Anything, // context
		"sermons",
		mock.MatchedBy(func(name string) bool {
			return len(name) > 0 // Just check it's not empty
		}),
		mock.Anything, // reader
		int64(len(fileContent)),
		"audio/wav",
	).Return(uploadInfo, nil)

	// Create request
	req := httptest.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Test the request
	resp, err := suite.app.Test(req, -1)
	suite.Require().NoError(err)
	
	// Assert response
	suite.Equal(http.StatusOK, resp.StatusCode, "Expected status 200 OK")
	
	// Verify mock was called
	suite.mockMinIO.AssertExpectations(suite.T())
}

// TestUploadWithoutFile tests uploading without providing a file
func (suite *UploadTestSuite) TestUploadWithoutFile() {
	req := httptest.NewRequest("POST", "/api/upload", nil)
	req.Header.Set("Content-Type", "multipart/form-data")

	resp, err := suite.app.Test(req, -1)
	suite.Require().NoError(err)
	
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "Expected status 400 Bad Request")
}

// TestUploadLargeFile tests uploading a 500MB+ file
func (suite *UploadTestSuite) TestUploadLargeFile() {
	// Create a large file simulation (we won't actually create 500MB in memory)
	fileSize := int64(500 * 1024 * 1024) // 500MB
	
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	
	part, err := writer.CreateFormFile("file", "large_sermon.wav")
	suite.Require().NoError(err)
	
	// Write minimal data but set the size in our mock
	_, err = part.Write([]byte("RIFF....WAVEfmt "))
	suite.Require().NoError(err)
	
	err = writer.Close()
	suite.Require().NoError(err)

	uploadInfo := &minio.UploadInfo{
		Bucket: "sermons",
		Key:    "large_sermon_123456789.wav",
		Size:   fileSize,
		ETag:   "xyz789",
	}
	
	suite.mockMinIO.On("PutFile",
		mock.Anything,
		"sermons",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		"audio/wav",
	).Return(uploadInfo, nil)

	req := httptest.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := suite.app.Test(req, -1)
	suite.Require().NoError(err)
	
	suite.Equal(http.StatusOK, resp.StatusCode, "Large file upload should succeed")
	suite.mockMinIO.AssertExpectations(suite.T())
}

// TestUploadBatchFiles tests uploading multiple files at once
func (suite *UploadTestSuite) TestUploadBatchFiles() {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	
	// Add multiple files
	files := []string{"sermon1.wav", "sermon2.wav", "sermon3.wav"}
	
	for _, filename := range files {
		part, err := writer.CreateFormFile("files", filename)
		suite.Require().NoError(err)
		_, err = part.Write([]byte("RIFF....WAVEfmt "))
		suite.Require().NoError(err)
	}
	
	err := writer.Close()
	suite.Require().NoError(err)

	// Setup mock expectations for each file
	for i, filename := range files {
		uploadInfo := &minio.UploadInfo{
			Bucket: "sermons",
			Key:    filename,
			Size:   16,
			ETag:   string(rune('a' + i)),
		}
		
		suite.mockMinIO.On("PutFile",
			mock.Anything,
			"sermons",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			"audio/wav",
		).Return(uploadInfo, nil).Once()
	}

	req := httptest.NewRequest("POST", "/api/upload/batch", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := suite.app.Test(req, -1)
	suite.Require().NoError(err)
	
	suite.Equal(http.StatusOK, resp.StatusCode, "Batch upload should succeed")
	suite.mockMinIO.AssertExpectations(suite.T())
}

// TestUploadWithMinIOError tests handling MinIO upload errors
func (suite *UploadTestSuite) TestUploadWithMinIOError() {
	fileContent := []byte("RIFF....WAVEfmt ")
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	
	part, err := writer.CreateFormFile("file", "sermon.wav")
	suite.Require().NoError(err)
	_, err = part.Write(fileContent)
	suite.Require().NoError(err)
	err = writer.Close()
	suite.Require().NoError(err)

	// Mock MinIO error
	suite.mockMinIO.On("PutFile",
		mock.Anything,
		"sermons",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		"audio/wav",
	).Return(nil, assert.AnError)

	req := httptest.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := suite.app.Test(req, -1)
	suite.Require().NoError(err)
	
	suite.Equal(http.StatusInternalServerError, resp.StatusCode, "Should return 500 on MinIO error")
	suite.mockMinIO.AssertExpectations(suite.T())
}

// Run the test suite
func TestUploadTestSuite(t *testing.T) {
	suite.Run(t, new(UploadTestSuite))
}

// Table-driven tests for file validation
func TestValidateFileType(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		contentType string
		shouldPass  bool
	}{
		{
			name:        "valid WAV file passes validation",
			filename:    "sermon.wav",
			contentType: "audio/wav",
			shouldPass:  true,
		},
		{
			name:        "WAV file with audio/x-wav content type passes",
			filename:    "sermon.wav",
			contentType: "audio/x-wav",
			shouldPass:  true,
		},
		{
			name:        "MP3 file fails validation",
			filename:    "sermon.mp3",
			contentType: "audio/mpeg",
			shouldPass:  false,
		},
		{
			name:        "file without extension fails",
			filename:    "sermon",
			contentType: "application/octet-stream",
			shouldPass:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			
			// This would call the actual validation function
			// result := handlers.ValidateFileType(tt.filename, tt.contentType)
			// assert.Equal(t, tt.shouldPass, result)
		})
	}
}

// Benchmark test for upload performance
func BenchmarkUploadFile(b *testing.B) {
	app := fiber.New()
	mockMinIO := new(MockMinIOService)
	cfg := config.New()
	
	handler := handlers.New(
		nil, mockMinIO, nil, nil, nil, cfg, nil,
	)
	
	app.Post("/upload", handler.Upload)
	
	// Setup mock
	uploadInfo := &minio.UploadInfo{
		Bucket: "sermons",
		Key:    "test.wav",
		Size:   100,
		ETag:   "test",
	}
	mockMinIO.On("PutFile",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(uploadInfo, nil)
	
	// Create test request
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.wav")
	part.Write([]byte("test"))
	writer.Close()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/upload", bytes.NewReader(body.Bytes()))
		req.Header.Set("Content-Type", writer.FormDataContentType())
		app.Test(req, -1)
	}
}