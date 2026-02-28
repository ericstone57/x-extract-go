package infrastructure

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/x-extract-go/internal/domain"
)

func setupTestRepo(t *testing.T) (*SQLiteDownloadRepository, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "repo-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")
	repo, err := NewSQLiteDownloadRepository(dbPath)
	require.NoError(t, err)

	cleanup := func() {
		repo.Close()
		os.RemoveAll(tmpDir)
	}
	return repo, cleanup
}

func TestFindByURL_ReturnsMatchingDownload(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a completed download
	dl := domain.NewDownload("https://t.me/channel/123", domain.PlatformTelegram, domain.ModeDefault)
	dl.MarkCompleted("/path/to/file.mp4")
	require.NoError(t, repo.Create(dl))

	// Should find it when searching for completed status
	found, err := repo.FindByURL("https://t.me/channel/123", []domain.DownloadStatus{domain.StatusCompleted})
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, dl.ID, found.ID)
	assert.Equal(t, domain.StatusCompleted, found.Status)
}

func TestFindByURL_ReturnsNilWhenNoMatch(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Search for a URL that doesn't exist
	found, err := repo.FindByURL("https://t.me/nonexistent/999", []domain.DownloadStatus{domain.StatusQueued, domain.StatusCompleted})
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestFindByURL_FiltersOnStatus(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a failed download
	dl := domain.NewDownload("https://t.me/channel/456", domain.PlatformTelegram, domain.ModeDefault)
	dl.MarkFailed(assert.AnError)
	require.NoError(t, repo.Create(dl))

	// Should NOT find it when searching for queued/processing/completed
	found, err := repo.FindByURL("https://t.me/channel/456", []domain.DownloadStatus{
		domain.StatusQueued,
		domain.StatusProcessing,
		domain.StatusCompleted,
	})
	require.NoError(t, err)
	assert.Nil(t, found, "failed download should not match active statuses")

	// Should find it when searching for failed status
	found, err = repo.FindByURL("https://t.me/channel/456", []domain.DownloadStatus{domain.StatusFailed})
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, dl.ID, found.ID)
}

func TestFindByURL_ReturnsMostRecent(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	url := "https://t.me/channel/789"

	// Create an older failed download (allowed to re-add after failure)
	old := domain.NewDownload(url, domain.PlatformTelegram, domain.ModeDefault)
	old.MarkFailed(assert.AnError)
	require.NoError(t, repo.Create(old))

	// Create a newer queued download
	newer := domain.NewDownload(url, domain.PlatformTelegram, domain.ModeDefault)
	require.NoError(t, repo.Create(newer))

	// Should return the newer queued one
	found, err := repo.FindByURL(url, []domain.DownloadStatus{domain.StatusQueued})
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, newer.ID, found.ID)
}

func TestFindByURL_MultipleStatuses(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	url := "https://t.me/channel/multi"

	// Create a queued download
	dl := domain.NewDownload(url, domain.PlatformTelegram, domain.ModeDefault)
	require.NoError(t, repo.Create(dl))

	// Should find it with multiple statuses including queued
	found, err := repo.FindByURL(url, []domain.DownloadStatus{
		domain.StatusQueued,
		domain.StatusProcessing,
		domain.StatusCompleted,
	})
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, dl.ID, found.ID)
}

// ============================================================================
// TelegramMessageCache: GetMessagesByGroupedID tests
// ============================================================================

func TestGetMessagesByGroupedID_FindsGroupedMessages(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Save messages with the same grouped_id (simulating a media album)
	caches := []domain.TelegramMessageCache{
		{ChannelID: "chan1", MessageID: "1906", Text: "", GroupedID: "14126963880319333", Date: 1000},
		{ChannelID: "chan1", MessageID: "1907", Text: "Album caption text", GroupedID: "14126963880319333", Date: 1001},
		{ChannelID: "chan1", MessageID: "1908", Text: "", GroupedID: "14126963880319333", Date: 1002},
		{ChannelID: "chan1", MessageID: "2000", Text: "Unrelated message", GroupedID: "99999999", Date: 2000},
	}
	require.NoError(t, repo.SaveMessages(caches))

	// Should find all 3 messages with the same grouped_id
	results, err := repo.GetMessagesByGroupedID("chan1", "14126963880319333")
	require.NoError(t, err)
	assert.Len(t, results, 3)

	// Verify the one with text is among them
	foundText := false
	for _, r := range results {
		if r.Text == "Album caption text" {
			foundText = true
			break
		}
	}
	assert.True(t, foundText, "should find the message with text in the grouped results")
}

