package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
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
	MinChunkSize     = 5 * 1024 * 1024  // 5MB - MinIO minimum
	DefaultChunkSize = 5 * 1024 * 1024  // Using 5MB as per your decision
	MaxUploadSize    = 5 * 1024 * 1024 * 1024 // 5GB max file size
	PresignedURLExpiry = time.Hour      // 1 hour expiry for presigned URLs
	SessionTimeout   = 24 * time.Hour   // 24 hour session timeout
)

// Upload session management
type UploadSession struct {
	UploadID      string           `json:"uploadId"`
	Filename      string           `json:"filename"`
	FileSize      int64            `json:"fileSize"`
	ChunkSize     int64            `json:"chunkSize"`
	TotalParts    int              `json:"totalParts"`
	UploadedParts []CompletedPart  `json:"uploadedParts"`
	CreatedAt     time.Time        `json:"createdAt"`
	LastActivity  time.Time        `json:"lastActivity"`
	FileHash      string           `json:"fileHash"`
	Status        string           `json:"status"` // "active", "completed", "aborted"
}

type CompletedPart struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
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

type PresignedURLRequest struct {
	UploadID   string `json:"uploadId" validate:"required"`
	PartNumber int    `json:"partNumber" validate:"required,min=1"`
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
	minioService    MinIOService // Your existing MinIO service
	bucket          string
	config          *Config
	uploadSessions  map[string]*UploadSession
	sessionMutex    sync.RWMutex
	uploadSemaphore chan struct{} // Limit concurrent uploads
}

// NewMultipartUploadHandler creates a new multipart upload handler
func NewMultipartUploadHandler(config *Config) (*MultipartUploadHandler, error) {
	// Create MinIO client with HTTPS support
	var transport *http.Transport
	if config.MinIOSecure {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // For self-signed certs
			},
		}
	}

	minioClient, err := minio.New(config.MinIOEndpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(config.MinIOAccessKey, config.MinIOSecretKey, ""),
		Secure:    config.MinIOSecure,
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %v", err)
	}

	// Create bucket if it doesn't exist
	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, config.MinioBucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %v", err)
	}
	if !exists {
		err = minioClient.MakeBucket(ctx, config.MinioBucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %v", err)
		}
	}

	return &MultipartUploadHandler{
		minioClient:     minioClient,
		bucket:          config.MinioBucket,
		config:          config,
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
			"error":   true,
			"message": "Upload queue is full, please retry",
			"retry_after": 5, // seconds
		})
	}

	// Initiate multipart upload with MinIO
	uploadID, err := h.minioClient.NewMultipartUpload(
		ctx,
		h.bucket,
		req.Filename,
		minio.PutObjectOptions{
			ContentType: "audio/wav",
			UserMetadata: map[string]string{
				"file_hash":     req.FileHash,
				"original_size": strconv.FormatInt(req.FileSize, 10),
				"uploaded_at":   time.Now().Format(time.RFC3339),
			},
		},
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("Failed to initiate upload: %v", err),
		})
	}

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

	// Generate presigned URL for this part
	// For multipart uploads, we need to use a different approach
	// MinIO doesn't directly support presigned URLs for multipart parts
	// We'll use the zero-memory proxy approach instead
	
	// Build the URL for our proxy endpoint
	scheme := "https"
	if !h.config.MinIOSecure {
		scheme = "http"
	}
	
	// Use the public endpoint for the URL
	proxyURL := fmt.Sprintf("%s://%s/api/upload/multipart/proxy?uploadId=%s&partNumber=%d",
		scheme, h.config.PublicMinIOEndpoint, uploadID, partNumber)

	return c.JSON(PresignedURLResponse{
		URL:        proxyURL,
		PartNumber: partNumber,
		ExpiresAt:  time.Now().Add(PresignedURLExpiry).Unix(),
	})
}

