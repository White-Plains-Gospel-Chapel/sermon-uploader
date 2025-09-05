package optimization

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// ErrorHandler provides centralized error handling with Pi optimizations
type ErrorHandler struct {
	maxRetries     int
	baseDelay      time.Duration
	maxDelay       time.Duration
	circuitBreaker *CircuitBreaker
	errorRecovery  *ErrorRecovery
	mu             sync.RWMutex
}

// NewErrorHandler creates a new error handler optimized for Pi
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{
		maxRetries:     3, // Conservative retry count for Pi
		baseDelay:      100 * time.Millisecond,
		maxDelay:       30 * time.Second,
		circuitBreaker: NewCircuitBreaker(),
		errorRecovery:  NewErrorRecovery(),
	}
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation func() error

// ExecuteWithRetry executes an operation with exponential backoff retry logic
func (eh *ErrorHandler) ExecuteWithRetry(ctx context.Context, operation RetryableOperation) error {
	return eh.circuitBreaker.Execute(func() error {
		var lastErr error

		for attempt := 0; attempt <= eh.maxRetries; attempt++ {
			// Check context cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Execute operation
			err := operation()
			if err == nil {
				return nil // Success
			}

			lastErr = err

			// Check if error is retryable
			if !eh.isRetryableError(err) {
				return fmt.Errorf("non-retryable error: %w", err)
			}

			// Don't retry on last attempt
			if attempt == eh.maxRetries {
				break
			}

			// Calculate delay with exponential backoff
			delay := eh.calculateDelay(attempt)

			// Wait with context cancellation check
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
				// Continue to next attempt
			}
		}

		return fmt.Errorf("operation failed after %d attempts: %w", eh.maxRetries+1, lastErr)
	})
}

// isRetryableError determines if an error should be retried
func (eh *ErrorHandler) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common retryable errors
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return true
	case errors.Is(err, context.Canceled):
		return false // Don't retry cancelled operations
	default:
		// Check error message patterns
		errMsg := err.Error()
		retryablePatterns := []string{
			"connection reset",
			"connection refused",
			"timeout",
			"temporary failure",
			"server unavailable",
			"service unavailable",
			"too many requests",
		}

		for _, pattern := range retryablePatterns {
			if contains(errMsg, pattern) {
				return true
			}
		}

		return false
	}
}

// calculateDelay calculates exponential backoff delay with jitter
func (eh *ErrorHandler) calculateDelay(attempt int) time.Duration {
	delay := time.Duration(1<<uint(attempt)) * eh.baseDelay
	if delay > eh.maxDelay {
		delay = eh.maxDelay
	}

	// Add jitter (±25% variation)
	jitter := time.Duration(float64(delay) * 0.25)
	delay += time.Duration(float64(jitter) * (2*rand() - 1))

	return delay
}

// CircuitBreaker implements circuit breaker pattern for Pi resilience
type CircuitBreaker struct {
	maxFailures  int
	resetTimeout time.Duration
	state        CircuitState
	failures     int
	lastFailTime time.Time
	mu           sync.RWMutex
}

// CircuitState represents the state of the circuit breaker
type CircuitState int

const (
	Closed CircuitState = iota
	Open
	HalfOpen
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  5,                // Conservative failure threshold for Pi
		resetTimeout: 60 * time.Second, // Reset after 1 minute
		state:        Closed,
	}
}

// Execute executes an operation through the circuit breaker
func (cb *CircuitBreaker) Execute(operation func() error) error {
	cb.mu.Lock()

	// Check if circuit should be reset
	if cb.state == Open && time.Since(cb.lastFailTime) > cb.resetTimeout {
		cb.state = HalfOpen
		cb.failures = 0
	}

	// Reject if circuit is open
	if cb.state == Open {
		cb.mu.Unlock()
		return errors.New("circuit breaker is open")
	}

	cb.mu.Unlock()

	// Execute operation
	err := operation()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailTime = time.Now()

		if cb.failures >= cb.maxFailures {
			cb.state = Open
		}

		return err
	}

	// Success - reset circuit
	cb.failures = 0
	cb.state = Closed
	return nil
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// ErrorRecovery handles panic recovery and resource cleanup
type ErrorRecovery struct {
	panicHandlers []PanicHandler
	mu            sync.RWMutex
}

