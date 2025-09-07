// +build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	// API endpoints
	apiHost = "https://sermons.wpgc.church"
	
	// Pi with test files
	piTestHost = "gaius@ridgepoint" // 192.168.1.195
	testFilesDir = "/home/gaius/data/sermon-test-wavs"
	
	// Pi with MinIO and API
	piMinioHost = "192.168.1.127"
	
	// MinIO configuration
	minioEndpoint = "192.168.1.127:9000"
	minioAccessKey = "gaius"
	minioSecretKey = "John 3:16"
	minioBucket = "sermons"
	
	// Minimum file size for testing (500MB)
	minFileSize = 500 * 1024 * 1024
)

// UploadClient mimics the frontend client behavior
type UploadClient struct {
	baseURL    string
	httpClient *http.Client
	minioClient *minio.Client
}

// NewUploadClient creates a new upload client with MinIO access
func NewUploadClient(baseURL string) (*UploadClient, error) {
	// Create MinIO client for duplicate management
	minioClient, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}
	
	return &UploadClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Minute, // Long timeout for large files
		},
		minioClient: minioClient,
	}, nil
}

// TestFile represents a file on the Pi
type TestFile struct {
	Path string
	Name string
	Size int64
}

// GetTestFilesFromPi connects to the Pi via SSH and lists available test files
func GetTestFilesFromPi(t *testing.T) []TestFile {
	t.Helper()
	
	// SSH command to list files with sizes
	cmd := exec.Command("ssh", piTestHost, 
		fmt.Sprintf("find %s -type f -name '*.wav' -size +%dc -exec stat -c '%%s %%n' {} \\;", 
			testFilesDir, minFileSize))
	
	output, err := cmd.Output()
	if err != nil {
		t.Logf("Failed to connect to Pi at %s: %v", piTestHost, err)
		t.Log("Make sure you have SSH access configured to ridgepoint")
		return nil
	}
	
	var files []TestFile
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		
		size := int64(0)
		fmt.Sscanf(parts[0], "%d", &size)
		path := parts[1]
		
		if size >= minFileSize {
			files = append(files, TestFile{
				Path: path,
				Name: filepath.Base(path),
				Size: size,
			})
		}
	}
	
	t.Logf("Found %d test files on Pi (>500MB)", len(files))
	return files
}

// PresignedURLResponse represents the API response
type PresignedURLResponse struct {
	Success      bool   `json:"success"`
	IsDuplicate  bool   `json:"isDuplicate"`
	UploadURL    string `json:"uploadUrl"`
	IsLargeFile  bool   `json:"isLargeFile"`
	UploadMethod string `json:"uploadMethod"`
	Message      string `json:"message,omitempty"`
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

// ClearDuplicateFromBucket removes a duplicate file from MinIO bucket
func (c *UploadClient) ClearDuplicateFromBucket(filename string) error {
	ctx := context.Background()
	
	// List objects with the filename prefix
	objectsCh := c.minioClient.ListObjects(ctx, minioBucket, minio.ListObjectsOptions{
		Prefix: filename,
	})
	
	var objectsToDelete []string
	for object := range objectsCh {
		if object.Err != nil {
			return fmt.Errorf("error listing objects: %w", object.Err)
		}
		objectsToDelete = append(objectsToDelete, object.Key)
	}
	
	// Delete found objects
	for _, objectName := range objectsToDelete {
		err := c.minioClient.RemoveObject(ctx, minioBucket, objectName, minio.RemoveObjectOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete object %s: %w", objectName, err)
		}
		fmt.Printf("Deleted duplicate: %s\n", objectName)
	}
	
	return nil
}

// UploadFileFromPi uploads a file from Pi through presigned URL (streaming)
func (c *UploadClient) UploadFileFromPi(presignedURL string, testFile TestFile) error {
	// Use curl -T for efficient streaming upload (doesn't load file into memory)
	// The -T flag streams the file directly without loading it into memory
	curlCmd := fmt.Sprintf(
		"curl -X PUT -H 'Content-Type: audio/wav' -T '%s' '%s' -s -w '%%{http_code}'",
		testFile.Path, presignedURL)
	
	cmd := exec.Command("ssh", piTestHost, curlCmd)
	
	start := time.Now()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("upload failed: %w\nOutput: %s", err, output)
	}
	
	// Check HTTP status code
	statusCode := strings.TrimSpace(string(output))
	if statusCode != "200" && statusCode != "204" {
		return fmt.Errorf("upload returned HTTP %s", statusCode)
	}
	
	duration := time.Since(start)
	speedMBps := float64(testFile.Size) / duration.Seconds() / 1024 / 1024
	
	fmt.Printf("✓ Uploaded %s (%d MB) in %.2fs (%.2f MB/s)\n", 
		testFile.Name, testFile.Size/1024/1024, duration.Seconds(), speedMBps)
	
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
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("complete upload returned status %d: %s", resp.StatusCode, body)
	}
	
	return nil
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
	
	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}
	
	t.Logf("✅ API is healthy: %+v", health)
}

