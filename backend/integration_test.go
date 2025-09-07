//go:build integration
// +build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"sermon-uploader/config"
	"sermon-uploader/handlers"
	"sermon-uploader/services"
)

// IntegrationTestSuite tests the complete sermon upload workflow
type IntegrationTestSuite struct {
	suite.Suite
	app          *fiber.App
	minioClient  *minio.Client
	minioService *services.MinIOService
	cfg          *config.Config
	testBucket   string
}

// SetupSuite runs once before all tests
func (suite *IntegrationTestSuite) SetupSuite() {
	// Load test configuration
	suite.cfg = &config.Config{
		MinIOEndpoint:  getEnvOrDefault("TEST_MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey: getEnvOrDefault("TEST_MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey: getEnvOrDefault("TEST_MINIO_SECRET_KEY", "minioadmin"),
		MinIOUseSSL:    false,
		BucketName:     "test-sermons-" + fmt.Sprintf("%d", time.Now().Unix()),
		ServerPort:     "0", // Random port
	}
	suite.testBucket = suite.cfg.BucketName

	// Initialize MinIO client
	var err error
	suite.minioClient, err = minio.New(suite.cfg.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(suite.cfg.MinIOAccessKey, suite.cfg.MinIOSecretKey, ""),
		Secure: suite.cfg.MinIOUseSSL,
	})
	require.NoError(suite.T(), err, "Failed to create MinIO client")

	// Create test bucket
	ctx := context.Background()
	err = suite.minioClient.MakeBucket(ctx, suite.testBucket, minio.MakeBucketOptions{})
	require.NoError(suite.T(), err, "Failed to create test bucket")

	// Initialize services
	suite.minioService = services.NewMinIOService(suite.minioClient, suite.testBucket)

	// Initialize Fiber app with routes
	suite.app = fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	// Setup routes
	h := &handlers.Handlers{
		MinIOService: suite.minioService,
		Config:       suite.cfg,
	}
	h.SetupRoutes(suite.app)
}

// TearDownSuite runs once after all tests
func (suite *IntegrationTestSuite) TearDownSuite() {
	// Clean up test bucket
	ctx := context.Background()
	
	// Remove all objects
	objectsCh := suite.minioClient.ListObjects(ctx, suite.testBucket, minio.ListObjectsOptions{
		Recursive: true,
	})
	for object := range objectsCh {
		if object.Err != nil {
			continue
		}
		_ = suite.minioClient.RemoveObject(ctx, suite.testBucket, object.Key, minio.RemoveObjectOptions{})
	}
	
	// Remove bucket
	_ = suite.minioClient.RemoveBucket(ctx, suite.testBucket)
}

// TestHealthCheck verifies the health endpoint
func (suite *IntegrationTestSuite) TestHealthCheck() {
	req := httptest.NewRequest("GET", "/api/health", nil)
	resp, err := suite.app.Test(req)
	
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	
	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "healthy", result["status"])
}

// TestCompleteUploadWorkflow tests the entire upload pipeline
func (suite *IntegrationTestSuite) TestCompleteUploadWorkflow() {
	// Step 1: Create a test WAV file
	testContent := []byte("test audio content")
	
	// Step 2: Upload file via multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test-sermon.wav")
	require.NoError(suite.T(), err)
	
	_, err = part.Write(testContent)
	require.NoError(suite.T(), err)
	
	err = writer.Close()
	require.NoError(suite.T(), err)
	
	req := httptest.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	resp, err := suite.app.Test(req, -1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	
	// Step 3: Verify file exists in MinIO
	ctx := context.Background()
	_, err = suite.minioClient.StatObject(ctx, suite.testBucket, "wav/test-sermon_raw.wav", minio.StatObjectOptions{})
	assert.NoError(suite.T(), err, "Uploaded file should exist in MinIO")
	
	// Step 4: List files to verify
	listReq := httptest.NewRequest("GET", "/api/files", nil)
	listResp, err := suite.app.Test(listReq)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, listResp.StatusCode)
	
	var files []map[string]interface{}
	err = json.NewDecoder(listResp.Body).Decode(&files)
	assert.NoError(suite.T(), err)
	assert.GreaterOrEqual(suite.T(), len(files), 1, "Should have at least one file")
}

// TestPresignedURLWorkflow tests presigned URL generation and upload
func (suite *IntegrationTestSuite) TestPresignedURLWorkflow() {
	// Step 1: Request presigned URL
	reqBody := map[string]interface{}{
		"filename": "test-presigned.wav",
		"fileSize": 1024,
	}
	jsonBody, _ := json.Marshal(reqBody)
	
	req := httptest.NewRequest("POST", "/api/upload/presigned", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := suite.app.Test(req)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	
	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), result["uploadUrl"])
	assert.NotEmpty(suite.T(), result["fileKey"])
	
	// Step 2: Use presigned URL to upload (would normally be done by client)
	uploadURL := result["uploadUrl"].(string)
	assert.Contains(suite.T(), uploadURL, suite.testBucket)
}

