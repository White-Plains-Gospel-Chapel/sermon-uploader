package services

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// GetPublicEndpoint returns the public MinIO endpoint for global access
func (m *MinIOService) GetPublicEndpoint() string {
	// Check environment variable first
	if publicEndpoint := os.Getenv("MINIO_PUBLIC_ENDPOINT"); publicEndpoint != "" {
		return publicEndpoint
	}
	
	// Check if we have a public domain configured
	if publicDomain := os.Getenv("MINIO_PUBLIC_DOMAIN"); publicDomain != "" {
		return fmt.Sprintf("http://%s:9000", publicDomain)
	}
	
	// Fallback to internal endpoint
	return m.config.MinIOEndpoint
}

// CreatePublicClient creates a MinIO client for public endpoint
func (m *MinIOService) CreatePublicClient() (*minio.Client, error) {
	publicEndpoint := m.GetPublicEndpoint()
	
	// Parse endpoint to extract host and determine SSL
	endpoint := strings.TrimPrefix(publicEndpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	useSSL := strings.HasPrefix(publicEndpoint, "https://")
	
	// Create client with public endpoint
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(m.config.MinIOAccessKey, m.config.MinIOSecretKey, ""),
		Secure: useSSL,
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to create public MinIO client: %v", err)
	}
	
	return client, nil
}

// GeneratePublicPresignedPutURL generates a presigned URL using the public endpoint
// This allows global access to MinIO, bypassing CloudFlare
func (m *MinIOService) GeneratePublicPresignedPutURL(ctx context.Context, filename string, duration time.Duration) (string, error) {
	// Create public client if not exists
	publicClient, err := m.CreatePublicClient()
	if err != nil {
		return "", fmt.Errorf("failed to create public client: %v", err)
	}
	
	// Generate presigned URL with public endpoint
	presignedURL, err := publicClient.PresignedPutObject(ctx, m.config.MinioBucket, filename, duration)
	if err != nil {
		return "", fmt.Errorf("failed to generate public presigned URL: %v", err)
	}
	
	// Log for debugging
	fmt.Printf("🌐 Generated public MinIO URL: %s\n", presignedURL.String())
	fmt.Printf("📡 This URL bypasses CloudFlare completely\n")
	
	return presignedURL.String(), nil
}

// GeneratePublicPresignedGetURL generates a presigned GET URL using public endpoint
func (m *MinIOService) GeneratePublicPresignedGetURL(ctx context.Context, filename string, duration time.Duration) (string, error) {
	publicClient, err := m.CreatePublicClient()
	if err != nil {
		return "", fmt.Errorf("failed to create public client: %v", err)
	}
	
	presignedURL, err := publicClient.PresignedGetObject(ctx, m.config.MinioBucket, filename, duration, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate public presigned GET URL: %v", err)
	}
	
	return presignedURL.String(), nil
}

// TestPublicConnection tests the public MinIO endpoint
func (m *MinIOService) TestPublicConnection() error {
	publicClient, err := m.CreatePublicClient()
	if err != nil {
		return fmt.Errorf("failed to create public client: %v", err)
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Test connection by checking if bucket exists
	exists, err := publicClient.BucketExists(ctx, m.config.MinioBucket)
	if err != nil {
		return fmt.Errorf("public MinIO connection failed: %v", err)
	}
	
	if !exists {
		return fmt.Errorf("bucket '%s' not found on public endpoint", m.config.MinioBucket)
	}
	
	fmt.Printf("✅ Public MinIO connection successful: %s\n", m.GetPublicEndpoint())
	return nil
}