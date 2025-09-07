// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const (
	// API endpoints
	apiHost = "https://sermons.wpgc.church"
	// Test file location on Pi
	testFilesDir = "/home/gaius/data/sermon-test-wavs"
	// Minimum file size for testing (500MB)
	minFileSize = 500 * 1024 * 1024
)

// UploadClient mimics the frontend client behavior
type UploadClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewUploadClient creates a new upload client
func NewUploadClient(baseURL string) *UploadClient {
	return &UploadClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Minute, // Long timeout for large files
		},
	}
}

// PresignedURLResponse represents the API response
type PresignedURLResponse struct {
	Success      bool   `json:"success"`
	IsDuplicate  bool   `json:"isDuplicate"`
	UploadURL    string `json:"uploadUrl"`
	IsLargeFile  bool   `json:"isLargeFile"`
	UploadMethod string `json:"uploadMethod"`
}

// GetPresignedURL requests a presigned URL from the API (mimics frontend)
func (c *UploadClient) GetPresignedURL(filename string, fileSize int64) (*PresignedURLResponse, error) {
	payload := map[string]interface{}{
		"filename": filename,
		"fileSize": fileSize,
	}
	
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	req, err := http.NewRequest("POST", c.baseURL+"/api/upload/presigned", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	var result PresignedURLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return &result, nil
}

// UploadFile uploads a file using the presigned URL (mimics frontend behavior)
func (c *UploadClient) UploadFile(presignedURL string, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	
	// Create request with file stream (no memory loading!)
	req, err := http.NewRequest("PUT", presignedURL, file)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}
	
	req.Header.Set("Content-Type", "audio/wav")
	req.ContentLength = stat.Size()
	
	// Upload with progress tracking
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()
	
	duration := time.Since(start)
	speedMBps := float64(stat.Size()) / duration.Seconds() / 1024 / 1024
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, body)
	}
	
	fmt.Printf("✓ Uploaded %s (%d MB) in %.2fs (%.2f MB/s)\n", 
		filepath.Base(filePath), stat.Size()/1024/1024, duration.Seconds(), speedMBps)
	
	return nil
}

// CompleteUpload notifies the API that upload is complete
func (c *UploadClient) CompleteUpload(filename string) error {
	payload := map[string]string{"filename": filename}
	body, _ := json.Marshal(payload)
	
	req, err := http.NewRequest("POST", c.baseURL+"/api/upload/complete", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	return nil
}

// TestSingleLargeFileUpload tests uploading a single 500MB+ file
func TestSingleLargeFileUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	client := NewUploadClient(apiHost)
	
	// Find a large test file
	testFile := findLargeTestFile(t)
	if testFile == "" {
		t.Skip("No 500MB+ test files found")
	}
	
	fileInfo, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}
	
	filename := filepath.Base(testFile)
	fileSize := fileInfo.Size()
	
	t.Logf("Testing with file: %s (%.2f GB)", filename, float64(fileSize)/1024/1024/1024)
	
	// Step 1: Get presigned URL (mimics frontend)
	t.Log("Requesting presigned URL...")
	presignedResp, err := client.GetPresignedURL(filename, fileSize)
	if err != nil {
		t.Fatalf("Failed to get presigned URL: %v", err)
	}
	
	if presignedResp.IsDuplicate {
		t.Log("File is duplicate, skipping upload")
		return
	}
	
	// Verify we got direct MinIO URL for large file
	if fileSize > 100*1024*1024 && !presignedResp.IsLargeFile {
		t.Error("Expected IsLargeFile=true for files >100MB")
	}
	
	if fileSize > 100*1024*1024 && presignedResp.UploadMethod != "direct_minio" {
		t.Errorf("Expected UploadMethod=direct_minio for large files, got %s", presignedResp.UploadMethod)
	}
	
	t.Logf("Got presigned URL (method: %s)", presignedResp.UploadMethod)
	
	// Step 2: Upload file (mimics frontend)
	t.Log("Uploading file...")
	if err := client.UploadFile(presignedResp.UploadURL, testFile); err != nil {
		t.Fatalf("Failed to upload file: %v", err)
	}
	
	// Step 3: Complete upload
	t.Log("Completing upload...")
	if err := client.CompleteUpload(filename); err != nil {
		t.Errorf("Failed to complete upload: %v", err)
	}
	
	t.Log("✅ Single large file upload test passed!")
}

