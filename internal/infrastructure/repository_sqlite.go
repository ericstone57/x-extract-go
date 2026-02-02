package infrastructure

import (
	"fmt"
	"time"

	"github.com/yourusername/x-extract-go/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// SQLiteDownloadRepository implements DownloadRepository and TelegramChannelRepository using SQLite
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

	// Auto-migrate the schema for Download and TelegramChannel
	if err := db.AutoMigrate(&domain.Download{}, &domain.TelegramChannel{}); err != nil {
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

// CountActive returns the number of active downloads (queued + processing)
func (r *SQLiteDownloadRepository) CountActive() (int64, error) {
	var count int64
	err := r.db.Model(&domain.Download{}).
		Where("status IN ?", []domain.DownloadStatus{domain.StatusQueued, domain.StatusProcessing}).
		Count(&count).Error
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

// ============================================================================
// TelegramChannelRepository implementation
// ============================================================================

// GetChannelName retrieves the channel name for a given channel ID
// Returns empty string if not found
func (r *SQLiteDownloadRepository) GetChannelName(channelID string) (string, error) {
	var channel domain.TelegramChannel
	err := r.db.Select("channel_name").Where("channel_id = ?", channelID).First(&channel).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return channel.ChannelName, nil
}

// GetChannel retrieves the full channel record for a given channel ID
// Returns nil if not found
func (r *SQLiteDownloadRepository) GetChannel(channelID string) (*domain.TelegramChannel, error) {
	var channel domain.TelegramChannel
	err := r.db.Where("channel_id = ?", channelID).First(&channel).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &channel, nil
}

// UpdateChannelList updates or inserts multiple channels
// channels is a map of channelID -> TelegramChannel
func (r *SQLiteDownloadRepository) UpdateChannelList(channels map[string]*domain.TelegramChannel) error {
	if len(channels) == 0 {
		return nil
	}

	// Convert map to slice
	channelList := make([]*domain.TelegramChannel, 0, len(channels))
	now := time.Now()
	for _, ch := range channels {
		ch.LastUpdatedAt = now
		channelList = append(channelList, ch)
	}

	// Upsert all channels (insert or update on conflict)
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "channel_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"channel_name", "channel_type", "username", "last_updated_at"}),
	}).Create(&channelList).Error
}

// ShouldUpdateChannelList checks if the channel list needs updating
// Returns true if the list is empty or the newest record is older than maxAge
func (r *SQLiteDownloadRepository) ShouldUpdateChannelList(maxAge time.Duration) (bool, error) {
	var count int64
	if err := r.db.Model(&domain.TelegramChannel{}).Count(&count).Error; err != nil {
		return true, err
	}

	// If no records, should update
	if count == 0 {
		return true, nil
	}

	// Check the most recent update time
	lastUpdate, err := r.GetLastUpdateTime()
	if err != nil {
		return true, err
	}

	// If last update is older than maxAge, should update
	return time.Since(lastUpdate) > maxAge, nil
}

// GetLastUpdateTime returns the most recent LastUpdatedAt time
// Returns zero time if no records exist
func (r *SQLiteDownloadRepository) GetLastUpdateTime() (time.Time, error) {
	var channel domain.TelegramChannel
	err := r.db.Order("last_updated_at DESC").First(&channel).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return channel.LastUpdatedAt, nil
}
