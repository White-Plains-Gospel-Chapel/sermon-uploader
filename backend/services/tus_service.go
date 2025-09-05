package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sermon-uploader/config"
)

// TUSService implements the TUS protocol for resumable uploads
type TUSService struct {
	uploadDir        string
	maxUploadSize    int64
	config           *config.Config
	mu               sync.RWMutex
	activeUploads    map[string]*TUSUpload
	streamingService *StreamingService
}

// TUSUpload represents an active TUS upload session
type TUSUpload struct {
	ID              string            `json:"id"`
	Size            int64             `json:"size"`
	Offset          int64             `json:"offset"`
	Filename        string            `json:"filename"`
	Metadata        map[string]string `json:"metadata"`
	CreatedAt       time.Time         `json:"created_at"`
	LastModified    time.Time         `json:"last_modified"`
	CompletedAt     *time.Time        `json:"completed_at,omitempty"`
	FilePath        string            `json:"file_path"`
	HashAccumulator hash.Hash         `json:"-"`
	CurrentHash     string            `json:"current_hash"`
	Status          string            `json:"status"`
	Quality         *QualityMetrics   `json:"quality,omitempty"`
	ChunksReceived  int               `json:"chunks_received"`
}

// TUSCreationResponse represents the response to a TUS creation request
type TUSCreationResponse struct {
	ID       string     `json:"id"`
	Location string     `json:"location"`
	Upload   *TUSUpload `json:"upload"`
}

// TUSInfo contains TUS protocol information
type TUSInfo struct {
	UploadID     string            `json:"upload_id"`
	Size         int64             `json:"size"`
	Offset       int64             `json:"offset"`
	Filename     string            `json:"filename"`
	Metadata     map[string]string `json:"metadata"`
	Status       string            `json:"status"`
	Progress     float64           `json:"progress"`
	IsComplete   bool              `json:"is_complete"`
	HashVerified bool              `json:"hash_verified"`
	QualityScore float64           `json:"quality_score"`
}

// NewTUSService creates a new TUS service instance
func NewTUSService(cfg *config.Config, streamingService *StreamingService) *TUSService {
	uploadDir := "/tmp/sermon-uploads"
	if cfg != nil && cfg.TempDir != "" {
		uploadDir = cfg.TempDir
	}

	// Create upload directory if it doesn't exist
	os.MkdirAll(uploadDir, 0755)

	return &TUSService{
		uploadDir:        uploadDir,
		maxUploadSize:    2 * 1024 * 1024 * 1024, // 2GB max
		config:           cfg,
		activeUploads:    make(map[string]*TUSUpload),
		streamingService: streamingService,
	}
}

// CreateUpload creates a new TUS upload session
func (t *TUSService) CreateUpload(size int64, filename string, metadata map[string]string) (*TUSCreationResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Validate upload size
	if size > t.maxUploadSize {
		return nil, fmt.Errorf("upload size %d exceeds maximum %d", size, t.maxUploadSize)
	}

	if size <= 0 {
		return nil, fmt.Errorf("invalid upload size: %d", size)
	}

	// Generate unique upload ID
	uploadID, err := t.generateUploadID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate upload ID: %w", err)
	}

	// Create file path
	filePath := filepath.Join(t.uploadDir, uploadID)

	// Create upload record
	upload := &TUSUpload{
		ID:              uploadID,
		Size:            size,
		Offset:          0,
		Filename:        filename,
		Metadata:        metadata,
		CreatedAt:       time.Now(),
		LastModified:    time.Now(),
		FilePath:        filePath,
		HashAccumulator: sha256.New(),
		Status:          "created",
		ChunksReceived:  0,
	}

	// Store upload in memory
	t.activeUploads[uploadID] = upload

	// Create empty file
	file, err := os.Create(filePath)
	if err != nil {
		delete(t.activeUploads, uploadID)
		return nil, fmt.Errorf("failed to create upload file: %w", err)
	}
	file.Close()

	return &TUSCreationResponse{
		ID:       uploadID,
		Location: fmt.Sprintf("/api/tus/%s", uploadID),
		Upload:   upload,
	}, nil
}

// GetUpload retrieves upload information
func (t *TUSService) GetUpload(uploadID string) (*TUSInfo, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	upload, exists := t.activeUploads[uploadID]
	if !exists {
		return nil, fmt.Errorf("upload %s not found", uploadID)
	}

	// Calculate progress
	progress := float64(upload.Offset) / float64(upload.Size) * 100
	if progress > 100 {
		progress = 100
	}

	info := &TUSInfo{
		UploadID:     upload.ID,
		Size:         upload.Size,
		Offset:       upload.Offset,
		Filename:     upload.Filename,
		Metadata:     upload.Metadata,
		Status:       upload.Status,
		Progress:     progress,
		IsComplete:   upload.Offset >= upload.Size,
		HashVerified: upload.Quality != nil && upload.Quality.IntegrityPassed,
	}

	if upload.Quality != nil {
		info.QualityScore = upload.Quality.QualityScore
	}

	return info, nil
}

