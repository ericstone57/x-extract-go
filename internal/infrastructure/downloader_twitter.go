package infrastructure

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourusername/x-extract-go/internal/domain"
	"github.com/yourusername/x-extract-go/pkg/logger"
	"go.uber.org/zap"
)

// TwitterDownloader implements Downloader for X/Twitter
type TwitterDownloader struct {
	DownloadLogger // Embedded shared log file operations
	config         *domain.TwitterConfig
	incomingDir    string
	completedDir   string
	eventLogger    *logger.MultiLogger // For structured events only (LogQueueEvent, LogAppError)
}

// NewTwitterDownloader creates a new Twitter downloader
func NewTwitterDownloader(config *domain.TwitterConfig, incomingDir, completedDir, logsDir string, eventLogger *logger.MultiLogger) *TwitterDownloader {
	return &TwitterDownloader{
		DownloadLogger: DownloadLogger{LogsDir: logsDir},
		config:         config,
		incomingDir:    incomingDir,
		completedDir:   completedDir,
		eventLogger:    eventLogger,
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
		return err
	}

	// Ensure incoming directory exists
	if err := os.MkdirAll(d.incomingDir, 0755); err != nil {
		return fmt.Errorf("failed to create incoming directory: %w", err)
	}

	// Build yt-dlp command - download to incoming directory
	// Note: exec.Command passes args directly to process, no shell quoting needed
	args := []string{
		"--write-info-json",
		"--write-playlist-metafiles",
		"--restrict-filenames",
		"-o", "%(uploader_id)s_%(id)s.%(ext)s",
		"-P", d.incomingDir,
	}

	// Add cookie file if configured
	if d.config.CookieFile != "" && FileExists(d.config.CookieFile) {
		args = append(args, "--cookies", d.config.CookieFile)
	}

	args = append(args, download.URL)

	// Create default callback if nil
	if progressCallback == nil {
		progressCallback = func(output string, percent float64) {}
	}

	// Open per-download log file so parallel downloads don't interleave.
	downloadLog, err := d.OpenDownloadLogFile(download.ID)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer downloadLog.Close()

	// Write command header to download log (with proper shell escaping for display)
	cmdLine := ShellEscapeCommand(d.config.YTDLPBinary, args...)
	d.WriteLogHeader(downloadLog, download.ID, cmdLine)

	// Execute yt-dlp with direct file redirect
	// Redirect both stdout and stderr to the same file (like cmd > file 2>&1)
	cmd := exec.Command(d.config.YTDLPBinary, args...)
	cmd.Stdout = downloadLog
	cmd.Stderr = downloadLog

	// Run command and check exit code
	err = cmd.Run()

	// Write completion marker
	if err != nil {
		d.WriteLogFooter(downloadLog, false, fmt.Sprintf("yt-dlp failed: %v", err))
		progressCallback("", -1) // Signal failure
		return fmt.Errorf("yt-dlp failed: %w", err)
	}

	// Find downloaded files in incoming directory
	files, err := d.findDownloadedFiles(download.URL)
	if err != nil {
		d.WriteLogFooter(downloadLog, false, fmt.Sprintf("Failed to find files: %v", err))
		return err
	}

	if len(files) == 0 {
		d.WriteLogFooter(downloadLog, false, "No files downloaded")
		return fmt.Errorf("no files downloaded")
	}

	// Move files from incoming to completed directory
	completedFiles, err := d.moveToCompleted(files)
	if err != nil {
		d.WriteLogFooter(downloadLog, false, fmt.Sprintf("Failed to move files: %v", err))
		return fmt.Errorf("failed to move files to completed: %w", err)
	}

	// Store metadata
	if d.config.WriteMetadata {
		if err := d.storeMetadata(download, completedFiles); err != nil {
			if d.eventLogger != nil {
				d.eventLogger.LogAppError("Failed to store metadata", zap.Error(err))
			}
		}
	}

	// Update download with file path (use first file if multiple)
	download.FilePath = completedFiles[0]

	// Log successful completion
	d.WriteLogFooter(downloadLog, true, fmt.Sprintf("Downloaded: %s", download.FilePath))
	progressCallback("", 100) // Signal success

	return nil
}

