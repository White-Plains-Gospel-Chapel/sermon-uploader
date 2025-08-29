package services

import (
	"fmt"
	"mime/multipart"

	"sermon-uploader/config"
)

type FileService struct {
	minio    *MinIOService
	discord  *DiscordService
	wsHub    *WebSocketHub
	config   *config.Config
	metadata *MetadataService
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
	return &FileService{
		minio:    minio,
		discord:  discord,
		wsHub:    wsHub,
		config:   cfg,
		metadata: NewMetadataService("/app/temp"),
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

		// Read file data
		file, err := fileHeader.Open()
		if err != nil {
			result := FileUploadResult{
				Filename: fileHeader.Filename,
				Status:   "error",
				Message:  fmt.Sprintf("Failed to open file: %v", err),
			}
			results = append(results, result)
			failed++
			f.wsHub.BroadcastFileProgress(fileHeader.Filename, "error", result.Message, progress)
			continue
		}

		// Read file contents
		fileData := make([]byte, fileHeader.Size)
		_, err = file.Read(fileData)
		file.Close()
		if err != nil {
			result := FileUploadResult{
				Filename: fileHeader.Filename,
				Status:   "error",
				Message:  fmt.Sprintf("Failed to read file: %v", err),
			}
			results = append(results, result)
			failed++
			f.wsHub.BroadcastFileProgress(fileHeader.Filename, "error", result.Message, progress)
			continue
		}

		// Calculate file hash
		fileHash := f.minio.CalculateFileHash(fileData)

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

		// Upload file
		metadata, err := f.minio.UploadFile(fileData, fileHeader.Filename)
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