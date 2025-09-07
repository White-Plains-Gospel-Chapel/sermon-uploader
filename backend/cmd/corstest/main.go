package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// CORSTester performs CORS tests from command line (simulating browser behavior)
type CORSTester struct {
	backendURL string
	minioURL   string
	client     *http.Client
	verbose    bool
}

// NewCORSTester creates a new CORS tester
func NewCORSTester(backendURL, minioURL string, verbose bool) *CORSTester {
	return &CORSTester{
		backendURL: backendURL,
		minioURL:   minioURL,
		client:     &http.Client{Timeout: 30 * time.Second},
		verbose:    verbose,
	}
}

// RunAllTests runs all CORS tests
func (c *CORSTester) RunAllTests() error {
	fmt.Println("🧪 CORS Configuration Test Suite")
	fmt.Println("=" + repeatStr("=", 59))
	fmt.Printf("Backend: %s\n", c.backendURL)
	fmt.Printf("MinIO:   %s\n", c.minioURL)
	fmt.Println("=" + repeatStr("=", 59))

	tests := []struct {
		name string
		fn   func() error
	}{
		{"Backend Connectivity", c.testBackendConnectivity},
		{"MinIO Connectivity", c.testMinIOConnectivity},
		{"CORS Preflight Single File", c.testCORSPreflightSingle},
		{"CORS Bulk Upload (500MB+)", c.testCORSBulkUpload},
		{"CORS Different Origins", c.testCORSDifferentOrigins},
		{"Concurrent CORS Requests", c.testConcurrentCORS},
	}

	passed := 0
	failed := 0

	for i, test := range tests {
		fmt.Printf("\n[%d/%d] %s\n", i+1, len(tests), test.name)
		fmt.Print(repeatStr("-", 40) + "\n")
		
		startTime := time.Now()
		err := test.fn()
		duration := time.Since(startTime)

		if err != nil {
			fmt.Printf("❌ FAILED: %v (%.2fs)\n", err, duration.Seconds())
			failed++
		} else {
			fmt.Printf("✅ PASSED (%.2fs)\n", duration.Seconds())
			passed++
		}
	}

	fmt.Println("\n" + repeatStr("=", 60))
	fmt.Printf("Test Results: %d passed, %d failed\n", passed, failed)
	
	if failed > 0 {
		fmt.Println("❌ CORS configuration has issues - browser uploads may fail")
		c.printTroubleshooting()
		return fmt.Errorf("%d tests failed", failed)
	}

	fmt.Println("✅ CORS configuration is correct - browser uploads will work")
	return nil
}

func (c *CORSTester) testBackendConnectivity() error {
	resp, err := c.client.Get(c.backendURL + "/api/status")
	if err != nil {
		return fmt.Errorf("cannot reach backend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("backend returned status %d", resp.StatusCode)
	}

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return fmt.Errorf("invalid backend response: %w", err)
	}

	if c.verbose {
		fmt.Printf("  MinIO Connected: %v\n", status["minio_connected"])
		fmt.Printf("  Bucket: %v\n", status["bucket_name"])
	}

	return nil
}

func (c *CORSTester) testMinIOConnectivity() error {
	resp, err := c.client.Get(c.minioURL + "/minio/health/live")
	if err != nil {
		return fmt.Errorf("cannot reach MinIO: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MinIO health check returned status %d", resp.StatusCode)
	}

	if c.verbose {
		fmt.Println("  MinIO is healthy and accessible")
	}

	return nil
}

func (c *CORSTester) testCORSPreflightSingle() error {
	// Get presigned URL
	filename := fmt.Sprintf("cors-test-%d.wav", time.Now().Unix())
	presignedURL, uploadMethod, err := c.getPresignedURL(filename, 1024*1024*100) // 100MB
	if err != nil {
		return fmt.Errorf("failed to get presigned URL: %w", err)
	}

	if c.verbose {
		fmt.Printf("  File: %s\n", filename)
		fmt.Printf("  Method: %s\n", uploadMethod)
	}

	// Test CORS preflight
	req, err := http.NewRequest("OPTIONS", presignedURL, nil)
	if err != nil {
		return err
	}

	// Simulate browser preflight headers
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "PUT")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("CORS preflight request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("CORS preflight returned status %d (browser would block)", resp.StatusCode)
	}

	allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	if allowOrigin == "" {
		return fmt.Errorf("missing Access-Control-Allow-Origin header (browser would block)")
	}

	if c.verbose {
		fmt.Printf("  CORS Headers:\n")
		fmt.Printf("    Allow-Origin: %s\n", allowOrigin)
		if methods := resp.Header.Get("Access-Control-Allow-Methods"); methods != "" {
			fmt.Printf("    Allow-Methods: %s\n", methods)
		}
	}

	return nil
}

