package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config represents logger configuration
type Config struct {
	Level      string // debug, info, warn, error
	Format     string // json, console
	OutputPath string // stdout, stderr, file path, or "auto" for date-based logs
	LogsDir    string // Directory for date-based log files (used when OutputPath is "auto")
}

// New creates a new logger based on configuration
func New(config Config) (*zap.Logger, error) {
	// Parse log level
	level, err := zapcore.ParseLevel(config.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	// Configure encoder
	var encoderConfig zapcore.EncoderConfig
	if config.Format == "json" {
		encoderConfig = zap.NewProductionEncoderConfig()
	} else {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Create encoder
	var encoder zapcore.Encoder
	if config.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Configure output
	var writer zapcore.WriteSyncer
	switch config.OutputPath {
	case "stdout", "":
		writer = zapcore.AddSync(os.Stdout)
	case "stderr":
		writer = zapcore.AddSync(os.Stderr)
	case "auto":
		// Use date-based log file in logs directory
		if config.LogsDir == "" {
			return nil, fmt.Errorf("logs_dir must be specified when output_path is 'auto'")
		}
		logPath := getDateBasedLogPath(config.LogsDir)
		file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		writer = zapcore.AddSync(file)
	default:
		file, err := os.OpenFile(config.OutputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		writer = zapcore.AddSync(file)
	}

	// Create core
	core := zapcore.NewCore(encoder, writer, level)

	// Create logger
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return logger, nil
}

// NewDefault creates a default logger for development
func NewDefault() *zap.Logger {
	logger, _ := New(Config{
		Level:      "info",
		Format:     "console",
		OutputPath: "stdout",
	})
	return logger
}

// NewProduction creates a production logger
func NewProduction() (*zap.Logger, error) {
	return New(Config{
		Level:      "info",
		Format:     "json",
		OutputPath: "stdout",
	})
}

// getDateBasedLogPath generates a log file path with current date (YYYYMMDD.log)
func getDateBasedLogPath(logsDir string) string {
	dateStr := time.Now().Format("20060102")
	return filepath.Join(logsDir, fmt.Sprintf("%s.log", dateStr))
}
