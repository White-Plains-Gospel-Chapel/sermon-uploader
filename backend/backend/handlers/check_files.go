package handlers

import (
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
)

// FileInfo represents basic file information for quick duplicate check
type FileInfo struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Exists bool   `json:"exists"`
	Path   string `json:"path,omitempty"`
}

// CheckFilesByInfo checks if files exist by name and size (instant check)
func (h *Handlers) CheckFilesByInfo(c *fiber.Ctx) error {
	var files []FileInfo
	if err := c.BodyParser(&files); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Get all existing files from MinIO
	ctx := c.Context()
	existingFiles := make(map[string]int64) // filename -> size

	objectCh := h.minioService.GetClient().ListObjects(ctx, "sermons", minio.ListObjectsOptions{
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			continue
		}
		// Store base filename (without timestamp) and size
		baseName := extractBaseName(object.Key)
		existingFiles[baseName] = object.Size
	}

	// Check each file
	results := make([]FileInfo, len(files))
	duplicateCount := 0
	
	for i, file := range files {
		results[i] = FileInfo{
			Name:   file.Name,
			Size:   file.Size,
			Exists: false,
		}

		// Check by base name and size
		baseName := extractBaseName(file.Name)
		if existingSize, exists := existingFiles[baseName]; exists {
			// Check if size matches (high confidence it's the same file)
			if existingSize == file.Size {
				results[i].Exists = true
				results[i].Path = baseName
				duplicateCount++
				slog.Info("Duplicate detected by name+size",
					slog.String("name", file.Name),
					slog.Int64("size", file.Size))
			}
		}
	}

	slog.Info("Files checked for duplicates",
		slog.Int("total", len(files)),
		slog.Int("duplicates", duplicateCount))

	return c.JSON(fiber.Map{
		"files":      results,
		"duplicates": duplicateCount,
		"total":      len(files),
	})
}

// extractBaseName removes timestamp suffixes from filenames
func extractBaseName(filename string) string {
	// Remove common timestamp patterns like _1757288891.wav
	if idx := strings.LastIndex(filename, "_"); idx > 0 {
		ext := strings.LastIndex(filename, ".")
		if ext > idx {
			// Check if between _ and . is a number (timestamp)
			timestamp := filename[idx+1 : ext]
			isNumber := true
			for _, c := range timestamp {
				if c < '0' || c > '9' {
					isNumber = false
					break
				}
			}
			if isNumber && len(timestamp) >= 10 {
				// It's a timestamp, remove it
				return filename[:idx] + filename[ext:]
			}
		}
	}
	return filename
}