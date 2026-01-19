package domain

// DownloadRepository defines the interface for download persistence
type DownloadRepository interface {
	// Create creates a new download
	Create(download *Download) error

	// Update updates an existing download
	Update(download *Download) error

	// Delete deletes a download by ID
	Delete(id string) error

	// FindByID finds a download by ID
	FindByID(id string) (*Download, error)

	// FindByStatus finds downloads by status
	FindByStatus(status DownloadStatus) ([]*Download, error)

	// FindPending finds all pending downloads ordered by priority and creation time
	FindPending() ([]*Download, error)

	// FindAll finds all downloads with optional filters
	FindAll(filters map[string]interface{}) ([]*Download, error)

	// Count returns the total number of downloads
	Count() (int64, error)

	// CountByStatus returns the number of downloads by status
	CountByStatus(status DownloadStatus) (int64, error)

	// GetStats returns download statistics
	GetStats() (*DownloadStats, error)
}

// DownloadStats represents download statistics
type DownloadStats struct {
	Total      int64 `json:"total"`
	Queued     int64 `json:"queued"`
	Processing int64 `json:"processing"`
	Completed  int64 `json:"completed"`
	Failed     int64 `json:"failed"`
	Cancelled  int64 `json:"cancelled"`
}

