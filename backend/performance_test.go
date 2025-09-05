//go:build integration
// +build integration

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sermon-uploader/config"
	"sermon-uploader/services"
)

// PerformanceMetrics represents comprehensive performance measurements
type PerformanceMetrics struct {
	TestName         string        `json:"test_name"`
	Timestamp        time.Time     `json:"timestamp"`
	Duration         time.Duration `json:"duration"`
	ThroughputMBps   float64       `json:"throughput_mbps"`
	OperationsPerSec float64       `json:"operations_per_sec"`
	SuccessRate      float64       `json:"success_rate"`
	ErrorRate        float64       `json:"error_rate"`

	// Resource utilization
	CPUUsageBefore   float64 `json:"cpu_usage_before"`
	CPUUsageAfter    float64 `json:"cpu_usage_after"`
	CPUUsagePeak     float64 `json:"cpu_usage_peak"`
	MemoryUsedMB     int64   `json:"memory_used_mb"`
	MemoryPeakMB     int64   `json:"memory_peak_mb"`
	GoroutinesBefore int     `json:"goroutines_before"`
	GoroutinesAfter  int     `json:"goroutines_after"`
	GoroutinesPeak   int     `json:"goroutines_peak"`

	// Connection metrics
	ConnectionPoolActive int64         `json:"connection_pool_active"`
	ConnectionPoolIdle   int64         `json:"connection_pool_idle"`
	RetryCount           int64         `json:"retry_count"`
	ConnectionErrors     int64         `json:"connection_errors"`
	LatencyP50           time.Duration `json:"latency_p50"`
	LatencyP95           time.Duration `json:"latency_p95"`
	LatencyP99           time.Duration `json:"latency_p99"`

	// Test-specific metrics
	FilesSent       int64          `json:"files_sent"`
	FilesSuccessful int64          `json:"files_successful"`
	FilesFailed     int64          `json:"files_failed"`
	TotalBytes      int64          `json:"total_bytes"`
	FailureReasons  map[string]int `json:"failure_reasons,omitempty"`
}

// LatencyTracker tracks operation latencies for percentile calculation
type LatencyTracker struct {
	mu        sync.Mutex
	latencies []time.Duration
}

func NewLatencyTracker() *LatencyTracker {
	return &LatencyTracker{
		latencies: make([]time.Duration, 0),
	}
}

func (lt *LatencyTracker) Record(latency time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.latencies = append(lt.latencies, latency)
}

func (lt *LatencyTracker) GetPercentiles() (p50, p95, p99 time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	if len(lt.latencies) == 0 {
		return 0, 0, 0
	}

	// Simple percentile calculation (not optimized for performance)
	n := len(lt.latencies)

	// Sort latencies (simple bubble sort for small datasets)
	sorted := make([]time.Duration, len(lt.latencies))
	copy(sorted, lt.latencies)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	p50 = sorted[int(0.50*float64(n))]
	p95 = sorted[int(0.95*float64(n))]
	p99 = sorted[int(0.99*float64(n))]

	return p50, p95, p99
}

// PerformanceTester provides comprehensive performance testing functionality
type PerformanceTester struct {
	config         *config.Config
	minioService   *services.MinIOService
	testBucket     string
	latencyTracker *LatencyTracker
}

func NewPerformanceTester() *PerformanceTester {
	cfg := config.New()
	cfg.MinioBucket = cfg.MinioBucket + "-perf"

	return &PerformanceTester{
		config:         cfg,
		minioService:   services.NewMinIOService(cfg),
		testBucket:     cfg.MinioBucket,
		latencyTracker: NewLatencyTracker(),
	}
}

