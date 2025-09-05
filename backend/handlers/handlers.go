package handlers

import (
	"fmt"
	"log"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"sermon-uploader/config"
	"sermon-uploader/services"
)

type Handlers struct {
	fileService    *services.FileService
	minioService   *services.MinIOService
	discordService *services.DiscordService
	wsHub          *services.WebSocketHub
	config         *config.Config
}

type StatusResponse struct {
	MinIOConnected bool   `json:"minio_connected"`
	BucketExists   bool   `json:"bucket_exists"`
	FileCount      int    `json:"file_count"`
	Endpoint       string `json:"endpoint"`
	BucketName     string `json:"bucket_name"`
}

func New(fileService *services.FileService, minioService *services.MinIOService, discordService *services.DiscordService, wsHub *services.WebSocketHub, cfg *config.Config) *Handlers {
	return &Handlers{
		fileService:    fileService,
		minioService:   minioService,
		discordService: discordService,
		wsHub:          wsHub,
		config:         cfg,
	}
}

func (h *Handlers) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":    "healthy",
		"timestamp": fiber.Map{"now": "ok"},
		"service":   "sermon-uploader-go",
	})
}

func (h *Handlers) GetStatus(c *fiber.Ctx) error {
	// Test MinIO connection
	minioConnected := h.minioService.TestConnection() == nil

	// Check if bucket exists and get file count
	var bucketExists bool
	var fileCount int

	if minioConnected {
		if err := h.minioService.EnsureBucketExists(); err == nil {
			bucketExists = true
			if count, err := h.minioService.GetFileCount(); err == nil {
				fileCount = count
			}
		}
	}

	return c.JSON(StatusResponse{
		MinIOConnected: minioConnected,
		BucketExists:   bucketExists,
		FileCount:      fileCount,
		Endpoint:       h.config.MinIOEndpoint,
		BucketName:     h.config.MinioBucket,
	})
}

func (h *Handlers) TestDiscord(c *fiber.Ctx) error {
	// Send a test notification
	err := h.discordService.SendNotification(
		"🧪 Test Notification",
		"This is a test message from the Sermon Uploader backend to verify Discord webhook is working properly.",
		0x00ff00, // Green color
		[]services.DiscordField{
			{Name: "Status", Value: "Testing", Inline: true},
			{Name: "Backend", Value: "Go + Fiber", Inline: true},
			{Name: "Timestamp", Value: "Now", Inline: true},
		},
	)

	if err != nil {
		log.Printf("Discord test failed: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to send Discord notification",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Discord webhook test successful! Check your Discord channel for the test message.",
	})
}

func (h *Handlers) TestMinIO(c *fiber.Ctx) error {
	// Test connection
	if err := h.minioService.TestConnection(); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success":  false,
			"message":  "MinIO connection failed",
			"error":    err.Error(),
			"endpoint": h.config.MinIOEndpoint,
			"bucket":   h.config.MinioBucket,
		})
	}

	// Test bucket creation/access
	if err := h.minioService.EnsureBucketExists(); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success":  false,
			"message":  "Failed to access/create bucket",
			"error":    err.Error(),
			"endpoint": h.config.MinIOEndpoint,
			"bucket":   h.config.MinioBucket,
		})
	}

	// Get file count
	fileCount, err := h.minioService.GetFileCount()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to count files in bucket",
			"error":   err.Error(),
		})
	}

	// Get existing hashes (for duplicate detection test)
	hashes, err := h.minioService.GetExistingHashes()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get existing file hashes",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success":           true,
		"message":           "MinIO connection and bucket access successful!",
		"endpoint":          h.config.MinIOEndpoint,
		"bucket":            h.config.MinioBucket,
		"files_in_bucket":   fileCount,
		"unique_hashes":     len(hashes),
		"connection_secure": h.config.MinIOSecure,
	})
}

func (h *Handlers) UploadFiles(c *fiber.Ctx) error {
	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse multipart form",
			"error":   err.Error(),
		})
	}

	files := form.File["files"]
	if len(files) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "No files provided",
		})
	}

	// Filter for WAV files only
	var wavFiles []*multipart.FileHeader
	for _, file := range files {
		if strings.HasSuffix(strings.ToLower(file.Filename), ".wav") {
			wavFiles = append(wavFiles, file)
		}
	}

	if len(wavFiles) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "No WAV files found in upload",
		})
	}

	// Process files
	summary, err := h.fileService.ProcessFiles(wavFiles)
	if err != nil {
		h.wsHub.BroadcastError(err.Error())
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "File processing failed",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success":     summary.Failed == 0,
		"message":     "Upload processing complete",
		"total_files": summary.Total,
		"successful":  summary.Successful,
		"duplicates":  summary.Duplicates,
		"failed":      summary.Failed,
		"results":     summary.Results,
	})
}

