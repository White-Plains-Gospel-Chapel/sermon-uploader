package monitoring

import (
	"sync"
	"time"
)

// OperationMetrics tracks metrics for a specific operation
type OperationMetrics struct {
	Count         int64         `json:"count"`
	TotalDuration time.Duration `json:"total_duration"`
	LastOperation time.Time     `json:"last_operation"`
}

// PerformanceProfiler provides performance profiling capabilities
type PerformanceProfiler struct {
	mu      sync.RWMutex
	metrics map[string]*OperationMetrics
}

// MetricsCollector collects various application metrics
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics map[string]interface{}
}

// HealthChecker provides health checking capabilities
type HealthChecker struct {
	checks map[string]func() error
}

var (
	globalProfiler  *PerformanceProfiler
	globalCollector *MetricsCollector
	globalHealth    *HealthChecker
	initOnce        sync.Once
)

// InitGlobalMonitoring initializes global monitoring instances
func InitGlobalMonitoring() {
	initOnce.Do(func() {
		globalProfiler = &PerformanceProfiler{
			metrics: make(map[string]*OperationMetrics),
		}
		globalCollector = &MetricsCollector{
			metrics: make(map[string]interface{}),
		}
		globalHealth = &HealthChecker{
			checks: make(map[string]func() error),
		}
	})
}

// GetProfiler returns the global performance profiler
func GetProfiler() *PerformanceProfiler {
	if globalProfiler == nil {
		InitGlobalMonitoring()
	}
	return globalProfiler
}

// GetMetricsCollector returns the global metrics collector
func GetMetricsCollector() *MetricsCollector {
	if globalCollector == nil {
		InitGlobalMonitoring()
	}
	return globalCollector
}

// GetHealthChecker returns the global health checker
func GetHealthChecker() *HealthChecker {
	if globalHealth == nil {
		InitGlobalMonitoring()
	}
	return globalHealth
}

// PerformanceProfiler methods

// Profile records a performance metric
func (p *PerformanceProfiler) Profile(name string, duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if metrics, exists := p.metrics[name]; exists {
		metrics.Count++
		metrics.TotalDuration += duration
		metrics.LastOperation = time.Now()
	} else {
		p.metrics[name] = &OperationMetrics{
			Count:         1,
			TotalDuration: duration,
			LastOperation: time.Now(),
		}
	}
}

// GetMetrics returns all profiling metrics
func (p *PerformanceProfiler) GetMetrics() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]interface{})
	for name, metrics := range p.metrics {
		avgDuration := float64(0)
		if metrics.Count > 0 {
			avgDuration = float64(metrics.TotalDuration.Milliseconds()) / float64(metrics.Count)
		}

		result[name] = map[string]interface{}{
			"count":            metrics.Count,
			"total_duration":   metrics.TotalDuration.Milliseconds(),
			"average_duration": avgDuration,
			"last_operation":   metrics.LastOperation.Unix(),
		}
	}
	return result
}

// StartTimer starts a performance timer
func (p *PerformanceProfiler) StartTimer(name string) func() {
	start := time.Now()
	return func() {
		p.Profile(name, time.Since(start))
	}
}

// MetricsCollector methods

// RecordMetric records a metric value
func (m *MetricsCollector) RecordMetric(name string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics[name] = value
}

// GetMetrics returns all collected metrics
func (m *MetricsCollector) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]interface{})
	for k, v := range m.metrics {
		result[k] = v
	}
	return result
}

// IncrementCounter increments a counter metric
func (m *MetricsCollector) IncrementCounter(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if val, exists := m.metrics[name]; exists {
		if count, ok := val.(int64); ok {
			m.metrics[name] = count + 1
		} else {
			m.metrics[name] = int64(1)
		}
	} else {
		m.metrics[name] = int64(1)
	}
}

// RecordRequest records an HTTP request
func (m *MetricsCollector) RecordRequest() {
	m.IncrementCounter("requests_total")
}

// RecordError records an HTTP error
func (m *MetricsCollector) RecordError() {
	m.IncrementCounter("errors_total")
}

// RecordUpload records an upload
func (m *MetricsCollector) RecordUpload(fileSize int64, duration time.Duration) {
	m.IncrementCounter("uploads_total")
	m.RecordMetric("total_bytes_uploaded", fileSize)
	m.RecordMetric("last_upload_duration", duration)
}

// HealthChecker methods

// RegisterCheck registers a health check function
func (h *HealthChecker) RegisterCheck(name string, check func() error) {
	h.checks[name] = check
}

// CheckHealth runs all health checks
func (h *HealthChecker) CheckHealth() map[string]interface{} {
	results := make(map[string]interface{})
	overall := true

	for name, check := range h.checks {
		err := check()
		if err != nil {
			results[name] = map[string]interface{}{
				"status": "failed",
				"error":  err.Error(),
			}
			overall = false
		} else {
			results[name] = map[string]interface{}{
				"status": "ok",
			}
		}
	}

	results["overall"] = overall
	results["timestamp"] = time.Now().Unix()

	return results
}
