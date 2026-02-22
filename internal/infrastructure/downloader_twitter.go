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
	config       *domain.TwitterConfig
	logsDir      string
	incomingDir  string
	completedDir string
	eventLogger  *logger.MultiLogger // For structured events only (LogQueueEvent, LogAppError)
}

// NewTwitterDownloader creates a new Twitter downloader
func NewTwitterDownloader(config *domain.TwitterConfig, incomingDir, completedDir, logsDir string, eventLogger *logger.MultiLogger) *TwitterDownloader {
	return &TwitterDownloader{
		config:       config,
		logsDir:      logsDir,
		incomingDir:  incomingDir,
		completedDir: completedDir,
		eventLogger:  eventLogger,
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
	if d.config.CookieFile != "" && fileExists(d.config.CookieFile) {
		args = append(args, "--cookies", d.config.CookieFile)
	}

	args = append(args, download.URL)

	// Create default callback if nil
	if progressCallback == nil {
		progressCallback = func(output string, percent float64) {}
	}

	// Open log file for direct redirect (combines stdout and stderr like 2>&1)
	downloadLog, err := d.openLogFile()
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer downloadLog.Close()

	// Write command header to download log (with proper shell escaping for display)
	cmdLine := ShellEscapeCommand(d.config.YTDLPBinary, args...)
	d.writeLogHeader(downloadLog, download.ID, cmdLine)

	// Execute yt-dlp with direct file redirect
	// Redirect both stdout and stderr to the same file (like cmd > file 2>&1)
	cmd := exec.Command(d.config.YTDLPBinary, args...)
	cmd.Stdout = downloadLog
	cmd.Stderr = downloadLog

	// Run command and check exit code
	err = cmd.Run()

	// Write completion marker
	if err != nil {
		d.writeLogFooter(downloadLog, false, fmt.Sprintf("yt-dlp failed: %v", err))
		progressCallback("", -1) // Signal failure
		return fmt.Errorf("yt-dlp failed: %w", err)
	}

	// Find downloaded files in incoming directory
	files, err := d.findDownloadedFiles(download.URL)
	if err != nil {
		d.writeLogFooter(downloadLog, false, fmt.Sprintf("Failed to find files: %v", err))
		return err
	}

	if len(files) == 0 {
		d.writeLogFooter(downloadLog, false, "No files downloaded")
		return fmt.Errorf("no files downloaded")
	}

	// Move files from incoming to completed directory
	completedFiles, err := d.moveToCompleted(files)
	if err != nil {
		d.writeLogFooter(downloadLog, false, fmt.Sprintf("Failed to move files: %v", err))
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
	d.writeLogFooter(downloadLog, true, fmt.Sprintf("Downloaded: %s", download.FilePath))
	progressCallback("", 100) // Signal success

	return nil
}

// openLogFile opens the download log file for today
// All output (stdout and stderr) goes to this single file
func (d *TwitterDownloader) openLogFile() (*os.File, error) {
	// Ensure logs directory exists
	if err := os.MkdirAll(d.logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	dateStr := time.Now().Format("20060102")
	downloadPath := filepath.Join(d.logsDir, "download-"+dateStr+".log")
	return os.OpenFile(downloadPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}

// writeLogHeader writes the download start marker
func (d *TwitterDownloader) writeLogHeader(file *os.File, downloadID, cmdLine string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	file.WriteString(fmt.Sprintf("\n=== [%s] Download: %s ===\n", timestamp, downloadID))
	file.WriteString(fmt.Sprintf("$ %s\n", cmdLine))
}

// writeLogFooter writes the download end marker
func (d *TwitterDownloader) writeLogFooter(file *os.File, success bool, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	status := "SUCCESS"
	if !success {
		status = "FAILED"
	}
	file.WriteString(fmt.Sprintf("[%s] %s: %s\n", timestamp, status, message))
	file.WriteString("=== END ===\n\n")
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
		ext := strings.ToLower(filepath.Ext(path))
		isMedia := ext == ".mp4" || ext == ".mkv" || ext == ".avi" || ext == ".mov" || ext == ".webm" || ext == ".m4v" || ext == ".jpg" || ext == ".png" || ext == ".gif" || ext == ".webp"
		// Only include media files, NOT .info.json files (we'll handle those separately)
		if !info.IsDir() && isMedia && !strings.HasSuffix(path, ".info.json") {
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
			if err := copyFile(file, destPath); err != nil {
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
	var richMetadata map[string]interface{}

	// Look for .info.json files in the completed directory (files have been moved there)
	for _, file := range files {
		infoJSONPath := strings.TrimSuffix(file, filepath.Ext(file)) + ".info.json"
		if data, err := os.ReadFile(infoJSONPath); err == nil {
			// Parse the yt-dlp info JSON
			var infoData map[string]interface{}
			if json.Unmarshal(data, &infoData) == nil {
				// Copy relevant fields to our metadata format
				richMetadata = d.buildRichMetadata(infoData, download.URL, files)
				break
			}
		}
	}

	// If no .info.json found, build minimal metadata
	if richMetadata == nil {
		richMetadata = d.buildMinimalMetadata(download.URL, files)
	}

	data, err := json.Marshal(richMetadata)
	if err != nil {
		return err
	}

	download.Metadata = string(data)
	return nil
}

// buildRichMetadata extracts and formats rich metadata from yt-dlp's .info.json
func (d *TwitterDownloader) buildRichMetadata(infoData map[string]interface{}, url string, files []string) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Extract fields from yt-dlp info, with fallbacks
	id := getStringFromMap(infoData, "id")
	title := getStringFromMap(infoData, "title")
	description := getStringFromMap(infoData, "description")
	uploader := getStringFromMap(infoData, "uploader")
	uploaderID := getStringFromMap(infoData, "uploader_id")
	uploaderURL := getStringFromMap(infoData, "uploader_url")
	webpageURL := getStringFromMap(infoData, "webpage_url")

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

	// Add 'x' and 'twitter' as tags
	tags = append(tags, "x", "twitter")

	// Build metadata in a format consistent with Telegram
	metadata["id"] = id
	metadata["title"] = title
	metadata["description"] = description

	// Uploader fields
	metadata["uploader"] = uploader
	metadata["uploader_id"] = uploaderID
	metadata["uploader_url"] = uploaderURL

	// URL fields
	metadata["webpage_url"] = webpageURL
	if webpageURL == "" {
		metadata["webpage_url"] = url
	}

	// Timestamp fields
	metadata["timestamp"] = timestamp
	metadata["upload_date"] = uploadDate

	// Tags
	metadata["tags"] = tags

	// Extractor info
	metadata["extractor"] = getStringFromMap(infoData, "extractor")
	metadata["extractor_key"] = getStringFromMap(infoData, "extractor_key")

	// Additional fields
	metadata["url"] = url
	metadata["platform"] = "x"
	metadata["files"] = files

	// Add ext if available
	if ext, ok := infoData["ext"].(string); ok {
		metadata["ext"] = ext
	}

	return metadata
}

// buildMinimalMetadata creates basic metadata when .info.json is not available
func (d *TwitterDownloader) buildMinimalMetadata(url string, files []string) map[string]interface{} {
	// Extract username from URL
	// URL format: https://x.com/{username}/status/{tweet_id} or https://twitter.com/{username}/status/{tweet_id}
	// After removing protocol, parts should be: ["x.com", "username", "status", "tweet_id"]
	username := ""
	uploaderID := ""
	tweetID := ""

	// Remove protocol prefix
	urlWithoutProtocol := strings.TrimPrefix(url, "https://")
	urlWithoutProtocol = strings.TrimPrefix(urlWithoutProtocol, "http://")

	parts := strings.Split(urlWithoutProtocol, "/")
	if len(parts) >= 4 {
		// parts[0] = "x.com" or "twitter.com"
		// parts[1] = username
		// parts[2] = "status"
		// parts[3] = tweet_id
		username = parts[1]
		uploaderID = username
		lastPart := parts[len(parts)-1]
		if idx := strings.Index(lastPart, "?"); idx > 0 {
			lastPart = lastPart[:idx]
		}
		tweetID = lastPart
	}

	// Generate title from uploader_id and tweet ID
	title := fmt.Sprintf("%s_%s", uploaderID, tweetID)

	timestamp := int64(time.Now().Unix())
	uploadDate := time.Now().Format("20060102")

	return map[string]interface{}{
		"id":            tweetID,
		"title":         title,
		"description":   "",
		"uploader":      username,
		"uploader_id":   uploaderID,
		"timestamp":     timestamp,
		"upload_date":   uploadDate,
		"tags":          []string{"x", "twitter"},
		"extractor":     "x",
		"extractor_key": "X",
		"url":           url,
		"platform":      "x",
		"files":         files,
	}
}

// getStringFromMap safely extracts a string from a map
func getStringFromMap(data map[string]interface{}, key string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return ""
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isMediaFile checks if a file is a media file (excluding .info.json metadata files)
func isMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	// Note: .json is intentionally excluded here - .info.json files are metadata
	// and should be handled separately by storeMetadata
	mediaExts := []string{".mp4", ".mkv", ".avi", ".mov", ".webm", ".m4v", ".jpg", ".png", ".gif", ".webp"}
	for _, mediaExt := range mediaExts {
		if ext == mediaExt {
			return true
		}
	}
	return false
}
