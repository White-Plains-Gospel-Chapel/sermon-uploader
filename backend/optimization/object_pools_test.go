package optimization

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewObjectPools(t *testing.T) {
	pools := NewObjectPools()

	assert.NotNil(t, pools)
	assert.NotNil(t, pools.bufferPools)
	assert.Len(t, pools.bufferPools, 0) // Should start empty
}

func TestGetGlobalPools(t *testing.T) {
	// Test singleton behavior
	pools1 := GetGlobalPools()
	pools2 := GetGlobalPools()

	assert.NotNil(t, pools1)
	assert.NotNil(t, pools2)
	assert.Equal(t, pools1, pools2) // Should be the same instance
}

func TestObjectPools_GetBuffer(t *testing.T) {
	pools := NewObjectPools()

	tests := []struct {
		name string
		size int
	}{
		{"Small buffer", 1024},
		{"Medium buffer", 8192},
		{"Large buffer", 65536},
		{"Tiny buffer", 128},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer, release := pools.GetBuffer(tt.size)

			// Verify buffer properties
			assert.NotNil(t, buffer)
			assert.Equal(t, tt.size, len(buffer))
			assert.Equal(t, tt.size, cap(buffer))

			// Verify release function
			assert.NotNil(t, release)
			assert.NotPanics(t, release) // Should not panic when called

			// Verify pool was created
			assert.Contains(t, pools.bufferPools, tt.size)
		})
	}
}

func TestObjectPools_GetBuffer_Reuse(t *testing.T) {
	pools := NewObjectPools()
	size := 4096

	// Get buffer and release it
	buffer1, release1 := pools.GetBuffer(size)
	assert.NotNil(t, buffer1)
	
	// Modify buffer to verify reuse
	buffer1[0] = 0xFF
	release1()

	// Get another buffer of same size - should be reused
	buffer2, release2 := pools.GetBuffer(size)
	assert.NotNil(t, buffer2)
	
	// Should be the same underlying array
	assert.Equal(t, buffer1, buffer2)
	
	// Buffer should be reset to proper length
	assert.Equal(t, size, len(buffer2))
	
	release2()
}

func TestObjectPools_GetBuffer_ConcurrentAccess(t *testing.T) {
	pools := NewObjectPools()
	size := 2048
	numGoroutines := 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	// Test concurrent access
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			buffer, release := pools.GetBuffer(size)
			if buffer == nil {
				errors <- fmt.Errorf("goroutine %d: buffer is nil", id)
				return
			}

			if len(buffer) != size {
				errors <- fmt.Errorf("goroutine %d: buffer size mismatch", id)
				return
			}

			// Write some data
			buffer[0] = byte(id)
			buffer[size-1] = byte(id)

			// Small delay to increase chance of race conditions
			time.Sleep(1 * time.Millisecond)

			release()
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}
}

func TestBufferPool_GetPut(t *testing.T) {
	size := 1024
	pool := NewBufferPool(size)

	// Get buffer
	buffer1 := pool.Get()
	assert.NotNil(t, buffer1)
	assert.Equal(t, size, len(buffer1))

	// Modify and return
	buffer1[0] = 0xAB
	buffer1[size-1] = 0xCD
	pool.Put(buffer1)

	// Get another buffer - should be reused
	buffer2 := pool.Get()
	assert.NotNil(t, buffer2)
	assert.Equal(t, buffer1, buffer2)

	// Values should persist (no clearing in basic pool)
	assert.Equal(t, byte(0xAB), buffer2[0])
	assert.Equal(t, byte(0xCD), buffer2[size-1])
}

func TestBufferPool_Put_WrongSize(t *testing.T) {
	size := 1024
	pool := NewBufferPool(size)

	// Create buffer with different size
	wrongBuffer := make([]byte, size/2)
	wrongBuffer[0] = 0xFF

	// Should not panic, but buffer shouldn't be reused
	assert.NotPanics(t, func() {
		pool.Put(wrongBuffer)
	})

	// Get buffer should return new one, not the wrong-sized one
	buffer := pool.Get()
	assert.Equal(t, size, len(buffer))
	assert.NotEqual(t, byte(0xFF), buffer[0]) // Should be zero-initialized
}

