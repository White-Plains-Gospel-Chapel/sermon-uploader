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

	// Assert
	build, ok := body["build"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected build field in response")
	}
	if commit, _ := build["commit"].(string); commit != "test-commit-sha" {
		t.Fatalf("expected build.commit to be 'test-commit-sha', got '%s'", commit)
	}
}
