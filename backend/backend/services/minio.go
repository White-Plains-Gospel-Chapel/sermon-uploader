package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"sermon-uploader/config"
	"sermon-uploader/optimization"
)

type MinIOService struct {
	client         *minio.Client
	config         *config.Config
	pools          *optimization.ObjectPools
	copier         *optimization.StreamingCopier
	metrics        *MinIOMetrics
	connectionPool *ConnectionPoolManager
}

// RetryConfig defines retry behavior for MinIO operations
type RetryConfig struct {
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	RetryableErrors []string
}

// MinIOMetrics tracks MinIO performance and connection health
type MinIOMetrics struct {
	mu                  sync.RWMutex
	UploadLatency       time.Duration `json:"upload_latency"`
	DownloadLatency     time.Duration `json:"download_latency"`
	ConnectionErrors    int64         `json:"connection_errors"`
	RetryCount          int64         `json:"retry_count"`
	MultipartUploads    int64         `json:"multipart_uploads"`
	PartUploadTime      time.Duration `json:"avg_part_upload_time"`
	ConnectionPoolStats struct {
		Active int64 `json:"active_connections"`
		Idle   int64 `json:"idle_connections"`
		Total  int64 `json:"total_connections"`
	} `json:"connection_pool_stats"`
}

// ConnectionPoolManager monitors connection pool health
type ConnectionPoolManager struct {
	transport   *http.Transport
	mu          sync.RWMutex
	activeConns int64
	idleConns   int64
	totalConns  int64
}

type FileMetadata struct {
	OriginalFilename string    `json:"original_filename"`
	RenamedFilename  string    `json:"renamed_filename"`
	FileHash         string    `json:"file_hash"`
	FileSize         int64     `json:"file_size"`
	UploadDate       time.Time `json:"upload_date"`
	ProcessingStatus string    `json:"processing_status"`
	AIAnalysis       struct {
		Speaker          *string `json:"speaker"`
		Title            *string `json:"title"`
		Theme            *string `json:"theme"`
		Transcript       *string `json:"transcript"`
		ProcessingStatus string  `json:"processing_status"`
	} `json:"ai_analysis"`
}

// GetClient returns the MinIO client instance
func (s *MinIOService) GetClient() *minio.Client {
	return s.client
}

func NewMinIOService(cfg *config.Config) *MinIOService {
	// Create optimized HTTP transport for internet uploads with high bandwidth
	transport := &http.Transport{
		MaxIdleConns:          1000,             // Much higher for concurrent uploads
		MaxConnsPerHost:       100,              // Allow many concurrent parts
		MaxIdleConnsPerHost:   100,              // Keep many connections ready
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second, // Increase for internet latency
		TLSHandshakeTimeout:   30 * time.Second,
		ExpectContinueTimeout: 10 * time.Second,
		DisableCompression:    true, // WAV files don't compress
		ForceAttemptHTTP2:     false, // HTTP/1.1 often better for large files
	}

	// Initialize MinIO client with optimized transport
	client, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure:    cfg.MinIOSecure,
		Transport: transport,
	})
	if err != nil {
		log.Printf("Failed to initialize MinIO client: %v", err)
	}

	// Initialize optimization components
	pools := optimization.GetGlobalPools()
	copier := optimization.NewStreamingCopier(cfg.IOBufferSize, pools)

	// Initialize metrics and connection pool manager
	metrics := &MinIOMetrics{}
	connectionPool := &ConnectionPoolManager{
		transport: transport,
	}

	return &MinIOService{
		client:         client,
		config:         cfg,
		pools:          pools,
		copier:         copier,
		metrics:        metrics,
		connectionPool: connectionPool,
	}
}

