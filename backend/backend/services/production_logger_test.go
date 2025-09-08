package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProductionLogger_LogUploadFailure tests that upload failures are captured with context
// Should include: filename, size, error, timestamp, user IP
func TestProductionLogger_LogUploadFailure(t *testing.T) {
	tests := []struct {
		name           string
		filename       string
		fileSize       int64
		userIP         string
		err            error
		expectedFields []string
	}{
		{
			name:     "upload timeout error",
			filename: "sermon_2025.wav",
			fileSize: 524288000, // 500MB
			userIP:   "192.168.1.100",
			err:      errors.New("request timeout after 30s"),
			expectedFields: []string{
				"sermon_2025.wav",
				"524288000",
				"192.168.1.100",
				"request timeout after 30s",
				"upload_failure",
				"ERROR",
			},
		},
		{
			name:     "minio connection refused",
			filename: "sermon_2025_2.wav",
			fileSize: 786432000, // 750MB
			userIP:   "192.168.1.100",
			err:      errors.New("MinIO connection refused"),
			expectedFields: []string{
				"sermon_2025_2.wav",
				"786432000",
				"192.168.1.100",
				"MinIO connection refused",
				"upload_failure",
				"ERROR",
			},
		},
		{
			name:     "file too large error",
			filename: "large_sermon.wav",
			fileSize: 2147483648, // 2GB
			userIP:   "192.168.1.50",
			err:      errors.New("file exceeds maximum size limit"),
			expectedFields: []string{
				"large_sermon.wav",
				"2147483648",
				"192.168.1.50",
				"file exceeds maximum size limit",
				"upload_failure",
				"ERROR",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for logs
			tempDir := t.TempDir()
			
			// Create buffer to capture logs
			var buf bytes.Buffer
			
			// Create production logger (no Discord for this test to focus on log output)
			logger, err := NewProductionLogger(&ProductionLoggerConfig{
				LogDir:           tempDir,
				DiscordWebhookURL: "", // Disable Discord for this test
				MaxFileSize:      10 * 1024 * 1024, // 10MB
				RetentionDays:    7,
				Output:           &buf,
			})
			
			require.NoError(t, err)
			require.NotNil(t, logger)
			
			ctx := context.Background()
			
			// Log the upload failure
			err = logger.LogUploadFailure(ctx, UploadFailureContext{
				Filename:    tt.filename,
				FileSize:    tt.fileSize,
				UserIP:      tt.userIP,
				Error:       tt.err,
				Operation:   "file_upload",
				RequestID:   "req-12345",
				Timestamp:   time.Now(),
			})
			
			require.NoError(t, err)
			
			// Verify log output contains expected fields
			output := buf.String()
			for _, field := range tt.expectedFields {
				assert.Contains(t, output, field, "Expected field not found: %s", field)
			}
			
			// Verify it's valid JSON
			lines := strings.Split(strings.TrimSpace(output), "\n")
			assert.Greater(t, len(lines), 0, "Should have at least one log line")
			
			var logEntry map[string]interface{}
			err = json.Unmarshal([]byte(lines[0]), &logEntry)
			assert.NoError(t, err, "Log output should be valid JSON")
			
			// Verify required fields are present
			assert.Contains(t, logEntry, "filename")
			assert.Contains(t, logEntry, "file_size")
			assert.Contains(t, logEntry, "user_ip")
			assert.Contains(t, logEntry, "error")
			assert.Contains(t, logEntry, "event_type")
			assert.Equal(t, "upload_failure", logEntry["event_type"])
		})
	}
}

