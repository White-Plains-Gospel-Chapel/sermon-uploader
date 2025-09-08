package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ListPublicSermons returns public-facing sermon list
func (h *Handlers) ListPublicSermons(c *fiber.Ctx) error {
	// Query parameters for pagination and filtering
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	search := c.Query("search")
	speaker := c.Query("speaker")
	
	// Get all files from MinIO
	files, err := h.minioService.ListFiles()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve sermons",
		})
	}
	
	// Filter for public display (only processed files)
	var publicSermons []fiber.Map
	for _, file := range files {
		filename, _ := file["name"].(string)
		
		// Only show streamable/processed files
		if !contains(filename, "_streamable") && !contains(filename, ".aac") {
			continue
		}
		
		// Apply search filter if provided
		if search != "" && !contains(filename, search) {
			continue
		}
		
		// Apply speaker filter if provided
		if speaker != "" {
			// TODO: Implement speaker metadata filtering
		}
		
		sermon := fiber.Map{
			"id":          file["etag"],
			"title":       cleanFilename(filename),
			"filename":    filename,
			"size":        file["size"],
			"duration":    file["duration"],
			"uploadDate":  file["last_modified"],
			"streamUrl":   h.generateStreamURL(filename),
			"downloadUrl": h.generateDownloadURL(filename),
		}
		
		// Add metadata if available
		if metadata, ok := file["metadata"].(map[string]interface{}); ok {
			if speaker, ok := metadata["speaker"].(string); ok {
				sermon["speaker"] = speaker
			}
			if theme, ok := metadata["theme"].(string); ok {
				sermon["theme"] = theme
			}
		}
		
		publicSermons = append(publicSermons, sermon)
	}
	
	// Apply pagination
	start := (page - 1) * limit
	end := start + limit
	if end > len(publicSermons) {
		end = len(publicSermons)
	}
	if start > len(publicSermons) {
		start = len(publicSermons)
	}
	
	paginatedSermons := publicSermons[start:end]
	
	return c.JSON(fiber.Map{
		"sermons": paginatedSermons,
		"pagination": fiber.Map{
			"page":       page,
			"limit":      limit,
			"total":      len(publicSermons),
			"totalPages": (len(publicSermons) + limit - 1) / limit,
		},
	})
}

// GetSermon returns details for a specific sermon
func (h *Handlers) GetSermon(c *fiber.Ctx) error {
	sermonID := c.Params("id")
	if sermonID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Sermon ID is required",
		})
	}
	
	// Find sermon by ID (etag)
	files, err := h.minioService.ListFiles()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve sermon",
		})
	}
	
	for _, file := range files {
		if etag, ok := file["etag"].(string); ok && etag == sermonID {
			filename, _ := file["name"].(string)
			
			sermon := fiber.Map{
				"id":          etag,
				"title":       cleanFilename(filename),
				"filename":    filename,
				"size":        file["size"],
				"duration":    file["duration"],
				"uploadDate":  file["last_modified"],
				"streamUrl":   h.generateStreamURL(filename),
				"downloadUrl": h.generateDownloadURL(filename),
			}
			
			// Add metadata if available
			if metadata, ok := file["metadata"].(map[string]interface{}); ok {
				sermon["metadata"] = metadata
			}
			
			return c.JSON(sermon)
		}
	}
	
	return c.Status(404).JSON(fiber.Map{
		"error": "Sermon not found",
	})
}

// GetLatestSermons returns the most recent sermons
func (h *Handlers) GetLatestSermons(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 5)
	
	// Get all files from MinIO (already sorted by date)
	files, err := h.minioService.ListFiles()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve sermons",
		})
	}
	
	var latestSermons []fiber.Map
	count := 0
	
	for _, file := range files {
		if count >= limit {
			break
		}
		
		filename, _ := file["name"].(string)
		
		// Only show streamable/processed files
		if !contains(filename, "_streamable") && !contains(filename, ".aac") {
			continue
		}
		
		sermon := fiber.Map{
			"id":          file["etag"],
			"title":       cleanFilename(filename),
			"filename":    filename,
			"size":        file["size"],
			"duration":    file["duration"],
			"uploadDate":  file["last_modified"],
			"streamUrl":   h.generateStreamURL(filename),
		}
		
		latestSermons = append(latestSermons, sermon)
		count++
	}
	
	return c.JSON(fiber.Map{
		"sermons": latestSermons,
		"count":   len(latestSermons),
	})
}

// GetPublicEvents returns public church events
func (h *Handlers) GetPublicEvents(c *fiber.Ctx) error {
	// TODO: Implement when database is added
	// For now, return sample data
	events := []fiber.Map{
		{
			"id":          "1",
			"title":       "Sunday Service",
			"description": "Weekly Sunday worship service",
			"date":        time.Now().AddDate(0, 0, 7).Format(time.RFC3339),
			"time":        "10:00 AM",
			"location":    "Main Sanctuary",
			"recurring":   true,
		},
		{
			"id":          "2",
			"title":       "Wednesday Bible Study",
			"description": "Mid-week Bible study and prayer",
			"date":        time.Now().AddDate(0, 0, 3).Format(time.RFC3339),
			"time":        "7:00 PM",
			"location":    "Fellowship Hall",
			"recurring":   true,
		},
	}
	
	return c.JSON(fiber.Map{
		"events": events,
		"count":  len(events),
	})
}

// GetAnnouncements returns church announcements
func (h *Handlers) GetAnnouncements(c *fiber.Ctx) error {
	// TODO: Implement when database is added
	// For now, return sample data
	announcements := []fiber.Map{
		{
			"id":        "1",
			"title":     "New Sermon Series",
			"content":   "Join us for our new sermon series starting this Sunday",
			"date":      time.Now().Format(time.RFC3339),
			"important": true,
		},
		{
			"id":        "2",
			"title":     "Youth Group Meeting",
			"content":   "Youth group meets every Friday at 6 PM",
			"date":      time.Now().AddDate(0, 0, -1).Format(time.RFC3339),
			"important": false,
		},
	}
	
	return c.JSON(fiber.Map{
		"announcements": announcements,
		"count":         len(announcements),
	})
}

// Helper functions

func (h *Handlers) generateStreamURL(filename string) string {
	// Generate streaming URL based on configuration
	if h.config.MinIOSecure {
		return "https://" + h.config.MinIOEndpoint + "/" + h.config.MinioBucket + "/" + filename
	}
	return "http://" + h.config.MinIOEndpoint + "/" + h.config.MinioBucket + "/" + filename
}

func (h *Handlers) generateDownloadURL(filename string) string {
	// Generate download URL with content-disposition
	streamURL := h.generateStreamURL(filename)
	return streamURL + "?response-content-disposition=attachment"
}

func cleanFilename(filename string) string {
	// Remove suffixes and clean up filename for display
	cleaned := filename
	cleaned = strings.ReplaceAll(cleaned, "_streamable", "")
	cleaned = strings.ReplaceAll(cleaned, "_raw", "")
	cleaned = strings.ReplaceAll(cleaned, ".aac", "")
	cleaned = strings.ReplaceAll(cleaned, ".wav", "")
	cleaned = strings.ReplaceAll(cleaned, ".mp3", "")
	cleaned = strings.ReplaceAll(cleaned, "_", " ")
	cleaned = strings.TrimSpace(cleaned)
	
	// Capitalize first letter of each word
	words := strings.Split(cleaned, " ")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	
	return strings.Join(words, " ")
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}