package integration_test

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"sermon-uploader/config"
	"sermon-uploader/handlers"
	"sermon-uploader/services"
)

// TestServer manages a test instance of the application
type TestServer struct {
	app    *fiber.App
	port   string
	url    string
	stopCh chan struct{}
}

// StartTestServer starts a test server for integration tests
func StartTestServer(t *testing.T) *TestServer {
	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Set test environment
	os.Setenv("MINIO_ENDPOINT", "localhost:9000")
	os.Setenv("MINIO_ACCESS_KEY", "minioadmin")
	os.Setenv("MINIO_SECRET_KEY", "minioadmin")
	os.Setenv("MINIO_BUCKET", "test-bucket")
	os.Setenv("MINIO_SECURE", "false")
	os.Setenv("PORT", fmt.Sprintf("%d", port))

	// Initialize configuration
	cfg := config.New()

	// Create Fiber app
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Setup CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "*",
		ExposeHeaders:    "*",
		AllowCredentials: false,
	}))

	// Initialize services
	minioService := services.NewMinIOService(cfg)
	discordService := services.NewDiscordService("")
	wsHub := services.NewWebSocketHub()
	fileService := services.NewFileService(minioService, discordService, wsHub, cfg)

	// Initialize handlers
	h := handlers.NewHandlers(minioService, fileService, discordService, wsHub, cfg)

	// Setup routes
	api := app.Group("/api")
	
	// Health check
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy"})
	})

	// Upload routes
	upload := api.Group("/upload")
	upload.Post("/presigned-batch", h.GetPresignedURLBatch)
	upload.Post("/presigned", h.GetPresignedURL)
	upload.Post("/", h.UploadFiles)

	// Start server in background
	stopCh := make(chan struct{})
	go func() {
		if err := app.Listen(fmt.Sprintf(":%d", port)); err != nil {
			select {
			case <-stopCh:
				// Server was stopped intentionally
			default:
				t.Errorf("Server failed to start: %v", err)
			}
		}
	}()

	// Wait for server to be ready
	serverURL := fmt.Sprintf("http://localhost:%d", port)
	for i := 0; i < 30; i++ {
		resp, err := (&fiber.Client{}).Get(serverURL + "/api/health")
		if err == nil && resp.StatusCode() == 200 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	return &TestServer{
		app:    app,
		port:   fmt.Sprintf("%d", port),
		url:    serverURL,
		stopCh: stopCh,
	}
}

// Stop stops the test server
func (ts *TestServer) Stop() {
	close(ts.stopCh)
	ts.app.Shutdown()
}

// URL returns the test server URL
func (ts *TestServer) URL() string {
	return ts.url
}