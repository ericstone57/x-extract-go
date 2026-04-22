package infrastructure

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yourusername/x-extract-go/internal/domain"
)

// DownloadLogFileFormat is the format string for download log filenames.
// The %s placeholder is replaced with the date in YYYYMMDD format.
const DownloadLogFileFormat = "download-%s.log"

// ImportLogFileFormat is the format string for Eagle import log filenames.
// The %s placeholder is replaced with the date in YYYYMMDD format.
const ImportLogFileFormat = "import-%s.log"

// MediaExtensions is the canonical list of supported media file extensions.
// Used by IsMediaFile and file-finding functions across all downloaders.
var MediaExtensions = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".webm": true,
	".m4v":  true,
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
}

// IsMediaFile checks if a file is a media file based on its extension.
// Excludes metadata files like .info.json.
func IsMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return MediaExtensions[ext]
}

// FileExists checks if a file or directory exists at the given path.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// CopyFile copies a file from src to dst.
func CopyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// MoveFile moves a file from src to dst.
// Tries os.Rename first; if that fails (e.g., cross-device), falls back to copy+delete.
func MoveFile(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		// Rename failed (possibly cross-device), try copy and delete
		if err := CopyFile(src, dst); err != nil {
			return fmt.Errorf("failed to move file %s to %s: %w", src, dst, err)
		}
		os.Remove(src)
	}
	return nil
}

// GetStringFromMap safely extracts a string value from a map[string]interface{}.
// Returns empty string if the key doesn't exist or the value is not a string.
func GetStringFromMap(data map[string]interface{}, key string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return ""
}

// GetFirstStringFromMap returns the first non-empty string value found in data
// for the given keys, in order. Returns "" if none match.
func GetFirstStringFromMap(data map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v := GetStringFromMap(data, k); v != "" {
			return v
		}
	}
	return ""
}


// DownloadLogger provides common download log file operations.
// Embed this in downloader structs to share log file management.
type DownloadLogger struct {
	LogsDir string
}

// ImportLogger writes human-readable Eagle import logs to the logs directory.
type ImportLogger struct {
	LogsDir string
	RunID   string

	file *os.File
}

func openDailyLogFile(logsDir, fileFormat string) (*os.File, error) {
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	dateStr := time.Now().Format("20060102")
	logPath := filepath.Join(logsDir, fmt.Sprintf(fileFormat, dateStr))
	return os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}

// OpenLogFile opens the shared daily download log file.
func (dl *DownloadLogger) OpenLogFile() (*os.File, error) {
	return openDailyLogFile(dl.LogsDir, DownloadLogFileFormat)
}

// OpenDownloadLogFile opens a per-download log file named dl-{id}.log.
// Each download gets its own file so parallel downloads never interleave.
func (dl *DownloadLogger) OpenDownloadLogFile(downloadID string) (*os.File, error) {
	if err := os.MkdirAll(dl.LogsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}
	logPath := filepath.Join(dl.LogsDir, "dl-"+downloadID+".log")
	return os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
}

// WriteLogHeader writes the download start marker to the log file.
func (dl *DownloadLogger) WriteLogHeader(file *os.File, downloadID, cmdLine string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(file, "\n=== [%s] Download: %s ===\n", timestamp, downloadID)
	fmt.Fprintf(file, "$ %s\n", cmdLine)
}

// WriteLogFooter writes the download end marker to the log file.
func (dl *DownloadLogger) WriteLogFooter(file *os.File, success bool, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	status := "SUCCESS"
	if !success {
		status = "FAILED"
	}
	fmt.Fprintf(file, "[%s] %s: %s\n", timestamp, status, message)
	fmt.Fprintf(file, "=== END ===\n\n")
}

// NewImportLogger creates a new Eagle import logger and writes the run header.
func NewImportLogger(logsDir, runID, completedDir string, dryRun bool) (*ImportLogger, error) {
	file, err := openDailyLogFile(logsDir, ImportLogFileFormat)
	if err != nil {
		return nil, err
	}

	logger := &ImportLogger{
		LogsDir: logsDir,
		RunID:   runID,
		file:    file,
	}
	logger.WriteRunHeader(completedDir, dryRun)

	return logger, nil
}

// LogPath returns today's import log path.
func (il *ImportLogger) LogPath() string {
	dateStr := time.Now().Format("20060102")
	return filepath.Join(il.LogsDir, fmt.Sprintf(ImportLogFileFormat, dateStr))
}

// WriteRunHeader writes the start marker for one Eagle import invocation.
func (il *ImportLogger) WriteRunHeader(completedDir string, dryRun bool) {
	if il == nil || il.file == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(il.file, "\n=== [%s] Eagle import run: %s ===\n", timestamp, il.RunID)
	fmt.Fprintf(il.file, "completed_dir=%s dry_run=%t\n", completedDir, dryRun)
}

