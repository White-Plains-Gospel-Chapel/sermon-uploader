package handlers

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/gofiber/fiber/v2"

	"sermon-uploader/services"
)

// Helper function to log with Eastern Time
func logWithEasternTime(format string, args ...interface{}) {
	easternTZ, _ := time.LoadLocation("America/New_York")
	easternTime := time.Now().In(easternTZ)
	timestamp := easternTime.Format("2006/01/02 15:04:05 MST")
	log.Printf("[%s] "+format, append([]interface{}{timestamp}, args...)...)
}

// GetPresignedURL generates a presigned URL for direct upload to MinIO (with duplicate check)
func (h *Handlers) GetPresignedURL(c *fiber.Ctx) error {
	type Request struct {
		Filename string `json:"filename"`
		FileSize int64  `json:"fileSize"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request",
		})
	}

	// Check for duplicates first using filename-based detection (O(1) operation)
	isDuplicate, err := h.minioService.CheckDuplicateByFilename(req.Filename)
	if err != nil {
		// Log the duplicate check failure
		if h.productionLogger != nil {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			
			ctx := context.Background()
			failureContext := services.UploadFailureContext{
				Filename:    req.Filename,
				FileSize:    req.FileSize,
				UserIP:      c.IP(),
				Error:       err,
				Operation:   "duplicate_check",
				RequestID:   c.Get("X-Request-ID", "unknown"),
				Timestamp:   time.Now(),
				Component:   "handlers.GetPresignedURL",
				UserAgent:   c.Get("User-Agent"),
				ContentType: c.Get("Content-Type"),
			}
			h.productionLogger.LogUploadFailure(ctx, failureContext)
		}
		
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to check for duplicates",
		})
	}

	if isDuplicate {
		return c.Status(409).JSON(fiber.Map{
			"error":       true,
			"isDuplicate": true,
			"message":     "File already exists",
			"filename":    req.Filename,
		})
	}

	// Generate smart presigned URL based on file size (valid for 1 hour)
	presignedURL, isLargeFile, err := h.minioService.GeneratePresignedUploadURLSmart(req.Filename, req.FileSize, time.Hour)
	if err != nil {
		// Log presigned URL generation failure
		if h.productionLogger != nil {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			
			ctx := context.Background()
			failureContext := services.UploadFailureContext{
				Filename:    req.Filename,
				FileSize:    req.FileSize,
				UserIP:      c.IP(),
				Error:       err,
				Operation:   "generate_presigned_url",
				RequestID:   c.Get("X-Request-ID", "unknown"),
				Timestamp:   time.Now(),
				Component:   "handlers.GetPresignedURL",
				UserAgent:   c.Get("User-Agent"),
				ContentType: c.Get("Content-Type"),
			}
			h.productionLogger.LogUploadFailure(ctx, failureContext)
		}
		
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to generate upload URL",
		})
	}

	// Determine upload method based on whether it's a large file
	uploadMethod := "cloudflare"
	if isLargeFile {
		uploadMethod = "direct_minio"
	}

	response := fiber.Map{
		"success":      true,
		"isDuplicate":  false,
		"uploadUrl":    presignedURL,
		"filename":     req.Filename,
		"fileSize":     req.FileSize,
		"expires":      time.Now().Add(time.Hour).Unix(),
		"isLargeFile":  isLargeFile,
		"uploadMethod": uploadMethod,
	}

	// Add threshold info for debugging
	if isLargeFile {
		threshold := h.minioService.GetLargeFileThreshold()
		response["largeFileThreshold"] = threshold
		response["message"] = fmt.Sprintf("Large file (%.1f MB) will use direct MinIO upload to bypass CloudFlare 100MB limit", 
			float64(req.FileSize)/(1024*1024))
	}

	return c.JSON(response)
}

// ProcessUploadedFile handles post-upload processing (called after direct upload)
func (h *Handlers) ProcessUploadedFile(c *fiber.Ctx) error {
	type Request struct {
		Filename string `json:"filename"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request",
		})
	}

	// Check if file exists in MinIO
	exists, err := h.minioService.FileExists(req.Filename)
	if err != nil || !exists {
		// Log file verification failure
		if h.productionLogger != nil && err != nil {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			
			ctx := context.Background()
			failureContext := services.UploadFailureContext{
				Filename:    req.Filename,
				FileSize:    0, // Unknown at this point
				UserIP:      c.IP(),
				Error:       err,
				Operation:   "file_verification",
				RequestID:   c.Get("X-Request-ID", "unknown"),
				Timestamp:   time.Now(),
				Component:   "handlers.ProcessUploadedFile",
				UserAgent:   c.Get("User-Agent"),
				ContentType: c.Get("Content-Type"),
			}
			h.productionLogger.LogUploadFailure(ctx, failureContext)
		}
		
		return c.Status(404).JSON(fiber.Map{
			"error":   true,
			"message": "File not found in storage",
		})
	}

	// Get basic file info
	fileInfo, err := h.minioService.GetFileInfo(req.Filename)
	if err != nil {
		// Log file info retrieval failure
		if h.productionLogger != nil {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			
			ctx := context.Background()
			failureContext := services.UploadFailureContext{
				Filename:    req.Filename,
				FileSize:    0, // Unknown at this point
				UserIP:      c.IP(),
				Error:       err,
				Operation:   "get_file_info",
				RequestID:   c.Get("X-Request-ID", "unknown"),
				Timestamp:   time.Now(),
				Component:   "handlers.ProcessUploadedFile",
				UserAgent:   c.Get("User-Agent"),
				ContentType: c.Get("Content-Type"),
			}
			h.productionLogger.LogUploadFailure(ctx, failureContext)
		}
		
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to get file info",
		})
	}

	// Create basic metadata immediately for fast response
	basicMetadata := &services.AudioMetadata{
		Filename:   req.Filename,
		FileSize:   fileInfo.Size,
		UploadTime: time.Now(),
		IsValid:    true,
	}

	// Process comprehensive metadata in background (don't block the response)
	go func() {
		startTime := time.Now()
		logWithEasternTime("🔍 Starting background metadata processing for %s", req.Filename)

		metadataService := h.fileService.GetMetadataService()
		fullMetadata, err := metadataService.ExtractMetadataFromMinIO(h.minioService, req.Filename)

		processingDuration := time.Since(startTime)

		if err != nil {
			logWithEasternTime("Background metadata extraction failed for %s after %v: %v", req.Filename, processingDuration, err)
			return
		}

		// Add processing duration to metadata
		fullMetadata.ProcessingDuration = processingDuration

		// Store metadata as object metadata in MinIO
		if err := h.minioService.StoreMetadata(req.Filename, fullMetadata); err != nil {
			logWithEasternTime("Failed to store metadata for %s: %v", req.Filename, err)
		}

		// Send enhanced Discord notification with full metadata including timing
		if h.discordService != nil {
			h.discordService.SendUploadCompleteWithMetadata(fullMetadata)
		}

		logWithEasternTime("✅ Background metadata processing completed for %s in %v", req.Filename, processingDuration)
	}()

	return c.JSON(fiber.Map{
		"success":  true,
		"filename": req.Filename,
		"size":     fileInfo.Size,
		"metadata": basicMetadata,
		"message":  "File uploaded successfully, metadata processing in background",
	})
}

