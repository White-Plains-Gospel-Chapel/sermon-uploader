package logging

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// LoadConfigFromEnv loads logging configuration from environment variables
func LoadConfigFromEnv() *Config {
	config := DefaultConfig()

	// Log level
	if levelStr := os.Getenv("LOG_LEVEL"); levelStr != "" {
		switch strings.ToLower(levelStr) {
		case "debug":
			config.Level = slog.LevelDebug
		case "info":
			config.Level = slog.LevelInfo
		case "warn", "warning":
			config.Level = slog.LevelWarn
		case "error":
			config.Level = slog.LevelError
		}
	}

	// Output format
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		if format == "text" || format == "json" {
			config.OutputFormat = format
		}
	}

	// Add source
	if addSource := os.Getenv("LOG_ADD_SOURCE"); addSource != "" {
		config.AddSource = parseBool(addSource)
	}

	// Sampling
	if enableSampling := os.Getenv("LOG_ENABLE_SAMPLING"); enableSampling != "" {
		config.EnableSampling = parseBool(enableSampling)
	}

	if sampleRate := os.Getenv("LOG_SAMPLE_RATE"); sampleRate != "" {
		if rate, err := strconv.ParseFloat(sampleRate, 64); err == nil && rate > 0 && rate <= 1 {
			config.SampleRate = rate
		}
	}

	// Metrics
	if enableMetrics := os.Getenv("LOG_ENABLE_METRICS"); enableMetrics != "" {
		config.EnableMetrics = parseBool(enableMetrics)
	}

	return config
}

// parseBool parses a boolean string with common variations
func parseBool(s string) bool {
	switch strings.ToLower(s) {
	case "true", "1", "yes", "on", "enabled":
		return true
	default:
		return false
	}
}

// DefaultProductionConfig returns production-ready configuration
func DefaultProductionConfig() *Config {
	return &Config{
		Level:          slog.LevelInfo,
		OutputFormat:   "json",
		AddSource:      false, // Usually disabled in production for performance
		EnableSampling: false,
		SampleRate:     1.0,
		EnableMetrics:  true,
		Output:         os.Stdout,
	}
}

// DefaultDevelopmentConfig returns development-friendly configuration
func DefaultDevelopmentConfig() *Config {
	return &Config{
		Level:          slog.LevelDebug,
		OutputFormat:   "text", // More readable for development
		AddSource:      true,   // Helpful for debugging
		EnableSampling: false,
		SampleRate:     1.0,
		EnableMetrics:  false,
		Output:         os.Stdout,
	}
}

// ConfigForEnvironment returns configuration based on environment
func ConfigForEnvironment(env string) *Config {
	switch strings.ToLower(env) {
	case "production", "prod":
		return DefaultProductionConfig()
	case "development", "dev":
		return DefaultDevelopmentConfig()
	default:
		// Load from environment variables with defaults
		return LoadConfigFromEnv()
	}
}
