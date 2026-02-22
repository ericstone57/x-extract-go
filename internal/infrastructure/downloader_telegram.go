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

// Log file path for download output (stdout and stderr combined)
const DownloadLogFile = "download-%s.log"

// TelegramExportData represents the structure of tdl chat export JSON
type TelegramExportData struct {
	ID       int64                 `json:"id"`
	Messages []TelegramMessageData `json:"messages"`
}

// TelegramMessageData represents a single message in the export
type TelegramMessageData struct {
	ID   int                 `json:"id"`
	Type string              `json:"type"`
	File string              `json:"file"`
	Date int64               `json:"date"`
	Text string              `json:"text"`
	Raw  *TelegramRawMessage `json:"raw,omitempty"` // Raw message data when --raw flag is used
}

// TelegramRawMessage represents the raw Telegram message structure from tdl --raw export
// This is a subset of the tg.Message structure from gotd/td
type TelegramRawMessage struct {
	FromID     *TelegramPeerUser `json:"from_id,omitempty"`     // Sender info
	PeerID     *TelegramPeerInfo `json:"peer_id,omitempty"`     // Chat/channel info
	PostAuthor string            `json:"post_author,omitempty"` // Author signature for channel posts
}

// TelegramPeerUser represents a user peer in Telegram
type TelegramPeerUser struct {
	UserID int64 `json:"user_id,omitempty"`
}

// TelegramPeerInfo represents peer information (can be user, chat, or channel)
type TelegramPeerInfo struct {
	ChannelID int64 `json:"channel_id,omitempty"`
	ChatID    int64 `json:"chat_id,omitempty"`
	UserID    int64 `json:"user_id,omitempty"`
}

// TelegramDownloader implements Downloader for Telegram
type TelegramDownloader struct {
	config           *domain.TelegramConfig
	logsDir          string
	incomingDir      string
	completedDir     string
	eventLogger      *logger.MultiLogger // For structured events only (LogQueueEvent, LogAppError)
	channelRepo      domain.TelegramChannelRepository
	messageCacheRepo domain.TelegramMessageCacheRepository
}

// NewTelegramDownloader creates a new Telegram downloader
func NewTelegramDownloader(config *domain.TelegramConfig, incomingDir, completedDir, logsDir string, eventLogger *logger.MultiLogger) *TelegramDownloader {
	return &TelegramDownloader{
		config:       config,
		logsDir:      logsDir,
		incomingDir:  incomingDir,
		completedDir: completedDir,
		eventLogger:  eventLogger,
	}
}

// SetChannelRepository sets the channel repository for channel name lookups
func (d *TelegramDownloader) SetChannelRepository(repo domain.TelegramChannelRepository) {
	d.channelRepo = repo
}

