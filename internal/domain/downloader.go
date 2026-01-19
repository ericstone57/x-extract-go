package domain

// Downloader defines the interface for platform-specific downloaders
type Downloader interface {
	// Download downloads media from the given URL
	Download(download *Download) error

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

