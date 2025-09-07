// +build integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	productionAPI = "https://sermons.wpgc.church"
	localAPI      = "http://192.168.1.127:8000"
	expectedVersion = "1.1.0"
)

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

type HealthInfo struct {
	Status      string `json:"status"`
	Service     string `json:"service"`
	Version     string `json:"version"`
	FullVersion string `json:"fullVersion"`
}

func main() {
	fmt.Println("🔍 Sermon Uploader Deployment Verification")
	fmt.Println("==========================================")
	fmt.Printf("Expected Version: %s\n\n", expectedVersion)
	
	// Check both production and local endpoints
	endpoints := []struct {
		name string
		url  string
	}{
		{"Production (CloudFlare)", productionAPI},
		{"Direct Pi Access", localAPI},
	}
	
	allHealthy := true
	
	for _, endpoint := range endpoints {
		fmt.Printf("Checking %s...\n", endpoint.name)
		fmt.Printf("URL: %s\n", endpoint.url)
		
		// Check health endpoint
		healthOK := checkHealth(endpoint.url)
		
		// Check version endpoint
		versionOK := checkVersion(endpoint.url)
		
		if healthOK && versionOK {
			fmt.Printf("✅ %s is healthy and running version %s\n\n", endpoint.name, expectedVersion)
		} else {
			fmt.Printf("❌ %s verification failed\n\n", endpoint.name)
			allHealthy = false
		}
	}
	
	// Send Discord notification
	sendDiscordNotification(allHealthy)
	
	if allHealthy {
		fmt.Println("✅ Deployment Verification Successful!")
		fmt.Printf("Version %s is deployed and healthy\n", expectedVersion)
		os.Exit(0)
	} else {
		fmt.Println("❌ Deployment Verification Failed!")
		fmt.Println("Please check the deployment logs")
		os.Exit(1)
	}
}

func checkHealth(baseURL string) bool {
	client := &http.Client{Timeout: 10 * time.Second}
	
	resp, err := client.Get(baseURL + "/api/health")
	if err != nil {
		fmt.Printf("  ❌ Health check failed: %v\n", err)
		return false
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("  ❌ Health check returned status %d\n", resp.StatusCode)
		return false
	}
	
	var health HealthInfo
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		fmt.Printf("  ❌ Failed to decode health response: %v\n", err)
		return false
	}
	
	if health.Status != "healthy" {
		fmt.Printf("  ❌ Service is not healthy: %s\n", health.Status)
		return false
	}
	
	if health.Version != expectedVersion {
		fmt.Printf("  ⚠️  Version mismatch in health: expected %s, got %s\n", expectedVersion, health.Version)
		// Don't fail on version mismatch in health check
	}
	
	fmt.Printf("  ✓ Health check passed (status: %s, version: %s)\n", health.Status, health.Version)
	return true
}

func checkVersion(baseURL string) bool {
	client := &http.Client{Timeout: 10 * time.Second}
	
	resp, err := client.Get(baseURL + "/api/version")
	if err != nil {
		fmt.Printf("  ❌ Version check failed: %v\n", err)
		return false
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("  ❌ Version endpoint returned status %d\n", resp.StatusCode)
		return false
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("  ❌ Failed to read version response: %v\n", err)
		return false
	}
	
	var version VersionInfo
	if err := json.Unmarshal(body, &version); err != nil {
		fmt.Printf("  ❌ Failed to decode version response: %v\n", err)
		fmt.Printf("  Response body: %s\n", string(body))
		return false
	}
	
	if version.Version != expectedVersion {
		fmt.Printf("  ❌ Version mismatch: expected %s, got %s\n", expectedVersion, version.Version)
		return false
	}
	
	fmt.Printf("  ✓ Version check passed\n")
	fmt.Printf("    - Version: %s\n", version.Version)
	fmt.Printf("    - Full Version: %s\n", version.FullVersion)
	fmt.Printf("    - Build Time: %s\n", version.BuildTime)
	fmt.Printf("    - Git Commit: %s\n", version.GitCommit)
	fmt.Printf("    - Features: %v\n", version.Features)
	
	return true
}

func sendDiscordNotification(success bool) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" {
		fmt.Println("⚠️  Discord webhook not configured, skipping notification")
		return
	}
	
	var title string
	var color int
	var fields []map[string]interface{}
	
	if success {
		title = "✅ Deployment Verification Successful"
		color = 65280 // Green
		fields = []map[string]interface{}{
			{
				"name":   "Backend Version",
				"value":  fmt.Sprintf("%s-backend", expectedVersion),
				"inline": true,
			},
			{
				"name":   "Frontend Version",
				"value":  fmt.Sprintf("%s-frontend", expectedVersion),
				"inline": true,
			},
			{
				"name":   "Status",
				"value":  "✅ All endpoints healthy",
				"inline": false,
			},
			{
				"name":   "Verified At",
				"value":  time.Now().Format(time.RFC3339),
				"inline": false,
			},
		}
	} else {
		title = "❌ Deployment Verification Failed"
		color = 16711680 // Red
		fields = []map[string]interface{}{
			{
				"name":   "Expected Version",
				"value":  expectedVersion,
				"inline": true,
			},
			{
				"name":   "Status",
				"value":  "❌ Verification failed",
				"inline": true,
			},
			{
				"name":   "Action Required",
				"value":  "Check deployment logs and retry",
				"inline": false,
			},
		}
	}
	
	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":  title,
				"color":  color,
				"fields": fields,
				"footer": map[string]string{
					"text": fmt.Sprintf("Sermon Uploader v%s", expectedVersion),
				},
				"timestamp": time.Now().Format(time.RFC3339),
			},
		},
	}
	
	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("❌ Failed to marshal Discord payload: %v\n", err)
		return
	}
	
	// Send HTTP request to Discord webhook
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("❌ Failed to send Discord notification: %v\n", err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == 204 {
		fmt.Println("📢 Discord notification sent successfully")
	} else {
		fmt.Printf("⚠️  Discord notification failed with status: %d\n", resp.StatusCode)
	}
}