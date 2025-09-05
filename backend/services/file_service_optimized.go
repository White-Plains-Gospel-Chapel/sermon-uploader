package services

import (
	"context"
	"fmt"
	"mime/multipart"
	"time"

	"sermon-uploader/optimization"
)

// ProcessFilesOptimized processes files using all performance optimizations
func (f *FileService) ProcessFilesOptimized(files []*multipart.FileHeader) (*UploadSummary, error) {
	var summary *UploadSummary

	err := f.profiler.ProfileOperation("optimized_file_processing", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		// Ensure bucket exists
		if err := f.minio.EnsureBucketExists(); err != nil {
			return fmt.Errorf("failed to ensure bucket exists: %w", err)
		}

		// Get existing file hashes for duplicate detection
		existingHashes, err := f.minio.GetExistingHashes()
		if err != nil {
			return fmt.Errorf("failed to get existing hashes: %w", err)
		}

		// Send upload start notification
		isBatch := len(files) >= f.config.BatchThreshold
		if err := f.discord.SendUploadStart(len(files), isBatch); err != nil {
			fmt.Printf("Failed to send Discord notification: %v\n", err)
		}
		f.wsHub.BroadcastUploadStart(len(files), isBatch)

		// Use worker pool for concurrent processing
		results := make(chan FileUploadResult, len(files))

		for i, fileHeader := range files {
			workItem := WorkItem{
				ID:          fmt.Sprintf("upload_%d_%s", i, fileHeader.Filename),
				FileHeader:  fileHeader,
				Context:     ctx,
				Priority:    1,
				SubmittedAt: time.Now(),
				Callback: func(result *WorkResult) {
					uploadResult := FileUploadResult{
						Filename: result.ID,
						Status:   "success",
						Size:     result.BytesProcessed,
						Hash:     result.FileHash,
					}

					if !result.Success {
						uploadResult.Status = "error"
						uploadResult.Message = result.Error.Error()
					} else {
						// Check for duplicates
						if existingHashes[result.FileHash] {
							uploadResult.Status = "duplicate"
							uploadResult.Message = "File already exists in bucket"
						} else {
							uploadResult.Renamed = result.Metadata.RenamedFilename
						}
					}

					results <- uploadResult
				},
			}

			if err := f.workerPool.Submit(workItem); err != nil {
				results <- FileUploadResult{
					Filename: fileHeader.Filename,
					Status:   "error",
					Message:  fmt.Sprintf("Failed to submit work: %v", err),
				}
			}
		}

		// Collect results
		var uploadResults []FileUploadResult
		successful := 0
		duplicates := 0
		failed := 0

		for i := 0; i < len(files); i++ {
			select {
			case result := <-results:
				uploadResults = append(uploadResults, result)
				switch result.Status {
				case "success":
					successful++
				case "duplicate":
					duplicates++
				default:
					failed++
				}
			case <-ctx.Done():
				return fmt.Errorf("processing timed out")
			}
		}

		// Send completion notifications
		summary = &UploadSummary{
			Successful: successful,
			Duplicates: duplicates,
			Failed:     failed,
			Total:      len(files),
			Results:    uploadResults,
		}

		if err := f.discord.SendUploadComplete(successful, duplicates, failed, isBatch); err != nil {
			fmt.Printf("Failed to send Discord completion notification: %v\n", err)
		}

		f.wsHub.BroadcastUploadComplete(successful, duplicates, failed, uploadResults)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return summary, nil
}

// GetWorkerPoolStats returns worker pool statistics
func (f *FileService) GetWorkerPoolStats() WorkerPoolStats {
	return f.workerPool.GetStats()
}

// GetOptimizationStats returns optimization statistics
func (f *FileService) GetOptimizationStats() OptimizationStats {
	poolStats := f.pools.GetAllStats()
	workerStats := f.workerPool.GetStats()

	return OptimizationStats{
		PoolStats:   poolStats,
		WorkerStats: workerStats,
		Timestamp:   time.Now(),
	}
}

// OptimizationStats provides comprehensive optimization statistics
type OptimizationStats struct {
	PoolStats   optimization.ObjectPoolsStats `json:"pool_stats"`
	WorkerStats WorkerPoolStats               `json:"worker_stats"`
	Timestamp   time.Time                     `json:"timestamp"`
}

// Cleanup gracefully shuts down optimization resources
func (f *FileService) Cleanup() error {
	if f.workerPool != nil {
		return f.workerPool.Shutdown(30 * time.Second)
	}
	return nil
}
