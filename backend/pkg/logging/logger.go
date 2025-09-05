package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"
)

type contextKey string

const (
	ContextKeyCorrelationID     = contextKey("correlation_id")
	ContextKeyRequestID         = contextKey("request_id")
	ContextKeyUserID            = contextKey("user_id")
	ContextKeyOperationDuration = contextKey("operation_duration")
)

type SermonLogger struct {
	*slog.Logger
	config      *Config
	mu          sync.RWMutex
	serviceName string
	environment string
	timezone    *time.Location
	levelVar    *slog.LevelVar
}

type Config struct {
	Level          slog.Level
	OutputFormat   string // "json" or "text"
	AddSource      bool
	EnableSampling bool
	SampleRate     float64
	MaxMessageSize int
	EnableMetrics  bool
	Output         io.Writer // For testing, defaults to os.Stdout
}

func DefaultConfig() *Config {
	return &Config{
		Level:          slog.LevelInfo,
		OutputFormat:   "json",
		AddSource:      false,
		EnableSampling: false,
		SampleRate:     1.0,
		EnableMetrics:  false,
		Output:         os.Stdout,
	}
}

func New(serviceName string, cfg *Config) (*SermonLogger, error) {
	// Load Eastern timezone
	tz, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("failed to load timezone: %w", err)
	}

	// Set default output if not specified
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}

	// Create level var for dynamic level changes
	levelVar := &slog.LevelVar{}
	levelVar.Set(cfg.Level)

	// Create base handler options
	opts := &slog.HandlerOptions{
		Level:     levelVar,
		AddSource: cfg.AddSource,
	}

	// Create base handler based on format
	var handler slog.Handler
	if cfg.OutputFormat == "json" {
		handler = slog.NewJSONHandler(cfg.Output, opts)
	} else {
		handler = slog.NewTextHandler(cfg.Output, opts)
	}

	// Wrap with custom handlers
	handler = NewEasternTimeHandler(handler, tz)
	handler = NewContextualHandler(handler)

	if cfg.EnableSampling && cfg.SampleRate < 1.0 {
		handler = NewSamplingHandler(handler, cfg.SampleRate)
	}

	if cfg.EnableMetrics {
		handler = NewMetricsHandler(handler, serviceName)
	}

	// Get environment from env var
	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "development"
	}

	logger := slog.New(handler).With(
		slog.String("service", serviceName),
		slog.String("environment", environment),
		slog.Int("pid", os.Getpid()),
	)

	return &SermonLogger{
		Logger:      logger,
		config:      cfg,
		serviceName: serviceName,
		environment: environment,
		timezone:    tz,
		levelVar:    levelVar,
	}, nil
}

// SetLevel dynamically changes the log level
func (l *SermonLogger) SetLevel(level slog.Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.levelVar.Set(level)
	l.config.Level = level
}

// GetLevel returns the current log level
func (l *SermonLogger) GetLevel() slog.Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config.Level
}

// Operation-specific loggers
func (l *SermonLogger) ForUpload(filename string) *slog.Logger {
	return l.With(
		slog.String("operation", "upload"),
		slog.String("filename", filename),
	)
}

func (l *SermonLogger) ForWebSocket(clientID string) *slog.Logger {
	return l.With(
		slog.String("component", "websocket"),
		slog.String("client_id", clientID),
	)
}

func (l *SermonLogger) ForMinIO(bucket string) *slog.Logger {
	return l.With(
		slog.String("component", "minio"),
		slog.String("bucket", bucket),
	)
}

func (l *SermonLogger) ForDiscord() *slog.Logger {
	return l.With(
		slog.String("component", "discord"),
		slog.Bool("non_blocking", true),
	)
}

// WithOperation creates a logger with operation context
func (l *SermonLogger) WithOperation(operation string) *slog.Logger {
	return l.With(slog.String("operation", operation))
}

// WithUser creates a logger with user context
func (l *SermonLogger) WithUser(userID string) *slog.Logger {
	return l.With(slog.String("user_id", userID))
}

// LogRequest logs HTTP request details
func (l *SermonLogger) LogRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration) {
	level := slog.LevelInfo
	if statusCode >= 500 {
		level = slog.LevelError
	} else if statusCode >= 400 {
		level = slog.LevelWarn
	}

	l.LogAttrs(ctx, level, "http request",
		slog.String("method", method),
		slog.String("path", path),
		slog.Int("status_code", statusCode),
		slog.Duration("duration", duration),
		slog.String("type", "http_request"),
	)
}

// GetTimezone returns the logger's timezone
func (l *SermonLogger) GetTimezone() *time.Location {
	return l.timezone
}
