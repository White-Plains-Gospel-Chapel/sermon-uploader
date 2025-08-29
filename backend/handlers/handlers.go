package handlers

import (
	"fmt"
	"log"
	"mime/multipart"
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
			"success":   false,
			"message":   "MinIO connection failed",
			"error":     err.Error(),
			"endpoint":  h.config.MinIOEndpoint,
			"bucket":    h.config.MinioBucket,
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
			"success":  false,
			"message":  "Failed to count files in bucket",
			"error":    err.Error(),
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
			"total_files":     fileCount,
			"total_size_mb":   float64(totalSize) / (1024 * 1024),
			"last_upload":     lastUpload,
			"files_shown":     len(files),
		},
		"meta": map[string]interface{}{
			"generated_at":     time.Now().Format(time.RFC3339),
			"metadata_included": includeMetadata,
			"file_limit":       limit,
		},
	}
	
	return c.JSON(dashboard)
}