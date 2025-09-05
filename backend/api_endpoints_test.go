package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPI_HealthEndpoints tests all health-related API endpoints
func TestAPI_HealthEndpoints(t *testing.T) {
	app := fiber.New()

	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "healthy",
			"timestamp": fiber.Map{"now": "ok"},
			"service":   "sermon-uploader-go",
		})
	})

	app.Get("/api/status", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"minio_connected": true,
			"bucket_exists":   true,
			"file_count":      5,
			"endpoint":        "localhost:9000",
			"bucket_name":     "test-bucket",
		})
	})

	tests := []struct {
		name           string
		path           string
		method         string
		expectedStatus int
		expectedFields []string
	}{
		{
			name:           "health check endpoint",
			path:           "/api/health",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"status", "service"},
		},
		{
			name:           "status endpoint",
			path:           "/api/status",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"minio_connected", "bucket_exists", "file_count"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)

			for _, field := range tt.expectedFields {
				assert.Contains(t, response, field, "Response should contain field: %s", field)
			}
		})
	}
}

// TestAPI_DuplicateCheckEndpoint tests the duplicate checking functionality
func TestAPI_DuplicateCheckEndpoint(t *testing.T) {
	app := fiber.New()

	// Mock existing files
	existingFiles := map[string]bool{
		"existing.wav": true,
		"sermon_1.wav": true,
	}

	app.Post("/api/check-duplicate", func(c *fiber.Ctx) error {
		var req map[string]string
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error":   true,
				"message": "Invalid request",
			})
		}

		filename := req["filename"]
		if filename == "" {
			return c.Status(400).JSON(fiber.Map{
				"error":   true,
				"message": "Filename is required",
			})
		}

		isDuplicate := existingFiles[filename]

		return c.JSON(fiber.Map{
			"filename":    filename,
			"isDuplicate": isDuplicate,
			"message": func() string {
				if isDuplicate {
					return "File already exists"
				}
				return "File is unique"
			}(),
		})
	})

	tests := []struct {
		name           string
		requestBody    map[string]string
		expectedStatus int
		expectedDupe   bool
	}{
		{
			name:           "check existing file",
			requestBody:    map[string]string{"filename": "existing.wav"},
			expectedStatus: http.StatusOK,
			expectedDupe:   true,
		},
		{
			name:           "check non-existing file",
			requestBody:    map[string]string{"filename": "new.wav"},
			expectedStatus: http.StatusOK,
			expectedDupe:   false,
		},
		{
			name:           "invalid request - no filename",
			requestBody:    map[string]string{},
			expectedStatus: http.StatusBadRequest,
			expectedDupe:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/api/check-duplicate", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)

			if tt.expectedStatus == http.StatusOK {
				assert.Equal(t, tt.expectedDupe, response["isDuplicate"])
				assert.Contains(t, response, "filename")
				assert.Contains(t, response, "message")
			} else {
				assert.True(t, response["error"].(bool))
			}
		})
	}
}

// TestAPI_PresignedURLEndpoint tests presigned URL generation
func TestAPI_PresignedURLEndpoint(t *testing.T) {
	app := fiber.New()

	// Mock existing files for duplicate check
	existingFiles := map[string]bool{
		"existing.wav": true,
	}

	app.Post("/api/upload/presigned", func(c *fiber.Ctx) error {
		var req map[string]interface{}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error":   true,
				"message": "Invalid request",
			})
		}

		filename, ok := req["filename"].(string)
		if !ok || filename == "" {
			return c.Status(400).JSON(fiber.Map{
				"error":   true,
				"message": "Filename is required",
			})
		}

		// Check for duplicates
		if existingFiles[filename] {
			return c.Status(409).JSON(fiber.Map{
				"error":       true,
				"isDuplicate": true,
				"message":     "File already exists",
				"filename":    filename,
			})
		}

		// Generate mock presigned URL
		presignedURL := fmt.Sprintf("https://mock-minio.example.com/upload/%s?expires=%d",
			filename, time.Now().Add(time.Hour).Unix())

		return c.JSON(fiber.Map{
			"success":     true,
			"isDuplicate": false,
			"uploadUrl":   presignedURL,
			"filename":    filename,
			"fileSize":    req["fileSize"],
			"expires":     time.Now().Add(time.Hour).Unix(),
		})
	})

	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		expectURL      bool
		expectDupe     bool
	}{
		{
			name: "get presigned URL for new file",
			requestBody: map[string]interface{}{
				"filename": "new.wav",
				"fileSize": 1024,
			},
			expectedStatus: http.StatusOK,
			expectURL:      true,
			expectDupe:     false,
		},
		{
			name: "get presigned URL for existing file",
			requestBody: map[string]interface{}{
				"filename": "existing.wav",
				"fileSize": 1024,
			},
			expectedStatus: http.StatusConflict,
			expectURL:      false,
			expectDupe:     true,
		},
		{
			name: "invalid request - no filename",
			requestBody: map[string]interface{}{
				"fileSize": 1024,
			},
			expectedStatus: http.StatusBadRequest,
			expectURL:      false,
			expectDupe:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/api/upload/presigned", bytes.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)

			if tt.expectDupe {
				assert.True(t, response["isDuplicate"].(bool))
				assert.True(t, response["error"].(bool))
			} else if tt.expectURL {
				assert.False(t, response["isDuplicate"].(bool))
				assert.True(t, response["success"].(bool))
				assert.Contains(t, response, "uploadUrl")
				assert.Contains(t, response, "expires")
			} else {
				assert.True(t, response["error"].(bool))
			}
		})
	}
}

