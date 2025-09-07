package services

import (
	"fmt"
	"strings"
	"time"
)

// initializeDiscordMessage creates the initial system monitoring Discord message
func (s *SystemResourceMonitor) initializeDiscordMessage() error {
	if s.discordService == nil {
		return fmt.Errorf("Discord service not available")
	}

	content := s.buildSystemDiscordMessage()
	messageID, err := s.discordService.CreateMessage(content)
	if err != nil {
		return fmt.Errorf("failed to create Discord system monitoring message: %w", err)
	}

	s.messageID = messageID
	return nil
}

// updateDiscordMessage updates the Discord system monitoring message
func (s *SystemResourceMonitor) updateDiscordMessage() {
	if s.discordService == nil || s.messageID == "" {
		return
	}

	content := s.buildSystemDiscordMessage()
	if err := s.discordService.UpdateMessage(s.messageID, content); err != nil {
		s.logger.Warn("Failed to update Discord system monitoring message",
			"error", err.Error())
	}
}

// buildSystemDiscordMessage creates the formatted Discord message content
func (s *SystemResourceMonitor) buildSystemDiscordMessage() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	est := time.Now().In(getSystemMonitorESTLocation())
	sessionDuration := time.Since(s.sessionStart)

	var builder strings.Builder

	// Header
	builder.WriteString("🖥️ **Raspberry Pi 5 - System Monitor**\n")
	builder.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	builder.WriteString(fmt.Sprintf("📅 Session Started: %s\n", 
		s.sessionStart.In(getSystemMonitorESTLocation()).Format("3:04 PM EST")))
	builder.WriteString(fmt.Sprintf("⏱️ Runtime: %s\n", formatSystemDuration(sessionDuration)))
	builder.WriteString(fmt.Sprintf("🔄 Last Updated: %s\n\n", est.Format("3:04:05 PM EST")))

	// Simplified resource status - only what sermon-uploader uses
	builder.WriteString("**📊 Resource Usage (Sermon Uploader)**\n")
	
	// CPU (used for HTTP processing, file handling)
	cpuColor := getResourceStatusIcon(s.cpuUsage.UsagePercent, 70, 90)
	builder.WriteString(fmt.Sprintf("├─ %s CPU: %.1f%% | %d goroutines | Load: %.1f\n", 
		cpuColor, s.cpuUsage.UsagePercent, s.cpuUsage.GoRoutines, s.cpuUsage.LoadAvg1))
	
	// Memory (used for file uploads, streaming, Go runtime)
	memColor := getResourceStatusIcon(s.memoryUsage.UsagePercent, 60, 80)
	builder.WriteString(fmt.Sprintf("├─ %s Memory: %.1f%% (%.1f/%.1f GB) | Go: %.0fMB\n", 
		memColor, s.memoryUsage.UsagePercent, 
		s.memoryUsage.UsedMB/1024, s.memoryUsage.TotalMB/1024, s.memoryUsage.GoAllocMB))
	
	// Temperature (important for Pi 5 during file processing)
	tempColor := getThermalStatusIcon(s.thermalMetrics.CPUTempC)
	throttleIcon := ""
	if s.thermalMetrics.IsThrottling {
		throttleIcon = " [THROTTLING]"
	}
	builder.WriteString(fmt.Sprintf("├─ %s Temperature: %.1f°C%s\n", 
		tempColor, s.thermalMetrics.CPUTempC, throttleIcon))
	
	// Disk (used for MinIO storage, logs)
	diskColor := getResourceStatusIcon(s.diskMetrics.UsagePercent, 70, 90)
	builder.WriteString(fmt.Sprintf("├─ %s Disk: %.1f GB free (%.0f%% used)\n", 
		diskColor, s.diskMetrics.FreeGB, s.diskMetrics.UsagePercent))
	
	// Network (used for uploads, Discord webhooks, MinIO)
	if s.networkMetrics.Interface != "" {
		netIcon := "🟢"
		if !s.networkMetrics.IsUp {
			netIcon = "🔴"
		}
		networkErrors := ""
		if s.networkMetrics.ErrorsRx > 0 || s.networkMetrics.ErrorsTx > 0 {
			networkErrors = fmt.Sprintf(" [%d errors]", s.networkMetrics.ErrorsRx+s.networkMetrics.ErrorsTx)
		}
		builder.WriteString(fmt.Sprintf("└─ %s Network: %s (%s)%s\n\n", 
			netIcon, s.networkMetrics.Interface, formatInterfaceStatus(s.networkMetrics.IsUp), networkErrors))
	} else {
		builder.WriteString("└─ 🔍 Network: Detecting interface\n\n")
	}

	// Footer
	builder.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	builder.WriteString("🏷️ **Raspberry Pi 5** - ARM64 | Sermon Uploader v1.1.0\n")
	builder.WriteString("📊 Monitoring CPU, Memory, Thermal, Power, Storage & Network")

	return builder.String()
}

// Helper functions for Discord message formatting

func getResourceStatusIcon(usage, warning, critical float64) string {
	if usage >= critical {
		return "🔴"
	} else if usage >= warning {
		return "🟡"
	}
	return "🟢"
}

func getThermalStatusIcon(temp float64) string {
	if temp >= 80 {
		return "🔴"
	} else if temp >= 70 {
		return "🟡"
	}
	return "🟢"
}

func formatBoolStatus(status bool) string {
	if status {
		return "Yes ❌"
	}
	return "No ✅"
}

func formatInterfaceStatus(isUp bool) string {
	if isUp {
		return "UP"
	}
	return "DOWN"
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatSystemDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func getSystemMonitorESTLocation() *time.Location {
	loc, _ := time.LoadLocation("America/New_York")
	return loc
}