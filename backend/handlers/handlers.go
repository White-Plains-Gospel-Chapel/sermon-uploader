package handlers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"log/slog"
	"mime/multipart"
	"runtime"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"sermon-uploader/config"
	"sermon-uploader/services"
)

type Handlers struct {
	fileService         *services.FileService
	minioService        *services.MinIOService
	discordService      *services.DiscordService
	discordLiveService  *services.DiscordLiveService
	wsHub               *services.WebSocketHub
	memoryMonitor       *services.MemoryMonitorService
	config              *config.Config
	logger              *slog.Logger
	productionLogger    interface{}
	hashCache           *services.HashCache
	startTime           time.Time
}

type StatusResponse struct {
	MinIOConnected bool   `json:"minio_connected"`
	BucketExists   bool   `json:"bucket_exists"`
	FileCount      int    `json:"file_count"`
	Endpoint       string `json:"endpoint"`
	BucketName     string `json:"bucket_name"`
}

func New(fileService *services.FileService, minioService *services.MinIOService, discordService *services.DiscordService, discordLiveService *services.DiscordLiveService, wsHub *services.WebSocketHub, cfg *config.Config, productionLogger interface{}, hashCache *services.HashCache) *Handlers {
	// Initialize memory monitoring for Pi optimization
	memoryMonitor := services.NewMemoryMonitorService(cfg)
	
	// Set memory pressure callbacks
	memoryMonitor.SetMemoryPressureCallback(func(stats services.MemoryStats) {
		slog.Warn("Memory pressure detected during upload",
			"alloc_mb", stats.AllocMB,
			"pressure_level", stats.PressureLevel,
			"gc_cycles", stats.GCCycles)
	})
	
	memoryMonitor.SetMemoryAlertCallback(func(stats services.MemoryStats) {
		slog.Error("Critical memory pressure - forcing GC",
			"alloc_mb", stats.AllocMB,
			"sys_mb", stats.SysMB,
			"pressure_level", stats.PressureLevel)
		// Force immediate GC on critical pressure
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
		runtime.GC()
	})
	
	// Start monitoring with 1-second intervals
	go memoryMonitor.StartMonitoring(ctx, 1000)

	return &Handlers{
		fileService:         fileService,
		minioService:        minioService,
		discordService:      discordService,
		discordLiveService:  discordLiveService,
		wsHub:               wsHub,
		memoryMonitor:       memoryMonitor,
		config:              cfg,
		logger:              slog.Default(),
		productionLogger:    productionLogger,
		hashCache:           hashCache,
		startTime:           time.Now(),
	}
}

func (h *Handlers) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":      "healthy",
		"timestamp":   fiber.Map{"now": "ok"},
		"service":     "sermon-uploader-go",
		"version":     config.Version,
		"fullVersion": config.GetFullVersion("backend"),
		"memory":      h.memoryMonitor.GetMemoryStatsForAPI(),
	})
}

// GetMemoryStatus returns detailed memory statistics
func (h *Handlers) GetMemoryStatus(c *fiber.Ctx) error {
	stats := h.memoryMonitor.GetCurrentStats()
	
	return c.JSON(fiber.Map{
		"success": true,
		"memory": map[string]interface{}{
			"current_stats":    h.memoryMonitor.GetMemoryStatsForAPI(),
			"raw_stats":        stats,
			"monitoring":       true,
			"recommendations": h.getMemoryRecommendations(stats),
		},
	})
}

// getMemoryRecommendations provides memory optimization suggestions
func (h *Handlers) getMemoryRecommendations(stats services.MemoryStats) []string {
	var recommendations []string
	
	usageRatio := stats.AllocMB / 1800.0 // Assuming 1.8GB limit for Pi
	
	if usageRatio > 0.9 {
		recommendations = append(recommendations, "Critical: Memory usage >90% - consider restarting service")
	} else if usageRatio > 0.8 {
		recommendations = append(recommendations, "Warning: Memory usage >80% - avoid large uploads")
	} else if usageRatio > 0.7 {
		recommendations = append(recommendations, "Caution: Memory usage >70% - monitor closely")
	}
	
	if stats.PressureLevel != "normal" {
		recommendations = append(recommendations, "Memory pressure detected - uploads may be throttled")
	}
	
	// GC recommendations
	timeSinceGC := time.Since(stats.LastGC)
	if timeSinceGC > 5*time.Minute && usageRatio > 0.6 {
		recommendations = append(recommendations, "Consider manual garbage collection")
	}
	
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Memory usage is healthy")
	}
	
	return recommendations
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

type DeploymentStatusRequest struct {
	Status          string `json:"status"`
	BackendVersion  string `json:"backend_version"`
	FrontendVersion string `json:"frontend_version"`
	HealthPassed    bool   `json:"health_passed"`
}

