package api

import (
	"github.com/gin-gonic/gin"
	"github.com/yourusername/x-extract-go/api/handlers"
	"github.com/yourusername/x-extract-go/api/middleware"
	"github.com/yourusername/x-extract-go/internal/app"
	"go.uber.org/zap"
)

// SetupRouter sets up the HTTP router
func SetupRouter(
	queueMgr *app.QueueManager,
	downloadMgr *app.DownloadManager,
	logger *zap.Logger,
) *gin.Engine {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Middleware
	router.Use(middleware.Logger(logger))
	router.Use(middleware.Recovery(logger))
	router.Use(middleware.CORS())

	// Health endpoints
	healthHandler := handlers.NewHealthHandler(queueMgr)
	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Ready)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Download endpoints
		downloadHandler := handlers.NewDownloadHandler(queueMgr, downloadMgr, logger)
		downloads := v1.Group("/downloads")
		{
			downloads.POST("", downloadHandler.AddDownload)
			downloads.GET("", downloadHandler.ListDownloads)
			downloads.GET("/stats", downloadHandler.GetStats)
			downloads.GET("/:id", downloadHandler.GetDownload)
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

