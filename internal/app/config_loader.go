package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"github.com/yourusername/x-extract-go/internal/domain"
)

// LoadConfig loads configuration from file and environment
func LoadConfig(configPath string) (*domain.Config, error) {
	// Start with default config
	config := domain.DefaultConfig()

	// Set up viper
	v := viper.New()
	v.SetConfigType("yaml")

	// If config path is provided, use it
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// First try to load local.yaml from base_dir/config/
		// This will be attempted after we know the base_dir
		// For now, look for config in standard locations
		v.SetConfigName("config")
		v.AddConfigPath("./configs")
		v.AddConfigPath("$HOME/.x-extract")
		v.AddConfigPath("/etc/x-extract")
	}

	// Read environment variables
	v.SetEnvPrefix("XEXTRACT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Try to read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal into config struct
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Expand environment variables in paths
	config = expandPaths(config)

	// Try to load config.yaml from config directory (cascade)
	// This allows runtime config to override the default configs/config.yaml
	configDirConfigPath := filepath.Join(config.Download.ConfigDir(), "config.yaml")
	if _, err := os.Stat(configDirConfigPath); err == nil {
		configDirViper := viper.New()
		configDirViper.SetConfigFile(configDirConfigPath)
		if err := configDirViper.ReadInConfig(); err == nil {
			if err := configDirViper.Unmarshal(config); err == nil {
				config = expandPaths(config)
			}
		}
	}

	// Try to load local.yaml from config directory (cascade)
	// This has the highest priority for local overrides
	localConfigPath := filepath.Join(config.Download.ConfigDir(), "local.yaml")
	if _, err := os.Stat(localConfigPath); err == nil {
		localViper := viper.New()
		localViper.SetConfigFile(localConfigPath)
		if err := localViper.ReadInConfig(); err == nil {
			if err := localViper.Unmarshal(config); err == nil {
				config = expandPaths(config)
			}
		}
	}

	// Validate config
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// expandPaths expands environment variables in path configurations
func expandPaths(config *domain.Config) *domain.Config {
	config.Download.BaseDir = expandPath(config.Download.BaseDir)
	config.Queue.DatabasePath = expandPath(config.Queue.DatabasePath)
	config.Telegram.StoragePath = expandPath(config.Telegram.StoragePath)
	config.Twitter.CookieFile = expandPath(config.Twitter.CookieFile)

	if config.Logging.OutputPath != "stdout" && config.Logging.OutputPath != "stderr" {
		config.Logging.OutputPath = expandPath(config.Logging.OutputPath)
	}

	return config
}

// expandPath expands environment variables and ~ in paths
func expandPath(path string) string {
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
		} else if name == "queue.db" {
			// Move database to config directory
			if err := os.MkdirAll(config.Download.ConfigDir(), 0755); err != nil {
				return fmt.Errorf("failed to create config directory: %w", err)
			}
			newPath := filepath.Join(config.Download.ConfigDir(), name)
			if err := os.Rename(oldPath, newPath); err != nil {
				fmt.Printf("Warning: failed to migrate %s: %v\n", name, err)
			} else {
				fmt.Printf("Migrated: %s -> config/%s\n", name, name)
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
