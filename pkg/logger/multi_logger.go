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
	CategoryWebAccess        LogCategory = "web-access"
	CategoryDownloadProgress LogCategory = "download-progress"
	CategoryQueue            LogCategory = "queue"
	CategoryError            LogCategory = "error"
	CategoryGeneral          LogCategory = "general"
)

// MultiLogger provides categorized logging with separate output files
type MultiLogger struct {
	loggers map[LogCategory]*zap.Logger
	config  MultiLoggerConfig
	mu      sync.RWMutex
}

// MultiLoggerConfig contains configuration for multi-output logging
type MultiLoggerConfig struct {
	Level      string // debug, info, warn, error
	Format     string // json, console
	LogsDir    string // Directory for log files
	EnableJSON bool   // Use JSON format for file logs
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
		loggers: make(map[LogCategory]*zap.Logger),
		config:  config,
	}

	// Parse log level
	level, err := zapcore.ParseLevel(config.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	// Create loggers for each category
	categories := []LogCategory{
		CategoryWebAccess,
		CategoryDownloadProgress,
		CategoryQueue,
		CategoryError,
		CategoryGeneral,
	}

	for _, category := range categories {
		logger, err := ml.createCategoryLogger(category, level)
		if err != nil {
			return nil, fmt.Errorf("failed to create logger for %s: %w", category, err)
		}
		ml.loggers[category] = logger
	}

	return ml, nil
}

// createCategoryLogger creates a logger for a specific category
func (ml *MultiLogger) createCategoryLogger(category LogCategory, level zapcore.Level) (*zap.Logger, error) {
	// Configure encoder for JSON format
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.MessageKey = "message"
	encoderConfig.LevelKey = "level"
	encoderConfig.CallerKey = "caller"

	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// Create log file path with date
	logPath := ml.getCategoryLogPath(category)
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	writer := zapcore.AddSync(file)

	// Create core
	core := zapcore.NewCore(encoder, writer, level)

	// For error category, also log to error.log
	if category == CategoryError {
		// Error logger gets all error-level logs
		core = zapcore.NewCore(encoder, writer, zapcore.ErrorLevel)
	}

	// Create logger with caller and stacktrace
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.Fields(zap.String("category", string(category))),
	)

	return logger, nil
}

// getCategoryLogPath generates a log file path for a category with current date
func (ml *MultiLogger) getCategoryLogPath(category LogCategory) string {
	dateStr := time.Now().Format("20060102")
	filename := fmt.Sprintf("%s-%s.log", category, dateStr)
	return filepath.Join(ml.config.LogsDir, filename)
}

// GetLogger returns the logger for a specific category
func (ml *MultiLogger) GetLogger(category LogCategory) *zap.Logger {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	if logger, ok := ml.loggers[category]; ok {
		return logger
	}

	// Return general logger as fallback
	return ml.loggers[CategoryGeneral]
}

// WebAccess returns the web access logger
func (ml *MultiLogger) WebAccess() *zap.Logger {
	return ml.GetLogger(CategoryWebAccess)
}

// DownloadProgress returns the download progress logger
func (ml *MultiLogger) DownloadProgress() *zap.Logger {
	return ml.GetLogger(CategoryDownloadProgress)
}

// Queue returns the queue logger
func (ml *MultiLogger) Queue() *zap.Logger {
	return ml.GetLogger(CategoryQueue)
}

// Error returns the error logger
func (ml *MultiLogger) Error() *zap.Logger {
	return ml.GetLogger(CategoryError)
}

// General returns the general logger
func (ml *MultiLogger) General() *zap.Logger {
	return ml.GetLogger(CategoryGeneral)
}

// LogError logs an error to both the category logger and error.log
func (ml *MultiLogger) LogError(category LogCategory, msg string, fields ...zap.Field) {
	// Log to category logger
	ml.GetLogger(category).Error(msg, fields...)

	// Also log to error logger if not already error category
	if category != CategoryError {
		ml.Error().Error(msg, append(fields, zap.String("source_category", string(category)))...)
	}
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
	return ml.Sync()
}
