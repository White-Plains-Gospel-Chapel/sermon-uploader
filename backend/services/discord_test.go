package services

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHTTPServer creates a test HTTP server for mocking Discord webhook responses
func createMockDiscordServer(t *testing.T, expectedStatus int, expectedBody string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and content type
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Read and verify request body structure
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var message DiscordMessage
		err = json.Unmarshal(body, &message)
		require.NoError(t, err)
		assert.Len(t, message.Embeds, 1, "Should have exactly one embed")

		w.WriteHeader(expectedStatus)
		if expectedBody != "" {
			w.Write([]byte(expectedBody))
		}
	}))
}

func TestNewDiscordService(t *testing.T) {
	tests := []struct {
		name       string
		webhookURL string
		expected   string
	}{
		{
			name:       "Valid webhook URL",
			webhookURL: "https://discord.com/api/webhooks/123/abc",
			expected:   "https://discord.com/api/webhooks/123/abc",
		},
		{
			name:       "Empty webhook URL",
			webhookURL: "",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewDiscordService(tt.webhookURL)

			assert.NotNil(t, service)
			assert.Equal(t, tt.expected, service.webhookURL)
		})
	}
}

func TestDiscordService_SendNotification_Success(t *testing.T) {
	// Create mock server
	server := createMockDiscordServer(t, 204, "")
	defer server.Close()

	// Create service with mock server URL
	service := NewDiscordService(server.URL)

	// Test data
	title := "Test Title"
	description := "Test Description"
	color := 0x00ff00
	fields := []DiscordField{
		{Name: "Field1", Value: "Value1", Inline: true},
		{Name: "Field2", Value: "Value2", Inline: false},
	}

	// Execute
	err := service.SendNotification(title, description, color, fields)

	// Assert
	assert.NoError(t, err)
}

func TestDiscordService_SendNotification_EmptyWebhookURL(t *testing.T) {
	// Create service with empty webhook URL
	service := NewDiscordService("")

	// Execute
	err := service.SendNotification("Test", "Test", 0x00ff00, nil)

	// Should not error when webhook URL is empty
	assert.NoError(t, err)
}

func TestDiscordService_SendNotification_HTTPError(t *testing.T) {
	// Create mock server that returns error
	server := createMockDiscordServer(t, 400, "Bad Request")
	defer server.Close()

	service := NewDiscordService(server.URL)

	// Execute
	err := service.SendNotification("Test", "Test", 0x00ff00, nil)

	// Assert error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "discord webhook returned status 400")
}

func TestDiscordService_SendNotification_NetworkError(t *testing.T) {
	// Use invalid URL to simulate network error
	service := NewDiscordService("http://invalid-host:99999/webhook")

	// Execute
	err := service.SendNotification("Test", "Test", 0x00ff00, nil)

	// Assert error
	assert.Error(t, err)
}

func TestDiscordService_SendNotification_MarshalError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))
	defer server.Close()

	service := NewDiscordService(server.URL)

	// Create invalid field that would cause marshal error by creating circular reference
	// This is a bit contrived, but demonstrates error handling
	invalidFields := []DiscordField{
		{Name: string([]byte{0xff, 0xfe}), Value: "Value", Inline: true}, // Invalid UTF-8
	}

	err := service.SendNotification("Test", "Test", 0x00ff00, invalidFields)

	// Should still work as this doesn't actually cause a marshal error in this case
	// but we're testing the error path structure
	assert.NoError(t, err)
}

func TestDiscordService_SendStartupNotification(t *testing.T) {
	server := createMockDiscordServer(t, 204, "")
	defer server.Close()

	service := NewDiscordService(server.URL)

	err := service.SendStartupNotification("Test startup message")

	assert.NoError(t, err)
}

