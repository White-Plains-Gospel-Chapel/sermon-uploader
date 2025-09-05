package monitoring

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector collects and tracks performance metrics
type MetricsCollector struct {
	// System metrics
	cpuUsage    float64
	memoryUsage int64
	goroutines  int

	// Application metrics
	requestCount   int64
	errorCount     int64
	uploadCount    int64
	uploadBytes    int64
	uploadDuration int64 // microseconds

	// Pi-specific metrics
	temperature      float64
	thermalThrottled bool

	// Timing metrics
	startTime time.Time

	mu sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	mc := &MetricsCollector{
		startTime: time.Now(),
	}

	// Start background collection
	go mc.collectLoop()

	return mc
}

// collectLoop runs the metrics collection loop
func (mc *MetricsCollector) collectLoop() {
	ticker := time.NewTicker(1 * time.Second) // Collect every second
	defer ticker.Stop()

	for range ticker.C {
		mc.collectSystemMetrics()
		mc.collectPiMetrics()
	}
}

// collectSystemMetrics collects basic system metrics
func (mc *MetricsCollector) collectSystemMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.memoryUsage = int64(m.Alloc)
	mc.goroutines = runtime.NumGoroutine()
}

// collectPiMetrics collects Pi-specific metrics
func (mc *MetricsCollector) collectPiMetrics() {
	// Pi temperature monitoring (read from /sys/class/thermal/thermal_zone0/temp)
	temp := mc.readPiTemperature()

	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.temperature = temp
	mc.thermalThrottled = temp > 80.0 // Pi throttles around 80°C
}

// readPiTemperature reads Pi CPU temperature
func (mc *MetricsCollector) readPiTemperature() float64 {
	// Simplified - in production, read from /sys/class/thermal/thermal_zone0/temp
	// For now, return a simulated value
	return 65.0
}

// RecordRequest records a request metric
func (mc *MetricsCollector) RecordRequest() {
	atomic.AddInt64(&mc.requestCount, 1)
}

// RecordError records an error metric
func (mc *MetricsCollector) RecordError() {
	atomic.AddInt64(&mc.errorCount, 1)
}

// RecordUpload records an upload metric
func (mc *MetricsCollector) RecordUpload(bytes int64, duration time.Duration) {
	atomic.AddInt64(&mc.uploadCount, 1)
	atomic.AddInt64(&mc.uploadBytes, bytes)
	atomic.AddInt64(&mc.uploadDuration, duration.Microseconds())
}

// GetMetrics returns current metrics snapshot
func (mc *MetricsCollector) GetMetrics() Metrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	requestCount := atomic.LoadInt64(&mc.requestCount)
	errorCount := atomic.LoadInt64(&mc.errorCount)
	uploadCount := atomic.LoadInt64(&mc.uploadCount)
	uploadBytes := atomic.LoadInt64(&mc.uploadBytes)
	uploadDuration := atomic.LoadInt64(&mc.uploadDuration)

	// Calculate rates
	uptime := time.Since(mc.startTime).Seconds()
	requestRate := float64(requestCount) / uptime
	errorRate := float64(errorCount) / float64(max(requestCount, 1)) * 100

	var avgUploadSpeed float64
	if uploadCount > 0 {
		avgDurationSec := float64(uploadDuration) / float64(uploadCount) / 1000000                  // Convert microseconds to seconds
		avgUploadSpeed = float64(uploadBytes) / float64(uploadCount) / avgDurationSec / 1024 / 1024 // MB/s
	}

	return Metrics{
		Timestamp: time.Now(),
		Uptime:    time.Since(mc.startTime),

		// System metrics
		CPUUsage:    mc.cpuUsage,
		MemoryUsage: mc.memoryUsage,
		Goroutines:  mc.goroutines,

		// Pi metrics
		Temperature:      mc.temperature,
		ThermalThrottled: mc.thermalThrottled,

		// Application metrics
		RequestCount: requestCount,
		ErrorCount:   errorCount,
		UploadCount:  uploadCount,
		UploadBytes:  uploadBytes,

		// Calculated metrics
		RequestRate:    requestRate,
		ErrorRate:      errorRate,
		AvgUploadSpeed: avgUploadSpeed,
	}
}

// Metrics represents a snapshot of system metrics
type Metrics struct {
	Timestamp time.Time     `json:"timestamp"`
	Uptime    time.Duration `json:"uptime"`

	// System metrics
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage int64   `json:"memory_usage_bytes"`
	Goroutines  int     `json:"goroutines"`

	// Pi-specific metrics
	Temperature      float64 `json:"temperature_celsius"`
	ThermalThrottled bool    `json:"thermal_throttled"`

	// Application metrics
	RequestCount int64 `json:"request_count"`
	ErrorCount   int64 `json:"error_count"`
	UploadCount  int64 `json:"upload_count"`
	UploadBytes  int64 `json:"upload_bytes"`

	// Calculated metrics
	RequestRate    float64 `json:"request_rate_per_sec"`
	ErrorRate      float64 `json:"error_rate_percent"`
	AvgUploadSpeed float64 `json:"avg_upload_speed_mbps"`
}

// PerformanceProfiler provides performance profiling capabilities
type PerformanceProfiler struct {
	enabled    bool
	samples    []ProfileSample
	maxSamples int
	mu         sync.RWMutex
}

// ProfileSample represents a performance profile sample
type ProfileSample struct {
	Timestamp    time.Time     `json:"timestamp"`
	Operation    string        `json:"operation"`
	Duration     time.Duration `json:"duration"`
	MemoryBefore int64         `json:"memory_before"`
	MemoryAfter  int64         `json:"memory_after"`
	Success      bool          `json:"success"`
	Error        string        `json:"error,omitempty"`
}

