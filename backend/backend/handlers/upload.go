package handlers

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Upload handles single file upload to MinIO
func (h *Handlers) Upload(c *fiber.Ctx) error {
	logger := slog.With(
		slog.String("handler", "Upload"),
		slog.String("request_id", c.GetRespHeader("X-Request-ID")),
		slog.String("ip", c.IP()),
	)
	
	logger.Info("Upload request received")
	
	// Parse the multipart form
	file, err := c.FormFile("file")
	if err != nil {
		logger.Error("Failed to get file from form", slog.String("error", err.Error()))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No file provided",
		})
	}
	
	logger.Info("File received",
		slog.String("filename", file.Filename),
		slog.Int64("size", file.Size),
		slog.String("content_type", file.Header.Get("Content-Type")),
	)
	
	// Validate file type (only WAV files)
	if !isValidAudioFile(file.Filename, file.Header.Get("Content-Type")) {
		logger.Warn("Invalid file type attempted",
			slog.String("filename", file.Filename),
			slog.String("content_type", file.Header.Get("Content-Type")),
		)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Only WAV files are allowed",
		})
	}
	
	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		logger.Error("Failed to open uploaded file", slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to process file",
		})
	}
	defer src.Close()
	
	// Calculate file hash for duplicate detection
	fileHash, err := h.hashCache.CalculateHash(src)
	if err != nil {
		logger.Error("Failed to calculate file hash", slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to calculate file hash",
		})
	}
	
	// Check for duplicate BEFORE uploading
	if exists, existingFile := h.hashCache.CheckDuplicate(fileHash); exists {
		logger.Warn("Duplicate file rejected",
			slog.String("hash", fileHash[:8]+"..."),
			slog.String("existing_file", existingFile))
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Duplicate file detected",
			"existing_file": existingFile,
			"hash": fileHash,
		})
	}
	
	// Reset file reader after hash calculation
	src.Seek(0, 0)
	
	// Generate unique filename with timestamp
	ext := filepath.Ext(file.Filename)
	baseName := strings.TrimSuffix(filepath.Base(file.Filename), ext)
	objectName := fmt.Sprintf("%s_%d%s", baseName, time.Now().Unix(), ext)
	
	logger.Info("Uploading to MinIO",
		slog.String("bucket", "sermons"),
		slog.String("object_name", objectName),
		slog.Int64("size", file.Size),
	)
	
	// Upload to MinIO with hash metadata
	uploadInfo, err := h.minioService.PutFileWithHash(
		c.Context(),
		"sermons",
		objectName,
		src,
		file.Size,
		file.Header.Get("Content-Type"),
		fileHash,
	)
	
	if err != nil {
		logger.Error("MinIO upload failed",
			slog.String("error", err.Error()),
			slog.String("bucket", "sermons"),
			slog.String("object_name", objectName),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Upload failed: %v", err),
		})
	}
	
	logger.Info("Upload successful",
		slog.String("bucket", uploadInfo.Bucket),
		slog.String("key", uploadInfo.Key),
		slog.Int64("size", uploadInfo.Size),
		slog.String("etag", uploadInfo.ETag),
	)
	
	// Register hash in cache for future duplicate detection
	h.hashCache.AddHash(fileHash, uploadInfo.Key)
	
	// Send success response
	return c.JSON(fiber.Map{
		"success": true,
		"file": fiber.Map{
			"name": uploadInfo.Key,
			"size": uploadInfo.Size,
			"etag": uploadInfo.ETag,
		},
	})
}

