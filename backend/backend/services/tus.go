package services

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"sermon-uploader/config"
)

// TUSService handles resumable file uploads using the TUS protocol
type TUSService struct {
	config    *config.Config
	streaming *StreamingService
	tempDir   string
	uploads   map[string]*TUSUpload
	mu        sync.RWMutex
}

// TUSUpload represents an active upload session
type TUSUpload struct {
	ID       string            `json:"id"`
	Filename string            `json:"filename"`
	Size     int64             `json:"size"`
	Offset   int64             `json:"offset"`
	Metadata map[string]string `json:"metadata"`
	TempPath string            `json:"temp_path"`
	Created  time.Time         `json:"created"`
	Updated  time.Time         `json:"updated"`
	Hash     string            `json:"hash,omitempty"`
	mu       sync.Mutex
}

// TUSCreationResponse represents the response when creating a new upload
type TUSCreationResponse struct {
	ID       string            `json:"id"`
	Location string            `json:"location"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// TUSInfo contains information about an upload session
type TUSInfo struct {
	ID           string            `json:"id"`
	UploadID     string            `json:"upload_id"` // Alias for ID
	Filename     string            `json:"filename"`
	Size         int64             `json:"size"`
	Offset       int64             `json:"offset"`
	Metadata     map[string]string `json:"metadata"`
	Created      time.Time         `json:"created"`
	Updated      time.Time         `json:"updated"`
	Progress     float64           `json:"progress"`
	Status       string            `json:"status"`
	HashVerified bool              `json:"hash_verified"`
	QualityScore float64           `json:"quality_score"`
}

// TUSQuality represents the quality check results for an upload
type TUSQuality struct {
	IntegrityPassed bool   `json:"integrity_passed"`
	ExpectedHash    string `json:"expected_hash"`
	ActualHash      string `json:"actual_hash"`
	Message         string `json:"message,omitempty"`
}

// NewTUSService creates a new TUS service instance
func NewTUSService(cfg *config.Config, streaming *StreamingService) *TUSService {
	tempDir := "/tmp/tus-uploads"
	if cfg.TempDir != "" {
		tempDir = filepath.Join(cfg.TempDir, "tus-uploads")
	}

	// Ensure temp directory exists
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		fmt.Printf("Warning: failed to create TUS temp directory %s: %v\n", tempDir, err)
		tempDir = "/tmp/tus-uploads" // fallback
		os.MkdirAll(tempDir, 0755)
	}

	return &TUSService{
		config:    cfg,
		streaming: streaming,
		tempDir:   tempDir,
		uploads:   make(map[string]*TUSUpload),
	}
}

// CreateUpload creates a new upload session
func (t *TUSService) CreateUpload(size int64, filename string, metadata map[string]string) (*TUSCreationResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Generate unique upload ID
	uploadID := fmt.Sprintf("tus_%d_%x", time.Now().UnixNano(), sha256.Sum256([]byte(filename+fmt.Sprintf("%d", size))))[:16]

	// Create temp file path
	tempPath := filepath.Join(t.tempDir, uploadID)

	// Create upload session
	upload := &TUSUpload{
		ID:       uploadID,
		Filename: filename,
		Size:     size,
		Offset:   0,
		Metadata: metadata,
		TempPath: tempPath,
		Created:  time.Now(),
		Updated:  time.Now(),
	}

	// Create temp file
	file, err := os.Create(tempPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	file.Close()

	// Store upload session
	t.uploads[uploadID] = upload

	return &TUSCreationResponse{
		ID:       uploadID,
		Location: fmt.Sprintf("/uploads/%s", uploadID),
		Metadata: metadata,
	}, nil
}

// WriteChunk writes a chunk of data to an upload session
func (t *TUSService) WriteChunk(uploadID string, offset int64, data []byte) (*TUSInfo, error) {
	t.mu.RLock()
	upload, exists := t.uploads[uploadID]
	t.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("upload session not found: %s", uploadID)
	}

	upload.mu.Lock()
	defer upload.mu.Unlock()

	// Validate offset
	if offset != upload.Offset {
		return nil, fmt.Errorf("invalid offset: expected %d, got %d", upload.Offset, offset)
	}

	// Open temp file for writing
	file, err := os.OpenFile(upload.TempPath, os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open temp file: %w", err)
	}
	defer file.Close()

	// Seek to offset
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to offset: %w", err)
	}

	// Write data
	n, err := file.Write(data)
	if err != nil {
		return nil, fmt.Errorf("failed to write chunk: %w", err)
	}

	// Update offset
	upload.Offset += int64(n)
	upload.Updated = time.Now()

	// Calculate progress
	progress := float64(upload.Offset) / float64(upload.Size) * 100

	return &TUSInfo{
		ID:           upload.ID,
		UploadID:     upload.ID,
		Filename:     upload.Filename,
		Size:         upload.Size,
		Offset:       upload.Offset,
		Metadata:     upload.Metadata,
		Created:      upload.Created,
		Updated:      upload.Updated,
		Progress:     progress,
		Status:       "uploading",
		HashVerified: false,
		QualityScore: 0,
	}, nil
}

// GetUpload retrieves information about an upload session
func (t *TUSService) GetUpload(uploadID string) (*TUSInfo, error) {
	t.mu.RLock()
	upload, exists := t.uploads[uploadID]
	t.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("upload session not found: %s", uploadID)
	}

	upload.mu.Lock()
	defer upload.mu.Unlock()

	progress := float64(upload.Offset) / float64(upload.Size) * 100

	return &TUSInfo{
		ID:           upload.ID,
		UploadID:     upload.ID,
		Filename:     upload.Filename,
		Size:         upload.Size,
		Offset:       upload.Offset,
		Metadata:     upload.Metadata,
		Created:      upload.Created,
		Updated:      upload.Updated,
		Progress:     progress,
		Status:       "uploading",
		HashVerified: false,
		QualityScore: 0,
	}, nil
}

// VerifyUpload verifies the integrity of an uploaded file
func (t *TUSService) VerifyUpload(uploadID string, expectedHash string) (*TUSQuality, error) {
	t.mu.RLock()
	upload, exists := t.uploads[uploadID]
	t.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("upload session not found: %s", uploadID)
	}

	upload.mu.Lock()
	defer upload.mu.Unlock()

	// Check if upload is complete
	if upload.Offset != upload.Size {
		return &TUSQuality{
			IntegrityPassed: false,
			ExpectedHash:    expectedHash,
			Message:         fmt.Sprintf("upload incomplete: %d/%d bytes", upload.Offset, upload.Size),
		}, nil
	}

	// Calculate actual hash
	file, err := os.Open(upload.TempPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open temp file for verification: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil, fmt.Errorf("failed to calculate hash: %w", err)
	}

	actualHash := fmt.Sprintf("%x", hasher.Sum(nil))
	upload.Hash = actualHash

	// Verify hash
	integrityPassed := actualHash == expectedHash

	return &TUSQuality{
		IntegrityPassed: integrityPassed,
		ExpectedHash:    expectedHash,
		ActualHash:      actualHash,
		Message:         "",
	}, nil
}

// GetUploadReader returns a reader for the uploaded file
func (t *TUSService) GetUploadReader(uploadID string) (io.ReadCloser, error) {
	t.mu.RLock()
	upload, exists := t.uploads[uploadID]
	t.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("upload session not found: %s", uploadID)
	}

	file, err := os.Open(upload.TempPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open temp file: %w", err)
	}

	return file, nil
}

// DeleteUpload cleans up an upload session and its temporary files
func (t *TUSService) DeleteUpload(uploadID string) error {
	t.mu.Lock()
	upload, exists := t.uploads[uploadID]
	if exists {
		delete(t.uploads, uploadID)
	}
	t.mu.Unlock()

	if !exists {
		return fmt.Errorf("upload session not found: %s", uploadID)
	}

	// Remove temp file
	if err := os.Remove(upload.TempPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove temp file: %w", err)
	}

	return nil
}

// CleanupExpiredUploads removes expired upload sessions
func (t *TUSService) CleanupExpiredUploads(maxAge time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for uploadID, upload := range t.uploads {
		if now.Sub(upload.Updated) > maxAge {
			// Remove temp file
			os.Remove(upload.TempPath)
			// Remove from map
			delete(t.uploads, uploadID)
		}
	}
}

// ListUploads returns information about all active uploads
func (t *TUSService) ListUploads() []*TUSInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var uploads []*TUSInfo
	for _, upload := range t.uploads {
		upload.mu.Lock()
		progress := float64(upload.Offset) / float64(upload.Size) * 100
		info := &TUSInfo{
			ID:           upload.ID,
			UploadID:     upload.ID,
			Filename:     upload.Filename,
			Size:         upload.Size,
			Offset:       upload.Offset,
			Metadata:     upload.Metadata,
			Created:      upload.Created,
			Updated:      upload.Updated,
			Progress:     progress,
			Status:       "uploading",
			HashVerified: false,
			QualityScore: 0,
		}
		uploads = append(uploads, info)
		upload.mu.Unlock()
	}

	return uploads
}

// GetUploadSize returns the current size of an upload
func (t *TUSService) GetUploadSize(uploadID string) (int64, error) {
	t.mu.RLock()
	upload, exists := t.uploads[uploadID]
	t.mu.RUnlock()

	if !exists {
		return 0, fmt.Errorf("upload session not found: %s", uploadID)
	}

	upload.mu.Lock()
	defer upload.mu.Unlock()

	return upload.Offset, nil
}

// PatchUpload handles PATCH requests for resumable uploads
func (t *TUSService) PatchUpload(uploadID string, offset int64, data io.Reader) (*TUSInfo, error) {
	t.mu.RLock()
	upload, exists := t.uploads[uploadID]
	t.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("upload session not found: %s", uploadID)
	}

	upload.mu.Lock()
	defer upload.mu.Unlock()

	// Validate offset
	if offset != upload.Offset {
		return nil, fmt.Errorf("invalid offset: expected %d, got %d", upload.Offset, offset)
	}

	// Open temp file for writing
	file, err := os.OpenFile(upload.TempPath, os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open temp file: %w", err)
	}
	defer file.Close()

	// Seek to offset
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to offset: %w", err)
	}

	// Copy data with limited buffer to prevent memory issues
	buffer := make([]byte, 32*1024) // 32KB buffer
	n, err := io.CopyBuffer(file, data, buffer)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to write data: %w", err)
	}

	// Update offset
	upload.Offset += n
	upload.Updated = time.Now()

	// Calculate progress
	progress := float64(upload.Offset) / float64(upload.Size) * 100

	return &TUSInfo{
		ID:           upload.ID,
		UploadID:     upload.ID,
		Filename:     upload.Filename,
		Size:         upload.Size,
		Offset:       upload.Offset,
		Metadata:     upload.Metadata,
		Created:      upload.Created,
		Updated:      upload.Updated,
		Progress:     progress,
		Status:       "uploading",
		HashVerified: false,
		QualityScore: 0,
	}, nil
}
