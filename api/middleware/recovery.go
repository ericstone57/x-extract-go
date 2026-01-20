package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/x-extract-go/pkg/logger"
	"go.uber.org/zap"
)

// Recovery returns a gin middleware for panic recovery
func Recovery(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Error("Panic recovered",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}

// RecoveryWithAdapter returns a gin middleware for panic recovery using LoggerAdapter
func RecoveryWithAdapter(logAdapter *logger.LoggerAdapter) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logAdapter.LogError(logger.CategoryWebAccess, "Panic recovered",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
					zap.String("client_ip", c.ClientIP()),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}