func (s *MinIOService) TestConnection() error {
	ctx := context.Background()
	_, err := s.client.ListBuckets(ctx)
	return err
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
		"duration":        fmt.Sprintf("%.2f", metadata.Duration),
		"duration_text":   metadata.DurationText,
		"codec":           metadata.Codec,
		"sample_rate":     fmt.Sprintf("%d", metadata.SampleRate),
		"channels":        fmt.Sprintf("%d", metadata.Channels),
		"bitrate":         fmt.Sprintf("%d", metadata.Bitrate),
		"bits_per_sample": fmt.Sprintf("%d", metadata.BitsPerSample),
		"is_lossless":     fmt.Sprintf("%t", metadata.IsLossless),
		"quality":         metadata.Quality,
		"is_valid":        fmt.Sprintf("%t", metadata.IsValid),
		"upload_time":     metadata.UploadTime.Format(time.RFC3339),
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

// DownloadFileData downloads a file from MinIO and returns the data as bytes
// WARNING: This method loads entire file into memory - use DownloadFileStreaming for large files
func (s *MinIOService) DownloadFileData(filename string) ([]byte, error) {
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

// DownloadFileStreaming returns a streaming reader for the file without loading into memory
func (s *MinIOService) DownloadFileStreaming(filename string) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	object, err := s.client.GetObject(ctx, s.config.MinioBucket, filename, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %v", err)
	}

	return object, nil
}

// UploadFileStreaming uploads file using streaming with zero compression and optimizations
func (s *MinIOService) UploadFileStreaming(reader io.Reader, originalFilename string, size int64, fileHash string) (*FileMetadata, error) {
	// Use timeout context for Pi reliability
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute) // Allow 30 min for large files on Pi
	defer cancel()

	renamedFilename := s.getRenamedFilename(originalFilename)

	// Create metadata
	metadata := &FileMetadata{
		OriginalFilename: originalFilename,
		RenamedFilename:  renamedFilename,
		FileHash:         fileHash,
		FileSize:         size,
		UploadDate:       time.Now(),
		ProcessingStatus: "uploaded",
	}
	metadata.AIAnalysis.ProcessingStatus = "pending"

	// Create progress reader for large files
	var uploadReader io.Reader = reader
	if size > s.config.StreamingThreshold {
		// Wrap reader with progress tracking for large files
		uploadReader = optimization.NewStreamingReader(reader, size, func(bytesRead, totalSize int64) {
			// Progress callback - could broadcast to WebSocket if needed
			if totalSize > 0 {
				progress := float64(bytesRead) / float64(totalSize) * 100
				log.Printf("Upload progress for %s: %.1f%%", originalFilename, progress)
			}
		})
	}

	// Upload WAV file with zero compression settings optimized for Pi
	userMetadata := map[string]string{
		"X-Amz-Meta-File-Hash":        fileHash,
		"X-Amz-Meta-Upload-Date":      metadata.UploadDate.Format(time.RFC3339),
		"X-Amz-Meta-Original-Name":    originalFilename,
		"X-Amz-Meta-Quality":          "bit-perfect",
		"X-Amz-Meta-Compression":      "none",
		"X-Amz-Meta-Content-Encoding": "identity",
		"X-Amz-Meta-Storage-Class":    "STANDARD",
		"X-Amz-Meta-Pi-Optimized":     "true",
	}

	// Use application/octet-stream to ensure zero compression
	putOptions := minio.PutObjectOptions{
		ContentType:          "application/octet-stream",
		UserMetadata:         userMetadata,
		DisableMultipart:     size < 64*1024*1024, // Disable multipart for files < 64MB for better integrity
		DisableContentSha256: false,               // Keep SHA256 verification enabled
		SendContentMd5:       false,               // Disable MD5 to reduce Pi CPU load
	}

	// Adaptive part sizing based on file size - optimized for Pi memory constraints
	if size >= 64*1024*1024 {
		if size < 500*1024*1024 { // Files < 500MB
			putOptions.PartSize = uint64(8 * 1024 * 1024) // 8MB parts for smaller files
		} else if size < 1024*1024*1024 { // Files < 1GB
			putOptions.PartSize = uint64(16 * 1024 * 1024) // 16MB parts (current setting)
		} else { // Files > 1GB
			putOptions.PartSize = uint64(32 * 1024 * 1024) // 32MB parts for very large files
		}
	}

	// Track upload timing for metrics
	uploadStart := time.Now()

	// Use retry mechanism for reliable uploads
	err := s.uploadWithRetry(ctx, s.config.MinioBucket, renamedFilename, uploadReader, size, putOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file after retries: %w", err)
	}

	// Update metrics
	uploadDuration := time.Since(uploadStart)
	s.updateUploadMetrics(uploadDuration, size >= 64*1024*1024)

	// Upload metadata JSON with pooled buffer
	metadataBuffer := &bytes.Buffer{}
	encoder := json.NewEncoder(metadataBuffer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(metadata); err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	metadataReader := bytes.NewReader(metadataBuffer.Bytes())
	_, err = s.client.PutObject(ctx, s.config.MinioBucket, "metadata/"+renamedFilename+".json",
		metadataReader, int64(metadataBuffer.Len()), minio.PutObjectOptions{
			ContentType: "application/json",
		})
	if err != nil {
		log.Printf("Failed to upload metadata for %s: %v", originalFilename, err)
	}

	return metadata, nil
}

// UploadFileStreamingWithProgress uploads file with progress tracking
func (s *MinIOService) UploadFileStreamingWithProgress(reader io.Reader, originalFilename string, size int64, fileHash string, progressCallback func(bytesTransferred int64)) (*FileMetadata, error) {
	// Create progress reader wrapper
	progressReader := &ProgressReader{
		Reader:   reader,
		Size:     size,
		Callback: progressCallback,
	}

	return s.UploadFileStreaming(progressReader, originalFilename, size, fileHash)
}

// ProgressReader wraps an io.Reader to provide upload progress callbacks
type ProgressReader struct {
	Reader    io.Reader
	Size      int64
	BytesRead int64
	Callback  func(bytesTransferred int64)
}

// Read implements io.Reader with progress tracking
func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	if n > 0 {
		pr.BytesRead += int64(n)
		if pr.Callback != nil {
			pr.Callback(pr.BytesRead)
		}
	}
	return n, err
}

