package services

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitState represents the state of the circuit breaker
type CircuitState int32

const (
	StateClosed CircuitState = iota // Normal operation
	StateOpen                        // Failing, reject requests
	StateHalfOpen                    // Testing if service recovered
)

// CircuitBreaker prevents cascading failures
type CircuitBreaker struct {
	name          string
	maxFailures   int32
	resetTimeout  time.Duration
	halfOpenMax   int32
	
	failures      atomic.Int32
	lastFailTime  atomic.Int64
	state         atomic.Int32
	halfOpenTests atomic.Int32
	
	successCount  atomic.Int64
	failureCount  atomic.Int64
	rejectedCount atomic.Int64
	
	mu sync.RWMutex
}

// NewCircuitBreaker creates a circuit breaker for protecting Pi resources
func NewCircuitBreaker(name string, maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:         name,
		maxFailures:  int32(maxFailures),
		resetTimeout: resetTimeout,
		halfOpenMax:  3, // Max tests in half-open state
	}
}

// Call executes function with circuit breaker protection
func (cb *CircuitBreaker) Call(ctx context.Context, fn func() error) error {
	if !cb.canAttempt() {
		cb.rejectedCount.Add(1)
		return fmt.Errorf("circuit breaker is open for %s", cb.name)
	}
	
	// Execute the function
	err := fn()
	
	if err != nil {
		cb.recordFailure()
		return err
	}
	
	cb.recordSuccess()
	return nil
}

// canAttempt checks if we can attempt the operation
func (cb *CircuitBreaker) canAttempt() bool {
	state := CircuitState(cb.state.Load())
	
	switch state {
	case StateClosed:
		return true
		
	case StateOpen:
		// Check if we should transition to half-open
		lastFail := cb.lastFailTime.Load()
		if time.Since(time.Unix(0, lastFail)) > cb.resetTimeout {
			if cb.state.CompareAndSwap(int32(StateOpen), int32(StateHalfOpen)) {
				cb.halfOpenTests.Store(0)
			}
			return true
		}
		return false
		
	case StateHalfOpen:
		// Allow limited tests in half-open state
		tests := cb.halfOpenTests.Add(1)
		return tests <= cb.halfOpenMax
		
	default:
		return false
	}
}

// recordSuccess records a successful operation
func (cb *CircuitBreaker) recordSuccess() {
	cb.successCount.Add(1)
	
	state := CircuitState(cb.state.Load())
	
	switch state {
	case StateHalfOpen:
		// Successful test in half-open, close the circuit
		if cb.state.CompareAndSwap(int32(StateHalfOpen), int32(StateClosed)) {
			cb.failures.Store(0)
		}
		
	case StateClosed:
		// Reset failure count on success
		cb.failures.Store(0)
	}
}

// recordFailure records a failed operation
func (cb *CircuitBreaker) recordFailure() {
	cb.failureCount.Add(1)
	failures := cb.failures.Add(1)
	cb.lastFailTime.Store(time.Now().UnixNano())
	
	state := CircuitState(cb.state.Load())
	
	switch state {
	case StateClosed:
		if failures >= cb.maxFailures {
			// Open the circuit
			cb.state.Store(int32(StateOpen))
		}
		
	case StateHalfOpen:
		// Failed test in half-open, reopen the circuit
		cb.state.Store(int32(StateOpen))
		cb.failures.Store(cb.maxFailures)
	}
}

// GetState returns current circuit state
func (cb *CircuitBreaker) GetState() string {
	state := CircuitState(cb.state.Load())
	switch state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"name":          cb.name,
		"state":         cb.GetState(),
		"failures":      cb.failures.Load(),
		"success_count": cb.successCount.Load(),
		"failure_count": cb.failureCount.Load(),
		"rejected_count": cb.rejectedCount.Load(),
	}
}

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

// NewCircuitBreakerManager creates a manager for multiple circuit breakers
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// GetBreaker gets or creates a circuit breaker
func (m *CircuitBreakerManager) GetBreaker(name string) *CircuitBreaker {
	m.mu.RLock()
	cb, exists := m.breakers[name]
	m.mu.RUnlock()
	
	if exists {
		return cb
	}
	
	// Create new breaker with defaults
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Check again in case someone else created it
	if cb, exists = m.breakers[name]; exists {
		return cb
	}
	
	// Default configuration for Pi
	cb = NewCircuitBreaker(name, 5, 30*time.Second)
	m.breakers[name] = cb
	
	return cb
}

// GetAllStats returns stats for all circuit breakers
func (m *CircuitBreakerManager) GetAllStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := make(map[string]interface{})
	for name, cb := range m.breakers {
		stats[name] = cb.GetStats()
	}
	
	return stats
}