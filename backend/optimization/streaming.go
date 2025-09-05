package optimization

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"sync"
)

// StreamingHasher provides efficient streaming hash calculation
type StreamingHasher struct {
	hasher         hash.Hash
	bytesProcessed int64
	mu             sync.Mutex
}

// NewStreamingHasher creates a new streaming hasher
func NewStreamingHasher() *StreamingHasher {
	return &StreamingHasher{
		hasher: sha256.New(),
	}
}

// Write writes data to the hasher
func (sh *StreamingHasher) Write(data []byte) (int, error) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	n, err := sh.hasher.Write(data)
	if err == nil {
		sh.bytesProcessed += int64(n)
	}
	return n, err
}

// Sum returns the final hash as a hex string
func (sh *StreamingHasher) Sum() string {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	return fmt.Sprintf("%x", sh.hasher.Sum(nil))
}

// BytesProcessed returns the total bytes processed
func (sh *StreamingHasher) BytesProcessed() int64 {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	return sh.bytesProcessed
}

// Reset resets the hasher for reuse
func (sh *StreamingHasher) Reset() {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.hasher.Reset()
	sh.bytesProcessed = 0
}

// StreamingReader wraps an io.Reader to provide streaming capabilities with progress
type StreamingReader struct {
	reader           io.Reader
	totalSize        int64
	bytesRead        int64
	progressCallback func(bytesRead, totalSize int64)
	mu               sync.Mutex
}

// NewStreamingReader creates a new streaming reader with progress tracking
func NewStreamingReader(reader io.Reader, totalSize int64, progressCallback func(int64, int64)) *StreamingReader {
	return &StreamingReader{
		reader:           reader,
		totalSize:        totalSize,
		progressCallback: progressCallback,
	}
}

// Read implements io.Reader interface with progress tracking
func (sr *StreamingReader) Read(p []byte) (int, error) {
	n, err := sr.reader.Read(p)

	if n > 0 {
		sr.mu.Lock()
		sr.bytesRead += int64(n)
		bytesRead := sr.bytesRead
		totalSize := sr.totalSize
		sr.mu.Unlock()

		// Call progress callback if provided
		if sr.progressCallback != nil {
			sr.progressCallback(bytesRead, totalSize)
		}
	}

	return n, err
}

// BytesRead returns the total bytes read so far
func (sr *StreamingReader) BytesRead() int64 {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.bytesRead
}

// Progress returns the current progress as a percentage
func (sr *StreamingReader) Progress() float64 {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if sr.totalSize == 0 {
		return 0
	}
	return float64(sr.bytesRead) / float64(sr.totalSize) * 100
}

// ZeroCopyWriter implements zero-copy writing where possible
type ZeroCopyWriter struct {
	writer       io.Writer
	bytesWritten int64
	mu           sync.Mutex
}

// NewZeroCopyWriter creates a new zero-copy writer
func NewZeroCopyWriter(writer io.Writer) *ZeroCopyWriter {
	return &ZeroCopyWriter{
		writer: writer,
	}
}

// Write implements io.Writer interface
func (zcw *ZeroCopyWriter) Write(p []byte) (int, error) {
	n, err := zcw.writer.Write(p)

	if n > 0 {
		zcw.mu.Lock()
		zcw.bytesWritten += int64(n)
		zcw.mu.Unlock()
	}

	return n, err
}

// BytesWritten returns the total bytes written
func (zcw *ZeroCopyWriter) BytesWritten() int64 {
	zcw.mu.Lock()
	defer zcw.mu.Unlock()
	return zcw.bytesWritten
}

// StreamingCopier provides optimized copying between readers and writers
type StreamingCopier struct {
	bufferSize int
	pools      *ObjectPools
}

// NewStreamingCopier creates a new streaming copier
func NewStreamingCopier(bufferSize int, pools *ObjectPools) *StreamingCopier {
	if pools == nil {
		pools = GetGlobalPools()
	}

	return &StreamingCopier{
		bufferSize: bufferSize,
		pools:      pools,
	}
}

// Copy copies from src to dst using optimized streaming
func (sc *StreamingCopier) Copy(dst io.Writer, src io.Reader) (int64, error) {
	// Get buffer from pool
	buffer, release := sc.pools.GetBuffer(sc.bufferSize)
	defer release()

	return io.CopyBuffer(dst, src, buffer)
}

// CopyWithProgress copies with progress tracking
func (sc *StreamingCopier) CopyWithProgress(dst io.Writer, src io.Reader, totalSize int64, progressCallback func(int64, int64)) (int64, error) {
	// Wrap source with progress tracking
	progressReader := NewStreamingReader(src, totalSize, progressCallback)

	// Get buffer from pool
	buffer, release := sc.pools.GetBuffer(sc.bufferSize)
	defer release()

	return io.CopyBuffer(dst, progressReader, buffer)
}
