package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "localhost", config.Server.Host)
	assert.Equal(t, 8080, config.Server.Port)
	assert.Equal(t, 3, config.Download.MaxRetries)
	assert.Equal(t, 30*time.Second, config.Download.RetryDelay)
	assert.Equal(t, 1, config.Download.ConcurrentLimit)
	assert.True(t, config.Download.AutoStartWorkers)
	assert.Equal(t, 10*time.Second, config.Queue.CheckInterval)
	assert.Equal(t, "rogan", config.Telegram.Profile)
	assert.Equal(t, "bolt", config.Telegram.StorageType)
	assert.True(t, config.Telegram.UseGroup)
	assert.True(t, config.Telegram.RewriteExt)
	assert.True(t, config.Notification.Enabled)
	assert.Equal(t, "info", config.Logging.Level)
}

