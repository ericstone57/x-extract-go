//go:build integration
// +build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-extract-go/internal/app"
	"x-extract-go/internal/domain"
	"x-extract-go/internal/infrastructure/persistence"
)

func TestDownloadWorkflow_Success(t *testing.T) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "x-extract-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	repo, err := persistence.NewSQLiteRepository(dbPath)
	require.NoError(t, err)

	config := domain.DefaultConfig()
	config.Download.BaseDir = tmpDir
	config.Download.IncomingDir = filepath.Join(tmpDir, "incoming")
	config.Download.CompletedDir = filepath.Join(tmpDir, "completed")

	mockDownloader := &MockDownloader{}
	service := app.NewDownloadService(repo, mockDownloader, mockDownloader, config)

	// Create download
	download := domain.NewDownload("https://x.com/test/status/123", domain.PlatformX, domain.ModeDefault)
	err = service.AddDownload(download)
	require.NoError(t, err)

	// Verify download was added
	retrieved, err := service.GetDownload(download.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusQueued, retrieved.Status)

	// Process download
	err = service.ProcessDownload(download.ID)
	require.NoError(t, err)

	// Verify download completed
	completed, err := service.GetDownload(download.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCompleted, completed.Status)
	assert.NotEmpty(t, completed.FilePath)
}

func TestDownloadWorkflow_Retry(t *testing.T) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "x-extract-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	repo, err := persistence.NewSQLiteRepository(dbPath)
	require.NoError(t, err)

	config := domain.DefaultConfig()
	config.Download.BaseDir = tmpDir
	config.Download.IncomingDir = filepath.Join(tmpDir, "incoming")
	config.Download.CompletedDir = filepath.Join(tmpDir, "completed")
	config.Download.MaxRetries = 3

	failingDownloader := &FailingDownloader{failCount: 2}
	service := app.NewDownloadService(repo, failingDownloader, failingDownloader, config)

	// Create download
	download := domain.NewDownload("https://x.com/test/status/123", domain.PlatformX, domain.ModeDefault)
	err = service.AddDownload(download)
	require.NoError(t, err)

	// First attempt - should fail
	err = service.ProcessDownload(download.ID)
	assert.Error(t, err)

	retrieved, _ := service.GetDownload(download.ID)
	assert.Equal(t, domain.StatusFailed, retrieved.Status)
	assert.Equal(t, 1, retrieved.RetryCount)

	// Retry - should fail again
	err = service.RetryDownload(download.ID)
	require.NoError(t, err)

	err = service.ProcessDownload(download.ID)
	assert.Error(t, err)

	retrieved, _ = service.GetDownload(download.ID)
	assert.Equal(t, 2, retrieved.RetryCount)

	// Final retry - should succeed
	err = service.RetryDownload(download.ID)
	require.NoError(t, err)

	err = service.ProcessDownload(download.ID)
	require.NoError(t, err)

	completed, _ := service.GetDownload(download.ID)
	assert.Equal(t, domain.StatusCompleted, completed.Status)
}

func TestDownloadWorkflow_Cancel(t *testing.T) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "x-extract-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	repo, err := persistence.NewSQLiteRepository(dbPath)
	require.NoError(t, err)

	config := domain.DefaultConfig()
	service := app.NewDownloadService(repo, &MockDownloader{}, &MockDownloader{}, config)

	// Create download
	download := domain.NewDownload("https://x.com/test/status/123", domain.PlatformX, domain.ModeDefault)
	err = service.AddDownload(download)
	require.NoError(t, err)

	// Cancel download
	err = service.CancelDownload(download.ID)
	require.NoError(t, err)

	// Verify cancelled
	cancelled, err := service.GetDownload(download.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCancelled, cancelled.Status)

	// Attempt to process - should fail
	err = service.ProcessDownload(download.ID)
	assert.Error(t, err)
}

type FailingDownloader struct {
	failCount int
	attempts  int
}

func (f *FailingDownloader) Download(download *domain.Download) error {
	f.attempts++
	if f.attempts <= f.failCount {
		return assert.AnError
	}
	time.Sleep(50 * time.Millisecond)
	download.MarkCompleted("/tmp/test.mp4")
	return nil
}
