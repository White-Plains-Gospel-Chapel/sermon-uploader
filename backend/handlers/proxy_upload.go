package handlers

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
)

// ProxyUpload handles file uploads by proxying them through the backend to MinIO
// This bypasses browser restrictions on accessing private IP addresses
func (h *Handlers) ProxyUpload(c *fiber.Ctx) error {
	// Get filename from query params or form
	filename := c.Query("filename")
	if filename == "" {
		filename = c.FormValue("filename")
	}
	if filename == "" {
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": "Filename is required",
		})
	}

	// Get the file from the request
	file, err := c.FormFile("file")
	if err != nil {
		// For PUT requests or raw body uploads, handle as stream
		if c.Method() == "PUT" || c.Get("Content-Type") == "audio/wav" || 
		   c.Get("Content-Type") == "application/octet-stream" || 
		   c.Get("Content-Type") == "text/plain" {
			// Handle raw body upload
			return h.proxyRawUpload(c, filename)
		}
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": "No file provided",
		})
	}

	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to open uploaded file",
		})
	}
	defer src.Close()

	// Upload to MinIO using the service's proxy upload method
	ctx := context.Background()
	info, err := h.minioService.ProxyUploadFile(
		ctx,
		filename,
		src,
		file.Size,
		file.Header.Get("Content-Type"),
	)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("Failed to upload to storage: %v", err),
		})
	}

	// Process the uploaded file (extract metadata, etc.)
	go func() {
		ctx := context.Background()
		_ = h.minioService.ProcessUploadedFile(ctx, filename)
	}()

	return c.JSON(fiber.Map{
		"success":  true,
		"filename": filename,
		"size":     info.Size,
		"etag":     info.ETag,
		"message":  "File uploaded successfully via proxy",
	})
}

// proxyRawUpload handles raw body uploads (for streaming large files)
func (h *Handlers) proxyRawUpload(c *fiber.Ctx, filename string) error {
	// Get content length
	contentLength := c.Get("Content-Length")
	var fileSize int64 = -1
	if contentLength != "" {
		fmt.Sscanf(contentLength, "%d", &fileSize)
	}
	
	// Log the upload attempt
	fmt.Printf("Proxy upload attempt: filename=%s, size=%d bytes (%.1f MB)\n", 
		filename, fileSize, float64(fileSize)/(1024*1024))

	// Check if we're hitting CloudFlare's limit
	if fileSize > 100*1024*1024 {
		fmt.Printf("WARNING: File size %d exceeds CloudFlare 100MB limit\n", fileSize)
	}

	// Stream the body directly to MinIO
	ctx := context.Background()
	reader := c.Context().RequestBodyStream()
	
	info, err := h.minioService.ProxyUploadFile(
		ctx,
		filename,
		reader,
		fileSize,
		c.Get("Content-Type", "application/octet-stream"),
	)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("Failed to upload to storage: %v", err),
		})
	}

	// Process the uploaded file (extract metadata, etc.)
	go func() {
		ctx := context.Background()
		_ = h.minioService.ProcessUploadedFile(ctx, filename)
	}()

	return c.JSON(fiber.Map{
		"success":  true,
		"filename": filename,
		"size":     info.Size,
		"etag":     info.ETag,
		"message":  "File uploaded successfully via proxy",
	})
}

// GetProxyUploadURL returns a URL that goes through the backend proxy
// instead of direct to MinIO (to bypass browser Private Network Access restrictions)
func (h *Handlers) GetProxyUploadURL(c *fiber.Ctx) error {
	var req struct {
		Filename string `json:"filename"`
		FileSize int64  `json:"fileSize"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request format",
		})
	}

	// Check for duplicates
	exists, err := h.minioService.FileExists(req.Filename)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to check file existence",
		})
	}

	if exists {
		return c.Status(409).JSON(fiber.Map{
			"error":       true,
			"isDuplicate": true,
			"message":     "File already exists",
			"filename":    req.Filename,
		})
	}

	// Generate proxy upload URL (through our backend)
	scheme := "https"
	if c.Protocol() == "http" {
		scheme = "http"
	}
	
	// Use the actual domain from the request
	host := c.Hostname()
	
	proxyURL := fmt.Sprintf("%s://%s/api/upload/proxy?filename=%s", scheme, host, req.Filename)

	// For large files, we still want to indicate it's a large file
	// but the upload will go through our proxy
	isLargeFile := req.FileSize > h.minioService.GetLargeFileThreshold()

	response := fiber.Map{
		"success":             true,
		"isDuplicate":         false,
		"uploadUrl":           proxyURL,
		"filename":            req.Filename,
		"fileSize":            req.FileSize,
		"expires":             time.Now().Add(time.Hour).Unix(),
		"isLargeFile":         isLargeFile,
		"uploadMethod":        "backend_proxy",
		"largeFileThreshold":  h.minioService.GetLargeFileThreshold(),
	}

	if isLargeFile {
		response["message"] = fmt.Sprintf("Large file (%.1f MB) will be uploaded through backend proxy to bypass browser restrictions",
			float64(req.FileSize)/(1024*1024))
	}

	return c.JSON(response)
}

// StreamProxyUpload handles streaming uploads for very large files
func (h *Handlers) StreamProxyUpload(c *fiber.Ctx) error {
	filename := c.Query("filename")
	if filename == "" {
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": "Filename is required",
		})
	}

	// Set up a pipe for streaming
	pr, pw := io.Pipe()
	
	// Start uploading to MinIO in a goroutine
	errChan := make(chan error, 1)
	infoChan := make(chan minio.UploadInfo, 1)
	
	go func() {
		defer pw.Close()
		
		ctx := context.Background()
		info, err := h.minioService.StreamUploadFile(
			ctx,
			filename,
			pr,
			c.Get("Content-Type", "application/octet-stream"),
		)
		
		if err != nil {
			errChan <- err
			return
		}
		infoChan <- info
	}()

	// Copy request body to the pipe
	go func() {
		defer pw.Close()
		_, err := io.Copy(pw, c.Context().RequestBodyStream())
		if err != nil {
			errChan <- err
		}
	}()

	// Wait for upload to complete or error
	select {
	case err := <-errChan:
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("Upload failed: %v", err),
		})
	case info := <-infoChan:
		// Process the uploaded file
		go func() {
			ctx := context.Background()
			h.minioService.ProcessUploadedFile(ctx, filename)
		}()

		return c.JSON(fiber.Map{
			"success":  true,
			"filename": filename,
			"size":     info.Size,
			"etag":     info.ETag,
			"message":  "File streamed successfully via proxy",
		})
	case <-time.After(30 * time.Minute): // 30 minute timeout for large files
		return c.Status(504).JSON(fiber.Map{
			"error":   true,
			"message": "Upload timeout",
		})
	}
}