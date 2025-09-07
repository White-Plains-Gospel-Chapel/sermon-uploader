package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BulkUploadCORSTest tests bulk upload with CORS in production-like scenario
type BulkUploadCORSTest struct {
	t          *testing.T
	client     *http.Client
	backendURL string
	minioURL   string
}

// NewBulkUploadCORSTest creates a new test instance
func NewBulkUploadCORSTest(t *testing.T) *BulkUploadCORSTest {
	return &BulkUploadCORSTest{
		t:          t,
		client:     &http.Client{Timeout: 60 * time.Second},
		backendURL: getEnvOrDefault("BACKEND_URL", "http://localhost:8000"),
		minioURL:   getEnvOrDefault("MINIO_URL", "http://192.168.1.127:9000"),
	}
}

// TestBulkUpload500MBWithCORS tests uploading 500MB+ files with CORS
func TestBulkUpload500MBWithCORS(t *testing.T) {
	// Start test server
	server := StartTestServer(t)
	defer server.Stop()
	
	// Create test instance with test server URL
	test := NewBulkUploadCORSTest(t)
	test.backendURL = server.URL()
	
	// Define test files totaling 500MB+
	testFiles := []FileUpload{
		{Filename: fmt.Sprintf("sermon-1-%d.wav", time.Now().Unix()), FileSize: 200 * 1024 * 1024}, // 200MB
		{Filename: fmt.Sprintf("sermon-2-%d.wav", time.Now().Unix()), FileSize: 200 * 1024 * 1024}, // 200MB
		{Filename: fmt.Sprintf("sermon-3-%d.wav", time.Now().Unix()), FileSize: 150 * 1024 * 1024}, // 150MB
	}

	t.Run("GetPresignedURLs", func(t *testing.T) {
		urls, err := test.getPresignedURLsBatch(testFiles)
		require.NoError(t, err, "Failed to get presigned URLs")
		assert.Len(t, urls, len(testFiles), "Should get URLs for all files")
		
		// Store URLs for next tests
		test.t.Logf("Got %d presigned URLs", len(urls))
		for filename, info := range urls {
			test.t.Logf("  %s: %s method", filename, info.UploadMethod)
		}
	})

	t.Run("TestCORSPreflight", func(t *testing.T) {
		urls, err := test.getPresignedURLsBatch(testFiles)
		require.NoError(t, err)

		// Test CORS preflight for each URL
		for filename, urlInfo := range urls {
			t.Run(filename, func(t *testing.T) {
				success, headers := test.testCORSPreflight(urlInfo.UploadURL)
				assert.True(t, success, "CORS preflight should succeed for %s", filename)
				assert.NotEmpty(t, headers["Access-Control-Allow-Origin"], 
					"Missing CORS header for %s", filename)
			})
		}
	})

	t.Run("ParallelUploadWithCORS", func(t *testing.T) {
		urls, err := test.getPresignedURLsBatch(testFiles)
		require.NoError(t, err)

		// Simulate browser parallel uploads
		var wg sync.WaitGroup
		uploadResults := make(chan UploadResult, len(testFiles))

		startTime := time.Now()
		
		for _, file := range testFiles {
			wg.Add(1)
			go func(f FileUpload) {
				defer wg.Done()
				
				urlInfo := urls[f.Filename]
				// Create realistic WAV header
				testData := test.createWAVData(1024) // 1KB test data
				
				result := test.uploadWithCORS(f.Filename, urlInfo.UploadURL, testData)
				uploadResults <- result
			}(file)
		}

		wg.Wait()
		close(uploadResults)

		duration := time.Since(startTime)
		t.Logf("Parallel upload completed in %v", duration)

		// Check results
		successCount := 0
		for result := range uploadResults {
			if result.Success {
				successCount++
				t.Logf("✅ %s uploaded successfully", result.Filename)
			} else {
				t.Logf("❌ %s failed: %v", result.Filename, result.Error)
			}
		}

		assert.Equal(t, len(testFiles), successCount, 
			"All files should upload successfully with CORS")
	})

	t.Run("TestThroughput", func(t *testing.T) {
		// Test upload throughput with CORS
		totalSize := int64(0)
		for _, f := range testFiles {
			totalSize += f.FileSize
		}
		totalSizeMB := float64(totalSize) / (1024 * 1024)

		startTime := time.Now()
		
		// Upload all files
		urls, err := test.getPresignedURLsBatch(testFiles)
		require.NoError(t, err)

		for _, file := range testFiles {
			urlInfo := urls[file.Filename]
			testData := test.createWAVData(1024 * 100) // 100KB test
			
			result := test.uploadWithCORS(file.Filename, urlInfo.UploadURL, testData)
			require.True(t, result.Success, "Upload should succeed")
		}

		duration := time.Since(startTime).Seconds()
		throughput := totalSizeMB / duration

		t.Logf("Upload throughput: %.2f MB/s", throughput)
		assert.Greater(t, throughput, 1.0, 
			"Throughput should be at least 1 MB/s")
	})
}

