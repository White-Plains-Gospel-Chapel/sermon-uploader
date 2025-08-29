package config

import (
	"os"
	"strconv"
)

type Config struct {
	// MinIO Configuration
	MinIOEndpoint   string
	MinIOAccessKey  string
	MinIOSecretKey  string
	MinIOSecure     bool
	MinioBucket     string

	// Discord Configuration
	DiscordWebhookURL string

	// File Processing
	WAVSuffix      string
	AACSuffix      string
	BatchThreshold int

	// Server Configuration
	Port string
}

func New() *Config {
	secure, _ := strconv.ParseBool(getEnv("MINIO_SECURE", "false"))
	batchThreshold, _ := strconv.Atoi(getEnv("BATCH_THRESHOLD", "2"))

	return &Config{
		MinIOEndpoint:     getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:    getEnv("MINIO_ACCESS_KEY", "gaius"),
		MinIOSecretKey:    getEnv("MINIO_SECRET_KEY", "John 3:16"),
		MinIOSecure:       secure,
		MinioBucket:       getEnv("MINIO_BUCKET", "sermons"),
		DiscordWebhookURL: getEnv("DISCORD_WEBHOOK_URL", "https://discord.com/api/webhooks/1410698516891701400/Ve6k3d8sdd54kf0II1xFc7H6YkYLoWiPFDEe5NsHsmX4Qv6l4CNzD4rMmdlWPQxLnRPT"),
		WAVSuffix:         getEnv("WAV_SUFFIX", "_raw"),
		AACSuffix:         getEnv("AAC_SUFFIX", "_streamable"),
		BatchThreshold:    batchThreshold,
		Port:              getEnv("PORT", "8000"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}