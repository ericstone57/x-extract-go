package logger

import (
	"go.uber.org/zap"
)

// LoggerAdapter provides a unified interface for the multi-logger system
type LoggerAdapter struct {
	multiLogger *MultiLogger
}

// NewLoggerAdapter creates a new logger adapter
func NewLoggerAdapter(multiLogger *MultiLogger) *LoggerAdapter {
	return &LoggerAdapter{
		multiLogger: multiLogger,
	}
}

// Queue returns the queue logger (JSON format)
func (la *LoggerAdapter) Queue() *zap.Logger {
	return la.multiLogger.Queue()
}

// Error returns the error logger (JSON format)
func (la *LoggerAdapter) Error() *zap.Logger {
	return la.multiLogger.Error()
}

// Sync flushes all loggers
func (la *LoggerAdapter) Sync() error {
	return la.multiLogger.Sync()
}

// GetMultiLogger returns the underlying multi-logger
func (la *LoggerAdapter) GetMultiLogger() *MultiLogger {
	return la.multiLogger
}

// GetSingleLogger returns a single logger for backward compatibility
// Uses the error logger as the general-purpose logger
func (la *LoggerAdapter) GetSingleLogger() *zap.Logger {
	return la.multiLogger.Error()
}

// GetLogsDir returns the logs directory path
func (la *LoggerAdapter) GetLogsDir() string {
	return la.multiLogger.GetLogsDir()
}
