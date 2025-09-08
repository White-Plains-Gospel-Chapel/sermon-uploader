package services

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// UploadJob represents a file upload task
type UploadJob struct {
	ID          string
	Reader      io.Reader
	Size        int64
	Filename    string
	ContentType string
	Hash        string
	Result      chan UploadResult
}

// UploadResult contains the result of an upload
type UploadResult struct {
	Success bool
	Error   error
	Info    interface{}
	Duration time.Duration
}

// UploadWorkerPool manages concurrent uploads efficiently
type UploadWorkerPool struct {
	workers       int
	jobQueue      chan *UploadJob
	minioService  *MinIOService
	hashCache     *HashCache
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	
	// Metrics
	activeWorkers  int32
	completedJobs  int64
	failedJobs     int64
	totalBytes     int64
	avgUploadTime  int64 // in milliseconds
	
	logger *slog.Logger
}

// NewUploadWorkerPool creates an optimized worker pool for Pi
func NewUploadWorkerPool(minioService *MinIOService, hashCache *HashCache) *UploadWorkerPool {
	// Optimize worker count for Pi hardware
	numCPU := runtime.NumCPU()
	workers := numCPU * 2 // 2x CPU cores for I/O bound tasks
	
	// On Pi, limit to prevent memory pressure
	if runtime.GOARCH == "arm" || runtime.GOARCH == "arm64" {
		if workers > 8 {
			workers = 8 // Max 8 workers on Pi
		}
	}
	
	// Buffer size optimized for memory constraints
	queueSize := workers * 3
	
	ctx, cancel := context.WithCancel(context.Background())
	
	pool := &UploadWorkerPool{
		workers:      workers,
		jobQueue:     make(chan *UploadJob, queueSize),
		minioService: minioService,
		hashCache:    hashCache,
		ctx:          ctx,
		cancel:       cancel,
		logger:       slog.Default().With(slog.String("service", "upload-pool")),
	}
	
	pool.logger.Info("Creating upload worker pool",
		slog.Int("workers", workers),
		slog.Int("queue_size", queueSize),
		slog.String("arch", runtime.GOARCH))
	
	return pool
}

// Start initializes and starts all workers
func (p *UploadWorkerPool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	
	// Start metrics reporter
	go p.reportMetrics()
	
	p.logger.Info("Upload worker pool started", slog.Int("workers", p.workers))
}

// worker processes upload jobs
func (p *UploadWorkerPool) worker(id int) {
	defer p.wg.Done()
	
	p.logger.Debug("Worker started", slog.Int("worker_id", id))
	
	for {
		select {
		case job, ok := <-p.jobQueue:
			if !ok {
				p.logger.Debug("Worker stopping", slog.Int("worker_id", id))
				return
			}
			
			atomic.AddInt32(&p.activeWorkers, 1)
			startTime := time.Now()
			
			// Process the upload
			result := p.processUpload(job)
			
			// Update metrics
			duration := time.Since(startTime)
			atomic.AddInt32(&p.activeWorkers, -1)
			
			if result.Success {
				atomic.AddInt64(&p.completedJobs, 1)
				atomic.AddInt64(&p.totalBytes, job.Size)
				
				// Update average time (moving average)
				currentAvg := atomic.LoadInt64(&p.avgUploadTime)
				newAvg := (currentAvg*9 + duration.Milliseconds()) / 10
				atomic.StoreInt64(&p.avgUploadTime, newAvg)
			} else {
				atomic.AddInt64(&p.failedJobs, 1)
			}
			
			result.Duration = duration
			
			// Send result back
			select {
			case job.Result <- result:
			case <-time.After(5 * time.Second):
				p.logger.Warn("Result channel timeout", slog.String("job_id", job.ID))
			}
			
		case <-p.ctx.Done():
			p.logger.Debug("Worker context cancelled", slog.Int("worker_id", id))
			return
		}
	}
}

// processUpload handles the actual upload with optimizations
func (p *UploadWorkerPool) processUpload(job *UploadJob) UploadResult {
	// Check duplicate first (ultra-fast)
	if exists, existingFile := p.hashCache.CheckDuplicate(job.Hash); exists {
		return UploadResult{
			Success: false,
			Error:   fmt.Errorf("duplicate file: %s", existingFile),
		}
	}
	
	// Upload with hash metadata
	uploadInfo, err := p.minioService.PutFileWithHash(
		p.ctx,
		"sermons",
		job.Filename,
		job.Reader,
		job.Size,
		job.ContentType,
		job.Hash,
	)
	
	if err != nil {
		p.logger.Error("Upload failed",
			slog.String("filename", job.Filename),
			slog.String("error", err.Error()))
		return UploadResult{
			Success: false,
			Error:   err,
		}
	}
	
	// Register hash for future deduplication
	p.hashCache.AddHash(job.Hash, job.Filename)
	
	return UploadResult{
		Success: true,
		Info:    uploadInfo,
	}
}

// Submit adds a job to the queue (non-blocking with timeout)
func (p *UploadWorkerPool) Submit(job *UploadJob) error {
	select {
	case p.jobQueue <- job:
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("job queue full, timeout after 30s")
	case <-p.ctx.Done():
		return fmt.Errorf("worker pool shutting down")
	}
}

// SubmitAsync submits a job without waiting
func (p *UploadWorkerPool) SubmitAsync(job *UploadJob) {
	go func() {
		select {
		case p.jobQueue <- job:
		case <-p.ctx.Done():
		}
	}()
}

// GetMetrics returns current pool metrics
func (p *UploadWorkerPool) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"active_workers":  atomic.LoadInt32(&p.activeWorkers),
		"completed_jobs":  atomic.LoadInt64(&p.completedJobs),
		"failed_jobs":     atomic.LoadInt64(&p.failedJobs),
		"total_bytes":     atomic.LoadInt64(&p.totalBytes),
		"avg_upload_ms":   atomic.LoadInt64(&p.avgUploadTime),
		"queue_length":    len(p.jobQueue),
		"queue_capacity":  cap(p.jobQueue),
		"total_workers":   p.workers,
	}
}

// reportMetrics logs metrics periodically
func (p *UploadWorkerPool) reportMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			metrics := p.GetMetrics()
			if metrics["completed_jobs"].(int64) > 0 {
				p.logger.Info("Upload pool metrics",
					slog.Int("active", int(metrics["active_workers"].(int32))),
					slog.Int64("completed", metrics["completed_jobs"].(int64)),
					slog.Int64("failed", metrics["failed_jobs"].(int64)),
					slog.Int64("avg_ms", metrics["avg_upload_ms"].(int64)))
			}
		case <-p.ctx.Done():
			return
		}
	}
}

// Shutdown gracefully stops the worker pool
func (p *UploadWorkerPool) Shutdown(timeout time.Duration) error {
	p.logger.Info("Shutting down upload worker pool")
	
	// Stop accepting new jobs
	p.cancel()
	
	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		close(p.jobQueue)
		p.logger.Info("Upload worker pool shutdown complete")
		return nil
	case <-time.After(timeout):
		p.logger.Warn("Upload worker pool shutdown timeout")
		return fmt.Errorf("shutdown timeout after %v", timeout)
	}
}

// OptimizeForPi adjusts pool settings for Pi hardware
func (p *UploadWorkerPool) OptimizeForPi() {
	// Force aggressive GC on Pi to prevent memory pressure
	runtime.GC()
	
	// Limit memory for buffers
	runtime.MemProfileRate = 512 * 1024 // Profile every 512KB
	
	p.logger.Info("Optimized settings for Pi applied")
}