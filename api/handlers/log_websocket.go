package handlers

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/yourusername/x-extract-go/pkg/logger"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// LogWebSocketHandler handles WebSocket connections for real-time log streaming
type LogWebSocketHandler struct {
	logReader *logger.LogReader
	logger    *zap.Logger
	clients   map[*websocket.Conn]bool
	mu        sync.RWMutex
}

// NewLogWebSocketHandler creates a new WebSocket handler
func NewLogWebSocketHandler(logsDir string, log *zap.Logger) *LogWebSocketHandler {
	return &LogWebSocketHandler{
		logReader: logger.NewLogReader(logsDir),
		logger:    log,
		clients:   make(map[*websocket.Conn]bool),
	}
}

// HandleWebSocket handles WebSocket connections for log streaming
func (h *LogWebSocketHandler) HandleWebSocket(c *gin.Context) {
	categoryStr := c.Query("category")
	if categoryStr == "" {
		categoryStr = string(logger.CategoryDownload)
	}

	category := logger.LogCategory(categoryStr)

	// Upgrade connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade WebSocket", zap.Error(err))
		return
	}
	defer conn.Close()

	// Register client
	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
	}()

	h.logger.Info("WebSocket client connected",
		zap.String("category", string(category)),
		zap.String("remote_addr", c.Request.RemoteAddr))

	// Send initial logs (last 50 entries)
	entries, err := h.logReader.ReadTodayLogs(category, 50)
	if err == nil {
		for _, entry := range entries {
			data, _ := json.Marshal(entry)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				h.logger.Error("Failed to send initial logs", zap.Error(err))
				return
			}
		}
	}

	// Start tailing logs
	entryChan := make(chan logger.LogEntry, 100)
	stopChan := make(chan struct{})
	defer close(stopChan)

	// Start log tailer in goroutine
	go func() {
		if err := h.logReader.TailLogs(category, entryChan, stopChan); err != nil {
			h.logger.Error("Log tailing error", zap.Error(err))
		}
	}()

	// Handle incoming messages and send log entries
	done := make(chan struct{})

	// Read messages from client (for ping/pong)
	go func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	// Send log entries to client
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case entry := <-entryChan:
			data, err := json.Marshal(entry)
			if err != nil {
				h.logger.Error("Failed to marshal log entry", zap.Error(err))
				continue
			}

			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				h.logger.Error("Failed to send log entry", zap.Error(err))
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-done:
			return
		}
	}
}

// BroadcastLogEntry broadcasts a log entry to all connected clients
func (h *LogWebSocketHandler) BroadcastLogEntry(entry logger.LogEntry) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	data, err := json.Marshal(entry)
	if err != nil {
		h.logger.Error("Failed to marshal log entry for broadcast", zap.Error(err))
		return
	}

	for conn := range h.clients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			h.logger.Error("Failed to broadcast log entry", zap.Error(err))
			// Connection will be cleaned up by the handler goroutine
		}
	}
}
