package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/x-extract-go/internal/domain"
)

// mockDownloadManagerRepo implements domain.DownloadRepository for testing
type mockDownloadManagerRepo struct {
	downloads map[string]*domain.Download
}

func newMockDownloadManagerRepo() *mockDownloadManagerRepo {
	return &mockDownloadManagerRepo{downloads: make(map[string]*domain.Download)}
}

func (m *mockDownloadManagerRepo) Create(download *domain.Download) error {
	m.downloads[download.ID] = download
	return nil
}

func (m *mockDownloadManagerRepo) Update(download *domain.Download) error {
	m.downloads[download.ID] = download
	return nil
}

func (m *mockDownloadManagerRepo) Delete(id string) error {
	delete(m.downloads, id)
	return nil
}

func (m *mockDownloadManagerRepo) FindByID(id string) (*domain.Download, error) {
	if d, ok := m.downloads[id]; ok {
		return d, nil
	}
	return nil, nil
}

func (m *mockDownloadManagerRepo) FindByURL(url string, statuses []domain.DownloadStatus) (*domain.Download, error) {
	return nil, nil
}

func (m *mockDownloadManagerRepo) FindByStatus(status domain.DownloadStatus) ([]*domain.Download, error) {
	return nil, nil
}

func (m *mockDownloadManagerRepo) FindPending() ([]*domain.Download, error) {
	return nil, nil
}

func (m *mockDownloadManagerRepo) FindAll(filters map[string]interface{}) ([]*domain.Download, error) {
	return nil, nil
}

func (m *mockDownloadManagerRepo) Count() (int64, error) {
	return int64(len(m.downloads)), nil
}

func (m *mockDownloadManagerRepo) CountByStatus(status domain.DownloadStatus) (int64, error) {
	return 0, nil
}

func (m *mockDownloadManagerRepo) CountActive() (int64, error) {
	return 0, nil
}

func (m *mockDownloadManagerRepo) GetStats() (*domain.DownloadStats, error) {
	return nil, nil
}

func TestRetryDownload_Failed(t *testing.T) {
	repo := newMockDownloadManagerRepo()
	dm := NewDownloadManager(repo, nil, nil, &domain.DownloadConfig{MaxRetries: 3}, nil)

	download := &domain.Download{
		ID:     "test-1",
		URL:    "https://t.me/test/1",
		Status: domain.StatusFailed,
	}
	repo.Create(download)

	err := dm.RetryDownload(nil, "test-1")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusQueued, download.Status)
	assert.Equal(t, 0, download.RetryCount)
	assert.Empty(t, download.ErrorMessage)
}

func TestRetryDownload_Cancelled(t *testing.T) {
	repo := newMockDownloadManagerRepo()
	dm := NewDownloadManager(repo, nil, nil, &domain.DownloadConfig{MaxRetries: 3}, nil)

	download := &domain.Download{
		ID:     "test-2",
		URL:    "https://t.me/test/2",
		Status: domain.StatusCancelled,
	}
	repo.Create(download)

	err := dm.RetryDownload(nil, "test-2")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusQueued, download.Status)
	assert.Equal(t, 0, download.RetryCount)
	assert.Empty(t, download.ErrorMessage)
	assert.Nil(t, download.StartedAt)
	assert.Nil(t, download.CompletedAt)
}

func TestRetryDownload_AlreadyQueued(t *testing.T) {
	repo := newMockDownloadManagerRepo()
	dm := NewDownloadManager(repo, nil, nil, &domain.DownloadConfig{MaxRetries: 3}, nil)

	download := &domain.Download{
		ID:     "test-3",
		URL:    "https://t.me/test/3",
		Status: domain.StatusQueued,
	}
	repo.Create(download)

	err := dm.RetryDownload(nil, "test-3")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already queued")
}

func TestRetryDownload_Processing(t *testing.T) {
	repo := newMockDownloadManagerRepo()
	dm := NewDownloadManager(repo, nil, nil, &domain.DownloadConfig{MaxRetries: 3}, nil)

	download := &domain.Download{
		ID:     "test-4",
		URL:    "https://t.me/test/4",
		Status: domain.StatusProcessing,
	}
	repo.Create(download)

	err := dm.RetryDownload(nil, "test-4")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "currently processing")
}

func TestRetryDownload_Completed(t *testing.T) {
	repo := newMockDownloadManagerRepo()
	dm := NewDownloadManager(repo, nil, nil, &domain.DownloadConfig{MaxRetries: 3}, nil)

	download := &domain.Download{
		ID:     "test-5",
		URL:    "https://t.me/test/5",
		Status: domain.StatusCompleted,
	}
	repo.Create(download)

	err := dm.RetryDownload(nil, "test-5")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already completed")
}

func TestRetryDownload_NotFound(t *testing.T) {
	repo := newMockDownloadManagerRepo()
	dm := NewDownloadManager(repo, nil, nil, &domain.DownloadConfig{MaxRetries: 3}, nil)

	err := dm.RetryDownload(nil, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
