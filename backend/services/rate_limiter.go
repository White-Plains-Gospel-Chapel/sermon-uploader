package services

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
	
	"golang.org/x/time/rate"
)

// RateLimiter provides rate limiting for Pi resource protection
type RateLimiter struct {
	uploadLimiter  *rate.Limiter
	hashLimiter    *rate.Limiter
	apiLimiter     *rate.Limiter
	
	// Per-IP limiters for fairness
	ipLimiters map[string]*rate.Limiter
	mu         sync.RWMutex
	
	// Metrics
	allowed  int64
	denied   int64
}

// NewRateLimiter creates rate limiters optimized for Pi
func NewRateLimiter() *RateLimiter {
	// Adjust rates based on Pi capabilities
	uploadRate := rate.Every(500 * time.Millisecond) // 2 uploads per second max
	hashRate := rate.Every(100 * time.Millisecond)   // 10 hashes per second max
	apiRate := rate.Every(50 * time.Millisecond)     // 20 API calls per second
	
	return &RateLimiter{
		uploadLimiter: rate.NewLimiter(uploadRate, 2),  // Burst of 2
		hashLimiter:   rate.NewLimiter(hashRate, 5),    // Burst of 5
		apiLimiter:    rate.NewLimiter(apiRate, 10),    // Burst of 10
		ipLimiters:    make(map[string]*rate.Limiter),
	}
}

// AllowUpload checks if an upload is allowed
func (r *RateLimiter) AllowUpload(ctx context.Context) error {
	if !r.uploadLimiter.Allow() {
		r.denied++
		return fmt.Errorf("upload rate limit exceeded, please wait")
	}
	r.allowed++
	return nil
}

// WaitForUpload waits until upload is allowed
func (r *RateLimiter) WaitForUpload(ctx context.Context) error {
	return r.uploadLimiter.Wait(ctx)
}

// AllowHash checks if hash calculation is allowed
func (r *RateLimiter) AllowHash() bool {
	allowed := r.hashLimiter.Allow()
	if allowed {
		r.allowed++
	} else {
		r.denied++
	}
	return allowed
}

// AllowAPI checks if API call is allowed
func (r *RateLimiter) AllowAPI() bool {
	allowed := r.apiLimiter.Allow()
	if allowed {
		r.allowed++
	} else {
		r.denied++
	}
	return allowed
}

// GetIPLimiter gets or creates a per-IP rate limiter
func (r *RateLimiter) GetIPLimiter(ip string) *rate.Limiter {
	r.mu.RLock()
	limiter, exists := r.ipLimiters[ip]
	r.mu.RUnlock()
	
	if exists {
		return limiter
	}
	
	// Create new limiter for this IP
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Check again in case of race
	if limiter, exists = r.ipLimiters[ip]; exists {
		return limiter
	}
	
	// Per-IP: 1 upload every 2 seconds, burst of 1
	limiter = rate.NewLimiter(rate.Every(2*time.Second), 1)
	r.ipLimiters[ip] = limiter
	
	// Clean up old limiters periodically
	if len(r.ipLimiters) > 100 {
		r.cleanupOldLimiters()
	}
	
	return limiter
}

// AllowIP checks if request from IP is allowed
func (r *RateLimiter) AllowIP(ip string) bool {
	limiter := r.GetIPLimiter(ip)
	return limiter.Allow()
}

// cleanupOldLimiters removes inactive IP limiters
func (r *RateLimiter) cleanupOldLimiters() {
	// Keep only the 50 most recent IPs
	if len(r.ipLimiters) > 50 {
		// Simple cleanup: remove half
		count := 0
		for ip := range r.ipLimiters {
			delete(r.ipLimiters, ip)
			count++
			if count >= 25 {
				break
			}
		}
	}
}

// GetStats returns rate limiter statistics
func (r *RateLimiter) GetStats() map[string]interface{} {
	r.mu.RLock()
	ipCount := len(r.ipLimiters)
	r.mu.RUnlock()
	
	return map[string]interface{}{
		"allowed":          r.allowed,
		"denied":           r.denied,
		"denial_rate":      float64(r.denied) / float64(r.allowed+r.denied),
		"tracked_ips":      ipCount,
		"upload_rate":      r.uploadLimiter.Limit(),
		"upload_burst":     r.uploadLimiter.Burst(),
		"hash_rate":        r.hashLimiter.Limit(),
		"api_rate":         r.apiLimiter.Limit(),
	}
}

// AdaptiveRateLimiter adjusts rates based on system load
type AdaptiveRateLimiter struct {
	*RateLimiter
	
	// Adjustment parameters
	lastAdjustment time.Time
	adjustInterval time.Duration
}

// NewAdaptiveRateLimiter creates a rate limiter that adapts to system load
func NewAdaptiveRateLimiter() *AdaptiveRateLimiter {
	return &AdaptiveRateLimiter{
		RateLimiter:    NewRateLimiter(),
		adjustInterval: 30 * time.Second,
	}
}

// AdjustRates adjusts rate limits based on system metrics
func (a *AdaptiveRateLimiter) AdjustRates() {
	if time.Since(a.lastAdjustment) < a.adjustInterval {
		return
	}
	
	a.lastAdjustment = time.Now()
	
	// Check memory pressure
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	if m.Alloc > 500*1024*1024 { // If using more than 500MB
		// Reduce rates under memory pressure
		a.uploadLimiter.SetLimit(rate.Every(1 * time.Second))
		a.hashLimiter.SetLimit(rate.Every(200 * time.Millisecond))
		return
	}
	
	// Check denial rate
	denialRate := float64(a.denied) / float64(a.allowed+a.denied)
	if denialRate > 0.2 { // More than 20% denied
		// Too restrictive, increase limits slightly
		currentLimit := a.uploadLimiter.Limit()
		a.uploadLimiter.SetLimit(currentLimit * 1.1)
	} else if denialRate < 0.05 { // Less than 5% denied
		// Could be more restrictive to protect Pi
		currentLimit := a.uploadLimiter.Limit()
		a.uploadLimiter.SetLimit(currentLimit * 0.9)
	}
}