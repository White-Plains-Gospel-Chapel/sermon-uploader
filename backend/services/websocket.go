package services

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gofiber/websocket/v2"
)

type WebSocketHub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mutex      sync.RWMutex
}

type WebSocketMessage struct {
	Type     string      `json:"type"`
	Data     interface{} `json:"data,omitempty"`
	Filename string      `json:"filename,omitempty"`
	Status   string      `json:"status,omitempty"`
	Message  string      `json:"message,omitempty"`
	Progress float64     `json:"progress,omitempty"`
}

func NewWebSocketHub() *WebSocketHub {
	hub := &WebSocketHub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}

	go hub.run()
	return hub
}

func (h *WebSocketHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			log.Printf("WebSocket client connected. Total: %d", len(h.clients))

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mutex.Unlock()
			log.Printf("WebSocket client disconnected. Total: %d", len(h.clients))

		case message := <-h.broadcast:
			h.mutex.RLock()
			for client := range h.clients {
				err := client.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					log.Printf("WebSocket write error: %v", err)
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mutex.RUnlock()
		}
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
	message := WebSocketMessage{
		Type: msgType,
		Data: data,
	}

	jsonData, err := json.Marshal(message)
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