// generateTestData creates test data of specified size with optional pattern
func (pt *PerformanceTester) generateTestData(size int64, pattern string) ([]byte, string) {
	data := make([]byte, size)

	switch pattern {
	case "random":
		rand.Read(data)
	case "zeros":
		// Data is already zeros
	case "sequential":
		for i := int64(0); i < size; i++ {
			data[i] = byte(i % 256)
		}
	default:
		// Predictable pattern
		for i := int64(0); i < size; i++ {
			data[i] = byte(i % 256)
		}
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	return data, hash
}

// measureResourceUsage captures current system resource usage
func (pt *PerformanceTester) measureResourceUsage() (cpuPercent float64, memUsedMB int64, goroutines int) {
	// CPU usage (average over short period)
	if cpuPercentages, err := cpu.Percent(100*time.Millisecond, false); err == nil && len(cpuPercentages) > 0 {
		cpuPercent = cpuPercentages[0]
	}

	// Memory usage
	if memInfo, err := mem.VirtualMemory(); err == nil {
		memUsedMB = int64(memInfo.Used / (1024 * 1024))
	}

	// Goroutines
	goroutines = runtime.NumGoroutine()

	return cpuPercent, memUsedMB, goroutines
}

// BenchmarkUploadThroughput tests upload throughput with various file sizes
func (pt *PerformanceTester) BenchmarkUploadThroughput(t *testing.T) *PerformanceMetrics {
	testSizes := []struct {
		name   string
		sizeMB int64
	}{
		{"1MB", 1},
		{"10MB", 10},
		{"50MB", 50},
		{"100MB", 100},
		{"500MB", 500},
	}

	overallMetrics := &PerformanceMetrics{
		TestName:       "UploadThroughput",
		Timestamp:      time.Now(),
		FailureReasons: make(map[string]int),
	}

	// Measure initial state
	cpuBefore, memBefore, goroutinesBefore := pt.measureResourceUsage()
	overallMetrics.CPUUsageBefore = cpuBefore
	overallMetrics.MemoryUsedMB = memBefore
	overallMetrics.GoroutinesBefore = goroutinesBefore

	var totalBytes int64
	var totalDuration time.Duration
	var successfulFiles int64
	var failedFiles int64

	overallStart := time.Now()

	for _, testCase := range testSizes {
		t.Run(testCase.name, func(t *testing.T) {
			size := testCase.sizeMB * 1024 * 1024
			data, hash := pt.generateTestData(size, "sequential")

			start := time.Now()
			reader := bytes.NewReader(data)
			filename := fmt.Sprintf("throughput_test_%s.wav", testCase.name)

			_, err := pt.minioService.UploadFileStreaming(reader, filename, size, hash)

			duration := time.Since(start)
			pt.latencyTracker.Record(duration)

			totalBytes += size
			totalDuration += duration

			if err != nil {
				failedFiles++
				if overallMetrics.FailureReasons == nil {
					overallMetrics.FailureReasons = make(map[string]int)
				}
				overallMetrics.FailureReasons[err.Error()]++
				t.Logf("Upload failed for %s: %v", testCase.name, err)
			} else {
				successfulFiles++
				throughput := float64(size) / (1024 * 1024) / duration.Seconds()
				t.Logf("✓ %s uploaded: %.2f MB/s", testCase.name, throughput)
			}
		})
	}

	overallMetrics.Duration = time.Since(overallStart)

	// Measure final state
	cpuAfter, memAfter, goroutinesAfter := pt.measureResourceUsage()
	overallMetrics.CPUUsageAfter = cpuAfter
	overallMetrics.MemoryPeakMB = memAfter
	overallMetrics.GoroutinesAfter = goroutinesAfter
	overallMetrics.CPUUsagePeak = max(cpuBefore, cpuAfter)
	overallMetrics.GoroutinesPeak = max(goroutinesBefore, goroutinesAfter)

	// Calculate overall metrics
	overallMetrics.TotalBytes = totalBytes
	overallMetrics.FilesSent = successfulFiles + failedFiles
	overallMetrics.FilesSuccessful = successfulFiles
	overallMetrics.FilesFailed = failedFiles
	overallMetrics.ThroughputMBps = float64(totalBytes) / (1024 * 1024) / overallMetrics.Duration.Seconds()
	overallMetrics.SuccessRate = float64(successfulFiles) / float64(successfulFiles+failedFiles) * 100
	overallMetrics.ErrorRate = float64(failedFiles) / float64(successfulFiles+failedFiles) * 100
	overallMetrics.OperationsPerSec = float64(successfulFiles) / overallMetrics.Duration.Seconds()

	// Get latency percentiles
	overallMetrics.LatencyP50, overallMetrics.LatencyP95, overallMetrics.LatencyP99 = pt.latencyTracker.GetPercentiles()

	// Get connection pool stats
	poolStats := pt.minioService.GetConnectionPoolStats()
	overallMetrics.ConnectionPoolActive = poolStats["active"]
	overallMetrics.ConnectionPoolIdle = poolStats["idle"]

	// Get MinIO metrics
	minioMetrics := pt.minioService.GetMetrics()
	overallMetrics.RetryCount = minioMetrics.RetryCount
	overallMetrics.ConnectionErrors = minioMetrics.ConnectionErrors

	return overallMetrics
}

// BenchmarkConcurrentUploads tests concurrent upload performance
func (pt *PerformanceTester) BenchmarkConcurrentUploads(t *testing.T, numConcurrent int) *PerformanceMetrics {
	metrics := &PerformanceMetrics{
		TestName:       fmt.Sprintf("ConcurrentUploads_%d", numConcurrent),
		Timestamp:      time.Now(),
		FailureReasons: make(map[string]int),
	}

	// Measure initial state
	cpuBefore, memBefore, goroutinesBefore := pt.measureResourceUsage()
	metrics.CPUUsageBefore = cpuBefore
	metrics.MemoryUsedMB = memBefore
	metrics.GoroutinesBefore = goroutinesBefore

	// Generate test files (10MB each for concurrent testing)
	const fileSizeMB = 10
	fileSize := int64(fileSizeMB * 1024 * 1024)

	var wg sync.WaitGroup
	results := make([]error, numConcurrent)
	durations := make([]time.Duration, numConcurrent)

	overallStart := time.Now()

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			data, hash := pt.generateTestData(fileSize, "sequential")
			filename := fmt.Sprintf("concurrent_test_%d.wav", index)

			start := time.Now()
			reader := bytes.NewReader(data)

			_, err := pt.minioService.UploadFileStreaming(reader, filename, fileSize, hash)

			durations[index] = time.Since(start)
			results[index] = err

			if err == nil {
				pt.latencyTracker.Record(durations[index])
			}
		}(i)
	}

	wg.Wait()
	metrics.Duration = time.Since(overallStart)

	// Measure final state
	cpuAfter, memAfter, goroutinesAfter := pt.measureResourceUsage()
	metrics.CPUUsageAfter = cpuAfter
	metrics.MemoryPeakMB = memAfter
	metrics.GoroutinesAfter = goroutinesAfter
	metrics.CPUUsagePeak = max(cpuBefore, cpuAfter)
	metrics.GoroutinesPeak = max(goroutinesBefore, goroutinesAfter)

	// Analyze results
	var successCount, failCount int64
	var totalBytes int64

	for i, err := range results {
		if err != nil {
			failCount++
			if metrics.FailureReasons == nil {
				metrics.FailureReasons = make(map[string]int)
			}
			metrics.FailureReasons[err.Error()]++
			t.Logf("Concurrent upload %d failed: %v", i, err)
		} else {
			successCount++
			totalBytes += fileSize
		}
	}

	metrics.FilesSent = int64(numConcurrent)
	metrics.FilesSuccessful = successCount
	metrics.FilesFailed = failCount
	metrics.TotalBytes = totalBytes
	metrics.SuccessRate = float64(successCount) / float64(numConcurrent) * 100
	metrics.ErrorRate = float64(failCount) / float64(numConcurrent) * 100
	metrics.ThroughputMBps = float64(totalBytes) / (1024 * 1024) / metrics.Duration.Seconds()
	metrics.OperationsPerSec = float64(successCount) / metrics.Duration.Seconds()

	// Get latency percentiles
	metrics.LatencyP50, metrics.LatencyP95, metrics.LatencyP99 = pt.latencyTracker.GetPercentiles()

	// Get connection pool and MinIO stats
	poolStats := pt.minioService.GetConnectionPoolStats()
	metrics.ConnectionPoolActive = poolStats["active"]
	metrics.ConnectionPoolIdle = poolStats["idle"]

	minioMetrics := pt.minioService.GetMetrics()
	metrics.RetryCount = minioMetrics.RetryCount
	metrics.ConnectionErrors = minioMetrics.ConnectionErrors

	return metrics
}

