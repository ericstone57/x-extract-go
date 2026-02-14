package domain

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config represents the application configuration
type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Download     DownloadConfig     `mapstructure:"download"`
	Queue        QueueConfig        `mapstructure:"queue"`
	Telegram     TelegramConfig     `mapstructure:"telegram"`
	Twitter      TwitterConfig      `mapstructure:"twitter"`
	Notification NotificationConfig `mapstructure:"notification"`
	Logging      LoggingConfig      `mapstructure:"logging"`
}

// ServerConfig contains server-related configuration
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// DownloadConfig contains download-related configuration
type DownloadConfig struct {
	BaseDir    string        `mapstructure:"base_dir"`
	MaxRetries int           `mapstructure:"max_retries"`
	RetryDelay time.Duration `mapstructure:"retry_delay"`
	// Deprecated: ConcurrentLimit is no longer used for global concurrency control.
	// Downloads now use per-platform semaphores (limit=1 per platform), allowing
	// different platforms to download in parallel while serializing same-platform downloads.
	// This field is kept for backward compatibility with existing config files.
	ConcurrentLimit  int  `mapstructure:"concurrent_limit"`
	AutoStartWorkers bool `mapstructure:"auto_start_workers"`
}

// CompletedDir returns the completed downloads directory (base_dir/completed)
func (c *DownloadConfig) CompletedDir() string {
	return filepath.Join(c.BaseDir, "completed")
}

// IncomingDir returns the incoming downloads directory (base_dir/incoming)
func (c *DownloadConfig) IncomingDir() string {
	return filepath.Join(c.BaseDir, "incoming")
}

// CookiesDir returns the cookies directory (base_dir/cookies)
func (c *DownloadConfig) CookiesDir() string {
	return filepath.Join(c.BaseDir, "cookies")
}

// LogsDir returns the logs directory (base_dir/logs)
func (c *DownloadConfig) LogsDir() string {
	return filepath.Join(c.BaseDir, "logs")
}

// ConfigDir returns the config directory (base_dir/config)
func (c *DownloadConfig) ConfigDir() string {
	return filepath.Join(c.BaseDir, "config")
}

// QueueConfig contains queue-related configuration
type QueueConfig struct {
	DatabasePath    string        `mapstructure:"database_path"`
	CheckInterval   time.Duration `mapstructure:"check_interval"`
	AutoExitOnEmpty bool          `mapstructure:"auto_exit_on_empty"`
	EmptyWaitTime   time.Duration `mapstructure:"empty_wait_time"`
}

// TelegramConfig contains Telegram-specific configuration
type TelegramConfig struct {
	Profile     string `mapstructure:"profile"`
	StorageType string `mapstructure:"storage_type"`
	StoragePath string `mapstructure:"storage_path"`
	UseGroup    bool   `mapstructure:"use_group"`
	RewriteExt  bool   `mapstructure:"rewrite_ext"`
	ExtraParams string `mapstructure:"extra_params"`
	TDLBinary   string `mapstructure:"tdl_binary"`
	Takeout     bool   `mapstructure:"takeout"` // Use takeout mode for Telegram
}

// TwitterConfig contains Twitter/X-specific configuration
type TwitterConfig struct {
	CookieFile    string `mapstructure:"cookie_file"`
	YTDLPBinary   string `mapstructure:"ytdlp_binary"`
	WriteMetadata bool   `mapstructure:"write_metadata"`
}

// NotificationConfig contains notification-related configuration
type NotificationConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Sound   bool   `mapstructure:"sound"`
	Method  string `mapstructure:"method"` // osascript, notify-send, etc.
}

// LoggingConfig contains logging-related configuration
type LoggingConfig struct {
	Level      string `mapstructure:"level"`       // debug, info, warn, error
	Format     string `mapstructure:"format"`      // json, console
	OutputPath string `mapstructure:"output_path"` // stdout, stderr, or file path
}

// IsDocker detects if running inside a Docker container
func IsDocker() bool {
	// Check for .dockerenv file
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	// Check for Docker in cgroup
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		if strings.Contains(string(data), "docker") ||
			strings.Contains(string(data), "kubepods") ||
			strings.Contains(string(data), "containerd") {
			return true
		}
	}
	return false
}

// DefaultConfigDir returns the default configuration directory path
// Follows XDG Base Directory Specification:
// - Uses $XDG_CONFIG_HOME/x-extract-go if XDG_CONFIG_HOME is set
// - Otherwise uses $HOME/.config/x-extract-go
// - In Docker, uses /app/config
func DefaultConfigDir() string {
	if IsDocker() {
		return "/app/config"
	}

	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig != "" {
		return filepath.Join(xdgConfig, "x-extract-go")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "~"
	}
	return filepath.Join(home, ".config", "x-extract-go")
}

// DefaultConfigPath returns the default configuration file path
func DefaultConfigPath() string {
	return filepath.Join(DefaultConfigDir(), "config.yaml")
}

// DefaultQueueDBPath returns the default queue database path
func DefaultQueueDBPath() string {
	return filepath.Join(DefaultConfigDir(), "queue.db")
}

// DefaultBaseDir returns the default data directory (base_dir)
// - Local: $HOME/Downloads/x-download
// - Docker: /downloads
func DefaultBaseDir() string {
	if IsDocker() {
		return "/downloads"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "$HOME/Downloads/x-download"
	}
	return filepath.Join(home, "Downloads", "x-download")
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	baseDir := DefaultBaseDir()

	return &Config{
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Download: DownloadConfig{
			BaseDir:          baseDir,
			MaxRetries:       3,
			RetryDelay:       30 * time.Second,
			ConcurrentLimit:  3,
			AutoStartWorkers: true,
		},
		Queue: QueueConfig{
			DatabasePath:    "", // Empty means use DefaultQueueDBPath()
			CheckInterval:   10 * time.Second,
			AutoExitOnEmpty: true,
			EmptyWaitTime:   30 * time.Second,
		},
		Telegram: TelegramConfig{
			Profile:     "default",
			StorageType: "bolt",
			StoragePath: filepath.Join(baseDir, "cookies", "telegram", "default"),
			UseGroup:    true,
			RewriteExt:  true,
			ExtraParams: "",
			TDLBinary:   "tdl",
			Takeout:     false,
		},
		Twitter: TwitterConfig{
			CookieFile:    filepath.Join(baseDir, "cookies", "x.com", "default.cookie"),
			YTDLPBinary:   "yt-dlp",
			WriteMetadata: true,
		},
		Notification: NotificationConfig{
			Enabled: true,
			Sound:   true,
			Method:  "osascript",
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "console",
			OutputPath: "stdout",
		},
	}
}
