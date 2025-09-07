package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// CORSTestSuite tests CORS configuration for browser-based uploads
type CORSTestSuite struct {
	suite.Suite
	backendURL string
	minioURL   string
	client     *http.Client
}

// SetupSuite runs before all tests
func (suite *CORSTestSuite) SetupSuite() {
	suite.backendURL = "http://localhost:8000"
	suite.minioURL = "http://192.168.1.127:9000"
	suite.client = &http.Client{
		Timeout: 30 * time.Second,
	}
}

// TestCORSPreflightForDirectMinIO tests CORS preflight requests to MinIO
func (suite *CORSTestSuite) TestCORSPreflightForDirectMinIO() {
	// Arrange
	testFiles := []struct {
		filename string
		fileSize int64
	}{
		{"test-small-" + fmt.Sprint(time.Now().Unix()) + ".wav", 50 * 1024 * 1024},  // 50MB
		{"test-large-" + fmt.Sprint(time.Now().Unix()) + ".wav", 250 * 1024 * 1024}, // 250MB
	}

	// Act - Get presigned URLs
	presignedURLs, err := suite.getPresignedURLsBatch(testFiles)
	
	// Assert
	suite.Require().NoError(err, "Failed to get presigned URLs")
	suite.Require().NotEmpty(presignedURLs, "No presigned URLs returned")

	for filename, urlInfo := range presignedURLs {
		suite.Run("Preflight_"+filename, func() {
			// Act - Send OPTIONS request (browser preflight)
			req, err := http.NewRequest("OPTIONS", urlInfo.UploadURL, nil)
			suite.Require().NoError(err)

			// Add CORS preflight headers (what browsers send)
			req.Header.Set("Origin", "http://localhost:3000")
			req.Header.Set("Access-Control-Request-Method", "PUT")
			req.Header.Set("Access-Control-Request-Headers", "Content-Type")

			resp, err := suite.client.Do(req)
			suite.Require().NoError(err)
			defer resp.Body.Close()

			// Assert - CORS preflight should succeed
			suite.Assert().Contains([]int{200, 204}, resp.StatusCode, 
				"CORS preflight failed - browser uploads would be blocked")

			// Assert - Required CORS headers must be present
			allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
			suite.Assert().NotEmpty(allowOrigin, 
				"Missing Access-Control-Allow-Origin header - browser would block request")

			allowMethods := resp.Header.Get("Access-Control-Allow-Methods")
			if allowMethods != "" {
				suite.Assert().Contains(allowMethods, "PUT", 
					"PUT method not allowed in CORS response")
			}
		})
	}
}

// TestActualUploadWithCORS tests actual file upload with CORS headers
func (suite *CORSTestSuite) TestActualUploadWithCORS() {
	// Arrange
	testFile := struct {
		filename string
		fileSize int64
		content  []byte
	}{
		filename: "test-upload-" + fmt.Sprint(time.Now().Unix()) + ".wav",
		fileSize: 1024, // 1KB test file
		content:  bytes.Repeat([]byte("TEST"), 256),
	}

	// Act - Get presigned URL
	presignedURL, err := suite.getPresignedURL(testFile.filename, testFile.fileSize)
	suite.Require().NoError(err)

	// Act - Upload with Origin header (browser behavior)
	req, err := http.NewRequest("PUT", presignedURL, bytes.NewReader(testFile.content))
	suite.Require().NoError(err)

	req.Header.Set("Content-Type", "audio/wav")
	req.Header.Set("Origin", "http://localhost:3000") // Browser always sends this

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	// Assert - Upload should succeed
	suite.Assert().Contains([]int{200, 204}, resp.StatusCode,
		"Upload failed with CORS - browser uploads would fail")

	// Check for CORS header in response (optional but good practice)
	corsHeader := resp.Header.Get("Access-Control-Allow-Origin")
	if corsHeader != "" {
		suite.T().Logf("CORS header present in response: %s", corsHeader)
	}
}

