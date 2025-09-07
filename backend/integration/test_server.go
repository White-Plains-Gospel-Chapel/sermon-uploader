package integration

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"sermon-uploader/config"
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

	// Initialize configuration (needed for environment setup)
	_ = config.New()

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

	// Setup minimal routes for testing
	api := app.Group("/api")
	
	// Health check
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy"})
	})

	// Mock upload routes for testing
	upload := api.Group("/upload")
	
	// Mock presigned-batch endpoint
	upload.Post("/presigned-batch", func(c *fiber.Ctx) error {
		var req struct {
			Files []struct {
				Filename string `json:"filename"`
				FileSize int64  `json:"fileSize"`
			} `json:"files"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}
		
		// Return mock presigned URLs
		var urls []fiber.Map
		for _, file := range req.Files {
			urls = append(urls, fiber.Map{
				"filename": file.Filename,
				"url":      fmt.Sprintf("http://minio:9000/test-bucket/%s", file.Filename),
				"method":   "direct_minio",
			})
		}
		return c.JSON(urls)
	})
	
	// Mock single presigned endpoint
	upload.Post("/presigned", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"url":    "http://minio:9000/test-bucket/test.wav",
			"method": "direct_minio",
		})
	})
	
	// Mock upload endpoint
	upload.Post("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	})

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
	client := &fiber.Client{}
	for i := 0; i < 30; i++ {
		req := client.Get(serverURL + "/api/health")
		statusCode, _, errs := req.Bytes()
		if len(errs) == 0 && statusCode == 200 {
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