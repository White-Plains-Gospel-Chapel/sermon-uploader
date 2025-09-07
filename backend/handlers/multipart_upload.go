package handlers

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Constants for multipart upload
const (
	MinChunkSize       = 5 * 1024 * 1024      // 5MB - MinIO minimum
	DefaultChunkSize   = 5 * 1024 * 1024      // Using 5MB as per your decision
	MaxUploadSize      = 5 * 1024 * 1024 * 1024 // 5GB max file size
	PresignedURLExpiry = time.Hour            // 1 hour expiry for presigned URLs
	SessionTimeout     = 24 * time.Hour       // 24 hour session timeout
)

// Upload session management
type UploadSession struct {
	UploadID      string          `json:"uploadId"`
	Filename      string          `json:"filename"`
	FileSize      int64           `json:"fileSize"`
	ChunkSize     int64           `json:"chunkSize"`
	TotalParts    int             `json:"totalParts"`
	UploadedParts []CompletedPart `json:"uploadedParts"`
	CreatedAt     time.Time       `json:"createdAt"`
	LastActivity  time.Time       `json:"lastActivity"`
	FileHash      string          `json:"fileHash"`
	Status        string          `json:"status"` // "active", "completed", "aborted"
}

type CompletedPart struct {
	PartNumber int       `json:"partNumber"`
	ETag       string    `json:"etag"`
	Size       int64     `json:"size"`
	UploadedAt time.Time `json:"uploadedAt"`
}

// Request/Response types
type InitMultipartRequest struct {
	Filename  string `json:"filename" validate:"required"`
	FileSize  int64  `json:"fileSize" validate:"required,min=1"`
	ChunkSize int64  `json:"chunkSize,omitempty"`
	FileHash  string `json:"fileHash" validate:"required"`
}

type InitMultipartResponse struct {
	UploadID   string `json:"uploadId"`
	TotalParts int    `json:"totalParts"`
	ChunkSize  int64  `json:"chunkSize"`
}

type PresignedURLResponse struct {
	URL        string `json:"url"`
	PartNumber int    `json:"partNumber"`
	ExpiresAt  int64  `json:"expiresAt"`
}

type CompleteMultipartRequest struct {
	UploadID string          `json:"uploadId" validate:"required"`
	Parts    []CompletedPart `json:"parts" validate:"required,min=1"`
}

// MultipartUploadHandler handles multipart uploads with presigned URLs
type MultipartUploadHandler struct {
	minioClient     *minio.Client
	bucket          string
	uploadSessions  map[string]*UploadSession
	sessionMutex    sync.RWMutex
	uploadSemaphore chan struct{} // Limit concurrent uploads
}

// NewMultipartUploadHandler creates a new multipart upload handler
func NewMultipartUploadHandler(minioEndpoint, accessKey, secretKey, bucket string, secure bool) (*MultipartUploadHandler, error) {
	// Create MinIO client options
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	}
	
	// Only set custom transport for HTTPS with self-signed certs
	if secure {
		opts.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // For self-signed certs
			},
		}
	}

	minioClient, err := minio.New(minioEndpoint, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %v", err)
	}

	// Create bucket if it doesn't exist
	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %v", err)
	}
	if !exists {
		err = minioClient.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %v", err)
		}
	}

	return &MultipartUploadHandler{
		minioClient:     minioClient,
		bucket:          bucket,
		uploadSessions:  make(map[string]*UploadSession),
		uploadSemaphore: make(chan struct{}, 1), // Conservative: 1 file at a time
	}, nil
}