// findDownloadedFiles finds files downloaded for a specific URL in incoming directory
func (d *TwitterDownloader) findDownloadedFiles(url string) ([]string, error) {
	// Extract username from URL
	// URL format: https://x.com/{username}/status/{tweet_id} or https://twitter.com/{username}/status/{tweet_id}
	// After removing protocol, parts should be: ["x.com", "username", "status", "tweet_id"]
	username := ""

	// Remove protocol prefix
	urlWithoutProtocol := strings.TrimPrefix(url, "https://")
	urlWithoutProtocol = strings.TrimPrefix(urlWithoutProtocol, "http://")

	parts := strings.Split(urlWithoutProtocol, "/")
	if len(parts) >= 3 {
		// parts[0] = "x.com" or "twitter.com"
		// parts[1] = username
		username = parts[1]
	}

	var files []string

	// Walk through incoming directory to find recently created files
	err := filepath.Walk(d.incomingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		filename := filepath.Base(path)
		// Only include media files, NOT .info.json files (we'll handle those separately)
		if !info.IsDir() && IsMediaFile(path) {
			prefix := username + "_"
			// Only include files that match this username
			// Filename format: {username}_{video_id}.{ext}
			if strings.HasPrefix(filename, prefix) {
				files = append(files, path)
			}
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
			if err := CopyFile(file, destPath); err != nil {
				return nil, fmt.Errorf("failed to move file %s: %w", file, err)
			}
			os.Remove(file)
		}

		completedFiles = append(completedFiles, destPath)

		// Also move corresponding .info.json file if it exists
		infoJSONPath := strings.TrimSuffix(file, filepath.Ext(file)) + ".info.json"
		if infoData, err := os.ReadFile(infoJSONPath); err == nil {
			infoJSONDest := filepath.Join(d.completedDir, filepath.Base(infoJSONPath))
			if err := os.WriteFile(infoJSONDest, infoData, 0644); err == nil {
				os.Remove(infoJSONPath)
			}
		}
	}

	return completedFiles, nil
}

// storeMetadata stores download metadata by reading yt-dlp's .info.json files
func (d *TwitterDownloader) storeMetadata(download *domain.Download, files []string) error {
	// Try to read yt-dlp's .info.json file to extract rich metadata
	var meta *domain.MediaMetadata

	// Look for .info.json files in the completed directory (files have been moved there)
	for _, file := range files {
		infoJSONPath := strings.TrimSuffix(file, filepath.Ext(file)) + ".info.json"
		if data, err := os.ReadFile(infoJSONPath); err == nil {
			var infoData map[string]interface{}
			if json.Unmarshal(data, &infoData) == nil {
				meta = d.buildRichMetadata(infoData, download.URL, files)
				break
			}
		}
	}

	// If no .info.json found, build minimal metadata
	if meta == nil {
		meta = d.buildMinimalMetadata(download.URL, files)
	}

	data, err := json.Marshal(meta.ToMap())
	if err != nil {
		return err
	}

	download.Metadata = string(data)
	return nil
}

// buildRichMetadata extracts and formats rich metadata from yt-dlp's .info.json
func (d *TwitterDownloader) buildRichMetadata(infoData map[string]interface{}, url string, files []string) *domain.MediaMetadata {
	// Handle timestamp and upload_date
	timestamp := int64(time.Now().Unix())
	uploadDate := time.Now().Format("20060102")

	if ts, ok := infoData["timestamp"].(float64); ok {
		timestamp = int64(ts)
		uploadDate = time.Unix(int64(ts), 0).Format("20060102")
	}

	// Handle tags
	var tags []string
	if tagsRaw, ok := infoData["tags"].([]interface{}); ok {
		for _, tag := range tagsRaw {
			if tagStr, ok := tag.(string); ok {
				tags = append(tags, tagStr)
			}
		}
	}
	tags = append(tags, "x", "twitter")

	webpageURL := GetStringFromMap(infoData, "webpage_url")
	if webpageURL == "" {
		webpageURL = url
	}

	return &domain.MediaMetadata{
		ID:           GetStringFromMap(infoData, "id"),
		Title:        GetStringFromMap(infoData, "title"),
		Description:  GetStringFromMap(infoData, "description"),
		Uploader:     GetStringFromMap(infoData, "uploader"),
		UploaderID:   GetStringFromMap(infoData, "uploader_id"),
		UploaderURL:  GetStringFromMap(infoData, "uploader_url"),
		WebpageURL:   webpageURL,
		URL:          url,
		Timestamp:    timestamp,
		UploadDate:   uploadDate,
		Tags:         tags,
		Platform:     "x",
		Extractor:    GetStringFromMap(infoData, "extractor"),
		ExtractorKey: GetStringFromMap(infoData, "extractor_key"),
		Extension:    GetStringFromMap(infoData, "ext"),
		Files:        files,
	}
}

// buildMinimalMetadata creates basic metadata when .info.json is not available
func (d *TwitterDownloader) buildMinimalMetadata(url string, files []string) *domain.MediaMetadata {
	// Extract username and tweet ID from URL
	// URL format: https://x.com/{username}/status/{tweet_id}
	username := ""
	tweetID := ""

	urlWithoutProtocol := strings.TrimPrefix(url, "https://")
	urlWithoutProtocol = strings.TrimPrefix(urlWithoutProtocol, "http://")

	parts := strings.Split(urlWithoutProtocol, "/")
	if len(parts) >= 4 {
		username = parts[1]
		lastPart := parts[len(parts)-1]
		if idx := strings.Index(lastPart, "?"); idx > 0 {
			lastPart = lastPart[:idx]
		}
		tweetID = lastPart
	}

	return &domain.MediaMetadata{
		ID:           tweetID,
		Title:        fmt.Sprintf("%s_%s", username, tweetID),
		Uploader:     username,
		UploaderID:   username,
		URL:          url,
		Timestamp:    time.Now().Unix(),
		UploadDate:   time.Now().Format("20060102"),
		Tags:         []string{"x", "twitter"},
		Platform:     "x",
		Extractor:    "x",
		ExtractorKey: "X",
		Files:        files,
	}
}