func TestGetMessagesByGroupedID_NoResults(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	results, err := repo.GetMessagesByGroupedID("chan1", "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestGetMessagesByGroupedID_DifferentChannels(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Same grouped_id but different channels
	caches := []domain.TelegramMessageCache{
		{ChannelID: "chan1", MessageID: "100", Text: "Chan1 text", GroupedID: "group1", Date: 1000},
		{ChannelID: "chan2", MessageID: "100", Text: "Chan2 text", GroupedID: "group1", Date: 1000},
	}
	require.NoError(t, repo.SaveMessages(caches))

	// Should only return messages from chan1
	results, err := repo.GetMessagesByGroupedID("chan1", "group1")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "chan1", results[0].ChannelID)
}

// ============================================================================
// TelegramMessageCache: GetNearbyMessages tests
// ============================================================================

func TestGetNearbyMessages_FindsNearbyMessages(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	// Save messages with sequential IDs
	caches := []domain.TelegramMessageCache{
		{ChannelID: "chan1", MessageID: "1904", Text: "", Date: 1000},
		{ChannelID: "chan1", MessageID: "1905", Text: "", Date: 1001},
		{ChannelID: "chan1", MessageID: "1906", Text: "", Date: 1002},
		{ChannelID: "chan1", MessageID: "1907", Text: "Nearby text", Date: 1003},
		{ChannelID: "chan1", MessageID: "1908", Text: "", Date: 1004},
		{ChannelID: "chan1", MessageID: "1909", Text: "", Date: 1005},
		{ChannelID: "chan1", MessageID: "1910", Text: "", Date: 1006},
	}
	require.NoError(t, repo.SaveMessages(caches))

	// Search ±3 from message 1906 (should find 1903-1909 range, excluding 1906 itself)
	results, err := repo.GetNearbyMessages("chan1", "1906", 3)
	require.NoError(t, err)

	// Should find 1904, 1905, 1907, 1908, 1909 (5 messages, excluding 1906 and 1903/1910 outside ±3 for 1904-1909... wait)
	// Range: 1903-1909, but we only have 1904,1905,1907,1908,1909 in range (excluding 1906)
	assert.Len(t, results, 5)

	// Should NOT include the target message itself
	for _, r := range results {
		assert.NotEqual(t, "1906", r.MessageID)
	}

	// Should include the one with text
	foundText := false
	for _, r := range results {
		if r.Text == "Nearby text" {
			foundText = true
			break
		}
	}
	assert.True(t, foundText, "should find message with text in nearby results")
}

func TestGetNearbyMessages_ExcludesTargetMessage(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	caches := []domain.TelegramMessageCache{
		{ChannelID: "chan1", MessageID: "100", Text: "Target text", Date: 1000},
		{ChannelID: "chan1", MessageID: "101", Text: "Nearby", Date: 1001},
	}
	require.NoError(t, repo.SaveMessages(caches))

	results, err := repo.GetNearbyMessages("chan1", "100", 3)
	require.NoError(t, err)

	// Should NOT include message 100 itself
	for _, r := range results {
		assert.NotEqual(t, "100", r.MessageID)
	}
	assert.Len(t, results, 1)
	assert.Equal(t, "101", results[0].MessageID)
}

func TestGetNearbyMessages_InvalidMessageID(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	_, err := repo.GetNearbyMessages("chan1", "notanumber", 3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid message ID")
}

func TestGetNearbyMessages_DifferentChannels(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	caches := []domain.TelegramMessageCache{
		{ChannelID: "chan1", MessageID: "100", Text: "", Date: 1000},
		{ChannelID: "chan1", MessageID: "101", Text: "Chan1 nearby", Date: 1001},
		{ChannelID: "chan2", MessageID: "101", Text: "Chan2 nearby", Date: 1001},
	}
	require.NoError(t, repo.SaveMessages(caches))

	results, err := repo.GetNearbyMessages("chan1", "100", 3)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "chan1", results[0].ChannelID)
	assert.Equal(t, "Chan1 nearby", results[0].Text)
}
