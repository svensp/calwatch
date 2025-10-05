# Improvement 004: ICS VALARM Component Support with Unified Alert Architecture

## Context

CalWatch currently only supports global `automatic_alerts` configuration for all events, ignoring VALARM components defined within ICS files. This limits flexibility since calendar applications often embed event-specific alert preferences directly in the ICS data.

Additionally, there are architectural issues with the current alert handling:

1. **Missing VALARM Support**: ICS VALARM components are completely ignored during parsing
2. **Poor Alert Abstraction**: `automatic_alerts` config and potential VALARM data use different representations
3. **Incorrect Daily Indexing**: `OccursOn()` only considers when events happen, not when their alerts fire - causing events to be missing from daily indices on alert days
4. **Mixed Responsibilities**: `OccurrencesWithin()` receives raw config objects instead of processed Alert objects

## Problem Analysis

### Current Issues

1. **VALARM Ignored**: Despite ICS files containing VALARM components with event-specific alert preferences, these are completely ignored
2. **Daily Index Bug**: An event happening tomorrow with a 1-day warning should appear in today's daily index, but `OccursOn()` only checks the actual event date
3. **Architecture Inconsistency**: Config-based alerts and potential VALARM alerts have different data models
4. **Separation of Concerns Violation**: Event logic receives config objects instead of processed Alert objects

### VALARM Component Structure

ICS VALARM components can specify:
```ics
BEGIN:VALARM
ACTION:DISPLAY
DESCRIPTION:Reminder
TRIGGER:-PT15M          # 15 minutes before
END:VALARM

BEGIN:VALARM
ACTION:DISPLAY
DESCRIPTION:Final Warning
TRIGGER:-PT5M           # 5 minutes before
END:VALARM
```

TRIGGER formats:
- **Duration**: `-PT15M` (15 minutes before), `-P1D` (1 day before)
- **Absolute**: `19980101T050000Z` (specific time)
- **Related**: `TRIGGER;RELATED=END:-PT15M` (relative to event end)

## Proposed Solution

### 1. Unified Alert Architecture

Create a unified `Alert` struct that can represent both config-based and VALARM-based alerts:

```go
type Alert struct {
    Offset      time.Duration // How far before event to trigger (e.g., 15 minutes)
    Important   bool          // Whether this alert should use critical urgency
    Source      AlertSource   // Whether from config or VALARM
    Description string        // VALARM description or generated description
    Action      AlertAction   // DISPLAY, EMAIL, AUDIO (for future extensibility)
}

type AlertSource int
const (
    AlertSourceConfig AlertSource = iota  // From automatic_alerts config
    AlertSourceVALARM                     // From ICS VALARM component
)

type AlertAction int
const (
    AlertActionDisplay AlertAction = iota // Display notification (only supported for now)
    AlertActionEmail                      // Email notification (future)
    AlertActionAudio                      // Audio notification (future)
)
```

### 2. Enhanced Parser with VALARM Support

Extend the ICS parser to extract VALARM components:

```go
// In parser.go - extend convertGocalEvent
func (p *GocalParser) parseVALARMs(gocalEvent gocal.Event) ([]Alert, error) {
    var alerts []Alert
    
    // Check if gocal provides VALARM access
    // If not, implement custom VALARM parsing
    for _, valarm := range gocalEvent.Alarms { // Assuming gocal provides this
        alert, err := p.convertVALARM(valarm)
        if err != nil {
            continue // Skip invalid VALARMs
        }
        alerts = append(alerts, alert)
    }
    
    return alerts, nil
}

func (p *GocalParser) convertVALARM(valarm VAlarmData) (Alert, error) {
    // Parse TRIGGER field to determine offset
    offset, err := p.parseTrigger(valarm.Trigger)
    if err != nil {
        return Alert{}, err
    }
    
    return Alert{
        Offset:      offset,
        Important:   false, // VALARM doesn't specify importance, use default
        Source:      AlertSourceVALARM,
        Description: valarm.Description,
        Action:      AlertActionDisplay, // Only DISPLAY supported for now
    }, nil
}
```