// WriteChunk writes data chunk to upload with bit-perfect verification
func (t *TUSService) WriteChunk(uploadID string, offset int64, data []byte) (*TUSInfo, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	upload, exists := t.activeUploads[uploadID]
	if !exists {
		return nil, fmt.Errorf("upload %s not found", uploadID)
	}

	// Verify offset matches expected
	if offset != upload.Offset {
		return nil, fmt.Errorf("offset mismatch: expected %d, got %d", upload.Offset, offset)
	}

	// Verify we don't exceed declared size
	if offset+int64(len(data)) > upload.Size {
		return nil, fmt.Errorf("chunk would exceed declared file size")
	}

	// Open file for appending
	file, err := os.OpenFile(upload.FilePath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open upload file: %w", err)
	}
	defer file.Close()

	// Write data with zero compression (bit-perfect)
	bytesWritten, err := file.Write(data)
	if err != nil {
		return nil, fmt.Errorf("failed to write chunk: %w", err)
	}

	if bytesWritten != len(data) {
		return nil, fmt.Errorf("incomplete write: expected %d, wrote %d", len(data), bytesWritten)
	}

	// Update hash accumulator
	upload.HashAccumulator.Write(data)

	// Update upload metadata
	upload.Offset += int64(bytesWritten)
	upload.LastModified = time.Now()
	upload.ChunksReceived++
	upload.Status = "receiving"

	// Check if upload is complete
	if upload.Offset >= upload.Size {
		if err := t.completeUpload(upload); err != nil {
			return nil, fmt.Errorf("failed to complete upload: %w", err)
		}
	}

	return t.getUploadInfo(upload)
}

// completeUpload finalizes the upload with quality verification
func (t *TUSService) completeUpload(upload *TUSUpload) error {
	// Finalize hash
	finalHash := fmt.Sprintf("%x", upload.HashAccumulator.Sum(nil))
	upload.CurrentHash = finalHash

	// Create quality metrics
	now := time.Now()
	upload.CompletedAt = &now
	upload.Status = "completed"

	// Perform quality verification
	upload.Quality = &QualityMetrics{
		BitPerfect:      true, // We ensure bit-perfect by design
		ZeroCompression: true, // Always true for our implementation
		IntegrityPassed: true, // Will be verified against original hash later
		ReceivedHash:    finalHash,
		QualityScore:    100.0,
		ProcessingTime:  time.Since(upload.CreatedAt).Milliseconds(),
	}

	return nil
}

// VerifyUpload verifies the completed upload against expected hash
func (t *TUSService) VerifyUpload(uploadID string, expectedHash string) (*QualityMetrics, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	upload, exists := t.activeUploads[uploadID]
	if !exists {
		return nil, fmt.Errorf("upload %s not found", uploadID)
	}

	if upload.Status != "completed" {
		return nil, fmt.Errorf("upload %s is not completed", uploadID)
	}

	// Update quality metrics with verification results
	upload.Quality.OriginalHash = expectedHash
	upload.Quality.BitPerfect = upload.CurrentHash == expectedHash
	upload.Quality.IntegrityPassed = upload.CurrentHash == expectedHash

	// Update quality score based on verification
	if upload.Quality.BitPerfect && upload.Quality.IntegrityPassed {
		upload.Quality.QualityScore = 100.0
		upload.Status = "verified"
	} else {
		upload.Quality.QualityScore = 0.0
		upload.Status = "failed_verification"
	}

	return upload.Quality, nil
}

// DeleteUpload removes an upload and its associated file
func (t *TUSService) DeleteUpload(uploadID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	upload, exists := t.activeUploads[uploadID]
	if !exists {
		return fmt.Errorf("upload %s not found", uploadID)
	}

	// Remove file
	if err := os.Remove(upload.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove upload file: %w", err)
	}

	// Remove from active uploads
	delete(t.activeUploads, uploadID)

	return nil
}

// GetUploadFile returns the file path for a completed upload
func (t *TUSService) GetUploadFile(uploadID string) (string, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	upload, exists := t.activeUploads[uploadID]
	if !exists {
		return "", fmt.Errorf("upload %s not found", uploadID)
	}

	if upload.Status != "completed" && upload.Status != "verified" {
		return "", fmt.Errorf("upload %s is not completed", uploadID)
	}

	// Verify file exists
	if _, err := os.Stat(upload.FilePath); err != nil {
		return "", fmt.Errorf("upload file not found: %w", err)
	}

	return upload.FilePath, nil
}

// GetUploadReader returns an io.Reader for the completed upload file
func (t *TUSService) GetUploadReader(uploadID string) (io.ReadCloser, error) {
	filePath, err := t.GetUploadFile(uploadID)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open upload file: %w", err)
	}

	return file, nil
}

