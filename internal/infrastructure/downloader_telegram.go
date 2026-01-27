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

	"go.uber.org/zap"

	"github.com/yourusername/x-extract-go/internal/domain"
	"github.com/yourusername/x-extract-go/pkg/logger"
)

// TelegramExportData represents the structure of tdl chat export JSON
type TelegramExportData struct {
	ID       int64                 `json:"id"`
	Messages []TelegramMessageData `json:"messages"`
}

// TelegramMessageData represents a single message in the export
type TelegramMessageData struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	File string `json:"file"`
	Date int64  `json:"date"`
	Text string `json:"text"`
}

// TelegramDownloader implements Downloader for Telegram
type TelegramDownloader struct {
	config       *domain.TelegramConfig
	multiLogger  *logger.MultiLogger
	incomingDir  string
	completedDir string
}

// NewTelegramDownloader creates a new Telegram downloader
func NewTelegramDownloader(config *domain.TelegramConfig, incomingDir, completedDir string, multiLogger *logger.MultiLogger) *TelegramDownloader {
	return &TelegramDownloader{
		config:       config,
		multiLogger:  multiLogger,
		incomingDir:  incomingDir,
		completedDir: completedDir,
	}
}

// Platform returns the platform this downloader handles
func (d *TelegramDownloader) Platform() domain.Platform {
	return domain.PlatformTelegram
}

// Validate validates if the downloader can handle the given URL
func (d *TelegramDownloader) Validate(url string) error {
	if !strings.HasPrefix(url, "https://t.me") {
		return fmt.Errorf("invalid Telegram URL: %s", url)
	}
	return nil
}

// Download downloads media from Telegram
func (d *TelegramDownloader) Download(download *domain.Download, progressCallback domain.DownloadProgressCallback) error {
	// Validate URL
	if err := d.Validate(download.URL); err != nil {
		if d.multiLogger != nil {
			d.multiLogger.WriteDownloadComplete(download.ID, false, fmt.Sprintf("Invalid URL: %v", err))
		}
		return err
	}

	// Create temp directory for this download in incoming directory
	downloadTempDir := filepath.Join(d.incomingDir, "temp_"+download.ID)
	if err := os.MkdirAll(downloadTempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(downloadTempDir)

	// Ensure incoming directory exists
	if err := os.MkdirAll(d.incomingDir, 0755); err != nil {
		return fmt.Errorf("failed to create incoming directory: %w", err)
	}

	// Build tdl command
	args := d.buildTDLCommand(download, downloadTempDir)

	// Create progress callback for real-time output
	if progressCallback == nil {
		progressCallback = func(output string, percent float64) {}
	}

	// Write command to download log
	cmdLine := fmt.Sprintf("%s %s", d.config.TDLBinary, strings.Join(args, " "))
	if d.multiLogger != nil {
		d.multiLogger.WriteDownloadCommand(download.ID, cmdLine)
	}

	// Execute tdl with real-time output capture
	cmd := exec.Command(d.config.TDLBinary, args...)
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
			d.multiLogger.WriteDownloadComplete(download.ID, false, fmt.Sprintf("Failed to start tdl: %v", err))
		}
		return fmt.Errorf("failed to start tdl: %w", err)
	}

	// Read stdout line by line - write raw output to download log
	done := make(chan struct{})
	go func() {
		defer close(done)
		stdoutScanner := bufio.NewScanner(stdout)
		for stdoutScanner.Scan() {
			line := stdoutScanner.Text()
			percent := parseTDLProgress(line)
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
			d.multiLogger.WriteDownloadComplete(download.ID, false, fmt.Sprintf("tdl failed: %v", err))
		}
		return fmt.Errorf("tdl failed: %w", err)
	}

	// Move files from temp to completed directory
	files, err := d.moveDownloadedFiles(downloadTempDir, download.URL)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		if d.multiLogger != nil {
			d.multiLogger.WriteDownloadComplete(download.ID, false, "No files downloaded")
		}
		return fmt.Errorf("no files downloaded")
	}

	// Create metadata for each file
	for _, file := range files {
		if err := d.createMetadataFile(download.URL, file); err != nil {
			if d.multiLogger != nil {
				d.multiLogger.LogAppError("Failed to create metadata file", zap.String("file", file), zap.Error(err))
			}
		}
	}

	// Update download with file path (use first file if multiple)
	download.FilePath = files[0]

	// Store metadata
	metadata := map[string]interface{}{
		"url":      download.URL,
		"platform": download.Platform,
		"mode":     download.Mode,
		"files":    files,
	}
	data, _ := json.Marshal(metadata)
	download.Metadata = string(data)

	// Log successful completion
	if d.multiLogger != nil {
		d.multiLogger.WriteDownloadComplete(download.ID, true, fmt.Sprintf("Downloaded: %s", download.FilePath))
	}

	return nil
}

