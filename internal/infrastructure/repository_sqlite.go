package infrastructure

import (
	"fmt"

	"github.com/yourusername/x-extract-go/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SQLiteDownloadRepository implements DownloadRepository using SQLite
type SQLiteDownloadRepository struct {
	db *gorm.DB
}

// NewSQLiteDownloadRepository creates a new SQLite repository
func NewSQLiteDownloadRepository(dbPath string) (*SQLiteDownloadRepository, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&domain.Download{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &SQLiteDownloadRepository{db: db}, nil
}

// Create creates a new download
func (r *SQLiteDownloadRepository) Create(download *domain.Download) error {
	return r.db.Create(download).Error
}

// Update updates an existing download
func (r *SQLiteDownloadRepository) Update(download *domain.Download) error {
	return r.db.Save(download).Error
}

// Delete deletes a download by ID
func (r *SQLiteDownloadRepository) Delete(id string) error {
	return r.db.Delete(&domain.Download{}, "id = ?", id).Error
}

// FindByID finds a download by ID
func (r *SQLiteDownloadRepository) FindByID(id string) (*domain.Download, error) {
	var download domain.Download
	err := r.db.First(&download, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &download, nil
}

// FindByStatus finds downloads by status
func (r *SQLiteDownloadRepository) FindByStatus(status domain.DownloadStatus) ([]*domain.Download, error) {
	var downloads []*domain.Download
	err := r.db.Where("status = ?", status).Find(&downloads).Error
	return downloads, err
}

// FindPending finds all pending downloads ordered by priority and creation time
func (r *SQLiteDownloadRepository) FindPending() ([]*domain.Download, error) {
	var downloads []*domain.Download
	err := r.db.Where("status = ?", domain.StatusQueued).
		Order("priority DESC, created_at ASC").
		Find(&downloads).Error
	return downloads, err
}

// FindAll finds all downloads with optional filters
func (r *SQLiteDownloadRepository) FindAll(filters map[string]interface{}) ([]*domain.Download, error) {
	var downloads []*domain.Download
	query := r.db

	for key, value := range filters {
		query = query.Where(fmt.Sprintf("%s = ?", key), value)
	}

	err := query.Order("created_at DESC").Find(&downloads).Error
	return downloads, err
}

// Count returns the total number of downloads
func (r *SQLiteDownloadRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&domain.Download{}).Count(&count).Error
	return count, err
}

// CountByStatus returns the number of downloads by status
func (r *SQLiteDownloadRepository) CountByStatus(status domain.DownloadStatus) (int64, error) {
	var count int64
	err := r.db.Model(&domain.Download{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

// GetStats returns download statistics
func (r *SQLiteDownloadRepository) GetStats() (*domain.DownloadStats, error) {
	stats := &domain.DownloadStats{}

	// Get total count
	if err := r.db.Model(&domain.Download{}).Count(&stats.Total).Error; err != nil {
		return nil, err
	}

	// Get counts by status
	statusCounts := []struct {
		Status domain.DownloadStatus
		Count  int64
	}{}

	if err := r.db.Model(&domain.Download{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&statusCounts).Error; err != nil {
		return nil, err
	}

	for _, sc := range statusCounts {
		switch sc.Status {
		case domain.StatusQueued:
			stats.Queued = sc.Count
		case domain.StatusProcessing:
			stats.Processing = sc.Count
		case domain.StatusCompleted:
			stats.Completed = sc.Count
		case domain.StatusFailed:
			stats.Failed = sc.Count
		case domain.StatusCancelled:
			stats.Cancelled = sc.Count
		}
	}

	return stats, nil
}

// Close closes the database connection
func (r *SQLiteDownloadRepository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