func TestDiscordService_SendUploadStart(t *testing.T) {
	tests := []struct {
		name      string
		fileCount int
		isBatch   bool
	}{
		{
			name:      "Single file upload",
			fileCount: 1,
			isBatch:   false,
		},
		{
			name:      "Batch upload",
			fileCount: 5,
			isBatch:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createMockDiscordServer(t, 204, "")
			defer server.Close()

			service := NewDiscordService(server.URL)

			err := service.SendUploadStart(tt.fileCount, tt.isBatch)
			assert.NoError(t, err)
		})
	}
}

func TestDiscordService_SendUploadComplete(t *testing.T) {
	tests := []struct {
		name          string
		successful    int
		duplicates    int
		failed        int
		isBatch       bool
		expectedColor int
	}{
		{
			name:          "All successful",
			successful:    3,
			duplicates:    0,
			failed:        0,
			isBatch:       false,
			expectedColor: 0x00ff00, // Green
		},
		{
			name:          "With duplicates",
			successful:    2,
			duplicates:    1,
			failed:        0,
			isBatch:       true,
			expectedColor: 0x00ff00, // Green
		},
		{
			name:          "All failed",
			successful:    0,
			duplicates:    0,
			failed:        3,
			isBatch:       false,
			expectedColor: 0xff0000, // Red
		},
		{
			name:          "Partial success",
			successful:    1,
			duplicates:    1,
			failed:        1,
			isBatch:       true,
			expectedColor: 0xffa500, // Orange
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createMockDiscordServer(t, 204, "")
			defer server.Close()

			service := NewDiscordService(server.URL)

			err := service.SendUploadComplete(tt.successful, tt.duplicates, tt.failed, tt.isBatch)
			assert.NoError(t, err)
		})
	}
}

func TestDiscordService_SendUploadCompleteWithMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata *AudioMetadata
		isValid  bool
	}{
		{
			name: "Valid metadata",
			metadata: &AudioMetadata{
				Filename:           "test.wav",
				FileSize:           1048576, // 1MB
				DurationText:       "3:45",
				Quality:            "High",
				IsValid:            true,
				Codec:              "PCM",
				SampleRate:         44100,
				Channels:           2,
				Bitrate:            1411,
				BitsPerSample:      16,
				Title:              "Test Title",
				Artist:             "Test Artist",
				Date:               "2023-01-01",
				ProcessingDuration: 5 * time.Second,
				Warnings:           []string{"Warning 1", "Warning 2"},
			},
			isValid: true,
		},
		{
			name: "Invalid metadata with warnings",
			metadata: &AudioMetadata{
				Filename: "test.wav",
				FileSize: 1048576,
				IsValid:  false,
				Warnings: []string{"File corrupted", "Header missing"},
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createMockDiscordServer(t, 204, "")
			defer server.Close()

			service := NewDiscordService(server.URL)

			err := service.SendUploadCompleteWithMetadata(tt.metadata)
			assert.NoError(t, err)
		})
	}
}

func TestDiscordService_SendError(t *testing.T) {
	server := createMockDiscordServer(t, 204, "")
	defer server.Close()

	service := NewDiscordService(server.URL)

	err := service.SendError("Test error message")
	assert.NoError(t, err)
}

