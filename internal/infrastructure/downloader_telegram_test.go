package infrastructure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yourusername/x-extract-go/internal/domain"
)

// mockMessageCacheRepo is a mock implementation of TelegramMessageCacheRepository for testing
type mockMessageCacheRepo struct {
	messages []domain.TelegramMessageCache
}

func (m *mockMessageCacheRepo) GetMessage(channelID, messageID string) (*domain.TelegramMessageCache, error) {
	for _, msg := range m.messages {
		if msg.ChannelID == channelID && msg.MessageID == messageID {
			return &msg, nil
		}
	}
	return nil, nil
}

func (m *mockMessageCacheRepo) SaveMessage(cache *domain.TelegramMessageCache) error { return nil }
func (m *mockMessageCacheRepo) SaveMessages(caches []domain.TelegramMessageCache) error {
	return nil
}
func (m *mockMessageCacheRepo) HasChannelCache(channelID string) (bool, error) { return false, nil }
func (m *mockMessageCacheRepo) GetMaxDate(channelID string) (int64, error)     { return 0, nil }
func (m *mockMessageCacheRepo) GetCachedMessages(channelID string) (map[string]bool, error) {
	return nil, nil
}

func (m *mockMessageCacheRepo) GetMessagesByGroupedID(channelID, groupedID string) ([]domain.TelegramMessageCache, error) {
	var result []domain.TelegramMessageCache
	for _, msg := range m.messages {
		if msg.ChannelID == channelID && msg.GroupedID == groupedID {
			result = append(result, msg)
		}
	}
	return result, nil
}

func (m *mockMessageCacheRepo) GetNearbyMessages(channelID, messageID string, msgRange int) ([]domain.TelegramMessageCache, error) {
	var result []domain.TelegramMessageCache
	for _, msg := range m.messages {
		if msg.ChannelID == channelID && msg.MessageID != messageID {
			result = append(result, msg)
		}
	}
	return result, nil
}

func newTestTelegramDownloader(config *domain.TelegramConfig) *TelegramDownloader {
	return NewTelegramDownloader(config, "/tmp/incoming", "/tmp/completed", "/tmp/logs", nil)
}

func newTestTelegramDownloaderWithMockRepo(mockRepo *mockMessageCacheRepo) *TelegramDownloader {
	d := NewTelegramDownloader(&domain.TelegramConfig{}, "/tmp/incoming", "/tmp/completed", "/tmp/logs", nil)
	d.SetMessageCacheRepository(mockRepo)
	return d
}

func TestBuildTDLCommand_IncludesSkipSame(t *testing.T) {
	config := &domain.TelegramConfig{
		Profile:     "test",
		StorageType: "bolt",
		StoragePath: "/tmp/storage",
		TDLBinary:   "tdl",
	}
	downloader := newTestTelegramDownloader(config)

	dl := domain.NewDownload("https://t.me/channel/123", domain.PlatformTelegram, domain.ModeDefault)
	args := downloader.buildTDLCommand(dl, "/tmp/download")

	assert.Contains(t, args, "--skip-same", "tdl command should include --skip-same flag")
	assert.Contains(t, args, "--continue", "tdl command should include --continue flag to avoid interactive prompt")
}

func TestBuildTDLCommand_BasicArgs(t *testing.T) {
	config := &domain.TelegramConfig{
		Profile:     "myprofile",
		StorageType: "bolt",
		StoragePath: "/data/storage",
		TDLBinary:   "tdl",
	}
	downloader := newTestTelegramDownloader(config)

	dl := domain.NewDownload("https://t.me/channel/456", domain.PlatformTelegram, domain.ModeDefault)
	args := downloader.buildTDLCommand(dl, "/tmp/tempdir")

	assert.Contains(t, args, "-n")
	assert.Contains(t, args, "myprofile")
	assert.Contains(t, args, "dl")
	assert.Contains(t, args, "-u")
	assert.Contains(t, args, "https://t.me/channel/456")
	assert.Contains(t, args, "-d")
	assert.Contains(t, args, "/tmp/tempdir")
}