### 3. Simplified Event Interface Architecture

The current Event interface has redundant methods that should be eliminated:

**Current problematic interface:**
```go
type Event interface {
    // ... basic getters ...
    OccursOn(date time.Time) bool                                        // OLD: doesn't consider alerts
    OccurredWithin(start, end time.Time) []time.Time                    // OLD: raw times, superseded
    NextOccurrence(after time.Time) *time.Time                          // OLD: raw times, superseded  
    OccurrencesWithin(start, end time.Time, alerts []config.AlertConfig) []Occurrence // NEW: comprehensive
    // ... alert state methods ...
}
```

**New simplified interface:**
```go
type Event interface {
    // ... basic getters ...
    
    // Self-contained methods - events know their own alerts
    OccursOn(date time.Time) bool                                       // Enhanced: considers all alerts
    OccurrencesWithin(start, end time.Time) []Occurrence               // Uses event's complete alert set
    
    // Alert management - no external parameters needed
    GetAllAlerts() []Alert                                              // Complete merged alert set
    GetIntrinsicAlerts() []Alert                                        // Event's VALARM alerts only
    GetAutomaticAlerts() []Alert                                        // Config-based alerts only
    
    // ... alert state methods unchanged ...
}
```

**Architectural cleanup:**
- **Remove `OccurredWithin`** - superseded by `OccurrencesWithin` (can extract EventTime from Occurrence)
- **Remove `NextOccurrence`** - superseded by `OccurrencesWithin` with small time range
- **Keep recurrence layer methods** - `recurrence.Recurrence` still needs these for internal calculations
- **Refactor all callers** to use the comprehensive `OccurrencesWithin` method

### 4. Calendar-as-Entity Architecture

Upgrade Calendar from config reference to proper domain entity:

```go
// Calendar becomes a first-class entity managing its events and policies
type Calendar struct {
    Path            string    // Directory path
    Template        string    // Notification template
    AutomaticAlerts []Alert   // Live, updateable alert policies
    events          map[string]Event // Events belonging to this calendar
    mutex           sync.RWMutex
}

// Calendar manages its events and alert policies
func (c *Calendar) UpdateAutomaticAlerts(newAlerts []Alert) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    c.AutomaticAlerts = newAlerts
    // All events automatically see updated alerts via pointer reference
}

func (c *Calendar) AddEvent(event Event) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    c.events[event.GetUID()] = event
}

// CalendarEvent belongs to a Calendar entity
type CalendarEvent struct {
    // ... existing fields ...
    
    Calendar        *Calendar // Pointer to shared calendar entity
    IntrinsicAlerts []Alert   // VALARM-based alerts from ICS
}

// Events get live alert data from their calendar
func (e *CalendarEvent) GetAllAlerts() []Alert {
    e.Calendar.mutex.RLock()
    configAlerts := e.Calendar.AutomaticAlerts
    e.Calendar.mutex.RUnlock()
    
    allAlerts := make([]Alert, 0, len(e.IntrinsicAlerts)+len(configAlerts))
    
    // Add intrinsic VALARM alerts
    allAlerts = append(allAlerts, e.IntrinsicAlerts...)
    
    // Add current automatic alerts from calendar
    allAlerts = append(allAlerts, configAlerts...)
    
    // VALARM alerts take precedence over config alerts with same offset
    return deduplicateAlerts(allAlerts)
}
```

### 5. Self-Contained Daily Indexing

Fix `OccursOn()` to be self-contained and consider alert days:

