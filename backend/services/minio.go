package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"sermon-uploader/config"
)

type MinIOService struct {
	client *minio.Client
	config *config.Config
}

type FileMetadata struct {
	OriginalFilename string    `json:"original_filename"`
	RenamedFilename  string    `json:"renamed_filename"`
	FileHash         string    `json:"file_hash"`
	FileSize         int64     `json:"file_size"`
	UploadDate       time.Time `json:"upload_date"`
	ProcessingStatus string    `json:"processing_status"`
	AIAnalysis       struct {
		Speaker           *string `json:"speaker"`
		Title             *string `json:"title"`
		Theme             *string `json:"theme"`
		Transcript        *string `json:"transcript"`
		ProcessingStatus  string  `json:"processing_status"`
	} `json:"ai_analysis"`
}

func NewMinIOService(cfg *config.Config) *MinIOService {
	// Initialize MinIO client
	client, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOSecure,
	})
	if err != nil {
		log.Printf("Failed to initialize MinIO client: %v", err)
	}

	return &MinIOService{
		client: client,
		config: cfg,
	}
}

func (s *MinIOService) TestConnection() error {
	ctx := context.Background()
	_, err := s.client.ListBuckets(ctx)
	return err
}

func (s *MinIOService) GetClient() *minio.Client {
	return s.client
}

func (s *MinIOService) EnsureBucketExists() error {
	ctx := context.Background()
	
	exists, err := s.client.BucketExists(ctx, s.config.MinioBucket)
	if err != nil {
		return err
	}
	
	if !exists {
		err = s.client.MakeBucket(ctx, s.config.MinioBucket, minio.MakeBucketOptions{})
		if err != nil {
			return err
		}
		log.Printf("Created bucket: %s", s.config.MinioBucket)
	}
	
	return nil
}

func (s *MinIOService) GetFileCount() (int, error) {
	ctx := context.Background()
	
	count := 0
	objectCh := s.client.ListObjects(ctx, s.config.MinioBucket, minio.ListObjectsOptions{
		Recursive: true,
	})
	
	for object := range objectCh {
		if object.Err != nil {
			return 0, object.Err
		}
		if strings.HasSuffix(object.Key, ".wav") {
			count++
		}
	}
	
	return count, nil
}

func (s *MinIOService) GetExistingHashes() (map[string]bool, error) {
	ctx := context.Background()
	
	hashes := make(map[string]bool)
	objectCh := s.client.ListObjects(ctx, s.config.MinioBucket, minio.ListObjectsOptions{
		Recursive: true,
	})
	
	for object := range objectCh {
		if object.Err != nil {
			return nil, object.Err
		}
		
		if strings.HasSuffix(object.Key, ".wav") {
			// Get object metadata
			objInfo, err := s.client.StatObject(ctx, s.config.MinioBucket, object.Key, minio.StatObjectOptions{})
			if err != nil {
				continue
			}
			
			if hash, exists := objInfo.UserMetadata["X-Amz-Meta-File-Hash"]; exists {
				hashes[hash] = true
			}
		}
	}
	
	return hashes, nil
}

func (s *MinIOService) UploadFile(fileData []byte, originalFilename string) (*FileMetadata, error) {
	ctx := context.Background()
	
	// Calculate file hash
	hash := fmt.Sprintf("%x", sha256.Sum256(fileData))
	renamedFilename := s.getRenamedFilename(originalFilename)
	
	// Create metadata
	metadata := &FileMetadata{
		OriginalFilename: originalFilename,
		RenamedFilename:  renamedFilename,
		FileHash:         hash,
		FileSize:         int64(len(fileData)),
		UploadDate:       time.Now(),
		ProcessingStatus: "uploaded",
	}
	metadata.AIAnalysis.ProcessingStatus = "pending"
	
	// Upload WAV file
	reader := bytes.NewReader(fileData)
	userMetadata := map[string]string{
		"X-Amz-Meta-File-Hash":     hash,
		"X-Amz-Meta-Upload-Date":   metadata.UploadDate.Format(time.RFC3339),
		"X-Amz-Meta-Original-Name": originalFilename,
	}
	
	_, err := s.client.PutObject(ctx, s.config.MinioBucket, renamedFilename, reader, int64(len(fileData)), minio.PutObjectOptions{
		ContentType:  "audio/wav",
		UserMetadata: userMetadata,
	})
	if err != nil {
		return nil, err
	}
	
	// Upload metadata JSON
	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, err
	}
	
	metadataReader := bytes.NewReader(metadataJSON)
	_, err = s.client.PutObject(ctx, s.config.MinioBucket, "metadata/"+renamedFilename+".json", metadataReader, int64(len(metadataJSON)), minio.PutObjectOptions{
		ContentType: "application/json",
	})
	if err != nil {
		log.Printf("Failed to upload metadata for %s: %v", originalFilename, err)
	}
	
	return metadata, nil
}

