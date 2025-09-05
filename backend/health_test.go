//go:build integration
// +build integration

package main

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sermon-uploader/config"
	"sermon-uploader/services"
)

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	Component    string        `json:"component"`
	Status       string        `json:"status"` // "healthy", "warning", "critical"
	Message      string        `json:"message"`
	ResponseTime time.Duration `json:"response_time"`
	Details      interface{}   `json:"details,omitempty"`
}

// SystemHealth represents overall system health metrics
type SystemHealth struct {
	Timestamp     time.Time           `json:"timestamp"`
	OverallStatus string              `json:"overall_status"`
	Checks        []HealthCheckResult `json:"checks"`
	SystemMetrics SystemMetrics       `json:"system_metrics"`
}

// SystemMetrics represents system resource metrics
type SystemMetrics struct {
	Memory    MemoryMetrics  `json:"memory"`
	CPU       CPUMetrics     `json:"cpu"`
	Disk      DiskMetrics    `json:"disk"`
	Network   NetworkMetrics `json:"network"`
	Processes ProcessMetrics `json:"processes"`
}

type MemoryMetrics struct {
	Total       uint64  `json:"total_mb"`
	Available   uint64  `json:"available_mb"`
	Used        uint64  `json:"used_mb"`
	UsedPercent float64 `json:"used_percent"`
	Buffers     uint64  `json:"buffers_mb"`
	Cached      uint64  `json:"cached_mb"`
}

type CPUMetrics struct {
	Cores       int       `json:"cores"`
	Usage       []float64 `json:"usage_percent"`
	Temperature float64   `json:"temperature_c"`
	LoadAvg     []float64 `json:"load_avg"`
}

type DiskMetrics struct {
	Total       uint64  `json:"total_gb"`
	Free        uint64  `json:"free_gb"`
	Used        uint64  `json:"used_gb"`
	UsedPercent float64 `json:"used_percent"`
	Path        string  `json:"path"`
}

type NetworkMetrics struct {
	BytesSent   uint64 `json:"bytes_sent"`
	BytesRecv   uint64 `json:"bytes_recv"`
	PacketsSent uint64 `json:"packets_sent"`
	PacketsRecv uint64 `json:"packets_recv"`
	Errors      uint64 `json:"errors"`
}

type ProcessMetrics struct {
	Goroutines int `json:"goroutines"`
	CGoCalls   int `json:"cgo_calls"`
}

// HealthChecker provides comprehensive health checking functionality
type HealthChecker struct {
	config       *config.Config
	minioService *services.MinIOService
}

// NewHealthChecker creates a new health checker
func NewHealthChecker() *HealthChecker {
	cfg := config.New()
	return &HealthChecker{
		config:       cfg,
		minioService: services.NewMinIOService(cfg),
	}
}

// TestMinIOConnectivity tests MinIO connection and bucket access
func (hc *HealthChecker) TestMinIOConnectivity() HealthCheckResult {
	start := time.Now()
	result := HealthCheckResult{
		Component: "MinIO",
		Status:    "healthy",
	}

	// Test basic connection
	err := hc.minioService.TestConnection()
	if err != nil {
		result.Status = "critical"
		result.Message = fmt.Sprintf("MinIO connection failed: %v", err)
		result.ResponseTime = time.Since(start)
		return result
	}

	// Test bucket access
	err = hc.minioService.EnsureBucketExists()
	if err != nil {
		result.Status = "critical"
		result.Message = fmt.Sprintf("Bucket access failed: %v", err)
		result.ResponseTime = time.Since(start)
		return result
	}

	// Test file listing (basic operation)
	fileCount, err := hc.minioService.GetFileCount()
	if err != nil {
		result.Status = "warning"
		result.Message = fmt.Sprintf("File listing warning: %v", err)
	} else {
		result.Message = fmt.Sprintf("MinIO healthy, %d files in bucket", fileCount)
	}

	result.ResponseTime = time.Since(start)
	result.Details = map[string]interface{}{
		"endpoint": hc.config.MinIOEndpoint,
		"bucket":   hc.config.MinioBucket,
		"files":    fileCount,
	}

	return result
}

