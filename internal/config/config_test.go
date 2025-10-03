package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAlertConfig_Duration(t *testing.T) {
	tests := []struct {
		name    string
		alert   AlertConfig
		want    time.Duration
		wantErr bool
	}{
		{
			name:  "seconds",
			alert: AlertConfig{Value: 30, Unit: "seconds"},
			want:  30 * time.Second,
		},
		{
			name:  "minutes",
			alert: AlertConfig{Value: 5, Unit: "minutes"},
			want:  5 * time.Minute,
		},
		{
			name:  "hours",
			alert: AlertConfig{Value: 2, Unit: "hours"},
			want:  2 * time.Hour,
		},
		{
			name:  "days",
			alert: AlertConfig{Value: 1, Unit: "days"},
			want:  24 * time.Hour,
		},
		{
			name:    "invalid unit",
			alert:   AlertConfig{Value: 5, Unit: "weeks"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.alert.Duration()
			if (err != nil) != tt.wantErr {
				t.Errorf("AlertConfig.Duration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("AlertConfig.Duration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDirectoryConfig_ExpandPath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	tests := []struct {
		name     string
		dir      DirectoryConfig
		expected string
	}{
		{
			name:     "tilde expansion",
			dir:      DirectoryConfig{Directory: "~/.calendars"},
			expected: filepath.Join(homeDir, ".calendars"),
		},
		{
			name:     "absolute path",
			dir:      DirectoryConfig{Directory: "/tmp/calendars"},
			expected: "/tmp/calendars",
		},
		{
			name:     "relative path",
			dir:      DirectoryConfig{Directory: "calendars"},
			expected: "calendars",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dir.ExpandPath()
			if err != nil {
				t.Errorf("DirectoryConfig.ExpandPath() error = %v", err)
				return
			}
			if tt.dir.Directory != tt.expected {
				t.Errorf("DirectoryConfig.ExpandPath() = %v, want %v", tt.dir.Directory, tt.expected)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "calwatch_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Directories: []DirectoryConfig{
					{
						Directory: tempDir,
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
			},
			wantErr: false,
		},
		{
			name: "no directories",
			config: Config{
				Directories: []DirectoryConfig{},
			},
			wantErr: true,
		},
		{
			name: "empty directory path",
			config: Config{
				Directories: []DirectoryConfig{
					{Directory: ""},
				},
			},
			wantErr: true,
		},
		{
			name: "nonexistent directory",
			config: Config{
				Directories: []DirectoryConfig{
					{Directory: "/nonexistent/path"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid alert value",
			config: Config{
				Directories: []DirectoryConfig{
					{
						Directory: tempDir,
						AutomaticAlerts: []AlertConfig{
							{Value: -5, Unit: "minutes"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid alert unit",
			config: Config{
				Directories: []DirectoryConfig{
					{
						Directory: tempDir,
						AutomaticAlerts: []AlertConfig{
							{Value: 5, Unit: "weeks"},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	if len(config.Directories) != 1 {
		t.Errorf("DefaultConfig() should have 1 directory, got %d", len(config.Directories))
	}
	
	if config.Notification.Backend != "notify-send" {
		t.Errorf("DefaultConfig() notification backend = %v, want notify-send", config.Notification.Backend)
	}
	
	if config.Logging.Level != "info" {
		t.Errorf("DefaultConfig() logging level = %v, want info", config.Logging.Level)
	}
}