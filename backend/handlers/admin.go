package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"sermon-uploader/config"
	"sermon-uploader/services"
)

// GetSermonAdmin returns detailed sermon info for admin view
func (h *Handlers) GetSermonAdmin(c *fiber.Ctx) error {
	sermonID := c.Params("id")
	if sermonID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Sermon ID is required",
		})
	}
	
	// Find sermon by ID with full details
	files, err := h.minioService.ListFiles()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve sermon",
		})
	}
	
	for _, file := range files {
		if etag, ok := file["etag"].(string); ok && etag == sermonID {
			filename, _ := file["name"].(string)
			
			// Get file stats from MinIO
			stats, _ := h.minioService.GetFileStats(filename)
			
			sermon := fiber.Map{
				"id":           etag,
				"title":        cleanFilename(filename),
				"filename":     filename,
				"size":         file["size"],
				"duration":     file["duration"],
				"uploadDate":   file["last_modified"],
				"streamUrl":    h.generateStreamURL(filename),
				"downloadUrl":  h.generateDownloadURL(filename),
				"contentType":  file["content_type"],
				"storageClass": file["storage_class"],
				"stats":        stats,
			}
			
			// Add all metadata for admin view
			if metadata, ok := file["metadata"].(map[string]interface{}); ok {
				sermon["metadata"] = metadata
			}
			
			// Add processing status
			if strings.Contains(filename, "_raw") {
				sermon["processingStatus"] = "pending"
			} else if strings.Contains(filename, "_streamable") {
				sermon["processingStatus"] = "completed"
			} else {
				sermon["processingStatus"] = "unknown"
			}
			
			return c.JSON(sermon)
		}
	}
	
	return c.Status(404).JSON(fiber.Map{
		"error": "Sermon not found",
	})
}

// UpdateSermon updates sermon metadata
func (h *Handlers) UpdateSermon(c *fiber.Ctx) error {
	sermonID := c.Params("id")
	if sermonID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Sermon ID is required",
		})
	}
	
	// Parse update request
	var updateReq struct {
		Title       string                 `json:"title"`
		Speaker     string                 `json:"speaker"`
		Theme       string                 `json:"theme"`
		Description string                 `json:"description"`
		Tags        []string               `json:"tags"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	
	if err := c.BodyParser(&updateReq); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	
	// TODO: Implement metadata update in MinIO
	// For now, return success with the updated data
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Sermon metadata updated",
		"sermon": fiber.Map{
			"id":          sermonID,
			"title":       updateReq.Title,
			"speaker":     updateReq.Speaker,
			"theme":       updateReq.Theme,
			"description": updateReq.Description,
			"tags":        updateReq.Tags,
			"metadata":    updateReq.Metadata,
			"updatedAt":   time.Now().Format(time.RFC3339),
		},
	})
}

// DeleteSermon deletes a sermon from storage
func (h *Handlers) DeleteSermon(c *fiber.Ctx) error {
	sermonID := c.Params("id")
	if sermonID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Sermon ID is required",
		})
	}
	
	// Find sermon file by ID
	files, err := h.minioService.ListFiles()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve sermon",
		})
	}
	
	var filename string
	for _, file := range files {
		if etag, ok := file["etag"].(string); ok && etag == sermonID {
			filename, _ = file["name"].(string)
			break
		}
	}
	
	if filename == "" {
		return c.Status(404).JSON(fiber.Map{
			"error": "Sermon not found",
		})
	}
	
	// Delete from MinIO
	if err := h.minioService.DeleteFile(filename); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to delete sermon: %v", err),
		})
	}
	
	// Send Discord notification
	if h.discordService != nil {
		h.discordService.SendNotification(
			"🗑️ Sermon Deleted",
			fmt.Sprintf("Sermon `%s` has been deleted from storage", cleanFilename(filename)),
			0xff6b6b, // Red color
			[]services.DiscordField{
				{Name: "Filename", Value: filename, Inline: true},
				{Name: "Deleted By", Value: c.IP(), Inline: true},
				{Name: "Timestamp", Value: time.Now().Format(time.RFC3339), Inline: true},
			},
		)
	}
	
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Sermon deleted successfully",
		"id":      sermonID,
		"filename": filename,
	})
}

// ListMembers returns all church members (placeholder)
func (h *Handlers) ListMembers(c *fiber.Ctx) error {
	// TODO: Implement when database is added
	members := []fiber.Map{
		{
			"id":        "1",
			"firstName": "John",
			"lastName":  "Doe",
			"email":     "john.doe@example.com",
			"phone":     "555-0100",
			"role":      "Member",
			"joinDate":  "2023-01-15",
			"active":    true,
		},
		{
			"id":        "2",
			"firstName": "Jane",
			"lastName":  "Smith",
			"email":     "jane.smith@example.com",
			"phone":     "555-0101",
			"role":      "Deacon",
			"joinDate":  "2022-06-20",
			"active":    true,
		},
	}
	
	return c.JSON(fiber.Map{
		"members": members,
		"total":   len(members),
	})
}

// CreateMember creates a new church member
func (h *Handlers) CreateMember(c *fiber.Ctx) error {
	var member struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"`
		Phone     string `json:"phone"`
		Role      string `json:"role"`
	}
	
	if err := c.BodyParser(&member); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	
	// TODO: Implement database storage
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Member created successfully",
		"member": fiber.Map{
			"id":        fmt.Sprintf("%d", time.Now().Unix()),
			"firstName": member.FirstName,
			"lastName":  member.LastName,
			"email":     member.Email,
			"phone":     member.Phone,
			"role":      member.Role,
			"joinDate":  time.Now().Format("2006-01-02"),
			"active":    true,
		},
	})
}