// TestSingleLargeFileUpload tests uploading a single 500MB+ file from Pi
func TestSingleLargeFileUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	client, err := NewUploadClient(apiHost)
	if err != nil {
		t.Fatalf("Failed to create upload client: %v", err)
	}
	
	// Get test files from Pi
	testFiles := GetTestFilesFromPi(t)
	if len(testFiles) == 0 {
		t.Skip("No 500MB+ test files found on Pi")
	}
	
	// Use first test file
	testFile := testFiles[0]
	t.Logf("Testing with file from Pi: %s (%.2f GB)", testFile.Name, float64(testFile.Size)/1024/1024/1024)
	
	// Step 1: Get presigned URL (mimics frontend)
	t.Log("Requesting presigned URL...")
	presignedResp, err := client.GetPresignedURL(testFile.Name, testFile.Size)
	if err != nil {
		t.Fatalf("Failed to get presigned URL: %v", err)
	}
	
	// Handle duplicate files
	if presignedResp.IsDuplicate {
		t.Logf("File is duplicate, clearing from bucket and retrying...")
		if err := client.ClearDuplicateFromBucket(testFile.Name); err != nil {
			t.Logf("Warning: Failed to clear duplicate: %v", err)
		}
		
		// Retry after clearing
		time.Sleep(2 * time.Second)
		presignedResp, err = client.GetPresignedURL(testFile.Name, testFile.Size)
		if err != nil {
			t.Fatalf("Failed to get presigned URL after clearing duplicate: %v", err)
		}
		
		if presignedResp.IsDuplicate {
			t.Log("File still marked as duplicate after clearing, skipping upload")
			return
		}
	}
	
	// Verify we got direct MinIO URL for large file
	if testFile.Size > 100*1024*1024 && !presignedResp.IsLargeFile {
		t.Error("Expected IsLargeFile=true for files >100MB")
	}
	
	if testFile.Size > 100*1024*1024 && presignedResp.UploadMethod != "direct_minio" {
		t.Errorf("Expected UploadMethod=direct_minio for large files, got %s", presignedResp.UploadMethod)
	}
	
	t.Logf("Got presigned URL (method: %s, isLarge: %v)", presignedResp.UploadMethod, presignedResp.IsLargeFile)
	
	// Step 2: Upload file from Pi (mimics frontend)
	t.Log("Uploading file from Pi...")
	if err := client.UploadFileFromPi(presignedResp.UploadURL, testFile); err != nil {
		t.Fatalf("Failed to upload file: %v", err)
	}
	
	// Step 3: Complete upload
	t.Log("Completing upload...")
	if err := client.CompleteUpload(testFile.Name); err != nil {
		t.Errorf("Failed to complete upload: %v", err)
	}
	
	t.Log("✅ Single large file upload test passed!")
}

