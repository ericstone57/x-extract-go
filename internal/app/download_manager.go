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
	activeCancels      sync.Map                         // downloadID -> context.CancelFunc for running downloads
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

// isDownloadAborted re-fetches a download and returns true if it was cancelled or
// already completed while waiting (e.g. while queued for a semaphore or between retries).
func (dm *DownloadManager) isDownloadAborted(id string) (bool, error) {
	latest, err := dm.repo.FindByID(id)
	if err != nil {
		return false, fmt.Errorf("failed to fetch download: %w", err)
	}
	return latest.Status == domain.StatusCancelled || latest.Status == domain.StatusCompleted, nil
}

// ProcessDownload processes a single download.
// The download is marked "processing" only after it acquires the platform semaphore,
// so the UI correctly shows "Pending" for downloads waiting behind a semaphore.
func (dm *DownloadManager) ProcessDownload(ctx context.Context, download *domain.Download) error {
	// Re-fetch to get latest status (may have been cancelled or completed before we started)
	if aborted, err := dm.isDownloadAborted(download.ID); err != nil {
		return err
	} else if aborted {
		dm.logger.Info("Download already finished before processing, skipping", zap.String("id", download.ID))
		return nil
	}

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

	// Check again after acquiring semaphore (may have been cancelled while waiting)
	if aborted, err := dm.isDownloadAborted(download.ID); err != nil {
		return err
	} else if aborted {
		dm.logger.Info("Download finished while waiting for semaphore, skipping", zap.String("id", download.ID))
		return nil
	}

	// Create a per-download cancellable context so CancelDownload can kill the subprocess.
	dlCtx, dlCancel := context.WithCancel(ctx)
	dm.activeCancels.Store(download.ID, dlCancel)
	defer func() {
		dlCancel()
		dm.activeCancels.Delete(download.ID)
	}()

	// Mark as processing now that we hold the semaphore and are about to run the tool.
	download.MarkProcessing()
	if err := dm.repo.Update(download); err != nil {
		dm.logger.Error("Failed to mark download as processing", zap.Error(err))
	}

	dm.logger.Info("Processing download",
		zap.String("id", download.ID),
		zap.String("url", download.URL),
		zap.String("platform", string(download.Platform)))

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
		// Check for cancellation before each attempt
		if aborted, err := dm.isDownloadAborted(download.ID); err != nil {
			return err
		} else if aborted {
			dm.logger.Info("Download cancelled, stopping retry loop", zap.String("id", download.ID))
			return nil
		}

		if attempt > 0 {
			dm.logger.Info("Retrying download",
				zap.String("id", download.ID),
				zap.Int("attempt", attempt),
				zap.Int("max_retries", dm.config.MaxRetries))

			// Wait before retry
			select {
			case <-time.After(dm.config.RetryDelay):
			case <-dlCtx.Done():
				return dlCtx.Err()
			}

			download.IncrementRetry()
			dm.repo.Update(download)
		}

		// Perform download — dlCtx cancellation kills the subprocess immediately.
		err := downloader.Download(dlCtx, download, nil)
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

		// If the context was cancelled, the subprocess was killed intentionally —
		// don't retry and don't overwrite the cancelled status in the DB.
		if dlCtx.Err() != nil {
			dm.logger.Info("Download subprocess killed by cancellation", zap.String("id", download.ID))
			return nil
		}

		lastErr = err
		dm.logger.Warn("Download attempt failed",
			zap.String("id", download.ID),
			zap.Int("attempt", attempt),
			zap.Error(err))
	}

	// All retries exhausted — only mark failed if not already cancelled.
	if aborted, _ := dm.isDownloadAborted(download.ID); !aborted {
		download.MarkFailed(lastErr)
		if err := dm.repo.Update(download); err != nil {
			dm.logger.Error("Failed to update download status", zap.Error(err))
		}
		dm.logger.Error("Download failed after retries",
			zap.String("id", download.ID),
			zap.String("url", download.URL),
			zap.Error(lastErr))
		dm.notifier.NotifyDownloadFailed(download.URL, download.Platform, lastErr)
	}
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

	// Kill the subprocess if it is actively running.
	if cancelFn, ok := dm.activeCancels.Load(id); ok {
		cancelFn.(context.CancelFunc)()
	}

	dm.logger.Info("Download cancelled", zap.String("id", id))
	return nil
}

// RetryDownload retries a failed or cancelled download
func (dm *DownloadManager) RetryDownload(ctx context.Context, id string) error {
	download, err := dm.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("download not found: %w", err)
	}
	if download == nil {
		return fmt.Errorf("download not found: %s", id)
	}

	// Allow retry for failed or cancelled downloads
	if download.Status == domain.StatusQueued {
		return fmt.Errorf("download is already queued: %s", download.Status)
	}
	if download.Status == domain.StatusProcessing {
		return fmt.Errorf("download is currently processing: %s", download.Status)
	}
	if download.Status == domain.StatusCompleted {
		return fmt.Errorf("download is already completed: %s", download.Status)
	}

	// Reset download state
	download.Status = domain.StatusQueued
	download.RetryCount = 0
	download.ErrorMessage = ""
	download.StartedAt = nil
	download.CompletedAt = nil
	download.UpdatedAt = time.Now()

	if err := dm.repo.Update(download); err != nil {
		return fmt.Errorf("failed to update download: %w", err)
	}

	if dm.logger != nil {
		dm.logger.Info("Download queued for retry", zap.String("id", id))
	}
	return nil
}