```go
func (e *CalendarEvent) OccursOn(date time.Time) bool {
    // Check if the event itself occurs on this date
    if e.eventOccursOn(date) {
        return true
    }
    
    // Check if any alerts for this event fire on this date
    allAlerts := e.GetAllAlerts()
    
    // Get next event occurrence after the start of the given date  
    dayStart := date.Truncate(24 * time.Hour)
    dayEnd := dayStart.Add(24 * time.Hour)
    
    // Use OccurrencesWithin to find alerts firing on this day
    // Look ahead up to maximum alert offset to find events whose alerts fire today
    maxOffset := e.getMaxAlertOffset()
    searchEnd := dayEnd.Add(maxOffset)
    
    occurrences := e.OccurrencesWithin(dayStart, searchEnd)
    for _, occ := range occurrences {
        // Check if this occurrence's alert time falls on the target date
        if occ.AlertTime.After(dayStart) && occ.AlertTime.Before(dayEnd) {
            return true
        }
    }
    
    return false
}

func (e *CalendarEvent) getMaxAlertOffset() time.Duration {
    allAlerts := e.GetAllAlerts()
    var maxOffset time.Duration
    for _, alert := range allAlerts {
        if alert.Offset > maxOffset {
            maxOffset = alert.Offset
        }
    }
    return maxOffset
}

// Helper method for original event occurrence check
func (e *CalendarEvent) eventOccursOn(date time.Time) bool {
    // Current OccursOn logic unchanged
    // ...
}
```

## Implementation Plan

### Phase 1: Alert Architecture and Calendar Entity Foundation (TDD)
1. **Create Alert struct** with unified representation
2. **Create Calendar entity** with alert management and event collection
3. **Add conversion functions** from `config.AlertConfig` to `Alert`
4. **Write comprehensive tests** for Alert struct, Calendar entity, and conversions
5. **Update config package** to provide Alert conversion methods