func (s *MinIOService) ListFiles() ([]map[string]interface{}, error) {
	ctx := context.Background()
	
	var files []map[string]interface{}
	objectCh := s.client.ListObjects(ctx, s.config.MinioBucket, minio.ListObjectsOptions{
		Recursive: true,
	})
	
	for object := range objectCh {
		if object.Err != nil {
			return nil, object.Err
		}
		
		if strings.HasSuffix(object.Key, ".wav") {
			objInfo, err := s.client.StatObject(ctx, s.config.MinioBucket, object.Key, minio.StatObjectOptions{})
			if err != nil {
				continue
			}
			
			file := map[string]interface{}{
				"name":          object.Key, // Use the full object key as name
				"size":          object.Size,
				"last_modified": object.LastModified.Format(time.RFC3339),
				"metadata":      objInfo.UserMetadata,
			}
			files = append(files, file)
		}
	}
	
	return files, nil
}

func (s *MinIOService) CalculateFileHash(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func (s *MinIOService) getRenamedFilename(originalName string) string {
	parts := strings.Split(originalName, ".")
	if len(parts) > 1 {
		ext := parts[len(parts)-1]
		name := strings.Join(parts[:len(parts)-1], ".")
		return fmt.Sprintf("%s%s.%s", name, s.config.WAVSuffix, ext)
	}
	return originalName
}

func (s *MinIOService) getObjectPath(filename string) string {
	return filename // Store directly in bucket root, no subfolder
}

// DownloadFile downloads a file from MinIO to local filesystem for processing
func (s *MinIOService) DownloadFile(filename, localPath string) error {
	objectName := s.getObjectPath(filename)
	
	// Get the object from MinIO
	reader, err := s.client.GetObject(context.Background(), s.config.MinioBucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to get object from MinIO: %v", err)
	}
	defer reader.Close()
	
	// Create local file
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer localFile.Close()
	
	// Copy data from MinIO to local file
	_, err = io.Copy(localFile, reader)
	if err != nil {
		return fmt.Errorf("failed to download file: %v", err)
	}
	
	return nil
}

// StoreMetadata stores comprehensive metadata as object metadata in MinIO
func (s *MinIOService) StoreMetadata(filename string, metadata *AudioMetadata) error {
	objectName := s.getObjectPath(filename)
	
	// Convert metadata to key-value pairs for MinIO object metadata
	metadataMap := map[string]string{
		"duration":       fmt.Sprintf("%.2f", metadata.Duration),
		"duration_text":  metadata.DurationText,
		"codec":          metadata.Codec,
		"sample_rate":    fmt.Sprintf("%d", metadata.SampleRate),
		"channels":       fmt.Sprintf("%d", metadata.Channels),
		"bitrate":        fmt.Sprintf("%d", metadata.Bitrate),
		"bits_per_sample": fmt.Sprintf("%d", metadata.BitsPerSample),
		"is_lossless":    fmt.Sprintf("%t", metadata.IsLossless),
		"quality":        metadata.Quality,
		"is_valid":       fmt.Sprintf("%t", metadata.IsValid),
		"upload_time":    metadata.UploadTime.Format(time.RFC3339),
	}
	
	// Add optional metadata if present
	if metadata.Title != "" {
		metadataMap["title"] = metadata.Title
	}
	if metadata.Artist != "" {
		metadataMap["artist"] = metadata.Artist
	}
	if metadata.Album != "" {
		metadataMap["album"] = metadata.Album
	}
	if metadata.Date != "" {
		metadataMap["date"] = metadata.Date
	}
	if metadata.Genre != "" {
		metadataMap["genre"] = metadata.Genre
	}
	
	// Copy existing object with new metadata
	srcOpts := minio.CopySrcOptions{
		Bucket: s.config.MinioBucket,
		Object: objectName,
	}
	
	dstOpts := minio.CopyDestOptions{
		Bucket:          s.config.MinioBucket,
		Object:          objectName,
		UserMetadata:    metadataMap,
		ReplaceMetadata: true,
	}
	
	_, err := s.client.CopyObject(context.Background(), dstOpts, srcOpts)
	return err
}

// ClearBucket removes all objects from the bucket (dangerous operation)
func (s *MinIOService) ClearBucket() (*ClearBucketResult, error) {
	result := &ClearBucketResult{
		DeletedCount: 0,
		FailedCount:  0,
		Errors:       []string{},
	}
	
	// List all objects in the bucket
	objectCh := s.client.ListObjects(context.Background(), s.config.MinioBucket, minio.ListObjectsOptions{
		Recursive: true,
	})
	
	// Collect all object names
	var objectNames []string
	for object := range objectCh {
		if object.Err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to list object: %v", object.Err))
			result.FailedCount++
			continue
		}
		objectNames = append(objectNames, object.Key)
	}
	
	if len(objectNames) == 0 {
		return result, nil // Bucket is already empty
	}
	
	// Delete objects one by one for reliable error handling
	for _, objName := range objectNames {
		err := s.client.RemoveObject(context.Background(), s.config.MinioBucket, objName, minio.RemoveObjectOptions{})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to delete %s: %v", objName, err))
			result.FailedCount++
		} else {
			result.DeletedCount++
		}
	}
	
	return result, nil
}