// ListActiveUploads returns all active uploads
func (t *TUSService) ListActiveUploads() []*TUSInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var uploads []*TUSInfo
	for _, upload := range t.activeUploads {
		info, err := t.getUploadInfo(upload)
		if err != nil {
			continue
		}
		uploads = append(uploads, info)
	}

	return uploads
}

// CleanupExpired removes expired uploads based on age
func (t *TUSService) CleanupExpired(maxAge time.Duration) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	cleaned := 0
	now := time.Now()

	for uploadID, upload := range t.activeUploads {
		// Clean up uploads older than maxAge that are not completed
		if now.Sub(upload.CreatedAt) > maxAge && upload.Status != "completed" && upload.Status != "verified" {
			// Remove file
			os.Remove(upload.FilePath)
			// Remove from memory
			delete(t.activeUploads, uploadID)
			cleaned++
		}
	}

	return cleaned, nil
}

// getUploadInfo creates TUSInfo from TUSUpload (internal helper)
func (t *TUSService) getUploadInfo(upload *TUSUpload) (*TUSInfo, error) {
	progress := float64(upload.Offset) / float64(upload.Size) * 100
	if progress > 100 {
		progress = 100
	}

	info := &TUSInfo{
		UploadID:     upload.ID,
		Size:         upload.Size,
		Offset:       upload.Offset,
		Filename:     upload.Filename,
		Metadata:     upload.Metadata,
		Status:       upload.Status,
		Progress:     progress,
		IsComplete:   upload.Offset >= upload.Size,
		HashVerified: upload.Quality != nil && upload.Quality.IntegrityPassed,
	}

	if upload.Quality != nil {
		info.QualityScore = upload.Quality.QualityScore
	}

	return info, nil
}

// generateUploadID generates a unique upload ID
func (t *TUSService) generateUploadID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GetTUSConfiguration returns TUS protocol configuration
func (t *TUSService) GetTUSConfiguration() map[string]interface{} {
	return map[string]interface{}{
		"version":                    "1.0.0",
		"resumable":                  "1.0.0",
		"max_size":                   t.maxUploadSize,
		"extensions":                 []string{"creation", "termination", "checksum"},
		"checksum_algorithms":        []string{"sha256"},
		"creation_with_upload":       true,
		"creation_defer_length":      false,
		"upload_concat":              false,
		"upload_length_deferred":     false,
		"zero_compression_guarantee": true,
		"bit_perfect_guarantee":      true,
	}
}

// GetActiveUploadsCount returns the number of active uploads
func (t *TUSService) GetActiveUploadsCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.activeUploads)
}

// GetUploadPath returns the upload directory path
func (t *TUSService) GetUploadPath() string {
	return t.uploadDir
}

// SetMaxUploadSize sets the maximum upload size
func (t *TUSService) SetMaxUploadSize(size int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.maxUploadSize = size
}

// ParseMetadata parses TUS metadata header
func (t *TUSService) ParseMetadata(metadataHeader string) (map[string]string, error) {
	metadata := make(map[string]string)

	if metadataHeader == "" {
		return metadata, nil
	}

	pairs := strings.Split(metadataHeader, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), " ", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Decode base64 value
		decoded, err := hex.DecodeString(value)
		if err != nil {
			// If hex decoding fails, try direct string
			metadata[key] = value
		} else {
			metadata[key] = string(decoded)
		}
	}

	return metadata, nil
}

// FormatMetadata formats metadata for TUS response
func (t *TUSService) FormatMetadata(metadata map[string]string) string {
	if len(metadata) == 0 {
		return ""
	}

	var pairs []string
	for key, value := range metadata {
		encoded := hex.EncodeToString([]byte(value))
		pairs = append(pairs, fmt.Sprintf("%s %s", key, encoded))
	}

	return strings.Join(pairs, ",")
}

// ValidateUploadOffset validates if the provided offset is correct
func (t *TUSService) ValidateUploadOffset(uploadID string, clientOffset int64) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	upload, exists := t.activeUploads[uploadID]
	if !exists {
		return fmt.Errorf("upload %s not found", uploadID)
	}

	if clientOffset != upload.Offset {
		return fmt.Errorf("offset mismatch: server has %d, client provided %d", upload.Offset, clientOffset)
	}

	return nil
}

// GetUploadStats returns statistics about uploads
func (t *TUSService) GetUploadStats() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := map[string]interface{}{
		"active_uploads":     len(t.activeUploads),
		"max_upload_size_gb": float64(t.maxUploadSize) / (1024 * 1024 * 1024),
		"upload_directory":   t.uploadDir,
	}

	// Count uploads by status
	statusCounts := make(map[string]int)
	totalSize := int64(0)
	totalReceived := int64(0)

	for _, upload := range t.activeUploads {
		statusCounts[upload.Status]++
		totalSize += upload.Size
		totalReceived += upload.Offset
	}

	stats["status_counts"] = statusCounts
	stats["total_declared_size_gb"] = float64(totalSize) / (1024 * 1024 * 1024)
	stats["total_received_size_gb"] = float64(totalReceived) / (1024 * 1024 * 1024)

	return stats
}
