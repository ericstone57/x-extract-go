package domain

import (
	"time"
)

// TelegramMessageCache represents cached Telegram message metadata
// This avoids repeatedly calling tdl chat export for the same messages
type TelegramMessageCache struct {
	ChannelID  string    `json:"channel_id" gorm:"primaryKey;index:idx_channel_message"` // channel identifier
	MessageID  string    `json:"message_id" gorm:"primaryKey;index:idx_channel_message"` // message identifier (unique with channel_id)
	Text       string    `json:"text" gorm:"type:text"`                                  // message text/description
	Date       int64     `json:"date"`                                                   // message timestamp (for smart incremental export)
	SenderID   string    `json:"sender_id,omitempty"`                                    // sender user ID
	SenderName string    `json:"sender_name,omitempty"`                                  // sender name
	MediaType  string    `json:"media_type,omitempty"`                                   // type of media if present
	CachedAt   time.Time `json:"cached_at" gorm:"autoCreateTime"`                        // when this was cached
}

// TableName specifies the table name for GORM
func (TelegramMessageCache) TableName() string {
	return "telegram_message_cache"
}

// TelegramMessageCacheRepository defines the interface for message cache persistence
type TelegramMessageCacheRepository interface {
	// GetMessage retrieves cached message data for a specific channel+message
	// Returns nil if not found
	GetMessage(channelID, messageID string) (*TelegramMessageCache, error)

	// SaveMessage saves a single message to cache
	SaveMessage(cache *TelegramMessageCache) error

	// SaveMessages saves multiple messages in batch (more efficient)
	SaveMessages(caches []TelegramMessageCache) error

	// HasChannelCache checks if a channel has any cached messages
	HasChannelCache(channelID string) (bool, error)

	// GetMaxDate gets the maximum cached date for a channel (for smart incremental export)
	// Returns 0 if no messages are cached
	GetMaxDate(channelID string) (int64, error)

	// GetCachedMessages returns a map of all cached message IDs for a channel
	// This is used to filter out already-cached messages during export
	GetCachedMessages(channelID string) (map[string]bool, error)
}
