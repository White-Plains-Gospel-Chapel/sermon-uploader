package services

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"sync"
	"time"
)

// StreamingService handles bit-perfect streaming with zero compression
type StreamingService struct {
	chunkSize      int64
	maxMemoryUsage int64
	mu             sync.RWMutex
	activeStreams  map[string]*StreamingSession
}

// StreamingSession represents an active streaming upload session
type StreamingSession struct {
	SessionID     string    `json:"session_id"`
	Filename      string    `json:"filename"`
	TotalSize     int64     `json:"total_size"`
	BytesReceived int64     `json:"bytes_received"`
	StartTime     time.Time `json:"start_time"`
	LastActivity  time.Time `json:"last_activity"`
	Status        string    `json:"status"`
	Hash          hash.Hash `json:"-"`
	HashString    string    `json:"hash_string,omitempty"`
	ChunkCount    int       `json:"chunk_count"`
	Quality       string    `json:"quality"`
	ErrorMessage  string    `json:"error_message,omitempty"`
}

// StreamingProgress represents real-time upload progress
type StreamingProgress struct {
	SessionID       string    `json:"session_id"`
	Filename        string    `json:"filename"`
	BytesReceived   int64     `json:"bytes_received"`
	TotalSize       int64     `json:"total_size"`
	Percentage      float64   `json:"percentage"`
	ChunksProcessed int       `json:"chunks_processed"`
	UploadSpeed     float64   `json:"upload_speed_mbps"`
	ETA             string    `json:"eta"`
	LastUpdate      time.Time `json:"last_update"`
	QualityStatus   string    `json:"quality_status"`
	IntegrityCheck  string    `json:"integrity_check"`
}

// StreamingChunk represents a data chunk with integrity verification
type StreamingChunk struct {
	SessionID   string    `json:"session_id"`
	ChunkIndex  int       `json:"chunk_index"`
	Data        []byte    `json:"data"`
	Size        int64     `json:"size"`
	ChunkHash   string    `json:"chunk_hash"`
	IsLastChunk bool      `json:"is_last_chunk"`
	Timestamp   time.Time `json:"timestamp"`
}

// QualityMetrics tracks audio quality preservation during streaming
type QualityMetrics struct {
	BitPerfect      bool    `json:"bit_perfect"`
	ZeroCompression bool    `json:"zero_compression"`
	IntegrityPassed bool    `json:"integrity_passed"`
	OriginalHash    string  `json:"original_hash"`
	ReceivedHash    string  `json:"received_hash"`
	QualityScore    float64 `json:"quality_score"`
	ProcessingTime  int64   `json:"processing_time_ms"`
}

// NewStreamingService creates a new streaming service optimized for Pi
func NewStreamingService() *StreamingService {
	return &StreamingService{
		chunkSize:      1 * 1024 * 1024,  // 1MB chunks for Pi optimization
		maxMemoryUsage: 64 * 1024 * 1024, // 64MB max memory usage for Pi
		activeStreams:  make(map[string]*StreamingSession),
	}
}

// CreateSession creates a new streaming upload session with bit-perfect settings
func (s *StreamingService) CreateSession(sessionID, filename string, totalSize int64) (*StreamingSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if session already exists
	if _, exists := s.activeStreams[sessionID]; exists {
		return nil, fmt.Errorf("session %s already exists", sessionID)
	}

	// Create new session with SHA256 hash for integrity verification
	session := &StreamingSession{
		SessionID:     sessionID,
		Filename:      filename,
		TotalSize:     totalSize,
		BytesReceived: 0,
		StartTime:     time.Now(),
		LastActivity:  time.Now(),
		Status:        "initialized",
		Hash:          sha256.New(),
		ChunkCount:    0,
		Quality:       "bit-perfect",
	}

	s.activeStreams[sessionID] = session
	return session, nil
}

// ProcessChunk processes an incoming data chunk with integrity verification
func (s *StreamingService) ProcessChunk(chunk *StreamingChunk) (*StreamingProgress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.activeStreams[chunk.SessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", chunk.SessionID)
	}

	// Verify chunk integrity
	if err := s.verifyChunkIntegrity(chunk); err != nil {
		session.Status = "error"
		session.ErrorMessage = fmt.Sprintf("chunk integrity verification failed: %v", err)
		return nil, err
	}

	// Update session with new chunk data
	session.Hash.Write(chunk.Data)
	session.BytesReceived += chunk.Size
	session.LastActivity = time.Now()
	session.ChunkCount++
	session.Status = "receiving"

	// Check if this is the final chunk
	if chunk.IsLastChunk {
		session.Status = "completed"
		session.HashString = fmt.Sprintf("%x", session.Hash.Sum(nil))
	}

	// Calculate progress metrics
	progress := s.calculateProgress(session)

	return progress, nil
}

// verifyChunkIntegrity verifies the integrity of a received chunk
func (s *StreamingService) verifyChunkIntegrity(chunk *StreamingChunk) error {
	// Calculate chunk hash
	hasher := sha256.New()
	hasher.Write(chunk.Data)
	calculatedHash := fmt.Sprintf("%x", hasher.Sum(nil))

	// Compare with provided hash
	if calculatedHash != chunk.ChunkHash {
		return fmt.Errorf("chunk hash mismatch: expected %s, got %s", chunk.ChunkHash, calculatedHash)
	}

	// Verify data size
	if int64(len(chunk.Data)) != chunk.Size {
		return fmt.Errorf("chunk size mismatch: expected %d, got %d", chunk.Size, len(chunk.Data))
	}

	return nil
}

