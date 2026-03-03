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

// GalleryDownloader implements Downloader for gallery-dl (catch-all for 100+ sites)
type GalleryDownloader struct {
	DownloadLogger // Embedded shared log file operations
	config         *domain.GalleryDLConfig
	incomingDir    string
	completedDir   string
	eventLogger    *logger.MultiLogger
}

// NewGalleryDownloader creates a new gallery-dl downloader
func NewGalleryDownloader(config *domain.GalleryDLConfig, incomingDir, completedDir, logsDir string, eventLogger *logger.MultiLogger) *GalleryDownloader {
	return &GalleryDownloader{
		DownloadLogger: DownloadLogger{LogsDir: logsDir},
		config:         config,
		incomingDir:    incomingDir,
		completedDir:   completedDir,
		eventLogger:    eventLogger,
	}
}

// Platform returns the platform this downloader handles
func (d *GalleryDownloader) Platform() domain.Platform {
	return domain.PlatformGallery
}

// Validate validates if the downloader can handle the given URL
func (d *GalleryDownloader) Validate(url string) error {
	// Accept any HTTP/HTTPS URL - gallery-dl will decide if it can handle it
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("invalid URL: %s (must be http:// or https://)", url)
	}
	return nil
}

// Download downloads media using gallery-dl
func (d *GalleryDownloader) Download(download *domain.Download, progressCallback domain.DownloadProgressCallback) error {
	if err := d.Validate(download.URL); err != nil {
		return err
	}

	// Create a per-download temp directory inside incoming to isolate files
	downloadDir := filepath.Join(d.incomingDir, "gallery-dl-"+download.ID)
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return fmt.Errorf("failed to create download directory: %w", err)
	}
	defer os.RemoveAll(downloadDir) // Clean up temp dir after move

	// Build gallery-dl command
	args := []string{
		"--restrict-filenames",
		"-D", downloadDir,
	}

	// Write metadata if configured
	if d.config.WriteMetadata {
		args = append(args, "--write-metadata")
	}

	// Add cookie file if configured
	if d.config.CookieFile != "" && FileExists(d.config.CookieFile) {
		args = append(args, "--cookies", d.config.CookieFile)
	}

	// Add extra params if configured
	if d.config.ExtraParams != "" {
		for _, param := range strings.Fields(d.config.ExtraParams) {
			args = append(args, param)
		}
	}

	args = append(args, download.URL)

	// Create default callback if nil
	if progressCallback == nil {
		progressCallback = func(output string, percent float64) {}
	}

	// Open log file for direct redirect
	downloadLog, err := d.OpenLogFile()
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer downloadLog.Close()

	// Write command header to download log
	cmdLine := ShellEscapeCommand(d.config.GalleryDLBinary, args...)
	d.WriteLogHeader(downloadLog, download.ID, cmdLine)

	// Execute gallery-dl
	cmd := exec.Command(d.config.GalleryDLBinary, args...)
	cmd.Stdout = downloadLog
	cmd.Stderr = downloadLog

	err = cmd.Run()
	if err != nil {
		d.WriteLogFooter(downloadLog, false, fmt.Sprintf("gallery-dl failed: %v", err))
		progressCallback("", -1)
		return fmt.Errorf("gallery-dl failed: %w", err)
	}

	// Find downloaded files in the download directory
	files, err := d.findDownloadedFiles(downloadDir)
	if err != nil {
		d.WriteLogFooter(downloadLog, false, fmt.Sprintf("Failed to find files: %v", err))
		return err
	}

	if len(files) == 0 {
		d.WriteLogFooter(downloadLog, false, "No files downloaded")
		return fmt.Errorf("no files downloaded")
	}

	// Move files from download dir to completed directory
	completedFiles, err := d.moveToCompleted(files, downloadDir)
	if err != nil {
		d.WriteLogFooter(downloadLog, false, fmt.Sprintf("Failed to move files: %v", err))
		return fmt.Errorf("failed to move files to completed: %w", err)
	}

	// Store metadata
	if d.config.WriteMetadata {
		if err := d.storeMetadata(download, completedFiles, downloadDir); err != nil {
			if d.eventLogger != nil {
				d.eventLogger.LogAppError("Failed to store gallery-dl metadata", zap.Error(err))
			}
		}
	}

	// Update download with file path (use first file if multiple)
	download.FilePath = completedFiles[0]

	d.WriteLogFooter(downloadLog, true, fmt.Sprintf("Downloaded: %s", download.FilePath))
	progressCallback("", 100)

	return nil
}

