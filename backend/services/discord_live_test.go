package services

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createMockDiscordLiveServer creates a test server for Discord Live webhook responses
func createMockDiscordLiveServer(t *testing.T, expectedStatus int, responseBody string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request structure based on method
		switch r.Method {
		case http.MethodPost:
			// For creating messages
			assert.Contains(t, r.URL.Path, "/webhook") // Should be basic webhook path
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			// Parse and validate request body
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var payload map[string]interface{}
			err = json.Unmarshal(body, &payload)
			require.NoError(t, err)

		case http.MethodPatch:
			// For updating messages
			assert.Contains(t, r.URL.Path, "/messages/") // Should contain message ID
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		}

		w.WriteHeader(expectedStatus)
		if responseBody != "" {
			w.Write([]byte(responseBody))
		}
	}))
}

func TestNewDiscordLiveService(t *testing.T) {
	tests := []struct {
		name          string
		webhookURL    string
		expectedID    string
		expectedToken string
	}{
		{
			name:          "Valid webhook URL",
			webhookURL:    "https://discord.com/api/webhooks/123456789/abcdefghijklmnop",
			expectedID:    "123456789",
			expectedToken: "abcdefghijklmnop",
		},
		{
			name:          "Webhook URL with trailing slash",
			webhookURL:    "https://discord.com/api/webhooks/987654321/zyxwvutsrqponmlk/",
			expectedID:    "987654321",
			expectedToken: "zyxwvutsrqponmlk",
		},
		{
			name:          "Empty webhook URL",
			webhookURL:    "",
			expectedID:    "",
			expectedToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewDiscordLiveService(tt.webhookURL)

			assert.NotNil(t, service)
			assert.Equal(t, tt.webhookURL, service.webhookURL)
			assert.Equal(t, tt.expectedID, service.webhookID)
			assert.Equal(t, tt.expectedToken, service.webhookToken)
			
			// For empty webhook URL, some fields might be nil
			if tt.webhookURL != "" {
				assert.NotNil(t, service.client)
				assert.NotNil(t, service.activeMessages)
				assert.Equal(t, 10*time.Second, service.client.Timeout)
			}
		})
	}
}

func TestDiscordLiveService_GetESTTime(t *testing.T) {
	service := NewDiscordLiveService("https://test.com/webhook")

	estTime := service.getESTTime()

	// Verify we get a valid time
	assert.False(t, estTime.IsZero())

	// Verify timezone (should be EST/EDT)
	zone, _ := estTime.Zone()
	assert.True(t, zone == "EST" || zone == "EDT", "Expected EST or EDT timezone, got %s", zone)
}

func TestDiscordLiveService_FormatESTTime(t *testing.T) {
	service := NewDiscordLiveService("https://test.com/webhook")

	// Test with a known time
	testTime := time.Date(2023, 12, 25, 15, 30, 0, 0, time.UTC)
	formatted := service.formatESTTime(testTime)

	// Should format as "3:04 PM EST" format
	assert.Regexp(t, `^\d{1,2}:\d{2} (AM|PM) EST$`, formatted)
}

func TestDiscordLiveService_CreateMessage_Success(t *testing.T) {
	// Mock server that returns a message with ID
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.RawQuery, "wait=true")

		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"id": "message_123",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	service := NewDiscordLiveService(server.URL)

	embed := map[string]interface{}{
		"title": "Test Message",
		"color": 0x00ff00,
	}

	messageID, err := service.createMessage("Test content", embed)

	assert.NoError(t, err)
	assert.Equal(t, "message_123", messageID)
}

func TestDiscordLiveService_CreateMessage_EmptyWebhook(t *testing.T) {
	service := NewDiscordLiveService("")

	messageID, err := service.createMessage("Test", nil)

	assert.NoError(t, err)
	assert.Empty(t, messageID)
}

func TestDiscordLiveService_CreateMessage_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	service := NewDiscordLiveService(server.URL)

	messageID, err := service.createMessage("Test", nil)

	assert.Error(t, err)
	assert.Empty(t, messageID)
	assert.Contains(t, err.Error(), "status 400")
}

func TestDiscordLiveService_UpdateMessage_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Initial creation
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{"id": "message_123"}
			json.NewEncoder(w).Encode(response)
		} else if r.Method == http.MethodPatch {
			// Message update
			assert.Contains(t, r.URL.Path, "message_123")
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	// Parse server URL to simulate webhook ID/token
	// parts := strings.Split(server.URL, "/") // unused for now
	baseURL := server.URL + "/webhooks/123/abc"

	service := &DiscordLiveService{
		webhookURL:     baseURL,
		webhookID:      "123",
		webhookToken:   "abc",
		activeMessages: make(map[string]*LiveMessage),
		client:         &http.Client{Timeout: 10 * time.Second},
	}

	err := service.updateMessage("message_123", "Updated content", nil)
	assert.NoError(t, err)
}

