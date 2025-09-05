# Go Best Practices for Raspberry Pi Deployment

This comprehensive guide outlines Go programming best practices specifically tailored for Raspberry Pi deployment, focusing on the sermon uploader project's requirements for handling large audio files with optimal performance and reliability.

## Table of Contents

1. [Pi Hardware Constraints](#pi-hardware-constraints)
2. [Memory Management](#memory-management)
3. [CPU and Concurrency](#cpu-and-concurrency)
4. [File I/O Optimization](#file-io-optimization)
5. [Network Programming](#network-programming)
6. [Error Handling](#error-handling)
7. [Performance Monitoring](#performance-monitoring)
8. [Security Considerations](#security-considerations)
9. [Build and Deployment](#build-and-deployment)
10. [Testing Strategies](#testing-strategies)

## Pi Hardware Constraints

Understanding Raspberry Pi limitations is crucial for optimal Go application design:

### Hardware Specifications (Pi 4/5)
- **CPU**: 4-8 ARM64 cores @ 1.5-2.4GHz
- **Memory**: 4-8GB LPDDR4
- **Storage**: SD card (limited write cycles)
- **Network**: Gigabit Ethernet
- **Thermal**: Passive cooling, throttling at 80°C

### Design Principles
```go
// ✅ Good: Design with Pi constraints in mind
const (
    MaxConcurrentUploads = 4      // Match CPU cores
    MaxMemoryPerFile     = 256    // MB - reasonable for Pi
    BufferSize           = 32768  // 32KB - optimal for SD cards
    GoroutinePoolSize    = 8      // Limit concurrent goroutines
)

// ❌ Bad: Ignoring Pi limitations
const MaxConcurrentUploads = 100 // Will overwhelm Pi resources
```

## Memory Management

Memory is precious on Pi. Every allocation matters.

### Preallocate Slices and Maps

```go
// ✅ Good: Pre-allocate with known capacity
func processAudioFiles(count int) []AudioMetadata {
    files := make([]AudioMetadata, 0, count) // Pre-allocated capacity
    metadataMap := make(map[string]AudioMetadata, count)
    
    for i := 0; i < count; i++ {
        // Process files...
        files = append(files, metadata)
        metadataMap[filename] = metadata
    }
    return files
}

// ❌ Bad: Growing slices repeatedly causes allocations
func processAudioFiles() []AudioMetadata {
    var files []AudioMetadata // No initial capacity
    // Each append may cause reallocation
    files = append(files, metadata)
    return files
}
```

### Use sync.Pool for Frequent Allocations

```go
// ✅ Good: Reuse buffers with sync.Pool
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, BufferSize)
    },
}

func processFile(filename string) error {
    buffer := bufferPool.Get().([]byte)
    defer bufferPool.Put(buffer)
    
    // Use buffer for file processing
    return nil
}

// ❌ Bad: Allocating new buffers each time
func processFile(filename string) error {
    buffer := make([]byte, BufferSize) // New allocation every call
    // Process file
    return nil
}
```

### Streaming for Large Files

```go
// ✅ Good: Stream large files to avoid loading into memory
func calculateChecksum(filename string) (string, error) {
    file, err := os.Open(filename)
    if err != nil {
        return "", err
    }
    defer file.Close()
    
    hasher := sha256.New()
    buffer := make([]byte, 32768) // 32KB buffer
    
    for {
        n, err := file.Read(buffer)
        if n > 0 {
            hasher.Write(buffer[:n])
        }
        if err == io.EOF {
            break
        }
        if err != nil {
            return "", err
        }
    }
    
    return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// ❌ Bad: Loading entire file into memory
func calculateChecksum(filename string) (string, error) {
    data, err := os.ReadFile(filename) // Loads entire file into memory
    if err != nil {
        return "", err
    }
    return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}
```

### String Building Optimization

```go
// ✅ Good: Use strings.Builder with pre-allocated capacity
func buildResponse(items []string) string {
    var builder strings.Builder
    builder.Grow(len(items) * 50) // Estimate total capacity
    
    for i, item := range items {
        if i > 0 {
            builder.WriteString(", ")
        }
        builder.WriteString(item)
    }
    return builder.String()
}

// ❌ Bad: String concatenation causes multiple allocations
func buildResponse(items []string) string {
    var result string
    for i, item := range items {
        if i > 0 {
            result += ", " // Each += creates new string
        }
        result += item
    }
    return result
}
```

## CPU and Concurrency

Optimize for Pi's limited CPU cores and prevent thermal throttling.

### Goroutine Pool Pattern

```go
// ✅ Good: Limit concurrent goroutines with worker pool
type WorkerPool struct {
    jobs    chan Job
    results chan Result
    workers int
}

func NewWorkerPool(workers int) *WorkerPool {
    return &WorkerPool{
        jobs:    make(chan Job, workers*2),    // Buffer for incoming jobs
        results: make(chan Result, workers*2),
        workers: workers,
    }
}

func (wp *WorkerPool) Start(ctx context.Context) {
    for i := 0; i < wp.workers; i++ {
        go wp.worker(ctx)
    }
}

func (wp *WorkerPool) worker(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case job := <-wp.jobs:
            result := job.Process()
            wp.results <- result
        }
    }
}

// ❌ Bad: Unlimited goroutine spawning
func processFiles(files []string) {
    for _, file := range files {
        go func(f string) { // Can create thousands of goroutines
            processFile(f)
        }(file)
    }
}
```

### Context-Based Cancellation

```go
// ✅ Good: Use context for proper goroutine lifecycle
func uploadFile(ctx context.Context, filename string) error {
    // Create timeout context
    uploadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    // Check for cancellation periodically
    select {
    case <-uploadCtx.Done():
        return uploadCtx.Err()
    default:
        // Continue processing
    }
    
    return performUpload(uploadCtx, filename)
}

// ❌ Bad: No cancellation mechanism
func uploadFile(filename string) error {
    // Long-running operation without cancellation
    return performUpload(filename) // Can't be cancelled
}
```

### CPU-Intensive Work with Yielding

```go
// ✅ Good: Yield CPU periodically in intensive loops
func processLargeFile(ctx context.Context, filename string) error {
    file, err := os.Open(filename)
    if err != nil {
        return err
    }
    defer file.Close()
    
    buffer := make([]byte, 64*1024) // 64KB chunks
    processed := 0
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }
        
        n, err := file.Read(buffer)
        if n > 0 {
            // Process chunk
            processChunk(buffer[:n])
            processed += n
            
            // Yield CPU every 10MB to prevent thermal throttling
            if processed%(10*1024*1024) == 0 {
                runtime.Gosched()
            }
        }
        
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

### GOMAXPROCS Configuration

```go
// ✅ Good: Set appropriate GOMAXPROCS for Pi
func init() {
    // On Pi 4/5, use all available cores
    numCPU := runtime.NumCPU()
    if numCPU > 8 {
        numCPU = 8 // Pi maximum
    }
    runtime.GOMAXPROCS(numCPU)
    
    log.Printf("GOMAXPROCS set to %d for Pi optimization", numCPU)
}
```

## File I/O Optimization

SD card I/O is a major bottleneck on Pi. Optimize accordingly.

### Buffered I/O Operations

```go
// ✅ Good: Use buffered I/O for better SD card performance
func copyFile(src, dst string) error {
    srcFile, err := os.Open(src)
    if err != nil {
        return err
    }
    defer srcFile.Close()
    
    dstFile, err := os.Create(dst)
    if err != nil {
        return err
    }
    defer dstFile.Close()
    
    // Use buffered reader/writer for SD card optimization
    reader := bufio.NewReaderSize(srcFile, 64*1024)   // 64KB read buffer
    writer := bufio.NewWriterSize(dstFile, 64*1024)   // 64KB write buffer
    defer writer.Flush()
    
    _, err = io.Copy(writer, reader)
    return err
}

// ❌ Bad: Direct file operations without buffering
func copyFile(src, dst string) error {
    data, err := os.ReadFile(src) // Reads entire file at once
    if err != nil {
        return err
    }
    return os.WriteFile(dst, data, 0644) // Writes entire file at once
}
```

### Minimize Write Operations

```go
// ✅ Good: Batch writes to reduce SD card wear
type MetadataWriter struct {
    file   *os.File
    writer *bufio.Writer
    batch  []AudioMetadata
    mu     sync.Mutex
}

func (mw *MetadataWriter) Add(metadata AudioMetadata) error {
    mw.mu.Lock()
    defer mw.mu.Unlock()
    
    mw.batch = append(mw.batch, metadata)
    
    // Batch writes every 10 items or on timer
    if len(mw.batch) >= 10 {
        return mw.flush()
    }
    return nil
}

func (mw *MetadataWriter) flush() error {
    for _, metadata := range mw.batch {
        data, _ := json.Marshal(metadata)
        mw.writer.Write(data)
        mw.writer.WriteByte('\n')
    }
    mw.batch = mw.batch[:0] // Reset slice but keep capacity
    return mw.writer.Flush()
}

// ❌ Bad: Immediate write for each metadata item
func writeMetadata(metadata AudioMetadata) error {
    file, err := os.OpenFile("metadata.json", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return err
    }
    defer file.Close()
    
    data, _ := json.Marshal(metadata)
    _, err = file.Write(data) // SD card write for each item
    return err
}
```

### Temporary File Management

```go
// ✅ Good: Proper temporary file cleanup
func processUpload(file multipart.File, header *multipart.FileHeader) error {
    // Create temporary file with cleanup
    tempFile, err := os.CreateTemp("", "upload_*.wav")
    if err != nil {
        return err
    }
    defer func() {
        tempFile.Close()
        os.Remove(tempFile.Name()) // Always cleanup
    }()
    
    // Copy uploaded file to temp location
    _, err = io.Copy(tempFile, file)
    if err != nil {
        return err
    }
    
    // Process the temporary file
    return processAudioFile(tempFile.Name())
}

// ❌ Bad: Temporary files without cleanup
func processUpload(file multipart.File) error {
    tempFile, _ := os.CreateTemp("", "upload_*.wav")
    io.Copy(tempFile, file)
    // Missing cleanup - temp files accumulate
    return processAudioFile(tempFile.Name())
}
```

## Network Programming

Optimize network operations for Pi's networking capabilities.

### Connection Pooling

```go
// ✅ Good: Reuse HTTP connections with proper timeouts
var httpClient = &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        10,               // Limit for Pi
        MaxIdleConnsPerHost: 4,                // Conservative for Pi
        IdleConnTimeout:     60 * time.Second,
        DialTimeout:         10 * time.Second,
        TLSHandshakeTimeout: 10 * time.Second,
        // Enable keep-alive
        DisableKeepAlives: false,
    },
}

func uploadToStorage(data []byte, url string) error {
    req, err := http.NewRequest("POST", url, bytes.NewReader(data))
    if err != nil {
        return err
    }
    
    resp, err := httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}

// ❌ Bad: Creating new client for each request
func uploadToStorage(data []byte, url string) error {
    client := &http.Client{} // New client each time
    resp, err := client.Post(url, "application/octet-stream", bytes.NewReader(data))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    return nil
}
```

### Streaming Large Uploads

```go
// ✅ Good: Stream large files instead of loading into memory
func uploadLargeFile(filename, uploadURL string) error {
    file, err := os.Open(filename)
    if err != nil {
        return err
    }
    defer file.Close()
    
    // Get file size for Content-Length header
    stat, err := file.Stat()
    if err != nil {
        return err
    }
    
    req, err := http.NewRequest("POST", uploadURL, file)
    if err != nil {
        return err
    }
    
    req.ContentLength = stat.Size()
    req.Header.Set("Content-Type", "audio/wav")
    
    resp, err := httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}

// ❌ Bad: Loading entire file into memory before upload
func uploadLargeFile(filename, uploadURL string) error {
    data, err := os.ReadFile(filename) // Entire file in memory
    if err != nil {
        return err
    }
    
    resp, err := http.Post(uploadURL, "audio/wav", bytes.NewReader(data))
    // Large memory usage on Pi
    defer resp.Body.Close()
    return err
}
```

### Network Buffer Optimization

```go
// ✅ Good: Optimize network buffers for Pi
func setupServer() *http.Server {
    return &http.Server{
        Addr:           ":8080",
        ReadTimeout:    30 * time.Second,
        WriteTimeout:   30 * time.Second,
        IdleTimeout:    60 * time.Second,
        MaxHeaderBytes: 1 << 16, // 64KB - reasonable for Pi
        
        // Custom connection handler
        ConnState: func(conn net.Conn, state http.ConnState) {
            if tcpConn, ok := conn.(*net.TCPConn); ok {
                // Optimize TCP buffer sizes for Pi
                tcpConn.SetReadBuffer(64 * 1024)  // 64KB
                tcpConn.SetWriteBuffer(64 * 1024) // 64KB
                tcpConn.SetNoDelay(true)          // Reduce latency
            }
        },
    }
}
```

## Error Handling

Robust error handling is crucial for Pi reliability.

### Structured Error Handling

```go
// ✅ Good: Structured error handling with context
type ProcessingError struct {
    Op       string
    Filename string
    Err      error
    Retryable bool
}

func (e *ProcessingError) Error() string {
    return fmt.Sprintf("processing error in %s for file %s: %v", e.Op, e.Filename, e.Err)
}

func (e *ProcessingError) Unwrap() error {
    return e.Err
}

func processAudioFile(filename string) error {
    // Open file
    file, err := os.Open(filename)
    if err != nil {
        return &ProcessingError{
            Op:       "open",
            Filename: filename,
            Err:      err,
            Retryable: false, // File not found isn't retryable
        }
    }
    defer file.Close()
    
    // Process with retry logic for network errors
    if err := uploadFile(file); err != nil {
        var netErr net.Error
        retryable := errors.As(err, &netErr) && netErr.Timeout()
        
        return &ProcessingError{
            Op:       "upload",
            Filename: filename,
            Err:      err,
            Retryable: retryable,
        }
    }
    
    return nil
}

// Retry logic
func processWithRetry(filename string, maxRetries int) error {
    for i := 0; i < maxRetries; i++ {
        err := processAudioFile(filename)
        if err == nil {
            return nil
        }
        
        var procErr *ProcessingError
        if errors.As(err, &procErr) && !procErr.Retryable {
            return err // Don't retry non-retryable errors
        }
        
        // Exponential backoff
        time.Sleep(time.Duration(i+1) * time.Second)
    }
    
    return fmt.Errorf("failed after %d retries", maxRetries)
}
```

### Resource Cleanup

```go
// ✅ Good: Guaranteed resource cleanup
func processMultipleFiles(filenames []string) error {
    var wg sync.WaitGroup
    errChan := make(chan error, len(filenames))
    semaphore := make(chan struct{}, 4) // Limit concurrent processing
    
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel() // Cleanup all goroutines
    
    for _, filename := range filenames {
        wg.Add(1)
        go func(fname string) {
            defer wg.Done()
            
            // Acquire semaphore
            select {
            case semaphore <- struct{}{}:
                defer func() { <-semaphore }()
            case <-ctx.Done():
                return
            }
            
            // Process file with resource cleanup
            if err := processFileWithCleanup(ctx, fname); err != nil {
                errChan <- err
                cancel() // Cancel other operations on error
            }
        }(filename)
    }
    
    // Wait for completion
    wg.Wait()
    close(errChan)
    
    // Check for errors
    for err := range errChan {
        if err != nil {
            return err
        }
    }
    
    return nil
}

func processFileWithCleanup(ctx context.Context, filename string) error {
    // Create cleanup tracker
    cleanup := &CleanupTracker{}
    defer cleanup.ExecuteAll()
    
    // Open file
    file, err := os.Open(filename)
    if err != nil {
        return err
    }
    cleanup.Add(func() { file.Close() })
    
    // Create temp file
    tempFile, err := os.CreateTemp("", "processing_*.tmp")
    if err != nil {
        return err
    }
    cleanup.Add(func() {
        tempFile.Close()
        os.Remove(tempFile.Name())
    })
    
    // Process with context cancellation
    return processWithContext(ctx, file, tempFile)
}

type CleanupTracker struct {
    cleanups []func()
    mu       sync.Mutex
}

func (ct *CleanupTracker) Add(cleanup func()) {
    ct.mu.Lock()
    defer ct.mu.Unlock()
    ct.cleanups = append(ct.cleanups, cleanup)
}

func (ct *CleanupTracker) ExecuteAll() {
    ct.mu.Lock()
    defer ct.mu.Unlock()
    
    // Execute in reverse order (LIFO)
    for i := len(ct.cleanups) - 1; i >= 0; i-- {
        func() {
            defer func() {
                if r := recover(); r != nil {
                    log.Printf("Cleanup panic: %v", r)
                }
            }()
            ct.cleanups[i]()
        }()
    }
}
```

## Performance Monitoring

Monitor Pi performance to prevent resource exhaustion.

### Runtime Metrics Collection

```go
// ✅ Good: Comprehensive Pi performance monitoring
type PiMetrics struct {
    Timestamp       time.Time
    CPUUsage        float64
    MemoryUsage     float64
    GoroutineCount  int
    HeapSize        uint64
    GCPauseMs       uint64
    FileDescriptors int
}

func collectMetrics() *PiMetrics {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    return &PiMetrics{
        Timestamp:       time.Now(),
        GoroutineCount:  runtime.NumGoroutine(),
        HeapSize:        m.HeapAlloc / 1024 / 1024, // MB
        GCPauseMs:       m.PauseNs[(m.NumGC+255)%256] / 1000000,
        // Additional system metrics would be collected here
    }
}

// Monitoring middleware
func monitoringMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        metrics := collectMetrics()
        
        // Check for resource exhaustion
        if metrics.GoroutineCount > 200 {
            log.Printf("High goroutine count: %d", metrics.GoroutineCount)
        }
        
        if metrics.HeapSize > 512 { // 512MB heap limit
            log.Printf("High memory usage: %d MB", metrics.HeapSize)
            runtime.GC() // Force garbage collection
        }
        
        next.ServeHTTP(w, r)
        
        duration := time.Since(start)
        log.Printf("Request: %s %s - Duration: %v", r.Method, r.URL.Path, duration)
    })
}
```

### Memory Leak Detection

```go
// ✅ Good: Memory leak detection for long-running Pi services
func startMemoryMonitor(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    var lastHeapSize uint64
    consecutiveIncreases := 0
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            var m runtime.MemStats
            runtime.ReadMemStats(&m)
            
            currentHeap := m.HeapAlloc
            
            // Detect potential memory leaks
            if currentHeap > lastHeapSize {
                consecutiveIncreases++
                if consecutiveIncreases > 5 { // 25 minutes of growth
                    log.Printf("Potential memory leak detected: heap grew from %d to %d MB over %d periods",
                        lastHeapSize/1024/1024, currentHeap/1024/1024, consecutiveIncreases)
                    
                    // Force GC and check again
                    runtime.GC()
                    runtime.ReadMemStats(&m)
                    
                    if m.HeapAlloc > currentHeap*9/10 { // Still high after GC
                        log.Printf("Memory leak confirmed: heap size %d MB after GC", m.HeapAlloc/1024/1024)
                        // Could trigger alert or restart
                    }
                }
            } else {
                consecutiveIncreases = 0
            }
            
            lastHeapSize = currentHeap
            
            // Log current metrics
            log.Printf("Memory: Heap=%dMB, Sys=%dMB, Goroutines=%d, GC=%d",
                m.HeapAlloc/1024/1024, m.Sys/1024/1024, runtime.NumGoroutine(), m.NumGC)
        }
    }
}
```

## Security Considerations

Security is crucial for Pi deployments, especially with audio file uploads.

### Input Validation

```go
// ✅ Good: Comprehensive input validation
func validateAudioFile(file multipart.File, header *multipart.FileHeader) error {
    // Check file size (Pi memory constraints)
    if header.Size > 500*1024*1024 { // 500MB limit
        return fmt.Errorf("file too large: %d bytes (max 500MB)", header.Size)
    }
    
    // Check file extension
    ext := strings.ToLower(filepath.Ext(header.Filename))
    allowedExts := map[string]bool{
        ".wav": true,
        ".mp3": true,
        ".m4a": true,
    }
    if !allowedExts[ext] {
        return fmt.Errorf("unsupported file type: %s", ext)
    }
    
    // Check filename for path traversal
    if strings.Contains(header.Filename, "..") || strings.Contains(header.Filename, "/") {
        return fmt.Errorf("invalid filename: %s", header.Filename)
    }
    
    // Read first few bytes to verify file type
    buffer := make([]byte, 512)
    n, err := file.Read(buffer)
    if err != nil && err != io.EOF {
        return err
    }
    
    // Reset file position
    if seeker, ok := file.(io.Seeker); ok {
        seeker.Seek(0, 0)
    }
    
    // Verify magic bytes
    if !isValidAudioFile(buffer[:n], ext) {
        return fmt.Errorf("file content doesn't match extension %s", ext)
    }
    
    return nil
}

func isValidAudioFile(data []byte, ext string) bool {
    switch ext {
    case ".wav":
        return len(data) >= 4 && string(data[0:4]) == "RIFF"
    case ".mp3":
        return len(data) >= 3 && (string(data[0:3]) == "ID3" || 
                                  (data[0] == 0xFF && (data[1]&0xE0) == 0xE0))
    default:
        return true // Basic check for other formats
    }
}
```

### Rate Limiting

```go
// ✅ Good: Rate limiting for Pi resource protection
type RateLimiter struct {
    requests map[string][]time.Time
    mutex    sync.RWMutex
    maxReqs  int
    window   time.Duration
}

func NewRateLimiter(maxReqs int, window time.Duration) *RateLimiter {
    rl := &RateLimiter{
        requests: make(map[string][]time.Time),
        maxReqs:  maxReqs,
        window:   window,
    }
    
    // Cleanup old entries periodically
    go rl.cleanup()
    return rl
}

func (rl *RateLimiter) Allow(clientIP string) bool {
    rl.mutex.Lock()
    defer rl.mutex.Unlock()
    
    now := time.Now()
    cutoff := now.Add(-rl.window)
    
    // Get and clean old requests
    requests := rl.requests[clientIP]
    validRequests := make([]time.Time, 0, len(requests))
    
    for _, reqTime := range requests {
        if reqTime.After(cutoff) {
            validRequests = append(validRequests, reqTime)
        }
    }
    
    // Check if under limit
    if len(validRequests) >= rl.maxReqs {
        rl.requests[clientIP] = validRequests
        return false
    }
    
    // Add current request
    validRequests = append(validRequests, now)
    rl.requests[clientIP] = validRequests
    return true
}

func (rl *RateLimiter) cleanup() {
    ticker := time.NewTicker(time.Hour)
    defer ticker.Stop()
    
    for range ticker.C {
        rl.mutex.Lock()
        cutoff := time.Now().Add(-rl.window)
        
        for ip, requests := range rl.requests {
            validRequests := make([]time.Time, 0, len(requests))
            for _, reqTime := range requests {
                if reqTime.After(cutoff) {
                    validRequests = append(validRequests, reqTime)
                }
            }
            
            if len(validRequests) == 0 {
                delete(rl.requests, ip)
            } else {
                rl.requests[ip] = validRequests
            }
        }
        rl.mutex.Unlock()
    }
}

// Rate limiting middleware
func rateLimitingMiddleware(rl *RateLimiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            clientIP := getClientIP(r)
            
            if !rl.Allow(clientIP) {
                http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}

func getClientIP(r *http.Request) string {
    // Check X-Forwarded-For header
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        return strings.Split(xff, ",")[0]
    }
    
    // Check X-Real-IP header
    if xri := r.Header.Get("X-Real-IP"); xri != "" {
        return xri
    }
    
    // Fall back to remote address
    host, _, _ := net.SplitHostPort(r.RemoteAddr)
    return host
}
```

## Build and Deployment

Optimize builds for Pi deployment.

### Cross-Compilation

```bash
#!/bin/bash
# build-pi.sh - Cross-compile for Raspberry Pi

set -e

echo "Building sermon uploader for Raspberry Pi..."

# Pi 4/5 (ARM64)
echo "Building for Pi 4/5 (ARM64)..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/sermon-uploader-pi4 ./cmd/server

# Pi 3 (ARM v7)
echo "Building for Pi 3 (ARM v7)..."
GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="-s -w" -o dist/sermon-uploader-pi3 ./cmd/server

# Check binary sizes
echo "Binary sizes:"
ls -lh dist/sermon-uploader-pi*

echo "Build completed successfully!"
```

### Optimized Dockerfile for Pi

```dockerfile
# Dockerfile.pi - Optimized for Raspberry Pi
FROM --platform=linux/arm64 golang:1.23-alpine AS builder

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with optimizations for Pi
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty)" \
    -a -installsuffix cgo \
    -o sermon-uploader \
    ./cmd/server

# Final image - minimal for Pi storage constraints
FROM --platform=linux/arm64 scratch

# Copy ca-certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary
COPY --from=builder /src/sermon-uploader /sermon-uploader

# Pi-specific settings
ENV GOMAXPROCS=4
ENV GOGC=100
ENV GODEBUG=gctrace=1

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/sermon-uploader", "healthcheck"]

# Run
ENTRYPOINT ["/sermon-uploader"]
```

### Systemd Service Configuration

```ini
# /etc/systemd/system/sermon-uploader.service
[Unit]
Description=Sermon Uploader Service
After=network.target
Wants=network.target

[Service]
Type=simple
User=sermon
Group=sermon
WorkingDirectory=/opt/sermon-uploader
ExecStart=/opt/sermon-uploader/sermon-uploader
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=10

# Pi-specific resource limits
LimitNOFILE=2048
LimitNPROC=512
MemoryMax=2G
CPUQuota=400%  # 4 cores max

# Environment variables
Environment=GOMAXPROCS=4
Environment=GOGC=100
Environment=GODEBUG=madvdontneed=1

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/sermon-uploader/uploads
ReadWritePaths=/opt/sermon-uploader/logs

[Install]
WantedBy=multi-user.target
```

## Testing Strategies

Comprehensive testing ensures Pi deployment reliability.

### Pi-Specific Benchmark Tests

```go
// benchmark_test.go - Pi performance validation
package main

import (
    "context"
    "runtime"
    "testing"
    "time"
)

func BenchmarkPiMemoryAllocation(b *testing.B) {
    // Test various allocation sizes on Pi
    sizes := []int{1024, 10240, 102400, 1048576} // 1KB to 1MB
    
    for _, size := range sizes {
        b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
            b.ReportAllocs()
            for i := 0; i < b.N; i++ {
                data := make([]byte, size)
                // Simulate some work
                data[0] = byte(i)
                _ = data
            }
        })
    }
}