// Logf appends a timestamped log line for the current Eagle import run.
func (il *ImportLogger) Logf(format string, args ...interface{}) {
	if il == nil || il.file == nil {
		return
	}

	message := fmt.Sprintf(format, args...)
	message = strings.TrimRight(message, "\n")
	if message == "" {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(il.file, "[%s] [%s] %s\n", timestamp, il.RunID, message)
}

// Close writes the run footer and closes the underlying log file.
func (il *ImportLogger) Close(imported, failed int) error {
	if il == nil || il.file == nil {
		return nil
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(il.file, "=== [%s] END %s imported=%d failed=%d ===\n\n", timestamp, il.RunID, imported, failed)
	err := il.file.Close()
	il.file = nil
	return err
}

// WriteInfoJSON writes a MediaMetadata as a yt-dlp compatible .info.json file next to the media file.
// metadataPath is derived from filePath by replacing the extension with .info.json.
func WriteInfoJSON(filePath string, meta *domain.MediaMetadata) error {
	ext := strings.TrimPrefix(filepath.Ext(filePath), ".")
	m := meta.ToFileMap(filePath, ext)

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal info.json: %w", err)
	}

	metadataPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".info.json"
	return os.WriteFile(metadataPath, data, 0644)
}

// illegalFilenameChars contains characters that are problematic for filesystems.
var illegalFilenameChars = []rune{'<', '>', ':', '"', '/', '\\', '|', '?', '*'}

// SanitizeFilename sanitizes a filename for filesystem compatibility using the same
// rules as the eagle-rename command. It:
//   - Replaces illegal characters (< > : " / \ | ? *) with dashes
//   - Trims leading spaces and trailing dots/spaces
//   - Truncates names exceeding 180 bytes, appending "…" and preserving the extension
//   - Prepends "_" to reserved Windows names (CON, PRN, AUX, NUL, COM1-9, LPT1-9)
//
// The sanitized name is returned unchanged when no modifications are needed.
func SanitizeFilename(name string) string {
	if strings.TrimSpace(name) == "" {
		return "unnamed_item"
	}

	proposed := name

	// Replace illegal characters with dashes.
	for _, c := range illegalFilenameChars {
		proposed = strings.ReplaceAll(proposed, string(c), "-")
	}

	// Trim trailing dots and spaces.
	proposed = strings.TrimRight(proposed, ". ")

	// Trim leading spaces.
	proposed = strings.TrimLeft(proposed, " ")

	// Truncate if exceeding 180 bytes, preserving the file extension.
	const maxLen = 180
	if len(proposed) > maxLen {
		ellipsis := "…" // U+2026, 3 bytes in UTF-8
		ellipsisLen := len(ellipsis)
		ext := filepath.Ext(proposed)
		if ext != "" {
			maxBaseLen := maxLen - len(ext) - ellipsisLen
			if maxBaseLen < 1 {
				maxBaseLen = 1
			}
			baseName := strings.TrimSuffix(proposed, ext)
			if len(baseName) > maxBaseLen {
				truncated := baseName[:maxBaseLen]
				for len(truncated) > 0 && !utf8.ValidString(truncated) {
					truncated = truncated[:len(truncated)-1]
				}
				proposed = truncated + ellipsis + ext
			} else {
				proposed = baseName + ellipsis + ext
			}
		} else {
			truncated := proposed[:maxLen-ellipsisLen]
			for len(truncated) > 0 && !utf8.ValidString(truncated) {
				truncated = truncated[:len(truncated)-1]
			}
			proposed = truncated + ellipsis
		}
	}

	// Prepend "_" to reserved Windows device names.
	reserved := []string{
		"CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
	}
	base := strings.Split(proposed, ".")[0]
	for _, r := range reserved {
		if strings.EqualFold(base, r) {
			proposed = "_" + proposed
			break
		}
	}

	// Final safety: if sanitization emptied the name, use a placeholder.
	if strings.TrimSpace(proposed) == "" {
		return "unnamed_item"
	}

	return proposed
}

// WriteEagleMetadata writes an Eagle-compatible metadata JSON file next to the media file.
// The file is named {basename}.eagle.json and contains fields for Eagle App's /api/item/addFromPath endpoint.
// The Name field is sanitized via SanitizeFilename to ensure filesystem compatibility.
func WriteEagleMetadata(filePath string, meta *domain.MediaMetadata) error {
	eagle := meta.ToEagleItem(filePath)
	eagle.Name = SanitizeFilename(eagle.Name)

	data, err := json.MarshalIndent(eagle, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eagle metadata: %w", err)
	}

	eaglePath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".eagle.json"
	return os.WriteFile(eaglePath, data, 0644)
}
