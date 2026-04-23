package binmanager

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKnownTools_YTDLPAssetName(t *testing.T) {
	spec := KnownTools["yt-dlp"]
	tests := []struct {
		goos, goarch string
		expected     string
	}{
		{"darwin", "arm64", "yt-dlp_macos"},
		{"darwin", "amd64", "yt-dlp_macos"},
		{"linux", "amd64", "yt-dlp_linux"},
		{"linux", "arm64", "yt-dlp_linux_aarch64"},
		{"windows", "amd64", "yt-dlp.exe"},
	}
	for _, tt := range tests {
		t.Run(tt.goos+"_"+tt.goarch, func(t *testing.T) {
			assert.Equal(t, tt.expected, spec.AssetName(tt.goos, tt.goarch))
		})
	}
}

func TestKnownTools_TDLAssetName(t *testing.T) {
	spec := KnownTools["tdl"]
	tests := []struct {
		goos, goarch string
		expected     string
	}{
		{"darwin", "arm64", "tdl_MacOS_arm64.tar.gz"},
		{"darwin", "amd64", "tdl_MacOS_64bit.tar.gz"},
		{"linux", "amd64", "tdl_Linux_64bit.tar.gz"},
		{"linux", "arm64", "tdl_Linux_arm64.tar.gz"},
		{"windows", "amd64", "tdl_Windows_64bit.zip"},
	}
	for _, tt := range tests {
		t.Run(tt.goos+"_"+tt.goarch, func(t *testing.T) {
			assert.Equal(t, tt.expected, spec.AssetName(tt.goos, tt.goarch))
		})
	}
}

func TestKnownTools_GalleryDLAssetName(t *testing.T) {
	spec := KnownTools["gallery-dl"]
	tests := []struct {
		goos, goarch string
		expected     string
	}{
		{"darwin", "arm64", ""}, // macOS: installed via pip3, see installGalleryDLViaPip
		{"darwin", "amd64", ""},
		{"linux", "amd64", "gallery-dl.bin"},
		{"linux", "arm64", "gallery-dl.bin"},
		{"windows", "amd64", "gallery-dl.exe"},
	}
	for _, tt := range tests {
		t.Run(tt.goos+"_"+tt.goarch, func(t *testing.T) {
			assert.Equal(t, tt.expected, spec.AssetName(tt.goos, tt.goarch))
		})
	}
}

func TestKnownTools_GalleryDLNoChecksum(t *testing.T) {
	spec := KnownTools["gallery-dl"]
	assert.Equal(t, "", spec.ChecksumAsset, "gallery-dl should have empty ChecksumAsset (uses PGP signatures)")
	assert.False(t, spec.IsArchive, "gallery-dl should not be an archive")
}

func TestResolveBinary_ConfigPath(t *testing.T) {
	// Create a temp binary file
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "my-tool")
	require.NoError(t, os.WriteFile(binPath, []byte("binary"), 0755))

	resolved, err := ResolveBinary("yt-dlp", binPath, tmpDir, false)
	require.NoError(t, err)
	assert.Equal(t, binPath, resolved)
}