func BenchmarkPiConcurrentProcessing(b *testing.B) {
    // Test goroutine performance on Pi
    concurrencyLevels := []int{1, 2, 4, 8, 16}
    
    for _, concurrency := range concurrencyLevels {
        b.Run(fmt.Sprintf("Goroutines%d", concurrency), func(b *testing.B) {
            work := func() {
                // Simulate CPU work
                sum := 0
                for i := 0; i < 10000; i++ {
                    sum += i
                }
            }
            
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                var wg sync.WaitGroup
                for j := 0; j < concurrency; j++ {
                    wg.Add(1)
                    go func() {
                        defer wg.Done()
                        work()
                    }()
                }
                wg.Wait()
            }
        })
    }
}

func TestPiResourceLimits(t *testing.T) {
    // Test resource consumption limits
    t.Run("GoroutineLimit", func(t *testing.T) {
        initial := runtime.NumGoroutine()
        var wg sync.WaitGroup
        
        // Spawn many goroutines
        for i := 0; i < 1000; i++ {
            wg.Add(1)
            go func() {
                defer wg.Done()
                time.Sleep(10 * time.Millisecond)
            }()
        }
        
        current := runtime.NumGoroutine()
        if current-initial > 200 { // Pi limit
            t.Errorf("Too many goroutines: %d (limit 200)", current-initial)
        }
        
        wg.Wait()
    })
    
    t.Run("MemoryPressure", func(t *testing.T) {
        var m runtime.MemStats
        runtime.ReadMemStats(&m)
        initial := m.HeapAlloc
        
        // Allocate memory
        data := make([][]byte, 1000)
        for i := range data {
            data[i] = make([]byte, 1024*1024) // 1MB each
        }
        
        runtime.ReadMemStats(&m)
        allocated := m.HeapAlloc - initial
        
        if allocated > 2*1024*1024*1024 { // 2GB limit for Pi
            t.Errorf("Excessive memory allocation: %d MB", allocated/1024/1024)
        }
        
        // Clear references for GC
        for i := range data {
            data[i] = nil
        }
        runtime.GC()
    })
}
```

### Integration Testing

```go
// integration_test.go - End-to-end Pi testing
// +build integration