func (h *Handlers) ListFiles(c *fiber.Ctx) error {
	files, err := h.minioService.ListFiles()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to list files",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"files":   files,
		"count":   len(files),
	})
}

func (h *Handlers) GetFileInfo(c *fiber.Ctx) error {
	filename := c.Params("filename")
	if filename == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Filename parameter is required",
		})
	}

	// This would get detailed file info including metadata
	// Implementation depends on specific needs
	return c.JSON(fiber.Map{
		"success":  true,
		"filename": filename,
		"message":  "File info endpoint - implementation needed",
	})
}

// ClearBucket removes all files from the bucket - DANGEROUS OPERATION
func (h *Handlers) ClearBucket(c *fiber.Ctx) error {
	// Optional: Add authentication/authorization check
	// if !isAuthorized(c) { return c.Status(401).JSON(...) }

	// Optional: Require confirmation parameter
	confirmParam := c.Query("confirm")
	if confirmParam != "yes-delete-everything" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "This operation requires confirmation. Add ?confirm=yes-delete-everything to proceed.",
			"warning": "This will permanently delete ALL files in the bucket!",
		})
	}

	// Get current file count for logging
	fileCount, _ := h.minioService.GetFileCount()

	// Perform the deletion
	result, err := h.minioService.ClearBucket()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to clear bucket",
			"error":   err.Error(),
		})
	}

	// Send Discord notification about the clearing
	if h.discordService != nil {
		h.discordService.SendNotification(
			"🗑️ Bucket Cleared",
			fmt.Sprintf("All files have been removed from the bucket.\n\n**Files deleted:** %d\n**Failed deletions:** %d",
				result.DeletedCount, result.FailedCount),
			0xff6b6b, // Red color
			[]services.DiscordField{
				{Name: "Previous File Count", Value: fmt.Sprintf("%d", fileCount), Inline: true},
				{Name: "Successfully Deleted", Value: fmt.Sprintf("%d", result.DeletedCount), Inline: true},
				{Name: "Failed Deletions", Value: fmt.Sprintf("%d", result.FailedCount), Inline: true},
			},
		)
	}

	response := fiber.Map{
		"success":       true,
		"message":       "Bucket cleared successfully",
		"deleted_count": result.DeletedCount,
		"failed_count":  result.FailedCount,
	}

	if len(result.Errors) > 0 {
		response["errors"] = result.Errors
	}

	return c.JSON(response)
}

// GetDashboard provides a unified endpoint with system status + file list
func (h *Handlers) GetDashboard(c *fiber.Ctx) error {
	includeMetadata := c.Query("metadata", "false") == "true"
	limit := c.QueryInt("limit", 10) // Default to 10 recent files

	// Get system status
	minioConnected := h.minioService.TestConnection() == nil
	var bucketExists bool
	var fileCount int

	if minioConnected {
		if err := h.minioService.EnsureBucketExists(); err == nil {
			bucketExists = true
			if count, err := h.minioService.GetFileCount(); err == nil {
				fileCount = count
			}
		}
	}

	// Get recent files
	var files []interface{}
	var totalSize int64
	var lastUpload string

	if bucketExists {
		allFiles, err := h.minioService.ListFiles()
		if err == nil {
			// Calculate total size and find most recent upload
			for i, fileData := range allFiles {
				if size, ok := fileData["size"].(int64); ok {
					totalSize += size
				}
				if i == 0 { // First file is most recent due to sorting
					if lastMod, ok := fileData["last_modified"].(string); ok {
						lastUpload = lastMod
					}
				}

				// Add to response (limit to requested count)
				if i < limit {
					files = append(files, fileData)
				}
			}
		}
	}

	// Build response
	dashboard := map[string]interface{}{
		"status": map[string]interface{}{
			"minio_connected": minioConnected,
			"bucket_exists":   bucketExists,
			"file_count":      fileCount,
			"endpoint":        h.config.MinIOEndpoint,
			"bucket_name":     h.config.MinioBucket,
		},
		"files": files,
		"summary": map[string]interface{}{
			"total_files":   fileCount,
			"total_size_mb": float64(totalSize) / (1024 * 1024),
			"last_upload":   lastUpload,
			"files_shown":   len(files),
		},
		"meta": map[string]interface{}{
			"generated_at":      time.Now().Format(time.RFC3339),
			"metadata_included": includeMetadata,
			"file_limit":        limit,
		},
	}

	return c.JSON(dashboard)
}

