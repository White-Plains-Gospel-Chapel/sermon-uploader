package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"sermon-uploader/config"
)

// TestConfig holds configuration for integration tests
type TestConfig struct {
	MinIOEndpoint     string
	MinIOAccessKey    string
	MinIOSecretKey    string
	MinioBucket       string
	DiscordWebhookURL string
	RedisEndpoint     string
}

// TestEnvironment manages test containers and configuration
type TestEnvironment struct {
	Config         *TestConfig
	MinIOContainer testcontainers.Container
	RedisContainer testcontainers.Container
	WebhookContainer testcontainers.Container
	Cleanup        func()
}

// SetupTestEnvironment creates and starts test containers
func SetupTestEnvironment(ctx context.Context) (*TestEnvironment, error) {
	env := &TestEnvironment{}

	// Start MinIO container
	minioContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "minio/minio:latest",
			ExposedPorts: []string{"9000/tcp", "9001/tcp"},
			Env: map[string]string{
				"MINIO_ROOT_USER":     "testuser",
				"MINIO_ROOT_PASSWORD": "testpassword",
			},
			Cmd: []string{"server", "/data", "--console-address", ":9001"},
			WaitingFor: wait.ForHTTP("/minio/health/live").
				WithPort("9000/tcp").
				WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start MinIO container: %w", err)
	}

	minioHost, err := minioContainer.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get MinIO host: %w", err)
	}

	minioPort, err := minioContainer.MappedPort(ctx, "9000")
	if err != nil {
		return nil, fmt.Errorf("failed to get MinIO port: %w", err)
	}

	// Start Redis container
	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor: wait.ForLog("Ready to accept connections").
				WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		minioContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to start Redis container: %w", err)
	}

	redisHost, err := redisContainer.Host(ctx)
	if err != nil {
		minioContainer.Terminate(ctx)
		redisContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to get Redis host: %w", err)
	}

	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		minioContainer.Terminate(ctx)
		redisContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to get Redis port: %w", err)
	}

	// Start webhook mock container
	webhookContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "mendhak/http-https-echo:latest",
			ExposedPorts: []string{"8080/tcp"},
			Env: map[string]string{
				"HTTP_PORT":           "8080",
				"LOG_WITHOUT_NEWLINE": "true",
			},
			WaitingFor: wait.ForHTTP("/").
				WithPort("8080/tcp").
				WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		minioContainer.Terminate(ctx)
		redisContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to start webhook container: %w", err)
	}

	webhookHost, err := webhookContainer.Host(ctx)
	if err != nil {
		minioContainer.Terminate(ctx)
		redisContainer.Terminate(ctx)
		webhookContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to get webhook host: %w", err)
	}

	webhookPort, err := webhookContainer.MappedPort(ctx, "8080")
	if err != nil {
		minioContainer.Terminate(ctx)
		redisContainer.Terminate(ctx)
		webhookContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to get webhook port: %w", err)
	}

	env.MinIOContainer = minioContainer
	env.RedisContainer = redisContainer
	env.WebhookContainer = webhookContainer

	env.Config = &TestConfig{
		MinIOEndpoint:     fmt.Sprintf("%s:%s", minioHost, minioPort.Port()),
		MinIOAccessKey:    "testuser",
		MinIOSecretKey:    "testpassword",
		MinioBucket:       "test-sermons",
		DiscordWebhookURL: fmt.Sprintf("http://%s:%s/webhook", webhookHost, webhookPort.Port()),
		RedisEndpoint:     fmt.Sprintf("%s:%s", redisHost, redisPort.Port()),
	}

	env.Cleanup = func() {
		minioContainer.Terminate(ctx)
		redisContainer.Terminate(ctx)
		webhookContainer.Terminate(ctx)
	}

	// Wait for services to be fully ready
	if err := env.waitForServices(ctx); err != nil {
		env.Cleanup()
		return nil, fmt.Errorf("services not ready: %w", err)
	}

	return env, nil
}

