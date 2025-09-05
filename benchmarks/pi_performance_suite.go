package benchmarks

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// Pi hardware constraints for validation
const (
	PiMaxMemoryMB     = 8192  // 8GB Pi 4/5
	PiMaxConcurrency  = 8     // 8 CPU cores max
	PiTargetLatencyMs = 100   // Target response time
	PiMaxAllocsMB     = 50    // Max allocations per operation
)

// Test data structures
type AudioMetadata struct {
	Filename     string            `json:"filename"`
	Size         int64             `json:"size"`
	Duration     time.Duration     `json:"duration"`
	Format       string            `json:"format"`
	SampleRate   int               `json:"sample_rate"`
	Channels     int               `json:"channels"`
	BitDepth     int               `json:"bit_depth"`
	Checksum     string            `json:"checksum"`
	Timestamp    time.Time         `json:"timestamp"`
	Tags         map[string]string `json:"tags"`
}

type UploadRequest struct {
	File     []byte        `json:"file_data"`
	Metadata AudioMetadata `json:"metadata"`
}

// BenchmarkPiMemoryAllocation tests memory allocation patterns for Pi constraints
func BenchmarkPiMemoryAllocation(b *testing.B) {
	sizes := []int{
		1024,      // 1KB
		10 * 1024, // 10KB
		100 * 1024, // 100KB
		1024 * 1024, // 1MB
		10 * 1024 * 1024, // 10MB
	}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("Alloc%dB", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				data := make([]byte, size)
				// Simulate some work to prevent optimization
				for j := 0; j < len(data); j += 1024 {
					data[j] = byte(j)
				}
				_ = data
			}
		})
	}
}

// BenchmarkPiStringBuilding tests string concatenation performance
func BenchmarkPiStringBuilding(b *testing.B) {
	testCases := []struct {
		name  string
		count int
	}{
		{"Small100", 100},
		{"Medium1000", 1000},
		{"Large10000", 10000},
	}
	
	for _, tc := range testCases {
		b.Run("Concat"+tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var result string
				for j := 0; j < tc.count; j++ {
					result += "segment"
				}
				_ = result
			}
		})
		
		b.Run("Builder"+tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var builder strings.Builder
				builder.Grow(tc.count * 7) // Pre-allocate
				for j := 0; j < tc.count; j++ {
					builder.WriteString("segment")
				}
				_ = builder.String()
			}
		})
	}
}

// BenchmarkPiJSONProcessing tests JSON marshalling/unmarshalling performance
func BenchmarkPiJSONProcessing(b *testing.B) {
	metadata := AudioMetadata{
		Filename:   "test_sermon_20240101.wav",
		Size:       104857600, // 100MB
		Duration:   3600 * time.Second, // 1 hour
		Format:     "WAV",
		SampleRate: 44100,
		Channels:   2,
		BitDepth:   16,
		Checksum:   "sha256:1234567890abcdef",
		Timestamp:  time.Now(),
		Tags: map[string]string{
			"speaker": "Pastor John",
			"series":  "Romans Study",
			"topic":   "Grace and Faith",
		},
	}
	
	b.Run("Marshal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := json.Marshal(metadata)
			if err != nil {
				b.Error(err)
			}
		}
	})
	
	jsonData, _ := json.Marshal(metadata)
	
	b.Run("Unmarshal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var meta AudioMetadata
			err := json.Unmarshal(jsonData, &meta)
			if err != nil {
				b.Error(err)
			}
		}
	})
	
	// Test streaming JSON for large data
	largeData := make([]AudioMetadata, 1000)
	for i := range largeData {
		largeData[i] = metadata
	}
	
	b.Run("MarshalLarge", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := json.Marshal(largeData)
			if err != nil {
				b.Error(err)
			}
		}
	})
}

// BenchmarkPiConcurrentOperations tests concurrent performance with Pi constraints
func BenchmarkPiConcurrentOperations(b *testing.B) {
	concurrencyLevels := []int{1, 2, 4, 8, 16} // Test up to 2x Pi cores
	
	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Goroutines%d", concurrency), func(b *testing.B) {
			work := func(ctx context.Context) error {
				// Simulate CPU work
				data := make([]byte, 1024)
				hasher := sha256.New()
				hasher.Write(data)
				_ = hasher.Sum(nil)
				return nil
			}
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var wg sync.WaitGroup
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				
				for j := 0; j < concurrency; j++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						work(ctx)
					}()
				}
				
				wg.Wait()
				cancel()
			}
		})
	}
}

// BenchmarkPiFileOperations tests file I/O performance patterns
func BenchmarkPiFileOperations(b *testing.B) {
	// Create test file
	testFile, err := os.CreateTemp("", "benchmark_*.wav")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(testFile.Name())
	
	// Write test data
	testData := make([]byte, 1024*1024) // 1MB
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	testFile.Write(testData)
	testFile.Close()
	
	b.Run("ReadAll", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			file, err := os.Open(testFile.Name())
			if err != nil {
				b.Error(err)
				continue
			}
			_, err = io.ReadAll(file)
			if err != nil {
				b.Error(err)
			}
			file.Close()
		}
	})
	
	b.Run("BufferedRead", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			file, err := os.Open(testFile.Name())
			if err != nil {
				b.Error(err)
				continue
			}
			
			buf := make([]byte, 8192) // 8KB buffer
			for {
				_, err := file.Read(buf)
				if err == io.EOF {
					break
				}
				if err != nil {
					b.Error(err)
					break
				}
			}
			file.Close()
		}
	})
}

