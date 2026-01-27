package domain

// DownloadProgressCallback is called with progress updates during download
type DownloadProgressCallback func(output string, percent float64)

// Downloader defines the interface for platform-specific downloaders
type Downloader interface {
	// Download downloads media from the given URL
	// progressCallback is called with each line of stdout/stderr and optional percent
	Download(download *Download, progressCallback DownloadProgressCallback) error

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