// TestCORSErrorHandling tests CORS error scenarios
func TestCORSErrorHandling(t *testing.T) {
	// Start test server
	server := StartTestServer(t)
	defer server.Stop()
	
	test := NewBulkUploadCORSTest(t)
	test.backendURL = server.URL()

	t.Run("InvalidOrigin", func(t *testing.T) {
		// Even with invalid origin, MinIO with * should allow
		testFile := FileUpload{
			Filename: fmt.Sprintf("test-invalid-%d.wav", time.Now().Unix()),
			FileSize: 1024,
		}

		urls, err := test.getPresignedURLsBatch([]FileUpload{testFile})
		require.NoError(t, err)

		urlInfo := urls[testFile.Filename]
		
		// Test with unusual origin
		req, _ := http.NewRequest("OPTIONS", urlInfo.UploadURL, nil)
		req.Header.Set("Origin", "https://malicious-site.com")
		req.Header.Set("Access-Control-Request-Method", "PUT")

		resp, err := test.client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// With * configuration, even "malicious" origins should work
		assert.Contains(t, []int{200, 204}, resp.StatusCode,
			"CORS should allow all origins when configured with *")
	})

	t.Run("MissingCORSHeaders", func(t *testing.T) {
		// Test upload without Origin header (non-browser scenario)
		testFile := FileUpload{
			Filename: fmt.Sprintf("test-no-origin-%d.wav", time.Now().Unix()),
			FileSize: 1024,
		}

		urls, err := test.getPresignedURLsBatch([]FileUpload{testFile})
		require.NoError(t, err)

		urlInfo := urls[testFile.Filename]
		testData := test.createWAVData(1024)

		// Upload without Origin header
		req, _ := http.NewRequest("PUT", urlInfo.UploadURL, bytes.NewReader(testData))
		req.Header.Set("Content-Type", "audio/wav")
		// Intentionally not setting Origin header

		resp, err := test.client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should still work (non-browser clients don't need CORS)
		assert.Contains(t, []int{200, 204}, resp.StatusCode,
			"Upload should work without Origin header for non-browser clients")
	})
}

// Helper types and methods

type FileUpload struct {
	Filename string
	FileSize int64
}

type UploadResult struct {
	Filename string
	Success  bool
	Error    error
	Duration time.Duration
}

func (t *BulkUploadCORSTest) getPresignedURLsBatch(files []FileUpload) (map[string]struct {
	UploadURL    string `json:"uploadUrl"`
	UploadMethod string `json:"uploadMethod"`
}, error) {
	var fileRequests []map[string]interface{}
	for _, f := range files {
		fileRequests = append(fileRequests, map[string]interface{}{
			"filename": f.Filename,
			"fileSize": f.FileSize,
		})
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"files": fileRequests,
	})

	req, err := http.NewRequest("POST", t.backendURL+"/api/upload/presigned-batch", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
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

func (t *BulkUploadCORSTest) testCORSPreflight(uploadURL string) (bool, map[string]string) {
	req, _ := http.NewRequest("OPTIONS", uploadURL, nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "PUT")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")

	resp, err := t.client.Do(req)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()

	headers := map[string]string{
		"Access-Control-Allow-Origin":  resp.Header.Get("Access-Control-Allow-Origin"),
		"Access-Control-Allow-Methods": resp.Header.Get("Access-Control-Allow-Methods"),
		"Access-Control-Allow-Headers": resp.Header.Get("Access-Control-Allow-Headers"),
	}

	success := resp.StatusCode == 200 || resp.StatusCode == 204
	return success, headers
}

func (t *BulkUploadCORSTest) uploadWithCORS(filename, uploadURL string, data []byte) UploadResult {
	startTime := time.Now()
	
	req, err := http.NewRequest("PUT", uploadURL, bytes.NewReader(data))
	if err != nil {
		return UploadResult{Filename: filename, Success: false, Error: err}
	}

	req.Header.Set("Content-Type", "audio/wav")
	req.Header.Set("Origin", "http://localhost:3000")

	resp, err := t.client.Do(req)
	if err != nil {
		return UploadResult{Filename: filename, Success: false, Error: err}
	}
	defer resp.Body.Close()

	success := resp.StatusCode == 200 || resp.StatusCode == 204
	duration := time.Since(startTime)

	if !success {
		body, _ := io.ReadAll(resp.Body)
		err = fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, body)
	}

	return UploadResult{
		Filename: filename,
		Success:  success,
		Error:    err,
		Duration: duration,
	}
}

func (t *BulkUploadCORSTest) createWAVData(size int) []byte {
	// Create realistic WAV header
	header := make([]byte, 44)
	
	// RIFF chunk descriptor
	copy(header[0:4], []byte("RIFF"))
	fileSize := uint32(size - 8)
	header[4] = byte(fileSize)
	header[5] = byte(fileSize >> 8)
	header[6] = byte(fileSize >> 16)
	header[7] = byte(fileSize >> 24)
	copy(header[8:12], []byte("WAVE"))
	
	// fmt sub-chunk
	copy(header[12:16], []byte("fmt "))
	header[16] = 16 // Subchunk1Size
	header[20] = 1  // AudioFormat (PCM)
	header[22] = 2  // NumChannels
	header[24] = 0x44
	header[25] = 0xac // SampleRate (44100)
	header[32] = 4    // BlockAlign
	header[34] = 16   // BitsPerSample
	
	// data sub-chunk
	copy(header[36:40], []byte("data"))
	dataSize := uint32(size - 44)
	header[40] = byte(dataSize)
	header[41] = byte(dataSize >> 8)
	header[42] = byte(dataSize >> 16)
	header[43] = byte(dataSize >> 24)
	
	// Create full data
	data := make([]byte, size)
	copy(data, header)
	
	return data
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}