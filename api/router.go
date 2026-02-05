package api

import (
	"io"
	"io/fs"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yourusername/x-extract-go/api/handlers"
	"github.com/yourusername/x-extract-go/api/middleware"
	"github.com/yourusername/x-extract-go/internal/app"
	"github.com/yourusername/x-extract-go/pkg/logger"
	dashboard "github.com/yourusername/x-extract-go/web-dashboard"
)

// SetupRouterWithMultiLogger sets up the HTTP router with multi-logger support
func SetupRouterWithMultiLogger(
	queueMgr *app.QueueManager,
	downloadMgr *app.DownloadManager,
	logAdapter *logger.LoggerAdapter,
	logsDir string,
) *gin.Engine {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Middleware
	router.Use(middleware.Logger(logAdapter.GetSingleLogger()))
	router.Use(middleware.Recovery(logAdapter.GetSingleLogger()))
	router.Use(middleware.CORS())

	// Health endpoints
	healthHandler := handlers.NewHealthHandler(queueMgr)
	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Ready)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Download endpoints
		downloadHandler := handlers.NewDownloadHandler(queueMgr, downloadMgr, logAdapter.GetSingleLogger())
		downloads := v1.Group("/downloads")
		{
			downloads.POST("", downloadHandler.AddDownload)
			downloads.GET("", downloadHandler.ListDownloads)
			downloads.GET("/stats", downloadHandler.GetStats)
			downloads.GET("/:id", downloadHandler.GetDownload)
			downloads.POST("/:id/cancel", downloadHandler.CancelDownload)
			downloads.POST("/:id/retry", downloadHandler.RetryDownload)
			downloads.DELETE("/:id", downloadHandler.DeleteDownload)
		}

		// Log endpoints
		logHandler := handlers.NewLogHandler(logsDir)
		logs := v1.Group("/logs")
		{
			logs.GET("/categories", logHandler.GetCategories)
			logs.GET("/:category", logHandler.GetLogs)
			logs.GET("/:category/search", logHandler.SearchLogs)
			logs.GET("/:category/export", logHandler.ExportLogs)
		}
	}

	// Serve embedded Next.js dashboard
	dashboardFS := dashboard.GetDashboardFS()

	// Serve static assets from _next directory
	router.GET("/_next/*filepath", func(c *gin.Context) {
		filePath := strings.TrimPrefix(c.Request.URL.Path, "/")
		serveFile(c, dashboardFS, filePath)
	})

	// Explicitly handle root path
	router.GET("/", func(c *gin.Context) {
		serveIndexHTML(c, dashboardFS)
	})

	// Serve all other routes with SPA routing
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Don't serve dashboard for API routes
		if strings.HasPrefix(path, "/api/") {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}

		// Remove leading slash for filesystem lookup
		filePath := strings.TrimPrefix(path, "/")

		// Root path
		if filePath == "" {
			serveIndexHTML(c, dashboardFS)
			return
		}

		// Remove trailing slash
		if strings.HasSuffix(filePath, "/") {
			filePath = strings.TrimSuffix(filePath, "/")
		}

		// For paths like /downloads, Next.js static export creates /downloads/index.html
		// Try the path with /index.html appended
		pathWithIndex := filePath + "/index.html"
		if _, err := fs.Stat(dashboardFS, pathWithIndex); err == nil {
			serveFile(c, dashboardFS, pathWithIndex)
			return
		}

		// Try the exact path
		if _, err := fs.Stat(dashboardFS, filePath); err == nil {
			serveFile(c, dashboardFS, filePath)
			return
		}

		// Fallback to index.html for client-side routing
		serveIndexHTML(c, dashboardFS)
	})

	return router
}

// serveIndexHTML serves the index.html file from the embedded filesystem
func serveIndexHTML(c *gin.Context, dashboardFS fs.FS) {
	serveFile(c, dashboardFS, "index.html")
}

// serveFile serves a file from the embedded filesystem with proper content type
func serveFile(c *gin.Context, dashboardFS fs.FS, filePath string) {
	// Read the file from the embedded filesystem
	file, err := dashboardFS.Open(filePath)
	if err != nil {
		c.String(404, "File not found: %v", err)
		return
	}
	defer file.Close()

	// Read the content
	content, err := io.ReadAll(file)
	if err != nil {
		c.String(500, "Failed to read file: %v", err)
		return
	}

	// Determine content type based on file extension
	contentType := "application/octet-stream"
	if strings.HasSuffix(filePath, ".html") {
		contentType = "text/html; charset=utf-8"
	} else if strings.HasSuffix(filePath, ".css") {
		contentType = "text/css; charset=utf-8"
	} else if strings.HasSuffix(filePath, ".js") {
		contentType = "application/javascript; charset=utf-8"
	} else if strings.HasSuffix(filePath, ".json") {
		contentType = "application/json; charset=utf-8"
	} else if strings.HasSuffix(filePath, ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(filePath, ".jpg") || strings.HasSuffix(filePath, ".jpeg") {
		contentType = "image/jpeg"
	} else if strings.HasSuffix(filePath, ".svg") {
		contentType = "image/svg+xml"
	} else if strings.HasSuffix(filePath, ".woff") {
		contentType = "font/woff"
	} else if strings.HasSuffix(filePath, ".woff2") {
		contentType = "font/woff2"
	} else if strings.HasSuffix(filePath, ".txt") {
		contentType = "text/plain; charset=utf-8"
	}

	// Serve with correct content type
	c.Data(200, contentType, content)
}