// VerifyUploadIntegrity verifies the integrity of an uploaded file
func (s *MinIOService) VerifyUploadIntegrity(filename string, expectedHash string) (*IntegrityResult, error) {
	ctx := context.Background()

	// Get object information
	objInfo, err := s.client.StatObject(ctx, s.config.MinioBucket, filename, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object info: %w", err)
	}

	// Get stored hash from metadata
	storedHash, exists := objInfo.UserMetadata["X-Amz-Meta-File-Hash"]
	if !exists {
		return &IntegrityResult{
			Filename:        filename,
			IntegrityPassed: false,
			ErrorMessage:    "no hash found in object metadata",
		}, nil
	}

	// Compare hashes
	integrityPassed := storedHash == expectedHash

	result := &IntegrityResult{
		Filename:        filename,
		ExpectedHash:    expectedHash,
		StoredHash:      storedHash,
		IntegrityPassed: integrityPassed,
		FileSize:        objInfo.Size,
		UploadTime:      objInfo.LastModified,
	}

	if !integrityPassed {
		result.ErrorMessage = fmt.Sprintf("hash mismatch: expected %s, got %s", expectedHash, storedHash)
	}

	return result, nil
}

// IntegrityResult represents the result of an integrity verification
type IntegrityResult struct {
	Filename        string    `json:"filename"`
	ExpectedHash    string    `json:"expected_hash"`
	StoredHash      string    `json:"stored_hash"`
	IntegrityPassed bool      `json:"integrity_passed"`
	FileSize        int64     `json:"file_size"`
	UploadTime      time.Time `json:"upload_time"`
	ErrorMessage    string    `json:"error_message,omitempty"`
}

// GetZeroCompressionStats returns statistics about zero-compression uploads
func (s *MinIOService) GetZeroCompressionStats() (*CompressionStats, error) {
	ctx := context.Background()

	stats := &CompressionStats{
		TotalFiles:           0,
		ZeroCompressionFiles: 0,
		BitPerfectFiles:      0,
		TotalSize:            0,
		Files:                make([]FileCompressionInfo, 0),
	}

	// List all objects
	objectCh := s.client.ListObjects(ctx, s.config.MinioBucket, minio.ListObjectsOptions{
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return nil, object.Err
		}

		// Skip metadata files
		if strings.HasSuffix(object.Key, ".json") {
			continue
		}

		// Get object metadata
		objInfo, err := s.client.StatObject(ctx, s.config.MinioBucket, object.Key, minio.StatObjectOptions{})
		if err != nil {
			continue
		}

		stats.TotalFiles++
		stats.TotalSize += object.Size

		// Check compression and quality flags
		compression := objInfo.UserMetadata["X-Amz-Meta-Compression"]
		quality := objInfo.UserMetadata["X-Amz-Meta-Quality"]

		isZeroCompression := compression == "none" || objInfo.ContentType == "application/octet-stream"
		isBitPerfect := quality == "bit-perfect"

		if isZeroCompression {
			stats.ZeroCompressionFiles++
		}
		if isBitPerfect {
			stats.BitPerfectFiles++
		}

		fileInfo := FileCompressionInfo{
			Filename:          object.Key,
			Size:              object.Size,
			ContentType:       objInfo.ContentType,
			Compression:       compression,
			Quality:           quality,
			IsZeroCompression: isZeroCompression,
			IsBitPerfect:      isBitPerfect,
			UploadDate:        object.LastModified,
		}

		if hash, exists := objInfo.UserMetadata["X-Amz-Meta-File-Hash"]; exists {
			fileInfo.Hash = hash
		}

		stats.Files = append(stats.Files, fileInfo)
	}

	return stats, nil
}

