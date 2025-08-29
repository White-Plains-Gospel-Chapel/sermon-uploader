package main

import (
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/websocket/v2"
	"github.com/joho/godotenv"

	"sermon-uploader/config"
	"sermon-uploader/handlers"
	"sermon-uploader/services"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}
	
	// Load Eastern Time zone for consistent logging
	easternTZ, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Printf("Failed to load Eastern timezone: %v", err)
		easternTZ = time.UTC // fallback to UTC
	}

	// Initialize configuration
	cfg := config.New()

	// Initialize services
	minioService := services.NewMinIOService(cfg)
	discordService := services.NewDiscordService(cfg.DiscordWebhookURL)
	wsHub := services.NewWebSocketHub()
	fileService := services.NewFileService(minioService, discordService, wsHub, cfg)

	// Test MinIO connection with Eastern Time logging
	if err := minioService.TestConnection(); err != nil {
		easternTime := time.Now().In(easternTZ)
		timestamp := easternTime.Format("2006/01/02 15:04:05 MST")
		log.Printf("[%s] ⚠️  MinIO connection failed: %v", timestamp, err)
	} else {
		easternTime := time.Now().In(easternTZ)
		timestamp := easternTime.Format("2006/01/02 15:04:05 MST")
		log.Printf("[%s] ✅ MinIO connection successful", timestamp)
	}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		BodyLimit: 2 * 1024 * 1024 * 1024, // 2GB limit for batch uploads of large WAV files
		ErrorHandler: func(ctx *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return ctx.Status(code).JSON(fiber.Map{
				"error":   true,
				"message": err.Error(),
			})
		},
	})

	// Middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
		AllowCredentials: true,
	}))

	// Configure logger with Eastern Time
	app.Use(logger.New(logger.Config{
		TimeZone: "America/New_York",
		Format:   "${time} | ${status} | ${latency} | ${ip} | ${method} | ${path}\n",
	}))

	// Initialize handlers
	h := handlers.New(fileService, minioService, discordService, wsHub, cfg)

	// Routes
	api := app.Group("/api")
	{
		api.Get("/health", h.HealthCheck)
		api.Get("/status", h.GetStatus)
		api.Get("/dashboard", h.GetDashboard)
		api.Post("/upload", h.UploadFiles)
		api.Get("/files", h.ListFiles)
		
		// Direct upload routes (better for large files)
		api.Post("/upload/presigned", h.GetPresignedURL)
		api.Post("/upload/presigned-batch", h.GetPresignedURLsBatch)
		api.Post("/upload/complete", h.ProcessUploadedFile)
		api.Post("/check-duplicate", h.CheckDuplicate)
		
		// Keep existing upload methods - chunking handled on frontend
		// Chunked uploads use the same presigned URL system
		
		// Test endpoints
		api.Post("/test/discord", h.TestDiscord)
		api.Get("/test/minio", h.TestMinIO)
		
		// Migration endpoint
		api.Post("/migrate/minio", h.MigrateMinIO)
		
		// Dangerous operations (require confirmation)
		api.Delete("/bucket/clear", h.ClearBucket)
	}

	// WebSocket endpoint
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		wsHub.HandleConnection(c)
	}))

	// Serve static files (React build)
	app.Static("/", "./frontend/out")
	app.Get("/*", func(c *fiber.Ctx) error {
		return c.SendFile("./frontend/out/index.html")
	})

	// Send startup notification
	go func() {
		if err := discordService.SendStartupNotification("🚀 Sermon Uploader Pi started successfully!"); err != nil {
			log.Printf("Failed to send startup notification: %v", err)
		}
	}()

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	// Log server startup with Eastern Time
	easternTime := time.Now().In(easternTZ)
	timestamp := easternTime.Format("2006/01/02 15:04:05 MST")
	log.Printf("[%s] 🚀 Server starting on port %s", timestamp, port)
	log.Printf("[%s] 🌐 Access at http://your-pi-ip:%s", timestamp, port)
	
	if err := app.Listen(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}