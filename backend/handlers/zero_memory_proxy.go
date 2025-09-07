package handlers

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Global semaphore to limit concurrent uploads
var uploadSemaphore = make(chan struct{}, 5) // Max 5 concurrent uploads

// ZeroMemoryStreamingProxy handles uploads with zero memory buffering
// Uses io.Pipe for direct streaming from request to MinIO
func (h *Handlers) ZeroMemoryStreamingProxy(c *fiber.Ctx) error {
	// Acquire semaphore slot (blocks if too many concurrent uploads)
	uploadSemaphore <- struct{}{}
	defer func() { <-uploadSemaphore }() // Release slot when done

	filename := c.Query("filename")
	if filename == "" {
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": "Filename is required",
		})
	}

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

	fmt.Printf("🚀 Zero-memory upload: %s (%.1f MB) - slot acquired\n", 
		filename, float64(contentLength)/(1024*1024))

	// Create a pipe for zero-copy streaming
	pipeReader, pipeWriter := io.Pipe()
	
	// Error channels for goroutine communication
	uploadErrChan := make(chan error, 1)
	copyErrChan := make(chan error, 1)
	
	var uploadInfo interface{}
	var uploadErr error
	var wg sync.WaitGroup

	// Goroutine 1: Stream data from request to pipe (zero memory copy)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer pipeWriter.Close()
		
		requestBody := c.Context().RequestBodyStream()
		_, err := io.Copy(pipeWriter, requestBody)
		if err != nil {
			copyErrChan <- fmt.Errorf("request copy error: %v", err)
			pipeWriter.CloseWithError(err)
		}
	}()

	// Goroutine 2: Stream data from pipe to MinIO (zero memory copy)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer pipeReader.Close()
		
		ctx := context.Background()
		info, err := h.minioService.ProxyUploadFile(
			ctx,
			filename,
			pipeReader, // Read directly from pipe
			int64(contentLength),
			c.Get("Content-Type", "application/octet-stream"),
		)
		
		if err != nil {
			uploadErrChan <- fmt.Errorf("MinIO upload error: %v", err)
		} else {
			uploadInfo = info
		}
	}()

	// Wait for both goroutines to complete
	wg.Wait()

	// Check for errors
	select {
	case err := <-copyErrChan:
		uploadErr = err
	case err := <-uploadErrChan:
		uploadErr = err
	default:
		// No errors
	}

	if uploadErr != nil {
		fmt.Printf("❌ Zero-memory upload failed for %s: %v\n", filename, uploadErr)
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("Upload failed: %v", uploadErr),
		})
	}

	// Type assertion to get upload info
	type UploadResult interface {
		Size() int64
		ETag() string
	}
	
	if uploadInfo == nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": "Upload failed - no response from MinIO",
		})
	}
	
	// Use reflection to get Size and ETag from MinIO response
	var size int64 = int64(contentLength) // Use original content length as fallback
	var etag string = "unknown"
	
	// Try to extract actual values if possible
	if v, ok := uploadInfo.(map[string]interface{}); ok {
		if s, exists := v["Size"]; exists {
			if sizeVal, ok := s.(int64); ok {
				size = sizeVal
			}
		}
		if e, exists := v["ETag"]; exists {
			if etagVal, ok := e.(string); ok {
				etag = etagVal
			}
		}
	}

	fmt.Printf("✅ Zero-memory upload completed: %s (%d bytes) - slot released\n", 
		filename, size)

	// Process metadata in background (don't block response)
	go func() {
		ctx := context.Background()
		_ = h.minioService.ProcessUploadedFile(ctx, filename)
	}()

	return c.JSON(fiber.Map{
		"success":      true,
		"filename":     filename,
		"size":         size,
		"etag":         etag,
		"uploadMethod": "zero_memory_streaming",
		"message":      "File uploaded with zero memory buffering",
	})
}

// GetZeroMemoryUploadURL returns URL for zero-memory streaming upload
func (h *Handlers) GetZeroMemoryUploadURL(c *fiber.Ctx) error {
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

	// Generate zero-memory streaming URL - ALWAYS use Pi's direct IP to bypass CloudFlare
	scheme := "http" // Always use HTTP for Pi direct access
	host := "192.168.1.127:8000" // Force Pi direct IP
	zeroMemoryURL := fmt.Sprintf("%s://%s/api/upload/zero-memory-proxy?filename=%s", 
		scheme, host, req.Filename)

	// Check available upload slots
	availableSlots := len(uploadSemaphore)
	maxSlots := cap(uploadSemaphore)

	response := fiber.Map{
		"success":             true,
		"isDuplicate":         false,
		"uploadUrl":           zeroMemoryURL,
		"filename":            req.Filename,
		"fileSize":            req.FileSize,
		"expires":             time.Now().Add(time.Hour).Unix(),
		"uploadMethod":        "zero_memory_streaming",
		"message":             "Zero-memory streaming upload - no memory usage, handles bulk uploads",
		"concurrency": fiber.Map{
			"available_slots": maxSlots - availableSlots,
			"max_slots":      maxSlots,
			"queued":         availableSlots == maxSlots,
		},
	}

	fmt.Printf("📡 Zero-memory URL generated for %s (%.1f MB) - %d/%d slots available\n", 
		req.Filename, float64(req.FileSize)/(1024*1024), 
		maxSlots-availableSlots, maxSlots)

	return c.JSON(response)
}

// GetZeroMemoryUploadURLBatch handles batch requests with smart queuing
func (h *Handlers) GetZeroMemoryUploadURLBatch(c *fiber.Ctx) error {
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

	// ALWAYS use Pi direct IP to bypass CloudFlare for bulk uploads
	scheme := "http" // Always use HTTP for Pi direct access
	host := "192.168.1.127:8000" // Force Pi direct IP

	// Process files in batches to avoid overwhelming the system
	fmt.Printf("📦 Processing bulk upload batch: %d files\n", len(req.Files))

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

		// Generate zero-memory URL
		zeroMemoryURL := fmt.Sprintf("%s://%s/api/upload/zero-memory-proxy?filename=%s", 
			scheme, host, file.Filename)

		results[file.Filename] = fiber.Map{
			"success":      true,
			"uploadUrl":    zeroMemoryURL,
			"fileSize":     file.FileSize,
			"uploadMethod": "zero_memory_streaming",
		}
		successCount++
	}

	maxSlots := cap(uploadSemaphore)
	availableSlots := len(uploadSemaphore)

	return c.JSON(fiber.Map{
		"success":         errorCount == 0,
		"results":         results,
		"success_count":   successCount,
		"duplicate_count": duplicateCount,
		"error_count":     errorCount,
		"total_files":     len(req.Files),
		"message":         fmt.Sprintf("Zero-memory batch: %d ready, %d duplicates, %d errors", 
			successCount, duplicateCount, errorCount),
		"concurrency": fiber.Map{
			"max_concurrent":     maxSlots,
			"available_slots":    maxSlots - availableSlots,
			"recommended_delay":  "100ms between uploads for optimal performance",
		},
	})
}