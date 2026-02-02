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
	config       *domain.TelegramConfig
	multiLogger  *logger.MultiLogger
	incomingDir  string
	completedDir string
	channelRepo  domain.TelegramChannelRepository
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

// SetChannelRepository sets the channel repository for channel name lookups
func (d *TelegramDownloader) SetChannelRepository(repo domain.TelegramChannelRepository) {
	d.channelRepo = repo
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

	// Update channel list if needed (for channel name lookups in metadata)
	// This runs once every 7 days and won't block downloads if it fails
	d.UpdateChannelListIfNeeded()

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
// The metadata structure is designed to be compatible with yt-dlp's .info.json format
func (d *TelegramDownloader) createMetadataFile(url, filePath string) error {
	metadataPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".info.json"

	// Extract message content from Telegram (with --raw flag for sender info)
	messageData, err := d.extractMessageContent(url)

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

	// If we successfully extracted message content, use it
	if err == nil && messageData != nil {
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
	if channelName != "" {
		tags = append(tags, channelName)
	}

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
func (d *TelegramDownloader) extractMessageContent(url string) (*TelegramMessageData, error) {
	channel := extractTelegramChannel(url)
	messageID := extractTelegramID(url)

	if channel == "unknown" || messageID == "unknown" {
		return nil, fmt.Errorf("invalid Telegram URL format")
	}

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
		if d.multiLogger != nil {
			d.multiLogger.LogAppError("failed to check if channel list needs updating", zap.Error(err))
		}
		return nil // Don't block downloads on this error
	}

	if !shouldUpdate {
		return nil
	}

	if d.multiLogger != nil {
		d.multiLogger.LogQueueEvent("telegram_channel_update_start",
			zap.String("reason", "channel list needs updating"))
	}

	channels, err := d.FetchChannelList()
	if err != nil {
		if d.multiLogger != nil {
			d.multiLogger.LogAppError("failed to fetch channel list", zap.Error(err))
		}
		return nil // Don't block downloads on this error
	}

	if err := d.channelRepo.UpdateChannelList(channels); err != nil {
		if d.multiLogger != nil {
			d.multiLogger.LogAppError("failed to update channel list in database", zap.Error(err))
		}
		return nil // Don't block downloads on this error
	}

	if d.multiLogger != nil {
		d.multiLogger.LogQueueEvent("telegram_channel_update_complete",
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
		if d.multiLogger != nil {
			d.multiLogger.LogAppError("failed to get channel name",
				zap.Error(err), zap.String("channel_id", channelID))
		}
		return channelID
	}

	if name == "" {
		return channelID
	}

	return name
}