func (c *CORSTester) testCORSBulkUpload() error {
	files := []struct {
		filename string
		fileSize int64
	}{
		{fmt.Sprintf("bulk-1-%d.wav", time.Now().Unix()), 200 * 1024 * 1024}, // 200MB
		{fmt.Sprintf("bulk-2-%d.wav", time.Now().Unix()), 200 * 1024 * 1024}, // 200MB
		{fmt.Sprintf("bulk-3-%d.wav", time.Now().Unix()), 150 * 1024 * 1024}, // 150MB
	}

	fmt.Printf("  Testing %d files (550MB total)\n", len(files))

	// Get batch presigned URLs
	urls, err := c.getPresignedURLsBatch(files)
	if err != nil {
		return fmt.Errorf("failed to get batch URLs: %w", err)
	}

	// Test CORS for each URL in parallel
	var wg sync.WaitGroup
	errorsChan := make(chan error, len(files))

	for filename, urlInfo := range urls {
		wg.Add(1)
		go func(name, url string) {
			defer wg.Done()

			req, _ := http.NewRequest("OPTIONS", url, nil)
			req.Header.Set("Origin", "http://localhost:3000")
			req.Header.Set("Access-Control-Request-Method", "PUT")

			resp, err := c.client.Do(req)
			if err != nil {
				errorsChan <- fmt.Errorf("%s: %w", name, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 && resp.StatusCode != 204 {
				errorsChan <- fmt.Errorf("%s: status %d", name, resp.StatusCode)
				return
			}

			if c.verbose {
				fmt.Printf("    ✓ %s: CORS OK\n", name)
			}
		}(filename, urlInfo.UploadURL)
	}

	wg.Wait()
	close(errorsChan)

	// Check for errors
	var errs []error
	for err := range errorsChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("%d files failed CORS check", len(errs))
	}

	fmt.Printf("  All %d files passed CORS check\n", len(files))
	return nil
}

func (c *CORSTester) testCORSDifferentOrigins() error {
	origins := []string{
		"http://localhost:3000",
		"https://example.com",
		"https://wpgc.org",
	}

	filename := fmt.Sprintf("origin-test-%d.wav", time.Now().Unix())
	presignedURL, _, err := c.getPresignedURL(filename, 1024)
	if err != nil {
		return err
	}

	for _, origin := range origins {
		req, _ := http.NewRequest("OPTIONS", presignedURL, nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "PUT")

		resp, err := c.client.Do(req)
		if err != nil {
			return fmt.Errorf("origin %s: %w", origin, err)
		}
		resp.Body.Close()

		if resp.StatusCode != 200 && resp.StatusCode != 204 {
			return fmt.Errorf("origin %s: status %d", origin, resp.StatusCode)
		}

		if c.verbose {
			fmt.Printf("    ✓ Origin %s: Allowed\n", origin)
		}
	}

	return nil
}

func (c *CORSTester) testConcurrentCORS() error {
	concurrency := 10
	fmt.Printf("  Testing %d concurrent CORS requests\n", concurrency)

	filename := fmt.Sprintf("concurrent-%d.wav", time.Now().Unix())
	presignedURL, _, err := c.getPresignedURL(filename, 1024)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errorsChan := make(chan error, concurrency)
	successCount := 0
	var mu sync.Mutex

	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req, _ := http.NewRequest("OPTIONS", presignedURL, nil)
			req.Header.Set("Origin", "http://localhost:3000")
			req.Header.Set("Access-Control-Request-Method", "PUT")

			resp, err := c.client.Do(req)
			if err != nil {
				errorsChan <- err
				return
			}
			resp.Body.Close()

			if resp.StatusCode == 200 || resp.StatusCode == 204 {
				mu.Lock()
				successCount++
				mu.Unlock()
			} else {
				errorsChan <- fmt.Errorf("request %d: status %d", id, resp.StatusCode)
			}
		}(i)
	}

	wg.Wait()
	close(errorsChan)

	duration := time.Since(startTime)
	
	if successCount != concurrency {
		return fmt.Errorf("only %d/%d requests succeeded", successCount, concurrency)
	}

	fmt.Printf("  All %d requests succeeded in %.2fs\n", concurrency, duration.Seconds())
	return nil
}

// Helper methods

func (c *CORSTester) getPresignedURL(filename string, fileSize int64) (string, string, error) {
	payload, _ := json.Marshal(map[string]interface{}{
		"filename": filename,
		"fileSize": fileSize,
	})

	req, err := http.NewRequest("POST", c.backendURL+"/api/upload/presigned", bytes.NewReader(payload))
	if err != nil {
		return "", "", err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("backend error: %s", body)
	}

	var result struct {
		UploadURL    string `json:"uploadUrl"`
		UploadMethod string `json:"uploadMethod"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	return result.UploadURL, result.UploadMethod, nil
}

func (c *CORSTester) getPresignedURLsBatch(files []struct {
	filename string
	fileSize int64
}) (map[string]struct {
	UploadURL string `json:"uploadUrl"`
}, error) {
	var fileRequests []map[string]interface{}
	for _, f := range files {
		fileRequests = append(fileRequests, map[string]interface{}{
			"filename": f.filename,
			"fileSize": f.fileSize,
		})
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"files": fileRequests,
	})

	req, err := http.NewRequest("POST", c.backendURL+"/api/upload/presigned-batch", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend error: %s", body)
	}

	var result struct {
		Results map[string]struct {
			UploadURL string `json:"uploadUrl"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

func (c *CORSTester) printTroubleshooting() {
	fmt.Println("\n📋 Troubleshooting Steps:")
	fmt.Println("1. Check MinIO CORS configuration:")
	fmt.Println("   mc admin config get myminio api | grep cors")
	fmt.Println("2. Set CORS if missing:")
	fmt.Println("   mc admin config set myminio api cors_allow_origin='*'")
	fmt.Println("3. Restart MinIO:")
	fmt.Println("   mc admin service restart myminio")
	fmt.Println("4. Verify backend CORS middleware is configured")
	fmt.Println("5. Check network connectivity between services")
}

func repeatStr(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}

func main() {
	var (
		backendURL = flag.String("backend", "http://localhost:8000", "Backend API URL")
		minioURL   = flag.String("minio", "http://192.168.1.127:9000", "MinIO URL")
		verbose    = flag.Bool("v", false, "Verbose output")
	)

	flag.Parse()

	tester := NewCORSTester(*backendURL, *minioURL, *verbose)
	
	if err := tester.RunAllTests(); err != nil {
		os.Exit(1)
	}
}