// TestProductionLogger_DiscordLiveMessage tests that errors update a single Discord message
// Not creating new messages for each error
func TestProductionLogger_DiscordLiveMessage(t *testing.T) {
	tempDir := t.TempDir()
	
	// Mock Discord service to track message updates
	mockDiscord := &MockDiscordLiveService{
		messages: make(map[string]*LiveMessage),
	}
	
	logger, err := NewProductionLogger(&ProductionLoggerConfig{
		LogDir:           tempDir,
		DiscordWebhookURL: "https://discord.com/api/webhooks/test/token",
		MaxFileSize:      10 * 1024 * 1024,
		RetentionDays:    7,
		DiscordService:   mockDiscord,
	})
	
	require.NoError(t, err)
	require.NotNil(t, logger)
	
	ctx := context.Background()
	
	// Log multiple upload failures
	failures := []UploadFailureContext{
		{
			Filename:  "sermon1.wav",
			FileSize:  500000000,
			UserIP:    "192.168.1.100",
			Error:     errors.New("timeout error"),
			Operation: "file_upload",
			RequestID: "req-1",
			Timestamp: time.Now(),
		},
		{
			Filename:  "sermon2.wav",
			FileSize:  600000000,
			UserIP:    "192.168.1.100",
			Error:     errors.New("connection refused"),
			Operation: "file_upload",
			RequestID: "req-2",
			Timestamp: time.Now().Add(1 * time.Minute),
		},
		{
			Filename:  "sermon3.wav",
			FileSize:  700000000,
			UserIP:    "192.168.1.100",
			Error:     errors.New("disk full"),
			Operation: "file_upload",
			RequestID: "req-3",
			Timestamp: time.Now().Add(2 * time.Minute),
		},
	}
	
	for _, failure := range failures {
		err := logger.LogUploadFailure(ctx, failure)
		require.NoError(t, err)
	}
	
	// Should have created exactly ONE Discord message and updated it
	assert.Equal(t, 1, mockDiscord.CreateMessageCallCount(), "Should create only one Discord message")
	assert.Equal(t, 3, mockDiscord.UpdateMessageCallCount(), "Should update the message for each error")
	
	// Verify the message contains all errors
	messageContent := mockDiscord.GetLatestMessageContent()
	assert.Contains(t, messageContent, "sermon1.wav")
	assert.Contains(t, messageContent, "sermon2.wav")
	assert.Contains(t, messageContent, "sermon3.wav")
	assert.Contains(t, messageContent, "Total Errors: 3")
}

// TestProductionLogger_LocalFileStorage tests that logs are also saved locally for debugging
// Should rotate daily, keep 7 days
func TestProductionLogger_LocalFileStorage(t *testing.T) {
	tempDir := t.TempDir()
	
	logger, err := NewProductionLogger(&ProductionLoggerConfig{
		LogDir:           tempDir,
		DiscordWebhookURL: "",
		MaxFileSize:      10 * 1024 * 1024,
		RetentionDays:    7,
	})
	
	require.NoError(t, err)
	require.NotNil(t, logger)
	
	ctx := context.Background()
	
	// Log some failures
	failure := UploadFailureContext{
		Filename:  "test.wav",
		FileSize:  1000000,
		UserIP:    "192.168.1.100",
		Error:     errors.New("test error"),
		Operation: "file_upload",
		RequestID: "req-test",
		Timestamp: time.Now(),
	}
	
	err = logger.LogUploadFailure(ctx, failure)
	require.NoError(t, err)
	
	// Verify log file was created
	today := time.Now().Format("2006-01-02")
	expectedLogFile := filepath.Join(tempDir, "production-"+today+".log")
	
	_, err = os.Stat(expectedLogFile)
	assert.NoError(t, err, "Log file should be created")
	
	// Verify log file contains the error
	content, err := os.ReadFile(expectedLogFile)
	require.NoError(t, err)
	
	logContent := string(content)
	assert.Contains(t, logContent, "test.wav")
	assert.Contains(t, logContent, "test error")
	assert.Contains(t, logContent, "upload_failure")
	
	// Test log rotation - create old log files
	for i := 1; i <= 10; i++ {
		oldDate := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		oldLogFile := filepath.Join(tempDir, "production-"+oldDate+".log")
		err := os.WriteFile(oldLogFile, []byte("old log content"), 0644)
		require.NoError(t, err)
	}
	
	// Trigger cleanup
	err = logger.CleanupOldLogs()
	require.NoError(t, err)
	
	// Verify only 7 days of logs are kept
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	
	logFileCount := 0
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "production-") && strings.HasSuffix(file.Name(), ".log") {
			logFileCount++
		}
	}
	
	assert.LessOrEqual(t, logFileCount, 7, "Should keep only 7 days of logs")
}

