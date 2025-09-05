package optimization

import (
	"bytes"
	"context"
	"sync"
	"time"
)

// BufferPool provides efficient buffer pooling for file processing
type BufferPool struct {
	pool           sync.Pool
	bufferSize     int
	allocatedCount int64
	reuseCount     int64
	mu             sync.RWMutex
}

// NewBufferPool creates a new buffer pool with the specified buffer size
func NewBufferPool(bufferSize int) *BufferPool {
	bp := &BufferPool{
		bufferSize: bufferSize,
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, bufferSize)
			},
		},
	}
	return bp
}

// Get retrieves a buffer from the pool
func (bp *BufferPool) Get() []byte {
	bp.mu.Lock()
	bp.reuseCount++
	bp.mu.Unlock()

	buffer := bp.pool.Get().([]byte)
	// Reset buffer content for security
	for i := range buffer {
		buffer[i] = 0
	}
	return buffer[:cap(buffer)]
}

// Put returns a buffer to the pool
func (bp *BufferPool) Put(buffer []byte) {
	if len(buffer) != bp.bufferSize {
		return // Only accept buffers of the correct size
	}
	bp.pool.Put(buffer)
}

// GetStats returns pool statistics
func (bp *BufferPool) GetStats() BufferPoolStats {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	return BufferPoolStats{
		BufferSize:     bp.bufferSize,
		AllocatedCount: bp.allocatedCount,
		ReuseCount:     bp.reuseCount,
	}
}

// BufferPoolStats provides statistics about buffer pool usage
type BufferPoolStats struct {
	BufferSize     int   `json:"buffer_size"`
	AllocatedCount int64 `json:"allocated_count"`
	ReuseCount     int64 `json:"reuse_count"`
}

// ObjectPools contains all object pools used throughout the application
type ObjectPools struct {
	// Buffer pools for different use cases
	SmallBuffers  *BufferPool // 4KB buffers
	MediumBuffers *BufferPool // 32KB buffers
	LargeBuffers  *BufferPool // 256KB buffers
	HugeBuffers   *BufferPool // 1MB buffers

	// Byte buffer pool for JSON marshaling
	ByteBuffers sync.Pool

	// Context pools for request handling
	ContextPool sync.Pool

	// String builders for efficient string concatenation
	StringBuilders sync.Pool

	// Map pools for metadata handling
	MapPools sync.Pool

	mu sync.RWMutex
}

// NewObjectPools creates and initializes all object pools
func NewObjectPools() *ObjectPools {
	pools := &ObjectPools{
		SmallBuffers:  NewBufferPool(4 * 1024),    // 4KB
		MediumBuffers: NewBufferPool(32 * 1024),   // 32KB
		LargeBuffers:  NewBufferPool(256 * 1024),  // 256KB
		HugeBuffers:   NewBufferPool(1024 * 1024), // 1MB

		ByteBuffers: sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},

		ContextPool: sync.Pool{
			New: func() interface{} {
				return context.Background()
			},
		},

		StringBuilders: sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},

		MapPools: sync.Pool{
			New: func() interface{} {
				return make(map[string]interface{}, 10)
			},
		},
	}

	return pools
}

// GetBuffer returns an appropriate buffer based on size requirements
func (op *ObjectPools) GetBuffer(sizeHint int) ([]byte, func()) {
	var pool *BufferPool

	switch {
	case sizeHint <= 4*1024:
		pool = op.SmallBuffers
	case sizeHint <= 32*1024:
		pool = op.MediumBuffers
	case sizeHint <= 256*1024:
		pool = op.LargeBuffers
	default:
		pool = op.HugeBuffers
	}

	buffer := pool.Get()
	releaseFunc := func() {
		pool.Put(buffer)
	}

	return buffer, releaseFunc
}

// GetByteBuffer returns a bytes.Buffer from the pool
func (op *ObjectPools) GetByteBuffer() (*bytes.Buffer, func()) {
	buf := op.ByteBuffers.Get().(*bytes.Buffer)
	buf.Reset()

	releaseFunc := func() {
		if buf.Cap() > 64*1024 { // Don't keep buffers larger than 64KB
			return
		}
		buf.Reset()
		op.ByteBuffers.Put(buf)
	}

	return buf, releaseFunc
}

// GetStringBuilder returns a string builder from the pool
func (op *ObjectPools) GetStringBuilder() (*bytes.Buffer, func()) {
	buf := op.StringBuilders.Get().(*bytes.Buffer)
	buf.Reset()

	releaseFunc := func() {
		if buf.Cap() > 32*1024 { // Don't keep builders larger than 32KB
			return
		}
		buf.Reset()
		op.StringBuilders.Put(buf)
	}

	return buf, releaseFunc
}

// GetMap returns a map from the pool
func (op *ObjectPools) GetMap() (map[string]interface{}, func()) {
	m := op.MapPools.Get().(map[string]interface{})

	// Clear the map
	for k := range m {
		delete(m, k)
	}

	releaseFunc := func() {
		if len(m) > 50 { // Don't keep large maps
			return
		}
		// Clear the map before returning
		for k := range m {
			delete(m, k)
		}
		op.MapPools.Put(m)
	}

	return m, releaseFunc
}

// GetContextWithTimeout returns a context with timeout from the pool
func (op *ObjectPools) GetContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// GetAllStats returns statistics for all pools
func (op *ObjectPools) GetAllStats() ObjectPoolsStats {
	op.mu.RLock()
	defer op.mu.RUnlock()

	return ObjectPoolsStats{
		SmallBuffers:  op.SmallBuffers.GetStats(),
		MediumBuffers: op.MediumBuffers.GetStats(),
		LargeBuffers:  op.LargeBuffers.GetStats(),
		HugeBuffers:   op.HugeBuffers.GetStats(),
	}
}

// ObjectPoolsStats provides statistics for all object pools
type ObjectPoolsStats struct {
	SmallBuffers  BufferPoolStats `json:"small_buffers"`
	MediumBuffers BufferPoolStats `json:"medium_buffers"`
	LargeBuffers  BufferPoolStats `json:"large_buffers"`
	HugeBuffers   BufferPoolStats `json:"huge_buffers"`
}

// Global pool instance - initialized once per application
var GlobalPools *ObjectPools

// InitGlobalPools initializes the global object pools
func InitGlobalPools() {
	if GlobalPools == nil {
		GlobalPools = NewObjectPools()
	}
}

// GetGlobalPools returns the global object pools instance
func GetGlobalPools() *ObjectPools {
	if GlobalPools == nil {
		InitGlobalPools()
	}
	return GlobalPools
}