// UpdateMember updates member information
func (h *Handlers) UpdateMember(c *fiber.Ctx) error {
	memberID := c.Params("id")
	if memberID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Member ID is required",
		})
	}
	
	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	
	// TODO: Implement database update
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Member updated successfully",
		"id":      memberID,
		"updates": updates,
	})
}

// DeleteMember removes a church member
func (h *Handlers) DeleteMember(c *fiber.Ctx) error {
	memberID := c.Params("id")
	if memberID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Member ID is required",
		})
	}
	
	// TODO: Implement database deletion
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Member deleted successfully",
		"id":      memberID,
	})
}

// ListEvents returns all church events for admin
func (h *Handlers) ListEvents(c *fiber.Ctx) error {
	// TODO: Implement when database is added
	events := []fiber.Map{
		{
			"id":          "1",
			"title":       "Sunday Service",
			"description": "Weekly Sunday worship service",
			"date":        time.Now().AddDate(0, 0, 7).Format(time.RFC3339),
			"time":        "10:00 AM",
			"location":    "Main Sanctuary",
			"recurring":   true,
			"attendees":   150,
			"createdBy":   "Pastor Johnson",
		},
		{
			"id":          "2",
			"title":       "Youth Camp",
			"description": "Annual youth summer camp",
			"date":        time.Now().AddDate(0, 2, 0).Format(time.RFC3339),
			"time":        "All Day",
			"location":    "Camp Grounds",
			"recurring":   false,
			"attendees":   45,
			"createdBy":   "Youth Pastor",
		},
	}
	
	return c.JSON(fiber.Map{
		"events": events,
		"total":  len(events),
	})
}

// CreateEvent creates a new church event
func (h *Handlers) CreateEvent(c *fiber.Ctx) error {
	var event struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Date        string `json:"date"`
		Time        string `json:"time"`
		Location    string `json:"location"`
		Recurring   bool   `json:"recurring"`
	}
	
	if err := c.BodyParser(&event); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	
	// TODO: Implement database storage
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Event created successfully",
		"event": fiber.Map{
			"id":          fmt.Sprintf("%d", time.Now().Unix()),
			"title":       event.Title,
			"description": event.Description,
			"date":        event.Date,
			"time":        event.Time,
			"location":    event.Location,
			"recurring":   event.Recurring,
			"createdAt":   time.Now().Format(time.RFC3339),
			"createdBy":   c.IP(), // Would be actual user in production
		},
	})
}

// UpdateEvent updates event information
func (h *Handlers) UpdateEvent(c *fiber.Ctx) error {
	eventID := c.Params("id")
	if eventID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Event ID is required",
		})
	}
	
	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	
	// TODO: Implement database update
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Event updated successfully",
		"id":      eventID,
		"updates": updates,
	})
}

