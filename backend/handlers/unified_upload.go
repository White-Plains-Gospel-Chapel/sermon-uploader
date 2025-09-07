package handlers

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// UnifiedUploadHandler handles all upload operations through a single endpoint
type UnifiedUploadHandler struct {
	minioClient     *minio.Client
	bucket          string
	publicEndpoint  string
	publicSecure    bool
	sessions        map[string]*UploadSession
	sessionMutex    sync.RWMutex
	uploadSemaphore chan struct{}
}

// UnifiedUploadRequest handles all upload operations
type UnifiedUploadRequest struct {
	// Required for all operations
	Action   string `json:"action" validate:"required"` // "start", "get_url", "complete", "abort", "status"
	UploadID string `json:"uploadId,omitempty"`
	
	// For "start" action
	Filename string `json:"filename,omitempty"`
	FileSize int64  `json:"fileSize,omitempty"`
	FileHash string `json:"fileHash,omitempty"`
	
	// For "get_url" action  
	PartNumber int `json:"partNumber,omitempty"`
	
	// For "complete" action
	Parts []CompletedPartInfo `json:"parts,omitempty"`
}

type CompletedPartInfo struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
}

// UnifiedUploadResponse returns appropriate data based on action
type UnifiedUploadResponse struct {
	Success bool        `json:"success"`
	Action  string      `json:"action"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// NewUnifiedUploadHandler creates a handler that manages all upload operations
func NewUnifiedUploadHandler(minioEndpoint, accessKey, secretKey, bucket string, secure bool) (*UnifiedUploadHandler, error) {
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	}
	
	if secure {
		opts.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	minioClient, err := minio.New(minioEndpoint, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %v", err)
	}

	// Ensure bucket exists
	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket: %v", err)
	}
	if !exists {
		err = minioClient.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %v", err)
		}
	}

	// Extract public endpoint
	publicEndpoint := minioEndpoint
	if idx := strings.Index(minioEndpoint, ":"); idx > 0 {
		publicEndpoint = "192.168.1.127" + minioEndpoint[idx:]
	}

	return &UnifiedUploadHandler{
		minioClient:     minioClient,
		bucket:          bucket,
		publicEndpoint:  publicEndpoint,
		publicSecure:    secure,
		sessions:        make(map[string]*UploadSession),
		uploadSemaphore: make(chan struct{}, 1), // 1 concurrent upload
	}, nil
}

// HandleUpload processes all upload operations through a single endpoint
func (h *UnifiedUploadHandler) HandleUpload(c *fiber.Ctx) error {
	var req UnifiedUploadRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(UnifiedUploadResponse{
			Success: false,
			Error:   "Invalid request format",
		})
	}

	switch req.Action {
	case "start":
		return h.startUpload(c, &req)
	case "get_url":
		return h.getUploadURL(c, &req)
	case "complete":
		return h.completeUpload(c, &req)
	case "abort":
		return h.abortUpload(c, &req)
	case "status":
		return h.getUploadStatus(c, &req)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(UnifiedUploadResponse{
			Success: false,
			Error:   fmt.Sprintf("Unknown action: %s", req.Action),
		})
	}
}

// startUpload initiates a new multipart upload
func (h *UnifiedUploadHandler) startUpload(c *fiber.Ctx, req *UnifiedUploadRequest) error {
	if req.Filename == "" || req.FileSize == 0 {
		return c.JSON(UnifiedUploadResponse{
			Success: false,
			Action:  "start",
			Error:   "Filename and fileSize are required",
		})
	}

	// Check for duplicate
	ctx := context.Background()
	_, err := h.minioClient.StatObject(ctx, h.bucket, req.Filename, minio.StatObjectOptions{})
	if err == nil {
		return c.JSON(UnifiedUploadResponse{
			Success: false,
			Action:  "start",
			Error:   "File already exists",
		})
	}

	// Calculate parts
	chunkSize := int64(5 * 1024 * 1024) // 5MB
	totalParts := int((req.FileSize + chunkSize - 1) / chunkSize)
	
	// Generate upload ID
	uploadID := fmt.Sprintf("upload_%s_%d", req.FileHash[:8], time.Now().Unix())
	
	// Create session
	session := &UploadSession{
		UploadID:      uploadID,
		Filename:      req.Filename,
		FileSize:      req.FileSize,
		ChunkSize:     chunkSize,
		TotalParts:    totalParts,
		UploadedParts: []CompletedPart{},
		CreatedAt:     time.Now(),
		LastActivity:  time.Now(),
		FileHash:      req.FileHash,
		Status:        "active",
	}

	h.sessionMutex.Lock()
	h.sessions[uploadID] = session
	h.sessionMutex.Unlock()

	return c.JSON(UnifiedUploadResponse{
		Success: true,
		Action:  "start",
		Data: map[string]interface{}{
			"uploadId":   uploadID,
			"totalParts": totalParts,
			"chunkSize":  chunkSize,
		},
	})
}

// getUploadURL generates a presigned URL for uploading a part
func (h *UnifiedUploadHandler) getUploadURL(c *fiber.Ctx, req *UnifiedUploadRequest) error {
	if req.UploadID == "" || req.PartNumber == 0 {
		return c.JSON(UnifiedUploadResponse{
			Success: false,
			Action:  "get_url",
			Error:   "uploadId and partNumber are required",
		})
	}

	h.sessionMutex.RLock()
	session, exists := h.sessions[req.UploadID]
	h.sessionMutex.RUnlock()

	if !exists {
		return c.JSON(UnifiedUploadResponse{
			Success: false,
			Action:  "get_url",
			Error:   "Upload session not found",
		})
	}

	// Generate part object name
	partObjectName := fmt.Sprintf("%s.part%d", session.Filename, req.PartNumber)
	
	// Generate presigned PUT URL
	ctx := context.Background()
	url, err := h.minioClient.PresignedPutObject(ctx, h.bucket, partObjectName, time.Hour)
	if err != nil {
		return c.JSON(UnifiedUploadResponse{
			Success: false,
			Action:  "get_url",
			Error:   fmt.Sprintf("Failed to generate URL: %v", err),
		})
	}

	// Replace with public endpoint
	urlStr := url.String()
	urlStr = strings.Replace(urlStr, "localhost", h.publicEndpoint[:strings.Index(h.publicEndpoint, ":")], 1)
	urlStr = strings.Replace(urlStr, "minio", h.publicEndpoint[:strings.Index(h.publicEndpoint, ":")], 1)
	
	if h.publicSecure {
		urlStr = strings.Replace(urlStr, "http://", "https://", 1)
	}

	// Update session activity
	h.sessionMutex.Lock()
	session.LastActivity = time.Now()
	h.sessionMutex.Unlock()

	return c.JSON(UnifiedUploadResponse{
		Success: true,
		Action:  "get_url",
		Data: map[string]interface{}{
			"url":        urlStr,
			"partNumber": req.PartNumber,
			"expiresIn":  3600, // seconds
		},
	})
}

// completeUpload assembles all parts into the final file
func (h *UnifiedUploadHandler) completeUpload(c *fiber.Ctx, req *UnifiedUploadRequest) error {
	if req.UploadID == "" || len(req.Parts) == 0 {
		return c.JSON(UnifiedUploadResponse{
			Success: false,
			Action:  "complete",
			Error:   "uploadId and parts are required",
		})
	}

	h.sessionMutex.RLock()
	session, exists := h.sessions[req.UploadID]
	h.sessionMutex.RUnlock()

	if !exists {
		return c.JSON(UnifiedUploadResponse{
			Success: false,
			Action:  "complete",
			Error:   "Upload session not found",
		})
	}

	// Compose object from parts
	ctx := context.Background()
	sources := make([]minio.CopySrcOptions, 0, len(req.Parts))
	
	for _, part := range req.Parts {
		partObjectName := fmt.Sprintf("%s.part%d", session.Filename, part.PartNumber)
		src := minio.CopySrcOptions{
			Bucket: h.bucket,
			Object: partObjectName,
		}
		sources = append(sources, src)
	}

	dst := minio.CopyDestOptions{
		Bucket: h.bucket,
		Object: session.Filename,
	}

	// Compose the final object
	_, err := h.minioClient.ComposeObject(ctx, dst, sources...)
	if err != nil {
		return c.JSON(UnifiedUploadResponse{
			Success: false,
			Action:  "complete",
			Error:   fmt.Sprintf("Failed to complete upload: %v", err),
		})
	}

	// Clean up part files
	for _, part := range req.Parts {
		partObjectName := fmt.Sprintf("%s.part%d", session.Filename, part.PartNumber)
		_ = h.minioClient.RemoveObject(ctx, h.bucket, partObjectName, minio.RemoveObjectOptions{})
	}

	// Update session status
	h.sessionMutex.Lock()
	session.Status = "completed"
	delete(h.sessions, req.UploadID)
	h.sessionMutex.Unlock()

	// Get file info
	info, _ := h.minioClient.StatObject(ctx, h.bucket, session.Filename, minio.StatObjectOptions{})

	return c.JSON(UnifiedUploadResponse{
		Success: true,
		Action:  "complete",
		Data: map[string]interface{}{
			"filename": session.Filename,
			"size":     info.Size,
			"etag":     info.ETag,
		},
	})
}

// abortUpload cancels an upload and cleans up
func (h *UnifiedUploadHandler) abortUpload(c *fiber.Ctx, req *UnifiedUploadRequest) error {
	if req.UploadID == "" {
		return c.JSON(UnifiedUploadResponse{
			Success: false,
			Action:  "abort",
			Error:   "uploadId is required",
		})
	}

	h.sessionMutex.Lock()
	session, exists := h.sessions[req.UploadID]
	if exists {
		session.Status = "aborted"
		delete(h.sessions, req.UploadID)
	}
	h.sessionMutex.Unlock()

	if !exists {
		return c.JSON(UnifiedUploadResponse{
			Success: false,
			Action:  "abort",
			Error:   "Upload session not found",
		})
	}

	// Clean up any uploaded parts
	ctx := context.Background()
	for i := 1; i <= session.TotalParts; i++ {
		partObjectName := fmt.Sprintf("%s.part%d", session.Filename, i)
		_ = h.minioClient.RemoveObject(ctx, h.bucket, partObjectName, minio.RemoveObjectOptions{})
	}

	return c.JSON(UnifiedUploadResponse{
		Success: true,
		Action:  "abort",
		Data: map[string]interface{}{
			"uploadId": req.UploadID,
			"message":  "Upload aborted successfully",
		},
	})
}

// getUploadStatus returns the current status of an upload
func (h *UnifiedUploadHandler) getUploadStatus(c *fiber.Ctx, req *UnifiedUploadRequest) error {
	if req.UploadID == "" {
		// Return all active sessions
		h.sessionMutex.RLock()
		sessions := make([]*UploadSession, 0, len(h.sessions))
		for _, session := range h.sessions {
			sessions = append(sessions, session)
		}
		h.sessionMutex.RUnlock()

		return c.JSON(UnifiedUploadResponse{
			Success: true,
			Action:  "status",
			Data:    sessions,
		})
	}

	// Return specific session
	h.sessionMutex.RLock()
	session, exists := h.sessions[req.UploadID]
	h.sessionMutex.RUnlock()

	if !exists {
		return c.JSON(UnifiedUploadResponse{
			Success: false,
			Action:  "status",
			Error:   "Upload session not found",
		})
	}

	// Check which parts exist
	ctx := context.Background()
	existingParts := []int{}
	for i := 1; i <= session.TotalParts; i++ {
		partObjectName := fmt.Sprintf("%s.part%d", session.Filename, i)
		_, err := h.minioClient.StatObject(ctx, h.bucket, partObjectName, minio.StatObjectOptions{})
		if err == nil {
			existingParts = append(existingParts, i)
		}
	}

	return c.JSON(UnifiedUploadResponse{
		Success: true,
		Action:  "status",
		Data: map[string]interface{}{
			"uploadId":      session.UploadID,
			"filename":      session.Filename,
			"fileSize":      session.FileSize,
			"totalParts":    session.TotalParts,
			"uploadedParts": existingParts,
			"status":        session.Status,
			"createdAt":     session.CreatedAt,
			"lastActivity":  session.LastActivity,
		},
	})
}

// CleanupStaleSessions removes old upload sessions
func (h *UnifiedUploadHandler) CleanupStaleSessions() {
	h.sessionMutex.Lock()
	defer h.sessionMutex.Unlock()

	ctx := context.Background()
	now := time.Now()
	
	for id, session := range h.sessions {
		if now.Sub(session.LastActivity) > 24*time.Hour {
			// Clean up parts
			for i := 1; i <= session.TotalParts; i++ {
				partObjectName := fmt.Sprintf("%s.part%d", session.Filename, i)
				_ = h.minioClient.RemoveObject(ctx, h.bucket, partObjectName, minio.RemoveObjectOptions{})
			}
			delete(h.sessions, id)
		}
	}
}