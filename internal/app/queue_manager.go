package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/yourusername/x-extract-go/internal/domain"
	"github.com/yourusername/x-extract-go/pkg/logger"
)

// QueueManager manages the download queue
type QueueManager struct {
	repo        domain.DownloadRepository
	downloadMgr *DownloadManager
	config      *domain.QueueConfig
	multiLogger *logger.MultiLogger
	mu          sync.RWMutex
	running     bool
	stopChan    chan struct{}
	exitChan    chan struct{} // Signals when auto-exit is triggered
	workerWg    sync.WaitGroup
}

// NewQueueManager creates a new queue manager
func NewQueueManager(
	repo domain.DownloadRepository,
	downloadMgr *DownloadManager,
	config *domain.QueueConfig,
	multiLogger *logger.MultiLogger,
) *QueueManager {
	return &QueueManager{
		repo:        repo,
		downloadMgr: downloadMgr,
		config:      config,
		multiLogger: multiLogger,
		stopChan:    make(chan struct{}),
		exitChan:    make(chan struct{}),
	}
}

// WaitForExit returns a channel that is closed when auto-exit is triggered
func (qm *QueueManager) WaitForExit() <-chan struct{} {
	return qm.exitChan
}

// Start starts the queue processor
func (qm *QueueManager) Start(ctx context.Context) error {
	qm.mu.Lock()
	if qm.running {
		qm.mu.Unlock()
		return fmt.Errorf("queue manager already running")
	}
	qm.running = true
	qm.mu.Unlock()

	if qm.multiLogger != nil {
		qm.multiLogger.LogQueueEvent("queue_started")
	}

	qm.workerWg.Add(1)
	go qm.processQueue(ctx)

	return nil
}

// Stop stops the queue processor
func (qm *QueueManager) Stop() error {
	qm.mu.Lock()
	if !qm.running {
		qm.mu.Unlock()
		return fmt.Errorf("queue manager not running")
	}
	qm.running = false
	qm.mu.Unlock()

	if qm.multiLogger != nil {
		qm.multiLogger.LogQueueEvent("queue_stopped")
	}
	close(qm.stopChan)
	qm.workerWg.Wait()

	return nil
}

// IsRunning returns whether the queue manager is running
func (qm *QueueManager) IsRunning() bool {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	return qm.running
}

// AddDownload adds a download to the queue
func (qm *QueueManager) AddDownload(url string, platform domain.Platform, mode domain.DownloadMode) (*domain.Download, error) {
	// Validate platform
	if !domain.ValidatePlatform(platform) {
		return nil, fmt.Errorf("invalid platform: %s", platform)
	}

	// Validate mode
	if !domain.ValidateMode(mode) {
		return nil, fmt.Errorf("invalid mode: %s", mode)
	}

	// Create download
	download := domain.NewDownload(url, platform, mode)

	// Save to repository
	if err := qm.repo.Create(download); err != nil {
		return nil, fmt.Errorf("failed to create download: %w", err)
	}

	// Log queue event
	if qm.multiLogger != nil {
		qm.multiLogger.LogQueueEvent("download_added",
			zap.String("id", download.ID),
			zap.String("url", url),
			zap.String("platform", string(platform)),
			zap.String("mode", string(mode)))
	}

	return download, nil
}

// GetDownload retrieves a download by ID
func (qm *QueueManager) GetDownload(id string) (*domain.Download, error) {
	return qm.repo.FindByID(id)
}

// ListDownloads lists all downloads with optional filters
func (qm *QueueManager) ListDownloads(filters map[string]interface{}) ([]*domain.Download, error) {
	return qm.repo.FindAll(filters)
}

// GetStats returns queue statistics
func (qm *QueueManager) GetStats() (*domain.DownloadStats, error) {
	return qm.repo.GetStats()
}