func TestBuildTDLCommand_GroupMode(t *testing.T) {
	config := &domain.TelegramConfig{
		Profile:     "test",
		StorageType: "bolt",
		StoragePath: "/tmp/storage",
		UseGroup:    false,
	}
	downloader := newTestTelegramDownloader(config)

	// ModeGroup should force --group
	dl := domain.NewDownload("https://t.me/channel/789", domain.PlatformTelegram, domain.ModeGroup)
	args := downloader.buildTDLCommand(dl, "/tmp/tempdir")
	assert.Contains(t, args, "--group", "ModeGroup should add --group flag")

	// ModeSingle should NOT have --group even if config says UseGroup=true
	config.UseGroup = true
	dl2 := domain.NewDownload("https://t.me/channel/789", domain.PlatformTelegram, domain.ModeSingle)
	args2 := downloader.buildTDLCommand(dl2, "/tmp/tempdir")
	assert.NotContains(t, args2, "--group", "ModeSingle should not have --group flag")
}

func TestBuildTDLCommand_RewriteExt(t *testing.T) {
	config := &domain.TelegramConfig{
		Profile:     "test",
		StorageType: "bolt",
		StoragePath: "/tmp/storage",
		RewriteExt:  true,
	}
	downloader := newTestTelegramDownloader(config)

	dl := domain.NewDownload("https://t.me/channel/123", domain.PlatformTelegram, domain.ModeDefault)
	args := downloader.buildTDLCommand(dl, "/tmp/tempdir")
	assert.Contains(t, args, "--rewrite-ext")
}

func TestBuildTDLCommand_ExtraParams(t *testing.T) {
	config := &domain.TelegramConfig{
		Profile:     "test",
		StorageType: "bolt",
		StoragePath: "/tmp/storage",
		ExtraParams: "--threads 8 --limit 4",
	}
	downloader := newTestTelegramDownloader(config)

	dl := domain.NewDownload("https://t.me/channel/123", domain.PlatformTelegram, domain.ModeDefault)
	args := downloader.buildTDLCommand(dl, "/tmp/tempdir")
	assert.Contains(t, args, "--threads")
	assert.Contains(t, args, "8")
	assert.Contains(t, args, "--limit")
	assert.Contains(t, args, "4")
}

func TestGetExistingDownloadedFiles_WithMetadata(t *testing.T) {
	config := &domain.TelegramConfig{}
	downloader := newTestTelegramDownloader(config)

	download := &domain.Download{
		ID:       "test-123",
		URL:      "https://t.me/channel/123",
		Platform: domain.PlatformTelegram,
		Metadata: `{"files": ["/tmp/completed/file1.mp4", "/tmp/completed/file2.jpg"]}`,
	}

	files := downloader.getExistingDownloadedFiles(download)
	assert.Len(t, files, 2)
	assert.Contains(t, files, "/tmp/completed/file1.mp4")
	assert.Contains(t, files, "/tmp/completed/file2.jpg")
}

func TestGetExistingDownloadedFiles_NoMetadata(t *testing.T) {
	config := &domain.TelegramConfig{}
	downloader := newTestTelegramDownloader(config)

	download := &domain.Download{
		ID:       "test-123",
		URL:      "https://t.me/channel/123",
		Platform: domain.PlatformTelegram,
		Metadata: "",
	}

	files := downloader.getExistingDownloadedFiles(download)
	assert.Nil(t, files)
}

func TestGetExistingDownloadedFiles_InvalidMetadata(t *testing.T) {
	config := &domain.TelegramConfig{}
	downloader := newTestTelegramDownloader(config)

	download := &domain.Download{
		ID:       "test-123",
		URL:      "https://t.me/channel/123",
		Platform: domain.PlatformTelegram,
		Metadata: "not valid json",
	}

	files := downloader.getExistingDownloadedFiles(download)
	assert.Nil(t, files)
}