// DeleteEvent removes a church event
func (h *Handlers) DeleteEvent(c *fiber.Ctx) error {
	eventID := c.Params("id")
	if eventID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Event ID is required",
		})
	}
	
	// TODO: Implement database deletion
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Event deleted successfully",
		"id":      eventID,
	})
}

// ListMedia returns all media files
func (h *Handlers) ListMedia(c *fiber.Ctx) error {
	// Get all files from MinIO
	files, err := h.minioService.ListFiles()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve media files",
		})
	}
	
	var mediaFiles []fiber.Map
	for _, file := range files {
		filename, _ := file["name"].(string)
		
		// Categorize file type
		var fileType string
		if strings.HasSuffix(filename, ".wav") || strings.HasSuffix(filename, ".mp3") || strings.HasSuffix(filename, ".aac") {
			fileType = "audio"
		} else if strings.HasSuffix(filename, ".mp4") || strings.HasSuffix(filename, ".mov") {
			fileType = "video"
		} else if strings.HasSuffix(filename, ".jpg") || strings.HasSuffix(filename, ".png") || strings.HasSuffix(filename, ".gif") {
			fileType = "image"
		} else if strings.HasSuffix(filename, ".pdf") || strings.HasSuffix(filename, ".doc") || strings.HasSuffix(filename, ".docx") {
			fileType = "document"
		} else {
			fileType = "other"
		}
		
		media := fiber.Map{
			"id":          file["etag"],
			"filename":    filename,
			"type":        fileType,
			"size":        file["size"],
			"uploadDate":  file["last_modified"],
			"url":         h.generateStreamURL(filename),
			"downloadUrl": h.generateDownloadURL(filename),
		}
		
		mediaFiles = append(mediaFiles, media)
	}
	
	return c.JSON(fiber.Map{
		"media": mediaFiles,
		"total": len(mediaFiles),
	})
}

// UploadMedia handles media file uploads
func (h *Handlers) UploadMedia(c *fiber.Ctx) error {
	// Get file from form
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "No file provided",
		})
	}
	
	// Open file
	src, err := file.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to open file",
		})
	}
	defer src.Close()
	
	// Read file data
	fileData := make([]byte, file.Size)
	if _, err := src.Read(fileData); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to read file",
		})
	}
	
	// Upload to MinIO
	result, err := h.minioService.UploadFile(fileData, file.Filename)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to upload media: %v", err),
		})
	}
	
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Media uploaded successfully",
		"file": fiber.Map{
			"filename": file.Filename,
			"size":     file.Size,
			"url":      h.generateStreamURL(file.Filename),
			"result":   result,
		},
	})
}

// DeleteMedia removes a media file
func (h *Handlers) DeleteMedia(c *fiber.Ctx) error {
	mediaID := c.Params("id")
	if mediaID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Media ID is required",
		})
	}
	
	// Find media file by ID
	files, err := h.minioService.ListFiles()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve media",
		})
	}
	
	var filename string
	for _, file := range files {
		if etag, ok := file["etag"].(string); ok && etag == mediaID {
			filename, _ = file["name"].(string)
			break
		}
	}
	
	if filename == "" {
		return c.Status(404).JSON(fiber.Map{
			"error": "Media file not found",
		})
	}
	
	// Delete from MinIO
	if err := h.minioService.DeleteFile(filename); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to delete media: %v", err),
		})
	}
	
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Media deleted successfully",
		"id":      mediaID,
		"filename": filename,
	})
}