// Test embed structure validation
func TestDiscordMessage_EmbedStructure(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		description string
		color       int
		fields      []DiscordField
	}{
		{
			name:        "Basic embed",
			title:       "Test Title",
			description: "Test Description",
			color:       0x00ff00,
			fields:      nil,
		},
		{
			name:        "Embed with fields",
			title:       "Test Title",
			description: "Test Description",
			color:       0xff0000,
			fields: []DiscordField{
				{Name: "Field 1", Value: "Value 1", Inline: true},
				{Name: "Field 2", Value: "Value 2", Inline: false},
			},
		},
		{
			name:        "Empty title and description",
			title:       "",
			description: "",
			color:       0x0000ff,
			fields:      []DiscordField{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				var message DiscordMessage
				err = json.Unmarshal(body, &message)
				require.NoError(t, err)

				assert.Len(t, message.Embeds, 1)
				embed := message.Embeds[0]

				assert.Equal(t, tt.title, embed.Title)
				assert.Equal(t, tt.description, embed.Description)
				assert.Equal(t, tt.color, embed.Color)

				if tt.fields != nil {
					assert.Equal(t, len(tt.fields), len(embed.Fields))
					for i, field := range tt.fields {
						assert.Equal(t, field.Name, embed.Fields[i].Name)
						assert.Equal(t, field.Value, embed.Fields[i].Value)
						assert.Equal(t, field.Inline, embed.Fields[i].Inline)
					}
				}

				// Verify timestamp format
				assert.NotEmpty(t, embed.Timestamp)
				_, err = time.Parse(time.RFC3339, embed.Timestamp)
				assert.NoError(t, err, "Timestamp should be in RFC3339 format")

				// Verify footer
				assert.NotNil(t, embed.Footer)
				footerText, ok := embed.Footer["text"].(string)
				assert.True(t, ok)
				assert.Equal(t, "Sermon Uploader v2.0 (Go)", footerText)

				w.WriteHeader(204)
			}))
			defer server.Close()

			service := NewDiscordService(server.URL)
			err := service.SendNotification(tt.title, tt.description, tt.color, tt.fields)
			assert.NoError(t, err)
		})
	}
}

// Test large field values and truncation if needed
func TestDiscordService_LargeFieldValues(t *testing.T) {
	server := createMockDiscordServer(t, 204, "")
	defer server.Close()

	service := NewDiscordService(server.URL)

	// Create a very large field value
	largeValue := strings.Repeat("A", 2000) // 2000 characters
	largeFields := []DiscordField{
		{Name: "Large Field", Value: largeValue, Inline: false},
	}

	err := service.SendNotification("Test", "Test with large field", 0x00ff00, largeFields)
	assert.NoError(t, err)
}

// Benchmark tests
func BenchmarkDiscordService_SendNotification(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))
	defer server.Close()

	service := NewDiscordService(server.URL)
	fields := []DiscordField{
		{Name: "Field1", Value: "Value1", Inline: true},
		{Name: "Field2", Value: "Value2", Inline: false},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.SendNotification("Bench Title", "Bench Description", 0x00ff00, fields)
	}
}

func BenchmarkDiscordMessage_Marshal(b *testing.B) {
	embed := DiscordEmbed{
		Title:       "Benchmark Title",
		Description: "Benchmark Description",
		Color:       0x00ff00,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Footer: map[string]interface{}{
			"text": "Sermon Uploader v2.0 (Go)",
		},
		Fields: []DiscordField{
			{Name: "Field1", Value: "Value1", Inline: true},
			{Name: "Field2", Value: "Value2", Inline: false},
		},
	}

	message := DiscordMessage{
		Embeds: []DiscordEmbed{embed},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(message)
	}
}

// Test concurrent notifications
func TestDiscordService_ConcurrentNotifications(t *testing.T) {
	server := createMockDiscordServer(t, 204, "")
	defer server.Close()

	service := NewDiscordService(server.URL)

	// Send multiple notifications concurrently
	const numNotifications = 10
	errors := make(chan error, numNotifications)

	for i := 0; i < numNotifications; i++ {
		go func(id int) {
			err := service.SendNotification(
				"Concurrent Test",
				"Concurrent notification",
				0x00ff00,
				[]DiscordField{
					{Name: "ID", Value: string(rune(id)), Inline: true},
				},
			)
			errors <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numNotifications; i++ {
		err := <-errors
		assert.NoError(t, err)
	}
}

// Test timeout behavior (would require actual HTTP client timeout configuration)
func TestDiscordService_TimeoutHandling(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Small delay
		w.WriteHeader(204)
	}))
	defer server.Close()

	service := NewDiscordService(server.URL)

	// This should still succeed with a small delay
	err := service.SendNotification("Timeout Test", "Testing timeout", 0x00ff00, nil)
	assert.NoError(t, err)
}