// TestAPI_BatchPresignedURL tests batch presigned URL generation
func TestAPI_BatchPresignedURL(t *testing.T) {
	app := fiber.New()

	existingFiles := map[string]bool{
		"existing.wav": true,
	}

	app.Post("/api/upload/presigned-batch", func(c *fiber.Ctx) error {
		var req map[string]interface{}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error":   true,
				"message": "Invalid request format",
			})
		}

		files, ok := req["files"].([]interface{})
		if !ok || len(files) == 0 {
			return c.Status(400).JSON(fiber.Map{
				"error":   true,
				"message": "No files provided",
			})
		}

		if len(files) > 50 {
			return c.Status(400).JSON(fiber.Map{
				"error":   true,
				"message": "Too many files. Maximum 50 files per batch.",
			})
		}

		results := make(map[string]interface{})
		successCount := 0
		duplicateCount := 0
		errorCount := 0

		for _, fileReq := range files {
			fileMap, ok := fileReq.(map[string]interface{})
			if !ok {
				continue
			}

			filename, _ := fileMap["filename"].(string)
			if filename == "" {
				continue
			}

			fileResult := make(map[string]interface{})

			if existingFiles[filename] {
				fileResult["error"] = false
				fileResult["isDuplicate"] = true
				fileResult["message"] = "File already exists"
				duplicateCount++
			} else {
				presignedURL := fmt.Sprintf("https://mock-minio.example.com/upload/%s?expires=%d",
					filename, time.Now().Add(time.Hour).Unix())

				fileResult["error"] = false
				fileResult["isDuplicate"] = false
				fileResult["uploadUrl"] = presignedURL
				fileResult["expires"] = time.Now().Add(time.Hour).Unix()
				successCount++
			}

			results[filename] = fileResult
		}

		return c.JSON(fiber.Map{
			"success":         errorCount == 0,
			"total_files":     len(files),
			"success_count":   successCount,
			"duplicate_count": duplicateCount,
			"error_count":     errorCount,
			"results":         results,
			"message":         fmt.Sprintf("Processed %d files", len(files)),
		})
	})

	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		expectedCounts map[string]int
	}{
		{
			name: "batch presigned URLs mixed results",
			requestBody: map[string]interface{}{
				"files": []map[string]interface{}{
					{"filename": "new1.wav", "fileSize": 1024},
					{"filename": "existing.wav", "fileSize": 1024},
					{"filename": "new2.wav", "fileSize": 2048},
				},
			},
			expectedStatus: http.StatusOK,
			expectedCounts: map[string]int{
				"total_files":     3,
				"success_count":   2,
				"duplicate_count": 1,
				"error_count":     0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/api/upload/presigned-batch", bytes.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)

			if tt.expectedCounts != nil {
				for key, expectedCount := range tt.expectedCounts {
					assert.Equal(t, float64(expectedCount), response[key], "Mismatch for %s", key)
				}
			}
		})
	}
}

