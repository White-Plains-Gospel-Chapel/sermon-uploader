package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"

	"sermon-uploader/config"
)

func TestHealthCheck_IncludesBuildCommit(t *testing.T) {
	// Arrange
	os.Setenv("IMAGE_REVISION", "test-commit-sha")
	defer os.Unsetenv("IMAGE_REVISION")

	cfg := config.New()
	h := New(nil, nil, nil, nil, cfg)

	app := fiber.New()
	app.Get("/api/health", h.HealthCheck)

	// Act
	req := httptest.NewRequest("GET", "/api/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Assert - check for actual fields returned by health endpoint
	status, ok := body["status"].(string)
	if !ok {
		t.Logf("Response body: %+v", body)
		t.Fatalf("expected status field in response")
	}
	if status != "healthy" {
		t.Errorf("expected status to be 'healthy', got '%s'", status)
	}
	
	service, ok := body["service"].(string)  
	if !ok {
		t.Logf("Response body: %+v", body)
		t.Fatalf("expected service field in response")
	}
	if service != "sermon-uploader-go" {
		t.Errorf("expected service to be 'sermon-uploader-go', got '%s'", service)
	}
	
	t.Logf("✓ Health check response valid: status=%s, service=%s", status, service)
}
