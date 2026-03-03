package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestMetadata() *MediaMetadata {
	return &MediaMetadata{
		ID:           "12345",
		Title:        "Test Video Title",
		Description:  "A test video description",
		Uploader:     "TestUser",
		UploaderID:   "testuser",
		UploaderURL:  "https://x.com/testuser",
		WebpageURL:   "https://x.com/testuser/status/12345",
		URL:          "https://x.com/testuser/status/12345",
		Timestamp:    1700000000,
		UploadDate:   "20231114",
		Tags:         []string{"tag1", "tag2"},
		Platform:     "x",
		Extractor:    "twitter",
		ExtractorKey: "Twitter",
		Extension:    "mp4",
		Files:        []string{"/tmp/completed/file1.mp4", "/tmp/completed/file2.jpg"},
	}
}

func TestMediaMetadata_ToMap(t *testing.T) {
	meta := newTestMetadata()
	m := meta.ToMap()

	// Core identification
	assert.Equal(t, "12345", m["id"])
	assert.Equal(t, "Test Video Title", m["title"])
	assert.Equal(t, "A test video description", m["description"])

	// Uploader fields
	assert.Equal(t, "TestUser", m["uploader"])
	assert.Equal(t, "testuser", m["uploader_id"])
	assert.Equal(t, "https://x.com/testuser", m["uploader_url"])

	// URL fields
	assert.Equal(t, "https://x.com/testuser/status/12345", m["webpage_url"])
	assert.Equal(t, "https://x.com/testuser/status/12345", m["url"])

	// Timestamps
	assert.Equal(t, int64(1700000000), m["timestamp"])
	assert.Equal(t, "20231114", m["upload_date"])

	// Tags
	assert.Equal(t, []string{"tag1", "tag2"}, m["tags"])

	// Extractor info
	assert.Equal(t, "twitter", m["extractor"])
	assert.Equal(t, "Twitter", m["extractor_key"])
	assert.Equal(t, "x", m["platform"])

	// Optional fields present
	assert.Equal(t, "mp4", m["ext"])
	files, ok := m["files"].([]string)
	assert.True(t, ok)
	assert.Len(t, files, 2)
}

func TestMediaMetadata_ToMap_OmitsEmptyOptionals(t *testing.T) {
	meta := &MediaMetadata{
		ID:    "minimal",
		Title: "Minimal",
	}
	m := meta.ToMap()

	// Extension and Files should be absent when empty
	_, hasExt := m["ext"]
	assert.False(t, hasExt, "ext should be omitted when Extension is empty")

	_, hasFiles := m["files"]
	assert.False(t, hasFiles, "files should be omitted when Files is empty")
}

func TestMediaMetadata_ToFileMap(t *testing.T) {
	meta := newTestMetadata()
	fm := meta.ToFileMap("/tmp/completed/file1.mp4", "mp4")

	// Per-file fields should be added
	assert.Equal(t, "mp4", fm["ext"])
	assert.Equal(t, "/tmp/completed/file1.mp4", fm["local_file"])
	assert.Equal(t, "video", fm["_type"])
	assert.Equal(t, int64(1700000000), fm["epoch"])

	// Aggregate "files" should be removed from per-file map
	_, hasFiles := fm["files"]
	assert.False(t, hasFiles, "files should be removed from per-file metadata")

	// Common fields should still be present
	assert.Equal(t, "12345", fm["id"])
	assert.Equal(t, "Test Video Title", fm["title"])
	assert.Equal(t, "https://x.com/testuser/status/12345", fm["webpage_url"])
}

func TestMediaMetadata_ToFileMap_OverridesExt(t *testing.T) {
	meta := &MediaMetadata{
		ID:        "test",
		Extension: "mkv", // Original extension
	}
	fm := meta.ToFileMap("/tmp/file.jpg", "jpg")

	// Per-file ext should override the struct-level Extension
	assert.Equal(t, "jpg", fm["ext"])
}

func TestMediaMetadata_ToEagleItem(t *testing.T) {
	meta := newTestMetadata()
	eagle := meta.ToEagleItem("/tmp/completed/file1.mp4")

	// Required fields
	assert.Equal(t, "/tmp/completed/file1.mp4", eagle.Path)
	assert.Equal(t, "Test Video Title", eagle.Name)

	// Optional fields
	assert.Equal(t, "https://x.com/testuser/status/12345", eagle.Website)
	assert.Equal(t, []string{"tag1", "tag2"}, eagle.Tags)
	assert.Equal(t, "A test video description", eagle.Annotation)

	// FolderID should be empty (not set by ToEagleItem)
	assert.Empty(t, eagle.FolderID)
}

func TestMediaMetadata_ToEagleItem_MinimalFields(t *testing.T) {
	meta := &MediaMetadata{
		ID:    "minimal",
		Title: "Minimal Title",
	}
	eagle := meta.ToEagleItem("/tmp/file.mp4")

	assert.Equal(t, "/tmp/file.mp4", eagle.Path)
	assert.Equal(t, "Minimal Title", eagle.Name)
	assert.Empty(t, eagle.Website)
	assert.Nil(t, eagle.Tags)
	assert.Empty(t, eagle.Annotation)
}

func TestMediaMetadata_ToMap_NilTags(t *testing.T) {
	meta := &MediaMetadata{
		ID:   "test",
		Tags: nil,
	}
	m := meta.ToMap()

	// Tags should be nil (not an empty slice) when not set
	assert.Nil(t, m["tags"])
}

func TestMediaMetadata_ToMap_EmptyTags(t *testing.T) {
	meta := &MediaMetadata{
		ID:   "test",
		Tags: []string{},
	}
	m := meta.ToMap()

	// Empty tags slice should be preserved
	tags, ok := m["tags"].([]string)
	assert.True(t, ok)
	assert.Empty(t, tags)
}