// NewPerformanceProfiler creates a new performance profiler
func NewPerformanceProfiler(maxSamples int) *PerformanceProfiler {
	return &PerformanceProfiler{
		enabled:    true,
		maxSamples: maxSamples,
		samples:    make([]ProfileSample, 0, maxSamples),
	}
}

// ProfileOperation profiles an operation
func (pp *PerformanceProfiler) ProfileOperation(operation string, fn func() error) error {
	if !pp.enabled {
		return fn()
	}

	// Get initial memory stats
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	startTime := time.Now()
	err := fn()
	duration := time.Since(startTime)

	// Get final memory stats
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	// Create sample
	sample := ProfileSample{
		Timestamp:    startTime,
		Operation:    operation,
		Duration:     duration,
		MemoryBefore: int64(memBefore.Alloc),
		MemoryAfter:  int64(memAfter.Alloc),
		Success:      err == nil,
	}

	if err != nil {
		sample.Error = err.Error()
	}

	// Add sample
	pp.addSample(sample)

	return err
}

// addSample adds a sample to the profiler
func (pp *PerformanceProfiler) addSample(sample ProfileSample) {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	// Add sample
	pp.samples = append(pp.samples, sample)

	// Trim if necessary
	if len(pp.samples) > pp.maxSamples {
		pp.samples = pp.samples[1:]
	}
}

// GetSamples returns recent performance samples
func (pp *PerformanceProfiler) GetSamples() []ProfileSample {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	// Return copy
	samples := make([]ProfileSample, len(pp.samples))
	copy(samples, pp.samples)
	return samples
}

// Enable enables the profiler
func (pp *PerformanceProfiler) Enable() {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	pp.enabled = true
}

// Disable disables the profiler
func (pp *PerformanceProfiler) Disable() {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	pp.enabled = false
}

// Clear clears all samples
func (pp *PerformanceProfiler) Clear() {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	pp.samples = pp.samples[:0]
}

// HealthChecker monitors system health
type HealthChecker struct {
	metricsCollector *MetricsCollector
	thresholds       HealthThresholds
	mu               sync.RWMutex
}

// HealthThresholds defines health check thresholds
type HealthThresholds struct {
	MaxMemoryMB    int64   `json:"max_memory_mb"`
	MaxTemperature float64 `json:"max_temperature_celsius"`
	MaxErrorRate   float64 `json:"max_error_rate_percent"`
	MaxGoroutines  int     `json:"max_goroutines"`
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(collector *MetricsCollector) *HealthChecker {
	return &HealthChecker{
		metricsCollector: collector,
		thresholds: HealthThresholds{
			MaxMemoryMB:    800,  // 800MB for Pi
			MaxTemperature: 75.0, // 75°C for Pi
			MaxErrorRate:   5.0,  // 5% error rate
			MaxGoroutines:  100,  // 100 goroutines
		},
	}
}

// CheckHealth performs a comprehensive health check
func (hc *HealthChecker) CheckHealth() HealthStatus {
	metrics := hc.metricsCollector.GetMetrics()

	hc.mu.RLock()
	defer hc.mu.RUnlock()

	status := HealthStatus{
		Timestamp: time.Now(),
		Healthy:   true,
		Issues:    make([]string, 0),
		Metrics:   metrics,
	}

	// Check memory usage
	memoryMB := metrics.MemoryUsage / 1024 / 1024
	if memoryMB > hc.thresholds.MaxMemoryMB {
		status.Healthy = false
		status.Issues = append(status.Issues, "High memory usage")
	}

	// Check temperature
	if metrics.Temperature > hc.thresholds.MaxTemperature {
		status.Healthy = false
		status.Issues = append(status.Issues, "High temperature")
	}

	// Check error rate
	if metrics.ErrorRate > hc.thresholds.MaxErrorRate {
		status.Healthy = false
		status.Issues = append(status.Issues, "High error rate")
	}

	// Check goroutine count
	if metrics.Goroutines > hc.thresholds.MaxGoroutines {
		status.Healthy = false
		status.Issues = append(status.Issues, "High goroutine count")
	}

	return status
}

// HealthStatus represents the current health status
type HealthStatus struct {
	Timestamp time.Time `json:"timestamp"`
	Healthy   bool      `json:"healthy"`
	Issues    []string  `json:"issues,omitempty"`
	Metrics   Metrics   `json:"metrics"`
}

// Global metrics collector instance
var globalMetricsCollector *MetricsCollector
var globalProfiler *PerformanceProfiler
var globalHealthChecker *HealthChecker
var initOnce sync.Once

// InitGlobalMonitoring initializes global monitoring
func InitGlobalMonitoring() {
	initOnce.Do(func() {
		globalMetricsCollector = NewMetricsCollector()
		globalProfiler = NewPerformanceProfiler(1000) // Keep 1000 samples
		globalHealthChecker = NewHealthChecker(globalMetricsCollector)
	})
}

// GetMetricsCollector returns the global metrics collector
func GetMetricsCollector() *MetricsCollector {
	if globalMetricsCollector == nil {
		InitGlobalMonitoring()
	}
	return globalMetricsCollector
}

// GetProfiler returns the global profiler
func GetProfiler() *PerformanceProfiler {
	if globalProfiler == nil {
		InitGlobalMonitoring()
	}
	return globalProfiler
}

// GetHealthChecker returns the global health checker
func GetHealthChecker() *HealthChecker {
	if globalHealthChecker == nil {
		InitGlobalMonitoring()
	}
	return globalHealthChecker
}

// Helper function to get max of two int64 values
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
