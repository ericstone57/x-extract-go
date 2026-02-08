package infrastructure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yourusername/x-extract-go/internal/domain"
)

func newTestTelegramDownloader(config *domain.TelegramConfig) *TelegramDownloader {
	return NewTelegramDownloader(config, "/tmp/incoming", "/tmp/completed", nil)
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