func TestDiscordLiveService_SendServerStartup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{"id": "startup_message_123"}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	service := NewDiscordLiveService(server.URL)

	messageID, err := service.SendServerStartup()

	assert.NoError(t, err)
	assert.Equal(t, "startup_message_123", messageID)

	// Verify message was stored
	assert.Contains(t, service.activeMessages, messageID)
	assert.Equal(t, MessageTypeServer, service.activeMessages[messageID].Type)
}

func TestDiscordLiveService_UpdateServerStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": "msg_123"})
		} else if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	service := &DiscordLiveService{
		webhookURL:     server.URL,
		webhookID:      "123",
		webhookToken:   "abc",
		activeMessages: make(map[string]*LiveMessage),
		client:         &http.Client{Timeout: 10 * time.Second},
	}

	// Add a mock message to activeMessages
	service.activeMessages["msg_123"] = &LiveMessage{
		ID:        "msg_123",
		Type:      MessageTypeServer,
		CreatedAt: time.Now().Add(-5 * time.Minute),
		UpdatedAt: time.Now(),
	}

	tests := []struct {
		name    string
		status  string
		isReady bool
	}{
		{
			name:    "Server initializing",
			status:  "Initializing services...",
			isReady: false,
		},
		{
			name:    "Server ready",
			status:  "Ready to accept uploads",
			isReady: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.UpdateServerStatus("msg_123", tt.status, tt.isReady)
			assert.NoError(t, err)
		})
	}
}

func TestDiscordLiveService_CalculateUptime(t *testing.T) {
	service := NewDiscordLiveService("https://test.com/webhook")

	// Test with existing message
	createdAt := time.Now().Add(-2*time.Hour - 30*time.Minute - 45*time.Second)
	service.activeMessages["test_msg"] = &LiveMessage{
		ID:        "test_msg",
		CreatedAt: createdAt,
	}

	uptime := service.calculateUptime("test_msg")
	assert.Contains(t, uptime, "2h 30m") // Should show hours and minutes

	// Test with non-existent message
	uptime = service.calculateUptime("non_existent")
	assert.Equal(t, "Unknown", uptime)

	// Test with recent message (less than a minute)
	recentMessage := &LiveMessage{
		ID:        "recent_msg",
		CreatedAt: time.Now().Add(-30 * time.Second),
	}
	service.activeMessages["recent_msg"] = recentMessage

	uptime = service.calculateUptime("recent_msg")
	assert.Contains(t, uptime, "s") // Should show seconds
}

func TestDiscordLiveService_SendUploadProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{"id": "upload_progress_123"}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	service := NewDiscordLiveService(server.URL)

	files := []string{"file1.wav", "file2.wav", "file3.wav"}

	messageID, err := service.SendUploadProgress(files)

	assert.NoError(t, err)
	assert.Equal(t, "upload_progress_123", messageID)
}

func TestDiscordLiveService_UpdateUploadProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": "msg_123"})
		} else if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	service := &DiscordLiveService{
		webhookURL:     server.URL,
		webhookID:      "123",
		webhookToken:   "abc",
		activeMessages: make(map[string]*LiveMessage),
		client:         &http.Client{Timeout: 10 * time.Second},
	}

	// Add message to track uptime
	service.activeMessages["msg_123"] = &LiveMessage{
		ID:        "msg_123",
		CreatedAt: time.Now().Add(-1 * time.Minute),
	}

	fileProgress := map[string]int{
		"file1.wav": 50,
		"file2.wav": 100,
		"file3.wav": 25,
	}

	err := service.UpdateUploadProgress("msg_123", fileProgress)
	assert.NoError(t, err)
}

func TestDiscordLiveService_GenerateProgressBar(t *testing.T) {
	service := NewDiscordLiveService("https://test.com/webhook")

	tests := []struct {
		name     string
		percent  int
		expected string
	}{
		{
			name:     "0 percent",
			percent:  0,
			expected: "░░░░░░░░░░",
		},
		{
			name:     "50 percent",
			percent:  50,
			expected: "█████░░░░░",
		},
		{
			name:     "100 percent",
			percent:  100,
			expected: "██████████",
		},
		{
			name:     "Over 100 percent",
			percent:  150,
			expected: "██████████",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.generateProgressBar(tt.percent)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, 10, len(result)) // Always 10 characters
		})
	}
}

func TestDiscordLiveService_SendError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{"id": "error_123"}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	service := NewDiscordLiveService(server.URL)

	err := service.SendError("Test Error", "This is a test error message")
	assert.NoError(t, err)
}

