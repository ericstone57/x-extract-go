package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/x-extract-go/internal/domain"
	"github.com/yourusername/x-extract-go/internal/infrastructure"
)

// --- extractChannelIDFromFilename tests ---

func TestExtractChannelIDFromFilename_ValidMediaFile(t *testing.T) {
	result := extractChannelIDFromFilename("3464638440_1907_blacktiger88-15-11-2025-0i57ki4zky2rp84sk21ht_source.m4v")
	assert.Equal(t, "3464638440", result)
}

func TestExtractChannelIDFromFilename_InfoJsonFile(t *testing.T) {
	result := extractChannelIDFromFilename("3464638440_1907_blacktiger88-15-11-2025-0i57ki4zky2rp84sk21ht_source.info.json")
	assert.Equal(t, "3464638440", result)
}

func TestExtractChannelIDFromFilename_NonNumericID(t *testing.T) {
	result := extractChannelIDFromFilename("somechannel_1907_media.m4v")
	assert.Equal(t, "", result)
}

func TestExtractChannelIDFromFilename_TooFewParts(t *testing.T) {
	result := extractChannelIDFromFilename("singlepart.m4v")
	assert.Equal(t, "", result)
}

func TestExtractChannelIDFromFilename_EmptyString(t *testing.T) {
	result := extractChannelIDFromFilename("")
	assert.Equal(t, "", result)
}

func TestExtractChannelIDFromFilename_DifferentChannelID(t *testing.T) {
	result := extractChannelIDFromFilename("9876543210_42_some_media.mp4")
	assert.Equal(t, "9876543210", result)
}

// --- resolveMessageText tests ---

func setupTestRepoForCLI(t *testing.T) (*infrastructure.SQLiteDownloadRepository, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "cli-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")
	repo, err := infrastructure.NewSQLiteDownloadRepository(dbPath)
	require.NoError(t, err)

	cleanup := func() {
		repo.Close()
		os.RemoveAll(tmpDir)
	}
	return repo, cleanup
}

func TestResolveMessageText_DirectLookup(t *testing.T) {
	repo, cleanup := setupTestRepoForCLI(t)
	defer cleanup()

	// Save a message with text
	require.NoError(t, repo.SaveMessages([]domain.TelegramMessageCache{
		{ChannelID: "123", MessageID: "100", Text: "Hello world"},
	}))

	result := resolveMessageText(repo, "123", "100")
	assert.Equal(t, "Hello world", result)
}

func TestResolveMessageText_GroupedResolution(t *testing.T) {
	repo, cleanup := setupTestRepoForCLI(t)
	defer cleanup()

	// Simulate grouped messages: 1906 has no text, 1907 has text, same group
	require.NoError(t, repo.SaveMessages([]domain.TelegramMessageCache{
		{ChannelID: "3464638440", MessageID: "1906", Text: "", GroupedID: "14126963880319333"},
		{ChannelID: "3464638440", MessageID: "1907", Text: "Kengo系列六期。本期共3个批次，第2批次。#DJ0005 🔺会员专享🔻", GroupedID: "14126963880319333"},
	}))

	result := resolveMessageText(repo, "3464638440", "1906")
	assert.Equal(t, "Kengo系列六期。本期共3个批次，第2批次。#DJ0005 🔺会员专享🔻", result)
}

func TestResolveMessageText_NearbyFallback(t *testing.T) {
	repo, cleanup := setupTestRepoForCLI(t)
	defer cleanup()

	// Message 50 has no text and no grouped ID, but nearby message 52 has text
	require.NoError(t, repo.SaveMessages([]domain.TelegramMessageCache{
		{ChannelID: "123", MessageID: "50", Text: ""},
		{ChannelID: "123", MessageID: "52", Text: "Nearby text"},
	}))

	result := resolveMessageText(repo, "123", "50")
	assert.Equal(t, "Nearby text", result)
}

func TestResolveMessageText_NoTextFound(t *testing.T) {
	repo, cleanup := setupTestRepoForCLI(t)
	defer cleanup()

	// Message exists but no text anywhere nearby
	require.NoError(t, repo.SaveMessages([]domain.TelegramMessageCache{
		{ChannelID: "123", MessageID: "50", Text: ""},
	}))

	result := resolveMessageText(repo, "123", "50")
	assert.Equal(t, "", result)
}

func TestResolveMessageText_MessageNotInCache(t *testing.T) {
	repo, cleanup := setupTestRepoForCLI(t)
	defer cleanup()

	// No messages in cache at all
	result := resolveMessageText(repo, "123", "999")
	assert.Equal(t, "", result)
}

// --- extractIDsFromDownload tests ---

func TestExtractIDsFromDownload_FromFilesList(t *testing.T) {
	dl := domain.NewDownload("https://t.me/c/3464638440/1907", domain.PlatformTelegram, domain.ModeDefault)
	metadata := map[string]interface{}{
		"files": []interface{}{
			"/path/to/completed/3464638440_1907_blacktiger88_source.m4v",
		},
	}

	channelID, msgID := extractIDsFromDownload(dl, metadata)
	assert.Equal(t, "3464638440", channelID)
	assert.Equal(t, "1907", msgID)
}

func TestExtractIDsFromDownload_FromURL(t *testing.T) {
	dl := domain.NewDownload("https://t.me/c/3464638440/1907", domain.PlatformTelegram, domain.ModeDefault)
	metadata := map[string]interface{}{} // no files

	channelID, msgID := extractIDsFromDownload(dl, metadata)
	assert.Equal(t, "3464638440", channelID)
	assert.Equal(t, "1907", msgID)
}

func TestExtractIDsFromDownload_EmptyMetadata(t *testing.T) {
	dl := domain.NewDownload("https://example.com/video", domain.PlatformTelegram, domain.ModeDefault)
	metadata := map[string]interface{}{}

	channelID, msgID := extractIDsFromDownload(dl, metadata)
	assert.Equal(t, "", channelID)
	assert.Equal(t, "", msgID)
}

func TestExtractIDsFromDownload_NonTelegramURL(t *testing.T) {
	dl := domain.NewDownload("https://x.com/user/status/123", domain.PlatformTelegram, domain.ModeDefault)
	metadata := map[string]interface{}{}

	channelID, msgID := extractIDsFromDownload(dl, metadata)
	assert.Equal(t, "", channelID)
	assert.Equal(t, "", msgID)
}

