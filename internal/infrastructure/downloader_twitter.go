package infrastructure

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/yourusername/x-extract-go/internal/domain"
	"github.com/yourusername/x-extract-go/pkg/logger"
	"go.uber.org/zap"
)

// TwitterDownloader implements Downloader for X/Twitter
type TwitterDownloader struct {
	config       *domain.TwitterConfig
	multiLogger  *logger.MultiLogger
	incomingDir  string
	completedDir string
}

// NewTwitterDownloader creates a new Twitter downloader
func NewTwitterDownloader(config *domain.TwitterConfig, incomingDir, completedDir string, multiLogger *logger.MultiLogger) *TwitterDownloader {
	return &TwitterDownloader{
		config:       config,
		multiLogger:  multiLogger,
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
func (d *TwitterDownloader) Download(download *domain.Download, progressCallback domain.DownloadProgressCallback) error {
	// Validate URL
	if err := d.Validate(download.URL); err != nil {
		if d.multiLogger != nil {
			d.multiLogger.WriteDownloadComplete(download.ID, false, fmt.Sprintf("Invalid URL: %v", err))
		}
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

	// Create progress callback for real-time output
	if progressCallback == nil {
		progressCallback = func(output string, percent float64) {}
	}

	// Write command to download log
	cmdLine := fmt.Sprintf("%s %s", d.config.YTDLPBinary, strings.Join(args, " "))
	if d.multiLogger != nil {
		d.multiLogger.WriteDownloadCommand(download.ID, cmdLine)
	}

	// Execute yt-dlp with real-time output capture
	cmd := exec.Command(d.config.YTDLPBinary, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		if d.multiLogger != nil {
			d.multiLogger.WriteDownloadComplete(download.ID, false, fmt.Sprintf("Failed to start yt-dlp: %v", err))
		}
		return fmt.Errorf("failed to start yt-dlp: %w", err)
	}

	// Read stdout line by line - write raw output to download log
	done := make(chan struct{})
	go func() {
		defer close(done)
		stdoutScanner := bufio.NewScanner(stdout)
		for stdoutScanner.Scan() {
			line := stdoutScanner.Text()
			percent := parseYTDLProgress(line)
			progressCallback(line, percent)

			// Write raw stdout to download log
			if d.multiLogger != nil {
				d.multiLogger.WriteRawDownloadLog(line)
			}
		}
	}()

	// Read stderr - write to both download log (as [STDERR]) and error log
	go func() {
		stderrScanner := bufio.NewScanner(stderr)
		for stderrScanner.Scan() {
			line := stderrScanner.Text()
			// Write to download log and error log
			if d.multiLogger != nil {
				d.multiLogger.WriteRawError(download.ID, line)
			}
		}
	}()

	// Wait for stdout goroutine to complete
	<-done

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		if d.multiLogger != nil {
			d.multiLogger.WriteDownloadComplete(download.ID, false, fmt.Sprintf("yt-dlp failed: %v", err))
		}
		return fmt.Errorf("yt-dlp failed: %w", err)
	}

	// Find downloaded files in incoming directory
	files, err := d.findDownloadedFiles(download.URL)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		if d.multiLogger != nil {
			d.multiLogger.WriteDownloadComplete(download.ID, false, "No files downloaded")
		}
		return fmt.Errorf("no files downloaded")
	}

	// Move files from incoming to completed directory
	completedFiles, err := d.moveToCompleted(files)
	if err != nil {
		if d.multiLogger != nil {
			d.multiLogger.WriteDownloadComplete(download.ID, false, fmt.Sprintf("Failed to move files: %v", err))
		}
		return fmt.Errorf("failed to move files to completed: %w", err)
	}

	// Store metadata
	if d.config.WriteMetadata {
		if err := d.storeMetadata(download, completedFiles); err != nil {
			if d.multiLogger != nil {
				d.multiLogger.LogAppError("Failed to store metadata", zap.Error(err))
			}
		}
	}

	// Update download with file path (use first file if multiple)
	download.FilePath = completedFiles[0]

	// Log successful completion
	if d.multiLogger != nil {
		d.multiLogger.WriteDownloadComplete(download.ID, true, fmt.Sprintf("Downloaded: %s", download.FilePath))
	}

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

// isMediaFile checks if a file is a media file or metadata file
func isMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	mediaExts := []string{".mp4", ".mkv", ".avi", ".mov", ".webm", ".m4v", ".jpg", ".png", ".gif", ".webp", ".json"}
	for _, mediaExt := range mediaExts {
		if ext == mediaExt {
			return true
		}
	}
	return false
}

// parseYTDLProgress parses yt-dlp output to extract progress percentage
func parseYTDLProgress(line string) float64 {
	// Match patterns like: "  45.3% of 12.34MiB at 1.23MiB/s ETA 00:32"
	progressRegex := regexp.MustCompile(`([\d.]+)%`)
	if match := progressRegex.FindStringSubmatch(line); match != nil {
		percent, _ := strconv.ParseFloat(match[1], 64)
		return percent
	}
	return -1
}
