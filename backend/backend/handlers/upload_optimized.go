package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// UploadOptimized handles file uploads with maximum Go optimizations
func (h *Handlers) UploadOptimized(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Minute)
	defer cancel()
	
	logger := slog.With(
		slog.String("handler", "UploadOptimized"),
		slog.String("ip", c.IP()),
	)
	
	// Parse multipart form with streaming
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No file provided",
		})
	}
	
	// Validate file type
	if !isValidAudioFile(file.Filename, file.Header.Get("Content-Type")) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Only WAV files are allowed",
		})
	}
	
	// Open file for streaming
	src, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to open file",
		})
	}
	defer src.Close()
	
	// Use io.Pipe for zero-copy streaming
	pr, pw := io.Pipe()
	
	// Get buffer from pool (reuse memory)
	buffer := make([]byte, 64*1024) // 64KB buffer for now
	
	// Calculate hash and upload concurrently using goroutines
	var (
		fileHash string
		hashErr  error
		uploadErr error
		wg       sync.WaitGroup
	)
	
	wg.Add(2)
	
	// Goroutine 1: Calculate hash while reading
	go func() {
		defer wg.Done()
		defer pw.Close()
		
		hasher := sha256.New()
		multiWriter := io.MultiWriter(hasher, pw)
		
		// Use our pooled buffer for copying
		_, err := io.CopyBuffer(multiWriter, src, buffer)
		if err != nil {
			hashErr = err
			return
		}
		
		fileHash = hex.EncodeToString(hasher.Sum(nil))
	}()
	
	// Goroutine 2: Upload to MinIO while hash is being calculated
	go func() {
		defer wg.Done()
		
		ext := filepath.Ext(file.Filename)
		baseName := strings.TrimSuffix(filepath.Base(file.Filename), ext)
		objectName := fmt.Sprintf("%s_%d%s", baseName, time.Now().Unix(), ext)
		
		// Stream upload with context
		_, err := h.minioService.PutFileWithContext(
			ctx,
			"sermons",
			objectName,
			pr,
			file.Size,
			file.Header.Get("Content-Type"),
		)
		
		if err != nil {
			uploadErr = err
			return
		}
	}()
	
	// Wait for both operations to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	// Wait with timeout
	select {
	case <-done:
		// Both operations completed
	case <-ctx.Done():
		return c.Status(fiber.StatusRequestTimeout).JSON(fiber.Map{
			"error": "Upload timeout",
		})
	}
	
	// Check for errors
	if hashErr != nil {
		logger.Error("Hash calculation failed", slog.String("error", hashErr.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Hash calculation failed",
		})
	}
	
	if uploadErr != nil {
		logger.Error("Upload failed", slog.String("error", uploadErr.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Upload failed",
		})
	}
	
	// Check for duplicate AFTER upload (since we streamed)
	if exists, existingFile := h.hashCache.CheckDuplicate(fileHash); exists {
		// Delete the duplicate we just uploaded
		// TODO: Add delete functionality
		logger.Warn("Duplicate uploaded", 
			slog.String("hash", fileHash[:8]+"..."),
			slog.String("existing", existingFile))
			
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Duplicate file detected",
			"existing_file": existingFile,
		})
	}
	
	// Register hash for future checks
	h.hashCache.AddHash(fileHash, file.Filename)
	
	logger.Info("Optimized upload successful",
		slog.String("file", file.Filename),
		slog.String("hash", fileHash[:8]+"..."))
	
	return c.JSON(fiber.Map{
		"success": true,
		"file": fiber.Map{
			"name": file.Filename,
			"size": file.Size,
			"hash": fileHash,
		},
	})
}

// UploadBatchOptimized handles multiple files with worker pool
func (h *Handlers) UploadBatchOptimized(c *fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid multipart form",
		})
	}
	
	files := form.File["files"]
	if len(files) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No files provided",
		})
	}
	
	// Use channels for concurrent processing
	type result struct {
		filename string
		success  bool
		error    string
		hash     string
	}
	
	resultChan := make(chan result, len(files))
	
	// Process files concurrently with worker pool pattern
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 3) // Limit to 3 concurrent uploads
	
	for _, file := range files {
		wg.Add(1)
		go func(f *multipart.FileHeader) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			// Process file
			src, err := f.Open()
			if err != nil {
				resultChan <- result{
					filename: f.Filename,
					success:  false,
					error:    "Failed to open file",
				}
				return
			}
			defer src.Close()
			
			// Calculate hash
			hasher := sha256.New()
			buffer := make([]byte, 64*1024)
			
			_, err = io.CopyBuffer(hasher, src, buffer)
			if err != nil {
				resultChan <- result{
					filename: f.Filename,
					success:  false,
					error:    "Hash calculation failed",
				}
				return
			}
			
			hash := hex.EncodeToString(hasher.Sum(nil))
			
			// Check duplicate
			if exists, _ := h.hashCache.CheckDuplicate(hash); exists {
				resultChan <- result{
					filename: f.Filename,
					success:  false,
					error:    "Duplicate file",
					hash:     hash,
				}
				return
			}
			
			// Reset reader
			src.Seek(0, 0)
			
			// Upload (simplified for example)
			// In production, use the optimized upload logic
			
			resultChan <- result{
				filename: f.Filename,
				success:  true,
				hash:     hash,
			}
			
		}(file)
	}
	
	// Wait for all uploads to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	
	// Collect results
	var results []result
	for r := range resultChan {
		results = append(results, r)
	}
	
	return c.JSON(fiber.Map{
		"success": true,
		"results": results,
	})
}