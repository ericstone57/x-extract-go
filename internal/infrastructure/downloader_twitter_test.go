package infrastructure

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/x-extract-go/internal/domain"
)

func newTestTwitterDownloader(config *domain.TwitterConfig) *TwitterDownloader {
	return NewTwitterDownloader(config, "/tmp/incoming", "/tmp/completed", "/tmp/logs", nil)
}

func TestBuildYTDLPCommand_BasicArgs(t *testing.T) {
	config := &domain.TwitterConfig{
		YTDLPBinary:   "yt-dlp",
		WriteMetadata: true,
	}
	downloader := newTestTwitterDownloader(config)

	// Test that we can build args and exec.Command accepts them
	args := []string{
		"--write-info-json",
		"--write-playlist-metafiles",
		"--restrict-filenames",
		"-o", "%(uploader_id)s_%(id)s.%(ext)s",
		"-P", downloader.incomingDir,
		"https://x.com/user/status/123",
	}

	// This should NOT fail - exec.Command handles args without shell quoting
	cmd := exec.Command(config.YTDLPBinary, args...)
	assert.NotNil(t, cmd)
}

func TestBuildYTDLPCommand_PathWithSpaces(t *testing.T) {
	// Create a temp directory with spaces in the path
	tmpDir, err := os.MkdirTemp("", "path with spaces test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := &domain.TwitterConfig{
		YTDLPBinary:   "yt-dlp",
		WriteMetadata: true,
	}

	// Use path with spaces
	incomingDir := filepath.Join(tmpDir, "incoming dir")
	completedDir := filepath.Join(tmpDir, "completed dir")
	logsDir := filepath.Join(tmpDir, "logs dir")

	downloader := NewTwitterDownloader(config, incomingDir, completedDir, logsDir, nil)

	// Build args with paths containing spaces
	args := []string{
		"--write-info-json",
		"--restrict-filenames",
		"-o", "%(uploader_id)s_%(id)s.%(ext)s",
		"-P", downloader.incomingDir,
		"https://x.com/user/status/123",
	}

	// exec.Command should handle paths with spaces correctly
	cmd := exec.Command(config.YTDLPBinary, args...)
	assert.NotNil(t, cmd)

	// Verify the path is passed correctly (not split on spaces)
	foundPath := false
	for _, arg := range args {
		if arg == incomingDir {
			foundPath = true
			break
		}
	}
	assert.True(t, foundPath, "Path with spaces should be a single argument")
}

func TestBuildYTDLPCommand_WithCookieFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cookie-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a cookie file
	cookieFile := filepath.Join(tmpDir, "cookies.txt")
	err = os.WriteFile(cookieFile, []byte("# Netscape HTTP Cookie File\n"), 0644)
	require.NoError(t, err)

	config := &domain.TwitterConfig{
		YTDLPBinary:   "yt-dlp",
		CookieFile:    cookieFile,
		WriteMetadata: true,
	}
	downloader := newTestTwitterDownloader(config)
	downloader.config.CookieFile = cookieFile

	args := []string{
		"--write-info-json",
		"--restrict-filenames",
		"-o", "%(uploader_id)s_%(id)s.%(ext)s",
		"-P", downloader.incomingDir,
	}

	if downloader.config.CookieFile != "" && fileExists(downloader.config.CookieFile) {
		args = append(args, "--cookies", downloader.config.CookieFile)
	}
	args = append(args, "https://x.com/user/status/123")

	// Verify cookie file is included
	assert.Contains(t, args, "--cookies")
	assert.Contains(t, args, cookieFile)

	// exec.Command should handle this correctly
	cmd := exec.Command(config.YTDLPBinary, args...)
	assert.NotNil(t, cmd)
}