// ProxyPartUpload handles the actual part upload (proxy to MinIO)
func (h *MultipartUploadHandler) ProxyPartUpload(c *fiber.Ctx) error {
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

	// Stream the body directly to MinIO
	ctx := context.Background()
	body := c.Context().RequestBodyStream()
	contentLength := int64(c.Context().Request.Header.ContentLength())

	// Upload part to MinIO
	partInfo, err := h.minioClient.PutObjectPart(
		ctx,
		h.bucket,
		session.Filename,
		uploadID,
		partNumber,
		body,
		contentLength,
		"",  // MD5
		"",  // SHA256
		nil, // SSE
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("Failed to upload part: %v", err),
		})
	}

	// Record the uploaded part
	h.sessionMutex.Lock()
	session.UploadedParts = append(session.UploadedParts, CompletedPart{
		PartNumber: partNumber,
		ETag:       partInfo.ETag,
		Size:       partInfo.Size,
		UploadedAt: time.Now(),
	})
	session.LastActivity = time.Now()
	h.sessionMutex.Unlock()

	fmt.Printf("✅ Part %d/%d uploaded for %s\n", partNumber, session.TotalParts, session.Filename)

	return c.JSON(fiber.Map{
		"success":    true,
		"partNumber": partNumber,
		"etag":       partInfo.ETag,
	})
}

// CompleteMultipartUpload finalizes the multipart upload
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

	// Prepare parts for completion
	parts := make([]minio.CompletePart, len(req.Parts))
	for i, part := range req.Parts {
		parts[i] = minio.CompletePart{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		}
	}

	// Complete the multipart upload
	ctx := context.Background()
	_, err := h.minioClient.CompleteMultipartUpload(
		ctx,
		h.bucket,
		session.Filename,
		req.UploadID,
		parts,
		minio.PutObjectOptions{},
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("Failed to complete upload: %v", err),
		})
	}

	// Update session status
	h.sessionMutex.Lock()
	session.Status = "completed"
	session.LastActivity = time.Now()
	h.sessionMutex.Unlock()

	fmt.Printf("🎉 Upload completed: %s (%d MB)\n", 
		session.Filename, session.FileSize/(1024*1024))

	// Process metadata in background
	go func() {
		ctx := context.Background()
		if h.minioService != nil {
			_ = h.minioService.ProcessUploadedFile(ctx, session.Filename)
		}
	}()

	return c.JSON(fiber.Map{
		"success":  true,
		"filename": session.Filename,
		"size":     session.FileSize,
		"message":  "Upload completed successfully",
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

	// Abort the multipart upload in MinIO
	ctx := context.Background()
	err := h.minioClient.AbortMultipartUpload(ctx, h.bucket, session.Filename, uploadID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("Failed to abort upload: %v", err),
		})
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

	// List parts from MinIO
	ctx := context.Background()
	partsInfo, err := h.minioClient.ListObjectParts(
		ctx,
		h.bucket,
		session.Filename,
		uploadID,
		0,     // part number marker
		1000,  // max parts
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("Failed to list parts: %v", err),
		})
	}

	parts := make([]CompletedPart, len(partsInfo.Parts))
	for i, part := range partsInfo.Parts {
		parts[i] = CompletedPart{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
			Size:       part.Size,
			UploadedAt: part.LastModified,
		}
	}

	return c.JSON(fiber.Map{
		"uploadId":      uploadID,
		"filename":      session.Filename,
		"totalParts":    session.TotalParts,
		"uploadedParts": parts,
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
				"uploadId":   session.UploadID,
				"filename":   session.Filename,
				"fileSize":   session.FileSize,
				"progress":   float64(len(session.UploadedParts)) / float64(session.TotalParts) * 100,
				"createdAt":  session.CreatedAt,
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
			// Abort the upload in MinIO
			_ = h.minioClient.AbortMultipartUpload(ctx, h.bucket, session.Filename, uploadID)
			
			// Remove from sessions
			delete(h.uploadSessions, uploadID)
			
			fmt.Printf("🧹 Cleaned up stale upload: %s\n", session.Filename)
		}
	}
}