// InitiateMultipartUpload starts a new multipart upload session
func (h *MultipartUploadHandler) InitiateMultipartUpload(c *fiber.Ctx) error {
	var req InitMultipartRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request format",
		})
	}

	// Validate file size
	if req.FileSize > MaxUploadSize {
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("File size exceeds maximum of %d GB", MaxUploadSize/(1024*1024*1024)),
		})
	}

	// Use 5MB chunks as per decision
	if req.ChunkSize == 0 {
		req.ChunkSize = DefaultChunkSize
	}
	if req.ChunkSize < MinChunkSize {
		req.ChunkSize = MinChunkSize
	}

	// Check for duplicates using file hash
	h.sessionMutex.RLock()
	for _, session := range h.uploadSessions {
		if session.FileHash == req.FileHash && session.Status == "completed" {
			h.sessionMutex.RUnlock()
			return c.Status(409).JSON(fiber.Map{
				"error":       true,
				"isDuplicate": true,
				"message":     "File already uploaded",
				"filename":    session.Filename,
			})
		}
	}
	h.sessionMutex.RUnlock()

	// Check if file exists in MinIO
	ctx := context.Background()
	_, err := h.minioClient.StatObject(ctx, h.bucket, req.Filename, minio.StatObjectOptions{})
	if err == nil {
		return c.Status(409).JSON(fiber.Map{
			"error":       true,
			"isDuplicate": true,
			"message":     "File already exists in storage",
			"filename":    req.Filename,
		})
	}

	// Try to acquire upload slot (server-controlled queue)
	select {
	case h.uploadSemaphore <- struct{}{}:
		// Got a slot, proceed
		defer func() { <-h.uploadSemaphore }()
	default:
		// No slots available, queue the request
		return c.Status(429).JSON(fiber.Map{
			"error":       true,
			"message":     "Upload queue is full, please retry",
			"retry_after": 5, // seconds
		})
	}

	// For MinIO SDK v7, we'll use a different approach
	// We'll generate a unique upload ID ourselves and use presigned URLs
	uploadID := fmt.Sprintf("upload_%s_%d", req.FileHash[:8], time.Now().Unix())

	// Calculate number of parts
	totalParts := int((req.FileSize + req.ChunkSize - 1) / req.ChunkSize)

	// Create and store session
	session := &UploadSession{
		UploadID:      uploadID,
		Filename:      req.Filename,
		FileSize:      req.FileSize,
		ChunkSize:     req.ChunkSize,
		TotalParts:    totalParts,
		UploadedParts: []CompletedPart{},
		CreatedAt:     time.Now(),
		LastActivity:  time.Now(),
		FileHash:      req.FileHash,
		Status:        "active",
	}

	h.sessionMutex.Lock()
	h.uploadSessions[uploadID] = session
	h.sessionMutex.Unlock()

	fmt.Printf("📤 Multipart upload initiated: %s (%d MB) - %d parts\n",
		req.Filename, req.FileSize/(1024*1024), totalParts)

	return c.JSON(InitMultipartResponse{
		UploadID:   uploadID,
		TotalParts: totalParts,
		ChunkSize:  req.ChunkSize,
	})
}

// GetPresignedURL generates a presigned URL for uploading a part
func (h *MultipartUploadHandler) GetPresignedURL(c *fiber.Ctx) error {
	uploadID := c.Query("uploadId")
	partNumber, err := strconv.Atoi(c.Query("partNumber"))
	if err != nil || partNumber < 1 {
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid part number",
		})
	}

	// Get session
	h.sessionMutex.RLock()
	session, exists := h.uploadSessions[uploadID]
	h.sessionMutex.RUnlock()

	if !exists {
		return c.Status(404).JSON(fiber.Map{
			"error":   true,
			"message": "Upload session not found",
		})
	}

	if session.Status != "active" {
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("Upload session is %s", session.Status),
		})
	}

	// Update last activity
	h.sessionMutex.Lock()
	session.LastActivity = time.Now()
	h.sessionMutex.Unlock()

	// For MinIO v7, we'll use regular presigned PUT URLs
	// Parts will be assembled server-side
	ctx := context.Background()
	
	// Generate a unique object name for this part
	partObjectName := fmt.Sprintf("%s.part%d", session.Filename, partNumber)
	
	// Generate presigned PUT URL
	presignedURL, err := h.minioClient.PresignedPutObject(ctx, h.bucket, partObjectName, PresignedURLExpiry)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("Failed to generate presigned URL: %v", err),
		})
	}

	return c.JSON(PresignedURLResponse{
		URL:        presignedURL.String(),
		PartNumber: partNumber,
		ExpiresAt:  time.Now().Add(PresignedURLExpiry).Unix(),
	})
}

// CompleteMultipartUpload assembles all parts into the final file
func (h *MultipartUploadHandler) CompleteMultipartUpload(c *fiber.Ctx) error {
	var req CompleteMultipartRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request format",
		})
	}

	// Get session
	h.sessionMutex.RLock()
	session, exists := h.uploadSessions[req.UploadID]
	h.sessionMutex.RUnlock()

	if !exists {
		return c.Status(404).JSON(fiber.Map{
			"error":   true,
			"message": "Upload session not found",
		})
	}

	// For MinIO v7, we'll compose the object from parts
	ctx := context.Background()
	
	// Create compose sources from uploaded parts
	sources := make([]minio.CopySrcOptions, 0, len(req.Parts))
	for _, part := range req.Parts {
		partObjectName := fmt.Sprintf("%s.part%d", session.Filename, part.PartNumber)
		src := minio.CopySrcOptions{
			Bucket: h.bucket,
			Object: partObjectName,
		}
		sources = append(sources, src)
	}

	// Compose the final object
	dst := minio.CopyDestOptions{
		Bucket: h.bucket,
		Object: session.Filename,
	}

	// Compose object from parts
	_, err := h.minioClient.ComposeObject(ctx, dst, sources...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("Failed to complete upload: %v", err),
		})
	}

	// Clean up part objects
	for _, part := range req.Parts {
		partObjectName := fmt.Sprintf("%s.part%d", session.Filename, part.PartNumber)
		_ = h.minioClient.RemoveObject(ctx, h.bucket, partObjectName, minio.RemoveObjectOptions{})
	}

	// Update session status
	h.sessionMutex.Lock()
	session.Status = "completed"
	session.LastActivity = time.Now()
	h.sessionMutex.Unlock()

	fmt.Printf("🎉 Upload completed: %s (%d MB)\n",
		session.Filename, session.FileSize/(1024*1024))

	return c.JSON(fiber.Map{
		"success":  true,
		"filename": session.Filename,
		"size":     session.FileSize,
		"message":  "Upload completed successfully",
	})
}

