package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sermon-uploader/config"
)

func TestVersionEndpoint(t *testing.T) {
	// Arrange - Set version info
	originalVersion := config.Version
	originalBuildTime := config.BuildTime
	originalGitCommit := config.GitCommit
	
	config.Version = "1.1.0"
	config.BuildTime = time.Now().Format(time.RFC3339)
	config.GitCommit = "abc123"
	
	defer func() {
		config.Version = originalVersion
		config.BuildTime = originalBuildTime
		config.GitCommit = originalGitCommit
	}()
	
	cfg := config.New()
	h := New(nil, nil, nil, nil, nil, cfg, nil)
	
	app := fiber.New()
	app.Get("/api/version", h.GetVersion)
	
	// Act
	req := httptest.NewRequest("GET", "/api/version", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	
	// Assert
	assert.Equal(t, 200, resp.StatusCode)
	
	var version map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&version)
	require.NoError(t, err)
	
	// Check required fields
	assert.Equal(t, "1.1.0", version["version"])
	assert.Equal(t, "sermon-uploader-backend", version["service"])
	assert.Equal(t, "1.1.0-backend", version["fullVersion"])
	assert.NotEmpty(t, version["buildTime"])
	assert.NotEmpty(t, version["gitCommit"])
	assert.Equal(t, runtime.Version(), version["goVersion"])
	
	// Check features
	features, ok := version["features"].(map[string]interface{})
	assert.True(t, ok, "features should be present")
	assert.True(t, features["largeFileUpload"].(bool))
	assert.True(t, features["cloudflareBypass"].(bool))
	assert.Equal(t, "10GB", features["maxFileSize"])
	
	t.Logf("✅ Version endpoint test passed: %+v", version)
}

func TestVersionEndpoint_HealthCheck_Enhanced(t *testing.T) {
	// Arrange
	config.Version = "1.1.0"
	
	cfg := config.New()
	h := New(nil, nil, nil, nil, nil, cfg, nil)
	
	app := fiber.New()
	app.Get("/api/health", h.HealthCheck)
	
	// Act
	req := httptest.NewRequest("GET", "/api/health", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	
	// Assert
	assert.Equal(t, 200, resp.StatusCode)
	
	var health map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&health)
	require.NoError(t, err)
	
	// Health check should now include version
	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, "sermon-uploader-go", health["service"])
	assert.Equal(t, "1.1.0", health["version"])
	assert.Equal(t, "1.1.0-backend", health["fullVersion"])
	
	t.Logf("✅ Enhanced health check with version: %+v", health)
}

func TestVersionCompatibilityCheck(t *testing.T) {
	tests := []struct {
		name           string
		clientVersion  string
		serverVersion  string
		expectSuccess  bool
	}{
		{
			name:          "Same version - compatible",
			clientVersion: "1.1.0",
			serverVersion: "1.1.0",
			expectSuccess: true,
		},
		{
			name:          "Minor version difference - compatible",
			clientVersion: "1.1.0",
			serverVersion: "1.1.1",
			expectSuccess: true,
		},
		{
			name:          "Major version difference - incompatible",
			clientVersion: "1.0.0",
			serverVersion: "2.0.0",
			expectSuccess: false,
		},
		{
			name:          "Frontend/Backend suffix - compatible",
			clientVersion: "1.1.0-frontend",
			serverVersion: "1.1.0-backend",
			expectSuccess: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will test the version compatibility logic
			compatible := CheckVersionCompatibility(tt.clientVersion, tt.serverVersion)
			assert.Equal(t, tt.expectSuccess, compatible)
		})
	}
}

// Helper function to be implemented
func CheckVersionCompatibility(client, server string) bool {
	// Extract major.minor version for compatibility check
	// For now, this is a placeholder that will be implemented
	return true // Will fail tests initially (TDD Red phase)
}