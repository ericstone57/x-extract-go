package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yourusername/x-extract-go/internal/domain"
	"go.uber.org/zap"
)

// QueueManager manages the download queue
type QueueManager struct {
	repo         domain.DownloadRepository
	downloadMgr  *DownloadManager
	config       *domain.QueueConfig
	logger       *zap.Logger
	mu           sync.RWMutex
	running      bool
	stopChan     chan struct{}
	workerWg     sync.WaitGroup
}

// NewQueueManager creates a new queue manager
func NewQueueManager(
	repo domain.DownloadRepository,
	downloadMgr *DownloadManager,
	config *domain.QueueConfig,
	logger *zap.Logger,
) *QueueManager {
	return &QueueManager{
		repo:        repo,
		downloadMgr: downloadMgr,
		config:      config,
		logger:      logger,
		stopChan:    make(chan struct{}),
	}
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

	qm.logger.Info("Starting queue manager")

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

	qm.logger.Info("Stopping queue manager")
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

	qm.logger.Info("Download added to queue",
		zap.String("id", download.ID),
		zap.String("url", url),
		zap.String("platform", string(platform)),
		zap.String("mode", string(mode)))

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

// processQueue processes the download queue
func (qm *QueueManager) processQueue(ctx context.Context) {
	defer qm.workerWg.Done()

	ticker := time.NewTicker(qm.config.CheckInterval)
	defer ticker.Stop()

	emptyStartTime := time.Time{}

	for {
		select {
		case <-ctx.Done():
			qm.logger.Info("Queue processor stopped by context")
			return
		case <-qm.stopChan:
			qm.logger.Info("Queue processor stopped")
			return
		case <-ticker.C:
			// Get pending downloads
			pending, err := qm.repo.FindPending()
			if err != nil {
				qm.logger.Error("Failed to fetch pending downloads", zap.Error(err))
				continue
			}

			if len(pending) == 0 {
				// Queue is empty
				if emptyStartTime.IsZero() {
					emptyStartTime = time.Now()
					qm.logger.Debug("Queue is empty, waiting...")
				} else if qm.config.AutoExitOnEmpty && time.Since(emptyStartTime) > qm.config.EmptyWaitTime {
					qm.logger.Info("Queue empty for configured duration, exiting")
					return
				}
				continue
			}

			// Reset empty timer
			emptyStartTime = time.Time{}

			// Process downloads
			for _, download := range pending {
				if err := qm.downloadMgr.ProcessDownload(ctx, download); err != nil {
					qm.logger.Error("Failed to process download",
						zap.String("id", download.ID),
						zap.Error(err))
				}
			}
		}
	}
}