// SetMessageCacheRepository sets the message cache repository
func (d *TelegramDownloader) SetMessageCacheRepository(repo domain.TelegramMessageCacheRepository) {
	d.messageCacheRepo = repo
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
		return err
	}

	// Update channel list if needed (for channel name lookups in metadata)
	// This runs once every 7 days and won't block downloads if it fails
	d.UpdateChannelListIfNeeded()

	// Check if this is a re-download of a previously completed download
	// If files were deleted by user, we should not re-download them
	existingFiles := d.getExistingDownloadedFiles(download)
	if len(existingFiles) > 0 {
		// Some files from previous download still exist
		// Check if ALL files from metadata still exist
		allExist, missingFiles := d.checkFilesExist(existingFiles)
		if allExist {
			// All files exist, nothing to download
			download.FilePath = existingFiles[0]
			return nil
		}
		// Some files are missing (user deleted them intentionally)
		// Update metadata to reflect the current state and skip download
		_ = missingFiles // Log if needed
		download.FilePath = existingFiles[0]
		d.updateMetadataAfterPartialDeletion(download, existingFiles)
		return nil
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
	cmdLine := ShellEscapeCommand(d.config.TDLBinary, args...)
	d.writeLogHeader(downloadLog, download.ID, cmdLine)

	// Execute tdl with direct file redirect
	// Redirect both stdout and stderr to the same file (like cmd > file 2>&1)
	cmd := exec.Command(d.config.TDLBinary, args...)
	cmd.Stdout = downloadLog
	cmd.Stderr = downloadLog

	// Run command and check exit code
	err = cmd.Run()

	// Write completion marker and handle result
	if err != nil {
		d.writeLogFooter(downloadLog, false, fmt.Sprintf("tdl failed: %v", err))
		progressCallback("", -1) // Signal failure
		return fmt.Errorf("tdl failed: %w", err)
	}

	// Move files from temp to completed directory
	// Returns file paths and the actual message ID from the filename
	files, actualMsgID, err := d.moveDownloadedFiles(downloadTempDir, download.URL)
	if err != nil {
		d.writeLogFooter(downloadLog, false, fmt.Sprintf("Failed to move files: %v", err))
		return err
	}

	if len(files) == 0 {
		d.writeLogFooter(downloadLog, false, "No files downloaded")
		return fmt.Errorf("no files downloaded")
	}

	// Use the actual message ID from the filename if available (more accurate than URL)
	// This handles cases where tdl downloads a different message than expected
	messageURL := download.URL
	if actualMsgID != "" {
		channelID := extractTelegramChannel(download.URL)
		messageURL = fmt.Sprintf("https://t.me/c/%s/%s", channelID, actualMsgID)
		if d.eventLogger != nil {
			d.eventLogger.LogQueueEvent("telegram_actual_message_id",
				zap.String("download_id", download.ID),
				zap.String("url_message_id", extractTelegramID(download.URL)),
				zap.String("actual_message_id", actualMsgID))
		}
	}

	// Extract message content ONCE for all files (efficient for group downloads)
	// This avoids running tdl chat export multiple times
	messageData, err := d.extractMessageContent(messageURL)
	if err != nil {
		if d.eventLogger != nil {
			d.eventLogger.LogAppError("Failed to extract message content",
				zap.String("url", messageURL),
				zap.Error(err))
		}
		// Continue without message content - will use fallback metadata
	}

	// Create metadata for each file using shared message data
	for _, file := range files {
		if err := d.createMetadataFile(download.URL, file, messageData); err != nil {
			if d.eventLogger != nil {
				d.eventLogger.LogAppError("Failed to create metadata file", zap.String("file", file), zap.Error(err))
			}
		}
	}

	// Update download with file path (use first file if multiple)
	download.FilePath = files[0]

	// Build full metadata for the download record (includes title, description, uploader)
	metadata := d.buildDownloadMetadata(download.URL, messageData, files)
	data, _ := json.Marshal(metadata)
	download.Metadata = string(data)

	// Log successful completion
	d.writeLogFooter(downloadLog, true, fmt.Sprintf("Downloaded: %s", download.FilePath))
	progressCallback("", 100) // Signal success

	return nil
}