// UploadBatch handles multiple file uploads
func (h *Handlers) UploadBatch(c *fiber.Ctx) error {
	logger := slog.With(
		slog.String("handler", "UploadBatch"),
		slog.String("request_id", c.GetRespHeader("X-Request-ID")),
		slog.String("ip", c.IP()),
	)
	
	logger.Info("Batch upload request received")
	
	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		logger.Error("Failed to parse multipart form", slog.String("error", err.Error()))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid form data",
		})
	}
	
	// Get files from form
	files := form.File["files"]
	if len(files) == 0 {
		logger.Warn("No files provided in batch upload")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No files provided",
		})
	}
	
	logger.Info("Processing batch upload", slog.Int("file_count", len(files)))
	
	// Process each file
	results := []fiber.Map{}
	successCount := 0
	failCount := 0
	
	for _, file := range files {
		fileLogger := logger.With(
			slog.String("filename", file.Filename),
			slog.Int64("size", file.Size),
		)
		
		fileLogger.Info("Processing file")
		
		// Validate file type
		if !isValidAudioFile(file.Filename, file.Header.Get("Content-Type")) {
			fileLogger.Warn("Skipping invalid file type")
			results = append(results, fiber.Map{
				"filename": file.Filename,
				"success":  false,
				"error":    "Invalid file type",
			})
			failCount++
			continue
		}
		
		// Open file
		src, err := file.Open()
		if err != nil {
			fileLogger.Error("Failed to open file", slog.String("error", err.Error()))
			results = append(results, fiber.Map{
				"filename": file.Filename,
				"success":  false,
				"error":    "Failed to open file",
			})
			failCount++
			continue
		}
		defer src.Close()
		
		// Calculate file hash for duplicate detection
		fileHash, err := h.hashCache.CalculateHash(src)
		if err != nil {
			fileLogger.Error("Failed to calculate file hash", slog.String("error", err.Error()))
			results = append(results, fiber.Map{
				"filename": file.Filename,
				"success":  false,
				"error":    "Failed to calculate hash",
			})
			failCount++
			continue
		}
		
		// Check for duplicate
		if exists, existingFile := h.hashCache.CheckDuplicate(fileHash); exists {
			fileLogger.Warn("Duplicate file skipped",
				slog.String("hash", fileHash[:8]+"..."),
				slog.String("existing_file", existingFile))
			results = append(results, fiber.Map{
				"filename": file.Filename,
				"success":  false,
				"error":    "Duplicate file detected",
				"existing_file": existingFile,
			})
			failCount++
			continue
		}
		
		// Reset file reader after hash calculation
		src.Seek(0, 0)
		
		// Generate unique filename
		ext := filepath.Ext(file.Filename)
		baseName := strings.TrimSuffix(filepath.Base(file.Filename), ext)
		objectName := fmt.Sprintf("%s_%d%s", baseName, time.Now().UnixNano(), ext)
		
		// Upload to MinIO with hash metadata
		uploadInfo, err := h.minioService.PutFileWithHash(
			c.Context(),
			"sermons",
			objectName,
			src,
			file.Size,
			file.Header.Get("Content-Type"),
			fileHash,
		)
		
		if err != nil {
			fileLogger.Error("Upload failed", slog.String("error", err.Error()))
			results = append(results, fiber.Map{
				"filename": file.Filename,
				"success":  false,
				"error":    err.Error(),
			})
			failCount++
			continue
		}
		
		fileLogger.Info("File uploaded successfully",
			slog.String("object_name", objectName),
			slog.String("etag", uploadInfo.ETag),
		)
		
		// Register hash in cache
		h.hashCache.AddHash(fileHash, uploadInfo.Key)
		
		results = append(results, fiber.Map{
			"filename": file.Filename,
			"success":  true,
			"uploaded": fiber.Map{
				"name": uploadInfo.Key,
				"size": uploadInfo.Size,
				"etag": uploadInfo.ETag,
			},
		})
		successCount++
	}
	
	logger.Info("Batch upload completed",
		slog.Int("total", len(files)),
		slog.Int("success", successCount),
		slog.Int("failed", failCount),
	)
	
	return c.JSON(fiber.Map{
		"success": successCount > 0,
		"summary": fiber.Map{
			"total":   len(files),
			"success": successCount,
			"failed":  failCount,
		},
		"results": results,
	})
}

// isValidAudioFile checks if the file is a valid audio file (WAV)
func isValidAudioFile(filename, contentType string) bool {
	// Check file extension
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".wav" {
		return false
	}
	
	// Check content type if provided and specific
	if contentType != "" && contentType != "application/octet-stream" {
		ct := strings.ToLower(contentType)
		// Only reject if it's a specific non-WAV type
		if !strings.Contains(ct, "wav") && !strings.Contains(ct, "wave") && !strings.Contains(ct, "audio") {
			return false
		}
	}
	
	// If extension is .wav and content-type is generic or audio-related, accept it
	return true
}