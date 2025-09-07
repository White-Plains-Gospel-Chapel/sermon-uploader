package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// MessageType represents different types of Discord messages
type MessageType string

const (
	MessageTypeServer MessageType = "server"
	MessageTypeUpload MessageType = "upload"
	MessageTypeError  MessageType = "error"
	MessageTypeAdmin  MessageType = "admin"
)

// LiveMessage tracks a Discord message that can be updated
type LiveMessage struct {
	ID        string
	Type      MessageType
	CreatedAt time.Time
	UpdatedAt time.Time
	Data      interface{}
}

// DiscordLiveService provides Discord notifications with live update capability
type DiscordLiveService struct {
	webhookURL     string
	webhookID      string
	webhookToken   string
	activeMessages map[string]*LiveMessage
	mu             sync.RWMutex
	client         *http.Client
}

// NewDiscordLiveService creates a new Discord service with live update support
func NewDiscordLiveService(webhookURL string) *DiscordLiveService {
	if webhookURL == "" {
		return &DiscordLiveService{}
	}

	// Parse webhook URL to extract ID and token
	parts := strings.Split(strings.TrimSuffix(webhookURL, "/"), "/")
	webhookID := ""
	webhookToken := ""
	if len(parts) >= 2 {
		webhookToken = parts[len(parts)-1]
		webhookID = parts[len(parts)-2]
	}

	return &DiscordLiveService{
		webhookURL:     webhookURL,
		webhookID:      webhookID,
		webhookToken:   webhookToken,
		activeMessages: make(map[string]*LiveMessage),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// getESTTime returns current time in EST
func (d *DiscordLiveService) getESTTime() time.Time {
	loc, _ := time.LoadLocation("America/New_York")
	return time.Now().In(loc)
}

// formatESTTime formats time for display
func (d *DiscordLiveService) formatESTTime(t time.Time) string {
	loc, _ := time.LoadLocation("America/New_York")
	return t.In(loc).Format("3:04 PM EST")
}

// createMessage creates a new Discord message and returns its ID
func (d *DiscordLiveService) createMessage(content string, embed interface{}) (string, error) {
	if d.webhookURL == "" {
		return "", nil
	}

	payload := map[string]interface{}{}
	if content != "" {
		payload["content"] = content
	}
	if embed != nil {
		payload["embeds"] = []interface{}{embed}
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	// Send with ?wait=true to get message details back
	resp, err := d.client.Post(d.webhookURL+"?wait=true", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if id, ok := result["id"].(string); ok {
		return id, nil
	}

	return "", fmt.Errorf("no message ID in response")
}

// updateMessage updates an existing Discord message
func (d *DiscordLiveService) updateMessage(messageID string, content string, embed interface{}) error {
	if d.webhookURL == "" || messageID == "" {
		return nil
	}

	payload := map[string]interface{}{}
	if content != "" {
		payload["content"] = content
	}
	if embed != nil {
		payload["embeds"] = []interface{}{embed}
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	editURL := fmt.Sprintf("https://discord.com/api/webhooks/%s/%s/messages/%s",
		d.webhookID, d.webhookToken, messageID)

	req, err := http.NewRequest("PATCH", editURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update message: status %d", resp.StatusCode)
	}

	return nil
}

// SendServerStartup sends a live-updating server startup notification
func (d *DiscordLiveService) SendServerStartup() (string, error) {
	estTime := d.getESTTime()

	embed := map[string]interface{}{
		"title":       "🚀 Server Starting",
		"description": "Sermon Uploader is initializing...",
		"color":       0xffaa00, // Orange for in-progress
		"fields": []map[string]interface{}{
			{
				"name":   "Status",
				"value":  "⏳ Initializing services...",
				"inline": false,
			},
			{
				"name":   "Start Time",
				"value":  d.formatESTTime(estTime),
				"inline": true,
			},
			{
				"name":   "Environment",
				"value":  "Production",
				"inline": true,
			},
		},
		"footer": map[string]string{
			"text": fmt.Sprintf("Server Monitor • %s", d.formatESTTime(estTime)),
		},
		"timestamp": estTime.Format(time.RFC3339),
	}

	messageID, err := d.createMessage("", embed)
	if err != nil {
		return "", err
	}

	// Store the message for future updates
	d.mu.Lock()
	d.activeMessages[messageID] = &LiveMessage{
		ID:        messageID,
		Type:      MessageTypeServer,
		CreatedAt: estTime,
		UpdatedAt: estTime,
	}
	d.mu.Unlock()

	return messageID, nil
}

// UpdateServerStatus updates the server startup message with current status
func (d *DiscordLiveService) UpdateServerStatus(messageID string, status string, isReady bool) error {
	estTime := d.getESTTime()

	color := 0xffaa00 // Orange for in-progress
	statusEmoji := "⏳"
	if isReady {
		color = 0x00ff00 // Green for ready
		statusEmoji = "✅"
	}

	embed := map[string]interface{}{
		"title":       "🚀 Server Status",
		"description": "Sermon Uploader service status",
		"color":       color,
		"fields": []map[string]interface{}{
			{
				"name":   "Status",
				"value":  fmt.Sprintf("%s %s", statusEmoji, status),
				"inline": false,
			},
			{
				"name":   "Start Time",
				"value":  d.formatESTTime(d.getESTTime()),
				"inline": true,
			},
			{
				"name":   "Uptime",
				"value":  d.calculateUptime(messageID),
				"inline": true,
			},
		},
		"footer": map[string]string{
			"text": fmt.Sprintf("Server Monitor • %s", d.formatESTTime(estTime)),
		},
		"timestamp": estTime.Format(time.RFC3339),
	}

	return d.updateMessage(messageID, "", embed)
}

// calculateUptime calculates uptime for a message
func (d *DiscordLiveService) calculateUptime(messageID string) string {
	d.mu.RLock()
	msg, exists := d.activeMessages[messageID]
	d.mu.RUnlock()

	if !exists {
		return "Unknown"
	}

	duration := time.Since(msg.CreatedAt)
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// SendUploadProgress creates/updates an upload progress message
func (d *DiscordLiveService) SendUploadProgress(files []string) (string, error) {
	estTime := d.getESTTime()

	// Build file list
	fileFields := make([]map[string]interface{}, 0)
	for _, file := range files {
		fileFields = append(fileFields, map[string]interface{}{
			"name":   fmt.Sprintf("📄 %s", file),
			"value":  "░░░░░░░░░░ Detected",
			"inline": false,
		})
	}

	// Add summary fields
	fileFields = append(fileFields,
		map[string]interface{}{
			"name":   "📊 Total Files",
			"value":  fmt.Sprintf("%d", len(files)),
			"inline": true,
		},
		map[string]interface{}{
			"name":   "⏰ Started",
			"value":  d.formatESTTime(estTime),
			"inline": true,
		},
	)

	title := "📤 Upload Started"
	if len(files) > 1 {
		title = fmt.Sprintf("📤 Batch Upload (%d files)", len(files))
	}

	embed := map[string]interface{}{
		"title":       title,
		"description": "Processing sermon files...",
		"color":       0xffaa00, // Orange
		"fields":      fileFields,
		"footer": map[string]string{
			"text": fmt.Sprintf("Upload Monitor • %s", d.formatESTTime(estTime)),
		},
		"timestamp": estTime.Format(time.RFC3339),
	}

	return d.createMessage("", embed)
}

// UpdateUploadProgress updates the upload progress for specific files
func (d *DiscordLiveService) UpdateUploadProgress(messageID string, fileProgress map[string]int) error {
	estTime := d.getESTTime()

	// Calculate overall progress
	totalProgress := 0
	for _, progress := range fileProgress {
		totalProgress += progress
	}
	avgProgress := 0
	if len(fileProgress) > 0 {
		avgProgress = totalProgress / len(fileProgress)
	}

	// Build file fields with progress bars
	fileFields := make([]map[string]interface{}, 0)
	for file, progress := range fileProgress {
		bar := d.generateProgressBar(progress)
		status := "Uploading..."
		if progress >= 100 {
			status = "Complete ✅"
		}

		fileFields = append(fileFields, map[string]interface{}{
			"name":   fmt.Sprintf("📄 %s", file),
			"value":  fmt.Sprintf("%s %d%% - %s", bar, progress, status),
			"inline": false,
		})
	}

	// Add summary
	fileFields = append(fileFields,
		map[string]interface{}{
			"name":   "📊 Overall Progress",
			"value":  fmt.Sprintf("%d%%", avgProgress),
			"inline": true,
		},
		map[string]interface{}{
			"name":   "⏱️ Elapsed",
			"value":  d.calculateUptime(messageID),
			"inline": true,
		},
	)

	color := 0xffaa00 // Orange
	if avgProgress >= 100 {
		color = 0x00ff00 // Green
	}

	title := "📤 Upload Progress"
	if avgProgress >= 100 {
		title = "✅ Upload Complete"
	}

	embed := map[string]interface{}{
		"title":       title,
		"description": fmt.Sprintf("Processing %d sermon file(s)", len(fileProgress)),
		"color":       color,
		"fields":      fileFields,
		"footer": map[string]string{
			"text": fmt.Sprintf("Upload Monitor • %s", d.formatESTTime(estTime)),
		},
		"timestamp": estTime.Format(time.RFC3339),
	}

	return d.updateMessage(messageID, "", embed)
}

// generateProgressBar creates a visual progress bar
func (d *DiscordLiveService) generateProgressBar(percent int) string {
	filled := percent / 10
	empty := 10 - filled

	if filled > 10 {
		filled = 10
		empty = 0
	}

	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

// SendError sends an error notification (these don't update live)
func (d *DiscordLiveService) SendError(title, message string) error {
	estTime := d.getESTTime()

	embed := map[string]interface{}{
		"title":       fmt.Sprintf("❌ %s", title),
		"description": message,
		"color":       0xff0000, // Red
		"footer": map[string]string{
			"text": fmt.Sprintf("Error Report • %s", d.formatESTTime(estTime)),
		},
		"timestamp": estTime.Format(time.RFC3339),
	}

	_, err := d.createMessage("", embed)
	return err
}

// CleanupOldMessages removes old tracked messages
func (d *DiscordLiveService) CleanupOldMessages(hours int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)

	for id, msg := range d.activeMessages {
		if msg.CreatedAt.Before(cutoff) {
			delete(d.activeMessages, id)
		}
	}
}

// Production logger interface compatibility methods
func (d *DiscordLiveService) CreateMessage(content string) (string, error) {
	return d.createMessage(content, nil)
}

func (d *DiscordLiveService) UpdateMessage(messageID, content string) error {
	return d.updateMessage(messageID, content, nil)
}

// Backward compatibility methods

// SendNotification sends a simple notification (backward compatibility)
func (d *DiscordLiveService) SendNotification(title, description string, color int, fields []DiscordField) error {
	estTime := d.getESTTime()

	fieldMaps := make([]map[string]interface{}, 0)
	for _, f := range fields {
		fieldMaps = append(fieldMaps, map[string]interface{}{
			"name":   f.Name,
			"value":  f.Value,
			"inline": f.Inline,
		})
	}

	embed := map[string]interface{}{
		"title":       title,
		"description": description,
		"color":       color,
		"fields":      fieldMaps,
		"footer": map[string]string{
			"text": fmt.Sprintf("Sermon Processor • %s", d.formatESTTime(estTime)),
		},
		"timestamp": estTime.Format(time.RFC3339),
	}

	_, err := d.createMessage("", embed)
	return err
}

// SendStartupNotification for backward compatibility
func (d *DiscordLiveService) SendStartupNotification(message string) error {
	messageID, err := d.SendServerStartup()
	if err != nil {
		return err
	}

	// Update to ready status after creation
	time.Sleep(1 * time.Second)
	return d.UpdateServerStatus(messageID, message, true)
}