// buildTDLCommand builds the tdl command with appropriate flags
func (d *TelegramDownloader) buildTDLCommand(download *domain.Download, tempDir string) []string {
	args := []string{
		"-n", d.config.Profile,
		"--storage", fmt.Sprintf("type=%s,path=%s", d.config.StorageType, d.config.StoragePath),
		"dl",
		"-u", download.URL,
		"-d", tempDir,
	}

	// Determine if we should use --group flag
	useGroup := d.config.UseGroup
	switch download.Mode {
	case domain.ModeSingle:
		useGroup = false
	case domain.ModeGroup:
		useGroup = true
	}

	if useGroup {
		args = append(args, "--group")
	}

	if d.config.RewriteExt {
		args = append(args, "--rewrite-ext")
	}

	// Add extra parameters if configured
	if d.config.ExtraParams != "" {
		extraArgs := strings.Fields(d.config.ExtraParams)
		args = append(args, extraArgs...)
	}

	return args
}

// moveDownloadedFiles moves files from temp directory to completed directory
func (d *TelegramDownloader) moveDownloadedFiles(tempDir, url string) ([]string, error) {
	var movedFiles []string

	// Ensure completed directory exists
	if err := os.MkdirAll(d.completedDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create completed directory: %w", err)
	}

	err := filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && isMediaFile(path) {
			filename := filepath.Base(path)
			destPath := filepath.Join(d.completedDir, filename)

			// Move file
			if err := os.Rename(path, destPath); err != nil {
				// If rename fails, try copy and delete
				if err := copyFile(path, destPath); err != nil {
					return fmt.Errorf("failed to move file: %w", err)
				}
				os.Remove(path)
			}

			movedFiles = append(movedFiles, destPath)
		}

		return nil
	})

	return movedFiles, err
}

// createMetadataFile creates a JSON metadata file for a downloaded file
func (d *TelegramDownloader) createMetadataFile(url, filePath string) error {
	metadataPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".info.json"

	// Extract message content from Telegram
	messageData, err := d.extractMessageContent(url)

	// Prepare metadata fields with defaults
	messageID := extractTelegramID(url)
	channel := extractTelegramChannel(url)
	title := fmt.Sprintf("Telegram Media - %s", filepath.Base(filePath))
	description := fmt.Sprintf("Downloaded from Telegram: %s", url)
	uploader := "Telegram User"
	timestamp := time.Now().Unix()
	uploadDate := time.Now().Format("20060102")
	tags := []string{"telegram"}

	// If we successfully extracted message content, use it
	if err == nil && messageData != nil {
		// Create title in format: "Username + Channel + Message {MessageID}"
		title = fmt.Sprintf("%s + %s + Message %s", uploader, channel, messageID)

		// Use actual message text as description
		if messageData.Text != "" {
			description = strings.TrimSpace(messageData.Text)
		}

		// Use message timestamp if available
		if messageData.Date > 0 {
			timestamp = messageData.Date
			uploadDate = time.Unix(messageData.Date, 0).Format("20060102")
		}

		// Extract hashtags from message text
		tags = extractHashtags(messageData.Text)
		if len(tags) == 0 {
			tags = []string{"telegram"}
		}

		// Logging removed - no structured logger in refactored code
	} else {
		// Using fallback metadata (no logging needed)
	}

	metadata := map[string]interface{}{
		"id":            messageID,
		"title":         title,
		"description":   description,
		"uploader":      channel,
		"timestamp":     timestamp,
		"uploader_url":  fmt.Sprintf("https://t.me/%s", channel),
		"tags":          tags,
		"webpage_url":   url,
		"extractor":     "telegram",
		"extractor_key": "Telegram",
		"upload_date":   uploadDate,
		"epoch":         timestamp,
		"ext":           strings.TrimPrefix(filepath.Ext(filePath), "."),
		"_type":         "video",
		"source":        "telegram",
		"local_file":    filePath,
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0644)
}