// BenchmarkConnectionPoolEfficiency tests connection pool behavior under load
func (pt *PerformanceTester) BenchmarkConnectionPoolEfficiency(t *testing.T) *PerformanceMetrics {
	metrics := &PerformanceMetrics{
		TestName:       "ConnectionPoolEfficiency",
		Timestamp:      time.Now(),
		FailureReasons: make(map[string]int),
	}

	// Measure initial state
	cpuBefore, memBefore, goroutinesBefore := pt.measureResourceUsage()
	metrics.CPUUsageBefore = cpuBefore
	metrics.MemoryUsedMB = memBefore
	metrics.GoroutinesBefore = goroutinesBefore

	const numOperations = 100
	const operationConcurrency = 20

	var wg sync.WaitGroup
	operationResults := make([]error, numOperations)
	operationDurations := make([]time.Duration, numOperations)

	overallStart := time.Now()

	// Perform various MinIO operations to stress connection pool
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			start := time.Now()
			var err error

			switch index % 4 {
			case 0:
				// List files operation
				_, err = pt.minioService.ListFiles()
			case 1:
				// Get file count operation
				_, err = pt.minioService.GetFileCount()
			case 2:
				// Test connection
				err = pt.minioService.TestConnection()
			case 3:
				// Small file upload
				data, hash := pt.generateTestData(1024, "sequential") // 1KB
				reader := bytes.NewReader(data)
				filename := fmt.Sprintf("pool_test_%d.wav", index)
				_, err = pt.minioService.UploadFileStreaming(reader, filename, 1024, hash)
			}

			operationDurations[index] = time.Since(start)
			operationResults[index] = err

			if err == nil {
				pt.latencyTracker.Record(operationDurations[index])
			}

			// Throttle to control concurrency
			if index%operationConcurrency == 0 {
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	metrics.Duration = time.Since(overallStart)

	// Measure final state
	cpuAfter, memAfter, goroutinesAfter := pt.measureResourceUsage()
	metrics.CPUUsageAfter = cpuAfter
	metrics.MemoryPeakMB = memAfter
	metrics.GoroutinesAfter = goroutinesAfter
	metrics.CPUUsagePeak = max(cpuBefore, cpuAfter)
	metrics.GoroutinesPeak = max(goroutinesBefore, goroutinesAfter)

	// Analyze results
	var successCount, failCount int64

	for i, err := range operationResults {
		if err != nil {
			failCount++
			if metrics.FailureReasons == nil {
				metrics.FailureReasons = make(map[string]int)
			}
			metrics.FailureReasons[err.Error()]++
			t.Logf("Connection pool operation %d failed: %v", i, err)
		} else {
			successCount++
		}
	}

	metrics.FilesSent = numOperations
	metrics.FilesSuccessful = successCount
	metrics.FilesFailed = failCount
	metrics.SuccessRate = float64(successCount) / float64(numOperations) * 100
	metrics.ErrorRate = float64(failCount) / float64(numOperations) * 100
	metrics.OperationsPerSec = float64(successCount) / metrics.Duration.Seconds()

	// Get latency percentiles
	metrics.LatencyP50, metrics.LatencyP95, metrics.LatencyP99 = pt.latencyTracker.GetPercentiles()

	// Get connection pool and MinIO stats
	poolStats := pt.minioService.GetConnectionPoolStats()
	metrics.ConnectionPoolActive = poolStats["active"]
	metrics.ConnectionPoolIdle = poolStats["idle"]

	minioMetrics := pt.minioService.GetMetrics()
	metrics.RetryCount = minioMetrics.RetryCount
	metrics.ConnectionErrors = minioMetrics.ConnectionErrors

	return metrics
}

// BenchmarkRetryMechanism tests retry mechanism under simulated failures
func (pt *PerformanceTester) BenchmarkRetryMechanism(t *testing.T) *PerformanceMetrics {
	metrics := &PerformanceMetrics{
		TestName:       "RetryMechanism",
		Timestamp:      time.Now(),
		FailureReasons: make(map[string]int),
	}

	// Measure initial state
	cpuBefore, memBefore, goroutinesBefore := pt.measureResourceUsage()
	metrics.CPUUsageBefore = cpuBefore
	metrics.MemoryUsedMB = memBefore
	metrics.GoroutinesBefore = goroutinesBefore

	const numRetryTests = 20

	overallStart := time.Now()
	var successCount, failCount int64

	for i := 0; i < numRetryTests; i++ {
		// Create test data
		data, hash := pt.generateTestData(1024*1024, "sequential") // 1MB
		filename := fmt.Sprintf("retry_test_%d.wav", i)

		start := time.Now()

		// Use very short timeout to trigger retry mechanism
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		reader := bytes.NewReader(data)

		// This will likely fail and trigger retries
		_, err := pt.minioService.UploadFileStreaming(reader, filename, int64(len(data)), hash)
		cancel()

		duration := time.Since(start)
		pt.latencyTracker.Record(duration)

		if err != nil {
			failCount++
			if metrics.FailureReasons == nil {
				metrics.FailureReasons = make(map[string]int)
			}
			metrics.FailureReasons[err.Error()]++
		} else {
			successCount++
		}

		// Add delay between tests to avoid overwhelming the system
		time.Sleep(100 * time.Millisecond)
	}

	metrics.Duration = time.Since(overallStart)

	// Measure final state
	cpuAfter, memAfter, goroutinesAfter := pt.measureResourceUsage()
	metrics.CPUUsageAfter = cpuAfter
	metrics.MemoryPeakMB = memAfter
	metrics.GoroutinesAfter = goroutinesAfter
	metrics.CPUUsagePeak = max(cpuBefore, cpuAfter)
	metrics.GoroutinesPeak = max(goroutinesBefore, goroutinesAfter)

	metrics.FilesSent = numRetryTests
	metrics.FilesSuccessful = successCount
	metrics.FilesFailed = failCount
	metrics.SuccessRate = float64(successCount) / float64(numRetryTests) * 100
	metrics.ErrorRate = float64(failCount) / float64(numRetryTests) * 100
	metrics.OperationsPerSec = float64(numRetryTests) / metrics.Duration.Seconds()

	// Get latency percentiles
	metrics.LatencyP50, metrics.LatencyP95, metrics.LatencyP99 = pt.latencyTracker.GetPercentiles()

	// Get connection pool and MinIO stats (should show retry counts)
	poolStats := pt.minioService.GetConnectionPoolStats()
	metrics.ConnectionPoolActive = poolStats["active"]
	metrics.ConnectionPoolIdle = poolStats["idle"]

	minioMetrics := pt.minioService.GetMetrics()
	metrics.RetryCount = minioMetrics.RetryCount
	metrics.ConnectionErrors = minioMetrics.ConnectionErrors

	return metrics
}

// printPerformanceReport prints a detailed performance report
func (pt *PerformanceTester) printPerformanceReport(t *testing.T, metrics *PerformanceMetrics) {
	t.Logf("\n" + "="*80)
	t.Logf("PERFORMANCE REPORT: %s", metrics.TestName)
	t.Logf("=" * 80)
	t.Logf("Test Duration: %v", metrics.Duration)
	t.Logf("Throughput: %.2f MB/s", metrics.ThroughputMBps)
	t.Logf("Operations/sec: %.2f", metrics.OperationsPerSec)
	t.Logf("Success Rate: %.1f%% (%d/%d)", metrics.SuccessRate, metrics.FilesSuccessful, metrics.FilesSent)
	t.Logf("Error Rate: %.1f%% (%d failures)", metrics.ErrorRate, metrics.FilesFailed)

	t.Logf("\nResource Usage:")
	t.Logf("  CPU: %.1f%% → %.1f%% (peak: %.1f%%)",
		metrics.CPUUsageBefore, metrics.CPUUsageAfter, metrics.CPUUsagePeak)
	t.Logf("  Memory: %d MB (peak: %d MB)", metrics.MemoryUsedMB, metrics.MemoryPeakMB)
	t.Logf("  Goroutines: %d → %d (peak: %d)",
		metrics.GoroutinesBefore, metrics.GoroutinesAfter, metrics.GoroutinesPeak)

	t.Logf("\nLatency Percentiles:")
	t.Logf("  P50: %v", metrics.LatencyP50)
	t.Logf("  P95: %v", metrics.LatencyP95)
	t.Logf("  P99: %v", metrics.LatencyP99)

	t.Logf("\nConnection Pool:")
	t.Logf("  Active: %d", metrics.ConnectionPoolActive)
	t.Logf("  Idle: %d", metrics.ConnectionPoolIdle)
	t.Logf("  Retries: %d", metrics.RetryCount)
	t.Logf("  Connection Errors: %d", metrics.ConnectionErrors)

	if len(metrics.FailureReasons) > 0 {
		t.Logf("\nFailure Breakdown:")
		for reason, count := range metrics.FailureReasons {
			t.Logf("  %s: %d", reason, count)
		}
	}

	t.Logf("=" * 80)
}

// Test functions

func TestPerformanceUploadThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	pt := NewPerformanceTester()

	// Ensure test bucket exists
	require.NoError(t, pt.minioService.EnsureBucketExists(), "Test bucket should be created")
	defer func() {
		// Cleanup
		pt.minioService.ClearBucket()
	}()

	metrics := pt.BenchmarkUploadThroughput(t)
	pt.printPerformanceReport(t, metrics)

	// Performance assertions for Pi
	assert.Greater(t, metrics.SuccessRate, 80.0, "Success rate should be > 80%")
	assert.Greater(t, metrics.ThroughputMBps, 5.0, "Throughput should be > 5 MB/s on Pi")
	assert.Less(t, metrics.MemoryPeakMB, int64(1024), "Peak memory should be < 1GB on Pi")
	assert.Less(t, metrics.CPUUsagePeak, 95.0, "Peak CPU usage should be < 95%")
}