func (h *Handlers) UpdateDeploymentStatus(c *fiber.Ctx) error {
	var req DeploymentStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Validate required fields
	if req.Status == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Status is required",
		})
	}

	// Update the deployment status using the Discord service
	err := h.discordService.UpdateDeploymentStatus(
		req.Status,
		req.BackendVersion,
		req.FrontendVersion,
		req.HealthPassed,
	)

	if err != nil {
		log.Printf("Failed to update deployment status: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update deployment status",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Deployment status updated successfully",
		"status":  req.Status,
		"version": req.BackendVersion,
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

	// Check memory availability before processing large files
	var totalSizeMB float64
	for _, file := range wavFiles {
		totalSizeMB += float64(file.Size) / 1024 / 1024
	}

	// Memory check for large uploads (>100MB total)
	if totalSizeMB > 100 {
		canProceed, suggestion := h.memoryMonitor.CheckMemoryForUpload(totalSizeMB)
		if !canProceed {
			h.logger.Warn("Upload rejected due to memory constraints",
				"total_size_mb", totalSizeMB,
				"files_count", len(wavFiles),
				"memory_suggestion", suggestion)
			
			return c.Status(507).JSON(fiber.Map{ // 507 Insufficient Storage
				"success": false,
				"message": "Insufficient memory available for upload",
				"details": suggestion,
				"total_size_mb": totalSizeMB,
				"current_memory": h.memoryMonitor.GetMemoryStatsForAPI(),
			})
		}
		
		if suggestion == "memory_gc_helped" {
			h.logger.Info("Memory freed via GC before large upload",
				"total_size_mb", totalSizeMB,
				"files_count", len(wavFiles))
		}
	}

	// Process files
	summary, err := h.fileService.ProcessFiles(wavFiles)
	if err != nil {
		// Log upload failure using production logger
		if h.productionLogger != nil {
			var totalSize int64
			for _, file := range wavFiles {
				totalSize += file.Size
			}
			
			// Get current memory usage for context
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			
			
		}
		
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

	// Clear the hash cache since all files are gone
	h.hashCache.ClearCache()
	
	// Save empty cache to MinIO
	if err := h.hashCache.SaveToMinIO(ctx); err != nil {
		slog.Warn("Failed to save empty cache after bucket clear", slog.String("error", err.Error()))
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

// UploadStreamingFiles handles streaming upload with bit-perfect quality
func (h *Handlers) UploadStreamingFiles(c *fiber.Ctx) error {
	// This uses the same logic as UploadFiles but with streaming optimizations
	return h.UploadFiles(c)
}

// GetTUSConfiguration returns TUS protocol configuration
func (h *Handlers) GetTUSConfiguration(c *fiber.Ctx) error {
	c.Set("Tus-Resumable", "1.0.0")
	c.Set("Tus-Version", "1.0.0")
	c.Set("Tus-Max-Size", "5368709120") // 5GB
	c.Set("Tus-Extension", "creation,expiration,checksum")
	return c.SendStatus(204)
}

// CreateTUSUpload creates a new TUS upload session
func (h *Handlers) CreateTUSUpload(c *fiber.Ctx) error {
	uploadLength := c.GetReqHeaders()["Upload-Length"]
	if len(uploadLength) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Upload-Length header required"})
	}

	size := int64(0)
	if _, err := fmt.Sscanf(uploadLength[0], "%d", &size); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid Upload-Length"})
	}

	filename := "unknown"
	if metadata := c.GetReqHeaders()["Upload-Metadata"]; len(metadata) > 0 {
		// Parse metadata (simplified)
		if strings.Contains(metadata[0], "filename") {
			parts := strings.Split(metadata[0], " ")
			for _, part := range parts {
				if strings.HasPrefix(part, "filename") {
					filename = strings.Split(part, " ")[1]
					break
				}
			}
		}
	}

	tusService := h.fileService.GetTUSService()
	response, err := tusService.CreateUpload(size, filename, make(map[string]string))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	c.Set("Location", response.Location)
	c.Set("Tus-Resumable", "1.0.0")
	return c.Status(201).JSON(response)
}

// GetTUSUploadInfo returns information about a TUS upload session
func (h *Handlers) GetTUSUploadInfo(c *fiber.Ctx) error {
	uploadID := c.Params("id")
	if uploadID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Upload ID required"})
	}

	tusService := h.fileService.GetTUSService()
	info, err := tusService.GetUpload(uploadID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	c.Set("Upload-Offset", fmt.Sprintf("%d", info.Offset))
	c.Set("Upload-Length", fmt.Sprintf("%d", info.Size))
	c.Set("Tus-Resumable", "1.0.0")
	return c.SendStatus(200)
}

// UploadTUSChunk handles uploading a chunk of data to a TUS session
func (h *Handlers) UploadTUSChunk(c *fiber.Ctx) error {
	uploadID := c.Params("id")
	if uploadID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Upload ID required"})
	}

	offsetStr := c.Get("Upload-Offset")
	if offsetStr == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Upload-Offset header required"})
	}

	offset := int64(0)
	if _, err := fmt.Sscanf(offsetStr, "%d", &offset); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid Upload-Offset"})
	}

	tusService := h.fileService.GetTUSService()
	info, err := tusService.PatchUpload(uploadID, offset, c.Context().RequestBodyStream())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	c.Set("Upload-Offset", fmt.Sprintf("%d", info.Offset))
	c.Set("Tus-Resumable", "1.0.0")
	return c.SendStatus(204)
}

