package infrastructure

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yourusername/x-extract-go/internal/domain"
	"go.uber.org/zap"
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
	config  *domain.TelegramConfig
	logger  *zap.Logger
	baseDir string
	tempDir string
}

// NewTelegramDownloader creates a new Telegram downloader
func NewTelegramDownloader(config *domain.TelegramConfig, baseDir, tempDir string, logger *zap.Logger) *TelegramDownloader {
	return &TelegramDownloader{
		config:  config,
		logger:  logger,
		baseDir: baseDir,
		tempDir: tempDir,
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
func (d *TelegramDownloader) Download(download *domain.Download) error {
	d.logger.Info("Starting Telegram download",
		zap.String("url", download.URL),
		zap.String("id", download.ID),
		zap.String("mode", string(download.Mode)))

	// Validate URL
	if err := d.Validate(download.URL); err != nil {
		return err
	}

	// Create temp directory for this download
	downloadTempDir := filepath.Join(d.tempDir, download.ID)
	if err := os.MkdirAll(downloadTempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(downloadTempDir)

	// Ensure base directory exists
	if err := os.MkdirAll(d.baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}

	// Build tdl command
	args := d.buildTDLCommand(download, downloadTempDir)

	// Execute tdl
	cmd := exec.Command(d.config.TDLBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		d.logger.Error("tdl failed",
			zap.String("url", download.URL),
			zap.Error(err),
			zap.String("output", string(output)))
		return fmt.Errorf("tdl failed: %w - %s", err, string(output))
	}

	d.logger.Info("tdl completed",
		zap.String("url", download.URL),
		zap.String("output", string(output)))

	// Move files from temp to base directory
	files, err := d.moveDownloadedFiles(downloadTempDir, download.URL)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no files downloaded")
	}

	// Create metadata for each file
	for _, file := range files {
		if err := d.createMetadataFile(download.URL, file); err != nil {
			d.logger.Warn("Failed to create metadata file",
				zap.String("file", file),
				zap.Error(err))
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

// moveDownloadedFiles moves files from temp directory to base directory
func (d *TelegramDownloader) moveDownloadedFiles(tempDir, url string) ([]string, error) {
	var movedFiles []string

	err := filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && isMediaFile(path) {
			filename := filepath.Base(path)
			destPath := filepath.Join(d.baseDir, filename)

			// Move file
			if err := os.Rename(path, destPath); err != nil {
				// If rename fails, try copy and delete
				if err := copyFile(path, destPath); err != nil {
					return fmt.Errorf("failed to move file: %w", err)
				}
				os.Remove(path)
			}

			d.logger.Info("Moved file",
				zap.String("from", path),
				zap.String("to", destPath))

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

		d.logger.Info("Extracted message content",
			zap.String("url", url),
			zap.String("title", title),
			zap.String("description", description))
	} else {
		d.logger.Warn("Using fallback metadata",
			zap.String("url", url),
			zap.Error(err))
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

	// Create temp file for export
	tempFile := filepath.Join(d.tempDir, fmt.Sprintf("export_%s_%s.json", channel, messageID))
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		d.logger.Warn("tdl chat export failed, will use fallback metadata",
			zap.String("url", url),
			zap.Error(err),
			zap.String("output", string(output)))
		return nil, err
	}

	d.logger.Debug("tdl chat export completed",
		zap.String("url", url),
		zap.String("output", string(output)))

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
