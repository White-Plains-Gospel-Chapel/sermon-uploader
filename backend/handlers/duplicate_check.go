package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// CheckDuplicate checks if a filename already exists (fast O(1) operation)
func (h *Handlers) CheckDuplicate(c *fiber.Ctx) error {
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

	// Fast duplicate check - O(1) operation, works with millions of files
	isDuplicate, err := h.minioService.CheckDuplicateByFilename(req.Filename)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to check for duplicates",
		})
	}

	return c.JSON(fiber.Map{
		"filename":    req.Filename,
		"isDuplicate": isDuplicate,
		"message":     func() string {
			if isDuplicate {
				return "File already exists"
			}
			return "File is unique"
		}(),
	})
}