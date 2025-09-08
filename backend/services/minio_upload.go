package services

import (
	"context"
	"io"
	"log/slog"

	"github.com/minio/minio-go/v7"
)

// PutFile uploads a file to MinIO using a simple interface
func (s *MinIOService) PutFile(ctx context.Context, bucket, objectName string, reader io.Reader, size int64, contentType string) (*minio.UploadInfo, error) {
	return s.PutFileWithHash(ctx, bucket, objectName, reader, size, contentType, "")
}

// PutFileWithContext uploads with context for cancellation
func (s *MinIOService) PutFileWithContext(ctx context.Context, bucket, objectName string, reader io.Reader, size int64, contentType string) (*minio.UploadInfo, error) {
	return s.PutFileWithHash(ctx, bucket, objectName, reader, size, contentType, "")
}

// PutFileWithHash uploads a file to MinIO with hash metadata
func (s *MinIOService) PutFileWithHash(ctx context.Context, bucket, objectName string, reader io.Reader, size int64, contentType string, fileHash string) (*minio.UploadInfo, error) {
	logger := slog.With(
		slog.String("method", "PutFile"),
		slog.String("bucket", bucket),
		slog.String("object", objectName),
		slog.Int64("size", size),
	)
	
	logger.Info("Starting file upload to MinIO")
	
	// Ensure bucket exists
	exists, err := s.client.BucketExists(ctx, bucket)
	if err != nil {
		logger.Error("Failed to check bucket existence", slog.String("error", err.Error()))
		return nil, err
	}
	
	if !exists {
		logger.Info("Creating bucket")
		err = s.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		if err != nil {
			logger.Error("Failed to create bucket", slog.String("error", err.Error()))
			return nil, err
		}
	}
	
	// Prepare upload options with hash metadata and performance optimizations
	opts := minio.PutObjectOptions{
		ContentType: contentType,
		// Use 64MB part size for better internet performance (default is 5MB)
		PartSize: 64 * 1024 * 1024,
		// Enable concurrent multipart uploads
		NumThreads: 10,
		// Disable SHA256 computation for speed (we already have our hash)
		DisableContentSha256: true,
		// Storage class
		StorageClass: "STANDARD",
	}
	
	// Add hash to metadata if provided
	if fileHash != "" {
		opts.UserMetadata = map[string]string{
			"X-File-Hash": fileHash,
		}
	}
	
	// Upload the file
	info, err := s.client.PutObject(ctx, bucket, objectName, reader, size, opts)
	
	if err != nil {
		logger.Error("Upload failed", slog.String("error", err.Error()))
		return nil, err
	}
	
	logger.Info("Upload successful",
		slog.String("etag", info.ETag),
		slog.Int64("size", info.Size),
	)
	
	// Convert to UploadInfo for compatibility
	uploadInfo := &minio.UploadInfo{
		Bucket: info.Bucket,
		Key:    info.Key,
		ETag:   info.ETag,
		Size:   info.Size,
	}
	
	return uploadInfo, nil
}