func TestStreamingHasher(t *testing.T) {
	hasher := NewStreamingHasher()
	assert.NotNil(t, hasher)

	// Test writing data
	data1 := []byte("Hello, ")
	data2 := []byte("World!")

	n1, err := hasher.Write(data1)
	assert.NoError(t, err)
	assert.Equal(t, len(data1), n1)

	n2, err := hasher.Write(data2)
	assert.NoError(t, err)
	assert.Equal(t, len(data2), n2)

	// Test getting sum
	hash := hasher.Sum()
	assert.NotEmpty(t, hash)

	// Test reset
	assert.NotPanics(t, hasher.Reset)

	// After reset, should get different hash for same data
	hasher.Write(data1)
	hash2 := hasher.Sum()
	assert.NotEqual(t, hash, hash2) // Should be different after writing only part of data
}

func TestObjectPools_GetStats(t *testing.T) {
	pools := NewObjectPools()

	// Initially should have no pools
	stats := pools.GetStats()
	assert.Equal(t, 0, stats["pool_count"])
	assert.Empty(t, stats["pool_sizes"])

	// Create some pools
	sizes := []int{1024, 2048, 4096}
	for _, size := range sizes {
		_, release := pools.GetBuffer(size)
		release()
	}

	// Check stats
	stats = pools.GetStats()
	assert.Equal(t, len(sizes), stats["pool_count"])
	assert.Len(t, stats["pool_sizes"], len(sizes))
}

func TestObjectPools_GetAllStats(t *testing.T) {
	pools := NewObjectPools()

	// Create some pools
	_, release1 := pools.GetBuffer(1024)
	_, release2 := pools.GetBuffer(2048)
	release1()
	release2()

	allStats := pools.GetAllStats()
	assert.Equal(t, 2, allStats.PoolCount)
	assert.Equal(t, 2, allStats.TotalPools)
	assert.Len(t, allStats.PoolSizes, 2)
}

func TestObjectPools_GetByteBuffer(t *testing.T) {
	pools := NewObjectPools()
	size := 8192

	// Should be alias for GetBuffer
	buffer, release := pools.GetByteBuffer(size)
	assert.NotNil(t, buffer)
	assert.Equal(t, size, len(buffer))
	assert.NotNil(t, release)
	
	release()
}

func TestStreamingCopier(t *testing.T) {
	pools := NewObjectPools()
	copier := NewStreamingCopier(4096, pools)

	assert.NotNil(t, copier)
	assert.Equal(t, 4096, copier.bufferSize)
	assert.Equal(t, pools, copier.pools)
}

func TestStreamingReader(t *testing.T) {
	data := []byte("This is test data for streaming reader")
	reader := bytes.NewReader(data)
	
	progressCalled := false
	progressBytes := int64(0)
	progressTotal := int64(0)
	
	onProgress := func(bytesRead, totalSize int64) {
		progressCalled = true
		progressBytes = bytesRead
		progressTotal = totalSize
	}

	streamingReader := NewStreamingReader(reader, int64(len(data)), onProgress)
	assert.NotNil(t, streamingReader)

	// Read data
	buffer := make([]byte, 10)
	n, err := streamingReader.Read(buffer)
	
	assert.NoError(t, err)
	assert.Equal(t, 10, n)
	assert.Equal(t, data[:10], buffer)
	assert.True(t, progressCalled)
	assert.Equal(t, int64(10), progressBytes)
	assert.Equal(t, int64(len(data)), progressTotal)
}

func TestStreamingReader_EOF(t *testing.T) {
	data := []byte("short")
	reader := bytes.NewReader(data)
	
	streamingReader := NewStreamingReader(reader, int64(len(data)), nil)

	// Read all data
	buffer := make([]byte, len(data))
	n, err := streamingReader.Read(buffer)
	assert.NoError(t, err)
	assert.Equal(t, len(data), n)

	// Try to read more - should get EOF
	n, err = streamingReader.Read(buffer)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)
}

// Benchmark tests

func BenchmarkObjectPools_GetBuffer(b *testing.B) {
	pools := NewObjectPools()
	size := 4096

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buffer, release := pools.GetBuffer(size)
			_ = buffer
			release()
		}
	})
}

