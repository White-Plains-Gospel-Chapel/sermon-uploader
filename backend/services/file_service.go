package services

import (
	"fmt"
	"io"
	"mime/multipart"
	"sync"

	"sermon-uploader/config"
	"sermon-uploader/monitoring"
	"sermon-uploader/optimization"
)

type FileService struct {
	minio              *MinIOService
	discord            *DiscordService
	wsHub              *WebSocketHub
	config             *config.Config
	metadata           *MetadataService
	streaming          *StreamingService
	tus                *TUSService
	workerPool         *WorkerPool
	pools              *optimization.ObjectPools
	profiler           *monitoring.PerformanceProfiler
	lastCalculatedHash string
	lastUploadSummary  *UploadSummary
	mu                 sync.RWMutex
}

type FileUploadResult struct {
	Filename string `json:"filename"`
	Renamed  string `json:"renamed,omitempty"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Hash     string `json:"hash,omitempty"`
}

func NewFileService(minio *MinIOService, discord *DiscordService, wsHub *WebSocketHub, cfg *config.Config) *FileService {
	streamingService := NewStreamingService()
	tusService := NewTUSService(cfg, streamingService)

	// Initialize optimizations
	workerPool := NewWorkerPool(cfg)
	pools := optimization.GetGlobalPools()
	profiler := monitoring.GetProfiler()

	return &FileService{
		minio:      minio,
		discord:    discord,
		wsHub:      wsHub,
		config:     cfg,
		metadata:   NewMetadataService("/app/temp"),
		streaming:  streamingService,
		tus:        tusService,
		workerPool: workerPool,
		pools:      pools,
		profiler:   profiler,
	}
}

// GetMetadataService returns the metadata service instance
func (f *FileService) GetMetadataService() *MetadataService {
	return f.metadata
}

func (f *FileService) ProcessFiles(files []*multipart.FileHeader) (*UploadSummary, error) {
	// Ensure bucket exists
	if err := f.minio.EnsureBucketExists(); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket exists: %w", err)
	}

	// Get existing file hashes for duplicate detection
	existingHashes, err := f.minio.GetExistingHashes()
	if err != nil {
		return nil, fmt.Errorf("failed to get existing hashes: %w", err)
	}

	// Send upload start notification
	isBatch := len(files) >= f.config.BatchThreshold
	if err := f.discord.SendUploadStart(len(files), isBatch); err != nil {
		// Log but don't fail
		fmt.Printf("Failed to send Discord notification: %v\n", err)
	}
	f.wsHub.BroadcastUploadStart(len(files), isBatch)

	var results []FileUploadResult
	successful := 0
	duplicates := 0
	failed := 0

	for i, fileHeader := range files {
		progress := float64(i+1) / float64(len(files)) * 100

		// Process file using streaming approach
		fileHash, err := f.processFileStreaming(fileHeader)
		if err != nil {
			result := FileUploadResult{
				Filename: fileHeader.Filename,
				Status:   "error",
				Message:  fmt.Sprintf("Failed to process file: %v", err),
			}
			results = append(results, result)
			failed++
			f.wsHub.BroadcastFileProgress(fileHeader.Filename, "error", result.Message, progress)
			continue
		}

		// Check for duplicates
		if existingHashes[fileHash] {
			result := FileUploadResult{
				Filename: fileHeader.Filename,
				Status:   "duplicate",
				Message:  "File already exists in bucket",
			}
			results = append(results, result)
			duplicates++
			f.wsHub.BroadcastFileProgress(fileHeader.Filename, "duplicate", result.Message, progress)
			continue
		}

		// Broadcast upload progress
		f.wsHub.BroadcastFileProgress(fileHeader.Filename, "uploading", "Uploading file...", progress)

		// Upload file using streaming approach
		metadata, err := f.uploadFileStreaming(fileHeader, fileHash)
		if err != nil {
			result := FileUploadResult{
				Filename: fileHeader.Filename,
				Status:   "error",
				Message:  fmt.Sprintf("Upload failed: %v", err),
			}
			results = append(results, result)
			failed++
			f.wsHub.BroadcastFileProgress(fileHeader.Filename, "error", result.Message, progress)
			continue
		}

		// Mark hash as existing to prevent duplicates in the same batch
		existingHashes[fileHash] = true

		result := FileUploadResult{
			Filename: fileHeader.Filename,
			Renamed:  metadata.RenamedFilename,
			Status:   "success",
			Size:     metadata.FileSize,
			Hash:     metadata.FileHash,
		}
		results = append(results, result)
		successful++
		f.wsHub.BroadcastFileProgress(fileHeader.Filename, "success", "Upload complete", progress)
	}

	// Send completion notifications
	summary := &UploadSummary{
		Successful: successful,
		Duplicates: duplicates,
		Failed:     failed,
		Total:      len(files),
		Results:    results,
	}

	if err := f.discord.SendUploadComplete(successful, duplicates, failed, isBatch); err != nil {
		// Log but don't fail
		fmt.Printf("Failed to send Discord completion notification: %v\n", err)
	}

	f.wsHub.BroadcastUploadComplete(successful, duplicates, failed, results)

	return summary, nil
}

type UploadSummary struct {
	Successful int                `json:"successful"`
	Duplicates int                `json:"duplicates"`
	Failed     int                `json:"failed"`
	Total      int                `json:"total"`
	Results    []FileUploadResult `json:"results"`
}

// processFileStreaming calculates file hash without loading entire file to memory
func (f *FileService) processFileStreaming(fileHeader *multipart.FileHeader) (string, error) {
	var calculatedHash string

	timerDone := f.profiler.StartTimer("hash_calculation")
	defer timerDone()

	err := func() error {
		file, err := fileHeader.Open()
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		// Use optimized streaming hasher with pooled buffers
		hasher := optimization.NewStreamingHasher()

		// Get buffer from pool based on file size
		bufferSize := f.config.IOBufferSize
		if bufferSize <= 0 {
			bufferSize = 32768 // 32KB default
		}

		buffer, releaseBuffer := f.pools.GetBuffer(bufferSize)
		defer releaseBuffer()

		// Stream file content with progress tracking
		totalSize := fileHeader.Size
		bytesRead := int64(0)

		for {
			n, err := file.Read(buffer)
			if n > 0 {
				hasher.Write(buffer[:n])
				bytesRead += int64(n)

				// Send progress for large files
				if totalSize > f.config.StreamingThreshold {
					progress := float64(bytesRead) / float64(totalSize) * 100
					f.wsHub.BroadcastFileProgress(fileHeader.Filename, "hashing",
						fmt.Sprintf("Calculating hash: %.1f%%", progress), progress*0.3) // Hash is 30% of total progress
				}
			}

			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}
		}

		calculatedHash = hasher.Sum()
		return nil
	}()

	if err != nil {
		return "", err
	}

	return calculatedHash, nil
}

// uploadFileStreaming uploads file using streaming without loading entire file to memory
func (f *FileService) uploadFileStreaming(fileHeader *multipart.FileHeader, fileHash string) (*FileMetadata, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Upload using streaming approach
	return f.minio.UploadFileStreaming(file, fileHeader.Filename, fileHeader.Size, fileHash)
}

// ProcessFileWithTUS processes a file upload using the TUS protocol
func (f *FileService) ProcessFileWithTUS(filename string, size int64, metadata map[string]string) (*TUSCreationResponse, error) {
	// Create TUS upload session
	return f.tus.CreateUpload(size, filename, metadata)
}

// ProcessTUSChunk processes a chunk in a TUS upload session
func (f *FileService) ProcessTUSChunk(uploadID string, offset int64, data []byte) (*TUSInfo, error) {
	// Process chunk using TUS service
	return f.tus.WriteChunk(uploadID, offset, data)
}

// CompleteTUSUpload completes a TUS upload and transfers to MinIO
func (f *FileService) CompleteTUSUpload(uploadID string, expectedHash string) (*FileUploadResult, error) {
	// Verify upload integrity
	quality, err := f.tus.VerifyUpload(uploadID, expectedHash)
	if err != nil {
		return nil, fmt.Errorf("failed to verify upload: %w", err)
	}

	if !quality.IntegrityPassed {
		return &FileUploadResult{
			Status:  "error",
			Message: "Upload integrity verification failed",
		}, nil
	}

	// Get upload info
	info, err := f.tus.GetUpload(uploadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get upload info: %w", err)
	}

	// Transfer to MinIO using streaming
	reader, err := f.tus.GetUploadReader(uploadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get upload reader: %w", err)
	}
	defer reader.Close()

	metadata, err := f.minio.UploadFileStreaming(reader, info.Filename, info.Size, expectedHash)
	if err != nil {
		return nil, fmt.Errorf("failed to upload to MinIO: %w", err)
	}

	// Cleanup TUS upload
	f.tus.DeleteUpload(uploadID)

	return &FileUploadResult{
		Filename: info.Filename,
		Renamed:  metadata.RenamedFilename,
		Status:   "success",
		Size:     metadata.FileSize,
		Hash:     metadata.FileHash,
	}, nil
}

// GetStreamingService returns the streaming service instance
func (f *FileService) GetStreamingService() *StreamingService {
	return f.streaming
}

// GetTUSService returns the TUS service instance
func (f *FileService) GetTUSService() *TUSService {
	return f.tus
}

// ProcessConcurrentFiles processes multiple files concurrently with Pi optimization
func (f *FileService) ProcessConcurrentFiles(files []*multipart.FileHeader) (*UploadSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Limit concurrent operations for Pi optimization
	maxConcurrent := 2
	if f.config.MaxConcurrentUploads > 0 {
		maxConcurrent = f.config.MaxConcurrentUploads
	}

	// Create semaphore for concurrent processing
	semaphore := make(chan struct{}, maxConcurrent)
	resultsChan := make(chan FileUploadResult, len(files))
	var wg sync.WaitGroup

	// Ensure bucket exists
	if err := f.minio.EnsureBucketExists(); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket exists: %w", err)
	}

	// Get existing file hashes for duplicate detection
	existingHashes, err := f.minio.GetExistingHashes()
	if err != nil {
		return nil, fmt.Errorf("failed to get existing hashes: %w", err)
	}

	// Send upload start notification
	isBatch := len(files) >= f.config.BatchThreshold
	if err := f.discord.SendUploadStart(len(files), isBatch); err != nil {
		fmt.Printf("Failed to send Discord notification: %v\n", err)
	}
	f.wsHub.BroadcastUploadStart(len(files), isBatch)

	// Process files concurrently
	for i, fileHeader := range files {
		wg.Add(1)
		go func(idx int, fh *multipart.FileHeader) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			progress := float64(idx+1) / float64(len(files)) * 100

			// Process file with streaming
			result := f.processSingleFileStreaming(fh, existingHashes, progress)
			resultsChan <- result
		}(i, fileHeader)
	}

	// Wait for all uploads to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var results []FileUploadResult
	successful := 0
	duplicates := 0
	failed := 0

	for result := range resultsChan {
		results = append(results, result)
		switch result.Status {
		case "success":
			successful++
		case "duplicate":
			duplicates++
		default:
			failed++
		}
	}

	// Send completion notifications
	summary := &UploadSummary{
		Successful: successful,
		Duplicates: duplicates,
		Failed:     failed,
		Total:      len(files),
		Results:    results,
	}

	if err := f.discord.SendUploadComplete(successful, duplicates, failed, isBatch); err != nil {
		fmt.Printf("Failed to send Discord completion notification: %v\n", err)
	}

	f.wsHub.BroadcastUploadComplete(successful, duplicates, failed, results)

	return summary, nil
}

// processSingleFileStreaming processes a single file with streaming approach
func (f *FileService) processSingleFileStreaming(fileHeader *multipart.FileHeader, existingHashes map[string]bool, progress float64) FileUploadResult {
	// Calculate file hash using streaming
	fileHash, err := f.processFileStreaming(fileHeader)
	if err != nil {
		result := FileUploadResult{
			Filename: fileHeader.Filename,
			Status:   "error",
			Message:  fmt.Sprintf("Failed to calculate hash: %v", err),
		}
		f.wsHub.BroadcastFileProgress(fileHeader.Filename, "error", result.Message, progress)
		return result
	}

	// Check for duplicates
	if existingHashes[fileHash] {
		result := FileUploadResult{
			Filename: fileHeader.Filename,
			Status:   "duplicate",
			Message:  "File already exists in bucket",
		}
		f.wsHub.BroadcastFileProgress(fileHeader.Filename, "duplicate", result.Message, progress)
		return result
	}

	// Broadcast upload progress
	f.wsHub.BroadcastFileProgress(fileHeader.Filename, "uploading", "Uploading file...", progress)

	// Upload file using streaming approach
	metadata, err := f.uploadFileStreaming(fileHeader, fileHash)
	if err != nil {
		result := FileUploadResult{
			Filename: fileHeader.Filename,
			Status:   "error",
			Message:  fmt.Sprintf("Upload failed: %v", err),
		}
		f.wsHub.BroadcastFileProgress(fileHeader.Filename, "error", result.Message, progress)
		return result
	}

	// Mark hash as existing to prevent duplicates in the same batch
	existingHashes[fileHash] = true

	result := FileUploadResult{
		Filename: fileHeader.Filename,
		Renamed:  metadata.RenamedFilename,
		Status:   "success",
		Size:     metadata.FileSize,
		Hash:     metadata.FileHash,
	}
	f.wsHub.BroadcastFileProgress(fileHeader.Filename, "success", "Upload complete", progress)
	return result
}
