package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/x-extract-go/internal/app"
	"github.com/yourusername/x-extract-go/internal/domain"
	"go.uber.org/zap"
)

// DownloadHandler handles download-related HTTP requests
type DownloadHandler struct {
	queueMgr    *app.QueueManager
	downloadMgr *app.DownloadManager
	logger      *zap.Logger
}

// NewDownloadHandler creates a new download handler
func NewDownloadHandler(queueMgr *app.QueueManager, downloadMgr *app.DownloadManager, logger *zap.Logger) *DownloadHandler {
	return &DownloadHandler{
		queueMgr:    queueMgr,
		downloadMgr: downloadMgr,
		logger:      logger,
	}
}

// AddDownloadRequest represents a request to add a download
type AddDownloadRequest struct {
	URL      string `json:"url" binding:"required"`
	Platform string `json:"platform,omitempty"`
	Mode     string `json:"mode,omitempty"`
}

// AddDownload handles POST /api/downloads
func (h *DownloadHandler) AddDownload(c *gin.Context) {
	var req AddDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Auto-detect platform if not provided
	platform := domain.Platform(req.Platform)
	if platform == "" {
		platform = domain.DetectPlatform(req.URL)
		if platform == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported URL or platform"})
			return
		}
	}

	// Default mode
	mode := domain.DownloadMode(req.Mode)
	if mode == "" {
		mode = domain.ModeDefault
	}

	// Add to queue
	download, err := h.queueMgr.AddDownload(req.URL, platform, mode)
	if err != nil {
		h.logger.Error("Failed to add download", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, download)
}

// GetDownload handles GET /api/downloads/:id
func (h *DownloadHandler) GetDownload(c *gin.Context) {
	id := c.Param("id")

	download, err := h.queueMgr.GetDownload(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "download not found"})
		return
	}

	c.JSON(http.StatusOK, download)
}

// ListDownloads handles GET /api/downloads
func (h *DownloadHandler) ListDownloads(c *gin.Context) {
	// Parse query parameters for filtering
	filters := make(map[string]interface{})

	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if platform := c.Query("platform"); platform != "" {
		filters["platform"] = platform
	}

	downloads, err := h.queueMgr.ListDownloads(filters)
	if err != nil {
		h.logger.Error("Failed to list downloads", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, downloads)
}

// GetStats handles GET /api/downloads/stats
func (h *DownloadHandler) GetStats(c *gin.Context) {
	stats, err := h.queueMgr.GetStats()
	if err != nil {
		h.logger.Error("Failed to get stats", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// CancelDownload handles POST /api/downloads/:id/cancel
func (h *DownloadHandler) CancelDownload(c *gin.Context) {
	id := c.Param("id")

	if err := h.downloadMgr.CancelDownload(id); err != nil {
		h.logger.Error("Failed to cancel download", zap.String("id", id), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "download cancelled"})
}

// RetryDownload handles POST /api/downloads/:id/retry
func (h *DownloadHandler) RetryDownload(c *gin.Context) {
	id := c.Param("id")

	if err := h.downloadMgr.RetryDownload(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to retry download", zap.String("id", id), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "download queued for retry"})
}

