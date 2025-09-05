package logging

import (
	"fmt"
	"log/slog"
)

type ErrorCode string

const (
	ErrCodeMinIOConnection ErrorCode = "MINIO_CONNECTION_FAILED"
	ErrCodeUploadFailed    ErrorCode = "UPLOAD_FAILED"
	ErrCodeMetadataFailed  ErrorCode = "METADATA_FAILED"
	ErrCodeWebSocketError  ErrorCode = "WEBSOCKET_ERROR"
	ErrCodeDiscordFailed   ErrorCode = "DISCORD_NOTIFICATION_FAILED"
	ErrCodeValidation      ErrorCode = "VALIDATION_ERROR"
	ErrCodeInternal        ErrorCode = "INTERNAL_ERROR"
	ErrCodeTimeout         ErrorCode = "TIMEOUT_ERROR"
	ErrCodeNotFound        ErrorCode = "NOT_FOUND"
	ErrCodeUnauthorized    ErrorCode = "UNAUTHORIZED"
	ErrCodeRateLimit       ErrorCode = "RATE_LIMIT_EXCEEDED"
)

type SermonError struct {
	Code      ErrorCode              `json:"code"`
	Message   string                 `json:"message"`
	Operation string                 `json:"operation,omitempty"`
	Filename  string                 `json:"filename,omitempty"`
	Bucket    string                 `json:"bucket,omitempty"`
	Cause     error                  `json:"-"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Severity  string                 `json:"severity"`
}

// NewError creates a new SermonError with default severity "error"
func NewError(code ErrorCode, message string) *SermonError {
	return &SermonError{
		Code:     code,
		Message:  message,
		Severity: "error",
		Context:  make(map[string]interface{}),
	}
}

// NewWarning creates a SermonError with severity "warning"
func NewWarning(code ErrorCode, message string) *SermonError {
	return &SermonError{
		Code:     code,
		Message:  message,
		Severity: "warning",
		Context:  make(map[string]interface{}),
	}
}

// WithOperation adds operation context
func (e *SermonError) WithOperation(op string) *SermonError {
	e.Operation = op
	return e
}

// WithFile adds filename context
func (e *SermonError) WithFile(filename string) *SermonError {
	e.Filename = filename
	return e
}

// WithBucket adds bucket context
func (e *SermonError) WithBucket(bucket string) *SermonError {
	e.Bucket = bucket
	return e
}

// WithCause adds the underlying error
func (e *SermonError) WithCause(err error) *SermonError {
	e.Cause = err
	return e
}

// WithContext adds a key-value pair to the error context
func (e *SermonError) WithContext(key string, value interface{}) *SermonError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// Error implements the error interface
func (e *SermonError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// LogValue implements slog.LogValuer for structured logging
func (e *SermonError) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("error_code", string(e.Code)),
		slog.String("message", e.Message),
		slog.String("severity", e.Severity),
	}

	if e.Operation != "" {
		attrs = append(attrs, slog.String("operation", e.Operation))
	}

	if e.Filename != "" {
		attrs = append(attrs, slog.String("filename", e.Filename))
	}

	if e.Bucket != "" {
		attrs = append(attrs, slog.String("bucket", e.Bucket))
	}

	if e.Cause != nil {
		attrs = append(attrs, slog.String("cause", e.Cause.Error()))
	}

	// Add context fields
	if len(e.Context) > 0 {
		contextAttrs := make([]any, 0, len(e.Context)*2)
		for k, v := range e.Context {
			contextAttrs = append(contextAttrs, slog.Any(k, v))
		}
		attrs = append(attrs, slog.Group("context", contextAttrs...))
	}

	return slog.GroupValue(attrs...)
}

// IsRetryable returns true if the error is retryable
func (e *SermonError) IsRetryable() bool {
	switch e.Code {
	case ErrCodeTimeout, ErrCodeRateLimit:
		return true
	case ErrCodeMinIOConnection:
		// Connection errors are often transient
		return true
	default:
		return false
	}
}

// Common error constructors

// ErrMinIOConnection creates a MinIO connection error
func ErrMinIOConnection(message string, cause error) *SermonError {
	return NewError(ErrCodeMinIOConnection, message).
		WithCause(cause).
		WithOperation("minio_connect")
}

// ErrUpload creates an upload error
func ErrUpload(filename string, cause error) *SermonError {
	return NewError(ErrCodeUploadFailed, fmt.Sprintf("failed to upload %s", filename)).
		WithFile(filename).
		WithCause(cause).
		WithOperation("upload")
}

// ErrMetadata creates a metadata processing error
func ErrMetadata(filename string, cause error) *SermonError {
	return NewError(ErrCodeMetadataFailed, fmt.Sprintf("failed to process metadata for %s", filename)).
		WithFile(filename).
		WithCause(cause).
		WithOperation("metadata")
}

// ErrWebSocket creates a WebSocket error
func ErrWebSocket(message string, cause error) *SermonError {
	return NewError(ErrCodeWebSocketError, message).
		WithCause(cause).
		WithOperation("websocket")
}

// ErrDiscord creates a Discord notification error (as warning since it's non-blocking)
func ErrDiscord(message string, cause error) *SermonError {
	return NewWarning(ErrCodeDiscordFailed, message).
		WithCause(cause).
		WithOperation("discord_notify").
		WithContext("non_blocking", true)
}

// ErrValidation creates a validation error
func ErrValidation(field string, message string) *SermonError {
	return NewError(ErrCodeValidation, message).
		WithContext("field", field).
		WithOperation("validation")
}

// ErrInternal creates an internal error
func ErrInternal(message string, cause error) *SermonError {
	return NewError(ErrCodeInternal, message).
		WithCause(cause).
		WithOperation("internal")
}

// ErrTimeout creates a timeout error
func ErrTimeout(operation string, timeout interface{}) *SermonError {
	return NewError(ErrCodeTimeout, fmt.Sprintf("operation %s timed out", operation)).
		WithOperation(operation).
		WithContext("timeout", timeout)
}

// ErrNotFound creates a not found error
func ErrNotFound(resource string) *SermonError {
	return NewError(ErrCodeNotFound, fmt.Sprintf("%s not found", resource)).
		WithContext("resource", resource)
}

// ErrUnauthorized creates an unauthorized error
func ErrUnauthorized(message string) *SermonError {
	return NewError(ErrCodeUnauthorized, message).
		WithOperation("auth")
}

// ErrRateLimit creates a rate limit error
func ErrRateLimit(limit int, window string) *SermonError {
	return NewError(ErrCodeRateLimit, "rate limit exceeded").
		WithContext("limit", limit).
		WithContext("window", window)
}