// ClearBucketResult contains the results of a bucket clearing operation
type ClearBucketResult struct {
	DeletedCount int      `json:"deleted_count"`
	FailedCount  int      `json:"failed_count"`
	Errors       []string `json:"errors,omitempty"`
}

// CreateTempConnection creates a temporary MinIO connection for migration
func (s *MinIOService) CreateTempConnection(endpoint, accessKey, secretKey string) (*MinIOService, error) {
	// Remove protocol if present
	endpoint = strings.Replace(endpoint, "http://", "", 1)
	endpoint = strings.Replace(endpoint, "https://", "", 1)
	
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false, // Assume local network
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %v", err)
	}
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	_, err = client.ListBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MinIO: %v", err)
	}
	
	// Create temporary config
	tempConfig := &config.Config{
		MinioBucket: s.config.MinioBucket,
	}
	
	return &MinIOService{
		client: client,
		config: tempConfig,
	}, nil
}

// DownloadFile downloads a file from MinIO and returns the data
func (s *MinIOService) DownloadFile(filename string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	object, err := s.client.GetObject(ctx, s.config.MinioBucket, filename, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %v", err)
	}
	defer object.Close()
	
	data, err := io.ReadAll(object)
	if err != nil {
		return nil, fmt.Errorf("failed to read object data: %v", err)
	}
	
	return data, nil
}

// MigratePolicies migrates bucket policies and ensures proper permissions
func (s *MinIOService) MigratePolicies(sourceMinio *MinIOService) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	bucketName := s.config.MinioBucket
	
	// Get source bucket policy
	sourcePolicy, err := sourceMinio.client.GetBucketPolicy(ctx, bucketName)
	if err != nil {
		log.Printf("Warning: Could not get source bucket policy (this may be normal): %v", err)
		// Continue with default policy setup
	}
	
	// Ensure bucket exists in destination
	if err := s.EnsureBucketExists(); err != nil {
		return fmt.Errorf("failed to ensure bucket exists: %v", err)
	}
	
	// Apply source policy to destination, or set default public read policy
	if sourcePolicy != "" {
		log.Printf("Applying source bucket policy to destination")
		err = s.client.SetBucketPolicy(ctx, bucketName, sourcePolicy)
		if err != nil {
			log.Printf("Warning: Failed to set bucket policy: %v", err)
		}
	}
	
	// Set default public read policy for the bucket
	publicReadPolicy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": {"AWS": "*"},
				"Action": ["s3:GetObject"],
				"Resource": ["arn:aws:s3:::%s/*"]
			}
		]
	}`, bucketName)
	
	err = s.client.SetBucketPolicy(ctx, bucketName, publicReadPolicy)
	if err != nil {
		log.Printf("Warning: Failed to set public read policy: %v", err)
	} else {
		log.Printf("Applied public read policy to bucket: %s", bucketName)
	}
	
	return nil
}