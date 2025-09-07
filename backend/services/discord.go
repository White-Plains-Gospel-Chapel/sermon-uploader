package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"
	
	"sermon-uploader/config"
)

type DeploymentMessage struct {
	MessageID        string    `json:"message_id"`
	StartTime        time.Time `json:"start_time"`
	LastUpdate       time.Time `json:"last_update"`
	Status           string    `json:"status"`
	BackendVersion   string    `json:"backend_version"`
	FrontendVersion  string    `json:"frontend_version"`
	HealthCheckPassed bool     `json:"health_check_passed"`
}

type DiscordService struct {
	webhookURL     string
	webhookID      string
	webhookToken   string
	client         *http.Client
	mu             sync.RWMutex
	deploymentMsg  *DeploymentMessage
}

type DiscordEmbed struct {
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Color       int                    `json:"color"`
	Timestamp   string                 `json:"timestamp"`
	Footer      map[string]interface{} `json:"footer"`
	Fields      []DiscordField         `json:"fields,omitempty"`
}

type DiscordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type DiscordMessage struct {
	Embeds []DiscordEmbed `json:"embeds"`
}

func NewDiscordService(webhookURL string) *DiscordService {
	if webhookURL == "" {
		return &DiscordService{}
	}
	
	// Parse webhook URL to extract ID and token for message editing
	parts := strings.Split(strings.TrimSuffix(webhookURL, "/"), "/")
	webhookID := ""
	webhookToken := ""
	if len(parts) >= 2 {
		webhookToken = parts[len(parts)-1]
		webhookID = parts[len(parts)-2]
	}
	
	service := &DiscordService{
		webhookURL:   webhookURL,
		webhookID:    webhookID,
		webhookToken: webhookToken,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	
	// Load existing deployment message if available
	service.loadDeploymentMessage()
	
	return service
}

func (d *DiscordService) SendNotification(title, description string, color int, fields []DiscordField) error {
	if d.webhookURL == "" {
		return nil // Skip if no webhook URL configured
	}

	embed := DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Footer: map[string]interface{}{
			"text": fmt.Sprintf("Sermon Uploader v%s", config.GetFullVersion("backend")),
		},
		Fields: fields,
	}

	message := DiscordMessage{
		Embeds: []DiscordEmbed{embed},
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	resp, err := http.Post(d.webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func (d *DiscordService) SendStartupNotification(message string) error {
	fields := []DiscordField{
		{
			Name:   "Version",
			Value:  config.GetFullVersion("backend"),
			Inline: true,
		},
		{
			Name:   "Build Time",
			Value:  config.BuildTime,
			Inline: true,
		},
		{
			Name:   "Git Commit",
			Value:  config.GitCommit,
			Inline: true,
		},
	}
	
	return d.SendNotification(
		"🚀 Sermon Uploader Started",
		message,
		0x00ff00, // Green
		fields,
	)
}

func (d *DiscordService) SendUploadStart(fileCount int, isBatch bool) error {
	title := "📤 Upload Started"
	if isBatch {
		title = "📤 Batch Upload Started"
	}

	return d.SendNotification(
		title,
		fmt.Sprintf("Processing %d file(s)", fileCount),
		0x3498db, // Blue
		nil,
	)
}

func (d *DiscordService) SendUploadComplete(successful, duplicates, failed int, isBatch bool) error {
	var color int
	var status string

	if failed == 0 && duplicates >= 0 {
		color = 0x00ff00 // Green
		status = "✅ Success"
	} else if successful == 0 {
		color = 0xff0000 // Red
		status = "❌ Failed"
	} else {
		color = 0xffa500 // Orange
		status = "⚠️ Partial Success"
	}

	title := "Upload Complete"
	if isBatch {
		title = "Batch Upload Complete"
	}

	description := fmt.Sprintf("✅ %d uploaded", successful)
	if duplicates > 0 {
		description += fmt.Sprintf(", 🔄 %d duplicates", duplicates)
	}
	if failed > 0 {
		description += fmt.Sprintf(", ❌ %d failed", failed)
	}

	fields := []DiscordField{
		{Name: "Successful", Value: fmt.Sprintf("%d", successful), Inline: true},
	}

	if duplicates > 0 {
		fields = append(fields, DiscordField{Name: "Duplicates", Value: fmt.Sprintf("%d", duplicates), Inline: true})
	}

	if failed > 0 {
		fields = append(fields, DiscordField{Name: "Failed", Value: fmt.Sprintf("%d", failed), Inline: true})
	}

	return d.SendNotification(
		fmt.Sprintf("%s - %s", status, title),
		description,
		color,
		fields,
	)
}

// SendUploadCompleteWithMetadata sends an enhanced notification with audio metadata
func (d *DiscordService) SendUploadCompleteWithMetadata(metadata *AudioMetadata) error {
	var color int
	var status string

	if metadata.IsValid {
		color = 0x00ff00 // Green
		status = "✅ Upload Complete"
	} else {
		color = 0xffa500 // Orange
		status = "⚠️ Upload Complete (Issues Detected)"
	}

	// Build description with key audio info
	description := fmt.Sprintf("**%s** has been uploaded successfully", metadata.Filename)
	if metadata.DurationText != "" {
		description += fmt.Sprintf("\n🕒 Duration: %s", metadata.DurationText)
	}
	if metadata.Quality != "" {
		description += fmt.Sprintf("\n🎵 Quality: %s", metadata.Quality)
	}

	// Build detailed fields
	fields := []DiscordField{
		{Name: "File Size", Value: fmt.Sprintf("%.1f MB", float64(metadata.FileSize)/(1024*1024)), Inline: true},
	}

	if metadata.Codec != "" {
		fields = append(fields, DiscordField{Name: "Codec", Value: metadata.Codec, Inline: true})
	}

	if metadata.SampleRate > 0 {
		fields = append(fields, DiscordField{Name: "Sample Rate", Value: fmt.Sprintf("%d Hz", metadata.SampleRate), Inline: true})
	}

	if metadata.Channels > 0 {
		channelText := "Mono"
		if metadata.Channels == 2 {
			channelText = "Stereo"
		} else if metadata.Channels > 2 {
			channelText = fmt.Sprintf("%d Channels", metadata.Channels)
		}
		fields = append(fields, DiscordField{Name: "Channels", Value: channelText, Inline: true})
	}

	if metadata.Bitrate > 0 {
		fields = append(fields, DiscordField{Name: "Bitrate", Value: fmt.Sprintf("%d kbps", metadata.Bitrate), Inline: true})
	}

	if metadata.BitsPerSample > 0 {
		fields = append(fields, DiscordField{Name: "Bit Depth", Value: fmt.Sprintf("%d-bit", metadata.BitsPerSample), Inline: true})
	}

	// Add metadata tags if present
	if metadata.Title != "" {
		fields = append(fields, DiscordField{Name: "Title", Value: metadata.Title, Inline: false})
	}
	if metadata.Artist != "" {
		fields = append(fields, DiscordField{Name: "Artist", Value: metadata.Artist, Inline: true})
	}
	if metadata.Date != "" {
		fields = append(fields, DiscordField{Name: "Date", Value: metadata.Date, Inline: true})
	}

	// Add processing duration if available
	if metadata.ProcessingDuration > 0 {
		duration := metadata.ProcessingDuration
		var durationText string

		if duration < time.Second {
			durationText = fmt.Sprintf("%.0fms", float64(duration.Nanoseconds())/1e6)
		} else if duration < time.Minute {
			durationText = fmt.Sprintf("%.1fs", duration.Seconds())
		} else {
			durationText = fmt.Sprintf("%.1fm", duration.Minutes())
		}

		fields = append(fields, DiscordField{Name: "⚡ Processing Time", Value: durationText, Inline: true})
	}

	// Add warnings if any
	if len(metadata.Warnings) > 0 {
		warningText := strings.Join(metadata.Warnings, "\n")
		if len(warningText) > 1024 {
			warningText = warningText[:1021] + "..."
		}
		fields = append(fields, DiscordField{Name: "⚠️ Warnings", Value: warningText, Inline: false})
	}

	return d.SendNotification(status, description, color, fields)
}

func (d *DiscordService) SendError(message string) error {
	return d.SendNotification(
		"❌ Upload Error",
		message,
		0xff0000, // Red
		nil,
	)
}

// getESTTime returns current time in EST
func (d *DiscordService) getESTTime() time.Time {
	loc, _ := time.LoadLocation("America/New_York")
	return time.Now().In(loc)
}

// formatESTTime formats time for display
func (d *DiscordService) formatESTTime(t time.Time) string {
	loc, _ := time.LoadLocation("America/New_York")
	return t.In(loc).Format("3:04 PM EST")
}

// loadDeploymentMessage loads the deployment message from file if it exists
func (d *DiscordService) loadDeploymentMessage() {
	data, err := ioutil.ReadFile("/tmp/discord_deployment_message.json")
	if err != nil {
		return
	}
	
	var msg DeploymentMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}
	
	d.mu.Lock()
	d.deploymentMsg = &msg
	d.mu.Unlock()
}

// saveDeploymentMessage saves the deployment message to file
func (d *DiscordService) saveDeploymentMessage() {
	if d.deploymentMsg == nil {
		return
	}
	
	data, err := json.Marshal(d.deploymentMsg)
	if err != nil {
		return
	}
	
	ioutil.WriteFile("/tmp/discord_deployment_message.json", data, 0644)
}

// createMessage creates a new Discord message and returns its ID
func (d *DiscordService) createMessage(content string, embed interface{}) (string, error) {
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
func (d *DiscordService) updateMessage(messageID string, content string, embed interface{}) error {
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

// calculateUptime calculates uptime since deployment start
func (d *DiscordService) calculateUptime() string {
	d.mu.RLock()
	msg := d.deploymentMsg
	d.mu.RUnlock()

	if msg == nil {
		return "Unknown"
	}

	duration := time.Since(msg.StartTime)
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// StartDeploymentNotification creates a live-updating deployment message
func (d *DiscordService) StartDeploymentNotification() error {
	estTime := d.getESTTime()

	embed := map[string]interface{}{
		"title":       "🎯 Sermon Uploader Status - Live",
		"description": "━━━━━━━━━━━━━━━━━━━━━━━━━",
		"color":       0xffaa00, // Orange for in-progress
		"fields": []map[string]interface{}{
			{
				"name":   "🚀 Started",
				"value":  d.formatESTTime(estTime),
				"inline": true,
			},
			{
				"name":   "🔄 Status",
				"value":  "⏳ Initializing...",
				"inline": true,
			},
			{
				"name":   "Current Status",
				"value":  "🔄 STARTING",
				"inline": false,
			},
		},
		"footer": map[string]string{
			"text": fmt.Sprintf("🔄 Last Check: %s", d.formatESTTime(estTime)),
		},
		"timestamp": estTime.Format(time.RFC3339),
	}

	messageID, err := d.createMessage("", embed)
	if err != nil {
		return err
	}

	// Store the message for future updates
	d.mu.Lock()
	d.deploymentMsg = &DeploymentMessage{
		MessageID:   messageID,
		StartTime:   estTime,
		LastUpdate:  estTime,
		Status:      "starting",
	}
	d.mu.Unlock()

	d.saveDeploymentMessage()
	return nil
}

// UpdateDeploymentStatus updates the live deployment message
func (d *DiscordService) UpdateDeploymentStatus(status string, backendVersion, frontendVersion string, healthPassed bool) error {
	d.mu.Lock()
	if d.deploymentMsg == nil {
		d.mu.Unlock()
		// No existing message, create one
		if err := d.StartDeploymentNotification(); err != nil {
			return err
		}
		d.mu.Lock()
	}
	
	estTime := d.getESTTime()
	d.deploymentMsg.LastUpdate = estTime
	d.deploymentMsg.Status = status
	d.deploymentMsg.BackendVersion = backendVersion
	d.deploymentMsg.FrontendVersion = frontendVersion
	d.deploymentMsg.HealthCheckPassed = healthPassed
	
	messageID := d.deploymentMsg.MessageID
	startTime := d.deploymentMsg.StartTime
	d.mu.Unlock()

	// Determine colors and status emoji
	color := 0xffaa00 // Orange for in-progress
	statusEmoji := "🔄"
	currentStatus := "STARTING"
	
	switch status {
	case "deployed":
		if healthPassed {
			color = 0x00ff00 // Green for success
			statusEmoji = "✅"
			currentStatus = "HEALTHY"
		} else {
			color = 0xffa500 // Orange for partial
			statusEmoji = "⚠️"
			currentStatus = "DEPLOYED"
		}
	case "failed":
		color = 0xff0000 // Red for failure
		statusEmoji = "❌"
		currentStatus = "FAILED"
	case "verified":
		color = 0x00ff00 // Green for full success
		statusEmoji = "✅"
		currentStatus = "HEALTHY"
	}

	fields := []map[string]interface{}{
		{
			"name":   "🚀 Started",
			"value":  d.formatESTTime(startTime),
			"inline": true,
		},
		{
			"name":   "Uptime",
			"value":  d.calculateUptime(),
			"inline": true,
		},
	}

	// Add deployment info if available
	if status == "deployed" || status == "verified" {
		fields = append(fields, map[string]interface{}{
			"name":   "🔄 Deployed",
			"value":  d.formatESTTime(estTime),
			"inline": true,
		})
		
		if status == "verified" {
			fields = append(fields, map[string]interface{}{
				"name":   "✅ Verified",
				"value":  d.formatESTTime(estTime),
				"inline": true,
			})
		}
	}

	// Add version info
	if backendVersion != "" {
		fields = append(fields, map[string]interface{}{
			"name":   "Version",
			"value":  backendVersion,
			"inline": true,
		})
	}

	// Add current status
	fields = append(fields, map[string]interface{}{
		"name":   "Current Status",
		"value":  fmt.Sprintf("%s %s", statusEmoji, currentStatus),
		"inline": false,
	})

	embed := map[string]interface{}{
		"title":       "🎯 Sermon Uploader Status - Live",
		"description": "━━━━━━━━━━━━━━━━━━━━━━━━━",
		"color":       color,
		"fields":      fields,
		"footer": map[string]string{
			"text": fmt.Sprintf("🔄 Last Check: %s", d.formatESTTime(estTime)),
		},
		"timestamp": estTime.Format(time.RFC3339),
	}

	d.saveDeploymentMessage()
	return d.updateMessage(messageID, "", embed)
}

// SendDeploymentNotification sends a notification after successful deployment (backward compatibility)
func (d *DiscordService) SendDeploymentNotification(success bool, frontendVersion, backendVersion string) error {
	// Use the new live update system
	if success {
		return d.UpdateDeploymentStatus("verified", backendVersion, frontendVersion, true)
	} else {
		return d.UpdateDeploymentStatus("failed", backendVersion, frontendVersion, false)
	}
}
