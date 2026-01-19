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
		// Look for config in standard locations
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
		// Config file not found, use defaults
	}

	// Unmarshal into config struct
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Expand environment variables in paths
	config = expandPaths(config)

	// Validate config
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// expandPaths expands environment variables in path configurations
func expandPaths(config *domain.Config) *domain.Config {
	config.Download.BaseDir = expandPath(config.Download.BaseDir)
	config.Download.TempDir = expandPath(config.Download.TempDir)
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

