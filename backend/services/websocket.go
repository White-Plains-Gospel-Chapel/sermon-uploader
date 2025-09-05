package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"

	"sermon-uploader/optimization"
)

type WebSocketHub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	pools      *optimization.ObjectPools
	mutex      sync.RWMutex

	// Performance optimizations
	maxClients  int
	bufferSize  int
	messagePool sync.Pool
}

type WebSocketMessage struct {
	Type     string      `json:"type"`
	Data     interface{} `json:"data,omitempty"`
	Filename string      `json:"filename,omitempty"`
	Status   string      `json:"status,omitempty"`
	Message  string      `json:"message,omitempty"`
	Progress float64     `json:"progress,omitempty"`
	// Enhanced progress tracking
	BytesReceived   int64   `json:"bytes_received,omitempty"`
	TotalSize       int64   `json:"total_size,omitempty"`
	UploadSpeed     float64 `json:"upload_speed_mbps,omitempty"`
	ETA             string  `json:"eta,omitempty"`
	ChunksProcessed int     `json:"chunks_processed,omitempty"`
	QualityStatus   string  `json:"quality_status,omitempty"`
	IntegrityCheck  string  `json:"integrity_check,omitempty"`
	SessionID       string  `json:"session_id,omitempty"`
	Timestamp       int64   `json:"timestamp,omitempty"`
}

func NewWebSocketHub() *WebSocketHub {
	// Pi-optimized settings
	maxClients := 20  // Limit concurrent WebSocket connections on Pi
	bufferSize := 256 // Buffer size for broadcast channel

	hub := &WebSocketHub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte, bufferSize),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		pools:      optimization.GetGlobalPools(),
		maxClients: maxClients,
		bufferSize: bufferSize,
		messagePool: sync.Pool{
			New: func() interface{} {
				return &WebSocketMessage{}
			},
		},
	}

	go hub.run()
	return hub
}

func (h *WebSocketHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			// Check client limit for Pi optimization
			if len(h.clients) >= h.maxClients {
				h.mutex.Unlock()
				client.Close()
				log.Printf("WebSocket client rejected: max clients (%d) reached", h.maxClients)
				continue
			}
			h.clients[client] = true
			clientCount := len(h.clients)
			h.mutex.Unlock()
			log.Printf("WebSocket client connected. Total: %d", clientCount)

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			clientCount := len(h.clients)
			h.mutex.Unlock()
			log.Printf("WebSocket client disconnected. Total: %d", clientCount)

		case message := <-h.broadcast:
			h.broadcastToClients(message)
		}
	}
}

// broadcastToClients efficiently broadcasts messages to all connected clients
func (h *WebSocketHub) broadcastToClients(message []byte) {
	h.mutex.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mutex.RUnlock()

	// Track failed clients for cleanup
	var failedClients []*websocket.Conn

	// Send to all clients
	for _, client := range clients {
		err := client.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("WebSocket write error: %v", err)
			failedClients = append(failedClients, client)
		}
	}

	// Cleanup failed clients
	if len(failedClients) > 0 {
		h.mutex.Lock()
		for _, client := range failedClients {
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
		}
		h.mutex.Unlock()
	}
}

