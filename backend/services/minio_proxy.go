package services

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
)

// ProxyUploadFile uploads a file to MinIO through the backend proxy
// This method is used to bypass browser Private Network Access restrictions
func (m *MinIOService) ProxyUploadFile(ctx context.Context, filename string, reader io.Reader, size int64, contentType string) (minio.UploadInfo, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	opts := minio.PutObjectOptions{
		ContentType: contentType,
	}

	// Use the MinIO client to upload
	info, err := m.client.PutObject(
		ctx,
		m.config.MinioBucket,
		filename,
		reader,
		size,
		opts,
	)

	if err != nil {
		return minio.UploadInfo{}, fmt.Errorf("failed to upload file to MinIO: %w", err)
	}

	return info, nil
}

// StreamUploadFile handles streaming uploads for very large files
func (m *MinIOService) StreamUploadFile(ctx context.Context, filename string, reader io.Reader, contentType string) (minio.UploadInfo, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	opts := minio.PutObjectOptions{
		ContentType: contentType,
	}

	// Use -1 for unknown size (streaming)
	info, err := m.client.PutObject(
		ctx,
		m.config.MinioBucket,
		filename,
		reader,
		-1,
		opts,
	)

	if err != nil {
		return minio.UploadInfo{}, fmt.Errorf("failed to stream upload file to MinIO: %w", err)
	}

	return info, nil
}

// ProcessUploadedFile processes a file after it has been uploaded
// This includes extracting metadata, checking for duplicates, etc.
func (m *MinIOService) ProcessUploadedFile(ctx context.Context, filename string) error {
	// Get file info from MinIO
	stat, err := m.client.StatObject(ctx, m.config.MinioBucket, filename, minio.StatObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Extract and store metadata
	metadata := make(map[string]string)
	metadata["upload_time"] = time.Now().Format(time.RFC3339)
	metadata["size"] = fmt.Sprintf("%d", stat.Size)
	metadata["content_type"] = stat.ContentType
	metadata["etag"] = stat.ETag

	// For now, just log the metadata processing
	// In production, this would update metadata in MinIO or a database
	return nil
}

// GetBucketName returns the bucket name (for testing)
func (m *MinIOService) GetBucketName() string {
	return m.config.MinioBucket
}