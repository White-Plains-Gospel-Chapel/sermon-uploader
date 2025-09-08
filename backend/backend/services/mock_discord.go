package services

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// MockDiscordService implements DiscordLiveInterface for testing
type MockDiscordService struct {
	messages map[string]string // messageID -> content
	mu       sync.RWMutex
	enabled  bool
}

// NewMockDiscordService creates a mock Discord service for testing
func NewMockDiscordService() *MockDiscordService {
	return &MockDiscordService{
		messages: make(map[string]string),
		enabled:  true,
	}
}

// CreateMessage creates a new message and returns a mock message ID
func (m *MockDiscordService) CreateMessage(content string) (string, error) {
	if !m.enabled {
		return "", fmt.Errorf("mock Discord service disabled")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	messageID := fmt.Sprintf("mock_msg_%d", time.Now().UnixNano())
	m.messages[messageID] = content

	log.Printf("✅ MOCK DISCORD: Created message %s", messageID)
	log.Printf("📄 Content preview: %s", truncateContent(content, 100))
	
	return messageID, nil
}

// UpdateMessage updates an existing message
func (m *MockDiscordService) UpdateMessage(messageID, content string) error {
	if !m.enabled {
		return fmt.Errorf("mock Discord service disabled")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.messages[messageID]; !exists {
		return fmt.Errorf("message %s not found", messageID)
	}

	m.messages[messageID] = content
	log.Printf("🔄 MOCK DISCORD: Updated message %s", messageID)
	log.Printf("📄 Content preview: %s", truncateContent(content, 100))

	return nil
}

// GetMessage retrieves a message for testing
func (m *MockDiscordService) GetMessage(messageID string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	content, exists := m.messages[messageID]
	return content, exists
}

// GetAllMessages returns all messages for testing
func (m *MockDiscordService) GetAllMessages() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string)
	for id, content := range m.messages {
		result[id] = content
	}
	return result
}

// Clear removes all messages
func (m *MockDiscordService) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make(map[string]string)
	log.Printf("🧹 MOCK DISCORD: Cleared all messages")
}

// SetEnabled controls whether the mock service works
func (m *MockDiscordService) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = enabled
	log.Printf("🔧 MOCK DISCORD: Enabled = %t", enabled)
}

// ValidateMessageFormat checks if a message has the expected format
func (m *MockDiscordService) ValidateMessageFormat(content string) error {
	// Check for common Discord message elements
	if len(content) == 0 {
		return fmt.Errorf("content is empty")
	}

	// Check for valid JSON if it looks like embed content
	if content[0] == '{' {
		var jsonData interface{}
		if err := json.Unmarshal([]byte(content), &jsonData); err != nil {
			return fmt.Errorf("invalid JSON format: %w", err)
		}
	}

	// Check content length (Discord limit is 2000 chars for content)
	if len(content) > 2000 {
		return fmt.Errorf("content too long: %d chars (max 2000)", len(content))
	}

	return nil
}

// truncateContent helper for logging
func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}