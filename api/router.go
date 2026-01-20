package api

import (
	"github.com/gin-gonic/gin"
	"github.com/yourusername/x-extract-go/api/handlers"
	"github.com/yourusername/x-extract-go/api/middleware"
	"github.com/yourusername/x-extract-go/internal/app"
	"github.com/yourusername/x-extract-go/pkg/logger"
	"go.uber.org/zap"
)

// SetupRouter sets up the HTTP router
func SetupRouter(
	queueMgr *app.QueueManager,
	downloadMgr *app.DownloadManager,
	log *zap.Logger,
) *gin.Engine {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Middleware
	router.Use(middleware.Logger(log))
	router.Use(middleware.Recovery(log))
	router.Use(middleware.CORS())

	// Health endpoints
	healthHandler := handlers.NewHealthHandler(queueMgr)
	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Ready)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Download endpoints
		downloadHandler := handlers.NewDownloadHandler(queueMgr, downloadMgr, log)
		downloads := v1.Group("/downloads")
		{
			downloads.POST("", downloadHandler.AddDownload)
			downloads.GET("", downloadHandler.ListDownloads)
			downloads.GET("/stats", downloadHandler.GetStats)
			downloads.GET("/:id", downloadHandler.GetDownload)
			downloads.GET("/:id/logs", downloadHandler.GetDownloadLogs)
			downloads.POST("/:id/cancel", downloadHandler.CancelDownload)
			downloads.POST("/:id/retry", downloadHandler.RetryDownload)
		}
	}

	// Serve static files for web UI
	router.Static("/static", "./web/static")
	router.LoadHTMLGlob("./web/templates/*")
	router.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})

	return router
}

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

	// Middleware with multi-logger
	router.Use(middleware.LoggerWithAdapter(logAdapter))
	router.Use(middleware.RecoveryWithAdapter(logAdapter))
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
			downloads.GET("/:id/logs", downloadHandler.GetDownloadLogs)
			downloads.POST("/:id/cancel", downloadHandler.CancelDownload)
			downloads.POST("/:id/retry", downloadHandler.RetryDownload)
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

		// WebSocket endpoint for real-time logs
		wsHandler := handlers.NewLogWebSocketHandler(logsDir, logAdapter.GetSingleLogger())
		v1.GET("/logs/stream", wsHandler.HandleWebSocket)
	}

	// Serve static files for web UI
	router.Static("/static", "./web/static")
	router.LoadHTMLGlob("./web/templates/*")
	router.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})

	// Log viewer page
	router.GET("/logs", func(c *gin.Context) {
		c.HTML(200, "logs.html", nil)
	})

	return router
}
