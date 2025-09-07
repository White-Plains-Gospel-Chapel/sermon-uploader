package services

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"sermon-uploader/config"
)

// MemoryMonitorService tracks memory usage and detects memory pressure
type MemoryMonitorService struct {
	config               *config.Config
	logger               *slog.Logger
	mu                   sync.RWMutex
	isMonitoring         bool
	stopCh               chan struct{}
	currentStats         MemoryStats
	memoryPressureThresh float64 // Memory pressure threshold (0.0-1.0)
	maxMemoryMB          float64 // Maximum allowed memory in MB
	onMemoryPressure     func(stats MemoryStats)
	onMemoryAlert        func(stats MemoryStats)
}

// MemoryStats represents memory usage statistics
type MemoryStats struct {
	AllocMB      float64   `json:"alloc_mb"`
	SysMB        float64   `json:"sys_mb"`
	HeapAllocMB  float64   `json:"heap_alloc_mb"`
	HeapSysMB    float64   `json:"heap_sys_mb"`
	StackMB      float64   `json:"stack_mb"`
	GCCycles     uint32    `json:"gc_cycles"`
	LastGC       time.Time `json:"last_gc"`
	Timestamp    time.Time `json:"timestamp"`
	PressureLevel string   `json:"pressure_level"` // "normal", "warning", "critical"
}

// MemoryPressureLevel represents different levels of memory pressure
type MemoryPressureLevel int

const (
	MemoryPressureNormal MemoryPressureLevel = iota
	MemoryPressureWarning
	MemoryPressureCritical
)

func (level MemoryPressureLevel) String() string {
	switch level {
	case MemoryPressureNormal:
		return "normal"
	case MemoryPressureWarning:
		return "warning" 
	case MemoryPressureCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// NewMemoryMonitorService creates a new memory monitoring service
func NewMemoryMonitorService(cfg *config.Config) *MemoryMonitorService {
	// Default to Raspberry Pi 5 memory constraints (4GB total, ~2GB usable for app)
	maxMemoryMB := 1800.0 // 1.8GB limit for safety
	pressureThresh := 0.8 // 80% memory usage triggers warnings
	
	if cfg.MaxMemoryLimitMB > 0 {
		maxMemoryMB = float64(cfg.MaxMemoryLimitMB)
	}
	
	// Use fixed threshold since config doesn't have MemoryPressureThreshold field
	// pressureThresh = 0.8 (already set above)

	return &MemoryMonitorService{
		config:               cfg,
		logger:               slog.Default(),
		stopCh:               make(chan struct{}),
		memoryPressureThresh: pressureThresh,
		maxMemoryMB:          maxMemoryMB,
	}
}

// SetMemoryPressureCallback sets callback for memory pressure events
func (m *MemoryMonitorService) SetMemoryPressureCallback(callback func(stats MemoryStats)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onMemoryPressure = callback
}

// SetMemoryAlertCallback sets callback for memory alert events  
func (m *MemoryMonitorService) SetMemoryAlertCallback(callback func(stats MemoryStats)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onMemoryAlert = callback
}

// StartMonitoring begins continuous memory monitoring
func (m *MemoryMonitorService) StartMonitoring(ctx context.Context, intervalMs int) {
	m.mu.Lock()
	if m.isMonitoring {
		m.mu.Unlock()
		return
	}
	m.isMonitoring = true
	m.stopCh = make(chan struct{})
	m.mu.Unlock()

	m.logger.Info("Starting memory monitoring",
		"interval_ms", intervalMs,
		"pressure_threshold", m.memoryPressureThresh,
		"max_memory_mb", m.maxMemoryMB)

	go func() {
		ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-m.stopCh:
				return
			case <-ticker.C:
				stats := m.captureMemoryStats()
				m.mu.Lock()
				m.currentStats = stats
				m.mu.Unlock()
				
				m.checkMemoryPressure(stats)
			}
		}
	}()
}

// StopMonitoring stops memory monitoring
func (m *MemoryMonitorService) StopMonitoring() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.isMonitoring {
		return
	}
	
	m.isMonitoring = false
	close(m.stopCh)
	m.logger.Info("Stopped memory monitoring")
}

// GetCurrentStats returns current memory statistics
func (m *MemoryMonitorService) GetCurrentStats() MemoryStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentStats
}

// ForceGC forces garbage collection and returns memory stats before/after
func (m *MemoryMonitorService) ForceGC() (before, after MemoryStats) {
	before = m.captureMemoryStats()
	
	m.logger.Debug("Forcing garbage collection due to memory pressure")
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
	runtime.GC() // Double GC for thoroughness
	
	after = m.captureMemoryStats()
	
	freedMB := before.AllocMB - after.AllocMB
	m.logger.Info("Garbage collection completed",
		"freed_mb", freedMB,
		"before_alloc_mb", before.AllocMB,
		"after_alloc_mb", after.AllocMB)
		
	return before, after
}

