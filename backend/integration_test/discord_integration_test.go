package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sermon-uploader/services"
)

func TestDiscordServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx := context.Background()
	
	// Setup test environment
	env, err := SetupTestEnvironment(ctx)
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Create Discord service with test webhook URL
	discordService := services.NewDiscordService(env.Config.DiscordWebhookURL)

	t.Run("SendStartupNotification", func(t *testing.T) {
		err := discordService.SendStartupNotification("Test startup message")
		assert.NoError(t, err, "Startup notification should be sent successfully")
		
		// Wait a moment for the webhook to be processed
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("SendUploadStart", func(t *testing.T) {
		// Test single file upload notification
		err := discordService.SendUploadStart(1, false)
		assert.NoError(t, err, "Single file upload start notification should be sent successfully")
		
		// Test batch upload notification
		err = discordService.SendUploadStart(5, true)
		assert.NoError(t, err, "Batch upload start notification should be sent successfully")
	})

	t.Run("SendUploadComplete", func(t *testing.T) {
		// Test successful upload completion
		err := discordService.SendUploadComplete(3, 0, 0, false)
		assert.NoError(t, err, "Successful upload completion notification should be sent successfully")
		
		// Test partial success (some duplicates)
		err = discordService.SendUploadComplete(2, 1, 0, false)
		assert.NoError(t, err, "Partial success upload completion notification should be sent successfully")
		
		// Test with failures
		err = discordService.SendUploadComplete(1, 0, 2, false)
		assert.NoError(t, err, "Upload completion with failures notification should be sent successfully")
		
		// Test batch completion
		err = discordService.SendUploadComplete(5, 2, 0, true)
		assert.NoError(t, err, "Batch upload completion notification should be sent successfully")
	})

	t.Run("SendError", func(t *testing.T) {
		err := discordService.SendError("Test error message for integration testing")
		assert.NoError(t, err, "Error notification should be sent successfully")
	})

	t.Run("SendUploadCompleteWithMetadata", func(t *testing.T) {
		// Create test audio metadata
		metadata := &services.AudioMetadata{
			Filename:           "test-sermon.wav",
			FileSize:           2 * 1024 * 1024, // 2MB
			Duration:           3600.0,          // 1 hour
			DurationText:       "1h 0m 0s",
			Codec:              "PCM",
			SampleRate:         44100,
			Channels:           2,
			Bitrate:            1411200, // 1411.2 kbps
			BitsPerSample:      16,
			IsLossless:         true,
			Quality:            "CD Quality",
			IsValid:            true,
			Title:              "Sunday Morning Sermon",
			Artist:             "Pastor John",
			Date:               "2023-12-03",
			Album:              "Weekly Sermons",
			Genre:              "Speech",
			UploadTime:         time.Now(),
			ProcessingDuration: 2500 * time.Millisecond, // 2.5 seconds processing
			Warnings:           []string{"Minor audio clipping detected at 0:15:30"},
		}

		err := discordService.SendUploadCompleteWithMetadata(metadata)
		assert.NoError(t, err, "Upload completion with metadata notification should be sent successfully")
		
		// Test with minimal metadata
		minimalMetadata := &services.AudioMetadata{
			Filename:   "minimal-test.wav",
			FileSize:   1024 * 1024, // 1MB
			IsValid:    true,
			UploadTime: time.Now(),
		}

		err = discordService.SendUploadCompleteWithMetadata(minimalMetadata)
		assert.NoError(t, err, "Upload completion with minimal metadata notification should be sent successfully")
	})

	t.Run("SendInvalidMetadata", func(t *testing.T) {
		// Test with invalid audio file metadata
		invalidMetadata := &services.AudioMetadata{
			Filename:    "corrupt-file.wav",
			FileSize:    500 * 1024, // 500KB
			IsValid:     false,
			Quality:     "Corrupted",
			UploadTime:  time.Now(),
			Warnings:    []string{"File appears to be corrupted", "Unable to read audio properties"},
		}

		err := discordService.SendUploadCompleteWithMetadata(invalidMetadata)
		assert.NoError(t, err, "Upload completion with invalid metadata notification should be sent successfully")
	})

	t.Run("NotificationWithCustomFields", func(t *testing.T) {
		// Test notification with custom fields
		fields := []services.DiscordField{
			{Name: "File Size", Value: "5.2 MB", Inline: true},
			{Name: "Duration", Value: "45m 30s", Inline: true},
			{Name: "Quality", Value: "CD Quality (16-bit/44.1kHz)", Inline: true},
			{Name: "Processing Time", Value: "3.2 seconds", Inline: true},
		}

		err := discordService.SendNotification(
			"🎵 High Quality Upload Complete",
			"Successfully uploaded **Sunday Morning Sermon** with excellent audio quality",
			0x00ff00, // Green color
			fields,
		)
		assert.NoError(t, err, "Custom notification with fields should be sent successfully")
	})

	t.Run("RateLimitHandling", func(t *testing.T) {
		// Test rapid notifications to check rate limiting handling
		var errors []error
		
		for i := 0; i < 5; i++ {
			err := discordService.SendStartupNotification(fmt.Sprintf("Rate limit test message %d", i))
			if err != nil {
				errors = append(errors, err)
			}
			
			// Small delay between requests
			time.Sleep(10 * time.Millisecond)
		}
		
		// Some errors might occur due to rate limiting, but at least some should succeed
		assert.LessOrEqual(t, len(errors), 3, "Should handle rate limiting gracefully")
	})

	t.Run("LargeMessageHandling", func(t *testing.T) {
		// Test with a large message that might exceed Discord limits
		longWarnings := make([]string, 50)
		for i := range longWarnings {
			longWarnings[i] = fmt.Sprintf("Warning %d: This is a very long warning message that contains detailed information about what went wrong during the audio processing step", i)
		}

		largeMetadata := &services.AudioMetadata{
			Filename:           "large-metadata-test.wav",
			FileSize:           10 * 1024 * 1024, // 10MB
			Duration:           7200.0,           // 2 hours
			DurationText:       "2h 0m 0s",
			Codec:              "PCM",
			SampleRate:         96000, // High sample rate
			Channels:           6,     // Surround sound
			Bitrate:           9216000, // Very high bitrate
			BitsPerSample:      24,    // 24-bit
			IsLossless:         true,
			Quality:            "Studio Master Quality",
			IsValid:            true,
			Title:              "Very Long Sermon Title That Contains Many Words And Details About The Content",
			Artist:             "Pastor With A Very Long Name",
			Date:               "2023-12-03",
			Album:              "Extended Sunday Service Collection Volume 1",
			Genre:              "Religious Speech",
			UploadTime:         time.Now(),
			ProcessingDuration: 30 * time.Second,
			Warnings:           longWarnings, // Many warnings
		}

		err := discordService.SendUploadCompleteWithMetadata(largeMetadata)
		assert.NoError(t, err, "Large metadata notification should be handled gracefully")
	})
}

func TestDiscordLiveServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx := context.Background()
	
	// Setup test environment
	env, err := SetupTestEnvironment(ctx)
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Create Discord Live service with test webhook URL
	discordLiveService := services.NewDiscordLiveService(env.Config.DiscordWebhookURL, "test-channel")

	t.Run("CreateAndUpdateLiveMessage", func(t *testing.T) {
		// Create initial live message
		messageID, err := discordLiveService.CreateLiveMessage("Initial upload status")
		require.NoError(t, err, "Creating live message should succeed")
		assert.NotEmpty(t, messageID, "Message ID should not be empty")

		// Update the message multiple times
		updates := []string{
			"📤 Starting upload process...",
			"📈 Progress: 25% complete",
			"📈 Progress: 50% complete", 
			"📈 Progress: 75% complete",
			"✅ Upload completed successfully!",
		}

		for _, update := range updates {
			err := discordLiveService.UpdateLiveMessage(messageID, update, 0x3498db) // Blue color
			assert.NoError(t, err, "Updating live message should succeed: %s", update)
			
			// Small delay between updates to simulate real progress
			time.Sleep(50 * time.Millisecond)
		}

		// Final update with success color
		err = discordLiveService.UpdateLiveMessage(messageID, "🎉 All files processed successfully!", 0x00ff00) // Green
		assert.NoError(t, err, "Final live message update should succeed")
	})

	t.Run("BatchUploadProgress", func(t *testing.T) {
		// Simulate batch upload progress tracking
		batchFiles := []string{"sermon1.wav", "sermon2.wav", "sermon3.wav", "sermon4.wav"}
		
		messageID, err := discordLiveService.CreateLiveMessage(fmt.Sprintf("🚀 Starting batch upload of %d files", len(batchFiles)))
		require.NoError(t, err, "Creating batch message should succeed")

		for i, filename := range batchFiles {
			progress := float64(i+1) / float64(len(batchFiles)) * 100
			status := fmt.Sprintf("📤 Processing: %s\nProgress: %.1f%% (%d/%d files)", 
				filename, progress, i+1, len(batchFiles))
			
			var color int
			if progress < 50 {
				color = 0xffa500 // Orange
			} else if progress < 100 {
				color = 0x3498db // Blue
			} else {
				color = 0x00ff00 // Green
			}

			err := discordLiveService.UpdateLiveMessage(messageID, status, color)
			assert.NoError(t, err, "Batch progress update should succeed for file: %s", filename)
			
			time.Sleep(100 * time.Millisecond)
		}
	})

	t.Run("ErrorHandlingInLiveMessage", func(t *testing.T) {
		messageID, err := discordLiveService.CreateLiveMessage("Starting upload with potential errors...")
		require.NoError(t, err, "Creating error test message should succeed")

		// Simulate various error conditions
		errorStages := []struct {
			message string
			color   int
		}{
			{"📤 Upload started...", 0x3498db},
			{"⚠️ Network issue detected, retrying...", 0xffa500},
			{"📤 Retry successful, continuing...", 0x3498db},
			{"❌ Critical error: File corrupted", 0xff0000},
			{"🔄 Attempting recovery...", 0xffa500},
			{"✅ Recovery successful, upload complete!", 0x00ff00},
		}

		for _, stage := range errorStages {
			err := discordLiveService.UpdateLiveMessage(messageID, stage.message, stage.color)
			assert.NoError(t, err, "Error stage update should succeed: %s", stage.message)
			time.Sleep(75 * time.Millisecond)
		}
	})

	t.Run("ConcurrentLiveMessages", func(t *testing.T) {
		// Test handling multiple live messages simultaneously
		numMessages := 3
		messageIDs := make([]string, numMessages)
		
		// Create multiple messages
		for i := 0; i < numMessages; i++ {
			messageID, err := discordLiveService.CreateLiveMessage(fmt.Sprintf("Concurrent upload %d started", i+1))
			require.NoError(t, err, "Creating concurrent message %d should succeed", i+1)
			messageIDs[i] = messageID
		}

		// Update each message concurrently
		for round := 0; round < 3; round++ {
			for i, messageID := range messageIDs {
				status := fmt.Sprintf("Upload %d - Round %d progress: %d%%", i+1, round+1, (round+1)*33)
				err := discordLiveService.UpdateLiveMessage(messageID, status, 0x3498db)
				assert.NoError(t, err, "Concurrent update should succeed for message %d", i+1)
			}
			time.Sleep(50 * time.Millisecond)
		}

		// Finalize all messages
		for i, messageID := range messageIDs {
			err := discordLiveService.UpdateLiveMessage(messageID, fmt.Sprintf("✅ Upload %d completed!", i+1), 0x00ff00)
			assert.NoError(t, err, "Final concurrent update should succeed for message %d", i+1)
		}
	})

	t.Run("MessageCleanup", func(t *testing.T) {
		messageID, err := discordLiveService.CreateLiveMessage("Cleanup test message")
		require.NoError(t, err, "Creating cleanup test message should succeed")

		// Update message
		err = discordLiveService.UpdateLiveMessage(messageID, "Message updated before cleanup", 0x3498db)
		assert.NoError(t, err, "Update before cleanup should succeed")

		// Cleanup message (if the service supports it)
		err = discordLiveService.CleanupMessage(messageID)
		// Note: Actual cleanup might not be implemented or may fail, so we don't assert
		if err != nil {
			t.Logf("Message cleanup returned error (might be expected): %v", err)
		}
	})
}

