package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	
	"sermon-uploader/config"
)

type DiscordService struct {
	webhookURL string
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
	return &DiscordService{
		webhookURL: webhookURL,
	}
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

// SendDeploymentNotification sends a notification after successful deployment
func (d *DiscordService) SendDeploymentNotification(success bool, frontendVersion, backendVersion string) error {
	var title, description string
	var color int
	
	if success {
		title = "✅ Deployment Successful"
		description = "New version deployed and verified"
		color = 0x00ff00 // Green
	} else {
		title = "❌ Deployment Failed"
		description = "Deployment verification failed"
		color = 0xff0000 // Red
	}
	
	fields := []DiscordField{
		{
			Name:   "Backend Version",
			Value:  backendVersion,
			Inline: true,
		},
		{
			Name:   "Frontend Version",
			Value:  frontendVersion,
			Inline: true,
		},
		{
			Name:   "Deployed At",
			Value:  time.Now().Format("2006-01-02 15:04:05 MST"),
			Inline: false,
		},
	}
	
	if success {
		fields = append(fields, DiscordField{
			Name:   "Health Check",
			Value:  "✅ Passed",
			Inline: true,
		})
		fields = append(fields, DiscordField{
			Name:   "Version Match",
			Value:  "✅ Verified",
			Inline: true,
		})
	}
	
	return d.SendNotification(title, description, color, fields)
}
