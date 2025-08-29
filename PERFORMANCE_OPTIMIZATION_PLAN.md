# Performance Optimization Plan for Raspberry Pi Deployment

## Current Performance Issues

### Bottleneck Analysis
- **Sequential Processing**: Files upload one at a time (1-2GB each = 10-20 min per file)
- **Network Bandwidth**: Uncompressed WAV files saturate limited Pi network
- **Memory Usage**: Loading entire files into memory crashes on Pi (limited RAM)
- **No Recovery**: Network interruptions require complete re-upload
- **CPU Underutilization**: Single-threaded, not using Pi's 4 cores

## Proposed Solutions

### 1. Parallel Uploads (Immediate Impact: 3-4x faster)
```javascript
// Current: Sequential
for (const file of files) {
  await uploadFile(file) // Waits for each
}

// Optimized: Parallel with concurrency limit
const CONCURRENT_UPLOADS = 3 // Tuned for Pi's resources
await Promise.all(
  files.map((file, i) => 
    delay(i * 100).then(() => uploadFile(file))
  )
)
```

### 2. Chunked Uploads with Resume (Reliability + Speed)
```javascript
// Split large files into 10MB chunks
// - Resume from last successful chunk on failure
// - Progress tracking per chunk
// - Parallel chunk uploads per file
const CHUNK_SIZE = 10 * 1024 * 1024 // 10MB chunks
const chunks = Math.ceil(file.size / CHUNK_SIZE)
```

### 3. Client-Side Compression (50-70% size reduction)
```javascript
// Options:
// a) FLAC compression (lossless, 50% reduction)
// b) Opus/AAC (lossy but acceptable, 90% reduction)
// c) Gzip for transfer only (30% reduction, keeps WAV)
```

### 4. Smart Queue Management
```javascript
// Priority queue based on:
// - File size (smaller first for quick wins)
// - Retry count (deprioritize repeated failures)
// - Time in queue (prevent starvation)
```

### 5. Resource-Aware Processing
```javascript
// Detect Pi capabilities
const MAX_PARALLEL = navigator.hardwareConcurrency || 2
const MAX_MEMORY = 100 * 1024 * 1024 // 100MB max per upload
```

## Implementation Priority

### Phase 1: Parallel Uploads (1 hour)
**Impact: 3-4x faster**
- Modify `processBatch` to use Promise.allSettled
- Add concurrency limiter (p-limit library)
- Monitor Pi CPU/Memory during uploads

### Phase 2: Chunked Uploads (3 hours)
**Impact: Resume capability + memory efficiency**
- Implement file slicing in browser
- Add chunk tracking to backend
- Store chunk progress in localStorage
- Resume logic on page reload

### Phase 3: Transfer Compression (2 hours)
**Impact: 30-50% bandwidth reduction**
- Add pako (gzip) library for compression
- Compress chunks before upload
- Decompress on backend
- Keep original WAV in MinIO

### Phase 4: Smart Queueing (1 hour)
**Impact: Better UX for mixed file sizes**
- Implement priority queue
- Add file size sorting
- Show estimated time remaining

## Raspberry Pi Specific Optimizations

### Memory Management
```javascript
// Use Blob.slice() instead of loading entire file
const chunk = file.slice(start, end)

// Free memory after each chunk
URL.revokeObjectURL(objectUrl)

// Limit concurrent uploads based on available RAM
const freeMemory = performance.memory?.usedJSHeapSize
```

### Network Optimization
```javascript
// Adaptive chunk size based on network speed
let chunkSize = 5 * 1024 * 1024 // Start with 5MB
if (uploadSpeed < 1) chunkSize = 1 * 1024 * 1024 // 1MB for slow networks
if (uploadSpeed > 10) chunkSize = 25 * 1024 * 1024 // 25MB for fast networks

// Exponential backoff for retries
const retryDelay = Math.min(1000 * Math.pow(2, attempt), 30000)
```

### Storage Optimization
```javascript
// Stream directly to MinIO without temp files
// Use multipart upload API
// Clean incomplete uploads after 24 hours
```

## Expected Results

### Before Optimization
- 1GB file: 15-20 minutes
- 10 files batch: 3-4 hours
- Failure recovery: Start over
- Memory usage: 2GB+ spike

### After Optimization
- 1GB file: 5-7 minutes (with compression)
- 10 files batch: 30-45 minutes (parallel)
- Failure recovery: Resume from last chunk
- Memory usage: <200MB constant

## Monitoring & Metrics

### Client-Side Metrics
- Upload speed (MB/s)
- Time per file
- Retry count
- Memory usage
- Network latency

### Server-Side Metrics
- Concurrent connections
- CPU usage
- Disk I/O
- Network throughput
- Error rates

## Fallback Strategy

If Pi can't handle optimizations:
1. Offload compression to server
2. Reduce parallel uploads to 2
3. Smaller chunk sizes (5MB)
4. Queue files locally, upload overnight
5. Consider edge caching server

## Testing Plan

1. Test with various file sizes (100MB, 500MB, 2GB)
2. Simulate network interruptions
3. Monitor Pi temperature/throttling
4. Test with 20+ file batches
5. Measure actual vs theoretical improvements