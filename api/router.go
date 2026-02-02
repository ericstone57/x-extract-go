package api

import (
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yourusername/x-extract-go/api/handlers"
	"github.com/yourusername/x-extract-go/api/middleware"
	"github.com/yourusername/x-extract-go/internal/app"
	"github.com/yourusername/x-extract-go/pkg/logger"
	"github.com/yourusername/x-extract-go/web"
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

	// Serve embedded static files for web UI
	staticFS := http.FS(web.GetStaticFS())
	router.StaticFS("/static", staticFS)

	// Load embedded HTML templates
	tmpl := template.Must(template.ParseFS(web.GetTemplatesFS(), "*.html"))
	router.SetHTMLTemplate(tmpl)

	router.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})

	// Log viewer page
	router.GET("/logs", func(c *gin.Context) {
		c.HTML(200, "logs.html", nil)
	})

	return router
}
