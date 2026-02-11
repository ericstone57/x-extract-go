package app

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/x-extract-go/internal/domain"
)

// mockRepo implements domain.DownloadRepository for testing
type mockRepo struct {
	downloads []*domain.Download
}

func newMockRepo() *mockRepo {
	return &mockRepo{downloads: make([]*domain.Download, 0)}
}

func (m *mockRepo) Create(download *domain.Download) error {
	m.downloads = append(m.downloads, download)
	return nil
}

func (m *mockRepo) Update(download *domain.Download) error {
	for i, d := range m.downloads {
		if d.ID == download.ID {
			m.downloads[i] = download
			return nil
		}
	}
	return nil
}

func (m *mockRepo) Delete(id string) error { return nil }

func (m *mockRepo) FindByID(id string) (*domain.Download, error) {
	for _, d := range m.downloads {
		if d.ID == id {
			return d, nil
		}
	}
	return nil, nil
}

func (m *mockRepo) FindByURL(url string, statuses []domain.DownloadStatus) (*domain.Download, error) {
	for i := len(m.downloads) - 1; i >= 0; i-- {
		d := m.downloads[i]
		if d.URL == url {
			for _, s := range statuses {
				if d.Status == s {
					return d, nil
				}
			}
		}
	}
	return nil, nil
}

func (m *mockRepo) FindByStatus(status domain.DownloadStatus) ([]*domain.Download, error) {
	return nil, nil
}
func (m *mockRepo) FindPending() ([]*domain.Download, error) { return nil, nil }
func (m *mockRepo) FindAll(filters map[string]interface{}) ([]*domain.Download, error) {
	return nil, nil
}
func (m *mockRepo) Count() (int64, error)                                     { return 0, nil }
func (m *mockRepo) CountByStatus(status domain.DownloadStatus) (int64, error) { return 0, nil }
func (m *mockRepo) CountActive() (int64, error)                               { return 0, nil }
func (m *mockRepo) ResetOrphanedProcessing() (int64, error)                   { return 0, nil }
func (m *mockRepo) GetStats() (*domain.DownloadStats, error)                  { return nil, nil }

func newTestQueueManager(repo domain.DownloadRepository) *QueueManager {
	config := &domain.QueueConfig{
		CheckInterval:   10 * time.Second,
		AutoExitOnEmpty: false,
		EmptyWaitTime:   30 * time.Second,
	}
	return NewQueueManager(repo, nil, config, nil)
}

func TestAddDownload_NewURL(t *testing.T) {
	repo := newMockRepo()
	qm := newTestQueueManager(repo)

	dl, err := qm.AddDownload("https://t.me/channel/123", domain.PlatformTelegram, domain.ModeDefault)
	require.NoError(t, err)
	require.NotNil(t, dl)
	assert.Equal(t, "https://t.me/channel/123", dl.URL)
	assert.Equal(t, domain.StatusQueued, dl.Status)
	assert.Len(t, repo.downloads, 1)
}

func TestAddDownload_DuplicateQueued(t *testing.T) {
	repo := newMockRepo()
	qm := newTestQueueManager(repo)

	// Add first download
	first, err := qm.AddDownload("https://t.me/channel/123", domain.PlatformTelegram, domain.ModeDefault)
	require.NoError(t, err)

	// Try to add same URL again - should return existing
	second, err := qm.AddDownload("https://t.me/channel/123", domain.PlatformTelegram, domain.ModeDefault)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "should return existing download, not create new one")
	assert.Len(t, repo.downloads, 1, "should not create a second entry")
}

func TestAddDownload_DuplicateCompleted_FileExists(t *testing.T) {
	repo := newMockRepo()
	qm := newTestQueueManager(repo)

	// Create a temp file to simulate existing completed file
	tmpFile, err := os.CreateTemp("", "test_download_*.mp4")
	require.NoError(t, err)
	tmpFilePath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpFilePath)

	// Add and complete a download
	first, err := qm.AddDownload("https://t.me/channel/exists", domain.PlatformTelegram, domain.ModeDefault)
	require.NoError(t, err)
	first.MarkCompleted(tmpFilePath)

	// Try to add same URL again - should return existing completed since file exists
	second, err := qm.AddDownload("https://t.me/channel/exists", domain.PlatformTelegram, domain.ModeDefault)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "should return existing completed download")
	assert.Equal(t, domain.StatusCompleted, second.Status)
	assert.Len(t, repo.downloads, 1, "should not create a second entry")
}

func TestAddDownload_DuplicateCompleted_FileMissing(t *testing.T) {
	repo := newMockRepo()
	qm := newTestQueueManager(repo)

	// Add and complete a download with a file path that doesn't exist
	first, err := qm.AddDownload("https://t.me/channel/missing", domain.PlatformTelegram, domain.ModeDefault)
	require.NoError(t, err)
	first.MarkCompleted("/path/to/nonexistent/file.mp4")

	// Try to add same URL again - should create NEW download since file is missing
	second, err := qm.AddDownload("https://t.me/channel/missing", domain.PlatformTelegram, domain.ModeDefault)
	require.NoError(t, err)
	assert.NotEqual(t, first.ID, second.ID, "should create new download when file is missing")
	assert.Equal(t, domain.StatusQueued, second.Status)
	assert.Len(t, repo.downloads, 2, "should create a second entry for re-download")
}

func TestAddDownload_AllowsRetryAfterFailure(t *testing.T) {
	repo := newMockRepo()
	qm := newTestQueueManager(repo)

	// Add and fail a download
	first, err := qm.AddDownload("https://t.me/channel/789", domain.PlatformTelegram, domain.ModeDefault)
	require.NoError(t, err)
	first.MarkFailed(assert.AnError)

	// Try to add same URL again - should create NEW download since previous one failed
	second, err := qm.AddDownload("https://t.me/channel/789", domain.PlatformTelegram, domain.ModeDefault)
	require.NoError(t, err)
	assert.NotEqual(t, first.ID, second.ID, "should create new download after failure")
	assert.Equal(t, domain.StatusQueued, second.Status)
	assert.Len(t, repo.downloads, 2, "should have two entries")
}

func TestAddDownload_AllowsRetryAfterCancellation(t *testing.T) {
	repo := newMockRepo()
	qm := newTestQueueManager(repo)

	// Add and cancel a download
	first, err := qm.AddDownload("https://t.me/channel/cancel", domain.PlatformTelegram, domain.ModeDefault)
	require.NoError(t, err)
	first.Status = domain.StatusCancelled

	// Try to add same URL again - should create NEW download since previous was cancelled
	second, err := qm.AddDownload("https://t.me/channel/cancel", domain.PlatformTelegram, domain.ModeDefault)
	require.NoError(t, err)
	assert.NotEqual(t, first.ID, second.ID, "should create new download after cancellation")
	assert.Len(t, repo.downloads, 2)
}
