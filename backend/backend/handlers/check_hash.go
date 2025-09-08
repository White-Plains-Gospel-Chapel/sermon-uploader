package handlers

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
)

// CheckHash performs ultra-fast duplicate check - O(1) lookup
func (h *Handlers) CheckHash(c *fiber.Ctx) error {
	startTime := time.Now()
	hash := c.Params("hash")
	
	if hash == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Hash parameter required",
		})
	}
	
	// Check if cache is ready
	if !h.hashCache.IsReady() {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Hash cache still loading, please try again in a moment",
			"ready": false,
		})
	}
	
	// Ultra-fast O(1) lookup
	exists, filename := h.hashCache.CheckDuplicate(hash)
	
	// Log the check (but after response for speed)
	defer func() {
		h.logger.Info("Hash check performed",
			slog.String("hash", hash[:8]+"..."),
			slog.Bool("exists", exists),
			slog.Duration("lookup_time", time.Since(startTime)))
	}()
	
	return c.JSON(fiber.Map{
		"exists":       exists,
		"filename":     filename,
		"lookup_time":  time.Since(startTime).Microseconds(), // in microseconds
	})
}

// GetHashStats returns cache statistics
func (h *Handlers) GetHashStats(c *fiber.Ctx) error {
	stats := h.hashCache.GetStats()
	return c.JSON(stats)
}