// CheckMemoryForUpload verifies if there's enough memory for an upload
func (m *MemoryMonitorService) CheckMemoryForUpload(fileSizeMB float64) (canProceed bool, suggestion string) {
	stats := m.captureMemoryStats()
	
	// Calculate if there's enough memory for the upload
	projectedUsageMB := stats.AllocMB + fileSizeMB
	usageRatio := projectedUsageMB / m.maxMemoryMB
	
	if usageRatio < m.memoryPressureThresh {
		return true, ""
	}
	
	if usageRatio < 0.95 { // Critical threshold
		// Try garbage collection first
		_, afterGC := m.ForceGC()
		projectedAfterGCMB := afterGC.AllocMB + fileSizeMB
		usageAfterGCRatio := projectedAfterGCMB / m.maxMemoryMB
		
		if usageAfterGCRatio < m.memoryPressureThresh {
			return true, "memory_gc_helped"
		}
		
		return false, fmt.Sprintf("insufficient_memory_after_gc: would use %.1fMB/%.1fMB (%.1f%%)",
			projectedAfterGCMB, m.maxMemoryMB, usageAfterGCRatio*100)
	}
	
	return false, fmt.Sprintf("insufficient_memory: would use %.1fMB/%.1fMB (%.1f%%)",
		projectedUsageMB, m.maxMemoryMB, usageRatio*100)
}

// captureMemoryStats captures current memory statistics
func (m *MemoryMonitorService) captureMemoryStats() MemoryStats {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	
	// Determine pressure level
	currentUsageRatio := float64(mem.Alloc) / (m.maxMemoryMB * 1024 * 1024)
	var pressureLevel string
	
	if currentUsageRatio < m.memoryPressureThresh {
		pressureLevel = "normal"
	} else if currentUsageRatio < 0.95 {
		pressureLevel = "warning"
	} else {
		pressureLevel = "critical"
	}
	
	lastGC := time.Unix(0, int64(mem.LastGC))
	if mem.LastGC == 0 {
		lastGC = time.Time{}
	}
	
	return MemoryStats{
		AllocMB:       float64(mem.Alloc) / 1024 / 1024,
		SysMB:         float64(mem.Sys) / 1024 / 1024,
		HeapAllocMB:   float64(mem.HeapAlloc) / 1024 / 1024,
		HeapSysMB:     float64(mem.HeapSys) / 1024 / 1024,
		StackMB:       float64(mem.StackSys) / 1024 / 1024,
		GCCycles:      mem.NumGC,
		LastGC:        lastGC,
		Timestamp:     time.Now(),
		PressureLevel: pressureLevel,
	}
}

// checkMemoryPressure evaluates memory pressure and triggers callbacks
func (m *MemoryMonitorService) checkMemoryPressure(stats MemoryStats) {
	usageRatio := stats.AllocMB / m.maxMemoryMB
	
	switch {
	case usageRatio >= 0.95: // Critical - 95%+
		m.logger.Warn("Critical memory pressure detected",
			"usage_mb", stats.AllocMB,
			"max_mb", m.maxMemoryMB,
			"usage_percent", usageRatio*100,
			"gc_cycles", stats.GCCycles)
			
		m.mu.RLock()
		callback := m.onMemoryAlert
		m.mu.RUnlock()
		
		if callback != nil {
			callback(stats)
		}
		
		// Force immediate GC on critical pressure
		m.ForceGC()
		
	case usageRatio >= m.memoryPressureThresh: // Warning threshold
		m.logger.Debug("Memory pressure warning",
			"usage_mb", stats.AllocMB,
			"max_mb", m.maxMemoryMB,
			"usage_percent", usageRatio*100)
			
		m.mu.RLock()
		callback := m.onMemoryPressure
		m.mu.RUnlock()
		
		if callback != nil {
			callback(stats)
		}
	}
}

// GetMemoryStatsForAPI returns memory stats formatted for API responses
func (m *MemoryMonitorService) GetMemoryStatsForAPI() map[string]interface{} {
	stats := m.GetCurrentStats()
	usageRatio := stats.AllocMB / m.maxMemoryMB
	
	return map[string]interface{}{
		"alloc_mb":         fmt.Sprintf("%.2f", stats.AllocMB),
		"sys_mb":           fmt.Sprintf("%.2f", stats.SysMB),
		"heap_alloc_mb":    fmt.Sprintf("%.2f", stats.HeapAllocMB),
		"usage_percent":    fmt.Sprintf("%.1f", usageRatio*100),
		"pressure_level":   stats.PressureLevel,
		"max_memory_mb":    m.maxMemoryMB,
		"gc_cycles":        stats.GCCycles,
		"last_gc":          stats.LastGC.Format(time.RFC3339),
		"monitoring":       m.isMonitoring,
	}
}