// MigrateMinIO handles migration from external MinIO to embedded MinIO
func (h *Handlers) MigrateMinIO(c *fiber.Ctx) error {
	// Get migration parameters
	sourceEndpoint := c.FormValue("source_endpoint")
	sourceAccessKey := c.FormValue("source_access_key")
	sourceSecretKey := c.FormValue("source_secret_key")
	// sourceBucket is implicitly the same as destination bucket

	if sourceEndpoint == "" || sourceAccessKey == "" || sourceSecretKey == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Missing required migration parameters",
		})
	}

	log.Printf("Starting MinIO migration from %s", sourceEndpoint)

	// Create temporary source MinIO service
	sourceMinio, err := h.minioService.CreateTempConnection(sourceEndpoint, sourceAccessKey, sourceSecretKey)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to connect to source MinIO",
			"error":   err.Error(),
		})
	}

	// List all files in source bucket
	files, err := sourceMinio.ListFiles()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to list files from source MinIO",
			"error":   err.Error(),
		})
	}

	log.Printf("Found %d files to migrate", len(files))

	// Ensure destination bucket exists and migrate policies
	if err := h.minioService.MigratePolicies(sourceMinio); err != nil {
		log.Printf("Warning: Policy migration failed: %v", err)
		// Continue with file migration even if policy migration fails
		if err := h.minioService.EnsureBucketExists(); err != nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"message": "Failed to create destination bucket",
				"error":   err.Error(),
			})
		}
	}

	// Migrate each file
	migratedCount := 0
	errors := []string{}

	for _, fileData := range files {
		fileName, ok := fileData["name"].(string)
		if !ok {
			continue
		}

		// Download from source
		data, err := sourceMinio.DownloadFileData(fileName)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to download %s: %v", fileName, err))
			continue
		}

		// Upload to destination
		_, err = h.minioService.UploadFile(data, fileName)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to upload %s: %v", fileName, err))
			continue
		}

		migratedCount++
		log.Printf("Migrated file %d/%d: %s", migratedCount, len(files), fileName)
	}

	log.Printf("Migration completed: %d files migrated, %d errors", migratedCount, len(errors))

	response := fiber.Map{
		"success":        true,
		"message":        fmt.Sprintf("Migration completed: %d files migrated", migratedCount),
		"migrated_count": migratedCount,
		"total_files":    len(files),
	}

	if len(errors) > 0 {
		response["errors"] = errors
		response["error_count"] = len(errors)
	}

	return c.JSON(response)
}

// CreateTUSUpload creates a new TUS upload session
func (h *Handlers) CreateTUSUpload(c *fiber.Ctx) error {
	// Parse upload metadata
	uploadLength := c.Get("Upload-Length")
	uploadMetadata := c.Get("Upload-Metadata")

	if uploadLength == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Upload-Length header is required",
		})
	}

	size, err := strconv.ParseInt(uploadLength, 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Invalid Upload-Length header",
			"error":   err.Error(),
		})
	}

	// Parse metadata
	tusService := h.fileService.GetTUSService()
	metadata, err := tusService.ParseMetadata(uploadMetadata)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Invalid Upload-Metadata header",
			"error":   err.Error(),
		})
	}

	// Extract filename from metadata
	filename := metadata["filename"]
	if filename == "" {
		filename = "unknown.wav"
	}

	// Create TUS upload
	response, err := h.fileService.ProcessFileWithTUS(filename, size, metadata)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create TUS upload",
			"error":   err.Error(),
		})
	}

	// Set TUS response headers
	c.Set("Location", response.Location)
	c.Set("Upload-Offset", "0")
	c.Set("Tus-Resumable", "1.0.0")

	return c.Status(201).JSON(fiber.Map{
		"success":   true,
		"upload_id": response.ID,
		"location":  response.Location,
	})
}

