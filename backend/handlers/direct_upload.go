package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// GetDirectMinIOUploadURL returns a presigned URL for direct browser-to-MinIO uploads
// This completely bypasses CloudFlare and the backend proxy
func (h *Handlers) GetDirectMinIOUploadURL(c *fiber.Ctx) error {
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

	// Generate presigned URL for direct MinIO upload
	ctx := context.Background()
	
	// Use public endpoint for globally accessible URLs
	presignedURL, err := h.minioService.GeneratePublicPresignedPutURL(ctx, req.Filename, 24*time.Hour)
	if err != nil {
		// Fallback to internal URL if public fails
		presignedURL, err = h.minioService.GeneratePresignedPutURL(req.Filename, 24*60) // 24 hours in minutes
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":   true,
				"message": fmt.Sprintf("Failed to generate upload URL: %v", err),
			})
		}
	}

	// Return direct MinIO URL
	response := fiber.Map{
		"success":             true,
		"isDuplicate":         false,
		"uploadUrl":           presignedURL,
		"filename":            req.Filename,
		"fileSize":            req.FileSize,
		"expires":             time.Now().Add(24 * time.Hour).Unix(),
		"uploadMethod":        "direct_minio",
		"message":             "Direct MinIO upload - no CloudFlare, no proxy",
	}

	// Log for debugging
	fmt.Printf("📡 Direct MinIO URL generated for %s (%.1f MB)\n", 
		req.Filename, float64(req.FileSize)/(1024*1024))

	return c.JSON(response)
}

// GetDirectMinIOUploadURLBatch returns presigned URLs for multiple files
func (h *Handlers) GetDirectMinIOUploadURLBatch(c *fiber.Ctx) error {
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

	ctx := context.Background()
	results := make(map[string]interface{})
	successCount := 0
	duplicateCount := 0
	errorCount := 0

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

		// Generate presigned URL with public endpoint
		presignedURL, err := h.minioService.GeneratePublicPresignedPutURL(ctx, file.Filename, 24*time.Hour)
		if err != nil {
			// Fallback to internal URL
			presignedURL, err = h.minioService.GeneratePresignedPutURL(file.Filename, 24*60) // 24 hours in minutes
		}
		if err != nil {
			results[file.Filename] = fiber.Map{
				"error":   true,
				"message": fmt.Sprintf("Failed to generate URL: %v", err),
			}
			errorCount++
			continue
		}

		results[file.Filename] = fiber.Map{
			"success":      true,
			"uploadUrl":    presignedURL,
			"fileSize":     file.FileSize,
			"uploadMethod": "direct_minio",
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
		"message":         fmt.Sprintf("Direct MinIO URLs: %d ready, %d duplicates, %d errors", successCount, duplicateCount, errorCount),
	})
}