// findDownloadedFiles finds all media files in the download directory (recursive)
func (d *GalleryDownloader) findDownloadedFiles(downloadDir string) ([]string, error) {
	var files []string

	err := filepath.Walk(downloadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && IsMediaFile(path) {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// moveToCompleted moves media files from download dir to completed directory.
// Also moves corresponding .json metadata files created by gallery-dl.
func (d *GalleryDownloader) moveToCompleted(files []string, downloadDir string) ([]string, error) {
	var completedFiles []string

	if err := os.MkdirAll(d.completedDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create completed directory: %w", err)
	}

	for _, file := range files {
		filename := filepath.Base(file)
		destPath := filepath.Join(d.completedDir, filename)

		if err := MoveFile(file, destPath); err != nil {
			return nil, err
		}

		completedFiles = append(completedFiles, destPath)

		// Also move corresponding gallery-dl metadata .json file if it exists
		metaPath := file + ".json"
		if FileExists(metaPath) {
			metaDest := filepath.Join(d.completedDir, filepath.Base(metaPath))
			_ = MoveFile(metaPath, metaDest) // Best effort
		}
	}

	return completedFiles, nil
}

// storeMetadata reads gallery-dl's metadata .json files and stores unified metadata
func (d *GalleryDownloader) storeMetadata(download *domain.Download, completedFiles []string, downloadDir string) error {
	var meta *domain.MediaMetadata

	// Try to read gallery-dl's metadata .json file
	for _, file := range completedFiles {
		metaPath := file + ".json"
		if data, err := os.ReadFile(metaPath); err == nil {
			var infoData map[string]interface{}
			if json.Unmarshal(data, &infoData) == nil {
				meta = d.buildRichMetadata(infoData, download.URL, completedFiles)
				break
			}
		}
	}

	// If no metadata file found, build minimal metadata
	if meta == nil {
		meta = d.buildMinimalMetadata(download.URL, completedFiles)
	}

	data, err := json.Marshal(meta.ToMap())
	if err != nil {
		return err
	}

	download.Metadata = string(data)

	// Write per-file .info.json and .eagle.json
	for _, file := range completedFiles {
		WriteInfoJSON(file, meta)
		WriteEagleMetadata(file, meta)
	}

	return nil
}

// buildRichMetadata extracts metadata from gallery-dl's .json metadata
func (d *GalleryDownloader) buildRichMetadata(infoData map[string]interface{}, url string, files []string) *domain.MediaMetadata {
	// gallery-dl metadata has different field names than yt-dlp
	// Common fields: category, subcategory, filename, extension, date
	category := GetStringFromMap(infoData, "category")
	subcategory := GetStringFromMap(infoData, "subcategory")

	title := GetStringFromMap(infoData, "title")
	if title == "" {
		title = GetStringFromMap(infoData, "filename")
	}
	if title == "" {
		title = fmt.Sprintf("%s_%s", category, subcategory)
	}

	description := GetStringFromMap(infoData, "description")
	if description == "" {
		description = GetStringFromMap(infoData, "content")
	}

	uploader := GetStringFromMap(infoData, "author")
	if uploader == "" {
		uploader = GetStringFromMap(infoData, "user")
	}
	if uploader == "" {
		uploader = GetStringFromMap(infoData, "username")
	}

	uploaderID := GetStringFromMap(infoData, "author_id")
	if uploaderID == "" {
		uploaderID = GetStringFromMap(infoData, "user_id")
	}

	webpageURL := GetStringFromMap(infoData, "url")
	if webpageURL == "" {
		webpageURL = url
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
	if category != "" {
		tags = append(tags, category)
	}
	tags = append(tags, "gallery-dl")

	// Handle timestamp
	timestamp := time.Now().Unix()
	uploadDate := time.Now().Format("20060102")
	if dateStr := GetStringFromMap(infoData, "date"); dateStr != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", dateStr); err == nil {
			timestamp = t.Unix()
			uploadDate = t.Format("20060102")
		} else if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
			timestamp = t.Unix()
			uploadDate = t.Format("20060102")
		}
	}

	return &domain.MediaMetadata{
		ID:           GetStringFromMap(infoData, "id"),
		Title:        title,
		Description:  description,
		Uploader:     uploader,
		UploaderID:   uploaderID,
		UploaderURL:  "",
		WebpageURL:   webpageURL,
		URL:          url,
		Timestamp:    timestamp,
		UploadDate:   uploadDate,
		Tags:         tags,
		Platform:     "gallery",
		Extractor:    category,
		ExtractorKey: category,
		Extension:    GetStringFromMap(infoData, "extension"),
		Files:        files,
	}
}

// buildMinimalMetadata creates basic metadata when gallery-dl metadata is not available
func (d *GalleryDownloader) buildMinimalMetadata(url string, files []string) *domain.MediaMetadata {
	// Try to extract something useful from the URL
	urlWithoutProtocol := strings.TrimPrefix(url, "https://")
	urlWithoutProtocol = strings.TrimPrefix(urlWithoutProtocol, "http://")

	parts := strings.SplitN(urlWithoutProtocol, "/", 3)
	site := ""
	if len(parts) > 0 {
		site = parts[0]
	}

	return &domain.MediaMetadata{
		ID:           "",
		Title:        filepath.Base(urlWithoutProtocol),
		Uploader:     "",
		UploaderID:   "",
		URL:          url,
		Timestamp:    time.Now().Unix(),
		UploadDate:   time.Now().Format("20060102"),
		Tags:         []string{site, "gallery-dl"},
		Platform:     "gallery",
		Extractor:    site,
		ExtractorKey: site,
		Files:        files,
	}
}
