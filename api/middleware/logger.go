package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/x-extract-go/pkg/logger"
	"go.uber.org/zap"
)

// Logger returns a gin middleware for logging
func Logger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		log.Info("HTTP request",
			zap.String("method", method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
			zap.String("client_ip", clientIP),
		)
	}
}

// LoggerWithAdapter returns a gin middleware for logging using LoggerAdapter
func LoggerWithAdapter(logAdapter *logger.LoggerAdapter) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		userAgent := c.Request.UserAgent()

		// Use web access logger
		logAdapter.WebAccess().Info("HTTP request",
			zap.String("method", method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
			zap.String("client_ip", clientIP),
			zap.String("user_agent", userAgent),
		)

		// Log errors to error log as well
		if statusCode >= 400 {
			logAdapter.LogError(logger.CategoryWebAccess, "HTTP error response",
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("status", statusCode),
				zap.String("client_ip", clientIP),
			)
		}
	}
}
