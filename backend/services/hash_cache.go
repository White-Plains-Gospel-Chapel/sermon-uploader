package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
)

// HashCache provides ultra-fast in-memory duplicate detection
type HashCache struct {
	minioClient *minio.Client
	bucket      string
	
	// Primary cache: hash -> filename for O(1) lookups
	hashes map[string]string
	
	// Reverse lookup: filename -> hash (for deletions)
	fileToHash map[string]string
	
	mu         sync.RWMutex
	logger     *slog.Logger
	loadTime   time.Time
	cacheReady bool
}

// NewHashCache creates a new hash cache service
func NewHashCache(minioClient *minio.Client, bucket string) *HashCache {
	cache := &HashCache{
		minioClient: minioClient,
		bucket:      bucket,
		hashes:      make(map[string]string),
		fileToHash:  make(map[string]string),
		logger:      slog.Default().With(slog.String("service", "hash-cache")),
	}
	
	// Load all hashes with optimization (from cache if available)
	cache.LoadAllHashesOptimized()
	
	return cache
}

// LoadAllHashes loads all file hashes from MinIO metadata ONLY
// NEVER downloads files - only reads metadata
func (c *HashCache) LoadAllHashes() {
	startTime := time.Now()
	ctx := context.Background()
	
	c.logger.Info("Loading file hashes from metadata (no downloads)...")
	
	// Create new maps to avoid locking during load
	newHashes := make(map[string]string)
	newFileToHash := make(map[string]string)
	
	// List all objects with metadata
	objectCh := c.minioClient.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
		Recursive: true,
		WithMetadata: true, // Include metadata in listing
	})
	
	count := 0
	skipped := 0
	for object := range objectCh {
		if object.Err != nil {
			c.logger.Error("Error listing object", slog.String("error", object.Err.Error()))
			continue
		}
		
		// Get object info with metadata (this is just metadata, not file content!)
		objInfo, err := c.minioClient.StatObject(ctx, c.bucket, object.Key, minio.StatObjectOptions{})
		if err != nil {
			c.logger.Warn("Could not stat object", 
				slog.String("object", object.Key),
				slog.String("error", err.Error()))
			continue
		}
		
		// Check for hash in metadata - NEVER download the file
		if hash, exists := objInfo.UserMetadata["X-File-Hash"]; exists && hash != "" {
			newHashes[hash] = object.Key
			newFileToHash[object.Key] = hash
			count++
		} else {
			// File without hash - will get hash when re-uploaded
			// NEVER download to calculate hash!
			skipped++
			c.logger.Debug("File without hash metadata (will add on next upload)", 
				slog.String("file", object.Key))
		}
	}
	
	// Atomic swap of the cache
	c.mu.Lock()
	c.hashes = newHashes
	c.fileToHash = newFileToHash
	c.loadTime = startTime
	c.cacheReady = true
	c.mu.Unlock()
	
	loadDuration := time.Since(startTime)
	c.logger.Info("Hash cache loaded (metadata only, no downloads!)", 
		slog.Int("files_with_hash", count),
		slog.Int("files_without_hash", skipped),
		slog.Duration("load_time", loadDuration),
		slog.Int("cache_size_bytes", count*64)) // Approximate memory usage
}

// CheckDuplicate performs ultra-fast O(1) duplicate check
func (c *HashCache) CheckDuplicate(hash string) (exists bool, filename string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if !c.cacheReady {
		c.logger.Warn("Cache not ready yet")
		return false, ""
	}
	
	filename, exists = c.hashes[hash]
	if exists {
		c.logger.Info("Duplicate detected", 
			slog.String("hash", hash[:8]+"..."), // Log first 8 chars
			slog.String("existing_file", filename))
	}
	return exists, filename
}

// AddHash adds a new hash to the cache (called after successful upload)
func (c *HashCache) AddHash(hash string, filename string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.hashes[hash] = filename
	c.fileToHash[filename] = hash
	
	c.logger.Info("Added hash to cache", 
		slog.String("hash", hash[:8]+"..."),
		slog.String("filename", filename))
}

// RemoveFile removes a file's hash from cache (for deletions)
func (c *HashCache) RemoveFile(filename string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if hash, exists := c.fileToHash[filename]; exists {
		delete(c.hashes, hash)
		delete(c.fileToHash, filename)
		
		c.logger.Info("Removed hash from cache", 
			slog.String("filename", filename))
	}
}

// CalculateHash calculates SHA256 hash from a reader
func (c *HashCache) CalculateHash(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// NOTE: We never download files to calculate hashes!
// Hashes are calculated during upload and stored as metadata
// This ensures fast startup and no unnecessary downloads

// GetStats returns cache statistics
func (c *HashCache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return map[string]interface{}{
		"total_files":    len(c.hashes),
		"cache_ready":    c.cacheReady,
		"load_time":      c.loadTime.Format(time.RFC3339),
		"memory_usage":   fmt.Sprintf("~%d KB", len(c.hashes)*64/1024),
		"uptime":         time.Since(c.loadTime).String(),
	}
}

// IsReady returns whether the cache is ready for use
func (c *HashCache) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cacheReady
}

// ClearCache removes all entries from the hash cache
func (c *HashCache) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.hashes = make(map[string]string)
	c.fileToHash = make(map[string]string)
	
	c.logger.Info("Hash cache cleared")
}