// extractHashtags extracts hashtags from message text
func extractHashtags(text string) []string {
	re := regexp.MustCompile(`#\w+`)
	matches := re.FindAllString(text, -1)

	// Remove duplicates and convert to lowercase
	seen := make(map[string]bool)
	var tags []string
	for _, tag := range matches {
		tagLower := strings.ToLower(tag)
		if !seen[tagLower] {
			seen[tagLower] = true
			tags = append(tags, tagLower)
		}
	}

	return tags
}

// extractTelegramID extracts ID from Telegram URL
func extractTelegramID(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "unknown"
}

// extractTelegramChannel extracts channel/chat name from Telegram URL
func extractTelegramChannel(url string) string {
	// URL format: https://t.me/channelname/messageid
	parts := strings.Split(url, "/")
	if len(parts) >= 4 {
		return parts[3]
	}
	return "unknown"
}

// extractMessageContent fetches message content from Telegram using tdl chat export
func (d *TelegramDownloader) extractMessageContent(url string) (*TelegramMessageData, error) {
	channel := extractTelegramChannel(url)
	messageID := extractTelegramID(url)

	if channel == "unknown" || messageID == "unknown" {
		return nil, fmt.Errorf("invalid Telegram URL format")
	}

	// Create temp file for export in incoming directory
	tempFile := filepath.Join(d.incomingDir, fmt.Sprintf("export_%s_%s.json", channel, messageID))
	defer os.Remove(tempFile)

	// Build tdl chat export command
	args := []string{
		"-n", d.config.Profile,
		"--storage", fmt.Sprintf("type=%s,path=%s", d.config.StorageType, d.config.StoragePath),
		"chat", "export",
		"-c", channel,
		"-T", "id",
		"-i", messageID,
		"--with-content",
		"-o", tempFile,
	}

	// Execute tdl chat export
	cmd := exec.Command(d.config.TDLBinary, args...)
	_, err := cmd.CombinedOutput()
	if err != nil {
		// Chat export failed, caller will use fallback metadata
		return nil, err
	}

	// Read and parse the export file
	data, err := os.ReadFile(tempFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read export file: %w", err)
	}

	var exportData TelegramExportData
	if err := json.Unmarshal(data, &exportData); err != nil {
		return nil, fmt.Errorf("failed to parse export data: %w", err)
	}

	// Find the message with matching ID
	for _, msg := range exportData.Messages {
		if fmt.Sprintf("%d", msg.ID) == messageID {
			return &msg, nil
		}
	}

	return nil, fmt.Errorf("message not found in export")
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// parseTDLProgress parses tdl output to extract progress percentage
func parseTDLProgress(line string) float64 {
	// Match patterns like: "Downloading: filename.mp4 45.3% (12.34 MB / 27.18 MB) - 1.23 MB/s"
	tdlProgressRegex := regexp.MustCompile(`([\d.]+)%`)
	if match := tdlProgressRegex.FindStringSubmatch(line); match != nil {
		percent, _ := strconv.ParseFloat(match[1], 64)
		return percent
	}
	return -1
}
