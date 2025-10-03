# CalWatch - CalDAV Directory Watcher Daemon

## Overview

CalWatch is a lightweight daemon that monitors CalDAV directories for changes and sends desktop notifications for upcoming events. It directly parses ICS files without using an intermediate database, handles recurring events properly, and integrates seamlessly with Linux desktop environments.

## Architecture

### Core Components

```
calwatch/
├── cmd/calwatch/main.go           # Entry point and daemon orchestration
├── internal/
│   ├── config/config.go           # YAML configuration with XDG support
│   ├── storage/events.go          # In-memory event storage and indexing
│   ├── parser/caldav.go           # ICS parsing with recurring event support
│   ├── watcher/inotify.go         # File system change monitoring
│   ├── alerts/scheduler.go        # Alert timing and scheduling logic
│   └── notifications/notifier.go  # Template rendering and notification delivery
├── templates/                     # Default notification templates
├── design.md                      # This architecture document
└── progress.md                    # Implementation progress tracking
```

## Component Design

### 1. Config Package

**Purpose**: Load and validate YAML configuration from XDG-compliant locations.

**Configuration Structure**:
```yaml
directories:
  - directory: ~/.calendars/family
    template: family.tpl
    automatic_alerts:
      - value: 5
        unit: minutes
      - value: 1  
        unit: hours

notification:
  backend: notify-send
  duration: 5000

logging:
  level: info
  # Default output to stderr for systemd/journald
```

**Key Features**:
- XDG Base Directory Specification compliance
- Per-directory alert configuration
- Template customization per calendar
- Validation of time units (minutes, hours, days)
- Environment variable expansion in paths

### 2. Storage Package

**Purpose**: Manage in-memory event storage with efficient daily indexing.

**Core Interface**:
```go
type EventStorage interface {
    UpsertEvent(event Event) error
    DeleteEvent(uid string) error
    GetEventsForDay(date time.Time) []Event
    GetUpcomingEvents(from time.Time, duration time.Duration) []Event
    RegenerateIndex(date time.Time) error
}

type Event interface {
    GetUID() string
    GetSummary() string
    GetDescription() string
    GetLocation() string
    GetStartTime() time.Time
    GetEndTime() time.Time
    GetTimezone() *time.Location
    OccursOn(date time.Time) bool
    NextOccurrence(after time.Time) *time.Time
    ShouldAlert(now time.Time, alertOffset time.Duration) bool
    GetAlertState(alertOffset time.Duration) AlertState
    SetAlertState(alertOffset time.Duration, state AlertState)
}

type AlertState int
const (
    AlertPending AlertState = iota
    AlertSent
    AlertSnoozed
)
```

**Implementation Details**:
- Daily index for fast "today's events" lookup
- Rolling 7-day window for recurring event expansion
- Alert state tracking to prevent duplicate notifications
- Memory-efficient storage with event deduplication
- Thread-safe operations for concurrent access

### 3. Parser Package

**Purpose**: Parse ICS files and convert them to internal Event objects.

**Key Features**:
- Uses `github.com/apognu/gocal` for robust ICS parsing
- Proper RRULE (recurring event) expansion
- Timezone handling via TZID parsing
- EXDATE and RECURRENCE-ID override support
- Incremental parsing (only process changed files)
- Error recovery for malformed ICS files

**Interface**:
```go
type CalDAVParser interface {
    ParseFile(filePath string) ([]Event, error)
    ParseDirectory(dirPath string) ([]Event, error)
    ValidateICS(data []byte) error
}
```

### 4. Watcher Package

**Purpose**: Monitor CalDAV directories for file system changes using inotify.

**Monitored Events**:
- File creation (new events)
- File modification (event updates)
- File deletion (event removal)
- Directory changes (calendar addition/removal)

**Interface**:
```go
type FileWatcher interface {
    WatchDirectory(path string, callback FileChangeCallback) error
    Stop() error
}

type FileChangeCallback func(event FileChangeEvent)

type FileChangeEvent struct {
    Path      string
    Operation FileOperation
    IsDir     bool
}

type FileOperation int
const (
    FileCreated FileOperation = iota
    FileModified
    FileDeleted
)
```

### 5. Alerts Package

**Purpose**: Determine when to send notifications based on event timing and alert configuration.

**Core Logic**:
- Runs every minute at hh:mm:00
- Queries storage for today's events
- Checks each event against configured alert offsets
- Prevents duplicate alerts using alert state tracking
- Handles timezone conversions for accurate timing

**Interface**:
```go
type AlertScheduler interface {
    CheckAlerts() []AlertRequest
    ScheduleNextCheck() time.Duration
}

type AlertRequest struct {
    Event       Event
    AlertOffset time.Duration
    Template    string
}
```

