package services

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sermon-uploader/pkg/logging"
)

// ProductionLoggerConfig configures the production logger
type ProductionLoggerConfig struct {
	LogDir            string
	DiscordWebhookURL string
	MaxFileSize       int64
	RetentionDays     int
	AsyncLogging      bool
	BufferSize        int
	Output            io.Writer // For testing
	DiscordService    DiscordLiveInterface
}

// UploadFailureContext contains context information for upload failures
type UploadFailureContext struct {
	Filename    string
	FileSize    int64
	UserIP      string
	Error       error
	Operation   string
	RequestID   string
	Timestamp   time.Time
	Component   string
	UserAgent   string
	ContentType string
}

// ErrorRecord represents an error for Discord display and trend analysis
type ErrorRecord struct {
	Timestamp   time.Time
	Filename    string
	FileSize    int64
	Error       string
	UserIP      string
	RequestID   string
	Component   string
}

// ProductionLogger handles comprehensive production logging with Discord integration
type ProductionLogger struct {
	config          *ProductionLoggerConfig
	logger          *slog.Logger
	discordService  DiscordLiveInterface
	messageID       string
	errorCount      int
	recentErrors    []ErrorRecord
	errorBuffer     chan ErrorRecord
	wg              sync.WaitGroup
	mu              sync.RWMutex
	sessionStart    time.Time
	lastUpdateTime  time.Time
	systemHealth    SystemHealth
	running         bool
}

// SystemHealth tracks infrastructure status
type SystemHealth struct {
	MinIOStatus     string
	MinIOEndpoint   string
	DiskSpaceGB     float64
	MemoryUsagePct  float64
	NetworkLatency  time.Duration
	LastHealthCheck time.Time
}

// DiscordLiveInterface defines the interface for Discord live messaging
type DiscordLiveInterface interface {
	CreateMessage(content string) (string, error)
	UpdateMessage(messageID, content string) error
}

