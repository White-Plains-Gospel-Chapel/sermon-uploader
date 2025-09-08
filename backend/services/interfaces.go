package services

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
)

// MinIOInterface defines the interface for MinIO operations
type MinIOInterface interface {
	PutFile(ctx context.Context, bucket, objectName string, reader io.Reader, size int64, contentType string) (*minio.UploadInfo, error)
	UploadFile(fileData []byte, originalFilename string) (*FileMetadata, error)
	TestConnection() error
	GetClient() *minio.Client
}