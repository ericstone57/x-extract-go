package domain

import (
	"strings"
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
	PlatformGallery  Platform = "gallery"  // Gallery-dl (catch-all for 100+ sites)
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

// PlatformURLPrefixes maps URL prefixes to their corresponding platform.
// Used by DetectPlatform to identify which platform a URL belongs to.
// To add a new platform, add its URL prefix(es) here.
var PlatformURLPrefixes = map[string]Platform{
	"https://x.com":       PlatformX,
	"https://twitter.com": PlatformX,
	"https://t.me":        PlatformTelegram,
}

// ValidPlatforms is the set of all valid platforms.
// To add a new platform, add it here and define its constant above.
var ValidPlatforms = map[Platform]bool{
	PlatformX:        true,
	PlatformTelegram: true,
	PlatformGallery:  true,
}

// DetectPlatform detects the platform from a URL using the PlatformURLPrefixes registry.
// If no specific platform matches, any HTTP/HTTPS URL falls back to gallery-dl.
func DetectPlatform(url string) Platform {
	for prefix, platform := range PlatformURLPrefixes {
		if strings.HasPrefix(url, prefix) {
			return platform
		}
	}
	// Fallback: any HTTP/HTTPS URL goes to gallery-dl
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return PlatformGallery
	}
	return ""
}

// ValidatePlatform checks if a platform is valid using the ValidPlatforms registry.
func ValidatePlatform(platform Platform) bool {
	return ValidPlatforms[platform]
}

// ValidateMode checks if a download mode is valid
func ValidateMode(mode DownloadMode) bool {
	return mode == ModeDefault || mode == ModeSingle || mode == ModeGroup
}

// MetadataKeyGalleryFilters is the JSON key used to store gallery-dl filter options
// in Download.Metadata. Both queue_manager (writer) and GalleryDownloader (reader) use this.
const MetadataKeyGalleryFilters = "gallerydl_filters"

// XURLType represents the type of X/Twitter URL
type XURLType string

const (
	XURLTypeSingle   XURLType = "single"
	XURLTypeTimeline XURLType = "timeline"
)

// DetectXURLType returns XURLTypeSingle for tweet URLs, XURLTypeTimeline for
// account/profile URLs, and "" for non-X URLs.
// Strips query string before checking path to avoid /status/ false positives.
func DetectXURLType(url string) XURLType {
	if !strings.HasPrefix(url, "https://x.com/") &&
		!strings.HasPrefix(url, "https://twitter.com/") {
		return ""
	}
	// Strip query string before checking path
	path := url
	if idx := strings.IndexByte(path, '?'); idx >= 0 {
		path = path[:idx]
	}
	if strings.Contains(path, "/status/") {
		return XURLTypeSingle
	}
	return XURLTypeTimeline
}
