package infrastructure

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yourusername/x-extract-go/internal/domain"
	"go.uber.org/zap"
)

// TwitterDownloader implements Downloader for X/Twitter
type TwitterDownloader struct {
	config       *domain.TwitterConfig
	logger       *zap.Logger
	incomingDir  string
	completedDir string
}

// NewTwitterDownloader creates a new Twitter downloader
func NewTwitterDownloader(config *domain.TwitterConfig, incomingDir, completedDir string, logger *zap.Logger) *TwitterDownloader {
	return &TwitterDownloader{
		config:       config,
		logger:       logger,
		incomingDir:  incomingDir,
		completedDir: completedDir,
	}
}

// Platform returns the platform this downloader handles
func (d *TwitterDownloader) Platform() domain.Platform {
	return domain.PlatformX
}

// Validate validates if the downloader can handle the given URL
func (d *TwitterDownloader) Validate(url string) error {
	if !strings.HasPrefix(url, "https://x.com") && !strings.HasPrefix(url, "https://twitter.com") {
		return fmt.Errorf("invalid Twitter/X URL: %s", url)
	}
	return nil
}

// Download downloads media from Twitter/X
func (d *TwitterDownloader) Download(download *domain.Download) error {
	d.logger.Info("Starting Twitter download",
		zap.String("url", download.URL),
		zap.String("id", download.ID))

	// Validate URL
	if err := d.Validate(download.URL); err != nil {
		return err
	}

	// Ensure incoming directory exists
	if err := os.MkdirAll(d.incomingDir, 0755); err != nil {
		return fmt.Errorf("failed to create incoming directory: %w", err)
	}

	// Build yt-dlp command - download to incoming directory
	args := []string{
		"--write-info-json",
		"--write-playlist-metafiles",
		"--restrict-filenames",
		"-o", "%(uploader_id)s_%(id)s_%(title).20U.%(ext)s",
		"-P", d.incomingDir,
	}

	// Add cookie file if configured
	if d.config.CookieFile != "" && fileExists(d.config.CookieFile) {
		args = append(args, "--cookies", d.config.CookieFile)
	}

	args = append(args, download.URL)

	// Execute yt-dlp
	cmd := exec.Command(d.config.YTDLPBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		d.logger.Error("yt-dlp failed",
			zap.String("url", download.URL),
			zap.Error(err),
			zap.String("output", string(output)))
		return fmt.Errorf("yt-dlp failed: %w - %s", err, string(output))
	}

	d.logger.Info("yt-dlp completed",
		zap.String("url", download.URL),
		zap.String("output", string(output)))

	// Find downloaded files in incoming directory
	files, err := d.findDownloadedFiles(download.URL)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no files downloaded")
	}

	// Move files from incoming to completed directory
	completedFiles, err := d.moveToCompleted(files)
	if err != nil {
		return fmt.Errorf("failed to move files to completed: %w", err)
	}

	// Store metadata
	if d.config.WriteMetadata {
		if err := d.storeMetadata(download, completedFiles); err != nil {
			d.logger.Warn("Failed to store metadata", zap.Error(err))
		}
	}

	// Update download with file path (use first file if multiple)
	download.FilePath = completedFiles[0]

	return nil
}

// findDownloadedFiles finds files downloaded for a specific URL in incoming directory
func (d *TwitterDownloader) findDownloadedFiles(url string) ([]string, error) {
	var files []string

	// Extract tweet ID from URL
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid URL format")
	}

	// Walk through incoming directory to find recently created files
	err := filepath.Walk(d.incomingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && isMediaFile(path) {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// moveToCompleted moves files from incoming to completed directory
func (d *TwitterDownloader) moveToCompleted(files []string) ([]string, error) {
	var completedFiles []string

	// Ensure completed directory exists
	if err := os.MkdirAll(d.completedDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create completed directory: %w", err)
	}

	for _, file := range files {
		filename := filepath.Base(file)
		destPath := filepath.Join(d.completedDir, filename)

		// Move file
		if err := os.Rename(file, destPath); err != nil {
			// If rename fails, try copy and delete
			if err := copyFile(file, destPath); err != nil {
				return nil, fmt.Errorf("failed to move file %s: %w", file, err)
			}
			os.Remove(file)
		}

		d.logger.Info("Moved file to completed",
			zap.String("from", file),
			zap.String("to", destPath))

		completedFiles = append(completedFiles, destPath)
	}

	return completedFiles, nil
}

// storeMetadata stores download metadata
func (d *TwitterDownloader) storeMetadata(download *domain.Download, files []string) error {
	metadata := map[string]interface{}{
		"url":      download.URL,
		"platform": download.Platform,
		"files":    files,
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	download.Metadata = string(data)
	return nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isMediaFile checks if a file is a media file
func isMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	mediaExts := []string{".mp4", ".mkv", ".avi", ".mov", ".webm", ".m4v", ".jpg", ".png", ".gif", ".webp"}
	for _, mediaExt := range mediaExts {
		if ext == mediaExt {
			return true
		}
	}
	return false
}
