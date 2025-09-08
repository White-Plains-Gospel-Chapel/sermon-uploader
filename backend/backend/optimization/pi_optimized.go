package optimization

import (
	"context"
	"runtime"
	"runtime/debug"
	"sync"
	
	"golang.org/x/sync/semaphore"
)

// PiOptimizer contains Pi-specific optimizations
type PiOptimizer struct {
	// Semaphore for limiting concurrent operations
	uploadSem *semaphore.Weighted
	hashSem   *semaphore.Weighted
	
	// Resource limits
	maxConcurrentUploads int64
	maxMemoryMB          int
	
	// Metrics
	activeUploads int64
	mu            sync.RWMutex
}

// NewPiOptimizer creates optimizations specifically for Raspberry Pi
func NewPiOptimizer() *PiOptimizer {
	numCPU := runtime.NumCPU()
	
	// Pi-specific limits
	var maxUploads int64
	var maxMem int
	
	switch {
	case numCPU <= 2:
		// Pi Zero/1/2
		maxUploads = 1
		maxMem = 256
	case numCPU <= 4:
		// Pi 3/4
		maxUploads = 2
		maxMem = 512
	default:
		// Pi 5 or better
		maxUploads = 3
		maxMem = 1024
	}
	
	// Set runtime limits
	debug.SetGCPercent(50)                        // More aggressive GC
	debug.SetMemoryLimit(int64(maxMem) * 1024 * 1024) // Hard memory limit
	
	return &PiOptimizer{
		uploadSem:            semaphore.NewWeighted(maxUploads),
		hashSem:              semaphore.NewWeighted(int64(numCPU)),
		maxConcurrentUploads: maxUploads,
		maxMemoryMB:          maxMem,
	}
}

// AcquireUploadSlot gets permission to upload (blocks if at limit)
func (p *PiOptimizer) AcquireUploadSlot(ctx context.Context) error {
	return p.uploadSem.Acquire(ctx, 1)
}

// ReleaseUploadSlot releases an upload slot
func (p *PiOptimizer) ReleaseUploadSlot() {
	p.uploadSem.Release(1)
}

// AcquireHashSlot gets permission to hash (for CPU-bound operations)
func (p *PiOptimizer) AcquireHashSlot(ctx context.Context) error {
	return p.hashSem.Acquire(ctx, 1)
}

// ReleaseHashSlot releases a hash slot
func (p *PiOptimizer) ReleaseHashSlot() {
	p.hashSem.Release(1)
}

// CheckMemoryPressure returns true if memory usage is high
func (p *PiOptimizer) CheckMemoryPressure() bool {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	usedMB := int(m.Alloc / 1024 / 1024)
	threshold := int(float64(p.maxMemoryMB) * 0.8) // 80% threshold
	
	if usedMB > threshold {
		// Force GC if under pressure
		runtime.GC()
		return true
	}
	
	return false
}

// OptimizeForPi applies all Pi-specific optimizations
func OptimizeForPi() {
	// Set GOMAXPROCS based on CPU
	cpuCount := runtime.NumCPU()
	switch {
	case cpuCount <= 2:
		runtime.GOMAXPROCS(cpuCount)
	case cpuCount == 4:
		runtime.GOMAXPROCS(3) // Leave 1 core for OS
	default:
		runtime.GOMAXPROCS(cpuCount - 1)
	}
	
	// Tune GC for Pi
	debug.SetGCPercent(50) // More frequent GC
	
	// Set memory limit based on Pi model
	var memLimit int64
	switch cpuCount {
	case 1:
		memLimit = 256 * 1024 * 1024 // 256MB for Pi Zero
	case 4:
		memLimit = 768 * 1024 * 1024 // 768MB for Pi 3/4
	default:
		memLimit = 1536 * 1024 * 1024 // 1.5GB for Pi 5
	}
	debug.SetMemoryLimit(memLimit)
}

// GetPiStats returns Pi-specific runtime statistics
func GetPiStats() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	return map[string]interface{}{
		"cpu_cores":       runtime.NumCPU(),
		"gomaxprocs":      runtime.GOMAXPROCS(0),
		"goroutines":      runtime.NumGoroutine(),
		"memory_alloc_mb": m.Alloc / 1024 / 1024,
		"memory_sys_mb":   m.Sys / 1024 / 1024,
		"gc_runs":         m.NumGC,
		"gc_pause_ms":     m.PauseNs[(m.NumGC+255)%256] / 1000000,
	}
}