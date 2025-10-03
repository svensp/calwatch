# CalWatch

A lightweight CalDAV directory watcher daemon for Linux desktop environments. CalWatch monitors your local CalDAV directories (synced via vdirsyncer) and sends desktop notifications for upcoming calendar events.

## Disclaimer

I'm a programmer, just not for the go language. This daemon was built by providing claude with the use case and the
architecture I'd build in the languages I'm fluent in, see [design.md](./docs/design.md) then setting claude to work
see [progress.md](./docs/progress.md)

I have not yet read over it to look for glaring logic errors, just verified that the basic functionality is there. Use
at your own discretion.

## Features

- **Real-time monitoring** of CalDAV directories using inotify
- **Proper ICS parsing** with recurring event support via gocal
- **Configurable alerts** with multiple time offsets (minutes, hours, days)
- **Template-based notifications** with rich formatting options
- **Desktop integration** via D-Bus notifications (no external dependencies)
- **XDG compliant** configuration and template management
- **Systemd integration** for background daemon operation
- **No database dependency** - direct ICS file parsing
- **Memory efficient** with daily event indexing

## Use Case

Perfect for users who:
- Sync calendars via vdirsyncer with Nextcloud/CalDAV
- Use tiling window managers (Hyprland, Sway, i3)
- Want calendar notifications without running a full calendar application
- Prefer lightweight, terminal-focused tools

## Installation

### Prerequisites

- Go 1.20+ (or use the provided `shell.nix` for NixOS) 
- D-Bus session bus (standard on most Linux desktop environments)
- A CalDAV sync solution like vdirsyncer

### NixOS Installation (Recommended)

#### Using Flakes

```bash
# Install directly from the repository
nix profile install github:yourusername/calwatch

# Or build locally
git clone https://github.com/yourusername/calwatch.git
cd calwatch
nix build
sudo cp result/bin/calwatch /usr/local/bin/
```

#### NixOS System Configuration

Add to your `configuration.nix`:

```nix
{
  services.calwatch = {
    enable = true;
    user = "yourusername";
    settings = {
      directories = [
        {
          directory = "/home/yourusername/.calendars/personal";
          template = "detailed.tpl";
          automatic_alerts = [
            { value = 15; unit = "minutes"; }
            { value = 1; unit = "hours"; }
          ];
        }
      ];
      notification = {
        backend = "dbus";  # Default, no external dependencies
        duration = 5000;
      };
      logging = {
        level = "info";
      };
    };
  };
}
```

#### Development Environment

```bash
# Enter development shell with all dependencies
nix develop

# Or use legacy shell
nix-shell
```

### Build from Source (Non-NixOS)

```bash
git clone https://github.com/yourusername/calwatch.git
cd calwatch

# Build
go build ./cmd/calwatch

# Install (optional)
sudo cp calwatch /usr/local/bin/
```

## Quick Start

1. **Initialize configuration:**
   ```bash
   ./calwatch init
   ```

2. **Edit configuration** to point to your calendar directories:
   ```bash
   vim ~/.config/calwatch/config.yaml
   ```

3. **Run the daemon:**
   ```bash
   ./calwatch
   ```

## Configuration

### Basic Configuration

Edit `~/.config/calwatch/config.yaml`:

```yaml
directories:
  - directory: ~/.calendars/personal
    template: detailed.tpl
    automatic_alerts:
      - value: 15
        unit: minutes
      - value: 1
        unit: hours

  - directory: ~/.calendars/work
    template: minimal.tpl
    automatic_alerts:
      - value: 5
        unit: minutes

notification:
  backend: notify-send
  duration: 5000

logging:
  level: info
```

### Notification Templates

CalWatch includes several built-in templates:

- **default.tpl** - Simple event summary with time
- **detailed.tpl** - Full event details with emojis
- **minimal.tpl** - Just event name and time
- **family.tpl** - Family-friendly format with emoji

Create custom templates in `~/.config/calwatch/templates/`:

```
📅 {{.Summary}}
🕐 {{.StartTime}} - {{.EndTime}} ({{.Duration}})
{{if .Location}}📍 {{.Location}}{{end}}
{{if .Description}}📝 {{.Description}}{{end}}

⏰ {{.AlertOffset}} warning
```

### Available Template Variables