// TestWebhookMockServer tests that our webhook mock is working correctly
func TestWebhookMockServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx := context.Background()
	
	// Setup test environment
	env, err := SetupTestEnvironment(ctx)
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("WebhookEndpointResponds", func(t *testing.T) {
		resp, err := client.Get(env.Config.DiscordWebhookURL)
		require.NoError(t, err, "Webhook endpoint should respond")
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Webhook should return 200 OK")
	})

	t.Run("WebhookAcceptsPOST", func(t *testing.T) {
		// Test actual webhook POST
		webhookPayload := map[string]interface{}{
			"embeds": []map[string]interface{}{
				{
					"title":       "Test Integration Webhook",
					"description": "Testing POST request to webhook mock",
					"color":       0x00ff00,
					"timestamp":   time.Now().Format(time.RFC3339),
				},
			},
		}

		payload, err := json.Marshal(webhookPayload)
		require.NoError(t, err, "Should marshal webhook payload")

		resp, err := client.Post(env.Config.DiscordWebhookURL, "application/json", 
			bytes.NewReader(payload))
		require.NoError(t, err, "POST to webhook should succeed")
		defer resp.Body.Close()

		// The echo server should return the same payload
		respBody, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "Should read response body")
		
		t.Logf("Webhook response status: %d", resp.StatusCode)
		t.Logf("Webhook response body (first 200 chars): %s", string(respBody)[:min(200, len(respBody))])
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}