func TestDiscordLiveService_CleanupOldMessages(t *testing.T) {
	service := NewDiscordLiveService("https://test.com/webhook")

	// Add messages with different ages
	now := time.Now()
	service.activeMessages["old_msg"] = &LiveMessage{
		ID:        "old_msg",
		CreatedAt: now.Add(-48 * time.Hour), // 48 hours old
	}
	service.activeMessages["recent_msg"] = &LiveMessage{
		ID:        "recent_msg",
		CreatedAt: now.Add(-1 * time.Hour), // 1 hour old
	}

	// Clean up messages older than 24 hours
	service.CleanupOldMessages(24)

	// Old message should be removed, recent should remain
	assert.NotContains(t, service.activeMessages, "old_msg")
	assert.Contains(t, service.activeMessages, "recent_msg")
}

func TestDiscordLiveService_BackwardCompatibility(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodPost {
			json.NewEncoder(w).Encode(map[string]interface{}{"id": "compat_123"})
		}
	}))
	defer server.Close()

	service := NewDiscordLiveService(server.URL)

	// Test backward compatibility methods
	fields := []DiscordField{
		{Name: "Test", Value: "Value", Inline: true},
	}

	err := service.SendNotification("Test Title", "Test Description", 0x00ff00, fields)
	assert.NoError(t, err)

	err = service.SendStartupNotification("Test startup message")
	assert.NoError(t, err)
}

// Test message type constants
func TestMessageType_Constants(t *testing.T) {
	assert.Equal(t, MessageType("server"), MessageTypeServer)
	assert.Equal(t, MessageType("upload"), MessageTypeUpload)
	assert.Equal(t, MessageType("error"), MessageTypeError)
	assert.Equal(t, MessageType("admin"), MessageTypeAdmin)
}

// Test LiveMessage structure
func TestLiveMessage_Structure(t *testing.T) {
	now := time.Now()
	msg := &LiveMessage{
		ID:        "test_123",
		Type:      MessageTypeUpload,
		CreatedAt: now,
		UpdatedAt: now,
		Data:      map[string]interface{}{"test": "data"},
	}

	assert.Equal(t, "test_123", msg.ID)
	assert.Equal(t, MessageTypeUpload, msg.Type)
	assert.Equal(t, now, msg.CreatedAt)
	assert.Equal(t, now, msg.UpdatedAt)
	assert.NotNil(t, msg.Data)
}

// Benchmark tests
func BenchmarkDiscordLiveService_GenerateProgressBar(b *testing.B) {
	service := NewDiscordLiveService("https://test.com/webhook")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.generateProgressBar(50)
	}
}

func BenchmarkDiscordLiveService_FormatESTTime(b *testing.B) {
	service := NewDiscordLiveService("https://test.com/webhook")
	testTime := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.formatESTTime(testTime)
	}
}

// Test concurrent message management
func TestDiscordLiveService_ConcurrentMessageAccess(t *testing.T) {
	service := NewDiscordLiveService("https://test.com/webhook")

	// Add initial message
	service.activeMessages["test_msg"] = &LiveMessage{
		ID:        "test_msg",
		Type:      MessageTypeServer,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Run concurrent operations
	done := make(chan bool)

	// Goroutine 1: Read operations
	go func() {
		for i := 0; i < 100; i++ {
			service.calculateUptime("test_msg")
		}
		done <- true
	}()

	// Goroutine 2: Write operations
	go func() {
		for i := 0; i < 50; i++ {
			service.CleanupOldMessages(1)
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Should not panic or race
}

// Test edge cases
func TestDiscordLiveService_EdgeCases(t *testing.T) {
	service := NewDiscordLiveService("https://test.com/webhook")

	t.Run("Empty file list", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": "empty_123"})
		}))
		defer server.Close()

		service.webhookURL = server.URL
		messageID, err := service.SendUploadProgress([]string{})
		assert.NoError(t, err)
		assert.NotEmpty(t, messageID)
	})

	t.Run("Nil file progress map", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPatch {
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		service.webhookURL = server.URL
		service.webhookID = "123"
		service.webhookToken = "abc"

		err := service.UpdateUploadProgress("msg_123", nil)
		assert.NoError(t, err)
	})

	t.Run("Very long uptime", func(t *testing.T) {
		// Test with a message that's days old
		veryOldTime := time.Now().Add(-72 * time.Hour) // 3 days
		service.activeMessages["old_test"] = &LiveMessage{
			ID:        "old_test",
			CreatedAt: veryOldTime,
		}

		uptime := service.calculateUptime("old_test")
		assert.Contains(t, uptime, "h") // Should contain hours
	})
}
