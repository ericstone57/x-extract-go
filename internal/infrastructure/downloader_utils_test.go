package infrastructure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/x-extract-go/internal/domain"
)

func newTestMediaMetadata() *domain.MediaMetadata {
	return &domain.MediaMetadata{
		ID:           "67890",
		Title:        "Test Download",
		Description:  "Test description for utils",
		Uploader:     "Uploader",
		UploaderID:   "uploader_id",
		UploaderURL:  "https://x.com/uploader_id",
		WebpageURL:   "https://x.com/uploader_id/status/67890",
		URL:          "https://x.com/uploader_id/status/67890",
		Timestamp:    1700000000,
		UploadDate:   "20231114",
		Tags:         []string{"media", "test"},
		Platform:     "x",
		Extractor:    "twitter",
		ExtractorKey: "Twitter",
		Files:        []string{"/tmp/file1.mp4"},
	}
}

func TestWriteInfoJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-write-info-json-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test_video.mp4")
	require.NoError(t, os.WriteFile(filePath, []byte("fake video"), 0644))

	meta := newTestMediaMetadata()
	err = WriteInfoJSON(filePath, meta)
	require.NoError(t, err)

	// Verify .info.json was created
	infoPath := filepath.Join(tmpDir, "test_video.info.json")
	assert.True(t, FileExists(infoPath), "info.json should be created")

	// Read and parse the JSON
	data, err := os.ReadFile(infoPath)
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))

	// Per-file fields should be present
	assert.Equal(t, "mp4", result["ext"])
	assert.Equal(t, filePath, result["local_file"])
	assert.Equal(t, "video", result["_type"])
	assert.Equal(t, float64(1700000000), result["epoch"]) // JSON numbers are float64

	// Common fields
	assert.Equal(t, "67890", result["id"])
	assert.Equal(t, "Test Download", result["title"])
	assert.Equal(t, "https://x.com/uploader_id/status/67890", result["webpage_url"])

	// "files" should not be present in per-file metadata
	_, hasFiles := result["files"]
	assert.False(t, hasFiles, "files should not be in per-file info.json")
}

func TestWriteInfoJSON_DifferentExtensions(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		wantExt string
	}{
		{"mp4", "video.mp4", "mp4"},
		{"jpg", "photo.jpg", "jpg"},
		{"webm", "clip.webm", "webm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "test-ext-*")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			filePath := filepath.Join(tmpDir, tt.file)
			require.NoError(t, os.WriteFile(filePath, []byte("data"), 0644))

			err = WriteInfoJSON(filePath, newTestMediaMetadata())
			require.NoError(t, err)

			base := filePath[:len(filePath)-len(filepath.Ext(filePath))]
			infoPath := base + ".info.json"
			data, err := os.ReadFile(infoPath)
			require.NoError(t, err)

			var result map[string]interface{}
			require.NoError(t, json.Unmarshal(data, &result))
			assert.Equal(t, tt.wantExt, result["ext"])
		})
	}
}

func TestWriteEagleMetadata(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-write-eagle-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test_video.mp4")
	require.NoError(t, os.WriteFile(filePath, []byte("fake video"), 0644))

	meta := newTestMediaMetadata()
	err = WriteEagleMetadata(filePath, meta)
	require.NoError(t, err)

	// Verify .eagle.json was created
	eaglePath := filepath.Join(tmpDir, "test_video.eagle.json")
	assert.True(t, FileExists(eaglePath), "eagle.json should be created")

	// Read and parse the JSON
	data, err := os.ReadFile(eaglePath)
	require.NoError(t, err)

	var eagle domain.EagleItem
	require.NoError(t, json.Unmarshal(data, &eagle))

	// Required fields
	assert.Equal(t, filePath, eagle.Path)
	assert.Equal(t, "Test Download", eagle.Name)

	// Optional fields
	assert.Equal(t, "https://x.com/uploader_id/status/67890", eagle.Website)
	assert.Equal(t, []string{"media", "test"}, eagle.Tags)
	assert.Equal(t, "Test description for utils", eagle.Annotation)
	assert.Empty(t, eagle.FolderID)
}

func TestWriteInfoJSON_InvalidPath(t *testing.T) {
	meta := newTestMediaMetadata()
	err := WriteInfoJSON("/nonexistent/dir/file.mp4", meta)
	assert.Error(t, err, "should fail for non-existent directory")
}

func TestWriteEagleMetadata_InvalidPath(t *testing.T) {
	meta := newTestMediaMetadata()
	err := WriteEagleMetadata("/nonexistent/dir/file.mp4", meta)
	assert.Error(t, err, "should fail for non-existent directory")
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean name unchanged", "My Video Title", "My Video Title"},
		{"empty string", "", "unnamed_item"},
		{"whitespace only", "   ", "unnamed_item"},
		{"illegal chars replaced with dashes", `a<b>c:d"e/f\g|h?i*j`, "a-b-c-d-e-f-g-h-i-j"},
		{"trailing dots trimmed", "name...", "name"},
		{"trailing spaces trimmed", "name   ", "name"},
		{"trailing dot and space mixed", "name. . ", "name"},
		{"leading spaces trimmed", "   name", "name"},
		{"reserved name CON", "CON", "_CON"},
		{"reserved name con lowercase", "con", "_con"},
		{"reserved name NUL", "NUL", "_NUL"},
		{"reserved name COM1", "COM1", "_COM1"},
		{"reserved name LPT9", "LPT9", "_LPT9"},
		{"reserved name with extension", "CON.txt", "_CON.txt"},
		{"non-reserved name not prefixed", "CONSOLE", "CONSOLE"},
		{"name within 180 bytes unchanged", "short name", "short name"},
		{"name exactly 180 bytes unchanged", strings.Repeat("a", 180), strings.Repeat("a", 180)},
		{"name over 180 bytes truncated with ellipsis", strings.Repeat("a", 200), strings.Repeat("a", 177) + "…"},
		{"name over 180 bytes with ext preserves ext", strings.Repeat("a", 180) + ".mp4", strings.Repeat("a", 173) + "….mp4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeFilename(tt.input)
			assert.Equal(t, tt.want, got)
			// Result should never exceed 180 bytes
			assert.LessOrEqual(t, len(got), 180, "sanitized name should be <= 180 bytes")
		})
	}
}

func TestIsMediaFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"video.mp4", true},
		{"photo.jpg", true},
		{"photo.JPEG", true},
		{"clip.webm", true},
		{"movie.mkv", true},
		{"animation.gif", true},
		{"image.png", true},
		{"image.webp", true},
		{"video.m4v", true},
		{"video.mov", true},
		{"video.avi", true},
		{"metadata.info.json", false},
		{"data.json", false},
		{"readme.txt", false},
		{"script.sh", false},
		{"metadata.eagle.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, IsMediaFile(tt.path))
		})
	}
}
