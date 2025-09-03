package services

import (
	"context"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// GeneratePresignedUploadURL creates a presigned URL for direct upload to MinIO
func (m *MinIOService) GeneratePresignedUploadURL(filename string, expiry time.Duration) (string, error) {
	// Ensure bucket exists
	if err := m.EnsureBucketExists(); err != nil {
		return "", err
	}

	ctx := context.Background()

	// If a public endpoint is configured, generate the signature against that host
	if m.config.PublicMinIOEndpoint != "" {
		pubClient, err := minio.New(m.config.PublicMinIOEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(m.config.MinIOAccessKey, m.config.MinIOSecretKey, ""),
			Secure: m.config.PublicMinIOSecure,
		})
		if err != nil {
			return "", err
		}
		presignedURL, err := pubClient.PresignedPutObject(ctx, m.config.MinioBucket, filename, expiry)
		if err != nil {
			return "", err
		}
		return presignedURL.String(), nil
	}

	// Default: use internal client/endpoint
	presignedURL, err := m.client.PresignedPutObject(ctx, m.config.MinioBucket, filename, expiry)
	if err != nil {
		return "", err
	}
	return presignedURL.String(), nil
}

// FileExists checks if a file exists in MinIO
func (m *MinIOService) FileExists(filename string) (bool, error) {
	_, err := m.client.StatObject(
		context.Background(),
		m.config.MinioBucket,
		filename,
		minio.StatObjectOptions{},
	)
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetFileInfo gets information about a file in MinIO
func (m *MinIOService) GetFileInfo(filename string) (*minio.ObjectInfo, error) {
	info, err := m.client.StatObject(
		context.Background(),
		m.config.MinioBucket,
		filename,
		minio.StatObjectOptions{},
	)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// GeneratePresignedDownloadURL creates a presigned URL for downloading
func (m *MinIOService) GeneratePresignedDownloadURL(filename string, expiry time.Duration) (string, error) {
	reqParams := make(url.Values)
	ctx := context.Background()

	if m.config.PublicMinIOEndpoint != "" {
		pubClient, err := minio.New(m.config.PublicMinIOEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(m.config.MinIOAccessKey, m.config.MinIOSecretKey, ""),
			Secure: m.config.PublicMinIOSecure,
		})
		if err != nil {
			return "", err
		}
		presignedURL, err := pubClient.PresignedGetObject(ctx, m.config.MinioBucket, filename, expiry, reqParams)
		if err != nil {
			return "", err
		}
		return presignedURL.String(), nil
	}

	presignedURL, err := m.client.PresignedGetObject(ctx, m.config.MinioBucket, filename, expiry, reqParams)
	if err != nil {
		return "", err
	}
	return presignedURL.String(), nil
}
