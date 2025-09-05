package optimization

import (
	"io"
	"sync"
)

// ObjectPools manages reusable object pools for memory optimization
type ObjectPools struct {
	bufferPools map[int]*BufferPool
	mu          sync.RWMutex
}

// BufferPool manages a pool of byte buffers for efficient memory reuse
type BufferPool struct {
	pool sync.Pool
	size int
}

// StreamingHasher provides optimized streaming hash calculation
type StreamingHasher struct {
	hasher interface {
		Write([]byte) (int, error)
		Sum([]byte) []byte
		Reset()
	}
}

// Global pools instance
var globalPools *ObjectPools
var poolsOnce sync.Once

// GetGlobalPools returns the global object pools instance
func GetGlobalPools() *ObjectPools {
	poolsOnce.Do(func() {
		globalPools = &ObjectPools{
			bufferPools: make(map[int]*BufferPool),
		}
	})
	return globalPools
}

// NewObjectPools creates a new ObjectPools instance
func NewObjectPools() *ObjectPools {
	return &ObjectPools{
		bufferPools: make(map[int]*BufferPool),
	}
}

// GetBuffer gets a buffer from the pool with the specified size
func (op *ObjectPools) GetBuffer(size int) ([]byte, func()) {
	op.mu.RLock()
	pool, exists := op.bufferPools[size]
	op.mu.RUnlock()

	if !exists {
		op.mu.Lock()
		// Double-check after acquiring write lock
		if pool, exists = op.bufferPools[size]; !exists {
			pool = &BufferPool{
				size: size,
				pool: sync.Pool{
					New: func() interface{} {
						return make([]byte, size)
					},
				},
			}
			op.bufferPools[size] = pool
		}
		op.mu.Unlock()
	}

	buffer := pool.pool.Get().([]byte)
	releaseFunc := func() {
		pool.pool.Put(buffer[:size]) // Reset slice length
	}

	return buffer, releaseFunc
}

// NewBufferPool creates a new buffer pool for the specified size
func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		size: size,
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, size)
			},
		},
	}
}

// Get retrieves a buffer from the pool
func (bp *BufferPool) Get() []byte {
	return bp.pool.Get().([]byte)
}

// Put returns a buffer to the pool
func (bp *BufferPool) Put(buffer []byte) {
	if len(buffer) >= bp.size {
		bp.pool.Put(buffer[:bp.size])
	}
}

// NewStreamingHasher creates a new streaming hasher
func NewStreamingHasher() *StreamingHasher {
	// For now, we'll use a simple implementation
	// In the future, this could be optimized with hardware acceleration
	return &StreamingHasher{
		hasher: &simpleHasher{},
	}
}

// Write writes data to the hasher
func (sh *StreamingHasher) Write(data []byte) (int, error) {
	return sh.hasher.Write(data)
}

// Sum returns the final hash
func (sh *StreamingHasher) Sum() string {
	hash := sh.hasher.Sum(nil)
	return string(hash) // Simplified for now
}

// Reset resets the hasher state
func (sh *StreamingHasher) Reset() {
	sh.hasher.Reset()
}

// simpleHasher is a placeholder hasher implementation
type simpleHasher struct {
	data []byte
}

func (h *simpleHasher) Write(p []byte) (n int, err error) {
	h.data = append(h.data, p...)
	return len(p), nil
}

func (h *simpleHasher) Sum(b []byte) []byte {
	// Simple sum for demonstration - in real implementation use crypto/sha256
	sum := byte(0)
	for _, v := range h.data {
		sum ^= v
	}
	result := make([]byte, len(b)+32) // SHA256 size
	copy(result, b)
	for i := len(b); i < len(result); i++ {
		result[i] = sum
	}
	return result
}

func (h *simpleHasher) Reset() {
	h.data = h.data[:0]
}

// GetStats returns statistics about the object pools
func (op *ObjectPools) GetStats() map[string]interface{} {
	op.mu.RLock()
	defer op.mu.RUnlock()

	poolCount := len(op.bufferPools)
	poolSizes := make([]int, 0, poolCount)
	for size := range op.bufferPools {
		poolSizes = append(poolSizes, size)
	}

	return map[string]interface{}{
		"pool_count":  poolCount,
		"pool_sizes":  poolSizes,
		"total_pools": poolCount,
	}
}

// GetAllStats returns all statistics as ObjectPoolsStats
func (op *ObjectPools) GetAllStats() ObjectPoolsStats {
	stats := op.GetStats()
	return ObjectPoolsStats{
		PoolCount:  stats["pool_count"].(int),
		PoolSizes:  stats["pool_sizes"].([]int),
		TotalPools: stats["total_pools"].(int),
	}
}

// GetByteBuffer returns a byte buffer from the pool
func (op *ObjectPools) GetByteBuffer(size int) ([]byte, func()) {
	return op.GetBuffer(size)
}

// ObjectPoolsStats represents statistics about object pools
type ObjectPoolsStats struct {
	PoolCount  int   `json:"pool_count"`
	PoolSizes  []int `json:"pool_sizes"`
	TotalPools int   `json:"total_pools"`
}

// StreamingCopier handles optimized copying for streaming
type StreamingCopier struct {
	bufferSize int
	pools      *ObjectPools
}

// NewStreamingCopier creates a new streaming copier
func NewStreamingCopier(bufferSize int, pools *ObjectPools) *StreamingCopier {
	return &StreamingCopier{
		bufferSize: bufferSize,
		pools:      pools,
	}
}

// StreamingReader handles optimized reading for streaming with progress tracking
type StreamingReader struct {
	reader     io.Reader
	totalSize  int64
	bytesRead  int64
	onProgress func(bytesRead, totalSize int64)
}

// NewStreamingReader creates a new streaming reader with progress tracking
func NewStreamingReader(reader io.Reader, totalSize int64, onProgress func(bytesRead, totalSize int64)) io.Reader {
	return &StreamingReader{
		reader:     reader,
		totalSize:  totalSize,
		onProgress: onProgress,
	}
}

// Read implements io.Reader interface with progress tracking
func (sr *StreamingReader) Read(p []byte) (int, error) {
	n, err := sr.reader.Read(p)
	if n > 0 {
		sr.bytesRead += int64(n)
		if sr.onProgress != nil {
			sr.onProgress(sr.bytesRead, sr.totalSize)
		}
	}
	return n, err
}
