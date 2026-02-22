package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogCategory represents different log categories
type LogCategory string

const (
	CategoryQueue LogCategory = "queue" // Queue lifecycle events (JSON)
	CategoryError LogCategory = "error" // Application errors (JSON)
)

// MultiLogger provides categorized logging with separate output files
// Note: Raw download output (stdout/stderr from yt-dlp/tdl) is handled directly
// by the downloaders using file redirects, not through this logger.
type MultiLogger struct {
	loggers     map[LogCategory]*zap.Logger
	config      MultiLoggerConfig
	mu          sync.RWMutex
	currentDate string // Track current date for log rotation
}

// MultiLoggerConfig contains configuration for multi-output logging
type MultiLoggerConfig struct {
	Level   string // debug, info, warn, error
	LogsDir string // Directory for log files
}

// NewMultiLogger creates a new multi-output logger
func NewMultiLogger(config MultiLoggerConfig) (*MultiLogger, error) {
	if config.LogsDir == "" {
		return nil, fmt.Errorf("logs_dir must be specified")
	}

	// Ensure logs directory exists
	if err := os.MkdirAll(config.LogsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	ml := &MultiLogger{
		loggers:     make(map[LogCategory]*zap.Logger),
		config:      config,
		currentDate: time.Now().Format("20060102"),
	}

	// Parse log level
	level, err := zapcore.ParseLevel(config.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	// Create structured logger for queue (JSON format)
	queueLogger, err := ml.createStructuredLogger(CategoryQueue, level)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue logger: %w", err)
	}
	ml.loggers[CategoryQueue] = queueLogger

	// Create structured logger for error (JSON format for application errors)
	errorLogger, err := ml.createStructuredLogger(CategoryError, zapcore.ErrorLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create error logger: %w", err)
	}
	ml.loggers[CategoryError] = errorLogger

	return ml, nil
}

// createStructuredLogger creates a JSON-formatted logger for a category
func (ml *MultiLogger) createStructuredLogger(category LogCategory, level zapcore.Level) (*zap.Logger, error) {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "ts"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.MessageKey = "msg"
	encoderConfig.LevelKey = "level"
	encoderConfig.CallerKey = "" // Don't include caller for cleaner logs

	encoder := zapcore.NewJSONEncoder(encoderConfig)

	logPath := ml.getCategoryLogPath(category)
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	writer := zapcore.AddSync(file)
	core := zapcore.NewCore(encoder, writer, level)

	return zap.New(core), nil
}

// getCategoryLogPath generates a log file path for a category with current date
func (ml *MultiLogger) getCategoryLogPath(category LogCategory) string {
	dateStr := time.Now().Format("20060102")
	filename := fmt.Sprintf("%s-%s.log", category, dateStr)
	return filepath.Join(ml.config.LogsDir, filename)
}

// GetLogsDir returns the logs directory path
func (ml *MultiLogger) GetLogsDir() string {
	return ml.config.LogsDir
}

// GetLogger returns the structured logger for a specific category
func (ml *MultiLogger) GetLogger(category LogCategory) *zap.Logger {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	if logger, ok := ml.loggers[category]; ok {
		return logger
	}

	// Return error logger as fallback
	return ml.loggers[CategoryError]
}

// Queue returns the queue logger (JSON format)
func (ml *MultiLogger) Queue() *zap.Logger {
	return ml.GetLogger(CategoryQueue)
}

// Error returns the error logger (JSON format)
func (ml *MultiLogger) Error() *zap.Logger {
	return ml.GetLogger(CategoryError)
}

// LogAppError logs an application-level error (Go errors, panics)
func (ml *MultiLogger) LogAppError(msg string, fields ...zap.Field) {
	ml.Error().Error(msg, fields...)
}

// LogQueueEvent logs a queue lifecycle event with structured data
func (ml *MultiLogger) LogQueueEvent(event string, fields ...zap.Field) {
	ml.Queue().Info(event, fields...)
}

// Sync flushes all loggers
func (ml *MultiLogger) Sync() error {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	var lastErr error

	for _, logger := range ml.loggers {
		if err := logger.Sync(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// Close closes all loggers
func (ml *MultiLogger) Close() error {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	var lastErr error

	for _, logger := range ml.loggers {
		if err := logger.Sync(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}
