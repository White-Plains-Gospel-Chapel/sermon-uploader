package services

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ConcurrentProcessor handles parallel processing with optimal Go patterns
type ConcurrentProcessor struct {
	maxWorkers   int
	jobQueue     chan Job
	resultQueue  chan Result
	errorQueue   chan error
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	
	// Metrics
	processed    atomic.Int64
	failed       atomic.Int64
	inProgress   atomic.Int32
	totalTime    atomic.Int64 // in milliseconds
}

// Job represents a unit of work
type Job interface {
	Process(ctx context.Context) (Result, error)
	ID() string
}

// Result represents job output
type Result interface {
	JobID() string
	Success() bool
}

// NewConcurrentProcessor creates an optimized processor for Pi
func NewConcurrentProcessor() *ConcurrentProcessor {
	// Optimize worker count based on CPU
	workers := runtime.NumCPU() * 2
	if runtime.GOARCH == "arm" || runtime.GOARCH == "arm64" {
		workers = minInt(workers, 8) // Cap at 8 for Pi
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ConcurrentProcessor{
		maxWorkers:  workers,
		jobQueue:    make(chan Job, workers*3),
		resultQueue: make(chan Result, workers*3),
		errorQueue:  make(chan error, workers),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start initializes the processor
func (p *ConcurrentProcessor) Start() {
	// Start workers
	for i := 0; i < p.maxWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	
	// Start metrics collector
	go p.collectMetrics()
}

// worker processes jobs
func (p *ConcurrentProcessor) worker(id int) {
	defer p.wg.Done()
	
	// Each worker gets its own buffer from pool
	buffer := make([]byte, 64*1024) // 64KB working buffer
	_ = buffer // Use for processing
	
	for {
		select {
		case job, ok := <-p.jobQueue:
			if !ok {
				return // Channel closed
			}
			
			p.inProgress.Add(1)
			start := time.Now()
			
			// Process with timeout
			ctx, cancel := context.WithTimeout(p.ctx, 2*time.Minute)
			result, err := job.Process(ctx)
			cancel()
			
			elapsed := time.Since(start)
			p.totalTime.Add(elapsed.Milliseconds())
			p.inProgress.Add(-1)
			
			if err != nil {
				p.failed.Add(1)
				select {
				case p.errorQueue <- fmt.Errorf("job %s: %w", job.ID(), err):
				case <-p.ctx.Done():
					return
				}
			} else {
				p.processed.Add(1)
				select {
				case p.resultQueue <- result:
				case <-p.ctx.Done():
					return
				}
			}
			
		case <-p.ctx.Done():
			return
		}
	}
}

// Submit adds a job for processing
func (p *ConcurrentProcessor) Submit(job Job) error {
	select {
	case p.jobQueue <- job:
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("queue full, timeout")
	case <-p.ctx.Done():
		return fmt.Errorf("processor shutting down")
	}
}

// SubmitBatch processes multiple jobs efficiently
func (p *ConcurrentProcessor) SubmitBatch(jobs []Job) error {
	// Use goroutine pool pattern
	semaphore := make(chan struct{}, p.maxWorkers)
	var submitWg sync.WaitGroup
	
	for _, job := range jobs {
		submitWg.Add(1)
		go func(j Job) {
			defer submitWg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			if err := p.Submit(j); err != nil {
				select {
				case p.errorQueue <- err:
				default:
				}
			}
		}(job)
	}
	
	submitWg.Wait()
	return nil
}

// GetResults returns the result channel
func (p *ConcurrentProcessor) GetResults() <-chan Result {
	return p.resultQueue
}

// GetErrors returns the error channel
func (p *ConcurrentProcessor) GetErrors() <-chan error {
	return p.errorQueue
}

// collectMetrics periodically logs performance metrics
func (p *ConcurrentProcessor) collectMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			processed := p.processed.Load()
			failed := p.failed.Load()
			inProgress := p.inProgress.Load()
			avgTime := float64(p.totalTime.Load()) / float64(processed+failed)
			
			if processed+failed > 0 {
				fmt.Printf("Processor Stats - Processed: %d, Failed: %d, In Progress: %d, Avg Time: %.2fms\n",
					processed, failed, inProgress, avgTime)
			}
			
		case <-p.ctx.Done():
			return
		}
	}
}

// Shutdown gracefully stops the processor
func (p *ConcurrentProcessor) Shutdown(timeout time.Duration) error {
	p.cancel()
	
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		close(p.jobQueue)
		close(p.resultQueue)
		close(p.errorQueue)
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("shutdown timeout")
	}
}

// GetStats returns current metrics
func (p *ConcurrentProcessor) GetStats() map[string]interface{} {
	processed := p.processed.Load()
	failed := p.failed.Load()
	total := processed + failed
	
	avgTime := float64(0)
	if total > 0 {
		avgTime = float64(p.totalTime.Load()) / float64(total)
	}
	
	return map[string]interface{}{
		"processed":     processed,
		"failed":        failed,
		"in_progress":   p.inProgress.Load(),
		"avg_time_ms":   avgTime,
		"queue_length":  len(p.jobQueue),
		"max_workers":   p.maxWorkers,
	}
}

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}