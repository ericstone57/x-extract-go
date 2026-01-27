package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/x-extract-go/pkg/logger"
)

// LogHandler handles log-related requests
type LogHandler struct {
	logReader *logger.LogReader
}

// NewLogHandler creates a new log handler
func NewLogHandler(logsDir string) *LogHandler {
	return &LogHandler{
		logReader: logger.NewLogReader(logsDir),
	}
}

// GetLogs handles GET /api/v1/logs/:category
func (h *LogHandler) GetLogs(c *gin.Context) {
	categoryStr := c.Param("category")

	// Validate category
	category := logger.LogCategory(categoryStr)
	validCategories := map[logger.LogCategory]bool{
		logger.CategoryDownload: true,
		logger.CategoryQueue:    true,
		logger.CategoryError:    true,
	}

	if !validCategories[category] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category"})
		return
	}

	// Get query parameters
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000 // Max limit
	}

	dateStr := c.Query("date")
	var date time.Time
	if dateStr != "" {
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format, use YYYY-MM-DD"})
			return
		}
	} else {
		date = time.Now()
	}

	// Read logs
	entries, err := h.logReader.ReadLogs(category, date, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"category": category,
		"date":     date.Format("2006-01-02"),
		"count":    len(entries),
		"entries":  entries,
	})
}

// SearchLogs handles GET /api/v1/logs/:category/search
func (h *LogHandler) SearchLogs(c *gin.Context) {
	categoryStr := c.Param("category")
	category := logger.LogCategory(categoryStr)

	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 0 {
		limit = 100
	}

	dateStr := c.Query("date")
	var date time.Time
	if dateStr != "" {
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format"})
			return
		}
	} else {
		date = time.Now()
	}

	// Search logs
	entries, err := h.logReader.SearchLogs(category, date, query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"category": category,
		"query":    query,
		"count":    len(entries),
		"entries":  entries,
	})
}

// GetCategories handles GET /api/v1/logs/categories
func (h *LogHandler) GetCategories(c *gin.Context) {
	categories := []string{
		string(logger.CategoryDownload),
		string(logger.CategoryQueue),
		string(logger.CategoryError),
	}

	c.JSON(http.StatusOK, gin.H{
		"categories": categories,
	})
}

// ExportLogs handles GET /api/v1/logs/:category/export
func (h *LogHandler) ExportLogs(c *gin.Context) {
	categoryStr := c.Param("category")
	category := logger.LogCategory(categoryStr)

	dateStr := c.Query("date")
	var date time.Time
	var err error
	if dateStr != "" {
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format"})
			return
		}
	} else {
		date = time.Now()
	}

	// Get log file path
	logPath := h.logReader.GetLogPath(category, date)

	// Set headers for download
	filename := string(category) + "-" + date.Format("20060102") + ".log"
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/octet-stream")

	c.File(logPath)
}
