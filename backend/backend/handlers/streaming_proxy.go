package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// StreamingUploadProxy handles large file uploads by streaming them through the backend to MinIO
// This eliminates CORS issues while maintaining performance through zero-copy streaming
func (h *Handlers) StreamingUploadProxy(c *fiber.Ctx) error {
	// Get filename from query params
	filename := c.Query("filename")
	if filename == "" {
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": "Filename is required",
		})
	}

	// Get file size from header
	contentLength := c.Request().Header.ContentLength()
	if contentLength <= 0 {
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": "Content-Length header is required",
		})
	}

	// Check for duplicates
	exists, err := h.minioService.FileExists(filename)
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
			"filename":    filename,
		})
	}

	// Log upload attempt
	fmt.Printf("🚀 Streaming upload: %s (%.1f MB) via API proxy\n", 
		filename, float64(contentLength)/(1024*1024))

	// Stream the request body directly to MinIO
	ctx := context.Background()
	reader := c.Context().RequestBodyStream()

	// Upload to MinIO with streaming
	info, err := h.minioService.ProxyUploadFile(
		ctx,
		filename,
		reader,
		int64(contentLength),
		c.Get("Content-Type", "application/octet-stream"),
	)

	if err != nil {
		fmt.Printf("❌ Streaming upload failed for %s: %v\n", filename, err)
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("Upload failed: %v", err),
		})
	}

	fmt.Printf("✅ Streaming upload completed: %s (%d bytes)\n", filename, info.Size)

	// Process the uploaded file (extract metadata, etc.)
	go func() {
		ctx := context.Background()
		_ = h.minioService.ProcessUploadedFile(ctx, filename)
	}()

	return c.JSON(fiber.Map{
		"success":     true,
		"filename":    filename,
		"size":        info.Size,
		"etag":        info.ETag,
		"uploadMethod": "streaming_proxy",
		"message":     "File uploaded successfully via streaming proxy",
	})
}

// GetStreamingUploadURL returns the streaming proxy endpoint URL
func (h *Handlers) GetStreamingUploadURL(c *fiber.Ctx) error {
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

	// Generate streaming proxy URL (through our backend)
	scheme := "https"
	if c.Protocol() == "http" {
		scheme = "http"
	}
	
	// Use the actual domain from the request
	host := c.Hostname()
	
	streamingURL := fmt.Sprintf("%s://%s/api/upload/streaming-proxy?filename=%s", 
		scheme, host, req.Filename)

	response := fiber.Map{
		"success":             true,
		"isDuplicate":         false,
		"uploadUrl":           streamingURL,
		"filename":            req.Filename,
		"fileSize":            req.FileSize,
		"expires":             time.Now().Add(time.Hour).Unix(),
		"uploadMethod":        "streaming_proxy",
		"message":             "Upload via streaming proxy - no CORS issues, no size limits",
	}

	fmt.Printf("📡 Streaming proxy URL generated for %s (%.1f MB)\n", 
		req.Filename, float64(req.FileSize)/(1024*1024))

	return c.JSON(response)
}

// GetStreamingUploadURLBatch returns streaming proxy URLs for multiple files
func (h *Handlers) GetStreamingUploadURLBatch(c *fiber.Ctx) error {
	var req struct {
		Files []struct {
			Filename string `json:"filename"`
			FileSize int64  `json:"fileSize"`
		} `json:"files"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request format",
		})
	}

	results := make(map[string]interface{})
	successCount := 0
	duplicateCount := 0
	errorCount := 0

	// Get URL scheme and host
	scheme := "https"
	if c.Protocol() == "http" {
		scheme = "http"
	}
	host := c.Hostname()

	for _, file := range req.Files {
		// Check for duplicates
		exists, err := h.minioService.FileExists(file.Filename)
		if err != nil {
			results[file.Filename] = fiber.Map{
				"error":   true,
				"message": "Failed to check existence",
			}
			errorCount++
			continue
		}

		if exists {
			results[file.Filename] = fiber.Map{
				"isDuplicate": true,
				"message":     "File already exists",
			}
			duplicateCount++
			continue
		}

		// Generate streaming proxy URL
		streamingURL := fmt.Sprintf("%s://%s/api/upload/streaming-proxy?filename=%s", 
			scheme, host, file.Filename)

		results[file.Filename] = fiber.Map{
			"success":      true,
			"uploadUrl":    streamingURL,
			"fileSize":     file.FileSize,
			"uploadMethod": "streaming_proxy",
		}
		successCount++
	}

	return c.JSON(fiber.Map{
		"success":         errorCount == 0,
		"results":         results,
		"success_count":   successCount,
		"duplicate_count": duplicateCount,
		"error_count":     errorCount,
		"total_files":     len(req.Files),
		"message":         fmt.Sprintf("Streaming proxy URLs: %d ready, %d duplicates, %d errors", 
			successCount, duplicateCount, errorCount),
	})
}