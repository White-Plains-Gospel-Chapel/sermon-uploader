package services

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// GitHubWorkflowRun represents a GitHub workflow run event
type GitHubWorkflowRun struct {
	Action      string `json:"action"`
	WorkflowRun struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		HTMLURL   string `json:"html_url"`
		Status    string `json:"status"`
		Conclusion *string `json:"conclusion"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		HeadCommit struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		} `json:"head_commit"`
	} `json:"workflow_run"`
}

// GitHubWebhookService handles GitHub webhook events with Discord integration
type GitHubWebhookService struct {
	discordService *DiscordLiveService
	webhookSecret  string
	messageID      string
	pipelineState  map[string]string // phase -> status
}

// NewGitHubWebhookService creates a new GitHub webhook service
func NewGitHubWebhookService(discordService *DiscordLiveService, webhookSecret string) *GitHubWebhookService {
	return &GitHubWebhookService{
		discordService: discordService,
		webhookSecret:  webhookSecret,
		pipelineState:  make(map[string]string),
	}
}

// VerifySignature verifies GitHub webhook signature
func (g *GitHubWebhookService) VerifySignature(body []byte, signature string) bool {
	if g.webhookSecret == "" {
		// For testing, allow unsigned requests
		return true
	}

	if signature == "" {
		return false
	}

	// Handle both SHA1 and SHA256 signatures
	if strings.HasPrefix(signature, "sha1=") {
		mac := hmac.New(sha1.New, []byte(g.webhookSecret))
		mac.Write(body)
		expectedSignature := "sha1=" + hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(signature), []byte(expectedSignature))
	}

	if strings.HasPrefix(signature, "sha256=") {
		mac := hmac.New(sha256.New, []byte(g.webhookSecret))
		mac.Write(body)
		expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(signature), []byte(expectedSignature))
	}

	return false
}

// HandleWorkflowRun processes workflow run events
func (g *GitHubWebhookService) HandleWorkflowRun(body []byte) error {
	var payload GitHubWorkflowRun
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to parse workflow run payload: %w", err)
	}

	// Update granular pipeline state based on workflow name and status
	workflowName := payload.WorkflowRun.Name
	
	switch payload.WorkflowRun.Status {
	case "queued":
		if strings.Contains(strings.ToLower(workflowName), "test") {
			g.pipelineState["Test Setup"] = "queued"
		} else {
			g.pipelineState["Pipeline"] = "queued"
		}
	case "in_progress":
		// More granular tracking based on workflow progress
		g.updateGranularProgress(payload)
	case "completed":
		if payload.WorkflowRun.Conclusion != nil {
			switch *payload.WorkflowRun.Conclusion {
			case "success":
				g.updateSuccessfulStage(workflowName)
			case "failure":
				g.updateFailedStage(workflowName)
			}
		}
	}

	return g.updateDiscordMessage(payload)
}

// updateDiscordMessage creates or updates the Discord live message
func (g *GitHubWebhookService) updateDiscordMessage(payload GitHubWorkflowRun) error {
	if g.discordService == nil {
		return fmt.Errorf("Discord service not available")
	}

	content := g.buildDiscordContent(payload)

	if g.messageID == "" {
		// Create new message
		messageID, err := g.discordService.CreateMessage(content)
		if err != nil {
			return fmt.Errorf("failed to create Discord message: %w", err)
		}
		g.messageID = messageID
	} else {
		// Update existing message
		err := g.discordService.UpdateMessage(g.messageID, content)
		if err != nil {
			return fmt.Errorf("failed to update Discord message: %w", err)
		}
	}

	return nil
}

// buildDiscordContent creates the formatted Discord message
func (g *GitHubWebhookService) buildDiscordContent(payload GitHubWorkflowRun) string {
	est := time.Now().In(getESTLocation())
	commitID := payload.WorkflowRun.HeadCommit.ID
	if len(commitID) > 7 {
		commitID = commitID[:7]
	}

	var content strings.Builder
	content.WriteString("🚀 **Deployment Pipeline - Live Status**\n")
	content.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	content.WriteString(fmt.Sprintf("📊 Commit: %s (%s)\n", commitID, 
		truncateString(payload.WorkflowRun.HeadCommit.Message, 50)))
	content.WriteString("🌟 Version: v1.1.0\n\n")

	content.WriteString("**Pipeline Status:**\n")
	
	// Build consolidated one-line status with granular details
	pipelineStatus := g.buildGranularStatusLine()
	content.WriteString(fmt.Sprintf("🔄 %s\n\n", pipelineStatus))

	content.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	content.WriteString(fmt.Sprintf("🕐 Started: %s\n", 
		payload.WorkflowRun.CreatedAt.In(getESTLocation()).Format("3:04 PM EST")))
	content.WriteString(fmt.Sprintf("🔄 Last Updated: %s\n", est.Format("3:04 PM EST")))
	content.WriteString(fmt.Sprintf("📂 View Run: %s\n", payload.WorkflowRun.HTMLURL))

	return content.String()
}

// Helper functions
func getStatusIcon(status string) string {
	switch status {
	case "success":
		return "✅"
	case "failure":
		return "❌"
	case "in_progress":
		return "🔄"
	case "queued":
		return "⏳"
	default:
		return "⏳"
	}
}

func getStatusText(status string) string {
	switch status {
	case "success":
		return "success"
	case "failure":
		return "failed"
	case "in_progress":
		return "running..."
	case "queued":
		return "queued"
	default:
		return "pending"
	}
}

func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}

func getESTLocation() *time.Location {
	loc, _ := time.LoadLocation("America/New_York")
	return loc
}

// updateGranularProgress updates pipeline state with more detailed progress
func (g *GitHubWebhookService) updateGranularProgress(payload GitHubWorkflowRun) {
	workflowName := strings.ToLower(payload.WorkflowRun.Name)
	
	if strings.Contains(workflowName, "test") {
		g.pipelineState["Test"] = "in_progress"
		g.pipelineState["Backend Tests"] = "running"
		g.pipelineState["Frontend Tests"] = "pending"
	} else if strings.Contains(workflowName, "build") {
		g.pipelineState["Test"] = "success"
		g.pipelineState["Backend Tests"] = "passed"
		g.pipelineState["Frontend Tests"] = "passed"
		g.pipelineState["Docker Build"] = "in_progress"
		g.pipelineState["ARM64 Build"] = "running"
	} else {
		// General CI/CD pipeline
		g.pipelineState["Environment Setup"] = "running"
		g.pipelineState["Dependencies"] = "installing"
	}
}

// updateSuccessfulStage updates state when a stage completes successfully
func (g *GitHubWebhookService) updateSuccessfulStage(workflowName string) {
	workflowName = strings.ToLower(workflowName)
	
	if strings.Contains(workflowName, "test") {
		g.pipelineState["Backend Tests"] = "passed"
		g.pipelineState["Frontend Tests"] = "passed" 
		g.pipelineState["Docker Build"] = "starting"
	} else if strings.Contains(workflowName, "build") {
		g.pipelineState["Docker Build"] = "success"
		g.pipelineState["ARM64 Build"] = "completed"
		g.pipelineState["Deploy"] = "starting"
	} else {
		g.pipelineState["Pipeline"] = "success"
		g.pipelineState["Deploy"] = "ready"
	}
}

// updateFailedStage updates state when a stage fails
func (g *GitHubWebhookService) updateFailedStage(workflowName string) {
	workflowName = strings.ToLower(workflowName)
	
	if strings.Contains(workflowName, "test") {
		g.pipelineState["Backend Tests"] = "failed"
		g.pipelineState["Docker Build"] = "blocked"
	} else if strings.Contains(workflowName, "build") {
		g.pipelineState["Docker Build"] = "failed"
		g.pipelineState["ARM64 Build"] = "failed"
		g.pipelineState["Deploy"] = "blocked"
	} else {
		g.pipelineState["Pipeline"] = "failed"
	}
}

// buildGranularStatusLine creates a one-line status with detailed pipeline progress
func (g *GitHubWebhookService) buildGranularStatusLine() string {
	var statusParts []string
	
	// Define the pipeline stages in order
	stages := []struct {
		key     string
		display string
	}{
		{"Backend Tests", "Backend Tests"},
		{"Frontend Tests", "Frontend Tests"},
		{"Docker Build", "Docker Build"},
		{"ARM64 Build", "ARM64 Cross-Compile"},
		{"Deploy", "Deploy"},
	}
	
	for _, stage := range stages {
		status := g.pipelineState[stage.key]
		if status == "" {
			continue // Skip stages not yet started
		}
		
		icon := getGranularStatusIcon(status)
		statusParts = append(statusParts, fmt.Sprintf("%s %s", icon, stage.display))
	}
	
	if len(statusParts) == 0 {
		return "Pipeline starting..."
	}
	
	return strings.Join(statusParts, " • ")
}

// getGranularStatusIcon returns appropriate icon for granular status
func getGranularStatusIcon(status string) string {
	switch status {
	case "running", "in_progress", "installing", "starting":
		return "🔄"
	case "passed", "success", "completed", "ready":
		return "✅"
	case "failed", "blocked":
		return "❌"
	case "pending", "queued":
		return "⏳"
	default:
		return "⏳"
	}
}