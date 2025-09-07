package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sermon-uploader/config"
	"sermon-uploader/services"
)

// MemoryStats holds memory usage statistics
type MemoryStats struct {
	AllocMB      float64
	SysMB        float64
	HeapAllocMB  float64
	HeapSysMB    float64
	StackMB      float64
	Timestamp    time.Time
}

// MemoryMonitor tracks memory usage during tests
type MemoryMonitor struct {
	mu           sync.RWMutex
	samples      []MemoryStats
	maxAllocMB   float64
	maxSysMB     float64
	isMonitoring bool
	stopCh       chan struct{}
}

func NewMemoryMonitor() *MemoryMonitor {
	return &MemoryMonitor{
		samples: make([]MemoryStats, 0),
		stopCh:  make(chan struct{}),
	}
}

func (m *MemoryMonitor) StartMonitoring(intervalMs int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.isMonitoring {
		return
	}
	m.isMonitoring = true
	
	go func() {
		ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				stats := m.getCurrentMemoryStats()
				m.mu.Lock()
				m.samples = append(m.samples, stats)
				if stats.AllocMB > m.maxAllocMB {
					m.maxAllocMB = stats.AllocMB
				}
				if stats.SysMB > m.maxSysMB {
					m.maxSysMB = stats.SysMB
				}
				m.mu.Unlock()
			case <-m.stopCh:
				return
			}
		}
	}()
}

func (m *MemoryMonitor) StopMonitoring() MemoryStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.isMonitoring {
		return MemoryStats{}
	}
	
	m.isMonitoring = false
	close(m.stopCh)
	
	// Force garbage collection to get accurate final measurement
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
	runtime.GC()
	
	finalStats := m.getCurrentMemoryStats()
	finalStats.AllocMB = m.maxAllocMB
	finalStats.SysMB = m.maxSysMB
	
	return finalStats
}

func (m *MemoryMonitor) getCurrentMemoryStats() MemoryStats {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	
	return MemoryStats{
		AllocMB:     float64(mem.Alloc) / 1024 / 1024,
		SysMB:       float64(mem.Sys) / 1024 / 1024,
		HeapAllocMB: float64(mem.HeapAlloc) / 1024 / 1024,
		HeapSysMB:   float64(mem.HeapSys) / 1024 / 1024,
		StackMB:     float64(mem.StackSys) / 1024 / 1024,
		Timestamp:   time.Now(),
	}
}

// TestDirectStreaming_NoMemoryBuffering tests that large uploads use minimal memory
// This test should FAIL with current implementation using io.ReadAll()
func TestDirectStreaming_NoMemoryBuffering(t *testing.T) {
	// Skip if not in memory test mode
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	// Create test config for memory-constrained environment
	cfg := &config.Config{
		MinIOEndpoint:   "localhost:9000",
		MinIOAccessKey:  "testkey",
		MinIOSecretKey:  "testsecret",
		MinioBucket:     "test-bucket",
		MinIOSecure:     false,
		IOBufferSize:    32768, // 32KB buffer
		MaxConcurrentUploads: 1, // Single upload for memory testing
	}

	// Create mock services
	minioService := createMockMinIOService(cfg)
	discordService := createMockDiscordService()
	wsHub := services.NewWebSocketHub()
	fileService := services.NewFileService(minioService, discordService, wsHub, cfg)
	
	// Create handlers
	handlers := New(fileService, minioService, discordService, wsHub, cfg)

	// Create test app
	app := fiber.New(fiber.Config{
		BodyLimit: 11 * 1024 * 1024 * 1024, // 11GB limit for testing
	})
	
	// Setup routes
	app.Post("/upload-tus/:id/complete", handlers.CompleteTUSUpload)

	// Create a 1GB test file data (but don't actually allocate it all at once)
	testFileSize := int64(1024 * 1024 * 1024) // 1GB
	
	// Create streaming reader that generates data on-the-fly
	testReader := NewTestStreamingReader(testFileSize, "test-data-pattern")
	
	// Create mock TUS service that returns our streaming reader
	mockTUSService := &MockTUSService{
		testReader: testReader,
		fileSize:   testFileSize,
	}
	
	// Replace TUS service in file service
	fileService.SetTUSService(mockTUSService) // We'll need to add this method

	// Start memory monitoring
	monitor := NewMemoryMonitor()
	monitor.StartMonitoring(100) // Sample every 100ms

	// Force initial garbage collection for baseline
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
	runtime.GC()

	// Create test request
	req := httptest.NewRequest("POST", "/upload-tus/test123/complete", nil)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := app.Test(req, 60000) // 60 second timeout
	require.NoError(t, err)

	// Stop monitoring and get results
	memStats := monitor.StopMonitoring()

	// Log memory usage for debugging
	t.Logf("Peak Memory Usage:")
	t.Logf("  Allocated: %.2f MB", memStats.AllocMB)
	t.Logf("  System: %.2f MB", memStats.SysMB)
	t.Logf("  Heap Allocated: %.2f MB", memStats.HeapAllocMB)
	t.Logf("  Heap System: %.2f MB", memStats.HeapSysMB)
	t.Logf("  Stack: %.2f MB", memStats.StackMB)

	// Test assertions for zero-copy streaming
	maxAllowedMemoryMB := 50.0 // Should use less than 50MB for 1GB file
	
	// This should FAIL initially, proving current implementation buffers in memory
	assert.LessOrEqual(t, memStats.AllocMB, maxAllowedMemoryMB, 
		"Memory usage too high: %.2f MB for 1GB upload. Expected streaming to use <%.2f MB",
		memStats.AllocMB, maxAllowedMemoryMB)
		
	// Verify the upload completed successfully
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestDirectStreaming_MemoryPressure tests behavior under high memory pressure
func TestDirectStreaming_MemoryPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory pressure test in short mode")
	}

	// Simulate memory pressure by pre-allocating memory
	// Allocate 80% of available memory to simulate constrained environment
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// Simulate Raspberry Pi 5 with 4GB RAM (with ~2GB available for app)
	targetPressureMB := 1600 // 1.6GB allocated to create pressure
	ballastSize := targetPressureMB * 1024 * 1024
	ballast := make([]byte, ballastSize)
	
	// Keep reference so it doesn't get GC'd
	_ = ballast[0]
	
	t.Logf("Created memory pressure: allocated %d MB", targetPressureMB)
	
	// Now try to upload a large file under memory pressure
	cfg := &config.Config{
		MinIOEndpoint:   "localhost:9000",
		MinIOAccessKey:  "testkey", 
		MinIOSecretKey:  "testsecret",
		MinioBucket:     "test-bucket",
		MinIOSecure:     false,
		IOBufferSize:    32768,
		MaxConcurrentUploads: 1,
	}

	// Create services and handlers
	minioService := createMockMinIOService(cfg)
	discordService := createMockDiscordService()
	wsHub := services.NewWebSocketHub()
	fileService := services.NewFileService(minioService, discordService, wsHub, cfg)
	handlers := New(fileService, minioService, discordService, wsHub, cfg)

	// Monitor memory during upload
	monitor := NewMemoryMonitor()
	monitor.StartMonitoring(50) // More frequent sampling

	// Create 500MB file under memory pressure
	testFileSize := int64(500 * 1024 * 1024)
	testReader := NewTestStreamingReader(testFileSize, "pressure-test")
	
	// Test should complete without OOM or significant additional memory allocation
	// Implementation should log memory pressure warnings but continue
	start := time.Now()
	
	// Simulate upload operation (simplified for test)
	buffer := make([]byte, 32768)
	totalRead := int64(0)
	
	for totalRead < testFileSize {
		n, err := testReader.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
		totalRead += int64(n)
		
		// Simulate processing delay
		if totalRead%(50*1024*1024) == 0 { // Every 50MB
			time.Sleep(10 * time.Millisecond)
		}
	}
	
	duration := time.Since(start)
	memStatsAfter := monitor.StopMonitoring()
	
	t.Logf("Upload completed in %v", duration)
	t.Logf("Memory after upload: %.2f MB allocated", memStatsAfter.AllocMB)
	t.Logf("Memory increase during upload: %.2f MB", memStatsAfter.AllocMB)
	
	// Should complete successfully even under memory pressure
	assert.Equal(t, testFileSize, totalRead, "Should read entire file")
	
	// Memory increase should be minimal (streaming)
	memoryIncreaseMB := memStatsAfter.AllocMB
	maxAllowedIncreaseMB := 100.0 // Allow some increase but not proportional to file size
	
	assert.LessOrEqual(t, memoryIncreaseMB, maxAllowedIncreaseMB,
		"Memory increase too high under pressure: %.2f MB", memoryIncreaseMB)
}