func TestPiDeploymentFlow(t *testing.T) {
    // Start test server
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()
    
    server := startTestServer(ctx, t)
    defer server.Close()
    
    t.Run("FileUpload", func(t *testing.T) {
        // Test large file upload
        testFile := createTestAudioFile(t, 100*1024*1024) // 100MB
        defer os.Remove(testFile)
        
        start := time.Now()
        err := uploadFile(server.URL+"/upload", testFile)
        duration := time.Since(start)
        
        if err != nil {
            t.Fatalf("Upload failed: %v", err)
        }
        
        // Pi performance expectations
        if duration > 2*time.Minute {
            t.Errorf("Upload too slow: %v (expected < 2m)", duration)
        }
    })
    
    t.Run("ConcurrentUploads", func(t *testing.T) {
        // Test multiple concurrent uploads
        const numUploads = 4 // Pi core count
        
        var wg sync.WaitGroup
        errors := make(chan error, numUploads)
        
        for i := 0; i < numUploads; i++ {
            wg.Add(1)
            go func(index int) {
                defer wg.Done()
                
                testFile := createTestAudioFile(t, 50*1024*1024) // 50MB
                defer os.Remove(testFile)
                
                err := uploadFile(server.URL+"/upload", testFile)
                if err != nil {
                    errors <- fmt.Errorf("upload %d failed: %v", index, err)
                }
            }(i)
        }
        
        wg.Wait()
        close(errors)
        
        for err := range errors {
            if err != nil {
                t.Error(err)
            }
        }
    })
    
    t.Run("ResourceMonitoring", func(t *testing.T) {
        // Monitor resource usage during test
        var maxMemory uint64
        var maxGoroutines int
        
        monitor := time.NewTicker(time.Second)
        defer monitor.Stop()
        
        done := make(chan bool)
        go func() {
            for {
                select {
                case <-monitor.C:
                    var m runtime.MemStats
                    runtime.ReadMemStats(&m)
                    
                    if m.HeapAlloc > maxMemory {
                        maxMemory = m.HeapAlloc
                    }
                    
                    goroutines := runtime.NumGoroutine()
                    if goroutines > maxGoroutines {
                        maxGoroutines = goroutines
                    }
                    
                case <-done:
                    return
                }
            }
        }()
        
        // Run some operations
        time.Sleep(10 * time.Second)
        done <- true
        
        t.Logf("Max memory usage: %d MB", maxMemory/1024/1024)
        t.Logf("Max goroutines: %d", maxGoroutines)
        
        // Pi limits validation
        if maxMemory > 1024*1024*1024 { // 1GB
            t.Errorf("Excessive memory usage: %d MB", maxMemory/1024/1024)
        }
        
        if maxGoroutines > 100 {
            t.Errorf("Too many goroutines: %d", maxGoroutines)
        }
    })
}
```

## Conclusion

This guide provides comprehensive best practices for Go development targeting Raspberry Pi deployment. Key takeaways:

1. **Always consider Pi constraints** - memory, CPU, storage, and thermal limits
2. **Optimize memory usage** - preallocate, pool resources, stream large data
3. **Limit concurrency** - use worker pools, respect CPU core count
4. **Buffer I/O operations** - SD cards perform better with larger buffers
5. **Monitor performance** - track metrics and detect resource exhaustion
6. **Test thoroughly** - benchmark on actual Pi hardware
7. **Plan for failure** - robust error handling and resource cleanup

Following these practices will ensure your Go applications run efficiently and reliably on Raspberry Pi hardware, providing optimal performance for the sermon uploader project's audio file processing requirements.

For additional resources and updates to these best practices, refer to:
- [Go Performance Debugging](https://golang.org/doc/diagnostics.html)
- [Raspberry Pi Documentation](https://www.raspberrypi.org/documentation/)
- [Project Benchmarks](../benchmarks/README.md)
- [Performance Monitoring Tools](../tools/)