func TestPerformanceConcurrentUploads(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	pt := NewPerformanceTester()
	require.NoError(t, pt.minioService.EnsureBucketExists(), "Test bucket should be created")
	defer pt.minioService.ClearBucket()

	testCases := []int{5, 10, 20}

	for _, numConcurrent := range testCases {
		t.Run(fmt.Sprintf("Concurrent_%d", numConcurrent), func(t *testing.T) {
			metrics := pt.BenchmarkConcurrentUploads(t, numConcurrent)
			pt.printPerformanceReport(t, metrics)

			// Pi-specific performance assertions
			assert.Greater(t, metrics.SuccessRate, 70.0, "Success rate should be > 70% for concurrent uploads")
			assert.Greater(t, metrics.OperationsPerSec, 1.0, "Should complete > 1 operation/sec")
			assert.Less(t, metrics.MemoryPeakMB, int64(1024), "Peak memory should be < 1GB")

			// Latency should be reasonable
			assert.Less(t, metrics.LatencyP95.Seconds(), 60.0, "P95 latency should be < 60s")
		})
	}
}

func TestPerformanceConnectionPool(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	pt := NewPerformanceTester()
	require.NoError(t, pt.minioService.EnsureBucketExists(), "Test bucket should be created")
	defer pt.minioService.ClearBucket()

	metrics := pt.BenchmarkConnectionPoolEfficiency(t)
	pt.printPerformanceReport(t, metrics)

	// Connection pool efficiency assertions
	assert.Greater(t, metrics.SuccessRate, 90.0, "Connection pool should have > 90% success rate")
	assert.Greater(t, metrics.OperationsPerSec, 5.0, "Should achieve > 5 operations/sec")
	assert.Less(t, metrics.LatencyP50.Milliseconds(), int64(1000), "P50 latency should be < 1s")

	// Connection pool should be utilized efficiently
	assert.Greater(t, metrics.ConnectionPoolActive, int64(0), "Should have active connections")
	assert.Less(t, metrics.ConnectionErrors, int64(10), "Should have < 10 connection errors")
}