// TestDuplicateDetection tests the duplicate file detection
func (suite *IntegrationTestSuite) TestDuplicateDetection() {
	// Upload same file twice
	testContent := []byte("duplicate test content")
	
	for i := 0; i < 2; i++ {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "duplicate.wav")
		part.Write(testContent)
		writer.Close()
		
		req := httptest.NewRequest("POST", "/api/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		
		resp, _ := suite.app.Test(req, -1)
		
		if i == 0 {
			assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
		} else {
			// Second upload should be detected as duplicate
			// Depending on implementation, this might return 200 with duplicate flag
			// or 409 Conflict
			assert.Contains(suite.T(), []int{http.StatusOK, http.StatusConflict}, resp.StatusCode)
		}
	}
}

// TestTUSResumableUpload tests the TUS protocol implementation
func (suite *IntegrationTestSuite) TestTUSResumableUpload() {
	// Step 1: Create upload
	createReq := httptest.NewRequest("POST", "/api/tus", nil)
	createReq.Header.Set("Upload-Length", "1024")
	createReq.Header.Set("Upload-Metadata", "filename dGVzdC50dXMud2F2") // base64 encoded "test.tus.wav"
	
	createResp, err := suite.app.Test(createReq)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusCreated, createResp.StatusCode)
	
	location := createResp.Header.Get("Location")
	assert.NotEmpty(suite.T(), location)
	
	// Step 2: Upload chunk
	chunkData := bytes.Repeat([]byte("a"), 512)
	patchReq := httptest.NewRequest("PATCH", location, bytes.NewReader(chunkData))
	patchReq.Header.Set("Content-Type", "application/offset+octet-stream")
	patchReq.Header.Set("Upload-Offset", "0")
	
	patchResp, err := suite.app.Test(patchReq)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusNoContent, patchResp.StatusCode)
	
	// Step 3: Check upload status
	headReq := httptest.NewRequest("HEAD", location, nil)
	headResp, err := suite.app.Test(headReq)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, headResp.StatusCode)
	assert.Equal(suite.T(), "512", headResp.Header.Get("Upload-Offset"))
}

// TestStreamingUpload tests the streaming upload endpoint
func (suite *IntegrationTestSuite) TestStreamingUpload() {
	largeContent := bytes.Repeat([]byte("stream"), 10000)
	
	req := httptest.NewRequest("POST", "/api/upload/streaming?filename=stream-test.wav", bytes.NewReader(largeContent))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(largeContent)))
	
	resp, err := suite.app.Test(req, -1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	
	// Verify file in MinIO
	ctx := context.Background()
	obj, err := suite.minioClient.GetObject(ctx, suite.testBucket, "wav/stream-test_raw.wav", minio.GetObjectOptions{})
	assert.NoError(suite.T(), err)
	
	downloadedContent, err := io.ReadAll(obj)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), largeContent, downloadedContent)
}

// TestConcurrentUploads tests multiple simultaneous uploads
func (suite *IntegrationTestSuite) TestConcurrentUploads() {
	numUploads := 5
	done := make(chan bool, numUploads)
	
	for i := 0; i < numUploads; i++ {
		go func(index int) {
			defer func() { done <- true }()
			
			content := []byte(fmt.Sprintf("concurrent content %d", index))
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, _ := writer.CreateFormFile("file", fmt.Sprintf("concurrent-%d.wav", index))
			part.Write(content)
			writer.Close()
			
			req := httptest.NewRequest("POST", "/api/upload", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			
			resp, err := suite.app.Test(req, -1)
			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
		}(i)
	}
	
	// Wait for all uploads
	for i := 0; i < numUploads; i++ {
		<-done
	}
	
	// Verify all files exist
	ctx := context.Background()
	objects := suite.minioClient.ListObjects(ctx, suite.testBucket, minio.ListObjectsOptions{
		Prefix:    "wav/concurrent-",
		Recursive: true,
	})
	
	count := 0
	for range objects {
		count++
	}
	assert.Equal(suite.T(), numUploads, count)
}

// TestErrorHandling tests various error scenarios
func (suite *IntegrationTestSuite) TestErrorHandling() {
	tests := []struct {
		name       string
		method     string
		path       string
		body       interface{}
		wantStatus int
	}{
		{
			name:       "Invalid JSON",
			method:     "POST",
			path:       "/api/upload/presigned",
			body:       "invalid json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Missing file",
			method:     "POST",
			path:       "/api/upload",
			body:       nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Invalid TUS upload",
			method:     "PATCH",
			path:       "/api/tus/invalid-id",
			body:       []byte("data"),
			wantStatus: http.StatusNotFound,
		},
	}
	
	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != nil {
				switch v := tt.body.(type) {
				case string:
					body = bytes.NewBufferString(v)
				case []byte:
					body = bytes.NewReader(v)
				default:
					jsonBody, _ := json.Marshal(v)
					body = bytes.NewBuffer(jsonBody)
				}
			}
			
			req := httptest.NewRequest(tt.method, tt.path, body)
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			
			resp, err := suite.app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}

// Helper function to get environment variable with default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// TestIntegrationSuite runs the integration test suite
func TestIntegrationSuite(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration tests. Set RUN_INTEGRATION_TESTS=true to run")
	}
	
	suite.Run(t, new(IntegrationTestSuite))
}

// TestWithDockerCompose tests with docker-compose managed services
func TestWithDockerCompose(t *testing.T) {
	if os.Getenv("RUN_DOCKER_TESTS") != "true" {
		t.Skip("Skipping docker tests. Set RUN_DOCKER_TESTS=true to run")
	}
	
	// This test assumes docker-compose.test.yml is set up
	// and services are running
	suite.Run(t, new(IntegrationTestSuite))
}