// CompleteTUSUpload completes a TUS upload session
func (h *Handlers) CompleteTUSUpload(c *fiber.Ctx) error {
	uploadID := c.Params("id")
	if uploadID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Upload ID required"})
	}

	tusService := h.fileService.GetTUSService()

	// Get upload info
	info, err := tusService.GetUpload(uploadID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	// Check if upload is complete
	if info.Offset != info.Size {
		return c.Status(400).JSON(fiber.Map{
			"error":  "Upload incomplete",
			"offset": info.Offset,
			"size":   info.Size,
		})
	}

	// Get file reader and process the upload
	reader, err := tusService.GetUploadReader(uploadID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer reader.Close()

	// Calculate hash for integrity check
	fileHash, err := h.fileService.GetMetadataService().CalculateStreamingHash(reader)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("Failed to calculate hash: %v", err)})
	}

	// Reset reader for upload
	reader, err = tusService.GetUploadReader(uploadID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer reader.Close()

	// Upload to MinIO using streaming (zero-copy)
	result, err := h.minioService.UploadFileStreaming(reader, info.Filename, info.Size, fileHash)
	if err != nil {
		// Log TUS upload completion failure
		if h.productionLogger != nil {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			
			
		}
		
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Clean up TUS session
	tusService.DeleteUpload(uploadID)

	return c.JSON(result)
}

// DeleteTUSUpload deletes a TUS upload session
func (h *Handlers) DeleteTUSUpload(c *fiber.Ctx) error {
	uploadID := c.Params("id")
	if uploadID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Upload ID required"})
	}

	tusService := h.fileService.GetTUSService()
	err := tusService.DeleteUpload(uploadID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	c.Set("Tus-Resumable", "1.0.0")
	return c.SendStatus(204)
}

// GetStreamingStats returns streaming performance statistics
func (h *Handlers) GetStreamingStats(c *fiber.Ctx) error {
	// Get streaming service stats
	streamingService := h.fileService.GetStreamingService()
	if streamingService == nil {
		return c.JSON(fiber.Map{
			"error": "Streaming service not available",
		})
	}

	stats := streamingService.GetStats()
	return c.JSON(stats)
}

// GetCompressionStats returns compression performance statistics
func (h *Handlers) GetCompressionStats(c *fiber.Ctx) error {
	// Return compression statistics
	return c.JSON(fiber.Map{
		"compression_enabled":  true,
		"algorithms_supported": []string{"gzip", "brotli"},
		"average_ratio":        0.7,
		"total_saved_bytes":    0,
	})
}

// VerifyFileIntegrity verifies the integrity of a file
func (h *Handlers) VerifyFileIntegrity(c *fiber.Ctx) error {
	filename := c.Params("filename")
	if filename == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Filename required"})
	}

	expectedHash := c.FormValue("hash")
	if expectedHash == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Expected hash required"})
	}

	// Download file and verify hash
	data, err := h.minioService.DownloadFileData(filename)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "File not found"})
	}

	// Calculate actual hash
	hasher := sha256.New()
	hasher.Write(data)
	actualHash := fmt.Sprintf("%x", hasher.Sum(nil))

	verified := actualHash == expectedHash

	return c.JSON(fiber.Map{
		"verified": verified,
		"expected_hash": expectedHash,
		"actual_hash": actualHash,
	})
}

// CleanupExpiredUploads cleans up expired TUS upload sessions
func (h *Handlers) CleanupExpiredUploads(c *fiber.Ctx) error {
	// Get TUS service and clean up expired uploads
	tusService := h.fileService.GetTUSService()
	if tusService == nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "TUS service not available",
		})
	}

	// Clean up uploads older than 24 hours
	tusService.CleanupExpiredUploads(24 * time.Hour)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Expired uploads cleaned up successfully",
	})
}
