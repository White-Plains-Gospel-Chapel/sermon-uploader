package services

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
)

// GetFileStats returns detailed statistics for a specific file
func (s *MinIOService) GetFileStats(filename string) (map[string]interface{}, error) {
	ctx := context.Background()
	objectInfo, err := s.client.StatObject(ctx, s.config.MinioBucket, filename, minio.StatObjectOptions{})
	if err != nil {
		return nil, err
	}
	
	return map[string]interface{}{
		"size":         objectInfo.Size,
		"lastModified": objectInfo.LastModified,
		"contentType":  objectInfo.ContentType,
		"etag":         objectInfo.ETag,
		"metadata":     objectInfo.UserMetadata,
		"storageClass": objectInfo.StorageClass,
	}, nil
}

// DeleteFile removes a file from MinIO storage
func (s *MinIOService) DeleteFile(filename string) error {
	ctx := context.Background()
	err := s.client.RemoveObject(ctx, s.config.MinioBucket, filename, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete file %s: %w", filename, err)
	}
	
	// Also delete metadata if it exists
	metadataFilename := filename + ".json"
	_ = s.client.RemoveObject(ctx, s.config.MinioBucket, "metadata/"+metadataFilename, minio.RemoveObjectOptions{})
	
	return nil
}

// BatchUploadInfo tracks batch upload progress
type BatchUploadInfo struct {
	StartTime        time.Time
	EndTime          time.Time
	FileCount        int
	TotalSize        int64
	Completed        int
	Failed           int
	Duplicates       int
	InProgress       bool
	CurrentFile      string
	BytesTransferred int64
	CurrentFileIndex int
}