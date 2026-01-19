package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/x-extract-go/internal/app"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	queueMgr *app.QueueManager
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(queueMgr *app.QueueManager) *HealthHandler {
	return &HealthHandler{
		queueMgr: queueMgr,
	}
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Queue   struct {
		Running bool `json:"running"`
	} `json:"queue"`
}

// Health handles GET /health
func (h *HealthHandler) Health(c *gin.Context) {
	response := HealthResponse{
		Status:  "ok",
		Version: "1.0.0",
	}
	response.Queue.Running = h.queueMgr.IsRunning()

	c.JSON(http.StatusOK, response)
}

// Ready handles GET /ready
func (h *HealthHandler) Ready(c *gin.Context) {
	if !h.queueMgr.IsRunning() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "queue manager not running",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