// TestBulkUploadCORS tests CORS for bulk upload scenarios
func (suite *CORSTestSuite) TestBulkUploadCORS() {
	// Arrange - 500MB+ total upload
	testFiles := []struct {
		filename string
		fileSize int64
	}{
		{"bulk-1-" + fmt.Sprint(time.Now().Unix()) + ".wav", 200 * 1024 * 1024}, // 200MB
		{"bulk-2-" + fmt.Sprint(time.Now().Unix()) + ".wav", 200 * 1024 * 1024}, // 200MB
		{"bulk-3-" + fmt.Sprint(time.Now().Unix()) + ".wav", 150 * 1024 * 1024}, // 150MB
	}

	// Act - Get batch presigned URLs
	presignedURLs, err := suite.getPresignedURLsBatch(testFiles)
	suite.Require().NoError(err)

	// Assert - All files should have valid URLs
	suite.Assert().Len(presignedURLs, len(testFiles))

	// Test parallel CORS checks (simulating browser bulk upload)
	type corsResult struct {
		filename string
		success  bool
		error    error
	}

	results := make(chan corsResult, len(testFiles))

	for filename, urlInfo := range presignedURLs {
		go func(name string, url string) {
			// Test CORS preflight
			req, err := http.NewRequest("OPTIONS", url, nil)
			if err != nil {
				results <- corsResult{name, false, err}
				return
			}

			req.Header.Set("Origin", "http://localhost:3000")
			req.Header.Set("Access-Control-Request-Method", "PUT")

			resp, err := suite.client.Do(req)
			if err != nil {
				results <- corsResult{name, false, err}
				return
			}
			defer resp.Body.Close()

			success := resp.StatusCode == 200 || resp.StatusCode == 204
			results <- corsResult{name, success, nil}
		}(filename, urlInfo.UploadURL)
	}

	// Collect results
	successCount := 0
	for i := 0; i < len(testFiles); i++ {
		result := <-results
		if result.success {
			successCount++
			suite.T().Logf("✅ CORS check passed for %s", result.filename)
		} else {
			suite.T().Logf("❌ CORS check failed for %s: %v", result.filename, result.error)
		}
	}

	// Assert - All CORS checks should pass
	suite.Assert().Equal(len(testFiles), successCount,
		"Not all files passed CORS check - bulk upload would fail in browser")
}

// TestCORSWithDifferentOrigins tests CORS with various origins
func (suite *CORSTestSuite) TestCORSWithDifferentOrigins() {
	origins := []string{
		"http://localhost:3000",
		"https://example.com",
		"https://wpgc.org",
	}

	for _, origin := range origins {
		suite.Run("Origin_"+origin, func() {
			// Arrange
			testFile := "test-origin-" + fmt.Sprint(time.Now().Unix()) + ".wav"
			
			// Act - Get presigned URL
			presignedURL, err := suite.getPresignedURL(testFile, 1024)
			suite.Require().NoError(err)

			// Act - Test CORS with this origin
			req, err := http.NewRequest("OPTIONS", presignedURL, nil)
			suite.Require().NoError(err)

			req.Header.Set("Origin", origin)
			req.Header.Set("Access-Control-Request-Method", "PUT")

			resp, err := suite.client.Do(req)
			suite.Require().NoError(err)
			defer resp.Body.Close()

			// Assert - Should allow all origins (configured as "*")
			suite.Assert().Contains([]int{200, 204}, resp.StatusCode,
				"CORS should allow origin: %s", origin)

			allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
			suite.Assert().True(
				allowOrigin == "*" || allowOrigin == origin,
				"Expected CORS to allow origin %s, got %s", origin, allowOrigin,
			)
		})
	}
}

// Helper methods

func (suite *CORSTestSuite) getPresignedURL(filename string, fileSize int64) (string, error) {
	payload := map[string]interface{}{
		"filename": filename,
		"fileSize": fileSize,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", suite.backendURL+"/api/upload/presigned", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := suite.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get presigned URL: %s", body)
	}

	var result struct {
		UploadURL string `json:"uploadUrl"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.UploadURL, nil
}

func (suite *CORSTestSuite) getPresignedURLsBatch(files []struct {
	filename string
	fileSize int64
}) (map[string]struct {
	UploadURL    string `json:"uploadUrl"`
	UploadMethod string `json:"uploadMethod"`
}, error) {
	var fileRequests []map[string]interface{}
	for _, f := range files {
		fileRequests = append(fileRequests, map[string]interface{}{
			"filename": f.filename,
			"fileSize": f.fileSize,
		})
	}

	payload := map[string]interface{}{
		"files": fileRequests,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", suite.backendURL+"/api/upload/presigned-batch", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get batch presigned URLs: %s", body)
	}

	var result struct {
		Results map[string]struct {
			UploadURL    string `json:"uploadUrl"`
			UploadMethod string `json:"uploadMethod"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

// TestCORSTestSuite runs the test suite
func TestCORSTestSuite(t *testing.T) {
	suite.Run(t, new(CORSTestSuite))
}

// BenchmarkCORSPreflight benchmarks CORS preflight performance
func BenchmarkCORSPreflight(b *testing.B) {
	client := &http.Client{Timeout: 10 * time.Second}
	backendURL := "http://localhost:8000"

	// Get a presigned URL once
	payload := map[string]interface{}{
		"filename": "benchmark-test.wav",
		"fileSize": 1024,
	}
	body, _ := json.Marshal(payload)
	
	req, _ := http.NewRequest("POST", backendURL+"/api/upload/presigned", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)
	require.NoError(b, err)
	defer resp.Body.Close()

	var result struct {
		UploadURL string `json:"uploadUrl"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(b, err)

	b.ResetTimer()

	// Benchmark CORS preflight requests
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest("OPTIONS", result.UploadURL, nil)
			req.Header.Set("Origin", "http://localhost:3000")
			req.Header.Set("Access-Control-Request-Method", "PUT")

			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}
	})
}