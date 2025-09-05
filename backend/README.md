# Sermon Uploader Backend - Integration Testing Suite

A comprehensive integration testing framework for the sermon uploader system, designed to validate production readiness and ensure reliable deployment on Raspberry Pi hardware.

## Overview

This testing suite provides end-to-end validation of the sermon uploader system, including:
- **Complete upload workflows** from browser to MinIO storage
- **Large file handling** up to 1GB with multipart uploads  
- **Concurrent upload performance** testing with 20-30 simultaneous files
- **Connection pool health** and efficiency validation
- **Error recovery** and retry mechanism testing
- **Pi resource constraints** validation (memory, CPU, thermal)
- **Zero compression** bit-perfect audio preservation

## Test Architecture

### Test Categories

1. **Integration Tests** (`integration_test.go`)
   - End-to-end upload workflows
   - Large file handling (500MB-1GB)
   - Batch upload performance
   - Connection pool health
   - Error recovery scenarios
   - Pi resource constraints
   - Zero compression validation

2. **Health Checks** (`health_test.go`)
   - MinIO connectivity and bucket access
   - API endpoint availability
   - Memory usage within Pi limits (800MB threshold)
   - Disk space availability
   - Network connectivity
   - CPU temperature monitoring

3. **Performance Tests** (`performance_test.go`)
   - Upload throughput benchmarks
   - Connection pool efficiency
   - Retry mechanism effectiveness
   - Memory usage profiling
   - CPU usage monitoring

4. **Test Utilities** (`test_utils.go`)
   - WAV file generation with various characteristics
   - Test environment setup/teardown
   - Resource monitoring
   - MinIO test helpers

## Quick Start

### Prerequisites

```bash
# Install Go 1.23+
go version

# Start MinIO for testing (optional - tests can use containers)
docker run -p 9000:9000 -p 9001:9001 \
  -e "MINIO_ACCESS_KEY=gaius" \
  -e "MINIO_SECRET_KEY=John 3:16" \
  minio/minio server /data --console-address ":9001"
```

### Running Tests

```bash
# Fast integration tests (< 30 seconds) - ideal for pre-commit
make test-fast
# OR
./scripts/run-integration-tests.sh --fast

# Full integration test suite
make test-integration
# OR
./scripts/run-integration-tests.sh --integration

# Performance tests and benchmarks
make test-performance
# OR
./scripts/run-integration-tests.sh --performance

# Health checks only
make test-health
# OR
./scripts/run-integration-tests.sh --health

# All tests with unit tests
make test
```

### CI/CD Integration

```bash
# Run in CI mode with containerized MinIO
make test-ci
# OR
./scripts/run-integration-tests.sh --ci

# Pre-commit hook integration
make pre-commit
```

## Test Configuration

### Environment Files

- **`.env.test`** - Local testing configuration
- **`.env.ci`** - CI/CD environment configuration  
- **`test-config.yaml`** - Comprehensive test suite definitions

### Key Configuration Options

```bash
# MinIO Configuration
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=gaius
MINIO_SECRET_KEY=John 3:16
MINIO_BUCKET=sermons-test

# Performance Limits
MAX_CONCURRENT_UPLOADS=5
MAX_MEMORY_MB=800
THERMAL_THRESHOLD_C=75.0

# Test Behavior
TEST_TIMEOUT=300
TEST_CONTAINER_MODE=false
TEST_SKIP_LARGE_FILES=false
```

## Test Suites

### Fast Tests (< 30s)
Perfect for pre-commit hooks and rapid feedback:
- Basic upload workflow validation
- MinIO connectivity check
- Memory usage validation
- Quick health checks

```bash
./scripts/run-integration-tests.sh --fast
```

### Integration Tests (< 10m)
Comprehensive end-to-end validation:
- Large file uploads (500MB-1GB)
- Concurrent batch processing
- Connection pool stress testing
- Error recovery scenarios
- Pi resource constraint validation

```bash
./scripts/run-integration-tests.sh --integration
```

### Performance Tests (< 30m)
Detailed performance analysis and benchmarking:
- Upload throughput measurement
- Memory usage profiling
- CPU performance monitoring
- Connection pool efficiency
- Retry mechanism validation

```bash
./scripts/run-integration-tests.sh --performance
```

## Test File Generation

The system automatically generates various test files for validation:

```go
// Small files for basic testing
{"test_small_mono.wav", 5, 22050, 16, 1, "predictable"}     // ~500KB
{"test_small_stereo.wav", 5, 44100, 16, 2, "predictable"}   // ~1.7MB

// Medium files for performance testing  
{"test_medium_cd.wav", 30, 44100, 16, 2, "predictable"}     // ~10MB
{"test_medium_hd.wav", 30, 48000, 24, 2, "predictable"}     // ~17MB

// Large files for stress testing
{"test_large_cd.wav", 300, 44100, 16, 2, "predictable"}     // ~100MB
{"test_large_hd.wav", 120, 96000, 24, 2, "predictable"}     // ~70MB

// Huge files for multipart testing
{"test_huge_file.wav", size: 500MB}                         // Multipart upload
{"test_xlarge_file.wav", size: 1GB}                         // Large multipart
```

