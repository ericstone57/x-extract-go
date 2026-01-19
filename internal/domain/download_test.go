package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDownload(t *testing.T) {
	url := "https://x.com/user/status/123"
	platform := PlatformX
	mode := ModeDefault

	download := NewDownload(url, platform, mode)

	assert.NotEmpty(t, download.ID)
	assert.Equal(t, url, download.URL)
	assert.Equal(t, platform, download.Platform)
	assert.Equal(t, mode, download.Mode)
	assert.Equal(t, StatusQueued, download.Status)
	assert.Equal(t, 0, download.Priority)
	assert.Equal(t, 0, download.RetryCount)
}

func TestDownload_MarkProcessing(t *testing.T) {
	download := NewDownload("https://x.com/test", PlatformX, ModeDefault)
	
	download.MarkProcessing()
	
	assert.Equal(t, StatusProcessing, download.Status)
	assert.NotNil(t, download.StartedAt)
}

func TestDownload_MarkCompleted(t *testing.T) {
	download := NewDownload("https://x.com/test", PlatformX, ModeDefault)
	filePath := "/path/to/file.mp4"
	
	download.MarkCompleted(filePath)
	
	assert.Equal(t, StatusCompleted, download.Status)
	assert.Equal(t, filePath, download.FilePath)
	assert.NotNil(t, download.CompletedAt)
}

func TestDownload_MarkFailed(t *testing.T) {
	download := NewDownload("https://x.com/test", PlatformX, ModeDefault)
	err := errors.New("download failed")
	
	download.MarkFailed(err)
	
	assert.Equal(t, StatusFailed, download.Status)
	assert.Equal(t, "download failed", download.ErrorMessage)
}

func TestDownload_IncrementRetry(t *testing.T) {
	download := NewDownload("https://x.com/test", PlatformX, ModeDefault)
	
	download.IncrementRetry()
	assert.Equal(t, 1, download.RetryCount)
	
	download.IncrementRetry()
	assert.Equal(t, 2, download.RetryCount)
}

func TestDownload_CanRetry(t *testing.T) {
	download := NewDownload("https://x.com/test", PlatformX, ModeDefault)
	download.Status = StatusFailed
	
	assert.True(t, download.CanRetry(3))
	
	download.RetryCount = 3
	assert.False(t, download.CanRetry(3))
	
	download.RetryCount = 0
	download.Status = StatusCompleted
	assert.False(t, download.CanRetry(3))
}

func TestDownload_IsTerminal(t *testing.T) {
	download := NewDownload("https://x.com/test", PlatformX, ModeDefault)
	
	assert.False(t, download.IsTerminal())
	
	download.Status = StatusCompleted
	assert.True(t, download.IsTerminal())
	
	download.Status = StatusCancelled
	assert.True(t, download.IsTerminal())
	
	download.Status = StatusFailed
	assert.False(t, download.IsTerminal())
}

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		url      string
		expected Platform
	}{
		{"https://x.com/user/status/123", PlatformX},
		{"https://twitter.com/user/status/123", PlatformX},
		{"https://t.me/channel/123", PlatformTelegram},
		{"https://example.com", ""},
	}
	
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := DetectPlatform(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidatePlatform(t *testing.T) {
	assert.True(t, ValidatePlatform(PlatformX))
	assert.True(t, ValidatePlatform(PlatformTelegram))
	assert.False(t, ValidatePlatform("invalid"))
}

func TestValidateMode(t *testing.T) {
	assert.True(t, ValidateMode(ModeDefault))
	assert.True(t, ValidateMode(ModeSingle))
	assert.True(t, ValidateMode(ModeGroup))
	assert.False(t, ValidateMode("invalid"))
}

