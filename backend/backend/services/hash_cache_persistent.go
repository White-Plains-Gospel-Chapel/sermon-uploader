package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/minio/minio-go/v7"
)

const (
	hashCacheFile = "sermon-hashes.json"
	hashCacheBucket = "system-cache" // Separate bucket for system files
)

// HashCacheData represents the persistent hash cache structure
type HashCacheData struct {
	Version     string            `json:"version"`
	LastUpdated time.Time         `json:"last_updated"`
	Hashes      map[string]string `json:"hashes"`      // hash -> filename
	FileToHash  map[string]string `json:"file_hashes"` // filename -> hash
}

// ensureSystemBucket creates the system bucket if it doesn't exist
func (c *HashCache) ensureSystemBucket(ctx context.Context) error {
	exists, err := c.minioClient.BucketExists(ctx, hashCacheBucket)
	if err != nil {
		return err
	}
	
	if !exists {
		err = c.minioClient.MakeBucket(ctx, hashCacheBucket, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create system bucket: %w", err)
		}
		c.logger.Info("Created system cache bucket", slog.String("bucket", hashCacheBucket))
	}
	
	return nil
}

// SaveHashCache saves the current hash cache to MinIO
func (c *HashCache) SaveToMinIO(ctx context.Context) error {
	// Ensure system bucket exists
	if err := c.ensureSystemBucket(ctx); err != nil {
		return err
	}
	c.mu.RLock()
	data := HashCacheData{
		Version:     "1.0",
		LastUpdated: time.Now(),
		Hashes:      make(map[string]string),
		FileToHash:  make(map[string]string),
	}
	
	// Copy current cache data
	for k, v := range c.hashes {
		data.Hashes[k] = v
	}
	for k, v := range c.fileToHash {
		data.FileToHash[k] = v
	}
	c.mu.RUnlock()
	
	// Serialize to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hash cache: %w", err)
	}
	
	// Upload to MinIO
	reader := bytes.NewReader(jsonData)
	_, err = c.minioClient.PutObject(ctx, hashCacheBucket, hashCacheFile, reader, int64(len(jsonData)), minio.PutObjectOptions{
		ContentType: "application/json",
		UserMetadata: map[string]string{
			"X-Hash-Cache": "true",
			"X-Last-Save":  time.Now().Format(time.RFC3339),
		},
	})
	
	if err != nil {
		return fmt.Errorf("failed to save hash cache to MinIO: %w", err)
	}
	
	c.logger.Info("Hash cache saved to MinIO", 
		slog.Int("hash_count", len(data.Hashes)),
		slog.String("file", hashCacheFile))
	
	return nil
}

// LoadFromMinIO loads the hash cache from MinIO
func (c *HashCache) LoadFromMinIO(ctx context.Context) error {
	// Try to get the cache file
	object, err := c.minioClient.GetObject(ctx, hashCacheBucket, hashCacheFile, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to get hash cache from MinIO: %w", err)
	}
	defer object.Close()
	
	// Check if object exists
	stat, err := object.Stat()
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			c.logger.Info("No hash cache found in MinIO, starting fresh")
			return nil
		}
		return fmt.Errorf("failed to stat hash cache: %w", err)
	}
	
	// Read the JSON data
	buffer := make([]byte, stat.Size)
	_, err = object.Read(buffer)
	if err != nil && err.Error() != "EOF" {
		return fmt.Errorf("failed to read hash cache: %w", err)
	}
	
	// Parse JSON
	var data HashCacheData
	if err := json.Unmarshal(buffer, &data); err != nil {
		return fmt.Errorf("failed to unmarshal hash cache: %w", err)
	}
	
	// Load into memory
	c.mu.Lock()
	c.hashes = data.Hashes
	c.fileToHash = data.FileToHash
	c.cacheReady = true
	c.loadTime = data.LastUpdated
	c.mu.Unlock()
	
	c.logger.Info("Hash cache loaded from MinIO",
		slog.Int("hash_count", len(data.Hashes)),
		slog.String("last_updated", data.LastUpdated.Format(time.RFC3339)))
	
	return nil
}

// AutoSave periodically saves the hash cache
func (c *HashCache) StartAutoSave(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := c.SaveToMinIO(ctx); err != nil {
					c.logger.Error("Failed to auto-save hash cache", slog.String("error", err.Error()))
				}
			case <-ctx.Done():
				// Final save before shutdown
				c.logger.Info("Saving hash cache before shutdown...")
				if err := c.SaveToMinIO(context.Background()); err != nil {
					c.logger.Error("Failed to save hash cache on shutdown", slog.String("error", err.Error()))
				}
				return
			}
		}
	}()
}

// LoadAllHashesOptimized loads hashes from cache first, then checks for new files
func (c *HashCache) LoadAllHashesOptimized() {
	startTime := time.Now()
	ctx := context.Background()
	
	c.logger.Info("Loading hash cache...")
	
	// Try to load from MinIO first
	if err := c.LoadFromMinIO(ctx); err != nil {
		c.logger.Warn("Could not load hash cache from MinIO", slog.String("error", err.Error()))
	}
	
	// If we have a cache, just verify new files
	if c.cacheReady && len(c.hashes) > 0 {
		c.logger.Info("Using cached hashes, checking for new files only")
		c.checkForNewFiles(ctx)
	} else {
		// No cache, load from scratch (but don't calculate hashes)
		c.logger.Info("No cache found, loading file list...")
		c.LoadAllHashes()
	}
	
	// Start auto-save
	c.StartAutoSave(ctx, 5*time.Minute) // Save every 5 minutes
	
	loadDuration := time.Since(startTime)
	c.logger.Info("Hash cache ready", 
		slog.Int("files", len(c.hashes)),
		slog.Duration("load_time", loadDuration))
}

// checkForNewFiles only checks for files not in cache
func (c *HashCache) checkForNewFiles(ctx context.Context) {
	objectCh := c.minioClient.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
		Recursive: true,
		WithMetadata: true, // Get metadata to check for hashes
	})
	
	newWithHash := 0
	newWithoutHash := 0
	
	for object := range objectCh {
		if object.Err != nil {
			continue
		}
		
		// Skip the cache file itself
		if object.Key == hashCacheFile {
			continue
		}
		
		// Check if we already have this file
		c.mu.RLock()
		_, exists := c.fileToHash[object.Key]
		c.mu.RUnlock()
		
		if !exists {
			// Check if it has a hash in metadata
			objInfo, err := c.minioClient.StatObject(ctx, c.bucket, object.Key, minio.StatObjectOptions{})
			if err == nil {
				if hash, hasHash := objInfo.UserMetadata["X-File-Hash"]; hasHash && hash != "" {
					// Add to cache without downloading
					c.mu.Lock()
					c.hashes[hash] = object.Key
					c.fileToHash[object.Key] = hash
					c.mu.Unlock()
					newWithHash++
				} else {
					// File without hash - will get hash on next upload
					newWithoutHash++
				}
			}
		}
	}
	
	if newWithHash > 0 || newWithoutHash > 0 {
		c.logger.Info("Found new files", 
			slog.Int("with_hash", newWithHash),
			slog.Int("without_hash", newWithoutHash))
	}
}