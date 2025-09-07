package main

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
	
	"sermon-uploader/config"
	"sermon-uploader/handlers"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Initialize configuration
	cfg := config.New()

	// Create Fiber app
	app := fiber.New(fiber.Config{
		BodyLimit:         5 * 1024 * 1024 * 1024, // 5GB limit
		ServerHeader:      "Sermon-Uploader",
		AppName:           "Sermon Uploader API v3.0",
		ReadTimeout:       30 * time.Minute,
		WriteTimeout:      30 * time.Minute,
		IdleTimeout:       30 * time.Minute,
		StreamRequestBody: true,
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())
	
	// CORS - Allow all origins for simplicity
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "*",
		ExposeHeaders:    "*",
		AllowCredentials: false,
		MaxAge:           86400,
	}))

	// Initialize unified upload handler
	uploadHandler, err := handlers.NewUnifiedUploadHandler(
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinioBucket,
		cfg.MinIOSecure,
	)
	if err != nil {
		log.Fatalf("Failed to initialize upload handler: %v", err)
	}

	// API Routes
	api := app.Group("/api")
	
	// Health check
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "healthy",
			"time":   time.Now(),
		})
	})

	// ===== SINGLE UNIFIED UPLOAD ENDPOINT =====
	api.Post("/upload", uploadHandler.HandleUpload)
	
	// That's it! One endpoint handles everything:
	// - Start upload:    {"action": "start", "filename": "x.wav", "fileSize": 123}
	// - Get upload URL:  {"action": "get_url", "uploadId": "x", "partNumber": 1}
	// - Complete upload: {"action": "complete", "uploadId": "x", "parts": [...]}
	// - Abort upload:    {"action": "abort", "uploadId": "x"}
	// - Check status:    {"action": "status", "uploadId": "x"}

	// Start cleanup task
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		
		for range ticker.C {
			uploadHandler.CleanupStaleSessions()
			log.Println("Cleaned up stale upload sessions")
		}
	}()

	// Start server
	port := cfg.Port
	if port == "" {
		port = "8000"
	}

	log.Printf("🚀 Server starting on port %s", port)
	log.Printf("📦 MinIO endpoint: %s (HTTPS: %v)", cfg.MinIOEndpoint, cfg.MinIOSecure)
	log.Printf("✨ Single unified endpoint: POST /api/upload")
	log.Printf("📤 Actions: start, get_url, complete, abort, status")
	
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}