// DeleteDownload deletes a download by ID
func (qm *QueueManager) DeleteDownload(id string) error {
	// Check if download exists
	download, err := qm.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("download not found: %w", err)
	}

	// Don't allow deletion of processing downloads
	if download.Status == domain.StatusProcessing {
		return fmt.Errorf("cannot delete download in processing state")
	}

	if err := qm.repo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete download: %w", err)
	}

	qm.multiLogger.LogQueueEvent("download_deleted", zap.String("id", id))
	return nil
}

// processQueue processes the download queue
func (qm *QueueManager) processQueue(ctx context.Context) {
	defer qm.workerWg.Done()

	ticker := time.NewTicker(qm.config.CheckInterval)
	defer ticker.Stop()

	emptyStartTime := time.Time{}

	for {
		select {
		case <-ctx.Done():
			if qm.multiLogger != nil {
				qm.multiLogger.LogQueueEvent("queue_processor_stopped",
					zap.String("reason", "context_cancelled"))
			}
			return
		case <-qm.stopChan:
			if qm.multiLogger != nil {
				qm.multiLogger.LogQueueEvent("queue_processor_stopped",
					zap.String("reason", "stop_signal"))
			}
			return
		case <-ticker.C:
			// Get pending downloads
			pending, err := qm.repo.FindPending()
			if err != nil {
				if qm.multiLogger != nil {
					qm.multiLogger.LogAppError("Failed to fetch pending downloads", zap.Error(err))
				}
				continue
			}

			// Check if there are any active downloads (pending + processing)
			// This is important for parallel downloads - we need to wait for all to complete
			activeCount, err := qm.repo.CountActive()
			if err != nil {
				if qm.multiLogger != nil {
					qm.multiLogger.LogAppError("Failed to count active downloads", zap.Error(err))
				}
				continue
			}

			if len(pending) == 0 && activeCount == 0 {
				// Queue is truly empty (no pending and no processing)
				if emptyStartTime.IsZero() {
					emptyStartTime = time.Now()
					if qm.multiLogger != nil {
						qm.multiLogger.LogQueueEvent("queue_empty")
					}
				} else if qm.config.AutoExitOnEmpty && time.Since(emptyStartTime) > qm.config.EmptyWaitTime {
					if qm.multiLogger != nil {
						qm.multiLogger.LogQueueEvent("queue_auto_exit",
							zap.String("reason", "empty_timeout"),
							zap.Duration("wait_time", qm.config.EmptyWaitTime))
					}
					// Signal auto-exit to main server
					close(qm.exitChan)
					return
				}
				continue
			}

			// Reset empty timer if there are active downloads
			emptyStartTime = time.Time{}

			// Process downloads in parallel using goroutines
			for _, download := range pending {
				// Capture the download variable for the goroutine
				dl := download

				// Log download start
				if qm.multiLogger != nil {
					qm.multiLogger.LogQueueEvent("download_started",
						zap.String("id", dl.ID),
						zap.String("url", dl.URL),
						zap.String("platform", string(dl.Platform)))
				}

				// Spawn a goroutine for each download
				// The semaphore in DownloadManager controls actual concurrency
				qm.workerWg.Add(1)
				go func(download *domain.Download) {
					defer qm.workerWg.Done()

					if err := qm.downloadMgr.ProcessDownload(ctx, download); err != nil {
						// Log download failure
						if qm.multiLogger != nil {
							qm.multiLogger.LogQueueEvent("download_failed",
								zap.String("id", download.ID),
								zap.Error(err))
							qm.multiLogger.LogAppError("Failed to process download",
								zap.String("id", download.ID),
								zap.Error(err))
						}
					} else {
						// Log download completion
						if qm.multiLogger != nil {
							qm.multiLogger.LogQueueEvent("download_completed",
								zap.String("id", download.ID),
								zap.String("status", string(download.Status)),
								zap.String("file_path", download.FilePath))
						}
					}
				}(dl)
			}
		}
	}
}
