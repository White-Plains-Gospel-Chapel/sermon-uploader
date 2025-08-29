package services

import (
	"bytes"
	"context"
	"io"

	"github.com/minio/minio-go/v7"
)

// CheckDuplicateByFilename checks if exact filename exists (O(1) operation - very fast)
func (m *MinIOService) CheckDuplicateByFilename(filename string) (bool, error) {
	// Direct stat check - very fast even with millions of files
	// This is O(1) operation, not dependent on bucket size
	_, err := m.client.StatObject(
		context.Background(),
		m.config.MinioBucket,
		filename, // Store directly in bucket root
		minio.StatObjectOptions{},
	)

	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil // File doesn't exist
		}
		return false, err // Error
	}

	return true, nil // File exists - duplicate!
}

// UploadFileDirectly uploads file preserving exact quality (no compression)
func (m *MinIOService) UploadFileDirectly(fileData []byte, filename string) error {
	objectName := filename // Store directly in bucket root

	// Upload with NO compression - preserves exact WAV quality
	_, err := m.client.PutObject(
		context.Background(),
		m.config.MinioBucket,
		objectName,
		io.NopCloser(bytes.NewReader(fileData)),
		int64(len(fileData)),
		minio.PutObjectOptions{
			ContentType: "audio/wav",
			// No compression options - keeps original quality
		},
	)

	return err
}