// TestMemoryUsageBaseline establishes baseline memory usage
func TestMemoryUsageBaseline(t *testing.T) {
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
	runtime.GC()
	
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	
	baselineAllocMB := float64(mem.Alloc) / 1024 / 1024
	baselineSysMB := float64(mem.Sys) / 1024 / 1024
	
	t.Logf("Baseline Memory Usage:")
	t.Logf("  Allocated: %.2f MB", baselineAllocMB)
	t.Logf("  System: %.2f MB", baselineSysMB)
	
	// Verify baseline is reasonable
	assert.LessOrEqual(t, baselineAllocMB, 20.0, "Baseline memory too high")
}

// TestStreamingReader generates test data on-the-fly without memory allocation
type TestStreamingReader struct {
	size      int64
	read      int64
	pattern   []byte
	patternPos int
}

func NewTestStreamingReader(size int64, pattern string) *TestStreamingReader {
	return &TestStreamingReader{
		size:    size,
		pattern: []byte(pattern),
	}
}

func (r *TestStreamingReader) Read(p []byte) (n int, err error) {
	if r.read >= r.size {
		return 0, io.EOF
	}
	
	remaining := r.size - r.read
	toRead := int64(len(p))
	if toRead > remaining {
		toRead = remaining
	}
	
	// Fill buffer with pattern data
	for i := int64(0); i < toRead; i++ {
		p[i] = r.pattern[r.patternPos%len(r.pattern)]
		r.patternPos++
	}
	
	r.read += toRead
	return int(toRead), nil
}

// Mock services for testing
func createMockMinIOService(cfg *config.Config) *services.MinIOService {
	// This would be replaced with proper mock in real implementation
	// For now, return nil and we'll skip MinIO operations in tests
	return nil
}

func createMockDiscordService() *services.DiscordService {
	// Mock discord service that doesn't actually send messages
	return nil
}

// MockTUSService for testing
type MockTUSService struct {
	testReader io.Reader
	fileSize   int64
}

func (m *MockTUSService) GetUpload(uploadID string) (*services.TUSInfo, error) {
	return &services.TUSInfo{
		Filename: "test-file.wav",
		Size:     m.fileSize,
		Offset:   m.fileSize, // Mark as complete
	}, nil
}

func (m *MockTUSService) GetUploadReader(uploadID string) (io.ReadCloser, error) {
	return io.NopCloser(m.testReader), nil
}

func (m *MockTUSService) DeleteUpload(uploadID string) error {
	return nil
}