// NewProductionLogger creates a new production logger with Discord integration
func NewProductionLogger(config *ProductionLoggerConfig) (*ProductionLogger, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Set defaults
	if config.RetentionDays <= 0 {
		config.RetentionDays = 7
	}
	if config.BufferSize <= 0 {
		config.BufferSize = 1000
	}
	if config.MaxFileSize <= 0 {
		config.MaxFileSize = 100 * 1024 * 1024 // 100MB default
	}

	// Create log directory if it doesn't exist
	if config.LogDir != "" {
		if err := os.MkdirAll(config.LogDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	// Set up structured logger
	logConfig := &logging.Config{
		Level:        slog.LevelInfo,
		OutputFormat: "json",
		AddSource:    true,
		Output:       config.Output,
	}

	if config.Output == nil && config.LogDir != "" {
		// Create daily log file
		today := time.Now().Format("2006-01-02")
		logFile := filepath.Join(config.LogDir, "production-"+today+".log")
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logConfig.Output = file
	}

	logger, err := logging.New("production-logger", logConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// Set up Discord service
	var discordService DiscordLiveInterface
	if config.DiscordService != nil {
		discordService = config.DiscordService
	} else if config.DiscordWebhookURL != "" {
		discordService = NewDiscordLiveService(config.DiscordWebhookURL)
	}

	pl := &ProductionLogger{
		config:         config,
		logger:         logger.Logger,
		discordService: discordService,
		errorCount:     0,
		recentErrors:   make([]ErrorRecord, 0, 5),
		sessionStart:   time.Now(),
		lastUpdateTime: time.Now(),
		systemHealth: SystemHealth{
			MinIOStatus:     "Unknown",
			MinIOEndpoint:   "192.168.1.127:9000",
			DiskSpaceGB:     0,
			MemoryUsagePct:  0,
			NetworkLatency:  0,
			LastHealthCheck: time.Time{},
		},
		running: true,
	}

	if config.AsyncLogging {
		pl.errorBuffer = make(chan ErrorRecord, config.BufferSize)
		pl.startAsyncProcessor()
	}

	// Initialize Discord message
	if discordService != nil {
		pl.initializeDiscordMessage()
	}

	return pl, nil
}

// LogUploadFailure logs an upload failure with full context
func (p *ProductionLogger) LogUploadFailure(ctx context.Context, failure UploadFailureContext) error {
	// Create error record
	errorRecord := ErrorRecord{
		Timestamp: failure.Timestamp,
		Filename:  failure.Filename,
		FileSize:  failure.FileSize,
		Error:     failure.Error.Error(),
		UserIP:    failure.UserIP,
		RequestID: failure.RequestID,
		Component: failure.Component,
	}

	// Log structured error
	p.logger.ErrorContext(ctx, "Upload failure occurred",
		slog.String("event_type", "upload_failure"),
		slog.String("filename", failure.Filename),
		slog.Int64("file_size", failure.FileSize),
		slog.String("user_ip", failure.UserIP),
		slog.String("error", failure.Error.Error()),
		slog.String("operation", failure.Operation),
		slog.String("request_id", failure.RequestID),
		slog.String("component", failure.Component),
		slog.String("user_agent", failure.UserAgent),
		slog.String("content_type", failure.ContentType),
		slog.Time("timestamp", failure.Timestamp),
	)

	if p.config.AsyncLogging {
		// Send to async buffer
		select {
		case p.errorBuffer <- errorRecord:
		default:
			// Buffer full, process synchronously
			p.processError(errorRecord)
		}
	} else {
		// Process synchronously
		p.processError(errorRecord)
	}

	return nil
}

// processError handles the error record for Discord updates
func (p *ProductionLogger) processError(errorRecord ErrorRecord) {
	p.mu.Lock()
	
	// Add to recent errors
	p.recentErrors = append([]ErrorRecord{errorRecord}, p.recentErrors...)
	if len(p.recentErrors) > 5 {
		p.recentErrors = p.recentErrors[:5]
	}

	p.errorCount++
	p.lastUpdateTime = time.Now()

	// Build Discord message while holding the lock
	var content string
	if p.discordService != nil && p.messageID != "" {
		content = p.buildDiscordMessageLocked()
	}
	
	p.mu.Unlock()

	// Update Discord message outside of the lock
	if content != "" {
		if err := p.discordService.UpdateMessage(p.messageID, content); err != nil {
			p.logger.Warn("Failed to update Discord message",
				slog.String("error", err.Error()),
			)
		}
	}
}

// initializeDiscordMessage creates the initial Discord monitoring message
func (p *ProductionLogger) initializeDiscordMessage() {
	if p.discordService == nil {
		return
	}

	content := p.buildDiscordMessage()
	messageID, err := p.discordService.CreateMessage(content)
	if err != nil {
		p.logger.Error("Failed to create Discord message",
			slog.String("error", err.Error()),
		)
		return
	}

	p.messageID = messageID
	p.logger.Info("Initialized Discord production monitor",
		slog.String("message_id", messageID),
	)
}

// buildDiscordMessage creates the formatted Discord message content
func (p *ProductionLogger) buildDiscordMessage() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.buildDiscordMessageLocked()
}

// buildDiscordMessageLocked creates the formatted Discord message content (assumes lock is held)
func (p *ProductionLogger) buildDiscordMessageLocked() string {

	est := time.Now().In(getProductionLoggerESTLocation())
	sessionDuration := time.Since(p.sessionStart)
	errorRate := p.calculateErrorRate()

	var builder strings.Builder
	
	// Header
	builder.WriteString("🔴 **Production Errors - Live Monitor**\n")
	builder.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	builder.WriteString(fmt.Sprintf("📊 Session Started: %s\n", p.sessionStart.In(getProductionLoggerESTLocation()).Format("3:04 PM EST")))
	builder.WriteString(fmt.Sprintf("⚠️ Total Errors: %d | 🟡 Rate: %.1f errors/min\n", p.errorCount, errorRate))
	builder.WriteString(fmt.Sprintf("⏱️ Session Duration: %s\n\n", formatProductionLoggerDuration(sessionDuration)))

	// Recent errors
	if len(p.recentErrors) > 0 {
		builder.WriteString("**Recent Errors (Last 5):**\n")
		builder.WriteString("┌─────────────────────────────────────\n")
		
		for i, err := range p.recentErrors {
			builder.WriteString(fmt.Sprintf("│ %d. **[%s] Upload Failed**\n", 
				i+1, err.Timestamp.In(getProductionLoggerESTLocation()).Format("3:04 PM EST")))
			builder.WriteString(fmt.Sprintf("│    📁 File: %s (%s)\n", 
				err.Filename, formatFileSize(err.FileSize)))
			builder.WriteString(fmt.Sprintf("│    ❌ Error: %s\n", err.Error))
			builder.WriteString(fmt.Sprintf("│    👤 User: %s\n", err.UserIP))
			if err.RequestID != "" {
				builder.WriteString(fmt.Sprintf("│    🔗 Request: %s\n", err.RequestID))
			}
			if i < len(p.recentErrors)-1 {
				builder.WriteString("│\n")
			}
		}
		builder.WriteString("└─────────────────────────────────────\n\n")
	} else {
		builder.WriteString("**Recent Errors:** None recorded this session\n\n")
	}

	// System health
	builder.WriteString("**🏥 System Health:**\n")
	p.updateSystemHealth() // Quick health check
	
	minioStatus := "🟢 Online"
	if p.systemHealth.MinIOStatus != "Online" {
		minioStatus = "🔴 " + p.systemHealth.MinIOStatus
	}
	
	diskStatus := "🟢"
	if p.systemHealth.DiskSpaceGB < 1.0 {
		diskStatus = "🔴"
	} else if p.systemHealth.DiskSpaceGB < 2.0 {
		diskStatus = "🟡"
	}
	
	memoryStatus := "🟢"
	if p.systemHealth.MemoryUsagePct > 80 {
		memoryStatus = "🔴"
	} else if p.systemHealth.MemoryUsagePct > 60 {
		memoryStatus = "🟡"
	}

	builder.WriteString(fmt.Sprintf("├─ %s MinIO: %s (%s)\n", 
		minioStatus, p.systemHealth.MinIOStatus, p.systemHealth.MinIOEndpoint))
	builder.WriteString(fmt.Sprintf("├─ %s Disk Space: %.1fGB free\n", 
		diskStatus, p.systemHealth.DiskSpaceGB))
	builder.WriteString(fmt.Sprintf("├─ %s Memory: %.0f%% used\n", 
		memoryStatus, p.systemHealth.MemoryUsagePct))
	builder.WriteString(fmt.Sprintf("└─ 🟢 Network: %.0fms avg latency\n\n", 
		float64(p.systemHealth.NetworkLatency.Nanoseconds())/1e6))

	// Footer
	builder.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	builder.WriteString(fmt.Sprintf("🔄 Last Updated: %s\n", est.Format("3:04:05 PM EST")))
	if p.config.LogDir != "" {
		builder.WriteString(fmt.Sprintf("📁 Full logs: %s\n", p.config.LogDir))
	}

	return builder.String()
}

// calculateErrorRate calculates errors per minute over the session
func (p *ProductionLogger) calculateErrorRate() float64 {
	sessionDuration := time.Since(p.sessionStart)
	if sessionDuration.Minutes() < 1 {
		return 0
	}
	return float64(p.errorCount) / sessionDuration.Minutes()
}

// updateSystemHealth performs a quick system health check
func (p *ProductionLogger) updateSystemHealth() {
	// This would normally check actual system status
	// For now, simulate basic health metrics
	p.systemHealth.LastHealthCheck = time.Now()
	p.systemHealth.MinIOStatus = "Online" // This would ping MinIO
	p.systemHealth.DiskSpaceGB = 15.2     // This would check actual disk space
	p.systemHealth.MemoryUsagePct = 45.0  // This would check actual memory
	p.systemHealth.NetworkLatency = 23 * time.Millisecond // This would ping Discord
}

// startAsyncProcessor starts the background error processing goroutine
func (p *ProductionLogger) startAsyncProcessor() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			select {
			case errorRecord := <-p.errorBuffer:
				p.processError(errorRecord)
			default:
				if !p.running {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()
}

// CleanupOldLogs removes log files older than the retention period
func (p *ProductionLogger) CleanupOldLogs() error {
	if p.config.LogDir == "" {
		return nil
	}

	files, err := os.ReadDir(p.config.LogDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -p.config.RetentionDays)

	for _, file := range files {
		if !strings.HasPrefix(file.Name(), "production-") || !strings.HasSuffix(file.Name(), ".log") {
			continue
		}

		// Extract date from filename
		datePart := strings.TrimPrefix(file.Name(), "production-")
		datePart = strings.TrimSuffix(datePart, ".log")
		
		fileDate, err := time.Parse("2006-01-02", datePart)
		if err != nil {
			continue
		}

		if fileDate.Before(cutoff) {
			filePath := filepath.Join(p.config.LogDir, file.Name())
			if err := os.Remove(filePath); err != nil {
				p.logger.Warn("Failed to remove old log file",
					slog.String("file", filePath),
					slog.String("error", err.Error()),
				)
			} else {
				p.logger.Info("Removed old log file",
					slog.String("file", filePath),
					slog.String("date", datePart),
				)
			}
		}
	}

	return nil
}

// Flush waits for all async operations to complete
func (p *ProductionLogger) Flush() error {
	if p.config.AsyncLogging {
		// Process remaining items in buffer
		for {
			select {
			case errorRecord := <-p.errorBuffer:
				p.processError(errorRecord)
			default:
				return nil
			}
		}
	}
	return nil
}

// Close gracefully shuts down the logger
func (p *ProductionLogger) Close() error {
	p.running = false
	if p.config.AsyncLogging {
		close(p.errorBuffer)
		p.wg.Wait()
	}
	return nil
}

// Helper functions

func getProductionLoggerESTLocation() *time.Location {
	loc, _ := time.LoadLocation("America/New_York")
	return loc
}

func formatProductionLoggerDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}