// GetPresignedURLsBatch generates presigned URLs for multiple files at once
func (h *Handlers) GetPresignedURLsBatch(c *fiber.Ctx) error {
	type FileRequest struct {
		Filename string `json:"filename"`
		FileSize int64  `json:"fileSize"`
	}

	type BatchRequest struct {
		Files []FileRequest `json:"files"`
	}

	var req BatchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request format",
		})
	}

	if len(req.Files) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": "No files provided",
		})
	}

	if len(req.Files) > 50 { // Reasonable limit
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": "Too many files. Maximum 50 files per batch.",
		})
	}

	results := make(map[string]interface{})
	successCount := 0
	duplicateCount := 0
	errorCount := 0

	for _, fileReq := range req.Files {
		fileResult := make(map[string]interface{})

		// Check for duplicates first
		isDuplicate, err := h.minioService.CheckDuplicateByFilename(fileReq.Filename)
		if err != nil {
			fileResult["error"] = true
			fileResult["message"] = "Failed to check for duplicates"
			fileResult["isDuplicate"] = false
			errorCount++
		} else if isDuplicate {
			fileResult["error"] = false
			fileResult["isDuplicate"] = true
			fileResult["message"] = "File already exists"
			duplicateCount++
		} else {
			// Generate smart presigned URL based on file size
			presignedURL, isLargeFile, err := h.minioService.GeneratePresignedUploadURLSmart(fileReq.Filename, fileReq.FileSize, time.Hour)
			if err != nil {
				fileResult["error"] = true
				fileResult["message"] = "Failed to generate upload URL"
				fileResult["isDuplicate"] = false
				errorCount++
			} else {
				uploadMethod := "cloudflare"
				if isLargeFile {
					uploadMethod = "direct_minio"
				}
				
				fileResult["error"] = false
				fileResult["isDuplicate"] = false
				fileResult["uploadUrl"] = presignedURL
				fileResult["fileSize"] = fileReq.FileSize
				fileResult["expires"] = time.Now().Add(time.Hour).Unix()
				fileResult["isLargeFile"] = isLargeFile
				fileResult["uploadMethod"] = uploadMethod
				
				if isLargeFile {
					threshold := h.minioService.GetLargeFileThreshold()
					fileResult["largeFileThreshold"] = threshold
					fileResult["message"] = fmt.Sprintf("Large file (%.1f MB) will use direct MinIO upload", 
						float64(fileReq.FileSize)/(1024*1024))
				}
				
				successCount++
			}
		}

		results[fileReq.Filename] = fileResult
	}

	return c.JSON(fiber.Map{
		"success":         errorCount == 0,
		"total_files":     len(req.Files),
		"success_count":   successCount,
		"duplicate_count": duplicateCount,
		"error_count":     errorCount,
		"results":         results,
		"message": fmt.Sprintf("Processed %d files: %d ready for upload, %d duplicates, %d errors",
			len(req.Files), successCount, duplicateCount, errorCount),
	})
}