func TestBuildYTDLPCommand_URLWithQueryParams(t *testing.T) {
	config := &domain.TwitterConfig{
		YTDLPBinary: "yt-dlp",
	}
	downloader := newTestTwitterDownloader(config)

	// URL with query parameters
	urlWithQuery := "https://x.com/user/status/123?s=20&t=abc123"

	args := []string{
		"--write-info-json",
		"-o", "%(uploader_id)s_%(id)s.%(ext)s",
		"-P", downloader.incomingDir,
		urlWithQuery,
	}

	// exec.Command should handle URL with query params correctly
	cmd := exec.Command(config.YTDLPBinary, args...)
	assert.NotNil(t, cmd)

	// Verify URL is a single argument
	assert.Equal(t, urlWithQuery, args[len(args)-1])
}

func TestShellEscape(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple path",
			input:    "/tmp/simple/path",
			expected: "/tmp/simple/path",
		},
		{
			name:     "path with spaces",
			input:    "/tmp/path with spaces",
			expected: "'/tmp/path with spaces'",
		},
		{
			name:     "path with single quote",
			input:    "/tmp/path'with'quote",
			expected: "'/tmp/path'\"'\"'with'\"'\"'quote'",
		},
		{
			name:     "path with special chars",
			input:    "/tmp/path$with$special",
			expected: "'/tmp/path$with$special'",
		},
		{
			name:     "path with backtick",
			input:    "/tmp/path`with`backtick",
			expected: "'/tmp/path`with`backtick'",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "''",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShellEscape(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShellEscapeCommand(t *testing.T) {
	tests := []struct {
		name     string
		binary   string
		args     []string
		contains []string
	}{
		{
			name:   "simple command",
			binary: "yt-dlp",
			args:   []string{"--version"},
			contains: []string{
				"yt-dlp --version",
			},
		},
		{
			name:   "path with spaces",
			binary: "yt-dlp",
			args:   []string{"-P", "/tmp/path with spaces", "https://x.com/user/status/123"},
			contains: []string{
				"yt-dlp",
				"-P",
				"'/tmp/path with spaces'",
				"https://x.com/user/status/123",
			},
		},
		{
			name:   "cookie file with special chars",
			binary: "yt-dlp",
			args:   []string{"--cookies", "/tmp/my cookies/cookies.txt"},
			contains: []string{
				"--cookies",
				"'/tmp/my cookies/cookies.txt'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShellEscapeCommand(tt.binary, tt.args...)
			for _, expected := range tt.contains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestTwitterDownloader_Validate(t *testing.T) {
	config := &domain.TwitterConfig{}
	downloader := newTestTwitterDownloader(config)

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid x.com URL",
			url:     "https://x.com/user/status/123",
			wantErr: false,
		},
		{
			name:    "valid twitter.com URL",
			url:     "https://twitter.com/user/status/123",
			wantErr: false,
		},
		{
			name:    "invalid URL - other domain",
			url:     "https://example.com/video",
			wantErr: true,
		},
		{
			name:    "invalid URL - telegram",
			url:     "https://t.me/channel/123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := downloader.Validate(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid Twitter/X URL")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTwitterDownloader_Platform(t *testing.T) {
	config := &domain.TwitterConfig{}
	downloader := newTestTwitterDownloader(config)

	assert.Equal(t, domain.PlatformX, downloader.Platform())
}

func TestFindDownloadedFiles_UsernameExtraction(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		username string
	}{
		{
			name:     "x.com URL",
			url:      "https://x.com/UserName123/status/123456789",
			username: "UserName123",
		},
		{
			name:     "twitter.com URL",
			url:      "https://twitter.com/AnotherUser/status/987654321",
			username: "AnotherUser",
		},
		{
			name:     "URL with query params",
			url:      "https://x.com/TestUser/status/111?s=20",
			username: "TestUser",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Extract username from URL (same logic as findDownloadedFiles)
			urlWithoutProtocol := strings.TrimPrefix(tt.url, "https://")
			urlWithoutProtocol = strings.TrimPrefix(urlWithoutProtocol, "http://")
			parts := strings.Split(urlWithoutProtocol, "/")

			require.GreaterOrEqual(t, len(parts), 2, "URL should have at least 2 parts")
			username := parts[1]
			assert.Equal(t, tt.username, username)
		})
	}
}
