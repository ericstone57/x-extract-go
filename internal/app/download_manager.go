package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yourusername/x-extract-go/internal/domain"
	"github.com/yourusername/x-extract-go/internal/infrastructure"
	"go.uber.org/zap"
)

// DownloadManager manages download operations
type DownloadManager struct {
	repo               domain.DownloadRepository
	downloaders        map[domain.Platform]domain.Downloader
	notifier           *infrastructure.NotificationService
	config             *domain.DownloadConfig
	logger             *zap.Logger
	platformSemaphores map[domain.Platform]chan struct{} // Per-platform semaphores (limit=1 each)
	mu                 sync.RWMutex
}

// NewDownloadManager creates a new download manager
func NewDownloadManager(
	repo domain.DownloadRepository,
	downloaders map[domain.Platform]domain.Downloader,
	notifier *infrastructure.NotificationService,
	config *domain.DownloadConfig,
	logger *zap.Logger,
) *DownloadManager {
	// Initialize per-platform semaphores with limit=1 for each platform
	// This allows different platforms to download in parallel,
	// while serializing downloads within the same platform
	platformSemaphores := make(map[domain.Platform]chan struct{})
	for platform := range downloaders {
		platformSemaphores[platform] = make(chan struct{}, 1)
	}

	return &DownloadManager{
		repo:               repo,
		downloaders:        downloaders,
		notifier:           notifier,
		config:             config,
		logger:             logger,
		platformSemaphores: platformSemaphores,
	}
}

// ProcessDownload processes a single download
func (dm *DownloadManager) ProcessDownload(ctx context.Context, download *domain.Download) error {
	// Get platform-specific semaphore
	// This allows different platforms to download in parallel,
	// while serializing downloads within the same platform
	dm.mu.RLock()
	platformSem, ok := dm.platformSemaphores[download.Platform]
	dm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no semaphore for platform: %s", download.Platform)
	}

	// Acquire platform-specific semaphore
	select {
	case platformSem <- struct{}{}:
		defer func() { <-platformSem }()
	case <-ctx.Done():
		return ctx.Err()
	}

	dm.logger.Info("Processing download",
		zap.String("id", download.ID),
		zap.String("url", download.URL),
		zap.String("platform", string(download.Platform)))

	// Mark as processing
	download.MarkProcessing()
	if err := dm.repo.Update(download); err != nil {
		return fmt.Errorf("failed to update download status: %w", err)
	}

	// Send notification
	dm.notifier.NotifyDownloadStarted(download.URL, download.Platform)

	// Get appropriate downloader
	downloader, ok := dm.downloaders[download.Platform]
	if !ok {
		err := fmt.Errorf("no downloader for platform: %s", download.Platform)
		download.MarkFailed(err)
		dm.repo.Update(download)
		dm.notifier.NotifyDownloadFailed(download.URL, download.Platform, err)
		return err
	}

	// Attempt download with retries
	var lastErr error
	for attempt := 0; attempt <= dm.config.MaxRetries; attempt++ {
		if attempt > 0 {
			dm.logger.Info("Retrying download",
				zap.String("id", download.ID),
				zap.Int("attempt", attempt),
				zap.Int("max_retries", dm.config.MaxRetries))

			// Wait before retry
			select {
			case <-time.After(dm.config.RetryDelay):
			case <-ctx.Done():
				return ctx.Err()
			}

			download.IncrementRetry()
			dm.repo.Update(download)
		}

		// Perform download
		err := downloader.Download(download, nil)
		if err == nil {
			// Success
			download.MarkCompleted(download.FilePath)
			if err := dm.repo.Update(download); err != nil {
				dm.logger.Error("Failed to update download status", zap.Error(err))
			}

			dm.logger.Info("Download completed",
				zap.String("id", download.ID),
				zap.String("url", download.URL),
				zap.String("file", download.FilePath))

			dm.notifier.NotifyDownloadCompleted(download.URL, download.Platform)
			return nil
		}

		lastErr = err
		dm.logger.Warn("Download attempt failed",
			zap.String("id", download.ID),
			zap.Int("attempt", attempt),
			zap.Error(err))
	}

	// All retries exhausted
	download.MarkFailed(lastErr)
	if err := dm.repo.Update(download); err != nil {
		dm.logger.Error("Failed to update download status", zap.Error(err))
	}

	dm.logger.Error("Download failed after retries",
		zap.String("id", download.ID),
		zap.String("url", download.URL),
		zap.Error(lastErr))

	dm.notifier.NotifyDownloadFailed(download.URL, download.Platform, lastErr)
	return lastErr
}

// CancelDownload cancels a download
func (dm *DownloadManager) CancelDownload(id string) error {
	download, err := dm.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("download not found: %w", err)
	}

	if download.IsTerminal() {
		return fmt.Errorf("download already in terminal state: %s", download.Status)
	}

	download.Status = domain.StatusCancelled
	download.UpdatedAt = time.Now()

	if err := dm.repo.Update(download); err != nil {
		return fmt.Errorf("failed to update download: %w", err)
	}

	dm.logger.Info("Download cancelled", zap.String("id", id))
	return nil
}

// RetryDownload retries a failed download
func (dm *DownloadManager) RetryDownload(ctx context.Context, id string) error {
	download, err := dm.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("download not found: %w", err)
	}

	if download.Status != domain.StatusFailed {
		return fmt.Errorf("download is not in failed state: %s", download.Status)
	}

	// Reset download state
	download.Status = domain.StatusQueued
	download.RetryCount = 0
	download.ErrorMessage = ""
	download.UpdatedAt = time.Now()

	if err := dm.repo.Update(download); err != nil {
		return fmt.Errorf("failed to update download: %w", err)
	}

	dm.logger.Info("Download queued for retry", zap.String("id", id))
	return nil
}