// TestAPIEndpoints tests API endpoint availability and response times
func (hc *HealthChecker) TestAPIEndpoints() HealthCheckResult {
	start := time.Now()
	result := HealthCheckResult{
		Component: "API",
		Status:    "healthy",
	}

	baseURL := fmt.Sprintf("http://localhost:%s", hc.config.Port)
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	endpoints := map[string]string{
		"health":    "/health",
		"files":     "/api/files",
		"duplicate": "/api/check-duplicate",
	}

	endpointResults := make(map[string]interface{})
	failedEndpoints := 0

	for name, path := range endpoints {
		endpointStart := time.Now()
		resp, err := client.Get(baseURL + path)
		endpointDuration := time.Since(endpointStart)

		endpointResult := map[string]interface{}{
			"response_time": endpointDuration.Milliseconds(),
		}

		if err != nil {
			endpointResult["status"] = "failed"
			endpointResult["error"] = err.Error()
			failedEndpoints++
		} else {
			endpointResult["status"] = "ok"
			endpointResult["status_code"] = resp.StatusCode
			resp.Body.Close()
		}

		endpointResults[name] = endpointResult
	}

	if failedEndpoints > 0 {
		if failedEndpoints == len(endpoints) {
			result.Status = "critical"
			result.Message = "All API endpoints failed"
		} else {
			result.Status = "warning"
			result.Message = fmt.Sprintf("%d of %d endpoints failed", failedEndpoints, len(endpoints))
		}
	} else {
		result.Message = "All API endpoints responding"
	}

	result.ResponseTime = time.Since(start)
	result.Details = endpointResults

	return result
}

// TestMemoryUsage tests memory usage within Pi limits
func (hc *HealthChecker) TestMemoryUsage() HealthCheckResult {
	start := time.Now()
	result := HealthCheckResult{
		Component: "Memory",
		Status:    "healthy",
	}

	memInfo, err := mem.VirtualMemory()
	if err != nil {
		result.Status = "warning"
		result.Message = fmt.Sprintf("Failed to get memory info: %v", err)
		result.ResponseTime = time.Since(start)
		return result
	}

	// Convert to MB for easier reading
	totalMB := memInfo.Total / (1024 * 1024)
	usedMB := memInfo.Used / (1024 * 1024)
	availableMB := memInfo.Available / (1024 * 1024)

	// Pi-specific memory thresholds
	memoryThreshold := float64(hc.config.MaxMemoryLimitMB) // 800MB default for Pi
	criticalThreshold := memoryThreshold * 0.9             // 90% of limit
	warningThreshold := memoryThreshold * 0.75             // 75% of limit

	details := map[string]interface{}{
		"total_mb":     totalMB,
		"used_mb":      usedMB,
		"available_mb": availableMB,
		"used_percent": memInfo.UsedPercent,
		"threshold_mb": memoryThreshold,
		"buffers_mb":   memInfo.Buffers / (1024 * 1024),
		"cached_mb":    memInfo.Cached / (1024 * 1024),
	}

	if float64(usedMB) > criticalThreshold {
		result.Status = "critical"
		result.Message = fmt.Sprintf("Memory usage critical: %.1f MB (%.1f%%) exceeds threshold",
			float64(usedMB), memInfo.UsedPercent)
	} else if float64(usedMB) > warningThreshold {
		result.Status = "warning"
		result.Message = fmt.Sprintf("Memory usage high: %.1f MB (%.1f%%)",
			float64(usedMB), memInfo.UsedPercent)
	} else {
		result.Message = fmt.Sprintf("Memory usage normal: %.1f MB (%.1f%%)",
			float64(usedMB), memInfo.UsedPercent)
	}

	result.ResponseTime = time.Since(start)
	result.Details = details

	return result
}

// TestDiskSpace tests available disk space
func (hc *HealthChecker) TestDiskSpace() HealthCheckResult {
	start := time.Now()
	result := HealthCheckResult{
		Component: "Disk",
		Status:    "healthy",
	}

	// Check disk usage for the temp directory (where files are processed)
	tempDir := hc.config.TempDir
	if tempDir == "" {
		tempDir = "/tmp"
	}

	diskInfo, err := disk.Usage(tempDir)
	if err != nil {
		result.Status = "warning"
		result.Message = fmt.Sprintf("Failed to get disk info: %v", err)
		result.ResponseTime = time.Since(start)
		return result
	}

	// Convert to GB for easier reading
	totalGB := diskInfo.Total / (1024 * 1024 * 1024)
	freeGB := diskInfo.Free / (1024 * 1024 * 1024)
	usedGB := diskInfo.Used / (1024 * 1024 * 1024)

	// Disk space thresholds
	criticalThreshold := 95.0 // 95% used
	warningThreshold := 80.0  // 80% used

	details := map[string]interface{}{
		"total_gb":     totalGB,
		"free_gb":      freeGB,
		"used_gb":      usedGB,
		"used_percent": diskInfo.UsedPercent,
		"path":         diskInfo.Path,
	}

	if diskInfo.UsedPercent > criticalThreshold {
		result.Status = "critical"
		result.Message = fmt.Sprintf("Disk space critical: %.1f%% used, %.1f GB free",
			diskInfo.UsedPercent, float64(freeGB))
	} else if diskInfo.UsedPercent > warningThreshold {
		result.Status = "warning"
		result.Message = fmt.Sprintf("Disk space low: %.1f%% used, %.1f GB free",
			diskInfo.UsedPercent, float64(freeGB))
	} else {
		result.Message = fmt.Sprintf("Disk space adequate: %.1f GB free (%.1f%% free)",
			float64(freeGB), 100.0-diskInfo.UsedPercent)
	}

	result.ResponseTime = time.Since(start)
	result.Details = details

	return result
}