// CompressionStats provides statistics about file compression
type CompressionStats struct {
	TotalFiles           int                   `json:"total_files"`
	ZeroCompressionFiles int                   `json:"zero_compression_files"`
	BitPerfectFiles      int                   `json:"bit_perfect_files"`
	TotalSize            int64                 `json:"total_size_bytes"`
	Files                []FileCompressionInfo `json:"files"`
}

// FileCompressionInfo provides compression information for a single file
type FileCompressionInfo struct {
	Filename          string    `json:"filename"`
	Size              int64     `json:"size"`
	ContentType       string    `json:"content_type"`
	Compression       string    `json:"compression"`
	Quality           string    `json:"quality"`
	IsZeroCompression bool      `json:"is_zero_compression"`
	IsBitPerfect      bool      `json:"is_bit_perfect"`
	Hash              string    `json:"hash,omitempty"`
	UploadDate        time.Time `json:"upload_date"`
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

// uploadWithRetry performs upload with retry logic for better Pi reliability
func (s *MinIOService) uploadWithRetry(ctx context.Context, bucket, objectName string, reader io.Reader, size int64, opts minio.PutObjectOptions) error {
	retryConfig := RetryConfig{
		MaxRetries:      3,
		InitialDelay:    1 * time.Second,
		MaxDelay:        30 * time.Second,
		BackoffFactor:   2.0,
		RetryableErrors: []string{"connection reset", "timeout", "temporary failure", "context deadline exceeded"},
	}

	return s.retryOperation(ctx, func() error {
		_, err := s.client.PutObject(ctx, bucket, objectName, reader, size, opts)
		return err
	}, retryConfig)
}

// retryOperation executes an operation with exponential backoff retry
func (s *MinIOService) retryOperation(ctx context.Context, operation func() error, config RetryConfig) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Don't retry on final attempt
		if attempt == config.MaxRetries {
			break
		}

		// Check if error is retryable
		isRetryable := false
		errStr := strings.ToLower(err.Error())
		for _, retryableErr := range config.RetryableErrors {
			if strings.Contains(errStr, retryableErr) {
				isRetryable = true
				break
			}
		}

		if !isRetryable {
			break // Don't retry non-retryable errors
		}

		// Calculate delay with exponential backoff
		delay := time.Duration(float64(config.InitialDelay) * math.Pow(config.BackoffFactor, float64(attempt)))
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}

		// Update metrics
		s.metrics.mu.Lock()
		s.metrics.RetryCount++
		s.metrics.mu.Unlock()

		log.Printf("MinIO operation failed (attempt %d/%d), retrying in %v: %v", attempt+1, config.MaxRetries+1, delay, err)

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	// Update error metrics
	s.metrics.mu.Lock()
	s.metrics.ConnectionErrors++
	s.metrics.mu.Unlock()

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

// updateUploadMetrics updates upload performance metrics
func (s *MinIOService) updateUploadMetrics(duration time.Duration, isMultipart bool) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	s.metrics.UploadLatency = duration
	if isMultipart {
		s.metrics.MultipartUploads++
	}
}

// GetMetrics returns current MinIO performance metrics
func (s *MinIOService) GetMetrics() *MinIOMetrics {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()

	// Create a copy to avoid race conditions
	metrics := &MinIOMetrics{
		UploadLatency:    s.metrics.UploadLatency,
		DownloadLatency:  s.metrics.DownloadLatency,
		ConnectionErrors: s.metrics.ConnectionErrors,
		RetryCount:       s.metrics.RetryCount,
		MultipartUploads: s.metrics.MultipartUploads,
		PartUploadTime:   s.metrics.PartUploadTime,
	}

	// Get connection pool stats
	if s.connectionPool != nil {
		s.connectionPool.mu.RLock()
		metrics.ConnectionPoolStats.Active = s.connectionPool.activeConns
		metrics.ConnectionPoolStats.Idle = s.connectionPool.idleConns
		metrics.ConnectionPoolStats.Total = s.connectionPool.totalConns
		s.connectionPool.mu.RUnlock()
	}

	return metrics
}

// GetConnectionPoolStats returns current connection pool statistics
func (s *MinIOService) GetConnectionPoolStats() map[string]int64 {
	if s.connectionPool == nil {
		return map[string]int64{
			"active": 0,
			"idle":   0,
			"total":  0,
		}
	}

	s.connectionPool.mu.RLock()
	defer s.connectionPool.mu.RUnlock()

	return map[string]int64{
		"active": s.connectionPool.activeConns,
		"idle":   s.connectionPool.idleConns,
		"total":  s.connectionPool.totalConns,
	}
}

