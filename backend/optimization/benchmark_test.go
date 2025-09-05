package optimization

import (
	"bytes"
	cryptoRand "crypto/rand"
	"io"
	"runtime"
	"testing"
)

// BenchmarkBufferPool benchmarks buffer pool performance
func BenchmarkBufferPool(b *testing.B) {
	pool := NewBufferPool(32 * 1024) // 32KB buffers

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buffer := pool.Get()
			// Simulate work
			for i := 0; i < len(buffer); i += 1024 {
				buffer[i] = byte(i)
			}
			pool.Put(buffer)
		}
	})
}

// BenchmarkBufferPoolVsAllocation benchmarks pool vs direct allocation
func BenchmarkBufferPoolVsAllocation(b *testing.B) {
	bufferSize := 32 * 1024

	b.Run("BufferPool", func(b *testing.B) {
		pool := NewBufferPool(bufferSize)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			buffer := pool.Get()
			// Simulate work
			buffer[0] = 1
			buffer[len(buffer)-1] = 1
			pool.Put(buffer)
		}
	})

	b.Run("DirectAllocation", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			buffer := make([]byte, bufferSize)
			// Simulate work
			buffer[0] = 1
			buffer[len(buffer)-1] = 1
		}
	})
}

// BenchmarkStreamingHasher benchmarks streaming hash calculation
func BenchmarkStreamingHasher(b *testing.B) {
	data := make([]byte, 1024*1024) // 1MB test data
	cryptoRand.Read(data)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			hasher := NewStreamingHasher()
			reader := bytes.NewReader(data)

			buffer := make([]byte, 32*1024) // 32KB buffer
			for {
				n, err := reader.Read(buffer)
				if n > 0 {
					hasher.Write(buffer[:n])
				}
				if err == io.EOF {
					break
				}
			}

			_ = hasher.Sum()
		}
	})

	b.ReportMetric(float64(len(data)*b.N)/b.Elapsed().Seconds()/1024/1024, "MB/s")
}

// BenchmarkObjectPools benchmarks object pool performance
func BenchmarkObjectPools(b *testing.B) {
	InitGlobalPools()
	pools := GetGlobalPools()

	b.Run("ByteBuffer", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buf, release := pools.GetByteBuffer()
				buf.WriteString("test data for benchmark")
				_ = buf.String()
				release()
			}
		})
	})

	b.Run("Map", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				m, release := pools.GetMap()
				m["key1"] = "value1"
				m["key2"] = "value2"
				_ = len(m)
				release()
			}
		})
	})
}

// BenchmarkStreamingCopier benchmarks streaming copy performance
func BenchmarkStreamingCopier(b *testing.B) {
	InitGlobalPools()
	pools := GetGlobalPools()
	copier := NewStreamingCopier(32*1024, pools)

	data := make([]byte, 1024*1024) // 1MB test data
	cryptoRand.Read(data)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			src := bytes.NewReader(data)
			dst := &bytes.Buffer{}

			_, err := copier.Copy(dst, src)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.ReportMetric(float64(len(data)*b.N)/b.Elapsed().Seconds()/1024/1024, "MB/s")
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("WithPools", func(b *testing.B) {
		InitGlobalPools()
		pools := GetGlobalPools()

		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buffer, release := pools.GetBuffer(32 * 1024)
			// Simulate work
			buffer[0] = 1
			release()

			buf, releaseBuf := pools.GetByteBuffer()
			buf.WriteString("test data")
			releaseBuf()
		}
		b.StopTimer()

		runtime.GC()
		runtime.ReadMemStats(&m2)

		b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "bytes/op")
		b.ReportMetric(float64(m2.Mallocs-m1.Mallocs)/float64(b.N), "allocs/op")
	})

	b.Run("WithoutPools", func(b *testing.B) {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buffer := make([]byte, 32*1024)
			// Simulate work
			buffer[0] = 1

			buf := &bytes.Buffer{}
			buf.WriteString("test data")
		}
		b.StopTimer()

		runtime.GC()
		runtime.ReadMemStats(&m2)

		b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "bytes/op")
		b.ReportMetric(float64(m2.Mallocs-m1.Mallocs)/float64(b.N), "allocs/op")
	})
}

