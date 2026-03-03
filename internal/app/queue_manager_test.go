package app

import (
	"os"
	"path/filepath"
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
	return NewQueueManager(repo, nil, config, nil, "")
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

func TestExtractContentIDFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		platform domain.Platform
		want     string
	}{
		// Twitter / X
		{"x.com standard URL", "https://x.com/elonmusk/status/1234567890", domain.PlatformX, "1234567890"},
		{"twitter.com standard URL", "https://twitter.com/user/status/9876543210", domain.PlatformX, "9876543210"},
		{"x.com with trailing slash", "https://x.com/user/status/111222333/", domain.PlatformX, "111222333"},
		{"x.com with query params", "https://x.com/user/status/111222333?s=20", domain.PlatformX, "111222333"},
		{"x.com too few segments", "https://x.com/user", domain.PlatformX, ""},
		{"x.com http URL", "http://x.com/user/status/555666", domain.PlatformX, "555666"},

		// Telegram
		{"t.me standard URL", "https://t.me/channel/123", domain.PlatformTelegram, "123"},
		{"t.me private channel URL", "https://t.me/c/1234567890/456", domain.PlatformTelegram, "456"},
		{"t.me with trailing slash", "https://t.me/channel/789/", domain.PlatformTelegram, "789"},
		{"t.me with query params", "https://t.me/channel/789?single", domain.PlatformTelegram, "789"},
		{"t.me too few segments", "https://t.me/", domain.PlatformTelegram, ""},
		{"t.me http URL", "http://t.me/channel/999", domain.PlatformTelegram, "999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractContentIDFromURL(tt.url, tt.platform)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchesContentID(t *testing.T) {
	tests := []struct {
		name           string
		nameWithoutExt string
		contentID      string
		platform       domain.Platform
		want           bool
	}{
		// Twitter: {uploader_id}_{tweet_id}
		{"twitter match simple", "elonmusk_1234567890", "1234567890", domain.PlatformX, true},
		{"twitter match numeric uploader", "12345_9876543210", "9876543210", domain.PlatformX, true},
		{"twitter no match wrong id", "elonmusk_1111111111", "9999999999", domain.PlatformX, false},
		{"twitter no match single part", "1234567890", "1234567890", domain.PlatformX, false},
		{"twitter content id in wrong position", "1234567890_elonmusk", "1234567890", domain.PlatformX, false},

		// Telegram: {channel_id}_{message_id}_{media_id}
		{"telegram match standard", "3464638440_2685_6086895199301864978", "2685", domain.PlatformTelegram, true},
		{"telegram match two parts", "123456_789", "789", domain.PlatformTelegram, true},
		{"telegram no match wrong id", "3464638440_9999_6086895199301864978", "2685", domain.PlatformTelegram, false},
		{"telegram no match single part", "2685", "2685", domain.PlatformTelegram, false},
		{"telegram content id in wrong position (first)", "2685_3464638440_6086895199301864978", "2685", domain.PlatformTelegram, false},
		{"telegram content id in third position", "3464638440_1111_2685", "2685", domain.PlatformTelegram, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesContentID(tt.nameWithoutExt, tt.contentID, tt.platform)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestScanCompletedDirForURL(t *testing.T) {
	// Create a temp directory to simulate completed dir
	completedDir, err := os.MkdirTemp("", "test_completed_*")
	require.NoError(t, err)
	defer os.RemoveAll(completedDir)

	// Create fake completed files
	// Twitter file: elonmusk_1234567890.mp4
	twitterFile := filepath.Join(completedDir, "elonmusk_1234567890.mp4")
	require.NoError(t, os.WriteFile(twitterFile, []byte("fake"), 0644))

	// Telegram file: 3464638440_2685_6086895199301864978.jpg
	telegramFile := filepath.Join(completedDir, "3464638440_2685_6086895199301864978.jpg")
	require.NoError(t, os.WriteFile(telegramFile, []byte("fake"), 0644))

	// Metadata file (should be skipped)
	metadataFile := filepath.Join(completedDir, "elonmusk_1234567890.info.json")
	require.NoError(t, os.WriteFile(metadataFile, []byte("{}"), 0644))

	// A subdirectory (should be skipped)
	require.NoError(t, os.Mkdir(filepath.Join(completedDir, "subdir"), 0755))

	qm := &QueueManager{completedDir: completedDir}

	t.Run("finds twitter file by tweet ID", func(t *testing.T) {
		result := qm.scanCompletedDirForURL("https://x.com/elonmusk/status/1234567890", domain.PlatformX)
		assert.Equal(t, twitterFile, result)
	})

	t.Run("finds telegram file by message ID", func(t *testing.T) {
		result := qm.scanCompletedDirForURL("https://t.me/somechannel/2685", domain.PlatformTelegram)
		assert.Equal(t, telegramFile, result)
	})

	t.Run("returns empty for non-matching twitter URL", func(t *testing.T) {
		result := qm.scanCompletedDirForURL("https://x.com/user/status/9999999999", domain.PlatformX)
		assert.Empty(t, result)
	})

	t.Run("returns empty for non-matching telegram URL", func(t *testing.T) {
		result := qm.scanCompletedDirForURL("https://t.me/channel/9999", domain.PlatformTelegram)
		assert.Empty(t, result)
	})

	t.Run("returns empty when completedDir is empty string", func(t *testing.T) {
		emptyQM := &QueueManager{completedDir: ""}
		result := emptyQM.scanCompletedDirForURL("https://x.com/user/status/1234567890", domain.PlatformX)
		assert.Empty(t, result)
	})

	t.Run("returns empty when completedDir does not exist", func(t *testing.T) {
		badQM := &QueueManager{completedDir: "/nonexistent/path"}
		result := badQM.scanCompletedDirForURL("https://x.com/user/status/1234567890", domain.PlatformX)
		assert.Empty(t, result)
	})

	t.Run("skips info.json metadata files", func(t *testing.T) {
		// Create a dir with only a .info.json file matching the content ID
		metaOnlyDir, err := os.MkdirTemp("", "test_meta_only_*")
		require.NoError(t, err)
		defer os.RemoveAll(metaOnlyDir)

		require.NoError(t, os.WriteFile(
			filepath.Join(metaOnlyDir, "elonmusk_555.info.json"), []byte("{}"), 0644))

		metaQM := &QueueManager{completedDir: metaOnlyDir}
		result := metaQM.scanCompletedDirForURL("https://x.com/user/status/555", domain.PlatformX)
		assert.Empty(t, result, "should not match .info.json files")
	})
}

func TestAddDownload_FileScanDedup(t *testing.T) {
	// Create a temp directory to simulate completed dir with an existing file
	completedDir, err := os.MkdirTemp("", "test_completed_dedup_*")
	require.NoError(t, err)
	defer os.RemoveAll(completedDir)

	// Create a completed telegram file: 123456_789_999.mp4
	telegramFile := filepath.Join(completedDir, "123456_789_999.mp4")
	require.NoError(t, os.WriteFile(telegramFile, []byte("fake"), 0644))

	repo := newMockRepo()
	config := &domain.QueueConfig{
		CheckInterval:   10 * time.Second,
		AutoExitOnEmpty: false,
		EmptyWaitTime:   30 * time.Second,
	}
	qm := NewQueueManager(repo, nil, config, nil, completedDir)

	// Add a download for the same content — should be found on disk and returned as completed
	dl, err := qm.AddDownload("https://t.me/somechannel/789", domain.PlatformTelegram, domain.ModeDefault)
	require.NoError(t, err)
	require.NotNil(t, dl)
	assert.Equal(t, domain.StatusCompleted, dl.Status, "should be auto-completed from file scan")
	assert.Equal(t, telegramFile, dl.FilePath, "should have the found file path")
	assert.Len(t, repo.downloads, 1, "should create one completed record")

	// Adding the same URL again should now hit the DB-level completed check
	dl2, err := qm.AddDownload("https://t.me/somechannel/789", domain.PlatformTelegram, domain.ModeDefault)
	require.NoError(t, err)
	assert.Equal(t, dl.ID, dl2.ID, "should return same completed download from DB")
	assert.Len(t, repo.downloads, 1, "should not create another record")
}