// GeneratePresignedPutURL generates a presigned URL for file upload
func (s *MinIOService) GeneratePresignedPutURL(filename string, expiryMinutes int) (string, error) {
	ctx := context.Background()
	expiry := time.Duration(expiryMinutes) * time.Minute

	// Use renamed filename for consistency
	renamedFilename := s.getRenamedFilename(filename)

	presignedURL, err := s.client.PresignedPutObject(ctx, s.config.MinioBucket, renamedFilename, expiry)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned PUT URL: %w", err)
	}

	return presignedURL.String(), nil
}

// GeneratePresignedGetURL generates a presigned URL for file download
func (s *MinIOService) GeneratePresignedGetURL(filename string, expiryHours int) (string, error) {
	ctx := context.Background()
	expiry := time.Duration(expiryHours) * time.Hour

	presignedURL, err := s.client.PresignedGetObject(ctx, s.config.MinioBucket, filename, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned GET URL: %w", err)
	}

	return presignedURL.String(), nil
}

// GeneratePresignedMultipartURLs generates presigned URLs for multipart upload
func (s *MinIOService) GeneratePresignedMultipartURLs(filename string, parts int, expiryMinutes int) (*MultipartUploadURLs, error) {
	ctx := context.Background()
	renamedFilename := s.getRenamedFilename(filename)

	// Create core client for multipart operations
	core, err := minio.NewCore(s.config.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(s.config.MinIOAccessKey, s.config.MinIOSecretKey, ""),
		Secure: s.config.MinIOSecure,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create core client: %w", err)
	}

	// Initiate multipart upload using core API
	uploadID, err := core.NewMultipartUpload(ctx, s.config.MinioBucket, renamedFilename, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
		UserMetadata: map[string]string{
			"X-Amz-Meta-Quality":      "bit-perfect",
			"X-Amz-Meta-Compression":  "none",
			"X-Amz-Meta-Pi-Optimized": "true",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initiate multipart upload: %w", err)
	}

	// Generate presigned URLs for each part
	urls := make([]PartURL, parts)
	expiry := time.Duration(expiryMinutes) * time.Minute

	for i := 1; i <= parts; i++ {
		reqParams := make(url.Values)
		reqParams.Set("partNumber", fmt.Sprintf("%d", i))
		reqParams.Set("uploadId", uploadID)

		presignedURL, err := s.client.Presign(ctx, "PUT", s.config.MinioBucket, renamedFilename, expiry, reqParams)
		if err != nil {
			return nil, fmt.Errorf("failed to generate presigned URL for part %d: %w", i, err)
		}

		urls[i-1] = PartURL{
			PartNumber: i,
			URL:        presignedURL.String(),
		}
	}

	return &MultipartUploadURLs{
		UploadID:      uploadID,
		Bucket:        s.config.MinioBucket,
		ObjectName:    renamedFilename,
		OriginalName:  filename,
		PartURLs:      urls,
		ExpiryMinutes: expiryMinutes,
		CreatedAt:     time.Now(),
	}, nil
}

// CompleteMultipartUpload completes a multipart upload
func (s *MinIOService) CompleteMultipartUpload(uploadID string, objectName string, parts []CompletedPart) error {
	ctx := context.Background()

	// Create core client for multipart completion
	core, err := minio.NewCore(s.config.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(s.config.MinIOAccessKey, s.config.MinIOSecretKey, ""),
		Secure: s.config.MinIOSecure,
	})
	if err != nil {
		return fmt.Errorf("failed to create core client: %w", err)
	}

	// Convert to MinIO CompletePart format
	completeParts := make([]minio.CompletePart, len(parts))
	for i, part := range parts {
		completeParts[i] = minio.CompletePart{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		}
	}

	_, err = core.CompleteMultipartUpload(ctx, s.config.MinioBucket, objectName, uploadID, completeParts, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	// Update metrics
	s.metrics.mu.Lock()
	s.metrics.MultipartUploads++
	s.metrics.mu.Unlock()

	return nil
}

// MultipartUploadURLs contains presigned URLs for multipart upload
type MultipartUploadURLs struct {
	UploadID      string    `json:"upload_id"`
	Bucket        string    `json:"bucket"`
	ObjectName    string    `json:"object_name"`
	OriginalName  string    `json:"original_name"`
	PartURLs      []PartURL `json:"part_urls"`
	ExpiryMinutes int       `json:"expiry_minutes"`
	CreatedAt     time.Time `json:"created_at"`
}

// PartURL contains a presigned URL for a specific part
type PartURL struct {
	PartNumber int    `json:"part_number"`
	URL        string `json:"url"`
}

// CompletedPart represents a completed upload part
type CompletedPart struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
}