// ListParts returns the list of uploaded parts for resumability
func (h *MultipartUploadHandler) ListParts(c *fiber.Ctx) error {
	uploadID := c.Query("uploadId")

	// Get session
	h.sessionMutex.RLock()
	session, exists := h.uploadSessions[uploadID]
	h.sessionMutex.RUnlock()

	if !exists {
		return c.Status(404).JSON(fiber.Map{
			"error":   true,
			"message": "Upload session not found",
		})
	}

	// Check which parts exist
	ctx := context.Background()
	parts := []CompletedPart{}
	
	for i := 1; i <= session.TotalParts; i++ {
		partObjectName := fmt.Sprintf("%s.part%d", session.Filename, i)
		info, err := h.minioClient.StatObject(ctx, h.bucket, partObjectName, minio.StatObjectOptions{})
		if err == nil {
			parts = append(parts, CompletedPart{
				PartNumber: i,
				ETag:       info.ETag,
				Size:       info.Size,
				UploadedAt: info.LastModified,
			})
		}
	}

	return c.JSON(fiber.Map{
		"uploadId":      uploadID,
		"filename":      session.Filename,
		"totalParts":    session.TotalParts,
		"uploadedParts": parts,
	})
}

// AbortMultipartUpload cancels an ongoing multipart upload
func (h *MultipartUploadHandler) AbortMultipartUpload(c *fiber.Ctx) error {
	uploadID := c.Params("uploadId")

	// Get session
	h.sessionMutex.RLock()
	session, exists := h.uploadSessions[uploadID]
	h.sessionMutex.RUnlock()

	if !exists {
		return c.Status(404).JSON(fiber.Map{
			"error":   true,
			"message": "Upload session not found",
		})
	}

	// Clean up any uploaded parts
	ctx := context.Background()
	for i := 1; i <= session.TotalParts; i++ {
		partObjectName := fmt.Sprintf("%s.part%d", session.Filename, i)
		_ = h.minioClient.RemoveObject(ctx, h.bucket, partObjectName, minio.RemoveObjectOptions{})
	}

	// Update session status
	h.sessionMutex.Lock()
	session.Status = "aborted"
	delete(h.uploadSessions, uploadID)
	h.sessionMutex.Unlock()

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Upload aborted successfully",
	})
}

// ListActiveSessions returns all active upload sessions
func (h *MultipartUploadHandler) ListActiveSessions(c *fiber.Ctx) error {
	h.sessionMutex.RLock()
	defer h.sessionMutex.RUnlock()

	activeSessions := []fiber.Map{}
	for _, session := range h.uploadSessions {
		if session.Status == "active" {
			activeSessions = append(activeSessions, fiber.Map{
				"uploadId":     session.UploadID,
				"filename":     session.Filename,
				"fileSize":     session.FileSize,
				"progress":     float64(len(session.UploadedParts)) / float64(session.TotalParts) * 100,
				"createdAt":    session.CreatedAt,
				"lastActivity": session.LastActivity,
			})
		}
	}

	return c.JSON(fiber.Map{
		"sessions": activeSessions,
		"count":    len(activeSessions),
	})
}

// CleanupStaleSessions removes old upload sessions
func (h *MultipartUploadHandler) CleanupStaleSessions() {
	h.sessionMutex.Lock()
	defer h.sessionMutex.Unlock()

	ctx := context.Background()
	cutoff := time.Now().Add(-SessionTimeout)

	for uploadID, session := range h.uploadSessions {
		if session.LastActivity.Before(cutoff) && session.Status == "active" {
			// Clean up any uploaded parts
			for i := 1; i <= session.TotalParts; i++ {
				partObjectName := fmt.Sprintf("%s.part%d", session.Filename, i)
				_ = h.minioClient.RemoveObject(ctx, h.bucket, partObjectName, minio.RemoveObjectOptions{})
			}

			// Remove from sessions
			delete(h.uploadSessions, uploadID)

			fmt.Printf("🧹 Cleaned up stale upload: %s\n", session.Filename)
		}
	}
}