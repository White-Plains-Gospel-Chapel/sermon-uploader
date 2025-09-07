package main

import (
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/pprof"
	"github.com/gofiber/websocket/v2"
	"github.com/joho/godotenv"

	"sermon-uploader/config"
	"sermon-uploader/handlers"
	"sermon-uploader/monitoring"
	"sermon-uploader/optimization"
	"sermon-uploader/services"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Initialize configuration first
	cfg := config.New()

	// Apply Pi-specific runtime optimizations
	if cfg.PiOptimization {
		configurePiRuntime(cfg)
	}

	// Initialize global optimization pools
	_ = optimization.GetGlobalPools() // Initialize pools

	// Initialize monitoring
	monitoring.InitGlobalMonitoring()
	metricsCollector := monitoring.GetMetricsCollector()
	healthChecker := monitoring.GetHealthChecker()

	// Load Eastern Time zone for consistent logging
	easternTZ, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Printf("Failed to load Eastern timezone: %v", err)
		easternTZ = time.UTC // fallback to UTC
	}

	// Log Pi optimization status
	if cfg.PiOptimization {
		log.Printf("🔧 Pi optimizations enabled: MaxProcs=%d, GOGC=%d, MemLimit=%dMB",
			runtime.GOMAXPROCS(0), cfg.GCTargetPercentage, cfg.MaxMemoryLimitMB)
	}

	// Initialize services
	minioService := services.NewMinIOService(cfg)
	discordService := services.NewDiscordService(cfg.DiscordWebhookURL)
	wsHub := services.NewWebSocketHub()
	fileService := services.NewFileService(minioService, discordService, wsHub, cfg)

	// Register health checks
	healthChecker.RegisterCheck("minio", func() error {
		return minioService.TestConnection()
	})

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
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,HEAD,PATCH",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,Upload-Length,Upload-Offset,Upload-Metadata,Tus-Resumable,Upload-Checksum",
		ExposeHeaders:    "Upload-Offset,Upload-Length,Tus-Resumable,Tus-Version,Tus-Max-Size,Tus-Extension,Tus-Checksum-Algorithm,Location",
		AllowCredentials: true,
	}))

	// Configure logger with Eastern Time
	app.Use(logger.New(logger.Config{
		TimeZone: "America/New_York",
		Format:   "${time} | ${status} | ${latency} | ${ip} | ${method} | ${path}\n",
	}))

	// Add metrics collection middleware
	app.Use(func(c *fiber.Ctx) error {
		metricsCollector.RecordRequest()
		start := time.Now()

		err := c.Next()

		duration := time.Since(start)
		if err != nil {
			metricsCollector.RecordError()
		}

		// Record upload metrics if this is an upload endpoint
		if c.Path() == "/api/upload" || c.Path() == "/api/upload/streaming" {
			contentLength := c.Request().Header.ContentLength()
			if contentLength > 0 {
				metricsCollector.RecordUpload(int64(contentLength), duration)
			}
		}

		return err
	})

	// Add pprof middleware for performance debugging (only in development)
	if os.Getenv("ENV") == "development" {
		app.Use(pprof.New())
	}

	// Initialize handlers
	h := handlers.New(fileService, minioService, discordService, wsHub, cfg)

	// Routes
	api := app.Group("/api")
	{
		api.Get("/health", h.HealthCheck)
		api.Get("/version", h.GetVersion)
		api.Get("/status", h.GetStatus)
		api.Get("/dashboard", h.GetDashboard)
		api.Post("/upload", h.UploadFiles)
		api.Get("/files", h.ListFiles)

		// Direct upload routes (better for large files)
		api.Post("/upload/presigned", h.GetPresignedURL)
		api.Post("/upload/presigned-batch", h.GetPresignedURLsBatch)
		api.Post("/upload/complete", h.ProcessUploadedFile)
		api.Post("/check-duplicate", h.CheckDuplicate)

		// Streaming upload routes with bit-perfect quality
		api.Post("/upload/streaming", h.UploadStreamingFiles)

		// TUS protocol routes for resumable uploads
		tus := api.Group("/tus")
		{
			tus.Options("", h.GetTUSConfiguration)         // TUS discovery
			tus.Post("", h.CreateTUSUpload)                // Create upload
			tus.Head("/:id", h.GetTUSUploadInfo)           // Get upload info
			tus.Patch("/:id", h.UploadTUSChunk)            // Upload chunk
			tus.Post("/:id/complete", h.CompleteTUSUpload) // Complete upload
			tus.Delete("/:id", h.DeleteTUSUpload)          // Delete upload
		}

		// Statistics and monitoring endpoints
		api.Get("/stats/streaming", h.GetStreamingStats)
		api.Get("/stats/compression", h.GetCompressionStats)
		api.Post("/files/:filename/verify", h.VerifyFileIntegrity)

		// Performance monitoring endpoints
		api.Get("/metrics", func(c *fiber.Ctx) error {
			// metrics := metricsCollector.GetMetrics()  // Temporarily disabled
			metrics := map[string]interface{}{"disabled": "monitoring temporarily disabled"}
			return c.JSON(metrics)
		})
		api.Get("/health/detailed", func(c *fiber.Ctx) error {
			// health := healthChecker.CheckHealth()  // Temporarily disabled
			health := map[string]interface{}{"status": "ok", "disabled": "monitoring temporarily disabled"}
			return c.JSON(health)
		})
		api.Get("/stats/pools", func(c *fiber.Ctx) error {
			// pools := optimization.GetGlobalPools()  // Temporarily disabled
			// stats := pools.GetAllStats()  // Temporarily disabled
			stats := map[string]interface{}{"disabled": "optimization temporarily disabled"}
			return c.JSON(stats)
		})

		// Keep existing upload methods - chunking handled on frontend
		// Chunked uploads use the same presigned URL system

		// Test endpoints
		api.Post("/test/discord", h.TestDiscord)
		api.Get("/test/minio", h.TestMinIO)

		// Migration endpoint
		api.Post("/migrate/minio", h.MigrateMinIO)

		// Cleanup and maintenance
		api.Post("/cleanup/expired", h.CleanupExpiredUploads)

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

// configurePiRuntime applies Pi-specific runtime optimizations
func configurePiRuntime(cfg *config.Config) {
	// Set GOMAXPROCS for Pi optimization
	cpuCount := runtime.NumCPU()
	var maxProcs int

	switch cpuCount {
	case 1:
		maxProcs = 1
	case 2:
		maxProcs = 2
	case 4:
		// Pi 4/5 - Leave one core for system tasks to prevent thermal throttling
		maxProcs = 3
	default:
		maxProcs = int(float64(cpuCount) * 0.75) // Use 75% of cores
	}

	runtime.GOMAXPROCS(maxProcs)
	log.Printf("🔧 Pi optimization: Set GOMAXPROCS to %d (CPU cores: %d)", maxProcs, cpuCount)

	// Configure garbage collector for Pi memory constraints
	if cfg.GCTargetPercentage > 0 {
		debug.SetGCPercent(cfg.GCTargetPercentage)
		log.Printf("🔧 Pi optimization: Set GOGC to %d", cfg.GCTargetPercentage)
	}

	// Set memory limit for Pi (prevents OOM)
	if cfg.MaxMemoryLimitMB > 0 {
		memLimitBytes := cfg.MaxMemoryLimitMB * 1024 * 1024
		debug.SetMemoryLimit(memLimitBytes)
		log.Printf("🔧 Pi optimization: Set memory limit to %dMB", cfg.MaxMemoryLimitMB)
	}

	// Configure GC to be more aggressive on Pi to prevent memory pressure
	debug.SetGCPercent(50) // More frequent GC on memory-constrained Pi

	// Set soft memory limit to trigger GC earlier
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	targetHeap := cfg.MaxMemoryLimitMB * 1024 * 1024 * 70 / 100 // 70% of limit
	if targetHeap > 0 {
		debug.SetMemoryLimit(int64(targetHeap))
	}

	log.Printf("🔧 Pi optimization: Applied memory management settings")
}
