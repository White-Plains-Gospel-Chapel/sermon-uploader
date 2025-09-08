package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
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

	// Initialize Discord live service for production logging
	discordLiveService := services.NewDiscordLiveService(cfg.DiscordWebhookURL)

	// Initialize production logger
	productionLogger, err := services.NewProductionLogger(&services.ProductionLoggerConfig{
		LogDir:            "./logs",
		DiscordWebhookURL: cfg.DiscordWebhookURL,
		MaxFileSize:       100 * 1024 * 1024, // 100MB
		RetentionDays:     7,
		AsyncLogging:      true,
		BufferSize:        1000,
		DiscordService:    discordLiveService,
	})
	if err != nil {
		log.Printf("Failed to initialize production logger: %v", err)
		productionLogger = nil
	}

	// Initialize services
	minioService := services.NewMinIOService(cfg)
	discordService := services.NewDiscordService(cfg.DiscordWebhookURL)
	wsHub := services.NewWebSocketHub()
	fileService := services.NewFileService(minioService, discordService, wsHub, cfg)
	
	// Initialize hash cache for ultra-fast duplicate detection
	hashCache := services.NewHashCache(minioService.GetClient(), "sermons")
	
	// Initialize Pi optimizations
	piOptimizer := optimization.NewPiOptimizer()
	
	// Initialize circuit breakers for resilience
	circuitBreakers := services.NewCircuitBreakerManager()
	
	// Initialize rate limiters for resource protection
	rateLimiter := services.NewRateLimiter()

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

	// Create Fiber app with optimized settings for large file uploads
	app := fiber.New(fiber.Config{
		BodyLimit:         10 * 1024 * 1024 * 1024, // 10GB limit
		StreamRequestBody: true,                     // Enable streaming
		ReadBufferSize:    16 * 1024 * 1024,        // 16MB read buffer (was 4KB)
		WriteBufferSize:   16 * 1024 * 1024,        // 16MB write buffer (was 4KB)
		ReadTimeout:       300 * time.Second,        // 5 min read timeout
		WriteTimeout:      300 * time.Second,        // 5 min write timeout
		IdleTimeout:       120 * time.Second,
		DisableKeepalive:  false,
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
	// CORS configuration for multi-domain setup
	app.Use(cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			allowedOrigins := []string{
				"https://wpgc.church",              // Public website
				"https://www.wpgc.church",          // Public website with www
				"https://admin.wpgc.church",        // Admin dashboard
				"https://api.wpgc.church",          // API domain (if needed)
				"http://localhost:3000",            // Local admin dashboard
				"http://localhost:3001",            // Local public website
				"http://localhost:8000",            // Local API backend
			
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					return true
				}
			}
			
			// Log rejected origins for debugging
			if origin != "" {
				log.Printf("CORS rejected origin: %s", origin)
			}
			return false
		},
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,HEAD,PATCH",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,Upload-Length,Upload-Offset,Upload-Metadata,Tus-Resumable,Upload-Checksum,X-Chunk-Index,X-Total-Chunks",
		ExposeHeaders:    "Upload-Offset,Upload-Length,Tus-Resumable,Tus-Version,Tus-Max-Size,Tus-Extension,Tus-Checksum-Algorithm,Location",
		AllowCredentials: false,
		MaxAge:          86400, // Cache preflight for 24 hours
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

	// Initialize handlers with all optimizations
	h := handlers.New(fileService, minioService, discordService, discordLiveService, wsHub, cfg, productionLogger, hashCache)
	
	// Store optimizations for use in handlers
	_ = piOptimizer // Will be used in handlers
	_ = circuitBreakers // Will be used for MinIO calls
	_ = rateLimiter // Will be used in middleware

	// API Routes - Organized by domain/purpose
	api := app.Group("/api")
	{
		// ============================================
		// Core Health & Status (All domains)
		// ============================================
		api.Get("/health", h.HealthCheck)
		api.Get("/version", h.GetVersion)
		api.Get("/status", h.GetStatus)
		
		// ============================================
		// Public API (wpgc.church)
		// ============================================
		public := api.Group("/public")
		{
			public.Get("/sermons", h.ListPublicSermons)        // Public sermon list
			public.Get("/sermons/:id", h.GetSermon)            // Single sermon details
			public.Get("/sermons/latest", h.GetLatestSermons)  // Latest sermons
			public.Get("/events", h.GetPublicEvents)           // Public events
			public.Get("/announcements", h.GetAnnouncements)   // Church announcements
		}
		
		// ============================================
		// Admin API (admin.wpgc.church)
		// ============================================
		admin := api.Group("/admin")
		{
			// Sermon Management
			admin.Get("/sermons", h.ListFiles)                 // All sermons with admin info
			admin.Get("/sermons/:id", h.GetSermonAdmin)        // Detailed sermon info
			admin.Put("/sermons/:id", h.UpdateSermon)          // Update sermon metadata
			admin.Delete("/sermons/:id", h.DeleteSermon)       // Delete sermon
			
			// Member Management
			admin.Get("/members", h.ListMembers)
			admin.Post("/members", h.CreateMember)
			admin.Put("/members/:id", h.UpdateMember)
			admin.Delete("/members/:id", h.DeleteMember)
			
			// Event Management
			admin.Get("/events", h.ListEvents)
			admin.Post("/events", h.CreateEvent)
			admin.Put("/events/:id", h.UpdateEvent)
			admin.Delete("/events/:id", h.DeleteEvent)
			
			// Media Management
			admin.Get("/media", h.ListMedia)
			admin.Post("/media/upload", h.UploadMedia)
			admin.Delete("/media/:id", h.DeleteMedia)
			
			// Dashboard Stats
			admin.Get("/stats", h.GetDashboardStats)
			admin.Get("/stats/uploads", h.GetUploadStats)
			admin.Get("/stats/usage", h.GetUsageStats)
		}
		
		// ============================================
		// Upload API (uploads.wpgc.church)
		// ============================================
		uploads := api.Group("/uploads")
		{
			// Duplicate detection
			uploads.Get("/check-hash/:hash", h.CheckHash)
			uploads.Get("/hash-stats", h.GetHashStats)
			uploads.Post("/check-files", h.CheckFilesByInfo)
			
			// Upload operations
			uploads.Post("/sermon", h.Upload)                  // Single sermon upload
			uploads.Post("/sermons/batch", h.UploadBatch)      // Batch sermon upload
			uploads.Post("/media", h.UploadMedia)              // Media file upload
			
			// Upload management
			uploads.Get("/status/:uploadId", h.GetUploadStatus)
			uploads.Delete("/cancel/:uploadId", h.CancelUpload)
		}
		
		// ============================================
		// Maintenance & Testing (Admin only)
		// ============================================
		maintenance := api.Group("/maintenance")
		{
			maintenance.Get("/metrics", func(c *fiber.Ctx) error {
				metrics := map[string]interface{}{"disabled": "monitoring temporarily disabled"}
				return c.JSON(metrics)
			})
			maintenance.Get("/health/detailed", func(c *fiber.Ctx) error {
				health := map[string]interface{}{"status": "ok", "disabled": "monitoring temporarily disabled"}
				return c.JSON(health)
			})
			maintenance.Post("/cleanup/expired", h.CleanupExpiredUploads)
			maintenance.Delete("/bucket/clear", h.ClearBucket)
			maintenance.Post("/migrate/minio", h.MigrateMinIO)
		}
		
		// ============================================
		// Webhook endpoints
		// ============================================
		webhooks := api.Group("/webhooks")
		{
			webhooks.Post("/github", h.GitHubWebhook)
			webhooks.Post("/discord", h.TestDiscord)
		}
		
		// ============================================
		// Legacy routes (for backward compatibility)
		// ============================================
		api.Post("/upload", h.Upload)         // Redirect to /api/uploads/sermon
		api.Get("/files", h.ListFiles)        // Redirect to /api/admin/sermons
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

	// Serve the HTML upload page as the main interface
	// Use different paths for development vs production
	frontendPath := "../frontend"
	if os.Getenv("ENV") == "production" {
		frontendPath = "./frontend"
	}
	
	app.Static("/", frontendPath, fiber.Static{
		Index: "upload.html",
	})
	
	// Serve other static files if needed
	app.Static("/assets", frontendPath+"/assets")

	// Send startup notification using live update system
	go func() {
		if err := discordService.StartDeploymentNotification(); err != nil {
			log.Printf("Failed to start deployment notification: %v", err)
		} else {
			// Update to show service is starting
			time.Sleep(1 * time.Second)
			if err := discordService.UpdateDeploymentStatus("started", config.GetFullVersion("backend"), "", true); err != nil {
				log.Printf("Failed to update startup status: %v", err)
			}
		}
	}()

	// Start server with graceful shutdown
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	// Log server startup with Eastern Time
	easternTime := time.Now().In(easternTZ)
	timestamp := easternTime.Format("2006/01/02 15:04:05 MST")
	log.Printf("[%s] 🚀 Server starting on port %s", timestamp, port)
	log.Printf("[%s] 🌐 Access at http://your-pi-ip:%s", timestamp, port)
	
	// Graceful shutdown handling
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		
		log.Println("🛑 Shutting down gracefully...")
		
		// Give ongoing requests 30 seconds to complete
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		// Shutdown app
		if err := app.ShutdownWithContext(ctx); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
		
		log.Println("✅ Graceful shutdown complete")
		os.Exit(0)
	}()

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
