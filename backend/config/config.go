package config

import (
	"os"
	"runtime"
	"strconv"
)

type Config struct {
	// MinIO Configuration (internal)
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOSecure    bool
	MinioBucket    string

	// MinIO Public Endpoint for Presigned URLs (external)
	PublicMinIOEndpoint string // e.g., minio.wpgc.church
	PublicMinIOSecure   bool   // true => https

	// Discord Configuration
	DiscordWebhookURL string

	// File Processing
	WAVSuffix      string
	AACSuffix      string
	BatchThreshold int

	// Server Configuration
	Port string

	// Streaming and Concurrent Processing Configuration
	MaxConcurrentUploads int
	TempDir              string
	ChunkSize            int64
	MaxUploadSize        int64

	// Pi-Specific Performance Configuration
	PiOptimization     bool    // Enable Pi-specific optimizations
	MaxMemoryLimitMB   int64   // Memory limit in MB
	ThermalThrottling  bool    // Enable thermal throttling
	ThermalThresholdC  float64 // Temperature threshold in Celsius
	GCTargetPercentage int     // GOGC value for garbage collection
	MaxGoroutines      int     // Maximum goroutines

	// Buffer and Pool Configuration
	BufferPoolEnabled bool // Enable buffer pooling
	SmallBufferSize   int  // Size for small buffers (4KB)
	MediumBufferSize  int  // Size for medium buffers (32KB)
	LargeBufferSize   int  // Size for large buffers (256KB)
	HugeBufferSize    int  // Size for huge buffers (1MB)

	// I/O Optimization
	IOBufferSize       int   // I/O buffer size for file operations
	EnableZeroCopy     bool  // Enable zero-copy operations where possible
	StreamingThreshold int64 // File size threshold for streaming (bytes)

	// Connection Pooling
	MaxIdleConns    int // Maximum idle connections
	MaxConnsPerHost int // Maximum connections per host
	ConnTimeout     int // Connection timeout in seconds
	KeepAlive       int // Keep-alive timeout in seconds

	// Large File Upload Configuration
	LargeFileThresholdMB int64 // Files larger than this (MB) use direct MinIO URLs to bypass CloudFlare 100MB limit

	// Environment
	Environment string // development, staging, production
}

