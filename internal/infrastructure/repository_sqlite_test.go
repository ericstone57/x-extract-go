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

