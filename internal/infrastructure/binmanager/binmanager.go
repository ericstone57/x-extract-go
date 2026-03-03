package binmanager

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ToolSpec describes an external tool that can be auto-installed.
type ToolSpec struct {
	Name       string // e.g. "yt-dlp", "tdl"
	BinaryName string // final binary name on disk (e.g. "yt-dlp", "tdl")
	GitHubRepo string // e.g. "yt-dlp/yt-dlp", "iyear/tdl"

	// AssetName returns the GitHub release asset filename for the given OS/arch.
	AssetName func(goos, goarch string) string

	// ChecksumAsset is the name of the checksum file in the release (e.g. "SHA2-256SUMS").
	ChecksumAsset string

	// IsArchive indicates the asset is a tar.gz or zip that must be extracted.
	IsArchive bool
}

// KnownTools is the registry of tools that can be auto-managed.
var KnownTools = map[string]ToolSpec{
	"yt-dlp": {
		Name:       "yt-dlp",
		BinaryName: "yt-dlp",
		GitHubRepo: "yt-dlp/yt-dlp",
		AssetName: func(goos, goarch string) string {
			switch {
			case goos == "darwin":
				return "yt-dlp_macos"
			case goos == "linux" && goarch == "arm64":
				return "yt-dlp_linux_aarch64"
			case goos == "linux":
				return "yt-dlp_linux"
			case goos == "windows":
				return "yt-dlp.exe"
			default:
				return "yt-dlp_linux"
			}
		},
		ChecksumAsset: "SHA2-256SUMS",
		IsArchive:     false,
	},
	"tdl": {
		Name:       "tdl",
		BinaryName: "tdl",
		GitHubRepo: "iyear/tdl",
		AssetName: func(goos, goarch string) string {
			osName := map[string]string{"darwin": "MacOS", "linux": "Linux", "windows": "Windows"}[goos]
			if osName == "" {
				osName = "Linux"
			}
			archName := map[string]string{"amd64": "64bit", "arm64": "arm64"}[goarch]
			if archName == "" {
				archName = "64bit"
			}
			ext := "tar.gz"
			if goos == "windows" {
				ext = "zip"
			}
			return fmt.Sprintf("tdl_%s_%s.%s", osName, archName, ext)
		},
		ChecksumAsset: "tdl_checksums.txt",
		IsArchive:     true,
	},
	"gallery-dl": {
		Name:       "gallery-dl",
		BinaryName: "gallery-dl",
		GitHubRepo: "mikf/gallery-dl",
		AssetName: func(goos, goarch string) string {
			if goos == "windows" {
				return "gallery-dl.exe"
			}
			return "gallery-dl.bin"
		},
		ChecksumAsset: "", // gallery-dl uses PGP signatures, not SHA256 checksums
		IsArchive:     false,
	},
}

// ResolveBinary finds the path to a tool binary using the resolution order:
// 1. configPath (explicit user override in config)
// 2. System PATH (exec.LookPath) — skipped when preferManaged is true
// 3. Managed binary dir (binDir)
// Returns the resolved path, or an error if not found anywhere.
func ResolveBinary(toolName, configPath, binDir string, preferManaged bool) (string, error) {
	// 1. Explicit config path
	if configPath != "" && configPath != toolName {
		// User specified an absolute/relative path — check it exists
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
		// If the config path looks like an absolute path but doesn't exist, fail
		if filepath.IsAbs(configPath) {
			return "", fmt.Errorf("%s binary not found at configured path: %s", toolName, configPath)
		}
	}

	// 2. System PATH (skip when preferManaged is set — forces managed binary usage)
	if !preferManaged {
		if path, err := exec.LookPath(toolName); err == nil {
			return path, nil
		}
	}

	// 3. Managed binary dir
	spec, ok := KnownTools[toolName]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
	managedPath := filepath.Join(binDir, spec.BinaryName)
	if runtime.GOOS == "windows" {
		managedPath += ".exe"
	}
	if _, err := os.Stat(managedPath); err == nil {
		return managedPath, nil
	}

	return "", fmt.Errorf("%s not found (checked: PATH, %s). Enable auto_install or install manually", toolName, managedPath)
}

