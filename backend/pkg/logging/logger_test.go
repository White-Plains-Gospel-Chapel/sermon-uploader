package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSermonLogger(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		config      *Config
		wantErr     bool
	}{
		{
			name:        "create logger with default config",
			serviceName: "test-service",
			config:      DefaultConfig(),
			wantErr:     false,
		},
		{
			name:        "create logger with custom level",
			serviceName: "test-service",
			config: &Config{
				Level:        slog.LevelDebug,
				OutputFormat: "json",
				AddSource:    true,
			},
			wantErr: false,
		},
		{
			name:        "create logger with text format",
			serviceName: "test-service",
			config: &Config{
				Level:        slog.LevelInfo,
				OutputFormat: "text",
				AddSource:    false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := New(tt.serviceName, tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, logger)
			assert.Equal(t, tt.serviceName, logger.serviceName)
			assert.NotNil(t, logger.timezone)
			assert.Equal(t, "America/New_York", logger.timezone.String())
		})
	}
}

func TestSermonLoggerOutput(t *testing.T) {
	tests := []struct {
		name             string
		logFunc          func(*SermonLogger)
		expectedFields   []string
		unexpectedFields []string
	}{
		{
			name: "info log with service name",
			logFunc: func(l *SermonLogger) {
				l.Info("test message")
			},
			expectedFields: []string{
				`"msg":"test message"`,
				`"service":"test"`,
				`"level":"INFO"`,
			},
		},
		{
			name: "error log with additional fields",
			logFunc: func(l *SermonLogger) {
				l.Error("error occurred",
					slog.String("error_code", "TEST_ERROR"),
					slog.Int("retry_count", 3),
				)
			},
			expectedFields: []string{
				`"msg":"error occurred"`,
				`"error_code":"TEST_ERROR"`,
				`"retry_count":3`,
				`"level":"ERROR"`,
			},
		},
		{
			name: "debug log should not appear with info level",
			logFunc: func(l *SermonLogger) {
				l.Debug("debug message")
			},
			unexpectedFields: []string{
				`"msg":"debug message"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			config := &Config{
				Level:        slog.LevelInfo,
				OutputFormat: "json",
				AddSource:    false,
				Output:       &buf,
			}

			logger, err := New("test", config)
			require.NoError(t, err)

			tt.logFunc(logger)

			output := buf.String()

			for _, field := range tt.expectedFields {
				assert.Contains(t, output, field, "Expected field not found: %s", field)
			}

			for _, field := range tt.unexpectedFields {
				assert.NotContains(t, output, field, "Unexpected field found: %s", field)
			}

			if len(tt.expectedFields) > 0 {
				// Verify it's valid JSON
				var result map[string]interface{}
				err := json.Unmarshal([]byte(output), &result)
				assert.NoError(t, err, "Output should be valid JSON")
			}
		})
	}
}

func TestOperationSpecificLoggers(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:        slog.LevelInfo,
		OutputFormat: "json",
		Output:       &buf,
	}

	logger, err := New("test", config)
	require.NoError(t, err)

	t.Run("ForUpload adds upload context", func(t *testing.T) {
		buf.Reset()
		uploadLogger := logger.ForUpload("test.wav")
		uploadLogger.Info("uploading file")

		output := buf.String()
		assert.Contains(t, output, `"operation":"upload"`)
		assert.Contains(t, output, `"filename":"test.wav"`)
	})

	t.Run("ForWebSocket adds websocket context", func(t *testing.T) {
		buf.Reset()
		wsLogger := logger.ForWebSocket("client-123")
		wsLogger.Info("client connected")

		output := buf.String()
		assert.Contains(t, output, `"component":"websocket"`)
		assert.Contains(t, output, `"client_id":"client-123"`)
	})

	t.Run("ForMinIO adds minio context", func(t *testing.T) {
		buf.Reset()
		minioLogger := logger.ForMinIO("sermons")
		minioLogger.Info("bucket accessed")

		output := buf.String()
		assert.Contains(t, output, `"component":"minio"`)
		assert.Contains(t, output, `"bucket":"sermons"`)
	})

	t.Run("ForDiscord adds discord context", func(t *testing.T) {
		buf.Reset()
		discordLogger := logger.ForDiscord()
		discordLogger.Info("notification sent")

		output := buf.String()
		assert.Contains(t, output, `"component":"discord"`)
		assert.Contains(t, output, `"non_blocking":true`)
	})
}

func TestEasternTimeHandler(t *testing.T) {
	var buf bytes.Buffer

	// Create base handler
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	// Load Eastern timezone
	tz, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	// Wrap with Eastern time handler
	handler := NewEasternTimeHandler(baseHandler, tz)

	// Create logger with wrapped handler
	logger := slog.New(handler)

	// Log a message
	logger.Info("test message")

	// Parse output
	var result map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	// Verify time field exists
	assert.Contains(t, result, "time")

	// Parse the time and verify it's in Eastern timezone
	timeStr := result["time"].(string)
	parsedTime, err := time.Parse(time.RFC3339, timeStr)
	require.NoError(t, err)

	// The time should be parseable as Eastern time
	_, offset := parsedTime.Zone()
	easternTime := time.Now().In(tz)
	_, expectedOffset := easternTime.Zone()

	// Offsets should match (accounting for DST)
	assert.Equal(t, expectedOffset, offset, "Time should be in Eastern timezone")
}

func TestContextualHandler(t *testing.T) {
	var buf bytes.Buffer

	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	handler := NewContextualHandler(baseHandler)
	logger := slog.New(handler)

	t.Run("adds correlation ID from context", func(t *testing.T) {
		buf.Reset()
		ctx := context.WithValue(context.Background(), ContextKeyCorrelationID, "test-correlation-id")
		logger.InfoContext(ctx, "test message")

		output := buf.String()
		assert.Contains(t, output, `"correlation_id":"test-correlation-id"`)
	})

	t.Run("adds request ID from context", func(t *testing.T) {
		buf.Reset()
		ctx := context.WithValue(context.Background(), ContextKeyRequestID, "test-request-id")
		logger.InfoContext(ctx, "test message")

		output := buf.String()
		assert.Contains(t, output, `"request_id":"test-request-id"`)
	})

	t.Run("handles missing context values gracefully", func(t *testing.T) {
		buf.Reset()
		ctx := context.Background()
		logger.InfoContext(ctx, "test message")

		output := buf.String()
		assert.NotContains(t, output, "correlation_id")
		assert.NotContains(t, output, "request_id")
	})
}

func TestSermonError(t *testing.T) {
	t.Run("basic error creation", func(t *testing.T) {
		err := NewError(ErrCodeUploadFailed, "upload failed")
		assert.Equal(t, ErrCodeUploadFailed, err.Code)
		assert.Equal(t, "upload failed", err.Message)
		assert.Equal(t, "error", err.Severity)
	})

	t.Run("error with context", func(t *testing.T) {
		err := NewError(ErrCodeUploadFailed, "upload failed").
			WithOperation("upload").
			WithFile("test.wav").
			WithContext("size", 1024).
			WithContext("retry", 3)

		assert.Equal(t, "upload", err.Operation)
		assert.Equal(t, "test.wav", err.Filename)
		assert.Equal(t, 1024, err.Context["size"])
		assert.Equal(t, 3, err.Context["retry"])
	})

	t.Run("error with cause", func(t *testing.T) {
		cause := assert.AnError
		err := NewError(ErrCodeInternal, "internal error").WithCause(cause)

		assert.Equal(t, cause, err.Cause)
		assert.Contains(t, err.Error(), "caused by:")
	})

	t.Run("error LogValue", func(t *testing.T) {
		err := NewError(ErrCodeUploadFailed, "upload failed").
			WithOperation("upload").
			WithFile("test.wav").
			WithBucket("sermons")

		logValue := err.LogValue()

		// Convert to string for testing
		str := logValue.String()
		assert.Contains(t, str, "UPLOAD_FAILED")
		assert.Contains(t, str, "upload failed")
		assert.Contains(t, str, "upload")
		assert.Contains(t, str, "test.wav")
		assert.Contains(t, str, "sermons")
	})
}

func TestSamplingHandler(t *testing.T) {
	var buf bytes.Buffer

	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	// Create sampling handler with 50% sampling rate
	handler := NewSamplingHandler(baseHandler, 0.5)
	logger := slog.New(handler)

	// Log many messages
	messageCount := 1000
	for i := 0; i < messageCount; i++ {
		logger.Info("test message", slog.Int("index", i))
	}

	// Count logged messages
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	loggedCount := len(lines)

	// With 50% sampling, we expect roughly 500 messages (with some variance)
	expectedMin := 400
	expectedMax := 600

	assert.True(t, loggedCount >= expectedMin && loggedCount <= expectedMax,
		"Expected between %d and %d logs, got %d", expectedMin, expectedMax, loggedCount)

	// Verify each logged message has sampling metadata
	for _, line := range lines {
		if line != "" {
			assert.Contains(t, line, "sample_rate")
		}
	}
}

func TestPerformanceHandler(t *testing.T) {
	var buf bytes.Buffer

	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	// Set threshold to 100ms
	handler := NewPerformanceHandler(baseHandler, 100*time.Millisecond)
	logger := slog.New(handler)

	t.Run("adds warning for slow operations", func(t *testing.T) {
		buf.Reset()
		ctx := context.WithValue(context.Background(), ContextKeyOperationDuration, 200*time.Millisecond)
		logger.InfoContext(ctx, "operation completed")

		output := buf.String()
		assert.Contains(t, output, "performance_warning")
		assert.Contains(t, output, "threshold_exceeded_ms")
	})

	t.Run("no warning for fast operations", func(t *testing.T) {
		buf.Reset()
		ctx := context.WithValue(context.Background(), ContextKeyOperationDuration, 50*time.Millisecond)
		logger.InfoContext(ctx, "operation completed")

		output := buf.String()
		assert.NotContains(t, output, "performance_warning")
	})
}

func TestDynamicLogLevel(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:        slog.LevelInfo,
		OutputFormat: "json",
		Output:       &buf,
	}

	logger, err := New("test", config)
	require.NoError(t, err)

	t.Run("debug not logged at info level", func(t *testing.T) {
		buf.Reset()
		logger.Debug("debug message")
		assert.Empty(t, buf.String())
	})

	t.Run("info logged at info level", func(t *testing.T) {
		buf.Reset()
		logger.Info("info message")
		assert.NotEmpty(t, buf.String())
	})

	t.Run("change level to debug", func(t *testing.T) {
		logger.SetLevel(slog.LevelDebug)

		buf.Reset()
		logger.Debug("debug message after level change")
		assert.NotEmpty(t, buf.String())
		assert.Contains(t, buf.String(), "debug message after level change")
	})

	t.Run("change level to error", func(t *testing.T) {
		logger.SetLevel(slog.LevelError)

		buf.Reset()
		logger.Info("info message")
		assert.Empty(t, buf.String())

		logger.Error("error message")
		assert.NotEmpty(t, buf.String())
	})
}

func TestLoggerWithGroup(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:        slog.LevelInfo,
		OutputFormat: "json",
		Output:       &buf,
	}

	logger, err := New("test", config)
	require.NoError(t, err)

	// Create a grouped logger
	groupedLogger := logger.WithGroup("request")
	groupedLogger.Info("processing",
		slog.String("method", "GET"),
		slog.String("path", "/api/test"),
	)

	output := buf.String()

	// Parse JSON to verify structure
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	// Check for grouped fields
	assert.Contains(t, result, "request")
	requestGroup := result["request"].(map[string]interface{})
	assert.Equal(t, "GET", requestGroup["method"])
	assert.Equal(t, "/api/test", requestGroup["path"])
}

// Benchmark tests
func BenchmarkSermonLogger(b *testing.B) {
	config := &Config{
		Level:        slog.LevelInfo,
		OutputFormat: "json",
		Output:       bytes.NewBuffer(nil),
	}

	logger, _ := New("benchmark", config)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message",
				slog.String("key1", "value1"),
				slog.Int("key2", 123),
				slog.Bool("key3", true),
			)
		}
	})
}

func BenchmarkSermonLoggerWithContext(b *testing.B) {
	config := &Config{
		Level:        slog.LevelInfo,
		OutputFormat: "json",
		Output:       bytes.NewBuffer(nil),
	}

	logger, _ := New("benchmark", config)
	ctx := context.WithValue(context.Background(), ContextKeyCorrelationID, "bench-correlation-id")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.InfoContext(ctx, "benchmark message",
				slog.String("key1", "value1"),
				slog.Int("key2", 123),
				slog.Bool("key3", true),
			)
		}
	})
}
