package handlers

import (
	"runtime"
	"time"

	"github.com/gofiber/fiber/v2"

	"sermon-uploader/config"
)

// VersionInfo represents the version information of the service
type VersionInfo struct {
	Version      string                 `json:"version"`
	Service      string                 `json:"service"`
	FullVersion  string                 `json:"fullVersion"`
	BuildTime    string                 `json:"buildTime"`
	GitCommit    string                 `json:"gitCommit"`
	GoVersion    string                 `json:"goVersion"`
	Features     map[string]interface{} `json:"features"`
	Environment  string                 `json:"environment"`
	DeployedAt   string                 `json:"deployedAt"`
}

// GetVersion returns version information
func (h *Handlers) GetVersion(c *fiber.Ctx) error {
	versionInfo := VersionInfo{
		Version:     config.Version,
		Service:     "sermon-uploader-backend",
		FullVersion: config.GetFullVersion("backend"),
		BuildTime:   config.BuildTime,
		GitCommit:   config.GitCommit,
		GoVersion:   runtime.Version(),
		Features: map[string]interface{}{
			"largeFileUpload":  true,
			"cloudflareBypass": true,
			"maxFileSize":      "10GB",
			"tddTested":        true,
			"arm64Support":     true,
			"versionTracking":  true,
		},
		Environment: h.config.Environment,
		DeployedAt:  time.Now().Format(time.RFC3339),
	}
	
	// Log version request for monitoring
	if h.logger != nil {
		h.logger.Info("Version requested",
			"version", versionInfo.Version,
			"client_ip", c.IP(),
			"user_agent", c.Get("User-Agent"),
		)
	}
	
	return c.JSON(versionInfo)
}

// Enhanced HealthCheck now includes version
func (h *Handlers) HealthCheckEnhanced(c *fiber.Ctx) error {
	health := fiber.Map{
		"status":      "healthy",
		"service":     "sermon-uploader-go",
		"version":     config.Version,
		"fullVersion": config.GetFullVersion("backend"),
		"timestamp": fiber.Map{
			"now": time.Now().Format(time.RFC3339),
		},
		"uptime": time.Since(h.startTime).String(),
	}
	
	return c.JSON(health)
}