func TestCheckFilesExist_AllExist(t *testing.T) {
	config := &domain.TelegramConfig{}
	downloader := newTestTelegramDownloader(config)

	// Create temp files
	tmpDir, err := os.MkdirTemp("", "test-files-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	file1 := filepath.Join(tmpDir, "file1.mp4")
	file2 := filepath.Join(tmpDir, "file2.jpg")
	os.WriteFile(file1, []byte("test"), 0644)
	os.WriteFile(file2, []byte("test"), 0644)

	allExist, missing := downloader.checkFilesExist([]string{file1, file2})
	assert.True(t, allExist)
	assert.Nil(t, missing)
}

func TestCheckFilesExist_SomeMissing(t *testing.T) {
	config := &domain.TelegramConfig{}
	downloader := newTestTelegramDownloader(config)

	// Create temp directory and one file
	tmpDir, err := os.MkdirTemp("", "test-files-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	existingFile := filepath.Join(tmpDir, "existing.mp4")
	os.WriteFile(existingFile, []byte("test"), 0644)

	allExist, missing := downloader.checkFilesExist([]string{existingFile, "/nonexistent/path/file.mp4"})
	assert.False(t, allExist)
	assert.Len(t, missing, 1)
	assert.Contains(t, missing[0], "/nonexistent/path/file.mp4")
}

func TestUpdateMetadataAfterPartialDeletion(t *testing.T) {
	config := &domain.TelegramConfig{}
	downloader := newTestTelegramDownloader(config)

	download := &domain.Download{
		ID:       "test-123",
		URL:      "https://t.me/channel/123",
		Platform: domain.PlatformTelegram,
		Metadata: `{"files": ["/tmp/completed/file1.mp4", "/tmp/completed/file2.jpg", "/tmp/completed/file3.jpg"]}`,
	}

	remainingFiles := []string{"/tmp/completed/file1.mp4", "/tmp/completed/file3.jpg"}
	downloader.updateMetadataAfterPartialDeletion(download, remainingFiles)

	// Verify metadata was updated
	var metadata map[string]interface{}
	err := json.Unmarshal([]byte(download.Metadata), &metadata)
	assert.NoError(t, err)

	filesRaw := metadata["files"].([]interface{})
	assert.Len(t, filesRaw, 2)
	assert.Contains(t, metadata["note"], "deleted by user")
}

// ============================================================================
// formatGroupedID tests
// ============================================================================

func TestFormatGroupedID_NilRaw(t *testing.T) {
	assert.Equal(t, "", formatGroupedID(nil))
}

func TestFormatGroupedID_ZeroID(t *testing.T) {
	raw := &TelegramRawMessage{GroupedID: 0}
	assert.Equal(t, "", formatGroupedID(raw))
}

func TestFormatGroupedID_ValidID(t *testing.T) {
	raw := &TelegramRawMessage{GroupedID: 14126963880319333}
	assert.Equal(t, "14126963880319333", formatGroupedID(raw))
}

// ============================================================================
// resolveGroupedText tests
// ============================================================================

func TestResolveGroupedText_FindsTextByGroupedID(t *testing.T) {
	mockRepo := &mockMessageCacheRepo{
		messages: []domain.TelegramMessageCache{
			{ChannelID: "chan1", MessageID: "1906", Text: "", GroupedID: "group1"},
			{ChannelID: "chan1", MessageID: "1907", Text: "Album caption", GroupedID: "group1"},
		},
	}
	d := newTestTelegramDownloaderWithMockRepo(mockRepo)

	text := d.resolveGroupedText("chan1", "1906", "group1")
	assert.Equal(t, "Album caption", text)
}

func TestResolveGroupedText_FallsBackToNearbyMessages(t *testing.T) {
	mockRepo := &mockMessageCacheRepo{
		messages: []domain.TelegramMessageCache{
			{ChannelID: "chan1", MessageID: "1906", Text: "", GroupedID: ""},
			{ChannelID: "chan1", MessageID: "1907", Text: "Nearby text", GroupedID: ""},
		},
	}
	d := newTestTelegramDownloaderWithMockRepo(mockRepo)

	// No grouped_id, should fall back to nearby
	text := d.resolveGroupedText("chan1", "1906", "")
	assert.Equal(t, "Nearby text", text)
}

func TestResolveGroupedText_NoGroupedNoNearby(t *testing.T) {
	mockRepo := &mockMessageCacheRepo{
		messages: []domain.TelegramMessageCache{
			{ChannelID: "chan1", MessageID: "1906", Text: "", GroupedID: ""},
		},
	}
	d := newTestTelegramDownloaderWithMockRepo(mockRepo)

	// No grouped_id, no nearby messages with text
	text := d.resolveGroupedText("chan1", "1906", "")
	assert.Equal(t, "", text)
}

func TestResolveGroupedText_GroupedIDNoTextFallsBackToNearby(t *testing.T) {
	mockRepo := &mockMessageCacheRepo{
		messages: []domain.TelegramMessageCache{
			{ChannelID: "chan1", MessageID: "1906", Text: "", GroupedID: "group1"},
			{ChannelID: "chan1", MessageID: "1907", Text: "", GroupedID: "group1"}, // grouped but no text
			{ChannelID: "chan1", MessageID: "1908", Text: "Nearby", GroupedID: ""}, // not in group but has text
		},
	}
	d := newTestTelegramDownloaderWithMockRepo(mockRepo)

	// Grouped messages have no text, should fall back to nearby
	text := d.resolveGroupedText("chan1", "1906", "group1")
	assert.Equal(t, "Nearby", text)
}

// ============================================================================
// cachedToMessageData tests
// ============================================================================

func TestCachedToMessageData_WithText(t *testing.T) {
	d := newTestTelegramDownloaderWithMockRepo(&mockMessageCacheRepo{})

	cached := &domain.TelegramMessageCache{
		ChannelID: "chan1",
		MessageID: "1907",
		Text:      "Direct text",
		Date:      1234567890,
		SenderID:  "12345",
	}

	result := d.cachedToMessageData(cached)
	assert.Equal(t, 1907, result.ID)
	assert.Equal(t, "Direct text", result.Text)
	assert.Equal(t, int64(1234567890), result.Date)
	assert.Equal(t, int64(12345), result.Raw.FromID.UserID)
}

func TestCachedToMessageData_ResolvesGroupedText(t *testing.T) {
	mockRepo := &mockMessageCacheRepo{
		messages: []domain.TelegramMessageCache{
			{ChannelID: "chan1", MessageID: "1906", Text: "", GroupedID: "group1"},
			{ChannelID: "chan1", MessageID: "1907", Text: "Resolved from group", GroupedID: "group1"},
		},
	}
	d := newTestTelegramDownloaderWithMockRepo(mockRepo)

	cached := &domain.TelegramMessageCache{
		ChannelID: "chan1",
		MessageID: "1906",
		Text:      "",
		GroupedID: "group1",
		Date:      1000,
	}

	result := d.cachedToMessageData(cached)
	assert.Equal(t, 1906, result.ID)
	assert.Equal(t, "Resolved from group", result.Text)
}

func TestCachedToMessageData_NoCacheRepo(t *testing.T) {
	// No message cache repo set - should not attempt resolution
	d := newTestTelegramDownloader(&domain.TelegramConfig{})

	cached := &domain.TelegramMessageCache{
		ChannelID: "chan1",
		MessageID: "1906",
		Text:      "",
		GroupedID: "group1",
		Date:      1000,
	}

	result := d.cachedToMessageData(cached)
	assert.Equal(t, 1906, result.ID)
	assert.Equal(t, "", result.Text, "should not resolve text when no cache repo is set")
}

func TestCachedToMessageData_RealWorldScenario(t *testing.T) {
	// Simulate the exact real-world scenario from the bug report
	mockRepo := &mockMessageCacheRepo{
		messages: []domain.TelegramMessageCache{
			{ChannelID: "3464638440", MessageID: "1906", Text: "", GroupedID: "14126963880319333", Date: 1731600000},
			{ChannelID: "3464638440", MessageID: "1907", Text: "Kengo系列六期。本期共3个批次，第2批次。#DJ0005 🔺会员专享🔻", GroupedID: "14126963880319333", Date: 1731600001},
		},
	}
	d := newTestTelegramDownloaderWithMockRepo(mockRepo)

	cached := &domain.TelegramMessageCache{
		ChannelID: "3464638440",
		MessageID: "1906",
		Text:      "",
		GroupedID: "14126963880319333",
		Date:      1731600000,
	}

	result := d.cachedToMessageData(cached)
	assert.Equal(t, 1906, result.ID)
	assert.Equal(t, "Kengo系列六期。本期共3个批次，第2批次。#DJ0005 🔺会员专享🔻", result.Text)
}
