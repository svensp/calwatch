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
- **🔋 Laptop-optimized**: Sleep/wake detection with missed event recovery
- **🎯 Smart notifications**: Different durations for normal vs. missed events
- **⚙️ User-friendly config**: Human-readable durations (no more magic milliseconds)
- **📱 Flexible policies**: Configure how to handle missed events (all/summary/priority/skip)

## Use Case

Perfect for users who:
- Sync calendars via vdirsyncer with Nextcloud/CalDAV
- Use tiling window managers (Hyprland, Sway, i3)
- Want calendar notifications without running a full calendar application
- Prefer lightweight, terminal-focused tools
- **Use laptops** that frequently sleep/hibernate and need reliable event catch-up

## Installation

### Prerequisites

- Go 1.20+ (or use the provided `shell.nix` for NixOS) 
- D-Bus session bus (standard on most Linux desktop environments)
- A CalDAV sync solution like vdirsyncer

### NixOS/Home Manager Installation

Add CalWatch to your Home Manager configuration:

```nix
{ config, pkgs, lib, ... }:
let
  calwatch = pkgs.callPackage (builtins.fetchGit {
    url = "https://github.com/svensp/calwatch";
    ref = "main";
  } + "/default.nix") {};
in
{
  # Install calwatch package
  home.packages = [ calwatch ];

  # CalWatch configuration
  xdg.configFile."calwatch/config.yaml".text = ''
    directories:
      - directory: ~/.calendars/personal
        template: default.tpl
        automatic_alerts:
          - value: 15
            unit: minutes
          - value: 1
            unit: hours

    notification:
      backend: notify-send
      duration:
        type: timed
        value: 5
        unit: seconds
      duration_when_late:
        type: until_dismissed

    wakeup_handling:
      enable: true
      missed_event_policy: all

    logging:
      level: info
  '';

  # CalWatch service
  systemd.user.services.calwatch = {
    Unit = {
      Description = "Calendar event notification daemon";
      After = [ "graphical-session.target" ];
    };
    Service = {
      Type = "simple";
      ExecStart = "${calwatch}/bin/calwatch";
      Restart = "on-failure";
      RestartSec = "5";
      Environment = [
        "PATH=${pkgs.libnotify}/bin:/run/current-system/sw/bin"
      ];
    };
    Install = {
      WantedBy = [ "graphical-session.target" ];
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

### Build from Source

```bash
git clone https://github.com/svensp/calwatch.git
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
  # Normal notification duration
  duration:
    type: timed
    value: 5
    unit: seconds
  # Duration for missed/late notifications
  duration_when_late:
    type: until_dismissed  # Requires user action to dismiss

# Sleep/wake handling for laptop users
wakeup_handling:
  enable: true
  missed_event_policy: all           # all, summary, priority_only, skip
  max_missed_days: 7                 # Limit how far back to process
  summary_threshold: 5               # Show summary if more than N events
  max_catchup_time:
    value: 30
    unit: seconds

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

### 🔋 Laptop Sleep/Wake Handling

CalWatch is optimized for laptop users who frequently sleep/hibernate their machines. When the system wakes up, CalWatch automatically detects the gap and processes any missed events.

#### Missed Event Policies

Configure how CalWatch handles events that were missed during sleep:

```yaml
wakeup_handling:
  missed_event_policy: all  # Choose one of four policies
```

**Policy Options:**

- **`all`** - Show every missed event individually (good for light calendar usage)
- **`summary`** - Group missed events into summary notifications when threshold exceeded
- **`priority_only`** - Only show high-priority missed events (meetings, deadlines, etc.)
- **`skip`** - Clean slate approach, skip all missed events (useful after vacations)

#### Smart Priority Detection

CalWatch automatically classifies events by priority based on:
- **Keywords**: "meeting", "deadline", "urgent", "interview", etc.
- **Attendees**: Events with multiple people are prioritized
- **Work indicators**: Events in work calendars or with work-related keywords
- **Time sensitivity**: Events starting soon get higher priority

#### Notification Duration Types

```yaml
notification:
  # Normal notifications (auto-dismiss)
  duration:
    type: timed
    value: 5
    unit: seconds
    
  # Missed notifications (require user action)
  duration_when_late:
    type: until_dismissed
```

**Duration Types:**
- `timed` - Auto-dismiss after specified time (default behavior)
- `until_dismissed` - Persistent notification requiring user click

**Supported Time Units:**
- `milliseconds` (ms)
- `seconds` (default, most user-friendly) 
- `minutes` (for very long notifications)

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

## 🔧 Troubleshooting

### Missed Events Not Working

If missed events aren't being detected after sleep:

1. **Check wake-up handling is enabled:**
   ```yaml
   wakeup_handling:
     enable: true
   ```

2. **Verify state file location:**
   ```bash
   ls -la ~/.local/state/calwatch/state.json
   ```

3. **Check daemon logs:**
   ```bash
   journalctl --user -f -u calwatch@$(id -u).service
   ```

### Notifications Not Persistent

If missed event notifications aren't staying visible:

1. **Check duration configuration:**
   ```yaml
   notification:
     duration_when_late:
       type: until_dismissed  # Not "timed"
   ```

2. **Verify D-Bus support:**
   ```bash
   # Test D-Bus notifications
   notify-send --expire-time=0 "Test" "This should stay until clicked"
   ```

### Performance Issues After Long Sleep

If CalWatch is slow after extended sleep periods:

1. **Reduce catchup time limit:**
   ```yaml
   wakeup_handling:
     max_catchup_time:
       value: 10    # Reduce from 30 seconds
       unit: seconds
   ```

2. **Limit missed days processed:**
   ```yaml
   wakeup_handling:
     max_missed_days: 3  # Reduce from 7 days
   ```

3. **Use summary policy for many events:**
   ```yaml
   wakeup_handling:
     missed_event_policy: summary
     summary_threshold: 3
   ```

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

- **Config** - YAML configuration with XDG directory support and user-friendly durations
- **Storage** - In-memory event storage with daily indexing and persistent state management
- **Parser** - ICS file parsing using gocal library with timezone awareness
- **Watcher** - File system monitoring via fsnotify/inotify
- **Alerts** - Minute-based alert scheduling with wake-up detection and missed event processing
- **Notifications** - Template rendering and desktop notification delivery with context-aware durations
- **State** - XDG-compliant persistent state tracking for reliable sleep/wake recovery

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

- [ ] Support for ICS event VALARM components (respect event-defined alerts in addition to global configuration)
- [ ] Snooze and dismiss functionality with D-Bus action buttons to silence remaining alerts for specific event occurrences
- [ ] Alert policies for context-aware notifications (e.g., day-long events get 1 week/2 days/1 day alerts instead of minutes-before)

## Support

- **Issues**: Report bugs and feature requests on GitHub
- **Discussions**: Use GitHub Discussions for questions and ideas
- **Documentation**: Check the [design.md](docs/design.md) for technical details

---

**CalWatch** - Simple, effective calendar notifications for Linux desktop environments.