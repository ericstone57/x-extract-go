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
	config *domain.TwitterConfig
	logger *zap.Logger
	baseDir string
}

// NewTwitterDownloader creates a new Twitter downloader
func NewTwitterDownloader(config *domain.TwitterConfig, baseDir string, logger *zap.Logger) *TwitterDownloader {
	return &TwitterDownloader{
		config:  config,
		logger:  logger,
		baseDir: baseDir,
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

	// Ensure base directory exists
	if err := os.MkdirAll(d.baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}

	// Build yt-dlp command
	args := []string{
		"--write-info-json",
		"--write-playlist-metafiles",
		"--restrict-filenames",
		"-o", "%(uploader_id)s_%(id)s_%(title).20U.%(ext)s",
		"-P", d.baseDir,
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

	// Find downloaded files
	files, err := d.findDownloadedFiles(download.URL)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no files downloaded")
	}

	// Store metadata
	if d.config.WriteMetadata {
		if err := d.storeMetadata(download, files); err != nil {
			d.logger.Warn("Failed to store metadata", zap.Error(err))
		}
	}

	// Update download with file path (use first file if multiple)
	download.FilePath = files[0]

	return nil
}

// findDownloadedFiles finds files downloaded for a specific URL
func (d *TwitterDownloader) findDownloadedFiles(url string) ([]string, error) {
	var files []string

	// Extract tweet ID from URL
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid URL format")
	}

	// Walk through base directory to find recently created files
	err := filepath.Walk(d.baseDir, func(path string, info os.FileInfo, err error) error {
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