// waitForServices ensures all services are fully operational
func (env *TestEnvironment) waitForServices(ctx context.Context) error {
	// Test MinIO connection
	minioClient, err := minio.New(env.Config.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(env.Config.MinIOAccessKey, env.Config.MinIOSecretKey, ""),
		Secure: false,
	})
	if err != nil {
		return fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Wait for MinIO to be ready
	for i := 0; i < 30; i++ {
		_, err := minioClient.ListBuckets(ctx)
		if err == nil {
			break
		}
		if i == 29 {
			return fmt.Errorf("MinIO not ready after 30 attempts: %w", err)
		}
		time.Sleep(1 * time.Second)
	}

	// Create test bucket
	err = minioClient.MakeBucket(ctx, env.Config.MinioBucket, minio.MakeBucketOptions{})
	if err != nil {
		return fmt.Errorf("failed to create test bucket: %w", err)
	}

	// Test webhook endpoint
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s", env.Config.DiscordWebhookURL))
	if err != nil {
		return fmt.Errorf("webhook endpoint not ready: %w", err)
	}
	resp.Body.Close()

	return nil
}

// GetAppConfig returns an app config configured for integration testing
func (env *TestEnvironment) GetAppConfig() *config.Config {
	return &config.Config{
		MinIOEndpoint:      env.Config.MinIOEndpoint,
		MinIOAccessKey:     env.Config.MinIOAccessKey,
		MinIOSecretKey:     env.Config.MinIOSecretKey,
		MinIOSecure:        false,
		MinioBucket:        env.Config.MinioBucket,
		DiscordWebhookURL:  env.Config.DiscordWebhookURL,
		WAVSuffix:          "_raw",
		AACSuffix:          "_streamable",
		BatchThreshold:     2,
		Port:               "8000",
		MaxConcurrentUploads: 2,
		TempDir:            "/tmp/sermon-uploads-test",
		ChunkSize:          1024 * 1024, // 1MB
		MaxUploadSize:      100 * 1024 * 1024, // 100MB
		
		// Pi-specific settings for testing
		PiOptimization:     false,
		MaxMemoryLimitMB:   512,
		ThermalThrottling:  false,
		ThermalThresholdC:  75.0,
		GCTargetPercentage: 100,
		MaxGoroutines:      50,
		
		// Buffer settings
		BufferPoolEnabled: true,
		SmallBufferSize:   4096,
		MediumBufferSize:  32768,
		LargeBufferSize:   262144,
		HugeBufferSize:    1048576,
		
		// I/O settings
		IOBufferSize:       32768,
		EnableZeroCopy:     true,
		StreamingThreshold: 1048576, // 1MB
		
		// Connection settings
		MaxIdleConns:    10,
		MaxConnsPerHost: 5,
		ConnTimeout:     30,
		KeepAlive:       30,
		
		// Large file settings
		LargeFileThresholdMB: 10, // Lower threshold for testing
	}
}

// SetupTestEnvFromEnv creates test environment using docker-compose if available
func SetupTestEnvFromEnv() (*TestConfig, error) {
	// Check if we're running in CI or have docker-compose setup
	minioEndpoint := os.Getenv("TEST_MINIO_ENDPOINT")
	if minioEndpoint == "" {
		minioEndpoint = "localhost:9001" // Default from docker-compose.integration-test.yml
	}

	discordWebhook := os.Getenv("TEST_DISCORD_WEBHOOK_URL")
	if discordWebhook == "" {
		discordWebhook = "http://localhost:8081/webhook" // Default from docker-compose
	}

	return &TestConfig{
		MinIOEndpoint:     minioEndpoint,
		MinIOAccessKey:    "testuser",
		MinIOSecretKey:    "testpassword",
		MinioBucket:       "test-sermons",
		DiscordWebhookURL: discordWebhook,
		RedisEndpoint:     "localhost:6380",
	}, nil
}

// CreateTestFile creates a test audio file for testing
func CreateTestFile(filename string, size int64) ([]byte, error) {
	// Create a simple WAV-like test file
	data := make([]byte, size)
	
	// WAV header simulation (minimal)
	if size >= 44 {
		copy(data[0:4], []byte("RIFF"))
		copy(data[8:12], []byte("WAVE"))
		copy(data[12:16], []byte("fmt "))
		copy(data[36:40], []byte("data"))
	}
	
	// Fill rest with test pattern
	for i := 44; i < len(data); i++ {
		data[i] = byte(i % 256)
	}
	
	return data, nil
}

// WaitForWebhookCall waits for a webhook call and returns the request data
func WaitForWebhookCall(webhookURL string, timeout time.Duration) ([]byte, error) {
	// This is a simplified version - in real implementation you might want
	// to use a more sophisticated webhook mock that can capture requests
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(webhookURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// For now just return empty - webhook mock implementation would capture actual requests
	return []byte{}, nil
}