func TestPerformanceRetryMechanism(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	pt := NewPerformanceTester()
	require.NoError(t, pt.minioService.EnsureBucketExists(), "Test bucket should be created")
	defer pt.minioService.ClearBucket()

	metrics := pt.BenchmarkRetryMechanism(t)
	pt.printPerformanceReport(t, metrics)

	// Retry mechanism should show evidence of retries
	assert.Greater(t, metrics.RetryCount, int64(0), "Should have retry attempts")

	// Some operations might succeed despite timeouts if they're fast enough
	t.Logf("Retry mechanism test completed with %d retries and %.1f%% success rate",
		metrics.RetryCount, metrics.SuccessRate)
}

// Benchmark functions for go test -bench

func BenchmarkSmallFileUpload(b *testing.B) {
	pt := NewPerformanceTester()
	pt.minioService.EnsureBucketExists()
	defer pt.minioService.ClearBucket()

	// 1MB file
	data, hash := pt.generateTestData(1024*1024, "sequential")

	b.ResetTimer()
	b.SetBytes(1024 * 1024) // 1MB

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		filename := fmt.Sprintf("bench_small_%d.wav", i)

		_, err := pt.minioService.UploadFileStreaming(reader, filename, int64(len(data)), hash)
		if err != nil {
			b.Errorf("Upload failed: %v", err)
		}
	}
}

func BenchmarkMediumFileUpload(b *testing.B) {
	pt := NewPerformanceTester()
	pt.minioService.EnsureBucketExists()
	defer pt.minioService.ClearBucket()

	// 10MB file
	data, hash := pt.generateTestData(10*1024*1024, "sequential")

	b.ResetTimer()
	b.SetBytes(10 * 1024 * 1024) // 10MB

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		filename := fmt.Sprintf("bench_medium_%d.wav", i)

		_, err := pt.minioService.UploadFileStreaming(reader, filename, int64(len(data)), hash)
		if err != nil {
			b.Errorf("Upload failed: %v", err)
		}
	}
}

func BenchmarkConnectionOperations(b *testing.B) {
	pt := NewPerformanceTester()
	pt.minioService.EnsureBucketExists()
	defer pt.minioService.ClearBucket()

	operations := []func() error{
		func() error { return pt.minioService.TestConnection() },
		func() error { _, err := pt.minioService.GetFileCount(); return err },
		func() error { _, err := pt.minioService.ListFiles(); return err },
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		op := operations[i%len(operations)]
		if err := op(); err != nil {
			b.Errorf("Operation failed: %v", err)
		}
	}
}

// Helper function for Go < 1.21 compatibility
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
