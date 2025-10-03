package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Directories  []DirectoryConfig `yaml:"directories"`
	Notification NotificationConfig `yaml:"notification"`
	Logging      LoggingConfig     `yaml:"logging"`
}

// DirectoryConfig represents configuration for a single CalDAV directory
type DirectoryConfig struct {
	Directory       string        `yaml:"directory"`
	Template        string        `yaml:"template"`
	AutomaticAlerts []AlertConfig `yaml:"automatic_alerts"`
}

// AlertConfig represents an alert timing configuration
type AlertConfig struct {
	Value int    `yaml:"value"`
	Unit  string `yaml:"unit"`
}

// NotificationConfig represents notification system configuration
type NotificationConfig struct {
	Backend  string `yaml:"backend"`
	Duration int    `yaml:"duration"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file,omitempty"`
}

// Duration converts AlertConfig to time.Duration
func (a AlertConfig) Duration() (time.Duration, error) {
	switch a.Unit {
	case "seconds", "second", "s":
		return time.Duration(a.Value) * time.Second, nil
	case "minutes", "minute", "m":
		return time.Duration(a.Value) * time.Minute, nil
	case "hours", "hour", "h":
		return time.Duration(a.Value) * time.Hour, nil
	case "days", "day", "d":
		return time.Duration(a.Value) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported time unit: %s", a.Unit)
	}
}

// ExpandPath expands ~ and environment variables in paths
func (d *DirectoryConfig) ExpandPath() error {
	expanded := os.ExpandEnv(d.Directory)
	if len(expanded) > 0 && expanded[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		expanded = filepath.Join(homeDir, expanded[1:])
	}
	d.Directory = expanded
	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if len(c.Directories) == 0 {
		return fmt.Errorf("at least one directory must be configured")
	}

	for i, dir := range c.Directories {
		if dir.Directory == "" {
			return fmt.Errorf("directory %d: directory path cannot be empty", i)
		}

		// Expand path for validation
		if err := c.Directories[i].ExpandPath(); err != nil {
			return fmt.Errorf("directory %d: %w", i, err)
		}

		// Check if directory exists
		if _, err := os.Stat(c.Directories[i].Directory); os.IsNotExist(err) {
			return fmt.Errorf("directory %d: directory does not exist: %s", i, c.Directories[i].Directory)
		}

		// Validate alert configurations
		for j, alert := range dir.AutomaticAlerts {
			if alert.Value <= 0 {
				return fmt.Errorf("directory %d, alert %d: value must be positive", i, j)
			}
			if _, err := alert.Duration(); err != nil {
				return fmt.Errorf("directory %d, alert %d: %w", i, j, err)
			}
		}
	}

	// Validate notification backend
	if c.Notification.Backend == "" {
		c.Notification.Backend = "notify-send"
	}
	if c.Notification.Backend != "notify-send" {
		return fmt.Errorf("unsupported notification backend: %s", c.Notification.Backend)
	}

	// Validate logging level
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid logging level: %s", c.Logging.Level)
	}

	return nil
}

// Load loads configuration from XDG-compliant locations
func Load() (*Config, error) {
	// Try to find config file in XDG config directories
	configPath, err := xdg.SearchConfigFile("calwatch/config.yaml")
	if err != nil {
		// If not found, try creating a default config path
		configPath, err = xdg.ConfigFile("calwatch/config.yaml")
		if err != nil {
			return nil, fmt.Errorf("failed to determine config file path: %w", err)
		}
		
		// Check if config file exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found at %s", configPath)
		}
	}

	return LoadFromFile(configPath)
}

// LoadFromFile loads configuration from a specific file path
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Directories: []DirectoryConfig{
			{
				Directory: "~/.calendars",
				Template:  "default.tpl",
				AutomaticAlerts: []AlertConfig{
					{Value: 5, Unit: "minutes"},
				},
			},
		},
		Notification: NotificationConfig{
			Backend:  "notify-send",
			Duration: 5000,
		},
		Logging: LoggingConfig{
			Level: "info",
		},
	}
}

// WriteDefaultConfig writes a default configuration to the XDG config directory
func WriteDefaultConfig() (string, error) {
	configPath, err := xdg.ConfigFile("calwatch/config.yaml")
	if err != nil {
		return "", fmt.Errorf("failed to determine config file path: %w", err)
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	config := DefaultConfig()
	data, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal default config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write config file: %w", err)
	}

	return configPath, nil
}