// openLogFile opens the download log file for today
// All output (stdout and stderr) goes to this single file
func (d *TelegramDownloader) openLogFile() (*os.File, error) {
	// Ensure logs directory exists
	if err := os.MkdirAll(d.logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	dateStr := time.Now().Format("20060102")
	downloadPath := filepath.Join(d.logsDir, fmt.Sprintf(DownloadLogFile, dateStr))
	return os.OpenFile(downloadPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}

// writeLogHeader writes the download start marker
func (d *TelegramDownloader) writeLogHeader(file *os.File, downloadID, cmdLine string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	file.WriteString(fmt.Sprintf("\n=== [%s] Download: %s ===\n", timestamp, downloadID))
	file.WriteString(fmt.Sprintf("$ %s\n", cmdLine))
}

// writeLogFooter writes the download end marker
func (d *TelegramDownloader) writeLogFooter(file *os.File, success bool, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	status := "SUCCESS"
	if !success {
		status = "FAILED"
	}
	file.WriteString(fmt.Sprintf("[%s] %s: %s\n", timestamp, status, message))
	file.WriteString("=== END ===\n\n")
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

	// Always skip files with the same name and size to avoid re-downloading
	args = append(args, "--skip-same")

	// Use takeout mode if configured (useful for large downloads)
	if d.config.Takeout {
		args = append(args, "--takeout")
		if d.eventLogger != nil {
			d.eventLogger.LogQueueEvent("telegram_takeout_mode",
				zap.String("download_id", download.ID),
				zap.String("url", download.URL))
		}
	}

	// Add extra parameters if configured
	if d.config.ExtraParams != "" {
		extraArgs := strings.Fields(d.config.ExtraParams)
		args = append(args, extraArgs...)
	}

	return args
}

// moveDownloadedFiles moves files from temp directory to completed directory
// Returns both the file paths and the extracted message ID from the filename (if found)
func (d *TelegramDownloader) moveDownloadedFiles(tempDir, url string) ([]string, string, error) {
	var movedFiles []string

	// Ensure completed directory exists
	if err := os.MkdirAll(d.completedDir, 0755); err != nil {
		return nil, "", fmt.Errorf("failed to create completed directory: %w", err)
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

	if err != nil {
		return nil, "", err
	}

	// Extract actual message ID from the first file's filename
	// Format: {channel_id}_{message_id}_{media_id}.{ext}
	actualMsgID := ""
	if len(movedFiles) > 0 {
		actualMsgID = extractMessageIDFromFilename(filepath.Base(movedFiles[0]))
	}

	return movedFiles, actualMsgID, nil
}

// extractMessageIDFromFilename extracts the message ID from a Telegram downloaded filename
// Format: {channel_id}_{message_id}_{media_id}.{ext}
// Example: 3464638440_2685_6086895199301864978.jpg -> returns "2685"
func extractMessageIDFromFilename(filename string) string {
	// Remove extension
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	parts := strings.Split(name, "_")
	if len(parts) >= 2 {
		// Second part is the message ID
		return parts[1]
	}
	return ""
}

// createMetadataFile creates a JSON metadata file for a downloaded file
// The metadata structure is designed to be compatible with yt-dlp's .info.json format
// messageData is pre-extracted content for efficiency (especially for group downloads)
func (d *TelegramDownloader) createMetadataFile(url, filePath string, messageData *TelegramMessageData) error {
	metadataPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".info.json"

	// Prepare metadata fields with defaults
	messageID := extractTelegramID(url)
	channelID := extractTelegramChannel(url) // This is the numeric channel ID for private channels
	isPrivateChannel := isPrivateChannelURL(url)

	// Look up channel name from repository (falls back to channelID if not found)
	channelName := d.GetChannelName(channelID)

	// Default values - use channel name as fallback for uploader
	uploaderName := channelName
	uploaderID := channelID
	description := ""
	timestamp := time.Now().Unix()
	uploadDate := time.Now().Format("20060102")
	tags := []string{}

	// If we have message content, use it
	if messageData != nil {
		// Use actual message text as description (preserve original formatting)
		if messageData.Text != "" {
			description = messageData.Text
		}

		// Use message timestamp if available
		if messageData.Date > 0 {
			timestamp = messageData.Date
			uploadDate = time.Unix(messageData.Date, 0).Format("20060102")
		}

		// Extract hashtags from message text
		tags = extractHashtags(messageData.Text)

		// Extract sender/uploader info from raw message data
		uploaderName, uploaderID = extractSenderInfo(messageData, channelName)
	}

	// Add 'telegram' and channel name as tags
	tags = append(tags, "telegram")
	// if channelName != "" {
	// 	tags = append(tags, channelName)
	// }

	// Build title format: "{uploader_name}_{channel_name}_{message_id}"
	title := fmt.Sprintf("%s_%s_%s", uploaderName, channelName, messageID)

	// Build uploader format: "{channel_name}_{uploader_name}"
	// If uploader is same as channel, just use channel name
	uploader := channelName
	if uploaderName != channelName {
		uploader = fmt.Sprintf("%s_%s", channelName, uploaderName)
	}

	// Build URLs based on channel type (public vs private)
	// Note: URLs use channelID (numeric ID or username), not channelName
	var uploaderURL, webpageURL string
	if isPrivateChannel {
		// Private channel URLs include /c/ prefix
		uploaderURL = fmt.Sprintf("https://t.me/c/%s", channelID)
		webpageURL = fmt.Sprintf("https://t.me/c/%s/%s", channelID, messageID)
	} else {
		// Public channel URLs don't have /c/ prefix
		uploaderURL = fmt.Sprintf("https://t.me/%s", channelID)
		webpageURL = fmt.Sprintf("https://t.me/%s/%s", channelID, messageID)
	}

	// Build metadata in yt-dlp compatible format
	metadata := map[string]interface{}{
		// Core identification fields (yt-dlp compatible)
		"id":          messageID,
		"title":       title,
		"description": description,

		// Uploader fields (yt-dlp compatible)
		"uploader":     uploader,
		"uploader_id":  uploaderID,
		"uploader_url": uploaderURL,

		// URL fields (yt-dlp compatible)
		"webpage_url": webpageURL,

		// Timestamp fields (yt-dlp compatible)
		"timestamp":   timestamp,
		"upload_date": uploadDate,

		// Tags (yt-dlp compatible)
		"tags": tags,

		// Extractor info (yt-dlp compatible)
		"extractor":     "telegram",
		"extractor_key": "Telegram",

		// Additional metadata
		"ext":        strings.TrimPrefix(filepath.Ext(filePath), "."),
		"_type":      "video",
		"epoch":      timestamp,
		"local_file": filePath,
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0644)
}

// buildDownloadMetadata builds the metadata map for the download record (includes title, description, uploader)
func (d *TelegramDownloader) buildDownloadMetadata(url string, messageData *TelegramMessageData, files []string) map[string]interface{} {
	// Prepare metadata fields with defaults
	messageID := extractTelegramID(url)
	channelID := extractTelegramChannel(url)
	isPrivateChannel := isPrivateChannelURL(url)

	// Look up channel name from repository (falls back to channelID if not found)
	channelName := d.GetChannelName(channelID)

	// Default values - use channel name as fallback for uploader
	uploaderName := channelName
	uploaderID := channelID
	description := ""
	timestamp := time.Now().Unix()
	uploadDate := time.Now().Format("20060102")
	tags := []string{}

	// If we have message content, use it
	if messageData != nil {
		if messageData.Text != "" {
			description = messageData.Text
		}
		if messageData.Date > 0 {
			timestamp = messageData.Date
			uploadDate = time.Unix(messageData.Date, 0).Format("20060102")
		}
		tags = extractHashtags(messageData.Text)
		uploaderName, uploaderID = extractSenderInfo(messageData, channelName)
	}

	// Add 'telegram' as tag
	tags = append(tags, "telegram")

	// Build title format: "{uploader_name}_{channel_name}_{message_id}"
	title := fmt.Sprintf("%s_%s_%s", uploaderName, channelName, messageID)

	// Build uploader format
	uploader := channelName
	if uploaderName != channelName {
		uploader = fmt.Sprintf("%s_%s", channelName, uploaderName)
	}

	// Build URLs
	var uploaderURL, webpageURL string
	if isPrivateChannel {
		uploaderURL = fmt.Sprintf("https://t.me/c/%s", channelID)
		webpageURL = fmt.Sprintf("https://t.me/c/%s/%s", channelID, messageID)
	} else {
		uploaderURL = fmt.Sprintf("https://t.me/%s", channelID)
		webpageURL = fmt.Sprintf("https://t.me/%s/%s", channelID, messageID)
	}

	// Build metadata in yt-dlp compatible format
	metadata := map[string]interface{}{
		// Core identification fields
		"id":          messageID,
		"title":       title,
		"description": description,

		// Uploader fields
		"uploader":     uploader,
		"uploader_id":  uploaderID,
		"uploader_url": uploaderURL,

		// URL fields
		"webpage_url": webpageURL,

		// Timestamp fields
		"timestamp":   timestamp,
		"upload_date": uploadDate,

		// Tags
		"tags": tags,

		// Extractor info
		"extractor":     "telegram",
		"extractor_key": "Telegram",

		// Additional fields
		"url":      url,
		"platform": "telegram",
		"files":    files,
	}

	return metadata
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
// Handles both public and private channel URLs:
// - Public: https://t.me/channelname/messageid -> returns "channelname"
// - Private: https://t.me/c/1234567890/messageid -> returns "1234567890"
func extractTelegramChannel(url string) string {
	// URL format: https://t.me/channelname/messageid
	// or: https://t.me/c/channelid/messageid (private channels)
	parts := strings.Split(url, "/")
	if len(parts) >= 4 {
		// Check if this is a private channel URL (has /c/ prefix)
		if parts[3] == "c" && len(parts) >= 5 {
			// Private channel: https://t.me/c/1234567890/messageid
			return parts[4]
		}
		// Public channel: https://t.me/channelname/messageid
		return parts[3]
	}
	return "unknown"
}

// isPrivateChannelURL checks if a Telegram URL is for a private channel
// Private channel URLs have format: https://t.me/c/channelid/messageid
func isPrivateChannelURL(url string) bool {
	parts := strings.Split(url, "/")
	return len(parts) >= 5 && parts[3] == "c"
}

// extractSenderInfo extracts sender/uploader information from raw message data
// Returns (uploaderName, uploaderID) - uses channel as fallback if sender info unavailable
func extractSenderInfo(messageData *TelegramMessageData, channel string) (string, string) {
	// Default to channel name if no raw data available
	if messageData == nil || messageData.Raw == nil {
		return channel, channel
	}

	raw := messageData.Raw

	// Priority 1: Check for post_author (channel post signature)
	// This is the author name shown on channel posts (e.g., "John Doe" signature)
	if raw.PostAuthor != "" {
		// Clean the author name for use in filenames (replace spaces with underscores)
		cleanAuthor := strings.ReplaceAll(raw.PostAuthor, " ", "_")
		return cleanAuthor, cleanAuthor
	}

	// Priority 2: Check for from_id (sender user ID)
	// This is available for messages in groups/private chats
	if raw.FromID != nil && raw.FromID.UserID != 0 {
		// We have a user ID but not the username
		// Use the user ID as the uploader_id, channel as uploader_name
		userIDStr := fmt.Sprintf("%d", raw.FromID.UserID)
		return channel, userIDStr
	}

	// Fallback: use channel name
	return channel, channel
}

// extractMessageContent fetches message content from Telegram using tdl chat export
// It uses smart caching - exports all messages but only saves NEW messages to cache
func (d *TelegramDownloader) extractMessageContent(url string) (*TelegramMessageData, error) {
	channel := extractTelegramChannel(url)
	messageID := extractTelegramID(url)

	if channel == "unknown" || messageID == "unknown" {
		return nil, fmt.Errorf("invalid Telegram URL format")
	}

	// Check cache first if available
	if d.messageCacheRepo != nil {
		cached, err := d.messageCacheRepo.GetMessage(channel, messageID)
		if err == nil && cached != nil {
			// Cache hit - return cached data
			return &TelegramMessageData{
				ID:   parseMessageID(cached.MessageID),
				Text: cached.Text,
				Date: cached.Date,
				Raw: &TelegramRawMessage{
					FromID: &TelegramPeerUser{
						UserID: parseSenderID(cached.SenderID),
					},
				},
			}, nil
		}

		// Cache miss - check if we have existing cache for this channel
		hasCache, err := d.messageCacheRepo.HasChannelCache(channel)
		if err == nil && hasCache {
			// Channel has cache but message not found
			// Get all cached message IDs to filter them out later
			cachedIDs, _ := d.messageCacheRepo.GetCachedMessages(channel)

			if d.eventLogger != nil {
				d.eventLogger.LogQueueEvent("telegram_export_with_filter",
					zap.String("channel", channel),
					zap.String("message_id", messageID),
					zap.Int("cached_messages_count", len(cachedIDs)),
					zap.String("action", "Exporting all messages, saving only new ones"))
			}

			// Export all messages from channel, but only save NEW ones
			if err := d.exportAndSaveNewMessages(channel, cachedIDs); err != nil {
				if d.eventLogger != nil {
					d.eventLogger.LogAppError("Failed to export and save new messages", zap.Error(err))
				}
				// Fallback to single message export
			} else {
				// Try cache again
				cached, err := d.messageCacheRepo.GetMessage(channel, messageID)
				if err == nil && cached != nil {
					return &TelegramMessageData{
						ID:   parseMessageID(cached.MessageID),
						Text: cached.Text,
						Date: cached.Date,
						Raw: &TelegramRawMessage{
							FromID: &TelegramPeerUser{
								UserID: parseSenderID(cached.SenderID),
							},
						},
					}, nil
				}
			}
		} else {
			// No cache exists for this channel - export ALL messages and cache them
			if d.eventLogger != nil {
				d.eventLogger.LogQueueEvent("telegram_full_channel_export",
					zap.String("channel", channel),
					zap.String("message_id", messageID),
					zap.String("action", "Exporting all messages from channel and caching"))
			}
			if err := d.exportAndCacheAllMessages(channel); err != nil {
				if d.eventLogger != nil {
					d.eventLogger.LogAppError("Failed to export channel for cache", zap.Error(err))
				}
				// Fallback to single message export
			} else {
				// Try cache again
				cached, err := d.messageCacheRepo.GetMessage(channel, messageID)
				if err == nil && cached != nil {
					return &TelegramMessageData{
						ID:   parseMessageID(cached.MessageID),
						Text: cached.Text,
						Date: cached.Date,
						Raw: &TelegramRawMessage{
							FromID: &TelegramPeerUser{
								UserID: parseSenderID(cached.SenderID),
							},
						},
					}, nil
				}
			}
		}
	}

	// Final fallback: export single message (when no cache repo available)
	return d.exportMessageFromTelegram(channel, messageID)
}

// exportAndSaveNewMessages exports all messages from a channel but only saves
// messages that are not already in the cache. This is used when we have partial
// cache and want to add new messages without re-exporting cached ones.
// Note: tdl doesn't support date-based filtering via -j flag (it's for journal ID),
// so we export all messages and filter client-side.
func (d *TelegramDownloader) exportAndSaveNewMessages(channel string, cachedIDs map[string]bool) error {
	// Create temp file for export in incoming directory
	tempFile := filepath.Join(d.incomingDir, fmt.Sprintf("export_new_%s.json", channel))
	defer os.Remove(tempFile)

	// Build tdl chat export command for ALL messages
	args := []string{
		"-n", d.config.Profile,
		"--storage", fmt.Sprintf("type=%s,path=%s", d.config.StorageType, d.config.StoragePath),
		"chat", "export",
		"-c", channel,
		"--with-content",
		"--raw",
		"-o", tempFile,
	}

	// Execute tdl chat export
	cmd := exec.Command(d.config.TDLBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to export channel: %w, output: %s", err, string(output))
	}

	// Read and parse the export file
	data, err := os.ReadFile(tempFile)
	if err != nil {
		return fmt.Errorf("failed to read export file: %w", err)
	}

	var exportData TelegramExportData
	if err := json.Unmarshal(data, &exportData); err != nil {
		return fmt.Errorf("failed to parse export data: %w", err)
	}

	// Filter and convert only NEW messages (not in cache)
	newCaches := make([]domain.TelegramMessageCache, 0, len(exportData.Messages))
	newCount := 0
	for _, msg := range exportData.Messages {
		msgID := fmt.Sprintf("%d", msg.ID)
		if cachedIDs[msgID] {
			// Already cached, skip
			continue
		}
		cache := domain.TelegramMessageCache{
			ChannelID: channel,
			MessageID: msgID,
			Text:      msg.Text,
			Date:      msg.Date,
			SenderID:  formatSenderID(msg.Raw),
		}
		newCaches = append(newCaches, cache)
		newCount++
	}

	if len(newCaches) == 0 {
		// No new messages to save
		if d.eventLogger != nil {
			d.eventLogger.LogQueueEvent("telegram_no_new_messages",
				zap.String("channel", channel),
				zap.Int("cached_count", len(cachedIDs)))
		}
		return nil
	}

	// Bulk save only new messages to cache
	if err := d.messageCacheRepo.SaveMessages(newCaches); err != nil {
		return fmt.Errorf("failed to save new message cache: %w", err)
	}

	if d.eventLogger != nil {
		d.eventLogger.LogQueueEvent("telegram_new_messages_cached",
			zap.String("channel", channel),
			zap.Int("new_messages_cached", newCount),
			zap.Int("total_cached_before", len(cachedIDs)))
	}

	return nil
}

// exportAndCacheAllMessages exports ALL messages from a channel and saves them to cache
// This is used when there's no existing cache for the channel
func (d *TelegramDownloader) exportAndCacheAllMessages(channel string) error {
	// Create temp file for export in incoming directory
	tempFile := filepath.Join(d.incomingDir, fmt.Sprintf("export_all_%s.json", channel))
	defer os.Remove(tempFile)

	// Build tdl chat export command for ALL messages
	args := []string{
		"-n", d.config.Profile,
		"--storage", fmt.Sprintf("type=%s,path=%s", d.config.StorageType, d.config.StoragePath),
		"chat", "export",
		"-c", channel,
		"--with-content",
		"--raw",
		"-o", tempFile,
	}

	// Execute tdl chat export
	cmd := exec.Command(d.config.TDLBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to export channel: %w, output: %s", err, string(output))
	}

	// Read and parse the export file
	data, err := os.ReadFile(tempFile)
	if err != nil {
		return fmt.Errorf("failed to read export file: %w", err)
	}

	var exportData TelegramExportData
	if err := json.Unmarshal(data, &exportData); err != nil {
		return fmt.Errorf("failed to parse export data: %w", err)
	}

	// Convert all messages to cache entries
	caches := make([]domain.TelegramMessageCache, 0, len(exportData.Messages))
	for _, msg := range exportData.Messages {
		cache := domain.TelegramMessageCache{
			ChannelID: channel,
			MessageID: fmt.Sprintf("%d", msg.ID),
			Text:      msg.Text,
			Date:      msg.Date,
			SenderID:  formatSenderID(msg.Raw),
		}
		caches = append(caches, cache)
	}

	// Bulk save all messages to cache
	if err := d.messageCacheRepo.SaveMessages(caches); err != nil {
		return fmt.Errorf("failed to save message cache: %w", err)
	}

	if d.eventLogger != nil {
		d.eventLogger.LogQueueEvent("telegram_all_messages_cached",
			zap.String("channel", channel),
			zap.Int("messages_cached", len(caches)))
	}

	return nil
}

// parseMessageID converts a message ID string to int
func parseMessageID(id string) int {
	idInt, _ := strconv.Atoi(id)
	return idInt
}

// parseSenderID converts a sender ID string to int64
func parseSenderID(id string) int64 {
	idInt, _ := strconv.ParseInt(id, 10, 64)
	return idInt
}

// formatSenderID formats sender ID from Raw data
func formatSenderID(raw *TelegramRawMessage) string {
	if raw == nil || raw.FromID == nil {
		return ""
	}
	return fmt.Sprintf("%d", raw.FromID.UserID)
}

// extractSenderName extracts sender name from raw data
func extractSenderName(raw *TelegramRawMessage) string {
	// This is a placeholder - actual sender name extraction
	// would require additional data from the tdl export
	return ""
}

// exportMessageFromTelegram exports a single message from Telegram
func (d *TelegramDownloader) exportMessageFromTelegram(channel, messageID string) (*TelegramMessageData, error) {
	// Create temp file for export in incoming directory
	tempFile := filepath.Join(d.incomingDir, fmt.Sprintf("export_%s_%s.json", channel, messageID))
	defer os.Remove(tempFile)

	// Build tdl chat export command with --raw flag to get sender information
	args := []string{
		"-n", d.config.Profile,
		"--storage", fmt.Sprintf("type=%s,path=%s", d.config.StorageType, d.config.StoragePath),
		"chat", "export",
		"-c", channel,
		"-T", "id",
		"-i", messageID,
		"--with-content",
		"--raw", // Include raw message data to extract sender info
		"-o", tempFile,
	}

	// Execute tdl chat export
	cmd := exec.Command(d.config.TDLBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("failed to export message: %v, output: %s", err, string(output))
		if d.eventLogger != nil {
			d.eventLogger.LogAppError("telegram export failed",
				zap.String("channel", channel),
				zap.String("message_id", messageID),
				zap.String("error", errMsg))
		}
		return nil, fmt.Errorf("%s", errMsg)
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

// FetchChannelList executes `tdl chat ls` command and parses the output
// to extract channel ID and name mappings
func (d *TelegramDownloader) FetchChannelList() (map[string]*domain.TelegramChannel, error) {
	// Build tdl chat ls command
	args := []string{
		"-n", d.config.Profile,
		"--storage", fmt.Sprintf("type=%s,path=%s", d.config.StorageType, d.config.StoragePath),
		"chat", "ls",
	}

	cmd := exec.Command(d.config.TDLBinary, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute tdl chat ls: %w", err)
	}

	return parseTDLChatList(string(output))
}

// parseTDLChatList parses the output of `tdl chat ls` command
// Output format:
// ID         Type     VisibleName          Username             Topics
// 1454687932 group    秘密花园🏳️‍🌈            -                    -
// 3464638440 channel  a战士father2026.08   -                    -
func parseTDLChatList(output string) (map[string]*domain.TelegramChannel, error) {
	channels := make(map[string]*domain.TelegramChannel)

	scanner := bufio.NewScanner(strings.NewReader(output))
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// Skip header line
		if lineNum == 1 {
			continue
		}

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse the line - fields are separated by whitespace
		// ID Type VisibleName Username Topics
		// The challenge is that VisibleName can contain spaces and special characters
		// We'll use a more robust parsing approach

		channel := parseTDLChatLine(line)
		if channel != nil {
			channels[channel.ChannelID] = channel
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning tdl output: %w", err)
	}

	return channels, nil
}

// parseTDLChatLine parses a single line from `tdl chat ls` output
// Line format: ID Type VisibleName Username Topics
// Example: 3464638440 channel  a战士father2026.08   -                    -
func parseTDLChatLine(line string) *domain.TelegramChannel {
	// Split by whitespace, but we need to be careful about the VisibleName
	// which can contain spaces. The format appears to be fixed-width columns.

	// First, extract the ID (first field, all digits)
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return nil
	}

	channelID := fields[0]
	// Validate that ID is numeric
	if _, err := strconv.ParseInt(channelID, 10, 64); err != nil {
		return nil
	}

	channelType := fields[1]
	if channelType != "channel" && channelType != "group" && channelType != "private" {
		return nil
	}

	// The VisibleName is tricky - it's between Type and Username
	// Username is either a word or "-"
	// Topics is either "-" or a list

	// Strategy: Find the position after Type, and look for the Username pattern
	// Username is typically the second-to-last field before Topics if it's not "-"

	// Simpler approach: Since the columns seem to be aligned, we can try to
	// extract VisibleName by removing the first two fields and the last two fields

	// For now, use a simpler heuristic: everything between Type and the next "-" or username
	// Actually, looking at the output more carefully, the columns are space-padded

	// Let's try: join remaining fields, then find the pattern
	remaining := strings.Join(fields[2:], " ")

	// Find the last occurrence of " - " which separates Topics
	// and the second-to-last which separates Username
	// This is fragile, but let's try a different approach

	// Better approach: use the raw line and find column positions
	// Based on the header: ID, Type, VisibleName, Username, Topics
	// The columns seem to be roughly at positions: 0, 11, 20, 41, 62

	// Even simpler: Take everything after the type until we hit a pattern like
	// "  -" or "  username" at the end

	// Most reliable: split from the end
	// Topics is the last field (could be "-" or a long list)
	// Username is before Topics (could be "-" or a word)

	// Find the VisibleName - it's the third field potentially with spaces
	// We need to find where Username starts

	visibleName := ""
	username := "-"

	// Look for the pattern where we have "  -  " or "  word  -" at the end
	// The "-" for Topics is at the very end

	// Let's try regex to extract the fields
	// Pattern: ID Type VisibleName... Username Topics
	// Username is alphanumeric_ or "-"
	// Topics is "-" or "1: topic, 2: topic"

	// Use a regex to find username pattern before Topics
	// Pattern: (space)(alphanumeric+ or -)(space+)(-|Topics)$
	usernameRegex := regexp.MustCompile(`\s+(\S+)\s+(-|\d+:.*)$`)
	if match := usernameRegex.FindStringSubmatch(remaining); match != nil {
		username = match[1]
		// VisibleName is everything before the username match
		idx := strings.LastIndex(remaining, match[0])
		if idx > 0 {
			visibleName = strings.TrimSpace(remaining[:idx])
		}
	} else {
		// Fallback: just use the first part
		visibleName = fields[2]
	}

	// Clean up the visible name (remove trailing ellipsis from truncation)
	visibleName = strings.TrimSuffix(visibleName, "...")
	visibleName = strings.TrimSpace(visibleName)

	if visibleName == "" || visibleName == "-" {
		return nil
	}

	return &domain.TelegramChannel{
		ChannelID:   channelID,
		ChannelName: visibleName,
		ChannelType: channelType,
		Username:    username,
	}
}

// UpdateChannelListIfNeeded checks if the channel list needs updating and updates it if necessary
// This should be called before processing Telegram downloads
func (d *TelegramDownloader) UpdateChannelListIfNeeded() error {
	if d.channelRepo == nil {
		return nil // No repository configured, skip
	}

	shouldUpdate, err := d.channelRepo.ShouldUpdateChannelList(domain.ChannelUpdateMaxAge)
	if err != nil {
		if d.eventLogger != nil {
			d.eventLogger.LogAppError("failed to check if channel list needs updating", zap.Error(err))
		}
		return nil // Don't block downloads on this error
	}

	if !shouldUpdate {
		return nil
	}

	if d.eventLogger != nil {
		d.eventLogger.LogQueueEvent("telegram_channel_update_start",
			zap.String("reason", "channel list needs updating"))
	}

	channels, err := d.FetchChannelList()
	if err != nil {
		if d.eventLogger != nil {
			d.eventLogger.LogAppError("failed to fetch channel list", zap.Error(err))
		}
		return nil // Don't block downloads on this error
	}

	if err := d.channelRepo.UpdateChannelList(channels); err != nil {
		if d.eventLogger != nil {
			d.eventLogger.LogAppError("failed to update channel list in database", zap.Error(err))
		}
		return nil // Don't block downloads on this error
	}

	if d.eventLogger != nil {
		d.eventLogger.LogQueueEvent("telegram_channel_update_complete",
			zap.Int("channels_count", len(channels)))
	}

	return nil
}

// GetChannelName retrieves the channel name for a given channel ID from the repository
// Returns the channelID as fallback if not found or if repository is not configured
func (d *TelegramDownloader) GetChannelName(channelID string) string {
	if d.channelRepo == nil {
		return channelID
	}

	name, err := d.channelRepo.GetChannelName(channelID)
	if err != nil {
		if d.eventLogger != nil {
			d.eventLogger.LogAppError("failed to get channel name",
				zap.Error(err), zap.String("channel_id", channelID))
		}
		return channelID
	}

	if name == "" {
		return channelID
	}

	return name
}

// getExistingDownloadedFiles extracts the list of downloaded files from the download's metadata
// Returns an empty slice if no metadata exists or metadata is invalid
func (d *TelegramDownloader) getExistingDownloadedFiles(download *domain.Download) []string {
	if download.Metadata == "" {
		return nil
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(download.Metadata), &metadata); err != nil {
		return nil
	}

	filesRaw, ok := metadata["files"]
	if !ok {
		return nil
	}

	filesSlice, ok := filesRaw.([]interface{})
	if !ok {
		return nil
	}

	var files []string
	for _, f := range filesSlice {
		if fileStr, ok := f.(string); ok {
			files = append(files, fileStr)
		}
	}

	return files
}

// checkFilesExist checks if the given file paths exist on disk
// Returns (true, nil) if all files exist
// Returns (false, missingFiles) if some files are missing
func (d *TelegramDownloader) checkFilesExist(files []string) (allExist bool, missingFiles []string) {
	allExist = true
	for _, file := range files {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			allExist = false
			missingFiles = append(missingFiles, file)
		}
	}
	return allExist, missingFiles
}

// updateMetadataAfterPartialDeletion updates the download metadata to remove deleted files
func (d *TelegramDownloader) updateMetadataAfterPartialDeletion(download *domain.Download, remainingFiles []string) {
	metadata := map[string]interface{}{
		"url":      download.URL,
		"platform": download.Platform,
		"mode":     download.Mode,
		"files":    remainingFiles,
		"note":     "Some files were deleted by user after download",
	}
	data, _ := json.Marshal(metadata)
	download.Metadata = string(data)
}
