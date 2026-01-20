package logger

import (
	"go.uber.org/zap"
)

// LoggerAdapter provides a unified interface for both single and multi-logger
type LoggerAdapter struct {
	multiLogger *MultiLogger
	singleLogger *zap.Logger
	useMulti bool
}

// NewLoggerAdapter creates a new logger adapter
func NewLoggerAdapter(multiLogger *MultiLogger) *LoggerAdapter {
	return &LoggerAdapter{
		multiLogger: multiLogger,
		useMulti: true,
	}
}

// NewSingleLoggerAdapter creates an adapter for a single logger (backward compatibility)
func NewSingleLoggerAdapter(logger *zap.Logger) *LoggerAdapter {
	return &LoggerAdapter{
		singleLogger: logger,
		useMulti: false,
	}
}

// WebAccess returns the web access logger
func (la *LoggerAdapter) WebAccess() *zap.Logger {
	if la.useMulti {
		return la.multiLogger.WebAccess()
	}
	return la.singleLogger
}

// DownloadProgress returns the download progress logger
func (la *LoggerAdapter) DownloadProgress() *zap.Logger {
	if la.useMulti {
		return la.multiLogger.DownloadProgress()
	}
	return la.singleLogger
}

// Queue returns the queue logger
func (la *LoggerAdapter) Queue() *zap.Logger {
	if la.useMulti {
		return la.multiLogger.Queue()
	}
	return la.singleLogger
}

// Error returns the error logger
func (la *LoggerAdapter) Error() *zap.Logger {
	if la.useMulti {
		return la.multiLogger.Error()
	}
	return la.singleLogger
}

// General returns the general logger
func (la *LoggerAdapter) General() *zap.Logger {
	if la.useMulti {
		return la.multiLogger.General()
	}
	return la.singleLogger
}

// LogError logs an error to both category and error logs
func (la *LoggerAdapter) LogError(category LogCategory, msg string, fields ...zap.Field) {
	if la.useMulti {
		la.multiLogger.LogError(category, msg, fields...)
	} else {
		la.singleLogger.Error(msg, fields...)
	}
}

// Sync flushes all loggers
func (la *LoggerAdapter) Sync() error {
	if la.useMulti {
		return la.multiLogger.Sync()
	}
	return la.singleLogger.Sync()
}

// GetMultiLogger returns the underlying multi-logger (if available)
func (la *LoggerAdapter) GetMultiLogger() *MultiLogger {
	return la.multiLogger
}

// GetSingleLogger returns a single logger for backward compatibility
func (la *LoggerAdapter) GetSingleLogger() *zap.Logger {
	if la.useMulti {
		return la.multiLogger.General()
	}
	return la.singleLogger
}