// TestBufferPoolCorrectness tests buffer pool correctness
func TestBufferPoolCorrectness(t *testing.T) {
	pool := NewBufferPool(1024)

	// Test get/put cycle
	buffer1 := pool.Get()
	if len(buffer1) != 1024 {
		t.Errorf("Expected buffer size 1024, got %d", len(buffer1))
	}

	// Write data
	buffer1[0] = 0xFF
	buffer1[1023] = 0xFF

	pool.Put(buffer1)

	// Get another buffer - should be cleared
	buffer2 := pool.Get()
	if buffer2[0] != 0 || buffer2[1023] != 0 {
		t.Error("Buffer was not cleared properly")
	}

	pool.Put(buffer2)

	// Test statistics
	stats := pool.GetStats()
	if stats.BufferSize != 1024 {
		t.Errorf("Expected buffer size 1024 in stats, got %d", stats.BufferSize)
	}
}

// TestStreamingHasherCorrectness tests streaming hasher correctness
func TestStreamingHasherCorrectness(t *testing.T) {
	data := []byte("test data for hashing")

	hasher := NewStreamingHasher()
	hasher.Write(data)
	hash1 := hasher.Sum()

	// Hash same data again
	hasher.Reset()
	hasher.Write(data)
	hash2 := hasher.Sum()

	if hash1 != hash2 {
		t.Error("Hashes should be identical for same data")
	}

	// Hash different data
	hasher.Reset()
	hasher.Write([]byte("different data"))
	hash3 := hasher.Sum()

	if hash1 == hash3 {
		t.Error("Hashes should be different for different data")
	}
}

// TestObjectPoolsCorrectness tests object pools correctness
func TestObjectPoolsCorrectness(t *testing.T) {
	InitGlobalPools()
	pools := GetGlobalPools()

	// Test buffer pool
	buffer, release := pools.GetBuffer(1024)
	if len(buffer) < 1024 {
		t.Errorf("Expected buffer size >= 1024, got %d", len(buffer))
	}
	release()

	// Test byte buffer pool
	buf, releaseBuf := pools.GetByteBuffer()
	buf.WriteString("test")
	if buf.String() != "test" {
		t.Error("Byte buffer content incorrect")
	}
	releaseBuf()

	// Test map pool
	m, releaseMap := pools.GetMap()
	m["key"] = "value"
	if m["key"] != "value" {
		t.Error("Map content incorrect")
	}
	releaseMap()
}

// BenchmarkPiOptimizations benchmarks Pi-specific optimizations
func BenchmarkPiOptimizations(b *testing.B) {
	// Simulate Pi constraints
	originalMaxProcs := runtime.GOMAXPROCS(0)
	runtime.GOMAXPROCS(4) // Pi 4/5 has 4 cores
	defer runtime.GOMAXPROCS(originalMaxProcs)

	InitGlobalPools()
	pools := GetGlobalPools()

	b.Run("ConcurrentFileProcessing", func(b *testing.B) {
		data := make([]byte, 1024*1024) // 1MB file
		cryptoRand.Read(data)

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// Simulate file hash calculation
				hasher := NewStreamingHasher()
				buffer, release := pools.GetBuffer(32 * 1024)

				reader := bytes.NewReader(data)
				for {
					n, err := reader.Read(buffer)
					if n > 0 {
						hasher.Write(buffer[:n])
					}
					if err == io.EOF {
						break
					}
				}

				_ = hasher.Sum()
				release()
			}
		})
	})
}

// TestMemoryConstraints tests memory usage under Pi constraints
func TestMemoryConstraints(t *testing.T) {
	InitGlobalPools()
	pools := GetGlobalPools()

	// Simulate memory pressure
	const maxMemoryMB = 100 // Simulate 100MB limit

	var totalAllocated int64

	for i := 0; i < 1000; i++ {
		buffer, release := pools.GetBuffer(1024)
		totalAllocated += int64(len(buffer))

		if totalAllocated > maxMemoryMB*1024*1024 {
			t.Logf("Memory usage: %d MB", totalAllocated/1024/1024)
		}

		release()
	}

	// Verify pool statistics
	stats := pools.GetAllStats()
	t.Logf("Pool stats: %+v", stats)
}
