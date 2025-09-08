package monitoring

import (
	"sync"
	"time"
)

type MetricsCollector struct {
	mu           sync.RWMutex
	requests     int64
	errors       int64
	uploads      int64
	uploadBytes  int64
	uploadTime   time.Duration
}

type HealthChecker struct {
	mu     sync.RWMutex
	checks map[string]func() error
}

var (
	globalMetrics *MetricsCollector
	globalHealth  *HealthChecker
	once          sync.Once
)

func InitGlobalMonitoring() {
	once.Do(func() {
		globalMetrics = &MetricsCollector{}
		globalHealth = &HealthChecker{
			checks: make(map[string]func() error),
		}
	})
}

func GetMetricsCollector() *MetricsCollector {
	if globalMetrics == nil {
		InitGlobalMonitoring()
	}
	return globalMetrics
}

func GetHealthChecker() *HealthChecker {
	if globalHealth == nil {
		InitGlobalMonitoring()
	}
	return globalHealth
}

func (m *MetricsCollector) RecordRequest() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests++
}

func (m *MetricsCollector) RecordError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors++
}

func (m *MetricsCollector) RecordUpload(bytes int64, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.uploads++
	m.uploadBytes += bytes
	m.uploadTime += duration
}

func (h *HealthChecker) RegisterCheck(name string, check func() error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = check
}

func (h *HealthChecker) RunChecks() map[string]error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	results := make(map[string]error)
	for name, check := range h.checks {
		results[name] = check()
	}
	return results
}

type PerformanceProfiler struct {
	mu sync.RWMutex
}

type Timer struct {
	start time.Time
	name  string
}

func GetProfiler() *PerformanceProfiler {
	return &PerformanceProfiler{}
}

func (p *PerformanceProfiler) RecordOperation(name string, duration time.Duration) {
	// Implementation for performance profiling
}

func (p *PerformanceProfiler) StartTimer(name string) *Timer {
	return &Timer{
		start: time.Now(),
		name:  name,
	}
}

func (t *Timer) Stop() time.Duration {
	return time.Since(t.start)
}