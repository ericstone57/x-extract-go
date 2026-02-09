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
	"time"

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
		"-o", "%(uploader_id)s_%(id)s.%(ext)s",
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