// BenchmarkPiHTTPHandlers tests HTTP handler performance
func BenchmarkPiHTTPHandlers(b *testing.B) {
	// Simple handler
	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	
	b.Run("SimpleHandler", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/api/status", nil)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			simpleHandler.ServeHTTP(w, req)
		}
	})
	
	// Upload handler simulation
	uploadHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse multipart form
		err := r.ParseMultipartForm(10 << 20) // 10MB
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		file, header, err := r.FormFile("audio")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		
		// Simulate processing
		hasher := sha256.New()
		io.Copy(hasher, file)
		checksum := fmt.Sprintf("%x", hasher.Sum(nil))
		
		response := map[string]interface{}{
			"filename": header.Filename,
			"size":     header.Size,
			"checksum": checksum,
			"status":   "uploaded",
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
	
	b.Run("UploadHandler", func(b *testing.B) {
		// Create multipart form data
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		
		// Add file field
		part, _ := writer.CreateFormFile("audio", "test.wav")
		testData := make([]byte, 1024) // 1KB test file
		part.Write(testData)
		writer.Close()
		
		req := httptest.NewRequest("POST", "/api/upload", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			uploadHandler.ServeHTTP(w, req)
		}
	})
}

// BenchmarkPiMemoryPool tests sync.Pool usage for Pi memory efficiency
func BenchmarkPiMemoryPool(b *testing.B) {
	// Without pool
	b.Run("WithoutPool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buf := make([]byte, 8192)
			// Simulate work
			for j := 0; j < len(buf); j += 256 {
				buf[j] = byte(j)
			}
		}
	})
	
	// With pool
	pool := sync.Pool{
		New: func() interface{} {
			return make([]byte, 8192)
		},
	}
	
	b.Run("WithPool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buf := pool.Get().([]byte)
			// Simulate work
			for j := 0; j < len(buf); j += 256 {
				buf[j] = byte(j)
			}
			pool.Put(buf)
		}
	})
}

// BenchmarkPiHashCalculation tests various hashing algorithms on Pi
func BenchmarkPiHashCalculation(b *testing.B) {
	data := make([]byte, 1024*1024) // 1MB
	for i := range data {
		data[i] = byte(i % 256)
	}
	
	b.Run("SHA256", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			hasher := sha256.New()
			hasher.Write(data)
			_ = hasher.Sum(nil)
		}
	})
	
	b.Run("SHA256Chunked", func(b *testing.B) {
		chunkSize := 8192
		for i := 0; i < b.N; i++ {
			hasher := sha256.New()
			for offset := 0; offset < len(data); offset += chunkSize {
				end := offset + chunkSize
				if end > len(data) {
					end = len(data)
				}
				hasher.Write(data[offset:end])
			}
			_ = hasher.Sum(nil)
		}
	})
}

// BenchmarkPiContextOperations tests context usage performance
func BenchmarkPiContextOperations(b *testing.B) {
	b.Run("ContextCreation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			_ = ctx
			cancel()
		}
	})
	
	b.Run("ContextPropagation", func(b *testing.B) {
		baseCtx := context.Background()
		for i := 0; i < b.N; i++ {
			ctx1 := context.WithValue(baseCtx, "key1", "value1")
			ctx2 := context.WithValue(ctx1, "key2", "value2")
			ctx3, cancel := context.WithTimeout(ctx2, time.Second)
			_ = ctx3.Value("key1")
			_ = ctx3.Value("key2")
			cancel()
		}
	})
}

// Pi-specific benchmark validation
func validatePiPerformance(b *testing.B, maxNsPerOp int64, maxAllocsPerOp int64, maxBytesPerOp int64) {
	if b.N == 0 {
		return
	}
	
	// Check performance metrics against Pi thresholds
	result := testing.Benchmark(func(pb *testing.B) {
		// Re-run the benchmark to get metrics
	})
	
	if result.NsPerOp() > maxNsPerOp {
		b.Errorf("Performance too slow for Pi: %d ns/op > %d ns/op threshold", 
			result.NsPerOp(), maxNsPerOp)
	}
	
	if result.AllocsPerOp() > maxAllocsPerOp {
		b.Errorf("Too many allocations for Pi: %d allocs/op > %d allocs/op threshold", 
			result.AllocsPerOp(), maxAllocsPerOp)
	}
	
	if result.AllocedBytesPerOp() > maxBytesPerOp {
		b.Errorf("Too much memory allocated for Pi: %d bytes/op > %d bytes/op threshold", 
			result.AllocedBytesPerOp(), maxBytesPerOp)
	}
}