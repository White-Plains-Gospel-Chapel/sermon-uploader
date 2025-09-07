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
		BodyLimit:             int(cfg.MaxUploadSize), // 5GB limit
		DisableStartupMessage: false,
		ServerHeader:          "Sermon-Uploader",
		AppName:               "Sermon Uploader API v2.0",
		ReadTimeout:           30 * time.Minute, // Long timeout for large uploads
		WriteTimeout:          30 * time.Minute,
		IdleTimeout:           30 * time.Minute,
		StreamRequestBody:     true, // Enable streaming for large files
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} - ${latency}\n",
	}))

	// CORS Configuration - CRITICAL for browser uploads
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "https://sermons.wpgc.church,http://localhost:3000,https://localhost:3000",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,PATCH",
		AllowHeaders:     "*",
		ExposeHeaders:    "Upload-Offset,Location,Upload-Length,Tus-Resumable,Upload-Metadata",
		AllowCredentials: true,
		MaxAge:           86400,
	}))

	// Initialize handlers
	multipartHandler, err := handlers.NewMultipartUploadHandler(
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinioBucket,
		cfg.MinIOSecure,
	)
	if err != nil {
		log.Fatalf("Failed to initialize multipart handler: %v", err)
	}

	// Initialize existing handlers (your current zero-memory proxy)
	// standardHandlers := handlers.NewHandlers(cfg)
	// Note: Commenting out for now as NewHandlers is not defined yet

	// API Routes
	api := app.Group("/api")

	// Health check
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "healthy",
			"time":   time.Now(),
			"https":  cfg.MinIOSecure,
		})
	})

	// ===== NEW MULTIPART UPLOAD ROUTES =====
	multipart := api.Group("/upload/multipart")
	
	// Initialize multipart upload
	multipart.Post("/init", multipartHandler.InitiateMultipartUpload)
	
	// Get presigned URL for part upload
	multipart.Get("/presigned", multipartHandler.GetPresignedURL)
	
	// Note: Direct upload to MinIO using presigned URLs, no proxy needed
	
	// Complete multipart upload
	multipart.Post("/complete", multipartHandler.CompleteMultipartUpload)
	
	// Abort multipart upload
	multipart.Delete("/abort/:uploadId", multipartHandler.AbortMultipartUpload)
	
	// List uploaded parts (for resumability)
	multipart.Get("/parts", multipartHandler.ListParts)
	
	// List active upload sessions
	multipart.Get("/sessions", multipartHandler.ListActiveSessions)

	// ===== EXISTING ROUTES (keep your current implementation) =====
	// Note: Commenting out existing routes until standardHandlers is defined
	// upload := api.Group("/upload")
	
	// Zero-memory streaming routes (your current implementation)
	// upload.Post("/zero-memory-url", standardHandlers.GetZeroMemoryUploadURL)
	// upload.Post("/zero-memory-url-batch", standardHandlers.GetZeroMemoryUploadURLBatch)
	// upload.Put("/zero-memory-proxy", standardHandlers.ZeroMemoryStreamingProxy)

	// Legacy routes
	// upload.Post("/", standardHandlers.UploadFiles)
	// upload.Post("/presigned", standardHandlers.GetPresignedURL)
	// upload.Post("/direct", standardHandlers.DirectUpload)

	// File management routes
	// api.Get("/files", standardHandlers.ListFiles)
	// api.Delete("/files/:filename", standardHandlers.DeleteFile)
	// api.Get("/status", standardHandlers.GetStatus)

	// Start background cleanup task
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		
		for range ticker.C {
			multipartHandler.CleanupStaleSessions()
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
	log.Printf("🔄 Multipart upload enabled with %d MB chunks", handlers.DefaultChunkSize/(1024*1024))
	log.Printf("📤 Max concurrent uploads: 1 file at a time (server-controlled queue)")
	
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}