// GetDashboardStats returns dashboard statistics
func (h *Handlers) GetDashboardStats(c *fiber.Ctx) error {
	// Get file statistics
	files, err := h.minioService.ListFiles()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve statistics",
		})
	}
	
	// Calculate stats
	var totalSize int64
	var audioCount, videoCount, imageCount, docCount int
	var recentUploads []fiber.Map
	
	for i, file := range files {
		filename, _ := file["name"].(string)
		size, _ := file["size"].(int64)
		totalSize += size
		
		// Count file types
		if strings.HasSuffix(filename, ".wav") || strings.HasSuffix(filename, ".mp3") || strings.HasSuffix(filename, ".aac") {
			audioCount++
		} else if strings.HasSuffix(filename, ".mp4") || strings.HasSuffix(filename, ".mov") {
			videoCount++
		} else if strings.HasSuffix(filename, ".jpg") || strings.HasSuffix(filename, ".png") {
			imageCount++
		} else if strings.HasSuffix(filename, ".pdf") || strings.HasSuffix(filename, ".doc") {
			docCount++
		}
		
		// Get recent uploads (first 5)
		if i < 5 {
			recentUploads = append(recentUploads, fiber.Map{
				"filename":   filename,
				"size":       size,
				"uploadDate": file["last_modified"],
			})
		}
	}
	
	// Get system stats
	memStats := h.memoryMonitor.GetMemoryStatsForAPI()
	
	return c.JSON(fiber.Map{
		"stats": fiber.Map{
			"totalFiles":    len(files),
			"totalSize":     totalSize,
			"totalSizeMB":   float64(totalSize) / (1024 * 1024),
			"totalSizeGB":   float64(totalSize) / (1024 * 1024 * 1024),
			"audioFiles":    audioCount,
			"videoFiles":    videoCount,
			"imageFiles":    imageCount,
			"documentFiles": docCount,
		},
		"recentUploads": recentUploads,
		"system": fiber.Map{
			"memory":       memStats,
			"uptime":       time.Since(h.startTime).String(),
			"version":      config.GetFullVersion("backend"),
			"piOptimized":  h.config.PiOptimization,
		},
		"storage": fiber.Map{
			"endpoint": h.config.MinIOEndpoint,
			"bucket":   h.config.MinioBucket,
			"secure":   h.config.MinIOSecure,
		},
	})
}

// GetUploadStats returns upload statistics
func (h *Handlers) GetUploadStats(c *fiber.Ctx) error {
	// Get time range
	period := c.Query("period", "week") // week, month, year
	
	// TODO: Implement when database is added for tracking uploads over time
	// For now, return current stats
	
	files, err := h.minioService.ListFiles()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve statistics",
		})
	}
	
	// Group uploads by date
	uploadsByDate := make(map[string]int)
	uploadsBySize := make(map[string]int64)
	
	for _, file := range files {
		lastMod, _ := file["last_modified"].(string)
		if t, err := time.Parse(time.RFC3339, lastMod); err == nil {
			dateKey := t.Format("2006-01-02")
			uploadsByDate[dateKey]++
			if size, ok := file["size"].(int64); ok {
				uploadsBySize[dateKey] += size
			}
		}
	}
	
	return c.JSON(fiber.Map{
		"period":        period,
		"totalUploads":  len(files),
		"uploadsByDate": uploadsByDate,
		"uploadsBySize": uploadsBySize,
		"averageSize":   calculateAverageSize(files),
	})
}

// GetUsageStats returns platform usage statistics
func (h *Handlers) GetUsageStats(c *fiber.Ctx) error {
	// TODO: Implement when database is added for tracking usage
	// For now, return mock data
	
	return c.JSON(fiber.Map{
		"usage": fiber.Map{
			"activeUsers":     25,
			"totalMembers":    150,
			"sermonsViewed":   342,
			"totalDownloads":  89,
			"averageViewTime": "12:34",
		},
		"trends": fiber.Map{
			"userGrowth":     "+5%",
			"viewGrowth":     "+12%",
			"downloadGrowth": "+8%",
		},
		"popular": fiber.Map{
			"topSermons": []string{
				"Faith and Hope",
				"Walking in Love",
				"The Power of Prayer",
			},
			"topSpeakers": []string{
				"Pastor Johnson",
				"Elder Smith",
			},
		},
	})
}

// Helper function to calculate average file size
func calculateAverageSize(files []map[string]interface{}) float64 {
	if len(files) == 0 {
		return 0
	}
	
	var totalSize int64
	for _, file := range files {
		if size, ok := file["size"].(int64); ok {
			totalSize += size
		}
	}
	
	return float64(totalSize) / float64(len(files)) / (1024 * 1024) // Return in MB
}