// PanicHandler handles panic recovery
type PanicHandler func(recovered interface{}, stack []byte)

// NewErrorRecovery creates a new error recovery system
func NewErrorRecovery() *ErrorRecovery {
	er := &ErrorRecovery{
		panicHandlers: make([]PanicHandler, 0),
	}

	// Add default panic handler
	er.AddPanicHandler(func(recovered interface{}, stack []byte) {
		fmt.Printf("PANIC RECOVERED: %v\nStack trace:\n%s\n", recovered, string(stack))
	})

	return er
}

// AddPanicHandler adds a panic handler
func (er *ErrorRecovery) AddPanicHandler(handler PanicHandler) {
	er.mu.Lock()
	defer er.mu.Unlock()
	er.panicHandlers = append(er.panicHandlers, handler)
}

// RecoverAndLog recovers from panics and logs them
func (er *ErrorRecovery) RecoverAndLog() {
	if r := recover(); r != nil {
		// Get stack trace
		stack := make([]byte, 4096)
		n := runtime.Stack(stack, false)
		stack = stack[:n]

		// Call panic handlers
		er.mu.RLock()
		handlers := make([]PanicHandler, len(er.panicHandlers))
		copy(handlers, er.panicHandlers)
		er.mu.RUnlock()

		for _, handler := range handlers {
			handler(r, stack)
		}
	}
}

// SafeExecute safely executes a function with panic recovery
func (er *ErrorRecovery) SafeExecute(fn func()) (recovered interface{}) {
	defer func() {
		if r := recover(); r != nil {
			recovered = r
			// Get stack trace
			stack := make([]byte, 4096)
			n := runtime.Stack(stack, false)
			stack = stack[:n]

			// Call panic handlers
			er.mu.RLock()
			handlers := make([]PanicHandler, len(er.panicHandlers))
			copy(handlers, er.panicHandlers)
			er.mu.RUnlock()

			for _, handler := range handlers {
				handler(r, stack)
			}
		}
	}()

	fn()
	return nil
}

// ResourceManager manages resource cleanup and lifecycle
type ResourceManager struct {
	resources []Resource
	mu        sync.RWMutex
}

// Resource represents a managed resource
type Resource interface {
	Close() error
	String() string
}

// NewResourceManager creates a new resource manager
func NewResourceManager() *ResourceManager {
	return &ResourceManager{
		resources: make([]Resource, 0),
	}
}

// Register registers a resource for management
func (rm *ResourceManager) Register(resource Resource) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.resources = append(rm.resources, resource)
}

// CleanupAll cleans up all registered resources
func (rm *ResourceManager) CleanupAll() []error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	var errors []error

	// Cleanup in reverse order (LIFO)
	for i := len(rm.resources) - 1; i >= 0; i-- {
		if err := rm.resources[i].Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to cleanup %s: %w", rm.resources[i].String(), err))
		}
	}

	rm.resources = rm.resources[:0] // Clear the slice

	return errors
}

// GetResourceCount returns the number of registered resources
func (rm *ResourceManager) GetResourceCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return len(rm.resources)
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || (len(s) > len(substr) &&
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
				containsInner(s, substr))))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Simple random number generator (avoiding math/rand for simplicity)
func rand() float64 {
	return float64(time.Now().UnixNano()%1000) / 1000.0
}

// Global instances
var (
	globalErrorHandler    *ErrorHandler
	globalResourceManager *ResourceManager
	initOnce              sync.Once
)

// InitGlobalErrorHandling initializes global error handling
func InitGlobalErrorHandling() {
	initOnce.Do(func() {
		globalErrorHandler = NewErrorHandler()
		globalResourceManager = NewResourceManager()
	})
}

// GetErrorHandler returns the global error handler
func GetErrorHandler() *ErrorHandler {
	if globalErrorHandler == nil {
		InitGlobalErrorHandling()
	}
	return globalErrorHandler
}

// GetResourceManager returns the global resource manager
func GetResourceManager() *ResourceManager {
	if globalResourceManager == nil {
		InitGlobalErrorHandling()
	}
	return globalResourceManager
}