// GetTUSUploadInfo returns information about a TUS upload
func (h *Handlers) GetTUSUploadInfo(c *fiber.Ctx) error {
	uploadID := c.Params("id")
	if uploadID == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Upload ID is required",
		})
	}

	tusService := h.fileService.GetTUSService()
	info, err := tusService.GetUpload(uploadID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"message": "Upload not found",
			"error":   err.Error(),
		})
	}

	// Set TUS response headers
	c.Set("Upload-Offset", fmt.Sprintf("%d", info.Offset))
	c.Set("Upload-Length", fmt.Sprintf("%d", info.Size))
	c.Set("Tus-Resumable", "1.0.0")

	if info.Metadata != nil && len(info.Metadata) > 0 {
		c.Set("Upload-Metadata", tusService.FormatMetadata(info.Metadata))
	}

	return c.JSON(fiber.Map{
		"success": true,
		"upload":  info,
	})
}

// UploadTUSChunk uploads a chunk to a TUS upload session
func (h *Handlers) UploadTUSChunk(c *fiber.Ctx) error {
	uploadID := c.Params("id")
	if uploadID == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Upload ID is required",
		})
	}

	// Parse offset from header
	uploadOffset := c.Get("Upload-Offset")
	if uploadOffset == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Upload-Offset header is required",
		})
	}

	offset, err := strconv.ParseInt(uploadOffset, 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Invalid Upload-Offset header",
			"error":   err.Error(),
		})
	}

	// Read chunk data from request body
	body := c.Body()
	if len(body) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "No data in request body",
		})
	}

	// Validate offset matches server state
	tusService := h.fileService.GetTUSService()
	if err := tusService.ValidateUploadOffset(uploadID, offset); err != nil {
		return c.Status(409).JSON(fiber.Map{
			"success": false,
			"message": "Offset conflict",
			"error":   err.Error(),
		})
	}

	// Process the chunk
	info, err := h.fileService.ProcessTUSChunk(uploadID, offset, body)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to process chunk",
			"error":   err.Error(),
		})
	}

	// Set response headers
	c.Set("Upload-Offset", fmt.Sprintf("%d", info.Offset))
	c.Set("Tus-Resumable", "1.0.0")

	// Check if upload is complete
	if info.IsComplete {
		// Broadcast completion via WebSocket
		h.wsHub.BroadcastFileProgress(info.Filename, "tus_complete", "TUS upload completed", info.Progress)

		return c.Status(200).JSON(fiber.Map{
			"success":  true,
			"complete": true,
			"upload":   info,
			"message":  "Upload completed successfully",
		})
	}

	// Broadcast progress via WebSocket
	h.wsHub.BroadcastFileProgress(info.Filename, "tus_progress",
		fmt.Sprintf("Progress: %.1f%%", info.Progress), info.Progress)

	return c.Status(204).Send(nil)
}

// CompleteTUSUpload completes a TUS upload and transfers to MinIO
func (h *Handlers) CompleteTUSUpload(c *fiber.Ctx) error {
	uploadID := c.Params("id")
	if uploadID == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Upload ID is required",
		})
	}

	// Get expected hash from request
	expectedHash := c.Query("hash")
	if expectedHash == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Expected hash is required",
		})
	}

	// Complete the TUS upload
	result, err := h.fileService.CompleteTUSUpload(uploadID, expectedHash)
	if err != nil {
		h.wsHub.BroadcastError(fmt.Sprintf("TUS completion failed: %v", err))
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to complete TUS upload",
			"error":   err.Error(),
		})
	}

	if result.Status == "error" {
		h.wsHub.BroadcastFileProgress(result.Filename, "error", result.Message, 100)
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": result.Message,
			"result":  result,
		})
	}

	// Broadcast success
	h.wsHub.BroadcastFileProgress(result.Filename, "success", "Upload completed and verified", 100)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Upload completed successfully",
		"result":  result,
	})
}

