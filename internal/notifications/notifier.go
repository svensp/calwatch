package notifications

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/adrg/xdg"
	"calwatch/internal/alerts"
	"calwatch/internal/config"
	"calwatch/internal/storage"
)

// TemplateData represents the data available to notification templates
type TemplateData struct {
	Summary     string
	Description string
	Location    string
	StartTime   string
	EndTime     string
	Duration    string
	Organizer   string
	Attendees   []string
	AlertOffset string
	UID         string
}

// Notifier handles sending notifications
type Notifier interface {
	SendNotification(request alerts.AlertRequest) error
	LoadTemplate(path string) (*template.Template, error)
	ValidateTemplate(tmpl *template.Template, data TemplateData) error
	SetConfig(config config.NotificationConfig)
}

// NotifySendNotifier implements Notifier using notify-send
type NotifySendNotifier struct {
	config        config.NotificationConfig
	templates     map[string]*template.Template
	defaultTemplate *template.Template
}

// NewNotifySendNotifier creates a new notify-send based notifier
func NewNotifySendNotifier() *NotifySendNotifier {
	notifier := &NotifySendNotifier{
		templates: make(map[string]*template.Template),
		config: config.NotificationConfig{
			Backend:  "notify-send",
			Duration: 5000,
		},
	}

	// Load default template
	notifier.defaultTemplate = notifier.createDefaultTemplate()

	return notifier
}

// SetConfig sets the notification configuration
func (n *NotifySendNotifier) SetConfig(config config.NotificationConfig) {
	n.config = config
}

// SendNotification sends a notification for an alert request
func (n *NotifySendNotifier) SendNotification(request alerts.AlertRequest) error {
	// Create template data from the event
	data := n.createTemplateData(request.Event, request.AlertOffset)

	// Get the template to use
	tmpl, err := n.getTemplate(request.Template)
	if err != nil {
		// If template loading fails, send error notification and fall back to default
		n.sendErrorNotification(request, err)
		tmpl = n.defaultTemplate
	}

	// Render the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		// If template execution fails, send error notification and fall back to default
		n.sendErrorNotification(request, fmt.Errorf("template execution failed: %w", err))
		
		// Use default template
		buf.Reset()
		if err := n.defaultTemplate.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to execute default template: %w", err)
		}
	}

	// Send the notification
	return n.sendDesktopNotification(data.Summary, buf.String())
}

// createTemplateData creates template data from an event
func (n *NotifySendNotifier) createTemplateData(event storage.Event, alertOffset time.Duration) TemplateData {
	startTime := event.GetStartTime()
	endTime := event.GetEndTime()
	duration := endTime.Sub(startTime)

	// Format times in local timezone
	localStart := startTime.In(time.Local)
	localEnd := endTime.In(time.Local)

	return TemplateData{
		Summary:     event.GetSummary(),
		Description: event.GetDescription(),
		Location:    event.GetLocation(),
		StartTime:   localStart.Format("15:04"),
		EndTime:     localEnd.Format("15:04"),
		Duration:    formatDuration(duration),
		AlertOffset: formatDuration(alertOffset),
		UID:         event.GetUID(),
		// TODO: Add organizer and attendees when available in storage.Event
		Organizer:   "",
		Attendees:   []string{},
	}
}

// getTemplate retrieves or loads a template by name
func (n *NotifySendNotifier) getTemplate(templateName string) (*template.Template, error) {
	// If empty template name, use default
	if templateName == "" {
		return n.defaultTemplate, nil
	}

	// Check if template is already loaded
	if tmpl, exists := n.templates[templateName]; exists {
		return tmpl, nil
	}

	// Try to load template from XDG config directories
	templatePath, err := xdg.SearchConfigFile(filepath.Join("calwatch", "templates", templateName))
	if err != nil {
		// Try to load from the default templates directory
		templatePath = filepath.Join("templates", templateName)
	}

	tmpl, err := n.LoadTemplate(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load template %s: %w", templateName, err)
	}

	// Cache the template
	n.templates[templateName] = tmpl
	return tmpl, nil
}

// LoadTemplate loads a template from a file path
func (n *NotifySendNotifier) LoadTemplate(path string) (*template.Template, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("template file does not exist: %s", path)
	}

	// Read template content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file %s: %w", path, err)
	}

	// Parse template
	tmpl, err := template.New(filepath.Base(path)).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s: %w", path, err)
	}

	return tmpl, nil
}

// ValidateTemplate validates a template with sample data
func (n *NotifySendNotifier) ValidateTemplate(tmpl *template.Template, data TemplateData) error {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}
	return nil
}

