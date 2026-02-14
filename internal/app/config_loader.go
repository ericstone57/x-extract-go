package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"github.com/yourusername/x-extract-go/internal/domain"
)

// LoadConfig loads configuration following XDG Base Directory Specification.
// Config loading order (priority low to high):
// 1. Hardcoded defaults (domain.DefaultConfig())
// 2. System config (~/.config/x-extract-go/config.yaml or /app/config/config.yaml in Docker)
// 3. User override ($base_dir/config/config.yaml if exists)
func LoadConfig() (*domain.Config, error) {
	// 1. Start with hardcoded defaults
	config := domain.DefaultConfig()

	// 2. Ensure config directory exists
	configDir := domain.DefaultConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// 3. Create default config file if not exists
	configPath := domain.DefaultConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := createDefaultConfigFile(configPath); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
	}

	// 4. Load from default config file
	v := viper.New()
	v.SetConfigFile(configPath)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// 5. Expand paths (especially base_dir with $HOME)
	config = expandPaths(config)

	// 6. Create data directory structure based on base_dir
	if err := createBaseDirStructure(config); err != nil {
		return nil, err
	}

	// 7. Merge user override from base_dir/config/config.yaml (if exists)
	userConfigPath := filepath.Join(config.Download.ConfigDir(), "config.yaml")
	if _, err := os.Stat(userConfigPath); err == nil {
		userViper := viper.New()
		userViper.SetConfigFile(userConfigPath)
		if err := userViper.ReadInConfig(); err == nil {
			// Merge user config on top of system config
			if err := userViper.Unmarshal(config); err == nil {
				config = expandPaths(config)
			}
		}
	}

	// 8. Set default queue.db path if not specified
	if config.Queue.DatabasePath == "" {
		config.Queue.DatabasePath = domain.DefaultQueueDBPath()
	}

	// 9. Validate config
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// createDefaultConfigFile creates the default config.yaml with helpful comments
func createDefaultConfigFile(path string) error {
	content := `# X-Extract Configuration
# This file was auto-generated with default values.
# Edit as needed to customize your installation.

# Server settings
server:
  # Host address to bind the HTTP server
  host: localhost
  # Port for the HTTP server and web dashboard
  port: 8080

# Download settings
download:
  # Base directory for all downloads and data
  # Subdirectories are auto-created: completed/, incoming/, cookies/, logs/, config/
  # Local default: $HOME/Downloads/x-download
  # Docker default: /downloads
  base_dir: ""
  
  # Maximum retry attempts for failed downloads
  max_retries: 3
  
  # Delay between retry attempts
  retry_delay: 30s
  
  # Note: concurrent_limit is deprecated. Downloads use per-platform semaphores.
  concurrent_limit: 3
  
  # Automatically start download workers when server starts
  auto_start_workers: true

# Queue settings
queue:
  # Path to SQLite database (empty = use default: ~/.config/x-extract-go/queue.db)
  database_path: ""
  
  # Interval to check for new downloads
  check_interval: 10s
  
  # Automatically exit when queue is empty for the specified time
  # Set to false to keep the server running for dashboard access
  auto_exit_on_empty: true
  
  # Time to wait before auto-exit when queue is empty
  empty_wait_time: 30s

# Telegram settings
telegram:
  # Profile name for Telegram session
  profile: default
  
  # Storage type: bolt or memory
  storage_type: bolt
  
  # Path to Telegram session storage (empty = use default based on base_dir)
  storage_path: ""
  
  # Use group for downloads
  use_group: true
  
  # Rewrite file extensions
  rewrite_ext: true
  
  # Extra parameters for tdl command
  extra_params: ""
  
  # Path to tdl binary
  tdl_binary: tdl
  
  # Use takeout mode for Telegram
  takeout: false

# Twitter/X settings
twitter:
  # Path to cookie file (empty = use default based on base_dir)
  cookie_file: ""
  
  # Path to yt-dlp binary
  ytdlp_binary: yt-dlp
  
  # Write metadata alongside downloads
  write_metadata: true

# Notification settings
notification:
  # Enable desktop notifications
  enabled: true
  
  # Play sound on notification
  sound: true
  
  # Notification method: osascript (macOS), notify-send (Linux), etc.
  method: osascript

# Logging settings
logging:
  # Log level: debug, info, warn, error
  level: info
  
  # Log format: json, console
  format: console
  
  # Output path: stdout, stderr, auto (topic-based logs in base_dir/logs/), or file path
  output_path: stdout
`

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(content), 0644)
}

// expandPaths expands environment variables in path configurations
func expandPaths(config *domain.Config) *domain.Config {
	config.Download.BaseDir = expandPath(config.Download.BaseDir)
	config.Queue.DatabasePath = expandPath(config.Queue.DatabasePath)
	config.Telegram.StoragePath = expandPath(config.Telegram.StoragePath)
	config.Twitter.CookieFile = expandPath(config.Twitter.CookieFile)

	if config.Logging.OutputPath != "stdout" && config.Logging.OutputPath != "stderr" && config.Logging.OutputPath != "auto" {
		config.Logging.OutputPath = expandPath(config.Logging.OutputPath)
	}

	return config
}

// expandPath expands environment variables and ~ in paths
func expandPath(path string) string {
	if path == "" {
		return path
	}

	// Expand environment variables
	path = os.ExpandEnv(path)

	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	// Replace $HOME
	if strings.Contains(path, "$HOME") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = strings.ReplaceAll(path, "$HOME", home)
		}
	}

	return path
}