func New() *Config {
	secure, _ := strconv.ParseBool(getEnv("MINIO_SECURE", "false"))
	publicSecure, _ := strconv.ParseBool(getEnv("MINIO_PUBLIC_SECURE", "true"))
	batchThreshold, _ := strconv.Atoi(getEnv("BATCH_THRESHOLD", "2"))
	maxConcurrent, _ := strconv.Atoi(getEnv("MAX_CONCURRENT_UPLOADS", "2"))
	chunkSize, _ := strconv.ParseInt(getEnv("CHUNK_SIZE", "1048576"), 10, 64)             // 1MB default
	maxUploadSize, _ := strconv.ParseInt(getEnv("MAX_UPLOAD_SIZE", "2147483648"), 10, 64) // 2GB default

	// Pi-specific configuration
	piOptimization, _ := strconv.ParseBool(getEnv("PI_OPTIMIZATION", "true"))
	maxMemoryMB, _ := strconv.ParseInt(getEnv("MAX_MEMORY_MB", "800"), 10, 64) // 800MB for Pi
	thermalThrottling, _ := strconv.ParseBool(getEnv("THERMAL_THROTTLING", "true"))
	thermalThreshold, _ := strconv.ParseFloat(getEnv("THERMAL_THRESHOLD_C", "75.0"), 64)
	gcTargetPercentage, _ := strconv.Atoi(getEnv("GOGC", "100"))
	maxGoroutines, _ := strconv.Atoi(getEnv("MAX_GOROUTINES", "100"))

	// Buffer configuration
	bufferPoolEnabled, _ := strconv.ParseBool(getEnv("BUFFER_POOL_ENABLED", "true"))
	smallBufferSize, _ := strconv.Atoi(getEnv("SMALL_BUFFER_SIZE", "4096"))
	mediumBufferSize, _ := strconv.Atoi(getEnv("MEDIUM_BUFFER_SIZE", "32768"))
	largeBufferSize, _ := strconv.Atoi(getEnv("LARGE_BUFFER_SIZE", "262144"))
	hugeBufferSize, _ := strconv.Atoi(getEnv("HUGE_BUFFER_SIZE", "1048576"))

	// I/O configuration
	ioBufferSize, _ := strconv.Atoi(getEnv("IO_BUFFER_SIZE", "32768"))
	enableZeroCopy, _ := strconv.ParseBool(getEnv("ENABLE_ZERO_COPY", "true"))
	streamingThreshold, _ := strconv.ParseInt(getEnv("STREAMING_THRESHOLD", "1048576"), 10, 64) // 1MB

	// Connection configuration
	maxIdleConns, _ := strconv.Atoi(getEnv("MAX_IDLE_CONNS", "10"))
	maxConnsPerHost, _ := strconv.Atoi(getEnv("MAX_CONNS_PER_HOST", "5"))
	connTimeout, _ := strconv.Atoi(getEnv("CONN_TIMEOUT", "30"))
	keepAlive, _ := strconv.Atoi(getEnv("KEEP_ALIVE", "30"))

	// Large file configuration
	largeFileThresholdMB, _ := strconv.ParseInt(getEnv("LARGE_FILE_THRESHOLD_MB", "100"), 10, 64)

	// Auto-adjust concurrent uploads for Pi optimization
	if piOptimization {
		cpuCount := runtime.NumCPU()
		if maxConcurrent == 2 && cpuCount >= 4 {
			maxConcurrent = 3 // Use 3 workers on Pi 4/5 with 4 cores
		}
	}

	return &Config{
		MinIOEndpoint:        getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:       getEnv("MINIO_ACCESS_KEY", "gaius"),
		MinIOSecretKey:       getEnv("MINIO_SECRET_KEY", "John 3:16"),
		MinIOSecure:          secure,
		MinioBucket:          getEnv("MINIO_BUCKET", "sermons"),
		PublicMinIOEndpoint:  getEnv("MINIO_PUBLIC_ENDPOINT", ""),
		PublicMinIOSecure:    publicSecure,
		DiscordWebhookURL:    getEnv("DISCORD_WEBHOOK_URL", "https://discord.com/api/webhooks/1410698516891701400/Ve6k3d8sdd54kf0II1xFc7H6YkYLoWiPFDEe5NsHsmX4Qv6l4CNzD4rMmdlWPQxLnRPT"),
		WAVSuffix:            getEnv("WAV_SUFFIX", "_raw"),
		AACSuffix:            getEnv("AAC_SUFFIX", "_streamable"),
		BatchThreshold:       batchThreshold,
		Port:                 getEnv("PORT", "8000"),
		MaxConcurrentUploads: maxConcurrent,
		TempDir:              getEnv("TEMP_DIR", "/tmp/sermon-uploads"),
		ChunkSize:            chunkSize,
		MaxUploadSize:        maxUploadSize,

		// Pi-specific configuration
		PiOptimization:     piOptimization,
		MaxMemoryLimitMB:   maxMemoryMB,
		ThermalThrottling:  thermalThrottling,
		ThermalThresholdC:  thermalThreshold,
		GCTargetPercentage: gcTargetPercentage,
		MaxGoroutines:      maxGoroutines,

		// Buffer configuration
		BufferPoolEnabled: bufferPoolEnabled,
		SmallBufferSize:   smallBufferSize,
		MediumBufferSize:  mediumBufferSize,
		LargeBufferSize:   largeBufferSize,
		HugeBufferSize:    hugeBufferSize,

		// I/O configuration
		IOBufferSize:       ioBufferSize,
		EnableZeroCopy:     enableZeroCopy,
		StreamingThreshold: streamingThreshold,

		// Connection configuration
		MaxIdleConns:    maxIdleConns,
		MaxConnsPerHost: maxConnsPerHost,
		ConnTimeout:     connTimeout,
		KeepAlive:       keepAlive,

		// Large file configuration
		LargeFileThresholdMB: largeFileThresholdMB,

		// Environment
		Environment: getEnv("ENV", "production"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