### 6. Notifications Package

**Purpose**: Render notifications using templates and deliver via desktop notification system.

**Template System**:
- Go's `text/template` for flexible formatting
- Per-calendar template customization
- Error recovery with fallback formatting
- Rich event data available to templates

**Template Variables**:
```go
type TemplateData struct {
    Summary     string
    Description string
    Location    string
    StartTime   string    // Formatted for local timezone
    EndTime     string
    Duration    string
    Organizer   string
    Attendees   []string
    AlertOffset string    // "5 minutes", "1 hour"
}
```

**Error Handling**:
When template rendering fails, send a notification like:
```
Event: "Team Meeting" at 14:00
Template Error: family.tpl failed - undefined variable 'location'
```

**Interface**:
```go
type Notifier interface {
    SendNotification(request AlertRequest) error
    LoadTemplate(path string) (*template.Template, error)
    ValidateTemplate(tmpl *template.Template, data TemplateData) error
}
```

## Data Flow

1. **Startup**: Parse configuration, scan all CalDAV directories, populate event storage
2. **File Watching**: inotify triggers reparse of changed ICS files
3. **Minute Timer**: Alert scheduler checks for upcoming events every minute
4. **Notification**: Render templates and send desktop notifications
5. **Daily Rollover**: At midnight, regenerate daily index for new date

## Key Design Decisions

### No Database Dependency
- Events stored in memory for fast access
- Direct ICS file parsing eliminates sync issues
- Simple deployment without database setup

### Per-Event Timezone Handling
- Each event carries its own timezone from TZID
- No calendar-level timezone overrides needed
- Proper handling of multi-timezone calendars

### Template-Based Notifications
- Flexible formatting per calendar type
- Rich event data available to templates
- Graceful degradation on template errors

### Alert State Management
- Prevents duplicate notifications
- Supports snooze functionality (future enhancement)
- Per-event, per-alert-offset state tracking

### Minute-Level Precision
- Calendar events typically start at hh:mm:00
- Avoids unnecessary sub-minute complexity
- Efficient resource usage

## Error Handling

### File System Errors
- Continue monitoring other directories if one fails
- Log errors to stderr for systemd/journald capture
- Graceful degradation when files become unreadable

### Parsing Errors
- Skip malformed ICS files with detailed logging
- Continue processing other events in the same file
- Validate ICS structure before parsing

### Template Errors
- Send fallback notification with error details
- Load default template if custom template fails
- Provide actionable error messages to users

### Notification Delivery Errors
- Retry mechanism for transient failures
- Log delivery failures for debugging
- Continue processing other pending alerts

## Dependencies

### External Libraries
- `github.com/apognu/gocal` - ICS parsing with RRULE expansion
- `github.com/fsnotify/fsnotify` - Cross-platform file system notifications
- `gopkg.in/yaml.v3` - YAML configuration parsing
- `github.com/adrg/xdg` - XDG Base Directory Specification

### System Dependencies
- `notify-send` (libnotify) for desktop notifications
- inotify support in Linux kernel
- systemd for daemon management (optional)

## Security Considerations

### File System Access
- Only monitor explicitly configured directories
- Validate file paths to prevent directory traversal
- Respect file permissions and access controls

### Template Execution
- Use `text/template` (not `html/template`) to avoid XSS concerns
- Sanitize template data to prevent injection attacks
- Limit template complexity to prevent DoS

### Process Isolation
- Run as non-privileged user
- Limit resource usage (memory, CPU)
- Graceful shutdown on system signals

## Performance Characteristics

### Memory Usage
- Events stored in memory for fast access
- Bounded by number of events in configured time window
- Efficient recurring event expansion

### CPU Usage
- Minute-level timer minimizes CPU wake-ups
- Incremental file parsing on changes only
- Efficient inotify-based file watching

### I/O Patterns
- Read-only access to ICS files
- Minimal disk I/O after initial scan
- Batch processing of file changes

## Future Enhancements

### Notification Features
- Multiple notification backends (mako, dunst direct)
- Rich notifications with actions (snooze, dismiss)
- Sound and visual notification customization

### Template System
- HTML template support for rich notifications
- Template inheritance and includes
- Template validation and testing tools

### Configuration
- Dynamic configuration reloading
- Web-based configuration interface
- Configuration validation and migration

### Integration
- Waybar module for upcoming events display
- Calendar application integration
- Email notification backend

This design provides a solid foundation for a reliable, efficient CalDAV notification daemon that integrates well with modern Linux desktop environments while maintaining simplicity and avoiding common pitfalls of calendar synchronization systems.