func TestResolveBinary_ConfigPathAbsNotFound(t *testing.T) {
	_, err := ResolveBinary("yt-dlp", "/nonexistent/path/yt-dlp", t.TempDir(), false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found at configured path")
}

func TestResolveBinary_ManagedDir(t *testing.T) {
	// Create managed binary
	tmpDir := t.TempDir()
	managedPath := filepath.Join(tmpDir, "yt-dlp")
	require.NoError(t, os.WriteFile(managedPath, []byte("binary"), 0755))

	// configPath == toolName means "default" (not a custom path)
	resolved, err := ResolveBinary("yt-dlp", "yt-dlp", tmpDir, false)
	// Might find in PATH or managed dir. If yt-dlp is in PATH, it finds that.
	// If not, it should find the managed one.
	if err != nil {
		t.Skipf("yt-dlp not in PATH and managed dir lookup failed: %v", err)
	}
	assert.NotEmpty(t, resolved)
}

func TestResolveBinary_PreferManaged(t *testing.T) {
	// Create managed binary
	tmpDir := t.TempDir()
	managedPath := filepath.Join(tmpDir, "yt-dlp")
	require.NoError(t, os.WriteFile(managedPath, []byte("managed-binary"), 0755))

	// With preferManaged=true, should return managed path even if yt-dlp is in PATH
	resolved, err := ResolveBinary("yt-dlp", "yt-dlp", tmpDir, true)
	require.NoError(t, err)
	assert.Equal(t, managedPath, resolved)
}

func TestResolveBinary_PreferManagedNotInstalled(t *testing.T) {
	// With preferManaged=true but no managed binary, should error (not fall back to PATH)
	_, err := ResolveBinary("yt-dlp", "yt-dlp", t.TempDir(), true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveBinary_NotFound(t *testing.T) {
	_, err := ResolveBinary("yt-dlp", "yt-dlp", "/nonexistent/bin/dir", false)
	// If yt-dlp happens to be in PATH, this won't error — skip in that case
	if err == nil {
		t.Skip("yt-dlp found in PATH, cannot test not-found case")
	}
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveBinary_UnknownTool(t *testing.T) {
	_, err := ResolveBinary("unknown-tool", "unknown-tool", t.TempDir(), false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestResolveVersion_Specific(t *testing.T) {
	tag, err := resolveVersion("yt-dlp/yt-dlp", "2026.02.21")
	require.NoError(t, err)
	assert.Equal(t, "2026.02.21", tag)
}

func TestVerifyChecksum_Valid(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file
	content := []byte("hello world")
	filePath := filepath.Join(tmpDir, "test-file")
	require.NoError(t, os.WriteFile(filePath, content, 0644))

	// Compute expected hash
	h := sha256.Sum256(content)
	hash := hex.EncodeToString(h[:])

	// Create checksums file
	checksumContent := hash + "  test-file\n"
	checksumPath := filepath.Join(tmpDir, "checksums.txt")
	require.NoError(t, os.WriteFile(checksumPath, []byte(checksumContent), 0644))

	err := verifyChecksum(filePath, checksumPath, "test-file")
	assert.NoError(t, err)
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "test-file")
	require.NoError(t, os.WriteFile(filePath, []byte("hello world"), 0644))

	checksumPath := filepath.Join(tmpDir, "checksums.txt")
	require.NoError(t, os.WriteFile(checksumPath, []byte("0000000000000000000000000000000000000000000000000000000000000000  test-file\n"), 0644))

	err := verifyChecksum(filePath, checksumPath, "test-file")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
}

func TestVerifyChecksum_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "test-file")
	require.NoError(t, os.WriteFile(filePath, []byte("hello world"), 0644))

	checksumPath := filepath.Join(tmpDir, "checksums.txt")
	require.NoError(t, os.WriteFile(checksumPath, []byte("abcd1234  other-file\n"), 0644))

	err := verifyChecksum(filePath, checksumPath, "test-file")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no checksum found")
}

func TestExtractTarGz(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a tar.gz with a binary file
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	createTestTarGz(t, archivePath, "mybinary", []byte("binary-content"))

	err := extractTarGz(archivePath, tmpDir, "mybinary")
	require.NoError(t, err)

	// Verify extracted file
	data, err := os.ReadFile(filepath.Join(tmpDir, "mybinary"))
	require.NoError(t, err)
	assert.Equal(t, "binary-content", string(data))
}

func TestExtractTarGz_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	createTestTarGz(t, archivePath, "other-binary", []byte("content"))

	err := extractTarGz(archivePath, tmpDir, "mybinary")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in archive")
}

func TestExtractZip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a zip with a binary file
	archivePath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, archivePath, "mybinary", []byte("zip-binary-content"))

	err := extractZip(archivePath, tmpDir, "mybinary")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "mybinary"))
	require.NoError(t, err)
	assert.Equal(t, "zip-binary-content", string(data))
}

func TestExtractZip_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, archivePath, "other-binary", []byte("content"))

	err := extractZip(archivePath, tmpDir, "mybinary")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in archive")
}

func TestExtractBinary_TarGz(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	createTestTarGz(t, archivePath, "tool", []byte("content"))

	err := extractBinary(archivePath, tmpDir, "tool")
	assert.NoError(t, err)
}

func TestExtractBinary_Zip(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, archivePath, "tool", []byte("content"))

	err := extractBinary(archivePath, tmpDir, "tool")
	assert.NoError(t, err)
}

func TestExtractBinary_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.rar")
	require.NoError(t, os.WriteFile(archivePath, []byte("dummy"), 0644))

	err := extractBinary(archivePath, tmpDir, "tool")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported archive format")
}

func TestCopyFileAtomic(t *testing.T) {
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "src")
	dstPath := filepath.Join(tmpDir, "dst")
	require.NoError(t, os.WriteFile(srcPath, []byte("atomic-content"), 0644))

	err := copyFileAtomic(srcPath, dstPath)
	require.NoError(t, err)

	data, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, "atomic-content", string(data))
}

func TestResolveOrInstall_Found(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "yt-dlp")
	require.NoError(t, os.WriteFile(binPath, []byte("binary"), 0755))

	resolved, err := ResolveOrInstall("yt-dlp", binPath, tmpDir, "latest", false, false)
	require.NoError(t, err)
	assert.Equal(t, binPath, resolved)
}

func TestResolveOrInstall_NotFoundAutoInstallOff(t *testing.T) {
	_, err := ResolveOrInstall("yt-dlp", "/nonexistent/yt-dlp", t.TempDir(), "latest", false, false)
	assert.Error(t, err)
}

// --- Test helpers ---

func createTestTarGz(t *testing.T, archivePath, fileName string, content []byte) {
	t.Helper()
	f, err := os.Create(archivePath)
	require.NoError(t, err)
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: fileName,
		Mode: 0755,
		Size: int64(len(content)),
	}))
	_, err = tw.Write(content)
	require.NoError(t, err)
}

func createTestZip(t *testing.T, archivePath, fileName string, content []byte) {
	t.Helper()
	f, err := os.Create(archivePath)
	require.NoError(t, err)
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	w, err := zw.Create(fileName)
	require.NoError(t, err)
	_, err = w.Write(content)
	require.NoError(t, err)
}