- `{{.Summary}}` - Event title
- `{{.Description}}` - Event description  
- `{{.Location}}` - Event location
- `{{.StartTime}}` - Start time (HH:MM format)
- `{{.EndTime}}` - End time (HH:MM format)
- `{{.Duration}}` - Event duration (human readable)
- `{{.AlertOffset}}` - Alert timing (e.g. "15 minutes")
- `{{.UID}}` - Event unique identifier

## Systemd Integration

### User Service (Recommended)

1. **Install service file:**
   ```bash
   sudo cp calwatch.service /etc/systemd/system/calwatch@.service
   ```

2. **Enable and start:**
   ```bash
   systemctl --user enable calwatch@$(id -u).service
   systemctl --user start calwatch@$(id -u).service
   ```

3. **Check status:**
   ```bash
   systemctl --user status calwatch@$(id -u).service
   journalctl --user -f -u calwatch@$(id -u).service
   ```

## Integration with vdirsyncer

CalWatch works perfectly with vdirsyncer. Example vdirsyncer configuration:

```ini
[general]
status_path = "~/.vdirsyncer/status/"

[pair my_calendar]
a = "my_calendar_local"
b = "my_calendar_remote"
collections = ["from a", "from b"]

[storage my_calendar_local]
type = "filesystem"
path = "~/.calendars/personal"
fileext = ".ics"

[storage my_calendar_remote]
type = "caldav"
url = "https://your-nextcloud.com/remote.php/dav/"
username = "your-username"
password = "your-password"
```

Then configure CalWatch to watch `~/.calendars/personal`.

## Command Line Interface

```bash
calwatch                # Start the daemon
calwatch init           # Create default configuration and templates  
calwatch help           # Show usage information
calwatch status         # Show daemon status (planned)
calwatch stop           # Stop the daemon (planned)
```

## Architecture

CalWatch follows a clean, modular architecture:

- **Config** - YAML configuration with XDG directory support
- **Storage** - In-memory event storage with daily indexing
- **Parser** - ICS file parsing using gocal library
- **Watcher** - File system monitoring via fsnotify/inotify
- **Alerts** - Minute-based alert scheduling logic
- **Notifications** - Template rendering and desktop notification delivery

See [design.md](docs/design.md) for detailed architecture documentation.

## Waybar Integration

Display upcoming events in your status bar by creating a custom Waybar module:

```json
"custom/calendar": {
    "exec": "khal list now 24h --format '{start-time} {title}' | head -1",
    "interval": 300,
    "tooltip": true,
    "tooltip-format": "Upcoming Events"
}
```

## Development

### Prerequisites

- Go 1.20+
- For NixOS: Use the provided `shell.nix`

### Running Tests

```bash
go test ./...
```

### Project Structure

```
calwatch/
├── cmd/calwatch/           # Main application entry point
├── internal/               # Core packages
│   ├── config/            # Configuration management
│   ├── storage/           # Event storage and indexing
│   ├── parser/            # ICS file parsing
│   ├── watcher/           # File system monitoring  
│   ├── alerts/            # Alert scheduling
│   └── notifications/     # Desktop notifications
├── templates/             # Default notification templates
└── docs/                  # Documentation
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass (`go test ./...`)
6. Commit your changes (`git commit -m 'Add amazing feature'`)
7. Push to the branch (`git push origin feature/amazing-feature`)
8. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [gocal](https://github.com/apognu/gocal) - Excellent ICS parsing library
- [fsnotify](https://github.com/fsnotify/fsnotify) - Cross-platform file system notifications
- [vdirsyncer](https://github.com/pimutils/vdirsyncer) - CalDAV synchronization
- The CalDAV/CardDAV community for maintaining open standards

## Roadmap

- [ ] Enhanced RRULE support for complex recurring patterns
- [ ] Multiple notification backends (mako, dunst direct API)
- [ ] Snooze and dismiss functionality
- [ ] Web interface for configuration
- [ ] Integration with other calendar applications
- [ ] Mobile notifications via push services

## Support

- **Issues**: Report bugs and feature requests on GitHub
- **Discussions**: Use GitHub Discussions for questions and ideas
- **Documentation**: Check the [design.md](docs/design.md) for technical details

---

**CalWatch** - Simple, effective calendar notifications for Linux desktop environments.