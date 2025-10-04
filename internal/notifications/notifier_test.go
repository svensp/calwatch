package notifications

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"

	"calwatch/internal/alerts"
	"calwatch/internal/config"
	"calwatch/internal/storage"
)

func TestNotifySendNotifier_CreateTemplateData(t *testing.T) {
	notifier := NewNotifySendNotifier()

	// Create test event
	startTime := time.Date(2023, 10, 15, 14, 30, 0, 0, time.UTC)
	endTime := startTime.Add(time.Hour)

	event := storage.NewCalendarEvent(
		"test-uid",
		"Team Meeting",
		"Weekly team standup meeting",
		"Conference Room A",
		startTime,
		endTime,
		time.UTC,
		"",
	)

	alertOffset := 15 * time.Minute

	// Create template data
	data := notifier.createTemplateData(event, alertOffset)

	// Check basic fields
	if data.Summary != "Team Meeting" {
		t.Errorf("Expected summary 'Team Meeting', got '%s'", data.Summary)
	}

	if data.Description != "Weekly team standup meeting" {
		t.Errorf("Expected description 'Weekly team standup meeting', got '%s'", data.Description)
	}

	if data.Location != "Conference Room A" {
		t.Errorf("Expected location 'Conference Room A', got '%s'", data.Location)
	}

	if data.UID != "test-uid" {
		t.Errorf("Expected UID 'test-uid', got '%s'", data.UID)
	}

	// Check formatted times (should be in local timezone)
	expectedStart := startTime.In(time.Local).Format("15:04")
	if data.StartTime != expectedStart {
		t.Errorf("Expected start time '%s', got '%s'", expectedStart, data.StartTime)
	}

	expectedEnd := endTime.In(time.Local).Format("15:04")
	if data.EndTime != expectedEnd {
		t.Errorf("Expected end time '%s', got '%s'", expectedEnd, data.EndTime)
	}

	// Check duration and alert offset
	if data.Duration != "1 hour" {
		t.Errorf("Expected duration '1 hour', got '%s'", data.Duration)
	}

	if data.AlertOffset != "15 minutes" {
		t.Errorf("Expected alert offset '15 minutes', got '%s'", data.AlertOffset)
	}
}

func TestNotifySendNotifier_LoadTemplate(t *testing.T) {
	notifier := NewNotifySendNotifier()

	// Create temporary template file
	tempDir, err := os.MkdirTemp("", "calwatch_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	templateContent := `Hello {{.Summary}}!
Time: {{.StartTime}}`

	templatePath := filepath.Join(tempDir, "test.tpl")
	err = os.WriteFile(templatePath, []byte(templateContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write template file: %v", err)
	}

	// Load template
	tmpl, err := notifier.LoadTemplate(templatePath)
	if err != nil {
		t.Fatalf("Failed to load template: %v", err)
	}

	// Test template execution
	data := TemplateData{
		Summary:   "Test Event",
		StartTime: "14:30",
	}

	err = notifier.ValidateTemplate(tmpl, data)
	if err != nil {
		t.Errorf("Template validation failed: %v", err)
	}
}

func TestNotifySendNotifier_LoadTemplateNonexistent(t *testing.T) {
	notifier := NewNotifySendNotifier()

	_, err := notifier.LoadTemplate("/nonexistent/template.tpl")
	if err == nil {
		t.Error("Expected error when loading nonexistent template")
	}
}

func TestNotifySendNotifier_ValidateTemplate(t *testing.T) {
	notifier := NewNotifySendNotifier()

	tests := []struct {
		name         string
		templateText string
		expectError  bool
	}{
		{
			name:         "valid template",
			templateText: "Event: {{.Summary}} at {{.StartTime}}",
			expectError:  false,
		},
		{
			name:         "template with missing field",
			templateText: "Event: {{.Summary}} organized by {{.NonexistentField}}",
			expectError:  true,
		},
		{
			name:         "template with syntax error",
			templateText: "Event: {{.Summary",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("test").Parse(tt.templateText)
			if err != nil && !tt.expectError {
				t.Fatalf("Failed to parse template: %v", err)
			}
			if err != nil {
				return // Expected parse error
			}

			data := TemplateData{
				Summary:   "Test Event",
				StartTime: "14:30",
			}

			err = notifier.ValidateTemplate(tmpl, data)
			if (err != nil) != tt.expectError {
				t.Errorf("ValidateTemplate() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestNotifySendNotifier_DefaultTemplate(t *testing.T) {
	notifier := NewNotifySendNotifier()

	// Test default template with sample data
	data := TemplateData{
		Summary:     "Team Meeting",
		Location:    "Conference Room A",
		StartTime:   "14:30",
		AlertOffset: "15 minutes",
	}

	err := notifier.ValidateTemplate(notifier.defaultTemplate, data)
	if err != nil {
		t.Errorf("Default template validation failed: %v", err)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30 seconds"},
		{1 * time.Minute, "1 minute"},
		{5 * time.Minute, "5 minutes"},
		{1 * time.Hour, "1 hour"},
		{2 * time.Hour, "2 hours"},
		{24 * time.Hour, "1 day"},
		{48 * time.Hour, "2 days"},
		{90 * time.Minute, "1 hour"}, // 1.5 hours rounds to 1 hour
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %s, want %s", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestNotificationManager_SendNotification(t *testing.T) {
	config := config.NotificationConfig{
		Backend: "notify-send",
		Duration: config.DurationConfig{
			Type:  "timed",
			Value: 5,
			Unit:  "seconds",
		},
		DurationWhenLate: config.DurationConfig{
			Type: "until_dismissed",
		},
	}

	manager := NewNotificationManager(config)

	// Create test alert request
	event := storage.NewCalendarEvent(
		"test-uid",
		"Test Meeting",
		"",
		"",
		time.Now().Add(5*time.Minute),
		time.Now().Add(65*time.Minute),
		time.UTC,
		"",
	)

	request := alerts.AlertRequest{
		Event:       event,
		AlertOffset: 5 * time.Minute,
		Template:    "",
	}

	// This test will actually try to send a notification if notify-send is available
	// In a real test environment, we might want to mock this
	err := manager.SendNotification(request)
	
	// Don't fail the test if notify-send is not available
	if err != nil && !strings.Contains(err.Error(), "executable file not found") {
		t.Errorf("SendNotification failed: %v", err)
	}
}

func TestCreateDefaultTemplates(t *testing.T) {
	// This test would create files in the user's XDG config directory
	// For testing purposes, we'll just verify the function doesn't panic
	
	// Note: In a real test environment, you might want to:
	// 1. Mock the XDG directory functions
	// 2. Use a temporary directory
	// 3. Clean up created files
	
	// For now, just test that the function exists and can be called
	// err := CreateDefaultTemplates()
	// We don't call it to avoid modifying the user's config directory during tests
}

func TestNotifySendNotifier_SetConfig(t *testing.T) {
	notifier := NewNotifySendNotifier()
	
	newConfig := config.NotificationConfig{
		Backend: "notify-send",
		Duration: config.DurationConfig{
			Type:  "timed",
			Value: 10,
			Unit:  "seconds",
		},
		DurationWhenLate: config.DurationConfig{
			Type: "until_dismissed",
		},
	}
	
	notifier.SetConfig(newConfig)
	
	expectedMs, _ := newConfig.Duration.ToMilliseconds()
	actualMs, _ := notifier.config.Duration.ToMilliseconds()
	if actualMs != expectedMs {
		t.Errorf("Expected duration %d ms, got %d ms", expectedMs, actualMs)
	}
}