// DownloadTool downloads and installs a tool from GitHub releases.
// version can be "latest" or a specific version tag like "2026.02.21" or "v0.20.1".
func DownloadTool(spec ToolSpec, version, binDir string) (string, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Resolve version tag
	tag, err := resolveVersion(spec.GitHubRepo, version)
	if err != nil {
		return "", fmt.Errorf("resolve version for %s: %w", spec.Name, err)
	}

	assetName := spec.AssetName(goos, goarch)
	assetURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", spec.GitHubRepo, tag, assetName)

	// Create temp dir for download
	tmpDir, err := os.MkdirTemp("", "binmanager-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download asset
	assetPath := filepath.Join(tmpDir, assetName)
	if err := downloadFile(assetURL, assetPath); err != nil {
		return "", fmt.Errorf("download %s: %w", spec.Name, err)
	}

	// Download and verify checksum
	if spec.ChecksumAsset != "" {
		checksumURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", spec.GitHubRepo, tag, spec.ChecksumAsset)
		checksumPath := filepath.Join(tmpDir, spec.ChecksumAsset)
		if err := downloadFile(checksumURL, checksumPath); err != nil {
			return "", fmt.Errorf("download checksum for %s: %w", spec.Name, err)
		}
		if err := verifyChecksum(assetPath, checksumPath, assetName); err != nil {
			return "", fmt.Errorf("checksum verification failed for %s: %w", spec.Name, err)
		}
	}

	// Ensure binDir exists
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("create bin directory: %w", err)
	}

	destPath := filepath.Join(binDir, spec.BinaryName)
	if runtime.GOOS == "windows" {
		destPath += ".exe"
	}

	// Extract or copy
	if spec.IsArchive {
		if err := extractBinary(assetPath, tmpDir, spec.BinaryName); err != nil {
			return "", fmt.Errorf("extract %s: %w", spec.Name, err)
		}
		// The extracted binary is in tmpDir
		extractedPath := filepath.Join(tmpDir, spec.BinaryName)
		if runtime.GOOS == "windows" {
			extractedPath += ".exe"
		}
		if err := copyFileAtomic(extractedPath, destPath); err != nil {
			return "", fmt.Errorf("install %s: %w", spec.Name, err)
		}
	} else {
		if err := copyFileAtomic(assetPath, destPath); err != nil {
			return "", fmt.Errorf("install %s: %w", spec.Name, err)
		}
	}

	// Make executable
	if err := os.Chmod(destPath, 0755); err != nil {
		return "", fmt.Errorf("chmod %s: %w", spec.Name, err)
	}

	return destPath, nil
}

// ResolveOrInstall resolves a binary, auto-downloading if not found and autoInstall is true.
// When preferManaged is true, system PATH is skipped so the managed binary is always used
// (downloading it if necessary and autoInstall is true).
func ResolveOrInstall(toolName, configPath, binDir, version string, autoInstall, preferManaged bool) (string, error) {
	path, err := ResolveBinary(toolName, configPath, binDir, preferManaged)
	if err == nil {
		return path, nil
	}

	if !autoInstall {
		return "", err
	}

	spec, ok := KnownTools[toolName]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}

	return DownloadTool(spec, version, binDir)
}

// resolveVersion resolves "latest" to the actual tag name from GitHub.
func resolveVersion(repo, version string) (string, error) {
	if version != "" && version != "latest" {
		return version, nil
	}

	url := fmt.Sprintf("https://github.com/%s/releases/latest", repo)
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirect, we want the Location header
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		return "", fmt.Errorf("unexpected status %d from GitHub releases", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	// Location is like: https://github.com/yt-dlp/yt-dlp/releases/tag/2026.02.21
	parts := strings.Split(location, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("could not parse version from redirect: %s", location)
	}
	tag := parts[len(parts)-1]
	if tag == "" {
		return "", fmt.Errorf("empty tag from redirect: %s", location)
	}
	return tag, nil
}

// downloadFile downloads a URL to a local file path.
func downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", destPath, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("write file %s: %w", destPath, err)
	}
	return nil
}

// verifyChecksum verifies the SHA256 checksum of a file against a checksums file.
// The checksums file should have lines in format: "<hash>  <filename>" or "<hash> <filename>".
func verifyChecksum(filePath, checksumFilePath, assetName string) error {
	// Compute SHA256 of the file
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash file: %w", err)
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	// Read checksums file and find matching line
	data, err := os.ReadFile(checksumFilePath)
	if err != nil {
		return fmt.Errorf("read checksum file: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "<hash>  <filename>" or "<hash> <filename>"
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == assetName {
			expectedHash := strings.ToLower(fields[0])
			if actualHash != expectedHash {
				return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
			}
			return nil
		}
	}

	return fmt.Errorf("no checksum found for %s in checksum file", assetName)
}

// extractBinary extracts a binary from a tar.gz or zip archive.
func extractBinary(archivePath, destDir, binaryName string) error {
	if strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
		return extractTarGz(archivePath, destDir, binaryName)
	}
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZip(archivePath, destDir, binaryName)
	}
	return fmt.Errorf("unsupported archive format: %s", archivePath)
}

// extractTarGz extracts the named binary from a tar.gz archive.
func extractTarGz(archivePath, destDir, binaryName string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Match the binary name (may be in a subdirectory)
		name := filepath.Base(header.Name)
		if name == binaryName || name == binaryName+".exe" {
			destPath := filepath.Join(destDir, name)
			outFile, err := os.Create(destPath)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
			return nil
		}
	}
	return fmt.Errorf("binary %s not found in archive", binaryName)
}

// extractZip extracts the named binary from a zip archive.
func extractZip(archivePath, destDir, binaryName string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if name == binaryName || name == binaryName+".exe" {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			destPath := filepath.Join(destDir, name)
			outFile, err := os.Create(destPath)
			if err != nil {
				rc.Close()
				return err
			}
			if _, err := io.Copy(outFile, rc); err != nil {
				outFile.Close()
				rc.Close()
				return err
			}
			outFile.Close()
			rc.Close()
			return nil
		}
	}
	return fmt.Errorf("binary %s not found in archive", binaryName)
}

// copyFileAtomic copies a file atomically by writing to a temp file then renaming.
func copyFileAtomic(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	tmpPath := dst + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0755); err != nil {
		return err
	}

	return os.Rename(tmpPath, dst)
}