// TestNetworkConnectivity tests network connectivity to required services
func (hc *HealthChecker) TestNetworkConnectivity() HealthCheckResult {
	start := time.Now()
	result := HealthCheckResult{
		Component: "Network",
		Status:    "healthy",
	}

	// Test connectivity to MinIO endpoint
	minioHost := hc.config.MinIOEndpoint

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	connectivityResults := make(map[string]interface{})
	failedConnections := 0

	// Test MinIO connectivity
	minioURL := fmt.Sprintf("http://%s/minio/health/live", minioHost)
	if hc.config.MinIOSecure {
		minioURL = fmt.Sprintf("https://%s/minio/health/live", minioHost)
	}

	connStart := time.Now()
	resp, err := client.Get(minioURL)
	connDuration := time.Since(connStart)

	if err != nil {
		connectivityResults["minio"] = map[string]interface{}{
			"status":        "failed",
			"error":         err.Error(),
			"response_time": connDuration.Milliseconds(),
		}
		failedConnections++
	} else {
		connectivityResults["minio"] = map[string]interface{}{
			"status":        "ok",
			"status_code":   resp.StatusCode,
			"response_time": connDuration.Milliseconds(),
		}
		resp.Body.Close()
	}

	// Test Discord webhook if configured
	if hc.config.DiscordWebhookURL != "" {
		connStart = time.Now()
		// Just test if the webhook URL is reachable (without sending a message)
		req, _ := http.NewRequest("GET", hc.config.DiscordWebhookURL, nil)
		resp, err := client.Do(req)
		connDuration = time.Since(connStart)

		if err != nil {
			connectivityResults["discord"] = map[string]interface{}{
				"status":        "failed",
				"error":         err.Error(),
				"response_time": connDuration.Milliseconds(),
			}
			failedConnections++
		} else {
			connectivityResults["discord"] = map[string]interface{}{
				"status":        "ok",
				"status_code":   resp.StatusCode,
				"response_time": connDuration.Milliseconds(),
			}
			resp.Body.Close()
		}
	}

	if failedConnections > 0 {
		result.Status = "warning"
		result.Message = fmt.Sprintf("%d network connections failed", failedConnections)
	} else {
		result.Message = "All network connections healthy"
	}

	result.ResponseTime = time.Since(start)
	result.Details = connectivityResults

	return result
}

// TestCPUTemperature tests CPU temperature (Pi-specific)
func (hc *HealthChecker) TestCPUTemperature() HealthCheckResult {
	start := time.Now()
	result := HealthCheckResult{
		Component: "CPU",
		Status:    "healthy",
	}

	// Get CPU usage
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		result.Status = "warning"
		result.Message = fmt.Sprintf("Failed to get CPU usage: %v", err)
		result.ResponseTime = time.Since(start)
		return result
	}

	// Get CPU temperature (Pi-specific - may not work on all systems)
	temps, err := host.SensorsTemperatures()

	details := map[string]interface{}{
		"cores":         runtime.NumCPU(),
		"usage_percent": cpuPercent,
		"goroutines":    runtime.NumGoroutine(),
	}

	var maxTemp float64
	if err == nil && len(temps) > 0 {
		for _, temp := range temps {
			if temp.Temperature > maxTemp {
				maxTemp = temp.Temperature
			}
		}
		details["temperature_c"] = maxTemp

		// Pi thermal throttling thresholds
		criticalTemp := hc.config.ThermalThresholdC // 75°C default
		warningTemp := criticalTemp - 10.0          // 65°C

		if maxTemp > criticalTemp {
			result.Status = "critical"
			result.Message = fmt.Sprintf("CPU temperature critical: %.1f°C", maxTemp)
		} else if maxTemp > warningTemp {
			result.Status = "warning"
			result.Message = fmt.Sprintf("CPU temperature high: %.1f°C", maxTemp)
		} else {
			result.Message = fmt.Sprintf("CPU healthy: %.1f°C, %.1f%% usage", maxTemp, cpuPercent[0])
		}
	} else {
		// Temperature not available, just check CPU usage
		if len(cpuPercent) > 0 && cpuPercent[0] > 90.0 {
			result.Status = "warning"
			result.Message = fmt.Sprintf("CPU usage high: %.1f%%", cpuPercent[0])
		} else {
			result.Message = fmt.Sprintf("CPU usage normal: %.1f%%", cpuPercent[0])
		}
	}

	result.ResponseTime = time.Since(start)
	result.Details = details

	return result
}