## Performance Metrics

### Expected Performance Thresholds

| Metric | Minimum | Target | Notes |
|--------|---------|--------|-------|
| Upload Throughput | 5 MB/s | 15 MB/s | Pi hardware dependent |
| Success Rate | 80% | 95% | Under normal conditions |
| Memory Usage | < 1GB | < 800MB | Pi 4 constraint |
| P95 Latency | < 60s | < 30s | Large file uploads |
| Concurrent Files | 5 | 10-20 | Depends on file size |

### Sample Performance Output

```
PERFORMANCE REPORT: UploadThroughput
=====================================
Test Duration: 45.2s
Throughput: 12.3 MB/s  
Operations/sec: 2.1
Success Rate: 95.0% (19/20)
Error Rate: 5.0% (1 failures)

Resource Usage:
  CPU: 15.2% → 45.8% (peak: 67.3%)
  Memory: 234 MB (peak: 456 MB)
  Goroutines: 8 → 15 (peak: 23)

Latency Percentiles:
  P50: 3.2s
  P95: 12.8s  
  P99: 18.6s

Connection Pool:
  Active: 3
  Idle: 2
  Retries: 4
  Connection Errors: 0
```

## Production Readiness Validation

The integration tests validate several critical production scenarios:

### 1. End-to-End Upload Flow
- ✅ Browser → API → MinIO → Storage validation
- ✅ Multipart upload handling for large files
- ✅ Hash verification and integrity checks
- ✅ Metadata preservation and retrieval

### 2. Raspberry Pi Constraints  
- ✅ Memory usage stays under 800MB limit
- ✅ CPU usage reasonable under load
- ✅ Thermal throttling awareness
- ✅ Connection pool efficiency

### 3. Error Handling
- ✅ Network failure recovery
- ✅ Timeout handling with retries
- ✅ Partial upload recovery
- ✅ Connection pool health maintenance

### 4. Audio Quality Preservation
- ✅ Zero compression validation
- ✅ Bit-perfect audio preservation
- ✅ Hash consistency verification
- ✅ Content-type preservation

### 5. Scale Testing
- ✅ 20-30 concurrent file uploads
- ✅ Large file handling (500MB-1GB)
- ✅ Sustained load performance
- ✅ Resource cleanup after load

## Troubleshooting

### Common Issues

1. **MinIO Connection Failed**
   ```bash
   # Start local MinIO
   docker run -p 9000:9000 minio/minio server /data
   
   # Or skip MinIO-dependent tests
   ./scripts/run-integration-tests.sh --skip-minio
   ```

2. **Tests Timing Out**
   ```bash
   # Run with longer timeout
   go test -tags=integration -timeout=600s ./...
   
   # Or skip large file tests
   export TEST_SKIP_LARGE_FILES=true
   ```

3. **Memory Issues on Pi**
   ```bash
   # Reduce concurrent uploads
   export MAX_CONCURRENT_UPLOADS=2
   export MAX_MEMORY_MB=400
   ```

4. **Container Mode Issues**
   ```bash
   # Ensure Docker is available
   docker --version
   
   # Run without containers
   ./scripts/run-integration-tests.sh --no-container
   ```

### Debug Mode

```bash
# Enable verbose output
./scripts/run-integration-tests.sh --verbose

# Run specific test with debug info
go test -tags=integration -v -run=TestEndToEndUploadFlow ./...

# Enable resource monitoring
export TEST_MONITOR_RESOURCES=true
```

## Development

### Adding New Tests

1. Create test in appropriate file (`*_test.go`)
2. Use build tag `//go:build integration`
3. Follow naming convention `Test*`
4. Use test utilities for common operations
5. Add cleanup in test teardown

Example:
```go
//go:build integration
// +build integration

func TestMyNewFeature(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    // Test implementation
    te, err := NewTestEnvironment(false)
    require.NoError(t, err)
    defer te.Close()
    
    // Your test logic here
}
```

### Contributing

1. Run fast tests before committing: `make pre-commit`
2. Ensure new tests have appropriate timeouts
3. Add performance thresholds for new metrics
4. Update documentation for new test categories
5. Test on actual Pi hardware when possible

## Deployment Validation

Before production deployment, run the complete validation suite:

```bash
# Complete validation pipeline
make clean
make test-integration
make test-performance
make test-health

# Or use the comprehensive script
./scripts/run-integration-tests.sh --integration --verbose
```

This ensures your deployment is production-ready with validated:
- ✅ Upload functionality under load
- ✅ Resource usage within Pi constraints  
- ✅ Error recovery and resilience
- ✅ Audio quality preservation
- ✅ Performance meeting thresholds

The integration test suite provides confidence that your sermon uploader system will perform reliably in production environments.