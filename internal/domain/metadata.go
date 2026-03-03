package domain

// MediaMetadata is the unified metadata structure shared by all downloaders.
// It captures common fields that map to Eagle App's API and yt-dlp's .info.json format.
//
// Eagle API mapping:
//   - Required: path → FilePath, name → Title
//   - Optional: website → WebpageURL, tags → Tags, annotation → Description
//
// To add metadata for a new downloader, populate this struct from platform-specific data
// and call ToMap() for .info.json or ToEagleItem() for Eagle import.
type MediaMetadata struct {
	// Core identification
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`

	// Uploader / author
	Uploader    string `json:"uploader"`
	UploaderID  string `json:"uploader_id"`
	UploaderURL string `json:"uploader_url"`

	// URLs
	WebpageURL string `json:"webpage_url"`
	URL        string `json:"url"` // Original download URL

	// Timestamps
	Timestamp  int64  `json:"timestamp"`
	UploadDate string `json:"upload_date"` // YYYYMMDD format

	// Classification
	Tags         []string `json:"tags"`
	Platform     string   `json:"platform"`
	Extractor    string   `json:"extractor"`
	ExtractorKey string   `json:"extractor_key"`

	// File info
	Extension string   `json:"ext,omitempty"`
	Files     []string `json:"files,omitempty"`
}

// EagleItem represents the metadata structure for importing into Eagle App.
// See: https://api.eagle.cool/item/add-from-path
//
// Required fields: Path, Name
// Optional fields: Website, Tags, Annotation, FolderID
type EagleItem struct {
	Path       string   `json:"path"`
	Name       string   `json:"name"`
	Website    string   `json:"website,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Annotation string   `json:"annotation,omitempty"`
	FolderID   string   `json:"folderId,omitempty"`
}

// ToMap converts MediaMetadata to a map[string]interface{} for JSON serialization.
// The output is compatible with yt-dlp's .info.json format.
func (m *MediaMetadata) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		// Core identification (yt-dlp compatible)
		"id":          m.ID,
		"title":       m.Title,
		"description": m.Description,

		// Uploader fields (yt-dlp compatible)
		"uploader":     m.Uploader,
		"uploader_id":  m.UploaderID,
		"uploader_url": m.UploaderURL,

		// URL fields (yt-dlp compatible)
		"webpage_url": m.WebpageURL,

		// Timestamp fields (yt-dlp compatible)
		"timestamp":   m.Timestamp,
		"upload_date": m.UploadDate,

		// Tags (yt-dlp compatible)
		"tags": m.Tags,

		// Extractor info (yt-dlp compatible)
		"extractor":     m.Extractor,
		"extractor_key": m.ExtractorKey,

		// Additional fields
		"url":      m.URL,
		"platform": m.Platform,
	}

	if m.Extension != "" {
		result["ext"] = m.Extension
	}
	if len(m.Files) > 0 {
		result["files"] = m.Files
	}

	return result
}

// ToFileMap returns a map for per-file .info.json metadata.
// It includes file-specific fields (ext, local_file, _type, epoch) alongside the common fields.
func (m *MediaMetadata) ToFileMap(filePath, ext string) map[string]interface{} {
	result := m.ToMap()
	// Remove aggregate "files" from per-file metadata
	delete(result, "files")
	// Add per-file fields
	result["ext"] = ext
	result["local_file"] = filePath
	result["_type"] = "video"
	result["epoch"] = m.Timestamp
	return result
}

// ToEagleItem converts MediaMetadata to an EagleItem for Eagle App import.
// filePath is the local path to the media file.
func (m *MediaMetadata) ToEagleItem(filePath string) *EagleItem {
	return &EagleItem{
		Path:       filePath,
		Name:       m.Title,
		Website:    m.WebpageURL,
		Tags:       m.Tags,
		Annotation: m.Description,
	}
}