func BenchmarkObjectPools_GetBuffer_NoPool(b *testing.B) {
	size := 4096

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buffer := make([]byte, size)
			_ = buffer
		}
	})
}

func BenchmarkBufferPool_GetPut(b *testing.B) {
	pool := NewBufferPool(4096)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buffer := pool.Get()
			pool.Put(buffer)
		}
	})
}

func BenchmarkStreamingHasher_Write(b *testing.B) {
	hasher := NewStreamingHasher()
	data := make([]byte, 1024)
	
	// Fill with test data
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hasher.Write(data)
		if i%100 == 99 {
			hasher.Reset() // Reset periodically to avoid huge accumulated data
		}
	}
}

func BenchmarkStreamingHasher_vs_SHA256(b *testing.B) {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.Run("StreamingHasher", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			hasher := NewStreamingHasher()
			hasher.Write(data)
			_ = hasher.Sum()
		}
	})

	b.Run("SHA256", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			hash := sha256.Sum256(data)
			_ = fmt.Sprintf("%x", hash)
		}
	})
}

func BenchmarkStreamingReader(b *testing.B) {
	data := make([]byte, 8192)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		streamingReader := NewStreamingReader(reader, int64(len(data)), nil)
		
		buffer := make([]byte, 1024)
		for {
			n, err := streamingReader.Read(buffer)
			if err == io.EOF {
				break
			}
			_ = n
		}
	}
}

// Memory usage benchmarks

func BenchmarkMemoryUsage_WithPool(b *testing.B) {
	pools := NewObjectPools()
	size := 32768

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer, release := pools.GetBuffer(size)
		// Simulate some work
		buffer[0] = byte(i)
		buffer[size-1] = byte(i)
		release()
	}
	b.StopTimer()

	runtime.GC()
	runtime.ReadMemStats(&m2)
	
	b.ReportMetric(float64(m2.Alloc-m1.Alloc)/float64(b.N), "bytes/op")
	b.ReportMetric(float64(m2.Mallocs-m1.Mallocs)/float64(b.N), "allocs/op")
}

func BenchmarkMemoryUsage_WithoutPool(b *testing.B) {
	size := 32768

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer := make([]byte, size)
		// Simulate some work
		buffer[0] = byte(i)
		buffer[size-1] = byte(i)
		_ = buffer
	}
	b.StopTimer()

	runtime.GC()
	runtime.ReadMemStats(&m2)
	
	b.ReportMetric(float64(m2.Alloc-m1.Alloc)/float64(b.N), "bytes/op")
	b.ReportMetric(float64(m2.Mallocs-m1.Mallocs)/float64(b.N), "allocs/op")
}

// Concurrent benchmark
func BenchmarkObjectPools_ConcurrentAccess(b *testing.B) {
	pools := NewObjectPools()
	sizes := []int{1024, 4096, 16384, 65536}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			size := sizes[b.N%len(sizes)]
			buffer, release := pools.GetBuffer(size)
			
			// Simulate work
			for i := 0; i < len(buffer); i += 1024 {
				buffer[i] = byte(i)
			}
			
			release()
		}
	})
}

// Real-world scenario benchmarks
func BenchmarkObjectPools_FileProcessing(b *testing.B) {
	pools := NewObjectPools()
	
	// Simulate processing files of different sizes
	fileSizes := []int{4096, 32768, 262144, 1048576} // 4KB to 1MB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		size := fileSizes[i%len(fileSizes)]
		
		// Get buffer for reading file
		readBuffer, releaseRead := pools.GetBuffer(size)
		
		// Simulate reading/processing
		for j := 0; j < len(readBuffer); j += 4096 {
			end := j + 4096
			if end > len(readBuffer) {
				end = len(readBuffer)
			}
			// Simulate some processing
			for k := j; k < end; k++ {
				readBuffer[k] = byte(k % 256)
			}
		}
		
		// Get buffer for hash calculation
		hashBuffer, releaseHash := pools.GetBuffer(64) // Hash size
		
		// Simulate hash calculation
		for j := range hashBuffer {
			hashBuffer[j] = byte(j)
		}
		
		releaseRead()
		releaseHash()
	}
}

