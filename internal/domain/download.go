package domain

import (
	"time"

	"github.com/google/uuid"
)

// DownloadStatus represents the current status of a download
type DownloadStatus string

const (
	StatusQueued     DownloadStatus = "queued"
	StatusProcessing DownloadStatus = "processing"
	StatusCompleted  DownloadStatus = "completed"
	StatusFailed     DownloadStatus = "failed"
	StatusCancelled  DownloadStatus = "cancelled"
)

// Platform represents the source platform for downloads
type Platform string

const (
	PlatformX        Platform = "x"        // X/Twitter
	PlatformTelegram Platform = "telegram" // Telegram
)

// DownloadMode represents the download mode for Telegram
type DownloadMode string

const (
	ModeDefault DownloadMode = "default" // Use config settings
	ModeSingle  DownloadMode = "single"  // Single file download
	ModeGroup   DownloadMode = "group"   // Group download
)

// Download represents a download task
type Download struct {
	ID           string         `json:"id" gorm:"primaryKey"`
	URL          string         `json:"url" gorm:"not null"`
	Platform     Platform       `json:"platform" gorm:"not null"`
	Status       DownloadStatus `json:"status" gorm:"not null;index"`
	Mode         DownloadMode   `json:"mode" gorm:"default:default"`
	Priority     int            `json:"priority" gorm:"default:0;index"`
	RetryCount   int            `json:"retry_count" gorm:"default:0"`
	ErrorMessage string         `json:"error_message,omitempty"`
	FilePath     string         `json:"file_path,omitempty"`
	Metadata     string         `json:"metadata,omitempty" gorm:"type:text"`    // JSON metadata
	ProcessLog   string         `json:"process_log,omitempty" gorm:"type:text"` // Process output log (yt-dlp/tdl)
	CreatedAt    time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	StartedAt    *time.Time     `json:"started_at,omitempty"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
}

// NewDownload creates a new download task
func NewDownload(url string, platform Platform, mode DownloadMode) *Download {
	return &Download{
		ID:         uuid.New().String(),
		URL:        url,
		Platform:   platform,
		Status:     StatusQueued,
		Mode:       mode,
		Priority:   0,
		RetryCount: 0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// MarkProcessing marks the download as processing
func (d *Download) MarkProcessing() {
	d.Status = StatusProcessing
	now := time.Now()
	d.StartedAt = &now
	d.UpdatedAt = now
}

// MarkCompleted marks the download as completed
func (d *Download) MarkCompleted(filePath string) {
	d.Status = StatusCompleted
	d.FilePath = filePath
	now := time.Now()
	d.CompletedAt = &now
	d.UpdatedAt = now
}

// MarkFailed marks the download as failed
func (d *Download) MarkFailed(err error) {
	d.Status = StatusFailed
	d.ErrorMessage = err.Error()
	d.UpdatedAt = time.Now()
}

// IncrementRetry increments the retry count
func (d *Download) IncrementRetry() {
	d.RetryCount++
	d.UpdatedAt = time.Now()
}

// CanRetry checks if the download can be retried
func (d *Download) CanRetry(maxRetries int) bool {
	return d.RetryCount < maxRetries && d.Status == StatusFailed
}

// IsTerminal checks if the download is in a terminal state
func (d *Download) IsTerminal() bool {
	return d.Status == StatusCompleted || d.Status == StatusCancelled
}

// IsPending checks if the download is pending
func (d *Download) IsPending() bool {
	return d.Status == StatusQueued
}

// IsProcessing checks if the download is currently processing
func (d *Download) IsProcessing() bool {
	return d.Status == StatusProcessing
}

// DetectPlatform detects the platform from a URL
func DetectPlatform(url string) Platform {
	// Check for X/Twitter URLs
	if len(url) >= 13 {
		if url[:13] == "https://x.com" {
			return PlatformX
		}
	}
	if len(url) >= 19 {
		if url[:19] == "https://twitter.com" {
			return PlatformX
		}
	}
	// Check for Telegram URLs
	if len(url) >= 13 {
		if url[:13] == "https://t.me/" {
			return PlatformTelegram
		}
	}
	return ""
}

// ValidatePlatform checks if a platform is valid
func ValidatePlatform(platform Platform) bool {
	return platform == PlatformX || platform == PlatformTelegram
}

// ValidateMode checks if a download mode is valid
func ValidateMode(mode DownloadMode) bool {
	return mode == ModeDefault || mode == ModeSingle || mode == ModeGroup
}