// createBaseDirStructure creates the data directory structure based on base_dir
func createBaseDirStructure(config *domain.Config) error {
	if config.Download.BaseDir == "" {
		// Set default base_dir if empty
		config.Download.BaseDir = domain.DefaultBaseDir()
	}

	dirs := []string{
		config.Download.BaseDir,
		config.Download.ConfigDir(),
		config.Download.CookiesDir(),
		config.Download.IncomingDir(),
		config.Download.CompletedDir(),
		config.Download.LogsDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// validateConfig validates the configuration
func validateConfig(config *domain.Config) error {
	if config.Server.Port < 1 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", config.Server.Port)
	}

	if config.Download.BaseDir == "" {
		return fmt.Errorf("download base directory not configured")
	}

	if config.Download.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}

	if config.Download.ConcurrentLimit < 1 {
		return fmt.Errorf("concurrent limit must be at least 1")
	}

	if config.Queue.DatabasePath == "" {
		return fmt.Errorf("queue database path not configured")
	}

	if config.Telegram.Profile == "" {
		return fmt.Errorf("telegram profile not configured")
	}

	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}

	return nil
}

// SaveConfig saves configuration to file
func SaveConfig(config *domain.Config, path string) error {
	v := viper.New()
	v.SetConfigType("yaml")

	// Marshal config to viper
	v.Set("server", config.Server)
	v.Set("download", config.Download)
	v.Set("queue", config.Queue)
	v.Set("telegram", config.Telegram)
	v.Set("twitter", config.Twitter)
	v.Set("notification", config.Notification)
	v.Set("logging", config.Logging)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config file
	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// MigrateOldStructure migrates files from old directory structure to new structure
// This provides backward compatibility for existing installations
func MigrateOldStructure(config *domain.Config) error {
	baseDir := config.Download.BaseDir

	// Check if old structure exists (files directly in base_dir)
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		// Base directory doesn't exist yet, no migration needed
		return nil
	}

	// Look for media files or old cookie files in base directory
	hasOldFiles := false
	for _, entry := range entries {
		if entry.IsDir() {
			// Skip subdirectories
			continue
		}
		name := entry.Name()
		// Check for media files or cookie files
		if isMediaFileName(name) || strings.HasSuffix(name, ".cookie") || name == "queue.db" {
			hasOldFiles = true
			break
		}
	}

	if !hasOldFiles {
		// No old files found, no migration needed
		return nil
	}

	fmt.Println("Detected old directory structure. Migrating files...")

	// Migrate media files to completed directory
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		oldPath := filepath.Join(baseDir, name)

		if isMediaFileName(name) {
			// Move media files to completed directory
			newPath := filepath.Join(config.Download.CompletedDir(), name)
			if err := os.MkdirAll(config.Download.CompletedDir(), 0755); err != nil {
				return fmt.Errorf("failed to create completed directory: %w", err)
			}
			if err := os.Rename(oldPath, newPath); err != nil {
				fmt.Printf("Warning: failed to migrate %s: %v\n", name, err)
			} else {
				fmt.Printf("Migrated: %s -> completed/%s\n", name, name)
			}
		} else if strings.HasSuffix(name, ".cookie") {
			// Move cookie files to cookies/x.com directory
			cookieDir := filepath.Join(config.Download.CookiesDir(), "x.com")
			if err := os.MkdirAll(cookieDir, 0755); err != nil {
				return fmt.Errorf("failed to create cookies directory: %w", err)
			}
			newPath := filepath.Join(cookieDir, name)
			if err := os.Rename(oldPath, newPath); err != nil {
				fmt.Printf("Warning: failed to migrate %s: %v\n", name, err)
			} else {
				fmt.Printf("Migrated: %s -> cookies/x.com/%s\n", name, name)
			}
		}
	}

	// Migrate old tdl storage directories
	oldTdlPattern := filepath.Join(baseDir, "tdl-*")
	matches, _ := filepath.Glob(oldTdlPattern)
	for _, oldTdlPath := range matches {
		dirName := filepath.Base(oldTdlPath)
		profile := strings.TrimPrefix(dirName, "tdl-")
		newTdlPath := filepath.Join(config.Download.CookiesDir(), "telegram", profile)
		if err := os.MkdirAll(filepath.Dir(newTdlPath), 0755); err != nil {
			return fmt.Errorf("failed to create telegram directory: %w", err)
		}
		if err := os.Rename(oldTdlPath, newTdlPath); err != nil {
			fmt.Printf("Warning: failed to migrate %s: %v\n", dirName, err)
		} else {
			fmt.Printf("Migrated: %s -> cookies/telegram/%s\n", dirName, profile)
		}
	}

	fmt.Println("Migration completed!")
	return nil
}

// isMediaFileName checks if a filename is a media file
func isMediaFileName(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	mediaExts := []string{".mp4", ".mkv", ".avi", ".mov", ".webm", ".m4v", ".jpg", ".png", ".gif", ".webp", ".json"}
	for _, mediaExt := range mediaExts {
		if ext == mediaExt {
			return true
		}
	}
	return false
}

// GenerateDefaultConfig generates the default config content as bytes
func GenerateDefaultConfig() ([]byte, error) {
	config := domain.DefaultConfig()

	v := viper.New()
	v.Set("server", config.Server)
	v.Set("download", config.Download)
	v.Set("queue", config.Queue)
	v.Set("telegram", config.Telegram)
	v.Set("twitter", config.Twitter)
	v.Set("notification", config.Notification)
	v.Set("logging", config.Logging)

	// Write to temp file and read back
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	if err := v.WriteConfigAs(tmpFile.Name()); err != nil {
		return nil, err
	}

	return os.ReadFile(tmpFile.Name())
}
