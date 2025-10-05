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
	Directories    []DirectoryConfig   `yaml:"directories"`
	Notification   NotificationConfig  `yaml:"notification"`
	WakeupHandling WakeupHandlingConfig `yaml:"wakeup_handling"`
	Logging        LoggingConfig       `yaml:"logging"`
}

// DirectoryConfig represents configuration for a single CalDAV directory
type DirectoryConfig struct {
	Directory       string        `yaml:"directory"`
	Template        string        `yaml:"template"`
	AutomaticAlerts []AlertConfig `yaml:"automatic_alerts"`
}

// AlertConfig represents an alert timing configuration
type AlertConfig struct {
	Value     int  `yaml:"value"`
	Unit      string `yaml:"unit"`
	Important bool `yaml:"important"`
}

// DurationConfig represents a user-friendly duration configuration
type DurationConfig struct {
	Type  string `yaml:"type"`            // "timed" or "until_dismissed"
	Value int    `yaml:"value,omitempty"` // Only required for "timed" type
	Unit  string `yaml:"unit,omitempty"`  // Only required for "timed" type
}

// NotificationConfig represents notification system configuration
type NotificationConfig struct {
	Backend          string         `yaml:"backend"`
	Duration         DurationConfig `yaml:"duration"`
	DurationWhenLate DurationConfig `yaml:"duration_when_late"`
}

// WakeupHandlingConfig represents wake-up detection and missed event handling
type WakeupHandlingConfig struct {
	Enable             bool           `yaml:"enable"`
	MissedEventPolicy  string         `yaml:"missed_event_policy"`  // "all", "summary", "priority_only", "skip"
	MaxMissedDays      int            `yaml:"max_missed_days"`
	SummaryThreshold   int            `yaml:"summary_threshold"`
	MaxCatchupTime     DurationConfig `yaml:"max_catchup_time"`
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

// IsUntilDismissed returns true if this duration is of type "until_dismissed"
func (d DurationConfig) IsUntilDismissed() bool {
	return d.Type == "until_dismissed"
}

// ToMilliseconds converts the duration to milliseconds for D-Bus notifications
func (d DurationConfig) ToMilliseconds() (int32, error) {
	if d.IsUntilDismissed() {
		return 0, nil // D-Bus: 0 means never auto-dismiss
	}
	
	// Default to "timed" if type is not specified
	if d.Type == "" || d.Type == "timed" {
		duration, err := d.ToDuration()
		if err != nil {
			return 0, err
		}
		return int32(duration.Milliseconds()), nil
	}
	
	return 0, fmt.Errorf("unsupported duration type: %s", d.Type)
}

// ToDuration converts DurationConfig to time.Duration (only for "timed" type)
func (d DurationConfig) ToDuration() (time.Duration, error) {
	if d.IsUntilDismissed() {
		return 0, fmt.Errorf("cannot convert 'until_dismissed' duration to time.Duration")
	}
	
	if d.Value <= 0 {
		return 0, fmt.Errorf("duration value must be positive")
	}
	
	switch d.Unit {
	case "milliseconds", "millisecond", "ms":
		return time.Duration(d.Value) * time.Millisecond, nil
	case "seconds", "second", "s", "":
		// Default to seconds if unit is not specified
		return time.Duration(d.Value) * time.Second, nil
	case "minutes", "minute", "m":
		return time.Duration(d.Value) * time.Minute, nil
	case "hours", "hour", "h":
		return time.Duration(d.Value) * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported time unit: %s", d.Unit)
	}
}

// Validate validates the DurationConfig
func (d DurationConfig) Validate() error {
	// Default to "timed" if type is not specified
	if d.Type == "" {
		return nil // Will be handled by default value in validation
	}
	
	if d.Type != "timed" && d.Type != "until_dismissed" {
		return fmt.Errorf("duration type must be 'timed' or 'until_dismissed', got: %s", d.Type)
	}
	
	if d.Type == "timed" {
		if d.Value <= 0 {
			return fmt.Errorf("duration value must be positive for 'timed' type")
		}
		// Validate that we can convert to duration
		_, err := d.ToDuration()
		return err
	}
	
	// For "until_dismissed", value and unit are not required
	return nil
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

// Validate checks if the configuration is valid and applies defaults
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

	// Validate and apply defaults for notification configuration
	if c.Notification.Backend == "" {
		c.Notification.Backend = "notify-send"
	}
	if c.Notification.Backend != "notify-send" {
		return fmt.Errorf("unsupported notification backend: %s", c.Notification.Backend)
	}

	// Apply defaults for notification duration
	if c.Notification.Duration.Type == "" {
		c.Notification.Duration = DurationConfig{
			Type:  "timed",
			Value: 5,
			Unit:  "seconds",
		}
	}
	if err := c.Notification.Duration.Validate(); err != nil {
		return fmt.Errorf("notification duration: %w", err)
	}

	// Apply defaults for late notification duration
	if c.Notification.DurationWhenLate.Type == "" {
		c.Notification.DurationWhenLate = DurationConfig{
			Type: "until_dismissed",
		}
	}
	if err := c.Notification.DurationWhenLate.Validate(); err != nil {
		return fmt.Errorf("notification duration_when_late: %w", err)
	}

	// Apply defaults and validate wake-up handling
	if c.WakeupHandling.MissedEventPolicy == "" {
		c.WakeupHandling.MissedEventPolicy = "all"
	}
	validPolicies := map[string]bool{
		"all":           true,
		"summary":       true,
		"priority_only": true,
		"skip":          true,
	}
	if !validPolicies[c.WakeupHandling.MissedEventPolicy] {
		return fmt.Errorf("invalid missed_event_policy: %s", c.WakeupHandling.MissedEventPolicy)
	}

	if c.WakeupHandling.MaxMissedDays <= 0 {
		c.WakeupHandling.MaxMissedDays = 7
	}

	if c.WakeupHandling.SummaryThreshold <= 0 {
		c.WakeupHandling.SummaryThreshold = 5
	}

	// Apply defaults for max catchup time
	if c.WakeupHandling.MaxCatchupTime.Type == "" {
		c.WakeupHandling.MaxCatchupTime = DurationConfig{
			Type:  "timed",
			Value: 30,
			Unit:  "seconds",
		}
	}
	if err := c.WakeupHandling.MaxCatchupTime.Validate(); err != nil {
		return fmt.Errorf("wakeup_handling max_catchup_time: %w", err)
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
					{Value: 5, Unit: "minutes", Important: false},
				},
			},
		},
		Notification: NotificationConfig{
			Backend: "notify-send",
			Duration: DurationConfig{
				Type:  "timed",
				Value: 5,
				Unit:  "seconds",
			},
			DurationWhenLate: DurationConfig{
				Type: "until_dismissed",
			},
		},
		WakeupHandling: WakeupHandlingConfig{
			Enable:            true,
			MissedEventPolicy: "all",
			MaxMissedDays:     7,
			SummaryThreshold:  5,
			MaxCatchupTime: DurationConfig{
				Type:  "timed",
				Value: 30,
				Unit:  "seconds",
			},
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