package config

import (
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	// Save original environment
	originalVars := make(map[string]string)
	envVars := []string{
		"MINIO_ENDPOINT",
		"MINIO_ACCESS_KEY",
		"MINIO_SECRET_KEY",
		"MINIO_SECURE",
		"MINIO_BUCKET",
		"DISCORD_WEBHOOK_URL",
		"PI_OPTIMIZATION",
		"MAX_MEMORY_MB",
	}

	for _, env := range envVars {
		originalVars[env] = os.Getenv(env)
		os.Unsetenv(env)
	}

	// Restore environment after test
	defer func() {
		for env, val := range originalVars {
			if val != "" {
				os.Setenv(env, val)
			} else {
				os.Unsetenv(env)
			}
		}
	}()

	cfg := New()

	assert.NotNil(t, cfg)
	assert.Equal(t, "localhost:9000", cfg.MinIOEndpoint)
	assert.Equal(t, "gaius", cfg.MinIOAccessKey)
	assert.Equal(t, "John 3:16", cfg.MinIOSecretKey)
	assert.False(t, cfg.MinIOSecure)
	assert.Equal(t, "sermons", cfg.MinioBucket)
	assert.Equal(t, "_raw", cfg.WAVSuffix)
	assert.Equal(t, "_streamable", cfg.AACSuffix)
	assert.Equal(t, 2, cfg.BatchThreshold)
	assert.Equal(t, "8000", cfg.Port)
	assert.True(t, cfg.PiOptimization)
	assert.Equal(t, int64(800), cfg.MaxMemoryLimitMB)
}

func TestNewWithEnvironmentVariables(t *testing.T) {
	// Set test environment variables
	os.Setenv("MINIO_ENDPOINT", "test-endpoint:9001")
	os.Setenv("MINIO_ACCESS_KEY", "test-key")
	os.Setenv("MINIO_SECRET_KEY", "test-secret")
	os.Setenv("MINIO_SECURE", "true")
	os.Setenv("MINIO_BUCKET", "test-bucket")
	os.Setenv("DISCORD_WEBHOOK_URL", "https://test-webhook.com")
	os.Setenv("PI_OPTIMIZATION", "false")
	os.Setenv("MAX_MEMORY_MB", "1024")
	os.Setenv("BATCH_THRESHOLD", "5")
	os.Setenv("PORT", "9000")

	// Clean up after test
	defer func() {
		envVars := []string{
			"MINIO_ENDPOINT", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY",
			"MINIO_SECURE", "MINIO_BUCKET", "DISCORD_WEBHOOK_URL",
			"PI_OPTIMIZATION", "MAX_MEMORY_MB", "BATCH_THRESHOLD", "PORT",
		}
		for _, env := range envVars {
			os.Unsetenv(env)
		}
	}()

	cfg := New()

	assert.Equal(t, "test-endpoint:9001", cfg.MinIOEndpoint)
	assert.Equal(t, "test-key", cfg.MinIOAccessKey)
	assert.Equal(t, "test-secret", cfg.MinIOSecretKey)
	assert.True(t, cfg.MinIOSecure)
	assert.Equal(t, "test-bucket", cfg.MinioBucket)
	assert.Equal(t, "https://test-webhook.com", cfg.DiscordWebhookURL)
	assert.False(t, cfg.PiOptimization)
	assert.Equal(t, int64(1024), cfg.MaxMemoryLimitMB)
	assert.Equal(t, 5, cfg.BatchThreshold)
	assert.Equal(t, "9000", cfg.Port)
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "Environment variable exists",
			key:          "TEST_KEY",
			defaultValue: "default",
			envValue:     "env-value",
			expected:     "env-value",
		},
		{
			name:         "Environment variable does not exist",
			key:          "NONEXISTENT_KEY",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
		{
			name:         "Environment variable is empty",
			key:          "EMPTY_KEY",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up first
			os.Unsetenv(tt.key)

			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			result := getEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPiOptimizationDefaults(t *testing.T) {
	// Clear environment
	os.Unsetenv("PI_OPTIMIZATION")
	os.Unsetenv("MAX_MEMORY_MB")
	os.Unsetenv("THERMAL_THROTTLING")
	os.Unsetenv("GOGC")

	cfg := New()

	// Test Pi optimization defaults
	assert.True(t, cfg.PiOptimization)
	assert.Equal(t, int64(800), cfg.MaxMemoryLimitMB)
	assert.True(t, cfg.ThermalThrottling)
	assert.Equal(t, 75.0, cfg.ThermalThresholdC)
	assert.Equal(t, 100, cfg.GCTargetPercentage)
	assert.Equal(t, 100, cfg.MaxGoroutines)
}

func TestBufferConfiguration(t *testing.T) {
	// Clear environment
	os.Unsetenv("BUFFER_POOL_ENABLED")
	os.Unsetenv("SMALL_BUFFER_SIZE")
	os.Unsetenv("MEDIUM_BUFFER_SIZE")
	os.Unsetenv("LARGE_BUFFER_SIZE")
	os.Unsetenv("HUGE_BUFFER_SIZE")

	cfg := New()

	// Test buffer defaults
	assert.True(t, cfg.BufferPoolEnabled)
	assert.Equal(t, 4096, cfg.SmallBufferSize)   // 4KB
	assert.Equal(t, 32768, cfg.MediumBufferSize) // 32KB
	assert.Equal(t, 262144, cfg.LargeBufferSize) // 256KB
	assert.Equal(t, 1048576, cfg.HugeBufferSize) // 1MB
}

func TestIOConfiguration(t *testing.T) {
	// Clear environment
	os.Unsetenv("IO_BUFFER_SIZE")
	os.Unsetenv("ENABLE_ZERO_COPY")
	os.Unsetenv("STREAMING_THRESHOLD")

	cfg := New()

	// Test I/O defaults
	assert.Equal(t, 32768, cfg.IOBufferSize) // 32KB
	assert.True(t, cfg.EnableZeroCopy)
	assert.Equal(t, int64(1048576), cfg.StreamingThreshold) // 1MB
}

func TestConnectionConfiguration(t *testing.T) {
	// Clear environment
	os.Unsetenv("MAX_IDLE_CONNS")
	os.Unsetenv("MAX_CONNS_PER_HOST")
	os.Unsetenv("CONN_TIMEOUT")
	os.Unsetenv("KEEP_ALIVE")

	cfg := New()

	// Test connection defaults
	assert.Equal(t, 10, cfg.MaxIdleConns)
	assert.Equal(t, 5, cfg.MaxConnsPerHost)
	assert.Equal(t, 30, cfg.ConnTimeout)
	assert.Equal(t, 30, cfg.KeepAlive)
}

func TestConcurrentUploadsOptimization(t *testing.T) {
	// Clear environment
	os.Unsetenv("PI_OPTIMIZATION")
	os.Unsetenv("MAX_CONCURRENT_UPLOADS")

	// Test with Pi optimization enabled
	os.Setenv("PI_OPTIMIZATION", "true")
	defer os.Unsetenv("PI_OPTIMIZATION")

	cfg := New()

	// On systems with 4+ cores, should optimize concurrent uploads
	if runtime.NumCPU() >= 4 {
		assert.Equal(t, 3, cfg.MaxConcurrentUploads)
	} else {
		assert.Equal(t, 2, cfg.MaxConcurrentUploads)
	}
}

func TestPublicMinIOConfiguration(t *testing.T) {
	// Test public MinIO endpoint configuration
	os.Setenv("MINIO_PUBLIC_ENDPOINT", "minio.example.com")
	os.Setenv("MINIO_PUBLIC_SECURE", "true")
	defer func() {
		os.Unsetenv("MINIO_PUBLIC_ENDPOINT")
		os.Unsetenv("MINIO_PUBLIC_SECURE")
	}()

	cfg := New()

	assert.Equal(t, "minio.example.com", cfg.PublicMinIOEndpoint)
	assert.True(t, cfg.PublicMinIOSecure)
}

// BenchmarkNew benchmarks config creation
func BenchmarkNew(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = New()
	}
}

// BenchmarkGetEnv benchmarks environment variable lookup
func BenchmarkGetEnv(b *testing.B) {
	os.Setenv("BENCH_TEST_KEY", "test-value")
	defer os.Unsetenv("BENCH_TEST_KEY")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getEnv("BENCH_TEST_KEY", "default")
	}
}