// createDefaultTemplate creates the built-in default template
func (n *NotifySendNotifier) createDefaultTemplate() *template.Template {
	defaultTemplateText := `{{.Summary}}{{if .Location}} at {{.Location}}{{end}}
Starts: {{.StartTime}} ({{.AlertOffset}} warning)`

	tmpl, err := template.New("default").Parse(defaultTemplateText)
	if err != nil {
		// This should never happen with our static template
		panic(fmt.Sprintf("Failed to create default template: %v", err))
	}

	return tmpl
}

// sendErrorNotification sends a notification about template errors
func (n *NotifySendNotifier) sendErrorNotification(request alerts.AlertRequest, err error) {
	title := "Calendar Notification Error"
	message := fmt.Sprintf("Event: %s at %s\nTemplate Error: %s\nTemplate: %s",
		request.Event.GetSummary(),
		request.Event.GetStartTime().Format("15:04"),
		err.Error(),
		request.Template,
	)

	// Use notify-send directly to avoid infinite recursion
	n.sendDesktopNotification(title, message)
}

// sendDesktopNotification sends a notification using notify-send
func (n *NotifySendNotifier) sendDesktopNotification(title, message string) error {
	// Prepare notify-send command
	args := []string{
		"notify-send",
		"--app-name=calwatch",
		fmt.Sprintf("--expire-time=%d", n.config.Duration),
	}

	// Add title and message
	args = append(args, title, message)

	// Execute notify-send
	cmd := exec.Command(args[0], args[1:]...)
	
	// Set environment variables for proper notification delivery
	cmd.Env = append(os.Environ(),
		"DISPLAY=:0",
		fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/%d", os.Getuid()),
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	return nil
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		if minutes == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", minutes)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

// NotificationManager coordinates multiple notifiers
type NotificationManager struct {
	notifiers []Notifier
	config    config.NotificationConfig
}

// NewNotificationManager creates a new notification manager
func NewNotificationManager(config config.NotificationConfig) *NotificationManager {
	manager := &NotificationManager{
		config:    config,
		notifiers: make([]Notifier, 0),
	}

	// Add default notifier based on backend
	switch strings.ToLower(config.Backend) {
	case "notify-send", "":
		notifier := NewNotifySendNotifier()
		notifier.SetConfig(config)
		manager.AddNotifier(notifier)
	default:
		// Fallback to notify-send
		notifier := NewNotifySendNotifier()
		notifier.SetConfig(config)
		manager.AddNotifier(notifier)
	}

	return manager
}

// AddNotifier adds a notifier to the manager
func (nm *NotificationManager) AddNotifier(notifier Notifier) {
	nm.notifiers = append(nm.notifiers, notifier)
}

// SendNotification sends a notification using all configured notifiers
func (nm *NotificationManager) SendNotification(request alerts.AlertRequest) error {
	var lastError error

	for _, notifier := range nm.notifiers {
		if err := notifier.SendNotification(request); err != nil {
			lastError = err
			// Log error but continue with other notifiers
			fmt.Fprintf(os.Stderr, "Notification failed: %v\n", err)
		}
	}

	return lastError
}

// CreateDefaultTemplates creates default template files in the user's config directory
func CreateDefaultTemplates() error {
	templatesDir, err := xdg.ConfigFile("calwatch/templates")
	if err != nil {
		return fmt.Errorf("failed to get templates directory: %w", err)
	}

	// Create templates directory
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		return fmt.Errorf("failed to create templates directory: %w", err)
	}

	// Default template content
	templates := map[string]string{
		"default.tpl": `{{.Summary}}{{if .Location}} at {{.Location}}{{end}}
Starts: {{.StartTime}} ({{.AlertOffset}} warning)`,

		"detailed.tpl": `📅 {{.Summary}}
🕐 {{.StartTime}} - {{.EndTime}} ({{.Duration}}){{if .Location}}
📍 {{.Location}}{{end}}{{if .Description}}
📝 {{.Description}}{{end}}

⏰ {{.AlertOffset}} warning`,

		"minimal.tpl": `{{.Summary}} at {{.StartTime}}`,

		"family.tpl": `👨‍👩‍👧‍👦 {{.Summary}}{{if .Location}} at {{.Location}}{{end}}
Starts in {{.AlertOffset}}`,
	}

	for filename, content := range templates {
		templatePath := filepath.Join(templatesDir, filename)
		
		// Only create if doesn't exist
		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			if err := os.WriteFile(templatePath, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to create template %s: %w", filename, err)
			}
		}
	}

	return nil
}