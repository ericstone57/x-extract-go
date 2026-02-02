package domain

import (
	"time"
)

// TelegramChannel represents a Telegram channel with its ID and name mapping
type TelegramChannel struct {
	ChannelID     string    `json:"channel_id" gorm:"primaryKey"`
	ChannelName   string    `json:"channel_name" gorm:"not null"`
	ChannelType   string    `json:"channel_type" gorm:"default:channel"` // channel, group, private
	Username      string    `json:"username,omitempty"`                  // Public username if available
	LastUpdatedAt time.Time `json:"last_updated_at" gorm:"autoUpdateTime"`
}

// TableName specifies the table name for GORM
func (TelegramChannel) TableName() string {
	return "telegram_channels"
}

// TelegramChannelRepository defines the interface for Telegram channel persistence
type TelegramChannelRepository interface {
	// GetChannelName retrieves the channel name for a given channel ID
	// Returns empty string if not found
	GetChannelName(channelID string) (string, error)

	// GetChannel retrieves the full channel record for a given channel ID
	// Returns nil if not found
	GetChannel(channelID string) (*TelegramChannel, error)

	// UpdateChannelList updates or inserts multiple channels
	// channels is a map of channelID -> TelegramChannel
	UpdateChannelList(channels map[string]*TelegramChannel) error

	// ShouldUpdateChannelList checks if the channel list needs updating
	// Returns true if the list is empty or the newest record is older than maxAge
	ShouldUpdateChannelList(maxAge time.Duration) (bool, error)

	// GetLastUpdateTime returns the most recent LastUpdatedAt time
	// Returns zero time if no records exist
	GetLastUpdateTime() (time.Time, error)
}

// ChannelUpdateMaxAge is the default maximum age before channel list needs updating
const ChannelUpdateMaxAge = 7 * 24 * time.Hour // 7 days
