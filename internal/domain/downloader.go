package domain

import "context"

// DownloadProgressCallback is called with progress updates during download
type DownloadProgressCallback func(output string, percent float64)

// Downloader defines the interface for platform-specific downloaders
type Downloader interface {
	// Download downloads media from the given URL.
	// ctx is cancelled when the download is cancelled; the implementation must
	// use exec.CommandContext so the subprocess is killed immediately.
	Download(ctx context.Context, download *Download, progressCallback DownloadProgressCallback) error

	// Platform returns the platform this downloader handles
	Platform() Platform

	// Validate validates if the downloader can handle the given URL
	Validate(url string) error
}

// DownloadResult represents the result of a download operation
type DownloadResult struct {
	FilePath string
	Metadata map[string]interface{}
	Error    error
}