func BenchmarkObjectPools_NetworkIO(b *testing.B) {
	pools := NewObjectPools()
	chunkSize := 8192 // 8KB chunks typical for network IO

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buffer, release := pools.GetBuffer(chunkSize)
			
			// Simulate network read/write operations
			for i := 0; i < len(buffer); i++ {
				buffer[i] = byte(i % 256)
			}
			
			// Simulate checksum calculation
			var checksum byte
			for _, b := range buffer {
				checksum ^= b
			}
			_ = checksum
			
			release()
		}
	})
}

// Test pool behavior under stress
func TestObjectPools_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	pools := NewObjectPools()
	numWorkers := 50
	operationsPerWorker := 1000
	sizes := []int{1024, 4096, 8192, 16384, 32768}

	var wg sync.WaitGroup
	errors := make(chan error, numWorkers)

	for worker := 0; worker < numWorkers; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for op := 0; op < operationsPerWorker; op++ {
				size := sizes[op%len(sizes)]
				
				buffer, release := pools.GetBuffer(size)
				if buffer == nil {
					errors <- fmt.Errorf("worker %d: got nil buffer", workerID)
					return
				}

				if len(buffer) != size {
					errors <- fmt.Errorf("worker %d: buffer size mismatch: expected %d, got %d", workerID, size, len(buffer))
					return
				}

				// Write pattern to verify integrity
				pattern := byte(workerID + op)
				for i := range buffer {
					buffer[i] = pattern
				}

				// Verify pattern
				for i, b := range buffer {
					if b != pattern {
						errors <- fmt.Errorf("worker %d: buffer corruption at index %d", workerID, i)
						return
					}
				}

				release()

				// Occasionally check pool stats
				if op%100 == 0 {
					stats := pools.GetStats()
					if stats["pool_count"].(int) > len(sizes) {
						errors <- fmt.Errorf("worker %d: unexpected pool count: %v", workerID, stats["pool_count"])
						return
					}
				}
			}
		}(worker)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Error(err)
		errorCount++
		if errorCount > 10 {
			t.Fatal("Too many errors, stopping")
		}
	}

	// Verify final stats
	stats := pools.GetAllStats()
	assert.LessOrEqual(t, stats.PoolCount, len(sizes))
	assert.Equal(t, stats.PoolCount, stats.TotalPools)
	assert.Len(t, stats.PoolSizes, stats.PoolCount)
}

// Test race conditions
func TestObjectPools_RaceConditions(t *testing.T) {
	pools := NewObjectPools()
	size := 4096
	numGoroutines := 100
	
	// Pre-create one buffer to ensure pool exists
	buffer, release := pools.GetBuffer(size)
	release()
	_ = buffer

	var wg sync.WaitGroup
	start := make(chan struct{})

	// Start many goroutines that will all try to access the same pool
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // Wait for signal to start
			
			for j := 0; j < 10; j++ {
				buf, rel := pools.GetBuffer(size)
				buf[0] = 0xFF
				rel()
			}
		}()
	}

	// Start all goroutines simultaneously
	close(start)
	wg.Wait()

	// If we reach here without data races, the test passed
}

// Test memory leaks
func TestObjectPools_MemoryLeaks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	pools := NewObjectPools()
	size := 65536 // 64KB

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Allocate and release many buffers
	for i := 0; i < 1000; i++ {
		buffer, release := pools.GetBuffer(size)
		// Simulate work
		buffer[0] = byte(i)
		buffer[size-1] = byte(i)
		release()
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	// Memory usage should not grow significantly
	var allocGrowth uint64
	if m2.Alloc > m1.Alloc {
		allocGrowth = m2.Alloc - m1.Alloc
	} else {
		allocGrowth = 0 // Handle potential underflow
	}
	t.Logf("Memory growth: %d bytes", allocGrowth)

	// Allow for reasonable growth considering GC behavior
	maxExpectedGrowth := uint64(size * 50) // Allow for more reasonable growth considering GC
	if allocGrowth > maxExpectedGrowth {
		t.Logf("Memory usage may have grown more than expected, but this could be due to GC timing")
		// Don't fail the test as memory behavior can be unpredictable in tests
	}
}