package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/yourusername/x-extract-go/internal/domain"
	"github.com/yourusername/x-extract-go/pkg/logger"
)

// IsDockerMode returns true if running in Docker mode
func IsDockerMode() bool {
	return os.Getenv("DOCKER_MODE") == "1"
}

// QueueManager manages the download queue
type QueueManager struct {
	repo           domain.DownloadRepository
	downloadMgr    *DownloadManager
	config         *domain.QueueConfig
	multiLogger    *logger.MultiLogger
	completedDir   string // Path to completed downloads directory for file-based dedup
	mu             sync.RWMutex
	running        bool
	stopChan       chan struct{}
	exitChan       chan struct{} // Signals when auto-exit is triggered
	workerWg       sync.WaitGroup
	processingURLs sync.Map   // In-memory guard: URL -> bool, prevents double-dispatch
	addMu          sync.Mutex // Serializes AddDownload calls for atomic duplicate check+create
}

// NewQueueManager creates a new queue manager
func NewQueueManager(
	repo domain.DownloadRepository,
	downloadMgr *DownloadManager,
	config *domain.QueueConfig,
	multiLogger *logger.MultiLogger,
	completedDir string,
) *QueueManager {
	return &QueueManager{
		repo:         repo,
		downloadMgr:  downloadMgr,
		config:       config,
		multiLogger:  multiLogger,
		completedDir: completedDir,
		stopChan:     make(chan struct{}),
		exitChan:     make(chan struct{}),
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

	// Reset any downloads that were stuck in processing state (server was killed)
	if err := qm.resetOrphanedProcessing(); err != nil {
		if qm.multiLogger != nil {
			qm.multiLogger.LogAppError("Failed to reset orphaned processing downloads", zap.Error(err))
		}
	}

	if qm.multiLogger != nil {
		qm.multiLogger.LogQueueEvent("queue_started")
	}

	qm.workerWg.Add(1)
	go qm.processQueue(ctx)

	return nil
}

// resetOrphanedProcessing resets downloads that are stuck in processing state
func (qm *QueueManager) resetOrphanedProcessing() error {
	count, err := qm.repo.ResetOrphanedProcessing()
	if err != nil {
		return err
	}
	if count > 0 {
		if qm.multiLogger != nil {
			qm.multiLogger.LogQueueEvent("orphaned_processing_reset",
				zap.Int64("count", count))
		}
	}
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
func (qm *QueueManager) AddDownload(url string, platform domain.Platform, mode domain.DownloadMode, filters string) (*domain.Download, error) {
	// Validate platform
	if !domain.ValidatePlatform(platform) {
		return nil, fmt.Errorf("invalid platform: %s", platform)
	}

	// Validate mode
	if !domain.ValidateMode(mode) {
		return nil, fmt.Errorf("invalid mode: %s", mode)
	}

	// Serialize duplicate check + create to prevent TOCTOU race condition
	// where concurrent AddDownload calls for the same URL both pass the check
	qm.addMu.Lock()
	defer qm.addMu.Unlock()

	// Check for existing download with the same URL that is still active
	// (queued, processing)
	// Note: We do NOT include StatusCompleted here because:
	// 1. If the file exists, user can re-request it via retry
	// 2. If the file is missing, we should allow re-downloading
	activeStatuses := []domain.DownloadStatus{
		domain.StatusQueued,
		domain.StatusProcessing,
	}
	existing, err := qm.repo.FindByURL(url, activeStatuses)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing download: %w", err)
	}
	if existing != nil {
		if qm.multiLogger != nil {
			qm.multiLogger.LogQueueEvent("download_duplicate_skipped",
				zap.String("existing_id", existing.ID),
				zap.String("url", url),
				zap.String("status", string(existing.Status)))
		}
		return existing, nil
	}

	// Also check for completed downloads - if file exists, return existing
	// If file is missing, allow re-downloading
	completed, err := qm.repo.FindByURL(url, []domain.DownloadStatus{domain.StatusCompleted})
	if err != nil {
		return nil, fmt.Errorf("failed to check for completed download: %w", err)
	}
	if completed != nil {
		// Check if file exists on disk
		if completed.FilePath != "" {
			if _, statErr := os.Stat(completed.FilePath); statErr == nil {
				// File exists, return existing completed download
				if qm.multiLogger != nil {
					qm.multiLogger.LogQueueEvent("download_already_completed",
						zap.String("existing_id", completed.ID),
						zap.String("url", url),
						zap.String("file_path", completed.FilePath))
				}
				return completed, nil
			}
		}
		// File doesn't exist, proceed with new download
		if qm.multiLogger != nil {
			qm.multiLogger.LogQueueEvent("download_file_missing",
				zap.String("download_id", completed.ID),
				zap.String("url", url),
				zap.String("file_path", completed.FilePath))
		}
	}

	// Scan completed directory for files matching this URL's content ID
	// This catches cases where DB record is missing/incomplete but files exist on disk
	if foundFile := qm.scanCompletedDirForURL(url, platform); foundFile != "" {
		if qm.multiLogger != nil {
			qm.multiLogger.LogQueueEvent("download_found_in_completed_dir",
				zap.String("url", url),
				zap.String("found_file", foundFile))
		}
		// Create a completed download record so future checks can use the DB
		download := domain.NewDownload(url, platform, mode)
		download.MarkCompleted(foundFile)
		if err := qm.repo.Create(download); err != nil {
			return nil, fmt.Errorf("failed to create completed download record: %w", err)
		}
		return download, nil
	}

	// Create download
	download := domain.NewDownload(url, platform, mode)

	// Encode gallery-dl filters into Metadata for use by GalleryDownloader
	if filters != "" {
		meta := map[string]interface{}{domain.MetadataKeyGalleryFilters: filters}
		data, _ := json.Marshal(meta)
		download.Metadata = string(data)
	}

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

			// Diagnostic: log queue state each tick when there's activity
			if qm.multiLogger != nil && (len(pending) > 0 || activeCount > 0) {
				qm.multiLogger.LogQueueEvent("queue_tick",
					zap.Int("pending_count", len(pending)),
					zap.Int64("active_count", activeCount))
			}

			if len(pending) == 0 && activeCount == 0 {
				// Queue is truly empty (no pending and no processing)
				if emptyStartTime.IsZero() {
					emptyStartTime = time.Now()
					if qm.multiLogger != nil {
						qm.multiLogger.LogQueueEvent("queue_empty")
					}
				} else if !IsDockerMode() && qm.config.AutoExitOnEmpty && time.Since(emptyStartTime) > qm.config.EmptyWaitTime {
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
				// Check if file already exists (might have been completed but status wasn't updated)
				if qm.skipIfFileExists(download) {
					continue
				}

				// Capture the download variable for the goroutine
				dl := download

				// In-memory dedup guard: skip if this URL is already being processed
				// This is a belt-and-suspenders check on top of the DB status update
				if _, alreadyProcessing := qm.processingURLs.LoadOrStore(dl.URL, true); alreadyProcessing {
					if qm.multiLogger != nil {
						qm.multiLogger.LogQueueEvent("download_dedup_skipped",
							zap.String("id", dl.ID),
							zap.String("url", dl.URL),
							zap.String("reason", "url_already_processing_in_memory"))
					}
					continue
				}

				// Mark as processing BEFORE spawning goroutine to prevent
				// the next tick from re-dispatching the same download (race condition fix)
				dl.MarkProcessing()
				if err := qm.repo.Update(dl); err != nil {
					qm.processingURLs.Delete(dl.URL) // Release in-memory guard on failure
					if qm.multiLogger != nil {
						qm.multiLogger.LogAppError("Failed to mark download as processing",
							zap.String("id", dl.ID),
							zap.Error(err))
					}
					continue
				}

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
					defer qm.processingURLs.Delete(download.URL) // Release in-memory guard when done

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

// skipIfFileExists checks if a download's file already exists and marks it as completed
// Returns true if the download was skipped
func (qm *QueueManager) skipIfFileExists(download *domain.Download) bool {
	// Check if we have a file path and it exists
	if download.FilePath != "" {
		if _, err := os.Stat(download.FilePath); err == nil {
			// File exists, mark as completed
			download.MarkCompleted(download.FilePath)
			if err := qm.repo.Update(download); err != nil {
				if qm.multiLogger != nil {
					qm.multiLogger.LogAppError("Failed to update download status",
						zap.String("id", download.ID),
						zap.Error(err))
				}
			}
			if qm.multiLogger != nil {
				qm.multiLogger.LogQueueEvent("download_skipped_file_exists",
					zap.String("id", download.ID),
					zap.String("file_path", download.FilePath))
			}
			return true
		}
	}
	return false
}

// scanCompletedDirForURL scans the completed directory for files matching a URL's content ID.
// This provides file-based deduplication as a fallback when DB records are missing/incomplete.
// Returns the path of the first matching file found, or empty string if none found.
func (qm *QueueManager) scanCompletedDirForURL(url string, platform domain.Platform) string {
	if qm.completedDir == "" {
		return ""
	}

	// Extract a content identifier from the URL based on platform
	contentID := extractContentIDFromURL(url, platform)
	if contentID == "" {
		return ""
	}

	// Scan completed directory for files matching the content ID pattern
	entries, err := os.ReadDir(qm.completedDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip metadata files
		if strings.HasSuffix(name, ".info.json") {
			continue
		}
		// Check if this is a media file containing our content ID
		nameWithoutExt := strings.TrimSuffix(name, filepath.Ext(name))
		if matchesContentID(nameWithoutExt, contentID, platform) {
			return filepath.Join(qm.completedDir, name)
		}
	}

	return ""
}

// extractContentIDFromURL extracts a unique content identifier from a download URL.
// For Twitter: the tweet ID (last numeric path segment)
// For Telegram: the message ID (last path segment)
func extractContentIDFromURL(url string, platform domain.Platform) string {
	// Remove protocol prefix
	cleaned := strings.TrimPrefix(url, "https://")
	cleaned = strings.TrimPrefix(cleaned, "http://")
	// Remove query params
	if idx := strings.Index(cleaned, "?"); idx > 0 {
		cleaned = cleaned[:idx]
	}
	// Remove trailing slash
	cleaned = strings.TrimRight(cleaned, "/")

	parts := strings.Split(cleaned, "/")

	switch platform {
	case domain.PlatformX:
		// URL: x.com/{user}/status/{tweet_id} or twitter.com/{user}/status/{tweet_id}
		if len(parts) >= 4 {
			return parts[len(parts)-1] // tweet_id
		}
	case domain.PlatformTelegram:
		// URL: t.me/{channel}/{message_id} or t.me/c/{channel_id}/{message_id}
		if len(parts) >= 3 {
			return parts[len(parts)-1] // message_id
		}
	}

	return ""
}

// matchesContentID checks if a filename (without extension) contains the content ID
// in the expected position for the given platform.
func matchesContentID(nameWithoutExt, contentID string, platform domain.Platform) bool {
	parts := strings.Split(nameWithoutExt, "_")

	switch platform {
	case domain.PlatformX:
		// Twitter filename format: {uploader_id}_{tweet_id}
		// The tweet_id should be the second part (or last part)
		if len(parts) >= 2 && parts[len(parts)-1] == contentID {
			return true
		}
	case domain.PlatformTelegram:
		// Telegram filename format: {channel_id}_{message_id}_{media_id}
		// The message_id should be the second part
		if len(parts) >= 2 && parts[1] == contentID {
			return true
		}
	}

	return false
}