// TestBulkLargeFileUpload tests uploading multiple 500MB+ files concurrently
func TestBulkLargeFileUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	client := NewUploadClient(apiHost)
	
	// Find multiple large test files
	testFiles := findMultipleLargeTestFiles(t, 3)
	if len(testFiles) == 0 {
		t.Skip("No 500MB+ test files found")
	}
	
	t.Logf("Testing bulk upload with %d files", len(testFiles))
	
	// Upload files concurrently (mimics frontend batch upload)
	var wg sync.WaitGroup
	errors := make(chan error, len(testFiles))
	
	for _, testFile := range testFiles {
		wg.Add(1)
		go func(filePath string) {
			defer wg.Done()
			
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				errors <- fmt.Errorf("failed to stat %s: %w", filePath, err)
				return
			}
			
			filename := filepath.Base(filePath)
			fileSize := fileInfo.Size()
			
			// Get presigned URL
			presignedResp, err := client.GetPresignedURL(filename, fileSize)
			if err != nil {
				errors <- fmt.Errorf("failed to get presigned URL for %s: %w", filename, err)
				return
			}
			
			if presignedResp.IsDuplicate {
				t.Logf("File %s is duplicate, skipping", filename)
				return
			}
			
			// Upload file
			if err := client.UploadFile(presignedResp.UploadURL, filePath); err != nil {
				errors <- fmt.Errorf("failed to upload %s: %w", filename, err)
				return
			}
			
			// Complete upload
			if err := client.CompleteUpload(filename); err != nil {
				t.Logf("Warning: failed to complete upload for %s: %v", filename, err)
			}
		}(testFile)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	var failCount int
	for err := range errors {
		t.Errorf("Upload error: %v", err)
		failCount++
	}
	
	if failCount == 0 {
		t.Logf("✅ Bulk upload test passed! Successfully uploaded %d files", len(testFiles))
	} else {
		t.Fatalf("❌ Bulk upload failed with %d errors", failCount)
	}
}

// Helper function to find a large test file
func findLargeTestFile(t *testing.T) string {
	t.Helper()
	
	// Look for test files in the configured directory
	pattern := filepath.Join(testFilesDir, "**/*.wav")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Logf("Error searching for test files: %v", err)
		return ""
	}
	
	// Find first file >500MB
	for _, file := range matches {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		if info.Size() >= minFileSize {
			return file
		}
	}
	
	return ""
}

// Helper function to find multiple large test files
func findMultipleLargeTestFiles(t *testing.T, count int) []string {
	t.Helper()
	
	var files []string
	
	// For testing on Pi, use specific directory
	testDirs := []string{
		"/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files",
		"/home/gaius/data/sermon-test-wavs",
	}
	
	for _, dir := range testDirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip inaccessible paths
			}
			
			if !info.IsDir() && filepath.Ext(path) == ".wav" && info.Size() >= minFileSize {
				files = append(files, path)
				if len(files) >= count {
					return filepath.SkipDir
				}
			}
			return nil
		})
		
		if err != nil {
			t.Logf("Error walking directory %s: %v", dir, err)
		}
		
		if len(files) >= count {
			break
		}
	}
	
	return files
}

// TestHealthCheck verifies the API is accessible
func TestHealthCheck(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}
	
	resp, err := client.Get(apiHost + "/api/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Health check returned status %d", resp.StatusCode)
	}
	
	t.Log("✅ API is healthy")
}