func (h *WebSocketHub) HandleConnection(c *websocket.Conn) {
	defer func() {
		h.unregister <- c
		c.Close()
	}()

	h.register <- c

	for {
		// Read message from client (keep connection alive)
		_, _, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

func (h *WebSocketHub) BroadcastMessage(msgType string, data interface{}) error {
	// Get message from pool for efficiency
	msg := h.messagePool.Get().(*WebSocketMessage)
	defer h.messagePool.Put(msg)

	// Reset and populate message
	*msg = WebSocketMessage{
		Type:      msgType,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}

	// Use pooled buffer for JSON marshaling
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	if err := encoder.Encode(msg); err != nil {
		return err
	}

	// Create byte slice copy for the channel (buffer will be reused)
	jsonData := make([]byte, buffer.Len())
	copy(jsonData, buffer.Bytes())

	select {
	case h.broadcast <- jsonData:
	default:
		log.Println("WebSocket broadcast channel is full - message dropped")
		return fmt.Errorf("broadcast channel is full")
	}

	return nil
}

func (h *WebSocketHub) BroadcastFileProgress(filename, status, message string, progress float64) error {
	msg := WebSocketMessage{
		Type:     "file_progress",
		Filename: filename,
		Status:   status,
		Message:  message,
		Progress: progress,
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case h.broadcast <- jsonData:
	default:
		log.Println("WebSocket broadcast channel is full")
	}

	return nil
}

func (h *WebSocketHub) BroadcastUploadStart(totalFiles int, isBatch bool) error {
	return h.BroadcastMessage("upload_start", map[string]interface{}{
		"total_files": totalFiles,
		"is_batch":    isBatch,
	})
}

func (h *WebSocketHub) BroadcastUploadComplete(successful, duplicates, failed int, results interface{}) error {
	return h.BroadcastMessage("upload_complete", map[string]interface{}{
		"successful": successful,
		"duplicates": duplicates,
		"failed":     failed,
		"results":    results,
	})
}

func (h *WebSocketHub) BroadcastError(message string) error {
	return h.BroadcastMessage("error", map[string]interface{}{
		"message": message,
	})
}

// BroadcastStreamingProgress broadcasts detailed streaming progress
func (h *WebSocketHub) BroadcastStreamingProgress(progress *StreamingProgress) error {
	message := WebSocketMessage{
		Type:            "streaming_progress",
		Filename:        progress.Filename,
		Status:          "streaming",
		Message:         fmt.Sprintf("Streaming: %.1f%% complete", progress.Percentage),
		Progress:        progress.Percentage,
		BytesReceived:   progress.BytesReceived,
		TotalSize:       progress.TotalSize,
		UploadSpeed:     progress.UploadSpeed,
		ETA:             progress.ETA,
		ChunksProcessed: progress.ChunksProcessed,
		QualityStatus:   progress.QualityStatus,
		IntegrityCheck:  progress.IntegrityCheck,
		SessionID:       progress.SessionID,
		Timestamp:       progress.LastUpdate.UnixMilli(),
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	h.broadcast <- jsonData
	return nil
}

// BroadcastTUSProgress broadcasts TUS upload progress
func (h *WebSocketHub) BroadcastTUSProgress(info *TUSInfo) error {
	message := WebSocketMessage{
		Type:           "tus_progress",
		Filename:       info.Filename,
		Status:         info.Status,
		Message:        fmt.Sprintf("TUS Upload: %.1f%% complete", info.Progress),
		Progress:       info.Progress,
		BytesReceived:  info.Offset,
		TotalSize:      info.Size,
		QualityStatus:  "monitoring",
		IntegrityCheck: "pending",
		SessionID:      info.UploadID,
		Timestamp:      time.Now().UnixMilli(),
	}

	// Add quality information if available
	if info.HashVerified {
		message.IntegrityCheck = "verified"
	}
	if info.QualityScore > 0 {
		if info.QualityScore == 100 {
			message.QualityStatus = "excellent"
		} else if info.QualityScore >= 95 {
			message.QualityStatus = "good"
		} else {
			message.QualityStatus = "degraded"
		}
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	h.broadcast <- jsonData
	return nil
}

// BroadcastQualityAlert broadcasts quality or integrity alerts
func (h *WebSocketHub) BroadcastQualityAlert(filename, alertType, message string, severity string) error {
	wsMessage := WebSocketMessage{
		Type:     "quality_alert",
		Filename: filename,
		Status:   severity,
		Message:  fmt.Sprintf("%s: %s", alertType, message),
		Data: map[string]interface{}{
			"alert_type": alertType,
			"severity":   severity,
		},
		Timestamp: time.Now().UnixMilli(),
	}

	jsonData, err := json.Marshal(wsMessage)
	if err != nil {
		return err
	}

	h.broadcast <- jsonData
	return nil
}

// BroadcastCompressionStats broadcasts compression and quality statistics
func (h *WebSocketHub) BroadcastCompressionStats(stats *CompressionStats) error {
	message := WebSocketMessage{
		Type: "compression_stats",
		Data: stats,
		Message: fmt.Sprintf("Quality Stats: %d/%d files bit-perfect",
			stats.BitPerfectFiles, stats.TotalFiles),
		Timestamp: time.Now().UnixMilli(),
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	h.broadcast <- jsonData
	return nil
}

// BroadcastSystemAlert broadcasts system-level alerts (memory, performance, etc.)
func (h *WebSocketHub) BroadcastSystemAlert(alertType, message string, data interface{}) error {
	wsMessage := WebSocketMessage{
		Type:    "system_alert",
		Status:  "warning",
		Message: fmt.Sprintf("System Alert: %s", message),
		Data: map[string]interface{}{
			"alert_type": alertType,
			"details":    data,
		},
		Timestamp: time.Now().UnixMilli(),
	}

	jsonData, err := json.Marshal(wsMessage)
	if err != nil {
		return err
	}

	h.broadcast <- jsonData
	return nil
}

// BroadcastIntegrityCheck broadcasts file integrity verification results
func (h *WebSocketHub) BroadcastIntegrityCheck(result *IntegrityResult) error {
	status := "success"
	message := "Integrity verified"

	if !result.IntegrityPassed {
		status = "error"
		message = "Integrity check failed"
	}

	wsMessage := WebSocketMessage{
		Type:           "integrity_check",
		Filename:       result.Filename,
		Status:         status,
		Message:        message,
		Data:           result,
		QualityStatus:  map[bool]string{true: "excellent", false: "failed"}[result.IntegrityPassed],
		IntegrityCheck: map[bool]string{true: "passed", false: "failed"}[result.IntegrityPassed],
		Timestamp:      time.Now().UnixMilli(),
	}

	jsonData, err := json.Marshal(wsMessage)
	if err != nil {
		return err
	}

	h.broadcast <- jsonData
	return nil
}

// GetConnectedClientsCount returns the number of connected WebSocket clients
func (h *WebSocketHub) GetConnectedClientsCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.clients)
}