// TestBulkLargeFileUpload tests uploading multiple 500MB+ files concurrently from Pi
func TestBulkLargeFileUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	client, err := NewUploadClient(apiHost)
	if err != nil {
		t.Fatalf("Failed to create upload client: %v", err)
	}
	
	// Get test files from Pi
	testFiles := GetTestFilesFromPi(t)
	if len(testFiles) == 0 {
		t.Skip("No 500MB+ test files found on Pi")
	}
	
	// Limit to 3 files for bulk test
	maxFiles := 3
	if len(testFiles) > maxFiles {
		testFiles = testFiles[:maxFiles]
	}
	
	t.Logf("Testing bulk upload with %d files from Pi", len(testFiles))
	
	// Upload files concurrently (mimics frontend batch upload)
	var wg sync.WaitGroup
	errors := make(chan error, len(testFiles))
	
	for _, testFile := range testFiles {
		wg.Add(1)
		go func(file TestFile) {
			defer wg.Done()
			
			// Get presigned URL
			presignedResp, err := client.GetPresignedURL(file.Name, file.Size)
			if err != nil {
				errors <- fmt.Errorf("failed to get presigned URL for %s: %w", file.Name, err)
				return
			}
			
			// Handle duplicates
			if presignedResp.IsDuplicate {
				t.Logf("File %s is duplicate, clearing and retrying...", file.Name)
				if err := client.ClearDuplicateFromBucket(file.Name); err != nil {
					t.Logf("Warning: Failed to clear duplicate %s: %v", file.Name, err)
				}
				
				time.Sleep(2 * time.Second)
				presignedResp, err = client.GetPresignedURL(file.Name, file.Size)
				if err != nil {
					errors <- fmt.Errorf("failed to get presigned URL for %s after clearing: %w", file.Name, err)
					return
				}
				
				if presignedResp.IsDuplicate {
					t.Logf("File %s still duplicate after clearing, skipping", file.Name)
					return
				}
			}
			
			// Upload file from Pi
			if err := client.UploadFileFromPi(presignedResp.UploadURL, file); err != nil {
				errors <- fmt.Errorf("failed to upload %s: %w", file.Name, err)
				return
			}
			
			// Complete upload
			if err := client.CompleteUpload(file.Name); err != nil {
				t.Logf("Warning: failed to complete upload for %s: %v", file.Name, err)
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
		t.Logf("✅ Bulk upload test passed! Successfully uploaded %d files from Pi", len(testFiles))
	} else {
		t.Fatalf("❌ Bulk upload failed with %d errors", failCount)
	}
}

// TestClearAndReupload tests clearing duplicates and re-uploading
func TestClearAndReupload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	client, err := NewUploadClient(apiHost)
	if err != nil {
		t.Fatalf("Failed to create upload client: %v", err)
	}
	
	// Get one test file from Pi
	testFiles := GetTestFilesFromPi(t)
	if len(testFiles) == 0 {
		t.Skip("No test files found on Pi")
	}
	
	testFile := testFiles[0]
	t.Logf("Testing duplicate handling with: %s", testFile.Name)
	
	// First upload
	t.Log("First upload attempt...")
	presignedResp, err := client.GetPresignedURL(testFile.Name, testFile.Size)
	if err != nil {
		t.Fatalf("Failed to get presigned URL: %v", err)
	}
	
	if !presignedResp.IsDuplicate {
		// Upload if not duplicate
		if err := client.UploadFileFromPi(presignedResp.UploadURL, testFile); err != nil {
			t.Fatalf("Failed to upload: %v", err)
		}
		client.CompleteUpload(testFile.Name)
	}
	
	// Second attempt should show duplicate
	t.Log("Second upload attempt (should be duplicate)...")
	presignedResp, err = client.GetPresignedURL(testFile.Name, testFile.Size)
	if err != nil {
		t.Fatalf("Failed to get presigned URL: %v", err)
	}
	
	if !presignedResp.IsDuplicate {
		t.Error("Expected file to be marked as duplicate on second attempt")
	}
	
	// Clear and retry
	t.Log("Clearing duplicate from bucket...")
	if err := client.ClearDuplicateFromBucket(testFile.Name); err != nil {
		t.Fatalf("Failed to clear duplicate: %v", err)
	}
	
	time.Sleep(2 * time.Second)
	
	// Third attempt after clearing
	t.Log("Third upload attempt (after clearing)...")
	presignedResp, err = client.GetPresignedURL(testFile.Name, testFile.Size)
	if err != nil {
		t.Fatalf("Failed to get presigned URL: %v", err)
	}
	
	if presignedResp.IsDuplicate {
		t.Error("File still marked as duplicate after clearing from bucket")
	}
	
	t.Log("✅ Duplicate handling test passed!")
}