// TestAPI_FileListEndpoint tests file listing functionality
func TestAPI_FileListEndpoint(t *testing.T) {
	app := fiber.New()

	mockFiles := []map[string]interface{}{
		{
			"name":          "sermon_2023_12_25.wav",
			"size":          int64(1024 * 1024 * 50),
			"last_modified": time.Now().Format(time.RFC3339),
		},
		{
			"name":          "sermon_2023_12_18.wav",
			"size":          int64(1024 * 1024 * 45),
			"last_modified": time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339),
		},
	}

	app.Get("/api/files", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"files":   mockFiles,
			"count":   len(mockFiles),
		})
	})

	req := httptest.NewRequest("GET", "/api/files", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	assert.Equal(t, float64(2), response["count"])

	files := response["files"].([]interface{})
	assert.Len(t, files, 2)

	// Verify file structure
	firstFile := files[0].(map[string]interface{})
	assert.Contains(t, firstFile, "name")
	assert.Contains(t, firstFile, "size")
	assert.Contains(t, firstFile, "last_modified")
}

// TestAPI_CORSHeaders tests CORS header functionality
func TestAPI_CORSHeaders(t *testing.T) {
	app := fiber.New()

	// Add CORS middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,HEAD,PATCH",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
		AllowCredentials: true,
	}))

	app.Get("/api/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"test": "ok"})
	})

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		checkHeaders   []string
	}{
		{
			name:           "OPTIONS request",
			method:         "OPTIONS",
			expectedStatus: http.StatusNoContent,
			checkHeaders:   []string{"Access-Control-Allow-Origin", "Access-Control-Allow-Methods"},
		},
		{
			name:           "GET request with CORS headers",
			method:         "GET",
			expectedStatus: http.StatusOK,
			checkHeaders:   []string{"Access-Control-Allow-Origin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/test", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			for _, header := range tt.checkHeaders {
				assert.NotEmpty(t, resp.Header.Get(header), "Header %s should be present", header)
			}
		})
	}
}

// TestAPI_ConcurrentRequests tests concurrent request handling
func TestAPI_ConcurrentRequests(t *testing.T) {
	app := fiber.New()

	app.Get("/api/concurrent-test", func(c *fiber.Ctx) error {
		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)
		return c.JSON(fiber.Map{"timestamp": time.Now().Unix()})
	})

	const numRequests = 20
	results := make(chan int, numRequests)

	// Launch concurrent requests
	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/api/concurrent-test", nil)
			resp, err := app.Test(req, 5000) // 5 second timeout
			if err != nil {
				results <- 0
				return
			}
			resp.Body.Close()
			results <- resp.StatusCode
		}()
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		status := <-results
		if status == http.StatusOK {
			successCount++
		}
	}

	assert.Equal(t, numRequests, successCount, "All concurrent requests should succeed")
}

// TestAPI_JSONParsing tests JSON request parsing
func TestAPI_JSONParsing(t *testing.T) {
	app := fiber.New()

	app.Post("/api/json-test", func(c *fiber.Ctx) error {
		var data map[string]interface{}
		if err := c.BodyParser(&data); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error":   true,
				"message": "Invalid JSON",
			})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"data":    data,
		})
	})

	tests := []struct {
		name           string
		body           string
		contentType    string
		expectedStatus int
		expectedError  bool
	}{
		{
			name:           "valid JSON",
			body:           `{"test": "value"}`,
			contentType:    "application/json",
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
		{
			name:           "invalid JSON",
			body:           `{"test": value}`, // missing quotes
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
		{
			name:           "empty body",
			body:           "",
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/json-test", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)

			if tt.expectedError {
				assert.True(t, response["error"].(bool))
			} else {
				assert.True(t, response["success"].(bool))
			}
		})
	}
}

// BenchmarkAPI_HealthCheck benchmarks the health check endpoint
func BenchmarkAPI_HealthCheck(b *testing.B) {
	app := fiber.New()
	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"service": "sermon-uploader-go",
		})
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/api/health", nil)
			resp, err := app.Test(req)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

// BenchmarkAPI_JSONParsing benchmarks JSON parsing performance
func BenchmarkAPI_JSONParsing(b *testing.B) {
	app := fiber.New()
	app.Post("/api/benchmark", func(c *fiber.Ctx) error {
		var data map[string]interface{}
		if err := c.BodyParser(&data); err != nil {
			return err
		}
		return c.JSON(data)
	})

	jsonData := `{"filename": "test.wav", "size": 1024, "type": "audio/wav"}`

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("POST", "/api/benchmark", strings.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}