// GetSystemHealth performs comprehensive system health check
func (hc *HealthChecker) GetSystemHealth() *SystemHealth {
	checks := []HealthCheckResult{
		hc.TestMinIOConnectivity(),
		hc.TestMemoryUsage(),
		hc.TestDiskSpace(),
		hc.TestNetworkConnectivity(),
		hc.TestCPUTemperature(),
		hc.TestAPIEndpoints(),
	}

	// Determine overall status
	overallStatus := "healthy"
	for _, check := range checks {
		if check.Status == "critical" {
			overallStatus = "critical"
			break
		} else if check.Status == "warning" && overallStatus != "critical" {
			overallStatus = "warning"
		}
	}

	// Collect system metrics
	systemMetrics := hc.collectSystemMetrics()

	return &SystemHealth{
		Timestamp:     time.Now(),
		OverallStatus: overallStatus,
		Checks:        checks,
		SystemMetrics: systemMetrics,
	}
}

// collectSystemMetrics gathers detailed system metrics
func (hc *HealthChecker) collectSystemMetrics() SystemMetrics {
	metrics := SystemMetrics{}

	// Memory metrics
	if memInfo, err := mem.VirtualMemory(); err == nil {
		metrics.Memory = MemoryMetrics{
			Total:       memInfo.Total / (1024 * 1024),
			Available:   memInfo.Available / (1024 * 1024),
			Used:        memInfo.Used / (1024 * 1024),
			UsedPercent: memInfo.UsedPercent,
			Buffers:     memInfo.Buffers / (1024 * 1024),
			Cached:      memInfo.Cached / (1024 * 1024),
		}
	}

	// CPU metrics
	if cpuPercent, err := cpu.Percent(time.Second, true); err == nil {
		metrics.CPU.Cores = runtime.NumCPU()
		metrics.CPU.Usage = cpuPercent

		// Try to get temperature
		if temps, err := host.SensorsTemperatures(); err == nil && len(temps) > 0 {
			var maxTemp float64
			for _, temp := range temps {
				if temp.Temperature > maxTemp {
					maxTemp = temp.Temperature
				}
			}
			metrics.CPU.Temperature = maxTemp
		}
	}

	// Disk metrics
	tempDir := hc.config.TempDir
	if tempDir == "" {
		tempDir = "/tmp"
	}
	if diskInfo, err := disk.Usage(tempDir); err == nil {
		metrics.Disk = DiskMetrics{
			Total:       diskInfo.Total / (1024 * 1024 * 1024),
			Free:        diskInfo.Free / (1024 * 1024 * 1024),
			Used:        diskInfo.Used / (1024 * 1024 * 1024),
			UsedPercent: diskInfo.UsedPercent,
			Path:        diskInfo.Path,
		}
	}

	// Network metrics
	if netStats, err := net.IOCounters(false); err == nil && len(netStats) > 0 {
		metrics.Network = NetworkMetrics{
			BytesSent:   netStats[0].BytesSent,
			BytesRecv:   netStats[0].BytesRecv,
			PacketsSent: netStats[0].PacketsSent,
			PacketsRecv: netStats[0].PacketsRecv,
			Errors:      netStats[0].Errin + netStats[0].Errout,
		}
	}

	// Process metrics
	metrics.Processes = ProcessMetrics{
		Goroutines: runtime.NumGoroutine(),
		CGoCalls:   int(runtime.NumCgoCall()),
	}

	return metrics
}

// Test functions

func TestHealthMinIOConnectivity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping health check in short mode")
	}

	hc := NewHealthChecker()
	result := hc.TestMinIOConnectivity()

	assert.NotEmpty(t, result.Component, "Component should be set")
	assert.Contains(t, []string{"healthy", "warning", "critical"}, result.Status, "Status should be valid")
	assert.NotEmpty(t, result.Message, "Message should be provided")
	assert.Greater(t, result.ResponseTime, time.Duration(0), "Response time should be positive")

	if result.Status == "critical" {
		t.Errorf("MinIO connectivity critical: %s", result.Message)
	} else if result.Status == "warning" {
		t.Logf("MinIO connectivity warning: %s", result.Message)
	} else {
		t.Logf("MinIO connectivity healthy: %s", result.Message)
	}
}

func TestHealthMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping health check in short mode")
	}

	hc := NewHealthChecker()
	result := hc.TestMemoryUsage()

	assert.Equal(t, "Memory", result.Component, "Component should be Memory")
	assert.Contains(t, []string{"healthy", "warning", "critical"}, result.Status, "Status should be valid")

	// Memory usage should be within reasonable Pi limits
	if details, ok := result.Details.(map[string]interface{}); ok {
		if usedMB, exists := details["used_mb"]; exists {
			used := usedMB.(uint64)
			assert.Less(t, used, uint64(1024), "Memory usage should be less than 1GB on Pi")
		}
	}

	t.Logf("Memory health: %s (%s)", result.Status, result.Message)
}

func TestHealthDiskSpace(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping health check in short mode")
	}

	hc := NewHealthChecker()
	result := hc.TestDiskSpace()

	assert.Equal(t, "Disk", result.Component, "Component should be Disk")
	assert.Contains(t, []string{"healthy", "warning", "critical"}, result.Status, "Status should be valid")

	// Disk should have some free space
	if details, ok := result.Details.(map[string]interface{}); ok {
		if freeGB, exists := details["free_gb"]; exists {
			free := freeGB.(uint64)
			assert.Greater(t, free, uint64(1), "Should have at least 1GB free space")
		}
	}

	t.Logf("Disk health: %s (%s)", result.Status, result.Message)
}

func TestHealthNetworkConnectivity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping health check in short mode")
	}

	hc := NewHealthChecker()
	result := hc.TestNetworkConnectivity()

	assert.Equal(t, "Network", result.Component, "Component should be Network")
	assert.Contains(t, []string{"healthy", "warning", "critical"}, result.Status, "Status should be valid")

	t.Logf("Network health: %s (%s)", result.Status, result.Message)
}

func TestHealthCPUTemperature(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping health check in short mode")
	}

	hc := NewHealthChecker()
	result := hc.TestCPUTemperature()

	assert.Equal(t, "CPU", result.Component, "Component should be CPU")
	assert.Contains(t, []string{"healthy", "warning", "critical"}, result.Status, "Status should be valid")

	// CPU temperature should be reasonable if available
	if details, ok := result.Details.(map[string]interface{}); ok {
		if temp, exists := details["temperature_c"]; exists {
			temperature := temp.(float64)
			assert.Less(t, temperature, 85.0, "CPU temperature should be below 85°C")
			assert.Greater(t, temperature, 0.0, "CPU temperature should be positive")
		}
	}

	t.Logf("CPU health: %s (%s)", result.Status, result.Message)
}

func TestSystemHealthOverall(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping health check in short mode")
	}

	hc := NewHealthChecker()
	health := hc.GetSystemHealth()

	require.NotNil(t, health, "Health result should not be nil")
	assert.Contains(t, []string{"healthy", "warning", "critical"}, health.OverallStatus,
		"Overall status should be valid")
	assert.NotEmpty(t, health.Checks, "Should have health checks")
	assert.NotZero(t, health.Timestamp, "Should have timestamp")

	// Log detailed results
	t.Logf("Overall system health: %s", health.OverallStatus)
	for _, check := range health.Checks {
		t.Logf("  %s: %s - %s (%.2fms)",
			check.Component, check.Status, check.Message,
			float64(check.ResponseTime.Nanoseconds())/1000000)
	}

	// System metrics validation
	assert.Greater(t, health.SystemMetrics.Memory.Total, uint64(0), "Should have memory info")
	assert.Greater(t, health.SystemMetrics.CPU.Cores, 0, "Should have CPU info")
	assert.Greater(t, health.SystemMetrics.Disk.Total, uint64(0), "Should have disk info")
	assert.GreaterOrEqual(t, health.SystemMetrics.Processes.Goroutines, 1, "Should have process info")

	// Fail test if system is in critical state
	if health.OverallStatus == "critical" {
		t.Errorf("System health is critical - check individual components")
	}
}

// Benchmark health check performance
func BenchmarkHealthChecks(b *testing.B) {
	hc := NewHealthChecker()

	b.Run("MinIO", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			hc.TestMinIOConnectivity()
		}
	})

	b.Run("Memory", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			hc.TestMemoryUsage()
		}
	})

	b.Run("Disk", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			hc.TestDiskSpace()
		}
	})

	b.Run("CPU", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			hc.TestCPUTemperature()
		}
	})

	b.Run("Full", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			hc.GetSystemHealth()
		}
	})
}