// TestConfigValidation tests that all required fields are properly set
func TestConfigValidation(t *testing.T) {
	cfg := New()

	// Verify required fields are not empty
	assert.NotEmpty(t, cfg.MinIOEndpoint)
	assert.NotEmpty(t, cfg.MinIOAccessKey)
	assert.NotEmpty(t, cfg.MinIOSecretKey)
	assert.NotEmpty(t, cfg.MinioBucket)
	assert.NotEmpty(t, cfg.WAVSuffix)
	assert.NotEmpty(t, cfg.AACSuffix)
	assert.NotEmpty(t, cfg.Port)
	assert.NotEmpty(t, cfg.TempDir)

	// Verify numeric fields have reasonable values
	assert.Greater(t, cfg.BatchThreshold, 0)
	assert.Greater(t, cfg.MaxConcurrentUploads, 0)
	assert.Greater(t, cfg.ChunkSize, int64(0))
	assert.Greater(t, cfg.MaxUploadSize, int64(0))
	assert.Greater(t, cfg.MaxMemoryLimitMB, int64(0))
	assert.Greater(t, cfg.GCTargetPercentage, 0)
	assert.Greater(t, cfg.MaxGoroutines, 0)
}

// TestConfigConsistency tests that config creation is consistent
func TestConfigConsistency(t *testing.T) {
	cfg1 := New()
	cfg2 := New()

	// Should produce identical configurations when environment is the same
	assert.Equal(t, cfg1.MinIOEndpoint, cfg2.MinIOEndpoint)
	assert.Equal(t, cfg1.MinIOAccessKey, cfg2.MinIOAccessKey)
	assert.Equal(t, cfg1.MinIOSecretKey, cfg2.MinIOSecretKey)
	assert.Equal(t, cfg1.MinIOSecure, cfg2.MinIOSecure)
	assert.Equal(t, cfg1.MinioBucket, cfg2.MinioBucket)
	assert.Equal(t, cfg1.PiOptimization, cfg2.PiOptimization)
	assert.Equal(t, cfg1.MaxMemoryLimitMB, cfg2.MaxMemoryLimitMB)
}
