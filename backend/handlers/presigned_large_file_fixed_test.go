package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/valyala/fasthttp"

	"sermon-uploader/config"
)

// TDD Green Phase Test: Files >100MB should now use direct MinIO URLs (SHOULD PASS)
func TestPresignedURL_LargeFile_ShouldReturnDirectMinIOURL_Fixed(t *testing.T) {
	// Arrange
	mockMinio := &MockMinIOService{}
	mockConfig := &config.Config{
		PublicMinIOEndpoint: "sermons.wpgc.church", // CloudFlare proxy
		MinIOEndpoint:       "192.168.1.127:9000",  // Direct MinIO
		LargeFileThresholdMB: 100,                   // 100MB threshold
	}

	h := &TestHandlers{
		minioService: mockMinio,
		config:       mockConfig,
	}

	// File size > 100MB (CloudFlare free tier limit)
	largeFileSize := int64(150 * 1024 * 1024) // 150MB
	filename := "large_sermon_150mb.wav"

	// Mock expectations for the FIXED behavior
	mockMinio.On("CheckDuplicateByFilename", filename).Return(false, nil)
	// Handler should now call GeneratePresignedUploadURLSmart which returns direct MinIO URL for large files
	mockMinio.On("GeneratePresignedUploadURLSmart", filename, largeFileSize, mock.AnythingOfType("time.Duration")).Return("http://192.168.1.127:9000/sermons/large_sermon_150mb.wav?signature=direct123", true, nil)
	mockMinio.On("GetLargeFileThreshold").Return(int64(100 * 1024 * 1024)) // 100MB threshold

	// Create test request
	reqBody := map[string]interface{}{
		"filename": filename,
		"fileSize": largeFileSize,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Create fiber context
	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	ctx.Request().SetBody(bodyBytes)
	ctx.Request().Header.SetContentType("application/json")

	// Act
	err := h.GetPresignedURL(ctx)

	// Assert - should now PASS with the fix
	assert.NoError(t, err)
	assert.Equal(t, 200, ctx.Response().StatusCode())

	var response map[string]interface{}
	err = json.Unmarshal(ctx.Response().Body(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))

	uploadURL := response["uploadUrl"].(string)
	t.Logf("✅ FIXED BEHAVIOR: uploadURL = %s", uploadURL)

	// The key assertions - these should now PASS:
	assert.False(t, strings.Contains(uploadURL, "sermons.wpgc.church"), 
		"Large files (>100MB) should NOT use CloudFlare proxy URL to avoid 100MB upload limit")
	assert.True(t, strings.Contains(uploadURL, "192.168.1.127:9000"), 
		"Large files (>100MB) should use direct MinIO URL to bypass CloudFlare limits")
	assert.True(t, response["isLargeFile"].(bool), "Response should indicate this is a large file")
	assert.Equal(t, "direct_minio", response["uploadMethod"].(string), "Upload method should be direct_minio for large files")

	// Verify threshold information is included
	assert.Contains(t, response, "largeFileThreshold", "Response should include large file threshold")
	assert.Contains(t, response, "message", "Response should include explanatory message for large files")

	mockMinio.AssertExpectations(t)
	app.ReleaseCtx(ctx)
	
	t.Log("✅ Large file test PASSED - files >100MB now use direct MinIO URLs!")
}

// Test: Small files should still use CloudFlare URLs for CDN benefits  
func TestPresignedURL_SmallFile_ShouldUseCloudFlareURL_Fixed(t *testing.T) {
	// Arrange
	mockMinio := &MockMinIOService{}
	mockConfig := &config.Config{
		PublicMinIOEndpoint: "sermons.wpgc.church", // CloudFlare proxy
		MinIOEndpoint:       "192.168.1.127:9000",  // Direct MinIO
		LargeFileThresholdMB: 100,
	}

	h := &TestHandlers{
		minioService: mockMinio,
		config:       mockConfig,
	}

	// File size < 100MB (safe for CloudFlare)
	smallFileSize := int64(50 * 1024 * 1024) // 50MB
	filename := "small_sermon_50mb.wav"

	// Mock expectations - small files should still use CloudFlare
	mockMinio.On("CheckDuplicateByFilename", filename).Return(false, nil)
	mockMinio.On("GeneratePresignedUploadURLSmart", filename, smallFileSize, mock.AnythingOfType("time.Duration")).Return("https://sermons.wpgc.church/sermons/small_sermon_50mb.wav?signature=cf456", false, nil)

	// Create test request
	reqBody := map[string]interface{}{
		"filename": filename,
		"fileSize": smallFileSize,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Create fiber context
	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	ctx.Request().SetBody(bodyBytes)
	ctx.Request().Header.SetContentType("application/json")

	// Act
	err := h.GetPresignedURL(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 200, ctx.Response().StatusCode())

	var response map[string]interface{}
	err = json.Unmarshal(ctx.Response().Body(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))

	uploadURL := response["uploadUrl"].(string)

	// Small files should still use CloudFlare URLs for CDN benefits
	assert.True(t, strings.Contains(uploadURL, "sermons.wpgc.church"),
		"Small files (<100MB) should use CloudFlare proxy URL for CDN benefits")
	assert.False(t, strings.Contains(uploadURL, "192.168.1.127:9000"),
		"Small files should NOT use direct MinIO URL")

	// Should NOT be marked as large file
	assert.False(t, response["isLargeFile"].(bool), "Small files should not be marked as large files")
	assert.Equal(t, "cloudflare", response["uploadMethod"].(string), "Upload method should be cloudflare for small files")

	mockMinio.AssertExpectations(t)
	app.ReleaseCtx(ctx)
	
	t.Log("✅ Small file test PASSED - files <100MB still use CloudFlare URLs!")
}

// Test file size boundary conditions
func TestPresignedURL_FileSizeBoundaries_Fixed(t *testing.T) {
	testCases := []struct {
		name            string
		fileSize        int64
		expectedMethod  string
		shouldUseDirect bool
		description     string
	}{
		{
			name:            "99MB file should use CloudFlare",
			fileSize:        99 * 1024 * 1024,
			expectedMethod:  "cloudflare",
			shouldUseDirect: false,
			description:     "Just under the 100MB threshold",
		},
		{
			name:            "100MB file should use CloudFlare",
			fileSize:        100 * 1024 * 1024,
			expectedMethod:  "cloudflare", 
			shouldUseDirect: false,
			description:     "Exactly at the 100MB threshold",
		},
		{
			name:            "101MB file should use direct MinIO",
			fileSize:        101 * 1024 * 1024,
			expectedMethod:  "direct_minio",
			shouldUseDirect: true,
			description:     "Just over the 100MB threshold",
		},
		{
			name:            "500MB file should use direct MinIO",
			fileSize:        500 * 1024 * 1024,
			expectedMethod:  "direct_minio",
			shouldUseDirect: true,
			description:     "Well over the threshold",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			mockMinio := &MockMinIOService{}
			mockConfig := &config.Config{
				PublicMinIOEndpoint: "sermons.wpgc.church",
				MinIOEndpoint:       "192.168.1.127:9000",
				LargeFileThresholdMB: 100,
			}

			h := &TestHandlers{
				minioService: mockMinio,
				config:       mockConfig,
			}

			filename := "boundary_test_file.wav"

			// Mock expectations
			mockMinio.On("CheckDuplicateByFilename", filename).Return(false, nil)

			if tc.shouldUseDirect {
				// Large files use direct MinIO
				mockMinio.On("GeneratePresignedUploadURLSmart", filename, tc.fileSize, mock.AnythingOfType("time.Duration")).Return("http://192.168.1.127:9000/sermons/boundary_test_file.wav?signature=direct123", true, nil)
				mockMinio.On("GetLargeFileThreshold").Return(int64(100 * 1024 * 1024))
			} else {
				// Small files use CloudFlare
				mockMinio.On("GeneratePresignedUploadURLSmart", filename, tc.fileSize, mock.AnythingOfType("time.Duration")).Return("https://sermons.wpgc.church/sermons/boundary_test_file.wav?signature=cf456", false, nil)
			}

			// Create test request
			reqBody := map[string]interface{}{
				"filename": filename,
				"fileSize": tc.fileSize,
			}
			bodyBytes, _ := json.Marshal(reqBody)

			app := fiber.New()
			ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
			ctx.Request().SetBody(bodyBytes)
			ctx.Request().Header.SetContentType("application/json")

			// Act
			err := h.GetPresignedURL(ctx)

			// Assert
			assert.NoError(t, err)
			assert.Equal(t, 200, ctx.Response().StatusCode())

			var response map[string]interface{}
			err = json.Unmarshal(ctx.Response().Body(), &response)
			assert.NoError(t, err)
			assert.True(t, response["success"].(bool))

			// Check upload method
			assert.Equal(t, tc.expectedMethod, response["uploadMethod"].(string), 
				"Upload method should match expected based on file size (%s)", tc.description)

			// Check URL type
			uploadURL := response["uploadUrl"].(string)
			if tc.shouldUseDirect {
				assert.True(t, strings.Contains(uploadURL, "192.168.1.127:9000"), 
					"Large files should use direct MinIO URL (%s)", tc.description)
				assert.False(t, strings.Contains(uploadURL, "sermons.wpgc.church"), 
					"Large files should NOT use CloudFlare URL (%s)", tc.description)
				assert.True(t, response["isLargeFile"].(bool), 
					"Should be marked as large file (%s)", tc.description)
			} else {
				assert.True(t, strings.Contains(uploadURL, "sermons.wpgc.church"), 
					"Small files should use CloudFlare URL (%s)", tc.description)
				assert.False(t, strings.Contains(uploadURL, "192.168.1.127:9000"), 
					"Small files should NOT use direct MinIO URL (%s)", tc.description)
				assert.False(t, response["isLargeFile"].(bool), 
					"Should NOT be marked as large file (%s)", tc.description)
			}

			mockMinio.AssertExpectations(t)
			app.ReleaseCtx(ctx)
			
			t.Logf("✅ Boundary test PASSED: %s", tc.description)
		})
	}
}