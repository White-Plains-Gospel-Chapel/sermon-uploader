package handlers

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"sermon-uploader/services"
)

// GitHubWebhookHandler handles GitHub webhook requests
func (h *Handlers) GitHubWebhook(c *fiber.Ctx) error {
	// Get GitHub event type
	eventType := c.Get("X-GitHub-Event")
	if eventType == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Missing X-GitHub-Event header",
		})
	}

	// Read request body
	body := c.Body()

	// Get signature for verification
	signature := c.Get("X-Hub-Signature")
	if signature == "" {
		signature = c.Get("X-Hub-Signature-256")
	}

	// Get GitHub webhook service (create if not exists)
	githubService := h.getGitHubWebhookService()

	// Verify signature
	if !githubService.VerifySignature(body, signature) {
		log.Printf("GitHub webhook signature verification failed")
		return c.Status(401).JSON(fiber.Map{
			"success": false,
			"message": "Signature verification failed",
		})
	}

	// Log webhook event
	log.Printf("Received GitHub webhook: %s", eventType)

	// Handle different event types
	switch eventType {
	case "workflow_run":
		if err := githubService.HandleWorkflowRun(body); err != nil {
			log.Printf("Failed to handle workflow_run event: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"message": "Failed to process workflow_run event",
				"error":   err.Error(),
			})
		}
	case "workflow_job":
		// Handle individual job updates (optional, for more granular updates)
		log.Printf("Received workflow_job event (currently not processed)")
	case "ping":
		// GitHub sends a ping event to verify webhook setup
		log.Printf("GitHub webhook ping received")
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Pong! Webhook is configured correctly",
		})
	default:
		log.Printf("Unhandled GitHub event type: %s", eventType)
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Event type not processed",
			"event":   eventType,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Webhook processed successfully",
		"event":   eventType,
	})
}

// getGitHubWebhookService returns the GitHub webhook service, creating it if needed
func (h *Handlers) getGitHubWebhookService() *services.GitHubWebhookService {
	// For now, create a new one each time. In production, this would be cached.
	webhookSecret := h.getGitHubWebhookSecret()
	
	// Convert regular Discord service to live service if needed
	var discordLiveService *services.DiscordLiveService
	if h.discordLiveService != nil {
		discordLiveService = h.discordLiveService
	} else {
		// Create a new Discord live service from the webhook URL
		discordLiveService = services.NewDiscordLiveService(h.config.DiscordWebhookURL)
	}
	
	return services.NewGitHubWebhookService(discordLiveService, webhookSecret)
}

// getGitHubWebhookSecret retrieves the webhook secret from environment
func (h *Handlers) getGitHubWebhookSecret() string {
	// This could come from environment, config, or secrets manager
	// For now, use a default for testing
	// TODO: Add GitHubWebhookSecret to config if needed
	secret := "" // h.config.GitHubWebhookSecret
	if secret == "" {
		// Use default for testing - in production this would be required
		secret = "test-github-secret"
		log.Println("Warning: Using default GitHub webhook secret for testing")
	}
	return secret
}

// TestGitHubWebhook allows manual testing of the webhook endpoint
func (h *Handlers) TestGitHubWebhook(c *fiber.Ctx) error {
	// Create a test workflow_run payload
	testPayload := `{
		"action": "requested",
		"workflow_run": {
			"id": 999999,
			"name": "CI/CD Pipeline",
			"html_url": "https://github.com/test/repo/actions/runs/999999",
			"status": "in_progress",
			"conclusion": null,
			"created_at": "2025-09-07T14:13:00Z",
			"updated_at": "2025-09-07T14:13:30Z",
			"head_commit": {
				"id": "abc123def456",
				"message": "test: manual webhook test"
			},
			"jobs": [
				{
					"name": "Test",
					"status": "in_progress",
					"conclusion": null,
					"started_at": "2025-09-07T14:13:15Z",
					"completed_at": null
				}
			]
		}
	}`

	// Get GitHub webhook service
	githubService := h.getGitHubWebhookService()

	// Process the test payload
	if err := githubService.HandleWorkflowRun([]byte(testPayload)); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to process test webhook",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Test webhook processed successfully - check Discord for live message",
		"payload": "Test workflow_run event sent",
	})
}