// TestProductionLogger_PerformanceOnPi tests that logging doesn't impact performance
// Should be async, non-blocking
func TestProductionLogger_PerformanceOnPi(t *testing.T) {
	tempDir := t.TempDir()
	
	logger, err := NewProductionLogger(&ProductionLoggerConfig{
		LogDir:           tempDir,
		DiscordWebhookURL: "",
		MaxFileSize:      10 * 1024 * 1024,
		RetentionDays:    7,
		AsyncLogging:     true, // Enable async logging
		BufferSize:       1000,
	})
	
	require.NoError(t, err)
	require.NotNil(t, logger)
	
	ctx := context.Background()
	
	// Measure performance of logging many failures
	start := time.Now()
	
	for i := 0; i < 100; i++ {
		failure := UploadFailureContext{
			Filename:  "test.wav",
			FileSize:  1000000,
			UserIP:    "192.168.1.100",
			Error:     errors.New("test error"),
			Operation: "file_upload",
			RequestID: "req-test",
			Timestamp: time.Now(),
		}
		
		err := logger.LogUploadFailure(ctx, failure)
		require.NoError(t, err)
	}
	
	duration := time.Since(start)
	
	// Should be very fast (async, non-blocking)
	assert.Less(t, duration, 100*time.Millisecond, "Async logging should be fast")
	
	// Wait for async processing to complete
	err = logger.Flush()
	require.NoError(t, err)
	
	// Verify all logs were written
	today := time.Now().Format("2006-01-02")
	expectedLogFile := filepath.Join(tempDir, "production-"+today+".log")
	
	content, err := os.ReadFile(expectedLogFile)
	require.NoError(t, err)
	
	logLines := strings.Split(strings.TrimSpace(string(content)), "\n")
	assert.Equal(t, 100, len(logLines), "Should have logged all 100 failures")
}

// Test Discord rate limiting behavior
func TestProductionLogger_DiscordRateLimiting(t *testing.T) {
	tempDir := t.TempDir()
	
	// Mock Discord service with rate limiting
	mockDiscord := &MockDiscordLiveService{
		messages:         make(map[string]*LiveMessage),
		simulateRateLimit: true,
		rateLimitDelay:   100 * time.Millisecond,
	}
	
	logger, err := NewProductionLogger(&ProductionLoggerConfig{
		LogDir:           tempDir,
		DiscordWebhookURL: "https://discord.com/api/webhooks/test/token",
		MaxFileSize:      10 * 1024 * 1024,
		RetentionDays:    7,
		DiscordService:   mockDiscord,
	})
	
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// Send multiple failures rapidly
	start := time.Now()
	for i := 0; i < 10; i++ {
		failure := UploadFailureContext{
			Filename:  "rapid_test.wav",
			FileSize:  1000000,
			UserIP:    "192.168.1.100",
			Error:     errors.New("rapid fire error"),
			Operation: "file_upload",
			RequestID: "req-rapid",
			Timestamp: time.Now(),
		}
		
		err := logger.LogUploadFailure(ctx, failure)
		require.NoError(t, err)
	}
	duration := time.Since(start)
	
	// Should handle rate limiting gracefully without blocking too long
	assert.Less(t, duration, 2*time.Second, "Rate limiting should not block for too long")
	
	// Verify some messages were queued due to rate limiting
	assert.Greater(t, mockDiscord.GetQueuedMessageCount(), 0, "Some messages should be queued due to rate limiting")
}

// Mock Discord service for testing
type MockDiscordLiveService struct {
	messages           map[string]*LiveMessage
	createCallCount    int
	updateCallCount    int
	latestContent      string
	simulateRateLimit  bool
	rateLimitDelay     time.Duration
	queuedMessages     int
}

func (m *MockDiscordLiveService) CreateMessage(content string) (string, error) {
	if m.simulateRateLimit && m.createCallCount > 0 {
		time.Sleep(m.rateLimitDelay)
		m.queuedMessages++
	}
	
	m.createCallCount++
	messageID := "test-message-id"
	m.messages[messageID] = &LiveMessage{
		ID:        messageID,
		Type:      MessageTypeError,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.latestContent = content
	return messageID, nil
}

func (m *MockDiscordLiveService) UpdateMessage(messageID, content string) error {
	if m.simulateRateLimit {
		time.Sleep(m.rateLimitDelay)
		m.queuedMessages++
	}
	
	m.updateCallCount++
	if msg, exists := m.messages[messageID]; exists {
		msg.UpdatedAt = time.Now()
	}
	m.latestContent = content
	return nil
}

func (m *MockDiscordLiveService) CreateMessageCallCount() int {
	return m.createCallCount
}

func (m *MockDiscordLiveService) UpdateMessageCallCount() int {
	return m.updateCallCount
}

func (m *MockDiscordLiveService) GetLatestMessageContent() string {
	return m.latestContent
}

func (m *MockDiscordLiveService) GetQueuedMessageCount() int {
	return m.queuedMessages
}

// Test-specific interfaces and mocks