// UploadStreamingFiles handles streaming file uploads with concurrent processing
func (h *Handlers) UploadStreamingFiles(c *fiber.Ctx) error {
	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse multipart form",
			"error":   err.Error(),
		})
	}

	files := form.File["files"]
	if len(files) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "No files provided",
		})
	}

	// Filter for WAV files only
	var wavFiles []*multipart.FileHeader
	for _, file := range files {
		if strings.HasSuffix(strings.ToLower(file.Filename), ".wav") {
			wavFiles = append(wavFiles, file)
		}
	}

	if len(wavFiles) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "No WAV files found in upload",
		})
	}

	// Use concurrent processing for better performance on Pi
	summary, err := h.fileService.ProcessConcurrentFiles(wavFiles)
	if err != nil {
		h.wsHub.BroadcastError(err.Error())
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Streaming file processing failed",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success":     summary.Failed == 0,
		"message":     "Streaming upload processing complete",
		"total_files": summary.Total,
		"successful":  summary.Successful,
		"duplicates":  summary.Duplicates,
		"failed":      summary.Failed,
		"results":     summary.Results,
	})
}

// GetStreamingStats returns streaming upload statistics
func (h *Handlers) GetStreamingStats(c *fiber.Ctx) error {
	streamingService := h.fileService.GetStreamingService()
	tusService := h.fileService.GetTUSService()

	stats := map[string]interface{}{
		"streaming":                 streamingService.GetMemoryUsage(),
		"tus":                       tusService.GetUploadStats(),
		"active_streaming_sessions": streamingService.GetActiveSessionsCount(),
		"active_tus_uploads":        tusService.GetActiveUploadsCount(),
	}

	return c.JSON(fiber.Map{
		"success": true,
		"stats":   stats,
	})
}

// GetCompressionStats returns compression and quality statistics
func (h *Handlers) GetCompressionStats(c *fiber.Ctx) error {
	stats, err := h.minioService.GetZeroCompressionStats()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get compression stats",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"stats":   stats,
	})
}

// DeleteTUSUpload deletes a TUS upload session
func (h *Handlers) DeleteTUSUpload(c *fiber.Ctx) error {
	uploadID := c.Params("id")
	if uploadID == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Upload ID is required",
		})
	}

	tusService := h.fileService.GetTUSService()
	if err := tusService.DeleteUpload(uploadID); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to delete upload",
			"error":   err.Error(),
		})
	}

	c.Set("Tus-Resumable", "1.0.0")
	return c.Status(204).Send(nil)
}

// GetTUSConfiguration returns TUS protocol configuration
func (h *Handlers) GetTUSConfiguration(c *fiber.Ctx) error {
	tusService := h.fileService.GetTUSService()
	config := tusService.GetTUSConfiguration()

	// Set TUS headers
	c.Set("Tus-Resumable", "1.0.0")
	c.Set("Tus-Version", "1.0.0")
	c.Set("Tus-Max-Size", fmt.Sprintf("%d", config["max_size"]))
	c.Set("Tus-Extension", "creation,termination,checksum")
	c.Set("Tus-Checksum-Algorithm", "sha256")

	return c.JSON(config)
}

// VerifyFileIntegrity verifies the integrity of an uploaded file
func (h *Handlers) VerifyFileIntegrity(c *fiber.Ctx) error {
	filename := c.Params("filename")
	expectedHash := c.Query("hash")

	if filename == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Filename is required",
		})
	}

	if expectedHash == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Expected hash is required",
		})
	}

	result, err := h.minioService.VerifyUploadIntegrity(filename, expectedHash)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to verify file integrity",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success":         true,
		"integrity_check": result,
	})
}

// CleanupExpiredUploads cleans up expired TUS uploads
func (h *Handlers) CleanupExpiredUploads(c *fiber.Ctx) error {
	// Get max age from query parameter (default 24 hours)
	maxAgeHours := c.QueryInt("max_age_hours", 24)
	maxAge := time.Duration(maxAgeHours) * time.Hour

	tusService := h.fileService.GetTUSService()
	streamingService := h.fileService.GetStreamingService()

	// Cleanup expired uploads
	tusCount, err := tusService.CleanupExpired(maxAge)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to cleanup TUS uploads",
			"error":   err.Error(),
		})
	}

	streamingCount := streamingService.CleanupExpiredSessions(maxAge)

	return c.JSON(fiber.Map{
		"success":           true,
		"message":           fmt.Sprintf("Cleanup completed: %d TUS uploads, %d streaming sessions", tusCount, streamingCount),
		"tus_cleaned":       tusCount,
		"streaming_cleaned": streamingCount,
	})
}