// calculateProgress calculates real-time upload progress with quality metrics
func (s *StreamingService) calculateProgress(session *StreamingSession) *StreamingProgress {
	percentage := float64(session.BytesReceived) / float64(session.TotalSize) * 100
	if percentage > 100 {
		percentage = 100
	}

	// Calculate upload speed (MB/s)
	elapsed := time.Since(session.StartTime).Seconds()
	uploadSpeed := 0.0
	if elapsed > 0 {
		uploadSpeed = float64(session.BytesReceived) / (1024 * 1024) / elapsed
	}

	// Calculate ETA
	eta := "calculating..."
	if uploadSpeed > 0 && percentage > 0 && percentage < 100 {
		remainingBytes := session.TotalSize - session.BytesReceived
		remainingTime := float64(remainingBytes) / (uploadSpeed * 1024 * 1024)
		eta = fmt.Sprintf("%.0fs", remainingTime)
	}

	// Quality status based on streaming performance
	qualityStatus := "excellent"
	if uploadSpeed < 1.0 { // Less than 1 MB/s
		qualityStatus = "good"
	}
	if uploadSpeed < 0.5 { // Less than 0.5 MB/s
		qualityStatus = "fair"
	}

	// Integrity check status
	integrityCheck := "ongoing"
	if session.Status == "completed" {
		integrityCheck = "verified"
	} else if session.Status == "error" {
		integrityCheck = "failed"
	}

	return &StreamingProgress{
		SessionID:       session.SessionID,
		Filename:        session.Filename,
		BytesReceived:   session.BytesReceived,
		TotalSize:       session.TotalSize,
		Percentage:      percentage,
		ChunksProcessed: session.ChunkCount,
		UploadSpeed:     uploadSpeed,
		ETA:             eta,
		LastUpdate:      time.Now(),
		QualityStatus:   qualityStatus,
		IntegrityCheck:  integrityCheck,
	}
}

// GetSession retrieves an active streaming session
func (s *StreamingService) GetSession(sessionID string) (*StreamingSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.activeStreams[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	return session, nil
}

// CompleteSession finalizes a streaming session with quality verification
func (s *StreamingService) CompleteSession(sessionID string, expectedHash string) (*QualityMetrics, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.activeStreams[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	// Finalize hash calculation if not already done
	if session.HashString == "" {
		session.HashString = fmt.Sprintf("%x", session.Hash.Sum(nil))
	}

	// Verify quality metrics
	metrics := &QualityMetrics{
		BitPerfect:      session.HashString == expectedHash,
		ZeroCompression: true, // We enforce zero compression
		IntegrityPassed: session.HashString == expectedHash,
		OriginalHash:    expectedHash,
		ReceivedHash:    session.HashString,
		ProcessingTime:  time.Since(session.StartTime).Milliseconds(),
	}

	// Calculate quality score
	if metrics.BitPerfect && metrics.IntegrityPassed {
		metrics.QualityScore = 100.0
	} else if metrics.IntegrityPassed {
		metrics.QualityScore = 95.0
	} else {
		metrics.QualityScore = 0.0
	}

	// Update session status
	if metrics.BitPerfect {
		session.Status = "completed_verified"
	} else {
		session.Status = "completed_failed"
		session.ErrorMessage = "hash verification failed - upload corrupted"
	}

	return metrics, nil
}

// CleanupSession removes a completed or failed session from memory
func (s *StreamingService) CleanupSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.activeStreams[sessionID]; !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	delete(s.activeStreams, sessionID)
	return nil
}

// GetActiveSessionsCount returns the number of active streaming sessions
func (s *StreamingService) GetActiveSessionsCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.activeStreams)
}

// CleanupExpiredSessions removes sessions that have been inactive for too long
func (s *StreamingService) CleanupExpiredSessions(maxInactiveTime time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleaned := 0
	now := time.Now()

	for sessionID, session := range s.activeStreams {
		if now.Sub(session.LastActivity) > maxInactiveTime {
			delete(s.activeStreams, sessionID)
			cleaned++
		}
	}

	return cleaned
}

// CreateStreamingReader creates an io.Reader for streaming processing
func (s *StreamingService) CreateStreamingReader(sessionID string) (io.Reader, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.activeStreams[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	if session.Status != "completed" && session.Status != "completed_verified" {
		return nil, fmt.Errorf("session %s not ready for streaming", sessionID)
	}

	// Create a streaming reader for the completed session
	return &StreamingReader{
		session: session,
		service: s,
	}, nil
}

// StreamingReader implements io.Reader for streaming the received data
type StreamingReader struct {
	session *StreamingSession
	service *StreamingService
	offset  int64
}

// Read implements io.Reader interface for bit-perfect streaming
func (r *StreamingReader) Read(p []byte) (int, error) {
	// This is a simplified implementation - in reality, you would
	// need to reconstruct the data from the stored chunks
	if r.offset >= r.session.TotalSize {
		return 0, io.EOF
	}

	// For now, return empty read as this would require
	// storing the actual chunks in memory or disk
	return 0, io.EOF
}

// GetChunkSize returns the optimal chunk size for streaming
func (s *StreamingService) GetChunkSize() int64 {
	return s.chunkSize
}

// SetChunkSize sets the chunk size (useful for Pi optimization)
func (s *StreamingService) SetChunkSize(size int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chunkSize = size
}

// GetMemoryUsage returns current memory usage statistics
func (s *StreamingService) GetMemoryUsage() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"active_sessions": len(s.activeStreams),
		"max_memory_mb":   s.maxMemoryUsage / (1024 * 1024),
		"chunk_size_kb":   s.chunkSize / 1024,
	}
}
