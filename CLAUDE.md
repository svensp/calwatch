# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Building and Running
- `go build ./cmd/calwatch` - Build the calwatch binary
- `./calwatch` - Run the daemon
- `./calwatch init` - Create default configuration and templates
- `./calwatch help` - Show command line help

### Testing
- `go test ./...` - Run all tests
- `go test ./internal/config` - Test specific package

### Development Environment
- `nix-shell` - Enter development environment (NixOS users)
  - Provides: go, gopls, go-tools, golangci-lint, delve

### Linting
- `golangci-lint run` - Run linter (available in nix-shell)

## Architecture Overview

CalWatch is a CalDAV directory watcher daemon that monitors local calendar directories and sends desktop notifications for upcoming events. It's designed for users who sync calendars via vdirsyncer.

### Core Components

```
internal/
├── config/     - YAML configuration with XDG Base Directory support
├── storage/    - In-memory event storage with daily indexing  
├── parser/     - ICS file parsing using gocal library
├── watcher/    - File system monitoring via fsnotify/inotify
├── alerts/     - Minute-based alert scheduling logic
└── notifications/ - Template rendering and desktop notifications
```

### Key Design Patterns

- **No Database**: Events stored in memory, parsed directly from ICS files
- **Component Interfaces**: All major components implement interfaces for testability
- **XDG Compliance**: Configuration and templates stored in XDG directories
- **Template-Based Notifications**: Go text/template for flexible formatting
- **Daily Indexing**: Efficient "today's events" lookup with rolling 7-day window

### Data Flow
1. Parse configuration from `~/.config/calwatch/config.yaml`
2. Initial scan of configured CalDAV directories
3. File watching via inotify for real-time updates
4. Minute-based timer checks for upcoming alerts
5. Template rendering and desktop notification delivery

## Configuration

- Default config location: `~/.config/calwatch/config.yaml`
- Templates directory: `~/.config/calwatch/templates/`
- Example config: `config.example.yaml`

### Key Configuration Elements
- `directories[]` - CalDAV paths to monitor, with per-directory templates and alert timings
- `notification.backend` - Currently only "notify-send" supported
- `logging.level` - debug, info, warn, error

## Testing Approach

All packages have comprehensive unit tests. When adding new functionality:

1. **Test-Driven Development**: Write tests first when implementing new features
2. **Interface-Based Testing**: Mock interfaces for isolated unit testing
3. **Error Handling**: Test error conditions and edge cases
4. **Time-Based Logic**: Use deterministic time sources in tests

### Test Structure
- Each package has `*_test.go` files
- Mock implementations for external dependencies
- Table-driven tests for comprehensive coverage

## Dependencies

### Core Libraries
- `github.com/apognu/gocal` - ICS parsing with RRULE expansion
- `github.com/fsnotify/fsnotify` - Cross-platform file system notifications
- `gopkg.in/yaml.v3` - YAML configuration parsing  
- `github.com/adrg/xdg` - XDG Base Directory Specification

### System Requirements
- `notify-send` (libnotify) for desktop notifications
- Linux with inotify support
- Go 1.20+

## Important Implementation Notes

### Event Storage
- Events are stored in memory for performance
- Daily index provides O(1) lookup for "today's events"
- Alert state tracking prevents duplicate notifications
- Thread-safe operations for concurrent access

### File Watching
- Monitors CREATE, MODIFY, DELETE operations
- Handles both individual ICS files and directory changes
- Graceful error handling for permission issues

### Alert Logic
- Runs every minute at hh:mm:00 for consistency
- Supports multiple alert offsets per directory (5min, 1hr, etc.)
- Timezone-aware calculations using event TZID

### Template System
- Uses Go's `text/template` (not `html/template`)
- Fallback to simple format if template rendering fails
- Rich event data available: Summary, Description, Location, StartTime, etc.

## Deployment

### Systemd Integration
- Service file: `calwatch.service`
- User service recommended: `systemctl --user enable calwatch@$(id -u).service`
- Proper environment variables for desktop notifications
- Security sandboxing with minimal permissions

### Integration with vdirsyncer
- Monitor directories synced by vdirsyncer (e.g., `~/.calendars/personal`)
- No direct integration needed - file watching handles sync updates
- Supports multiple calendar directories with different templates

## Common Development Tasks

### Adding New Alert Backends
1. Implement `notifications.NotificationBackend` interface
2. Add backend selection logic in `notifications.NewNotificationManager()`
3. Update configuration validation in `config` package

### Adding Template Variables
1. Extend `notifications.TemplateData` struct
2. Update template rendering in `notifications.renderTemplate()`
3. Document new variables in templates and README

### Enhancing Event Storage
1. Modify `storage.Event` interface for new fields
2. Update `storage.EventStorage` implementations
3. Ensure thread-safety for concurrent access

## Security Considerations

- Runs as non-privileged user
- Read-only access to calendar directories
- Template execution uses `text/template` to avoid XSS
- Systemd sandboxing limits file system access

## Documentation Standards

### Improvement Documentation

When working on improvements to CalWatch, document each improvement in `docs/improvement-XXX.md` where XXX is a zero-padded number (001, 002, etc.).

**Improvement file structure**:
- **Context**: Background and motivation for the improvement
- **Conversation Summary**: Key points from the discussion
- **Implementation**: What was changed and how
- **Files Modified**: List of files affected
- **Git Commit**: Commit hash and message
- **Outcome**: Result and impact
- **Next Steps**: Follow-up tasks or related improvements

This pattern helps track the evolution of the project and provides context for future development decisions.

**Important**: Every improvement implementation MUST include updating the README.md file to document the new functionality for users.

## Version Management

### Semantic Versioning Requirements

All improvements MUST update the version number in `default.nix` following semantic versioning rules:

- **Major version** (X.0.0): Breaking changes that require user action (configuration format changes, API breaking changes)
- **Minor version** (X.Y.0): New features that are backward compatible (new functionality, enhancements)
- **Patch version** (X.Y.Z): Bug fixes and internal improvements that don't change external behavior

**Pre-1.0.0 Exception**: Since CalWatch is pre-1.0.0, breaking changes should increment the minor version instead of major version (0.X.0).

### Version Release Process

When creating a new version, follow these steps in order:

1. **Update version in `default.nix`** - Change the version number according to semver rules
2. **Commit the changes** - Include all code changes and the version bump in the commit
3. **Create and push git tag** - Tag the commit with the version number (e.g., `0.2.1`)
4. **Push both commit and tag** - Ensure both the commit and tag are pushed to the remote repository

This ensures proper version tracking and allows users to reference specific versions via git tags.