### Phase 2: VALARM Parsing (TDD)
1. **Research gocal VALARM support** - check if gocal.Event includes alarm data
2. **Implement VALARM parsing** in parser package (may require custom parsing if gocal doesn't support it)
3. **Add TRIGGER parsing logic** supporting duration and absolute formats
4. **Write parser tests** for various VALARM scenarios
5. **Handle VALARM parsing errors gracefully**

### Phase 3: Calendar-Aware Event Interface (TDD)
1. **Update Event interface** to remove parameter-based methods
2. **Modify CalendarEvent** to reference Calendar entity and IntrinsicAlerts
3. **Update Event constructor** to accept Calendar reference and VALARM alerts
4. **Remove OccurredWithin and NextOccurrence** from Event interface (keep in recurrence layer)  
5. **Implement self-contained OccursOn and OccurrencesWithin methods**
6. **Write tests** for calendar-aware events and alert merging
7. **Add Calendar entity management methods**

### Phase 4: Calendar-Centric Storage Refactor (TDD)
1. **Refactor MemoryEventStorage** to manage Calendar entities
2. **Implement Calendar creation and update logic** for config changes
3. **Update parser integration** to work with Calendar entities
4. **Migrate daily indexing** to use self-contained events
5. **Write comprehensive tests** for calendar-aware storage
6. **Test edge cases** like multi-day advance warnings and config updates

### Phase 5: Integration and Alert Scheduler Updates (TDD)
1. **Update alert scheduler** to work with Calendar entities
2. **Ensure proper alert state tracking** with new architecture
3. **Update file watching** to coordinate with Calendar management
4. **Implement config reload** that updates Calendar entities
5. **Write integration tests** for end-to-end VALARM and config alert processing

### Phase 6: Configuration Backwards Compatibility
1. **Ensure existing configs work unchanged**
2. **Test config Alert conversion thoroughly**
3. **Document VALARM behavior** in README and example configs
4. **Add configuration examples** showing VALARM + config interaction
5. **Update templates** if needed for VALARM-specific alert data

## Technical Details

### Calendar-Centric Storage Architecture

Storage layer manages Calendar entities, not just events:

**Calendar-aware storage:**
```go
type MemoryEventStorage struct {
    // Calendar entities managing their events and policies
    calendars     map[string]*Calendar  // path -> calendar entity
    
    // Global daily index for fast lookups across all calendars
    dailyIndex    map[string][]Event    // YYYY-MM-DD -> events
    
    // File tracking for updates
    fileToUID     map[string]string     // filename -> UID
    uidToCalendar map[string]*Calendar  // UID -> calendar (for quick lookup)
    
    mutex sync.RWMutex
}

// Storage creates and manages calendar entities
func (s *MemoryEventStorage) EnsureCalendar(path string, config config.DirectoryConfig) *Calendar {
    s.mutex.Lock()
    defer s.mutex.Unlock()
    
    if calendar, exists := s.calendars[path]; exists {
        // Update existing calendar with new config
        calendar.UpdateAutomaticAlerts(convertConfigAlerts(config.AutomaticAlerts))
        calendar.Template = config.Template
        return calendar
    }
    
    // Create new calendar entity
    calendar := &Calendar{
        Path:            path,
        Template:        config.Template,
        AutomaticAlerts: convertConfigAlerts(config.AutomaticAlerts),
        events:          make(map[string]Event),
    }
    
    s.calendars[path] = calendar
    return calendar
}
```

**Enhanced parser integration:**
```go
// Parser creates events with calendar entity references
func (p *GocalParser) ParseDirectoryWithCalendar(calendar *Calendar) ([]storage.Event, error) {
    // Parse ICS files from calendar's directory
    events, err := p.parseDirectoryEvents(calendar.Path)
    if err != nil {
        return nil, err
    }
    
    // Inject calendar reference into each event
    for _, event := range events {
        if calEvent, ok := event.(*CalendarEvent); ok {
            calEvent.Calendar = calendar  // Share calendar entity
            // VALARM alerts already parsed into IntrinsicAlerts
        }
        
        // Add event to calendar's collection
        calendar.AddEvent(event)
    }
    
    return events, nil
}
```

**Simplified daily indexing:**
```go
// Events are self-contained, storage just queries them
func (s *MemoryEventStorage) GetEventsForDay(date time.Time) []Event {
    var dayEvents []Event
    
    s.mutex.RLock()
    defer s.mutex.RUnlock()
    
    // Query events from all calendars
    for _, calendar := range s.calendars {
        calendar.mutex.RLock()
        for _, event := range calendar.events {
            // Event determines its own daily index inclusion (considers all alerts)
            if event.OccursOn(date) {
                dayEvents = append(dayEvents, event)
            }
        }
        calendar.mutex.RUnlock()
    }
    
    return dayEvents
}
```

### VALARM Parsing Challenges

If gocal doesn't provide VALARM access, implement custom parsing:

```go
func (p *GocalParser) parseCustomVALARMs(icsContent string, eventUID string) []Alert {
    // Custom regex/string parsing for VALARM components
    // This is a fallback if gocal doesn't support VALARMs
    
    // Look for VALARM blocks within the event
    // Parse TRIGGER, ACTION, DESCRIPTION fields
    // Handle various TRIGGER formats
}
```

### TRIGGER Parsing

Support multiple TRIGGER formats:

```go
func parseTrigger(trigger string) (time.Duration, error) {
    if strings.HasPrefix(trigger, "-P") {
        // Duration format: -PT15M, -P1DT2H30M
        return parseDurationTrigger(trigger)
    } else if strings.HasPrefix(trigger, "19") || strings.HasPrefix(trigger, "20") {
        // Absolute time format: 19980101T050000Z
        return parseAbsoluteTrigger(trigger)
    }
    return 0, fmt.Errorf("unsupported TRIGGER format: %s", trigger)
}
```

### Alert Deduplication Strategy

When merging VALARM and config alerts:

```go
func deduplicateAlerts(alerts []Alert) []Alert {
    seen := make(map[time.Duration]bool)
    var unique []Alert
    
    // VALARM alerts take precedence over config alerts for same offset
    for _, alert := range alerts {
        if alert.Source == AlertSourceVALARM || !seen[alert.Offset] {
            unique = append(unique, alert)
            seen[alert.Offset] = true
        }
    }
    
    return unique
}
```

### Daily Index Performance Considerations

The enhanced `OccursOn()` method needs optimization to avoid performance degradation:

```go
// Optimization: Cache alert calculation results
type alertCache struct {
    date   time.Time
    alerts []Alert
    result bool
}

func (e *CalendarEvent) OccursOn(date time.Time, alerts []Alert) bool {
    // Check cache first
    if cached := e.checkAlertCache(date, alerts); cached != nil {
        return cached.result
    }
    
    // Compute and cache result
    result := e.computeOccursOn(date, alerts)
    e.cacheAlertResult(date, alerts, result)
    return result
}
```

## Configuration Examples

### Mixed VALARM and Config Alerts

```yaml
directories:
  - directory: ~/.calendars/work
    automatic_alerts:
      - value: 10         # 10 minute config alert
        unit: minutes
        important: false
      - value: 1          # 1 hour config alert
        unit: hours  
        important: true
```

With ICS file containing:
```ics
BEGIN:VALARM
ACTION:DISPLAY  
DESCRIPTION:5 minute warning
TRIGGER:-PT5M              # 5 minute VALARM alert
END:VALARM
```

Result: Event gets 5min (VALARM), 10min (config), and 1hr (config) alerts.

### VALARM Priority

VALARM alerts override config alerts with the same timing:

- ICS VALARM: `-PT10M` (10 minutes)  
- Config alert: `10 minutes`
- Result: Only VALARM alert fires (takes precedence)

## Files to Modify

### Core Implementation
1. **internal/storage/event.go** - Add Alert struct, Calendar entity, update Event interface
2. **internal/storage/calendar.go** - **NEW**: Calendar entity implementation
3. **internal/parser/parser.go** - Add VALARM parsing logic, Calendar integration
4. **internal/config/config.go** - Add Alert conversion methods
5. **internal/storage/storage.go** - Refactor to manage Calendar entities, update daily indexing
6. **internal/alerts/scheduler.go** - Update to work with Calendar entities

### Testing and Documentation  
7. **internal/storage/event_test.go** - Tests for Calendar-aware events and daily indexing
8. **internal/storage/calendar_test.go** - **NEW**: Tests for Calendar entity
9. **internal/parser/parser_test.go** - Tests for VALARM parsing and Calendar integration
10. **config.example.yaml** - Examples showing VALARM behavior
11. **README.md** - Document VALARM support and mixed alert behavior

## Testing Strategy

### Unit Tests
1. **Alert struct tests** - Conversion, deduplication, merging
2. **VALARM parsing tests** - Various TRIGGER formats, malformed VALARMs
3. **Daily indexing tests** - Events with advance warnings appearing in correct daily indices
4. **Occurrence generation tests** - Mixed VALARM and config alerts

### Integration Tests  
1. **End-to-end VALARM processing** - ICS file with VALARMs → notifications
2. **Mixed alert scenarios** - Config + VALARM alerts working together
3. **Daily index integrity** - Events appearing in indices on both event and alert days
4. **Backwards compatibility** - Existing configs work unchanged

### Edge Case Tests
1. **Invalid VALARMs** - Malformed TRIGGER fields, unsupported ACTION types
2. **Alert conflicts** - VALARM and config with same timing
3. **Complex recurrence** - Recurring events with VALARMs and advance warnings
4. **Timezone handling** - VALARM alerts across timezone boundaries

## Success Criteria

- **VALARM Support**: ICS files with VALARM components trigger alerts as specified
- **Calendar Entity**: Calendar becomes a proper domain entity managing events and alert policies
- **Live Config Updates**: Changes to automatic_alerts update all events without daemon restart
- **Alert Merging**: Config and VALARM alerts work together without conflicts  
- **Daily Index Fix**: Events appear in daily indices on days when their alerts fire
- **Simplified Architecture**: Events are self-contained, no external parameter passing
- **Backwards Compatibility**: Existing configurations work unchanged
- **Performance**: Daily indexing performance remains acceptable with alert-aware logic
- **Documentation**: Clear explanation of VALARM behavior and Calendar entity architecture

## Next Steps

**Planning Complete**: This improvement is ready for implementation once user confirms the approach aligns with their requirements.

**Important Note**: As per CLAUDE.md, during planning I must NOT edit any files except CLAUDE.md and this improvement file. Implementation only begins once the user explicitly states the planning is over.