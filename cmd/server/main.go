package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/yourusername/x-extract-go/api"
	"github.com/yourusername/x-extract-go/internal/app"
	"github.com/yourusername/x-extract-go/internal/domain"
	"github.com/yourusername/x-extract-go/internal/infrastructure"
	"github.com/yourusername/x-extract-go/pkg/logger"
)

var serverMode = flag.Bool("server-mode", false, "Internal flag: run in server mode (called by daemon)")

func main() {
	flag.Parse()

	// If not in server mode, run as daemon
	if !*serverMode {
		startAsDaemon()
		return
	}

	// Run as server (called by daemon)
	runServer()
}

// startAsDaemon forks the current process and runs the server in background
func startAsDaemon() {
	// Get the executable path
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get executable path: %v\n", err)
		os.Exit(1)
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "/"
	}

	// Fork the process
	cmd := exec.Command(execPath, "-server-mode")
	cmd.Dir = cwd
	cmd.Env = os.Environ()

	// Detach from parent process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session
	}

	// Redirect output to /dev/null
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open /dev/null: %v\n", err)
		os.Exit(1)
	}
	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull

	// Start the child process
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start daemon: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Server started as daemon (PID: %d)\n", cmd.Process.Pid)
	os.Exit(0)
}

func runServer() {
	// Load configuration from default location (~/.config/x-extract-go/config.yaml)
	config, err := app.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Create logs directory
	if err := os.MkdirAll(config.Download.LogsDir(), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logs directory: %v\n", err)
		os.Exit(1)
	}

	// Initialize multi-logger (3 categories: download, queue, error)
	multiLog, err := logger.NewMultiLogger(logger.MultiLoggerConfig{
		Level:   config.Logging.Level,
		LogsDir: config.Download.LogsDir(),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer multiLog.Close()

	logAdapter := logger.NewLoggerAdapter(multiLog)
	log := logAdapter.GetSingleLogger()

	log.Info("Starting X-Extract server",
		zap.String("version", "1.0.0"),
		zap.String("host", config.Server.Host),
		zap.Int("port", config.Server.Port),
		zap.Bool("telegram_takeout", config.Telegram.Takeout),
		zap.String("telegram_profile", config.Telegram.Profile))

	// Create directories
	if err := createDirectories(config); err != nil {
		log.Fatal("Failed to create directories", zap.Error(err))
	}

	// Migrate old directory structure if needed
	if err := app.MigrateOldStructure(config); err != nil {
		log.Warn("Failed to migrate old structure", zap.Error(err))
	}

	// Initialize repository
	repo, err := infrastructure.NewSQLiteDownloadRepository(config.Queue.DatabasePath)
	if err != nil {
		log.Fatal("Failed to initialize repository", zap.Error(err))
	}
	defer repo.Close()

	// Initialize notification service
	notifier := infrastructure.NewNotificationService(&config.Notification, log)

	// Get logs directory for download output
	logsDir := config.Download.LogsDir()

	// Initialize downloaders with logs directory and event logger
	telegramDownloader := infrastructure.NewTelegramDownloader(
		&config.Telegram,
		config.Download.IncomingDir(),
		config.Download.CompletedDir(),
		logsDir,
		multiLog,
	)
	// Set channel repository for channel name lookups
	telegramDownloader.SetChannelRepository(repo)
	// Set message cache repository for caching message metadata
	telegramDownloader.SetMessageCacheRepository(repo)

	downloaders := map[domain.Platform]domain.Downloader{
		domain.PlatformX: infrastructure.NewTwitterDownloader(
			&config.Twitter,
			config.Download.IncomingDir(),
			config.Download.CompletedDir(),
			logsDir,
			multiLog,
		),
		domain.PlatformTelegram: telegramDownloader,
	}

	// Initialize download manager
	downloadMgr := app.NewDownloadManager(repo, downloaders, notifier, &config.Download, log)

	// Initialize queue manager
	queueMgr := app.NewQueueManager(repo, downloadMgr, &config.Queue, multiLog)

	// Start queue manager
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if config.Download.AutoStartWorkers {
		if err := queueMgr.Start(ctx); err != nil {
			log.Fatal("Failed to start queue manager", zap.Error(err))
		}
	}

	// Setup HTTP router
	router := api.SetupRouterWithMultiLogger(queueMgr, downloadMgr, logAdapter, config.Download.LogsDir())

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Info("HTTP server listening", zap.String("addr", addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal OR auto-exit from queue manager
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Info("Received shutdown signal")
	case <-queueMgr.WaitForExit():
		log.Info("Queue manager triggered auto-exit (all downloads complete)")
	}

	log.Info("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop queue manager
	if err := queueMgr.Stop(); err != nil {
		log.Error("Error stopping queue manager", zap.Error(err))
	}

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited")
}

func createDirectories(config *domain.Config) error {
	// Create all required subdirectories
	dirs := []string{
		config.Download.BaseDir,
		config.Download.CompletedDir(),
		config.Download.IncomingDir(),
		config.Download.CookiesDir(),
		config.Download.LogsDir(),
		config.Download.ConfigDir(),
		filepath.Join(config.Download.CookiesDir(), "x.com"),
		filepath.Join(config.Download.CookiesDir(), "telegram"),
		config.Telegram.StoragePath,
	}

	for _, dir := range dirs {
		// Skip empty paths (may be optional paths not configured)
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
