package logging

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// EasternTimeHandler ensures all timestamps are in Eastern Time
type EasternTimeHandler struct {
	slog.Handler
	location *time.Location
}

func NewEasternTimeHandler(h slog.Handler, loc *time.Location) *EasternTimeHandler {
	return &EasternTimeHandler{
		Handler:  h,
		location: loc,
	}
}

func (h *EasternTimeHandler) Handle(ctx context.Context, r slog.Record) error {
	// Convert timestamp to Eastern Time
	r.Time = r.Time.In(h.location)
	return h.Handler.Handle(ctx, r)
}

func (h *EasternTimeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &EasternTimeHandler{
		Handler:  h.Handler.WithAttrs(attrs),
		location: h.location,
	}
}

func (h *EasternTimeHandler) WithGroup(name string) slog.Handler {
	return &EasternTimeHandler{
		Handler:  h.Handler.WithGroup(name),
		location: h.location,
	}
}

// ContextualHandler adds correlation IDs and trace info from context
type ContextualHandler struct {
	slog.Handler
}

func NewContextualHandler(h slog.Handler) *ContextualHandler {
	return &ContextualHandler{Handler: h}
}

func (h *ContextualHandler) Handle(ctx context.Context, r slog.Record) error {
	// Extract correlation ID
	if corrID := ctx.Value(ContextKeyCorrelationID); corrID != nil {
		if id, ok := corrID.(string); ok && id != "" {
			r.Add("correlation_id", slog.StringValue(id))
		}
	}

	// Extract request ID
	if reqID := ctx.Value(ContextKeyRequestID); reqID != nil {
		if id, ok := reqID.(string); ok && id != "" {
			r.Add("request_id", slog.StringValue(id))
		}
	}

	// Extract user ID
	if userID := ctx.Value(ContextKeyUserID); userID != nil {
		if id, ok := userID.(string); ok && id != "" {
			r.Add("user_id", slog.StringValue(id))
		}
	}

	return h.Handler.Handle(ctx, r)
}

func (h *ContextualHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextualHandler{
		Handler: h.Handler.WithAttrs(attrs),
	}
}

func (h *ContextualHandler) WithGroup(name string) slog.Handler {
	return &ContextualHandler{
		Handler: h.Handler.WithGroup(name),
	}
}

// PerformanceHandler adds performance warnings for slow operations
type PerformanceHandler struct {
	slog.Handler
	threshold time.Duration
}

func NewPerformanceHandler(h slog.Handler, threshold time.Duration) *PerformanceHandler {
	return &PerformanceHandler{
		Handler:   h,
		threshold: threshold,
	}
}

func (h *PerformanceHandler) Handle(ctx context.Context, r slog.Record) error {
	// Check for operation duration in context
	if duration := ctx.Value(ContextKeyOperationDuration); duration != nil {
		if d, ok := duration.(time.Duration); ok {
			if d > h.threshold {
				r.Add("performance_warning", slog.BoolValue(true))
				r.Add("threshold_exceeded_ms", slog.Int64Value(d.Milliseconds()))
			}
			r.Add("operation_duration_ms", slog.Int64Value(d.Milliseconds()))
		}
	}

	return h.Handler.Handle(ctx, r)
}

func (h *PerformanceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &PerformanceHandler{
		Handler:   h.Handler.WithAttrs(attrs),
		threshold: h.threshold,
	}
}

func (h *PerformanceHandler) WithGroup(name string) slog.Handler {
	return &PerformanceHandler{
		Handler:   h.Handler.WithGroup(name),
		threshold: h.threshold,
	}
}

// SamplingHandler samples logs based on rate
type SamplingHandler struct {
	handler slog.Handler
	rate    float64
	counter uint64
	mu      sync.RWMutex
	rand    *rand.Rand
}

func NewSamplingHandler(handler slog.Handler, rate float64) *SamplingHandler {
	return &SamplingHandler{
		handler: handler,
		rate:    rate,
		rand:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (h *SamplingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *SamplingHandler) Handle(ctx context.Context, record slog.Record) error {
	// Increment counter atomically
	count := atomic.AddUint64(&h.counter, 1)

	// Sample based on rate
	h.mu.RLock()
	shouldLog := h.rand.Float64() < h.rate
	rate := h.rate
	h.mu.RUnlock()

	if !shouldLog {
		return nil
	}

	// Add sampling metadata
	record.Add("sample_rate", slog.Float64Value(rate))
	record.Add("sample_count", slog.Uint64Value(count))

	return h.handler.Handle(ctx, record)
}

func (h *SamplingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SamplingHandler{
		handler: h.handler.WithAttrs(attrs),
		rate:    h.rate,
		rand:    h.rand,
	}
}

func (h *SamplingHandler) WithGroup(name string) slog.Handler {
	return &SamplingHandler{
		handler: h.handler.WithGroup(name),
		rate:    h.rate,
		rand:    h.rand,
	}
}

// MetricsHandler collects metrics about logging
type MetricsHandler struct {
	slog.Handler
	serviceName string
	counters    map[slog.Level]uint64
	mu          sync.RWMutex
}

func NewMetricsHandler(h slog.Handler, serviceName string) *MetricsHandler {
	return &MetricsHandler{
		Handler:     h,
		serviceName: serviceName,
		counters:    make(map[slog.Level]uint64),
	}
}

func (h *MetricsHandler) Handle(ctx context.Context, r slog.Record) error {
	// Increment counter for this log level
	h.mu.Lock()
	h.counters[r.Level]++
	count := h.counters[r.Level]
	h.mu.Unlock()

	// Add metrics metadata
	r.Add("log_count", slog.Uint64Value(count))

	return h.Handler.Handle(ctx, r)
}

func (h *MetricsHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &MetricsHandler{
		Handler:     h.Handler.WithAttrs(attrs),
		serviceName: h.serviceName,
		counters:    h.counters,
	}
}

func (h *MetricsHandler) WithGroup(name string) slog.Handler {
	return &MetricsHandler{
		Handler:     h.Handler.WithGroup(name),
		serviceName: h.serviceName,
		counters:    h.counters,
	}
}

func (h *MetricsHandler) GetMetrics() map[slog.Level]uint64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make(map[slog.Level]uint64)
	for level, count := range h.counters {
		result[level] = count
	}
	return result
}
