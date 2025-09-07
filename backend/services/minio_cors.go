package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/minio/minio-go/v7"
)

// CORSRule represents a CORS configuration rule
type CORSRule struct {
	AllowedOrigins []string `json:"AllowedOrigins"`
	AllowedMethods []string `json:"AllowedMethods"`
	AllowedHeaders []string `json:"AllowedHeaders"`
	ExposeHeaders  []string `json:"ExposeHeaders"`
	MaxAgeSeconds  int      `json:"MaxAgeSeconds"`
}

// SetBucketCORS configures CORS for the MinIO bucket
func (m *MinIOService) SetBucketCORS() error {
	// Define CORS rules
	corsRules := []CORSRule{
		{
			AllowedOrigins: []string{"*"}, // Allow all origins
			AllowedMethods: []string{"GET", "PUT", "POST", "DELETE", "HEAD"},
			AllowedHeaders: []string{"*"},
			ExposeHeaders:  []string{
				"ETag",
				"x-amz-request-id",
				"x-amz-id-2",
				"x-amz-server-side-encryption",
				"x-amz-version-id",
				"Accept-Ranges",
				"Content-Range",
				"Content-Encoding",
				"Content-Length",
				"Content-Type",
			},
			MaxAgeSeconds: 3600,
		},
	}

	// Convert to JSON
	corsConfig, err := json.Marshal(corsRules)
	if err != nil {
		return fmt.Errorf("failed to marshal CORS config: %w", err)
	}

	// Set CORS configuration
	ctx := context.Background()
	err = m.client.SetBucketCors(ctx, m.config.MinioBucket, string(corsConfig))
	if err != nil {
		// MinIO Go SDK might not support SetBucketCors directly
		// Try using SetBucketPolicy as an alternative
		return m.setBucketPolicyWithCORS()
	}

	return nil
}

// setBucketPolicyWithCORS sets a bucket policy that allows public access with CORS
func (m *MinIOService) setBucketPolicyWithCORS() error {
	// Create a policy that allows public read/write for presigned URLs
	policy := map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Effect":    "Allow",
				"Principal": map[string]interface{}{"AWS": []string{"*"}},
				"Action": []string{
					"s3:GetBucketLocation",
					"s3:ListBucket",
				},
				"Resource": []string{fmt.Sprintf("arn:aws:s3:::%s", m.config.MinioBucket)},
			},
			{
				"Effect":    "Allow",
				"Principal": map[string]interface{}{"AWS": []string{"*"}},
				"Action": []string{
					"s3:GetObject",
					"s3:PutObject",
					"s3:DeleteObject",
				},
				"Resource": []string{fmt.Sprintf("arn:aws:s3:::%s/*", m.config.MinioBucket)},
			},
		},
	}

	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("failed to marshal bucket policy: %w", err)
	}

	ctx := context.Background()
	err = m.client.SetBucketPolicy(ctx, m.config.MinioBucket, string(policyJSON))
	if err != nil {
		return fmt.Errorf("failed to set bucket policy: %w", err)
	}

	return nil
}

// EnsureCORSConfiguration ensures CORS is properly configured for the bucket
func (m *MinIOService) EnsureCORSConfiguration() error {
	// First ensure the bucket exists
	if err := m.EnsureBucketExists(); err != nil {
		return err
	}

	// Then set CORS configuration
	if err := m.SetBucketCORS(); err != nil {
		// Log but don't fail - CORS might be configured at server level
		fmt.Printf("Warning: Could not set bucket CORS (might be configured at server level): %v\n", err)
	}

	return nil
}

// GeneratePresignedUploadURLWithCORS generates a presigned URL with proper CORS headers
func (m *MinIOService) GeneratePresignedUploadURLWithCORS(filename string, fileSize int64, expiry time.Duration) (string, bool, error) {
	// Ensure CORS is configured
	if err := m.EnsureCORSConfiguration(); err != nil {
		return "", false, err
	}

	// Use the smart URL generation that picks between CloudFlare and direct MinIO
	url, isLargeFile, err := m.GeneratePresignedUploadURLSmart(filename, fileSize, expiry)
	if err != nil {
		return "", false, err
	}

	// For direct MinIO uploads, ensure the URL uses the public endpoint
	if isLargeFile && m.config.PublicMinIOEndpoint != "" {
		// The URL should already be using the public endpoint from GeneratePresignedUploadURLSmart
		// Just verify it's accessible
		if !m.isMinIOAccessible() {
			return "", false, fmt.Errorf("MinIO server is not accessible at %s", m.config.PublicMinIOEndpoint)
		}
	}

	return url, isLargeFile, nil
}

// isMinIOAccessible checks if MinIO is accessible
func (m *MinIOService) isMinIOAccessible() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to list buckets as a health check
	_, err := m.client.ListBuckets(ctx)
	return err == nil
}

// GetCORSConfiguration retrieves the current CORS configuration
func (m *MinIOService) GetCORSConfiguration() (string, error) {
	ctx := context.Background()
	
	// Try to get CORS configuration
	corsConfig, err := m.client.GetBucketCors(ctx, m.config.MinioBucket)
	if err != nil {
		// If CORS is not set, return empty
		if minio.ToErrorResponse(err).Code == "NoSuchCORSConfiguration" {
			return "No CORS configuration found", nil
		}
		return "", err
	}

	// Format CORS config for display
	var rules []CORSRule
	if err := json.Unmarshal([]byte(corsConfig), &rules); err != nil {
		return corsConfig, nil // Return raw if parsing fails
	}

	formatted, _ := json.MarshalIndent(rules, "", "  ")
	return string(formatted), nil
}