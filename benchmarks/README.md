# Pi Performance Benchmarks

This directory contains performance benchmarks specifically designed for Raspberry Pi deployment validation.

## Overview

The benchmark suite validates Go code performance against Raspberry Pi hardware constraints:

- **Memory**: 8GB total, ~6GB available for applications
- **CPU**: 4-8 ARM64 cores at 1.5-2.4GHz
- **Storage**: SD card I/O limitations
- **Network**: Gigabit Ethernet, limited concurrent connections

## Running Benchmarks

### Basic Benchmark Run
```bash
cd benchmarks
go test -bench=. -benchmem -timeout=10m
```

### Detailed Analysis
```bash
# Run with CPU profiling
go test -bench=. -benchmem -cpuprofile=cpu.prof

# Run with memory profiling  
go test -bench=. -benchmem -memprofile=mem.prof

# Compare with baseline
go test -bench=. -benchmem -count=5 > current.txt
benchcmp baseline.txt current.txt
```

### Pi-Specific Validation
```bash
# Run only Pi-optimized benchmarks
go test -bench=BenchmarkPi -benchmem

# Test concurrency limits
go test -bench=BenchmarkPiConcurrent -benchmem -cpu=1,2,4,8

# Memory allocation tests
go test -bench=BenchmarkPiMemory -benchmem
```

## Benchmark Categories

### 1. Memory Allocation (`BenchmarkPiMemoryAllocation`)
Tests various allocation sizes against Pi memory constraints.

**Thresholds:**
- Single allocation: < 50MB
- Total allocations/op: < 1000
- Memory efficiency: Minimize allocations

### 2. String Operations (`BenchmarkPiStringBuilding`)
Compares string concatenation vs. builder performance.

**Key Insights:**
- `strings.Builder` significantly outperforms concatenation
- Pre-allocation with `Grow()` improves Pi performance

### 3. JSON Processing (`BenchmarkPiJSONProcessing`)
Tests JSON marshalling/unmarshalling for sermon metadata.

**Pi Considerations:**
- Use streaming for large JSON (>1MB)
- Struct-based unmarshalling vs. `interface{}`
- Memory allocation patterns

### 4. Concurrent Operations (`BenchmarkPiConcurrentOperations`)
Tests goroutine performance across different concurrency levels.

**Pi Limits:**
- Optimal: 4-8 goroutines (matches CPU cores)
- Maximum: 100 concurrent goroutines
- Context-based lifecycle management

### 5. File I/O (`BenchmarkPiFileOperations`)
Tests file reading strategies for large audio files.

**Pi Optimization:**
- Buffered reads outperform `ReadAll`
- 8KB-32KB buffer sizes optimal for SD cards
- Sequential access preferred

### 6. HTTP Handlers (`BenchmarkPiHTTPHandlers`)
Tests web handler performance under load.

**Metrics:**
- Response time: < 100ms for simple operations
- Throughput: > 100 requests/second
- Memory usage per request: < 1MB

### 7. Memory Pooling (`BenchmarkPiMemoryPool`)
Demonstrates `sync.Pool` benefits for Pi memory management.

**Results:**
- 3-5x reduction in allocations
- Significant GC pressure reduction
- Better memory reuse patterns

### 8. Hash Calculation (`BenchmarkPiHashCalculation`)
Tests checksum calculation strategies for large files.

**Optimization:**
- Chunked processing reduces memory usage
- SHA256 performance on ARM64
- Hardware acceleration utilization

## Performance Thresholds

### Pi 4/5 Target Performance

| Operation | Max ns/op | Max allocs/op | Max bytes/op |
|-----------|-----------|---------------|--------------|
| JSON Marshal | 10,000 | 100 | 10KB |
| File Read (1MB) | 50,000,000 | 10 | 1MB |
| Hash Calculate | 100,000,000 | 50 | 100KB |
| HTTP Handler | 1,000,000 | 200 | 50KB |

### Memory Constraints

- **Single Allocation**: < 50MB
- **Total Memory**: < 4GB application usage
- **GC Frequency**: < 10ms average pause
- **Goroutines**: < 100 concurrent

## Continuous Integration

The benchmark suite integrates with CI/CD:

```yaml
- name: Run Pi Benchmarks
  run: |
    cd benchmarks
    go test -bench=. -benchmem -timeout=10m > bench.txt
    # Compare with baseline and fail on regressions
```

## Profiling on Pi

### Memory Profiling
```bash
go test -bench=BenchmarkPiMemory -memprofile=mem.prof
go tool pprof mem.prof
```

### CPU Profiling  
```bash
go test -bench=BenchmarkPiConcurrent -cpuprofile=cpu.prof
go tool pprof cpu.prof
```

### Trace Analysis
```bash
go test -bench=BenchmarkPi -trace=trace.out
go tool trace trace.out
```

## Regression Detection

Benchmarks automatically detect performance regressions:

1. **Baseline Storage**: Results stored in `.benchmark_history`
2. **Comparison**: New results compared against baseline
3. **Thresholds**: >20% regression fails CI
4. **Reporting**: Detailed regression analysis

## Pi-Specific Optimizations

Based on benchmark results, apply these Pi optimizations:

1. **Memory**:
   - Use sync.Pool for frequently allocated objects
   - Prefer streaming over loading entire files
   - Pre-allocate slices with known capacity

2. **CPU**:
   - Limit goroutines to 4-8 (CPU core count)
   - Use context for cancellation
   - Chunk large operations

3. **I/O**:
   - Buffer file operations (8KB-32KB buffers)
   - Minimize SD card writes
   - Use connection pooling

4. **Networking**:
   - Set appropriate timeouts
   - Limit concurrent connections
   - Use keep-alive connections

## Adding New Benchmarks

1. Follow naming convention: `BenchmarkPi[Category][Operation]`
2. Include Pi-specific constraints in comments
3. Test multiple scales (small, medium, large)
4. Validate against Pi hardware limits
5. Update thresholds in this README

## Troubleshooting

### Common Issues

1. **OOM during benchmarks**: Reduce test data size
2. **Slow SD card I/O**: Use ramdisk for testing
3. **Thermal throttling**: Add cooldown between runs
4. **Network timeouts**: Increase timeout values

### Performance Analysis

1. Run benchmarks on actual Pi hardware
2. Monitor system resources during tests
3. Compare with x86 baseline performance
4. Profile memory and CPU usage patterns