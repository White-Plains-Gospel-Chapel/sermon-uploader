package logging

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// FiberMiddleware creates a Fiber middleware for structured logging
func FiberMiddleware(logger *SermonLogger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Generate or extract correlation ID
		correlationID := c.Get("X-Correlation-ID")
		if correlationID == "" {
			correlationID = uuid.New().String()
		}

		// Generate or extract request ID
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Set headers in response
		c.Set("X-Correlation-ID", correlationID)
		c.Set("X-Request-ID", requestID)

		// Create context with values
		ctx := context.WithValue(c.Context(), ContextKeyCorrelationID, correlationID)
		ctx = context.WithValue(ctx, ContextKeyRequestID, requestID)

		// Extract user ID if present (from auth middleware)
		if userID := c.Locals("user_id"); userID != nil {
			if id, ok := userID.(string); ok {
				ctx = context.WithValue(ctx, ContextKeyUserID, id)
			}
		}

		// Create request-scoped logger
		reqLogger := logger.With(
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.String("ip", c.IP()),
			slog.String("correlation_id", correlationID),
			slog.String("request_id", requestID),
			slog.String("user_agent", c.Get("User-Agent")),
		)

		// Store in locals for handler access
		c.SetUserContext(ctx)
		c.Locals("logger", reqLogger)
		c.Locals("correlation_id", correlationID)
		c.Locals("request_id", requestID)

		// Log request start (debug level to reduce noise)
		reqLogger.DebugContext(ctx, "request started",
			slog.String("query", string(c.Request().URI().QueryString())),
		)

		// Process request
		err := c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Add duration to context for performance monitoring
		ctx = context.WithValue(ctx, ContextKeyOperationDuration, duration)

		// Determine log level based on status code
		status := c.Response().StatusCode()
		level := slog.LevelInfo
		if err != nil || status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		} else if duration > 5*time.Second {
			// Warn for slow requests
			level = slog.LevelWarn
		}

		// Build log attributes
		attrs := []slog.Attr{
			slog.Int("status", status),
			slog.Duration("duration", duration),
			slog.Int("bytes", len(c.Response().Body())),
			slog.String("type", "http_request"),
		}

		// Add error details if present
		if err != nil {
			// Check if it's our custom error type
			if sermonErr, ok := err.(*SermonError); ok {
				attrs = append(attrs, slog.Any("error", sermonErr))
			} else {
				attrs = append(attrs, slog.String("error", err.Error()))
			}
		}

		// Log the request completion
		reqLogger.LogAttrs(ctx, level, "request completed", attrs...)

		return err
	}
}

// GetLogger retrieves the request-scoped logger from Fiber context
func GetLogger(c *fiber.Ctx) *slog.Logger {
	if logger, ok := c.Locals("logger").(*slog.Logger); ok {
		return logger
	}
	// Fallback to default logger
	return slog.Default()
}

// GetCorrelationID retrieves the correlation ID from Fiber context
func GetCorrelationID(c *fiber.Ctx) string {
	if id, ok := c.Locals("correlation_id").(string); ok {
		return id
	}
	return ""
}

// GetRequestID retrieves the request ID from Fiber context
func GetRequestID(c *fiber.Ctx) string {
	if id, ok := c.Locals("request_id").(string); ok {
		return id
	}
	return ""
}

// WithContext adds context values to the logger
func WithContext(c *fiber.Ctx, logger *slog.Logger) *slog.Logger {
	correlationID := GetCorrelationID(c)
	requestID := GetRequestID(c)

	if correlationID != "" {
		logger = logger.With(slog.String("correlation_id", correlationID))
	}
	if requestID != "" {
		logger = logger.With(slog.String("request_id", requestID))
	}

	// Add user ID if present
	if userID := c.Locals("user_id"); userID != nil {
		if id, ok := userID.(string); ok {
			logger = logger.With(slog.String("user_id", id))
		}
	}

	return logger
}

// ErrorHandler is a Fiber error handler that uses structured logging
func ErrorHandler(logger *SermonLogger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		// Get request logger
		reqLogger := GetLogger(c)

		// Default to 500 Internal Server Error
		code := fiber.StatusInternalServerError
		message := "Internal Server Error"

		// Check if it's a Fiber error
		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
			message = e.Message
		}

		// Check if it's our custom error
		var sermonErr *SermonError
		if se, ok := err.(*SermonError); ok {
			sermonErr = se
			// Map error codes to HTTP status codes
			switch se.Code {
			case ErrCodeValidation:
				code = fiber.StatusBadRequest
			case ErrCodeNotFound:
				code = fiber.StatusNotFound
			case ErrCodeUnauthorized:
				code = fiber.StatusUnauthorized
			case ErrCodeRateLimit:
				code = fiber.StatusTooManyRequests
			case ErrCodeTimeout:
				code = fiber.StatusRequestTimeout
			default:
				code = fiber.StatusInternalServerError
			}
			message = se.Message
		}

		// Log the error
		if sermonErr != nil {
			reqLogger.ErrorContext(c.UserContext(), "request error",
				slog.Any("error", sermonErr),
				slog.Int("status", code),
			)
		} else {
			reqLogger.ErrorContext(c.UserContext(), "request error",
				slog.String("error", err.Error()),
				slog.Int("status", code),
			)
		}

		// Send error response
		return c.Status(code).JSON(fiber.Map{
			"error": fiber.Map{
				"code":    code,
				"message": message,
			},
			"correlation_id": GetCorrelationID(c),
			"request_id":     GetRequestID(c),
		})
	}
}

// RecoveryMiddleware recovers from panics and logs them
func RecoveryMiddleware(logger *SermonLogger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				// Get request logger
				reqLogger := GetLogger(c)

				// Log the panic
				reqLogger.ErrorContext(c.UserContext(), "panic recovered",
					slog.Any("panic", r),
					slog.String("path", c.Path()),
					slog.String("method", c.Method()),
				)

				// Return 500 error
				c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fiber.Map{
						"code":    fiber.StatusInternalServerError,
						"message": "Internal Server Error",
					},
					"correlation_id": GetCorrelationID(c),
				})
			}
		}()

		return c.Next()
	}
}
