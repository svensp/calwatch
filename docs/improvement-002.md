# Improvement 002: Laptop Sleep/Wake Event Recovery

## Context

CalWatch is primarily intended for laptop use, but the current implementation has critical issues with system sleep/wake cycles:

1. **Missed Timer Events**: The midnight index recreation timer won't fire if the laptop is asleep at 00:00
2. **Missed Alerts**: Events that should have triggered alerts while the system was asleep will never fire
3. **Stale Index**: Daily event index becomes outdated after sleep periods spanning multiple days

## Problem Analysis

Current implementation assumes continuous operation with:
- Minute-based timer running consistently at hh:mm:00
- Daily index recreation happening exactly at midnight
- `Event.occursOn(date)` checking only specific days

## Proposed Solution

Implement system wake-up detection and catch-up logic:

### Core Changes

1. **Wake-up Detection**: Detect when system has been asleep by comparing last known tick time with current time
2. **Missed Event Processing**: Process all events that should have fired during sleep period
3. **Progressive Index Updates**: Update daily indexes for each missed day
4. **Enhanced Event Interface**: Replace `Event.occursOn(date)` with `Event.occurredWithin(lastAlertTick, currentTime)`

### Implementation Strategy

```go
// New interface method
type Event interface {
    // ... existing methods ...
    OccurredWithin(start, end time.Time) []time.Time  // Returns all occurrences in time range
}

// Wake-up recovery logic
func (a *AlertScheduler) HandleWakeup(lastTick, currentTime time.Time) error {
    // 1. Calculate missed days
    missedDays := calculateMissedDays(lastTick, currentTime)
    
    // 2. Process each missed day sequentially
    for _, day := range missedDays {
        // Fire alerts for that day (with "missed" indication)
        a.processMissedDay(day)
        
        // Update daily index for next day
        a.storage.RegenerateIndex(day.Add(24 * time.Hour))
    }
    
    // 3. Resume normal operation
    return nil
}
```

### Key Components to Modify

1. **Alert Scheduler** (`internal/alerts/`):
   - Add wake-up detection logic
   - Implement missed event processing
   - Track last successful tick time

2. **Event Interface** (`internal/storage/`):
   - Replace `OccursOn(date)` with `OccurredWithin(start, end time.Time)`
   - Return all event occurrences within time range
   - Handle recurring events across multiple days

3. **Storage Layer** (`internal/storage/`):
   - Support batch index regeneration for multiple days
   - Persist last tick time across daemon restarts
   - Efficient range-based event queries

4. **Notification System** (`internal/notifications/`):
   - Add "missed event" template variants
   - Batch notifications to avoid overwhelming user
   - Optional summary notifications for many missed events

### User Experience Considerations

1. **Missed Event Notifications**: Show events that were missed with clear indication
2. **Batch Processing**: Group multiple missed events to avoid notification spam
3. **Summary Option**: Provide summary of missed events rather than individual notifications
4. **Configuration**: Allow users to configure missed event behavior (show all, summary only, skip)

### Detailed Missed Event Notification Policies

#### Policy Types
1. **"all"**: Show every missed event individually
   - Use `duration_when_late` for each notification
   - Good for light calendar usage
   - Risk: notification spam after long sleep

2. **"summary"**: Group missed events into summary notifications
   - Single notification: "You missed 7 events while away"
   - Expandable to show details
   - Triggered when `summary_threshold` exceeded

3. **"priority_only"**: Only show high-priority missed events
   - Requires event priority classification
   - Business meetings > personal reminders
   - Based on keywords, attendees, or explicit priority

4. **"skip"**: Skip all missed events, resume normal operation
   - Clean slate approach
   - Useful for vacation scenarios

#### Duration Handling Strategies

**Notification Duration Types**:
- `"timed"`: Auto-dismiss after specified duration (default behavior)
- `"until_dismissed"`: Persistent notification requiring user click to dismiss

**Duration Configuration Structure**:
```yaml
duration:
  type: "timed"        # "timed" or "until_dismissed"
  value: 5             # Only required for "timed" type
  unit: "seconds"      # "milliseconds", "seconds", "minutes"
```

**Supported Time Units**:
- `"milliseconds"` (ms)
- `"seconds"` (default, most user-friendly)
- `"minutes"` (for very long notifications)

**D-Bus Duration Mapping**:
```go
// Convert user-friendly duration to D-Bus milliseconds
func convertDuration(config DurationConfig) int32 {
    switch config.Type {
    case "until_dismissed":
        return 0  // D-Bus: never auto-dismiss
    case "timed":
        return int32(config.toMilliseconds())
    default:
        return 5000  // default 5 seconds
    }
}

func (d DurationConfig) toMilliseconds() int {
    switch d.Unit {
    case "milliseconds": return d.Value
    case "seconds":      return d.Value * 1000
    case "minutes":      return d.Value * 60 * 1000
    default:             return d.Value * 1000 // default to seconds
    }
}
```

This eliminates "magic numbers" and provides user-friendly duration configuration.

## Technical Details

### Wake-up Detection Methods
- Compare `time.Now()` with last stored tick time
- Threshold-based detection (e.g., gap > 2 minutes indicates sleep)
- Persist last tick time to survive daemon restarts

### Range-based Event Processing
```go
// Example implementation
func (e *RecurringEvent) OccurredWithin(start, end time.Time) []time.Time {
    var occurrences []time.Time
    
    // Expand recurring rule within time range
    current := start
    for current.Before(end) {
        if e.rrule.OccursOn(current) {
            occurrences = append(occurrences, current)
        }
        current = current.Add(24 * time.Hour)
    }
    
    return occurrences
}
```

### Configuration Options
```yaml
wakeup_handling:
  enable: true
  missed_event_policy: "all"           # "all", "summary", "priority_only", "skip"
  max_missed_days: 7                   # Limit how far back to process
  summary_threshold: 5                 # Show summary if more than N events
  max_catchup_time: 
    value: 30
    unit: "seconds"                    # Max time to spend on catch-up processing
  
notification:
  # Normal notification duration
  duration:
    type: "timed"                      # "timed" (default) or "until_dismissed"
    value: 5
    unit: "seconds"
    
  # Duration for missed/late notifications  
  duration_when_late:
    type: "until_dismissed"            # Requires user action to dismiss
    # value/unit not needed for "until_dismissed" type
    
  # Alternative: timed late notifications with longer duration
  # duration_when_late:
  #   type: "timed"
  #   value: 30
  #   unit: "seconds"
```

## Implementation Plan

1. **Phase 1**: Enhance Event interface with time-range methods
2. **Phase 2**: Add wake-up detection to alert scheduler
3. **Phase 3**: Implement missed event processing logic
4. **Phase 4**: Add notification templates for missed events
5. **Phase 5**: Add configuration options and user controls
6. **Phase 6**: Testing with real sleep/wake scenarios

## Testing Strategy

1. **Unit Tests**: Test range-based event methods with various scenarios
2. **Integration Tests**: Simulate sleep/wake cycles with time manipulation
3. **Manual Testing**: Actual laptop sleep/wake testing with real calendar data
4. **Edge Cases**: Multi-day sleep, recurring events spanning sleep period, timezone changes

## Additional Critical Considerations

### 1. Persistence Layer
**Problem**: Last alert tick time must survive daemon restarts and system reboots.

**Solution**: 
- Store last tick time in `~/.local/state/calwatch/state.json` (via `xdg.StateFile()`)
- Atomic writes to prevent corruption
- Fallback to daemon start time if state file missing

**Key Insight**: This also detects missed events after daemon shutdown or system restart, which is **desired behavior** - not just sleep cycles.

### 2. Performance Constraints
**Problem**: Catching up after weeks of sleep could block the daemon.

**Solution**:
- `max_catchup_time` limit (e.g., 30 seconds)
- Process missed days in chunks with yield points
- Background processing with progress indication

### 3. Timezone Handling During Sleep
**Problem**: Laptop traveling across timezones while asleep.

**Solution**:
- Store events in UTC, convert for display
- Detect timezone changes on wake-up
- Recalculate missed events with correct timezone

### 4. Alert State Persistence
**Problem**: Current alert states (sent/pending) must be preserved across sleep.

**Solution**:
- Persist alert states in state file
- Reset "sent" states for events that should repeat
- Handle edge case of recurring events with multiple missed occurrences

### 5. Graceful Degradation
**Problem**: Overwhelming user after extended absence.

**Solution**:
- Hard limits: `max_missed_days`, `max_missed_events`
- Exponential backoff for very old events
- "Vacation mode" detection (>7 days = summary only)

### 6. Event Priority Classification
For `"priority_only"` policy:
```go
type EventPriority int
const (
    PriorityLow EventPriority = iota
    PriorityNormal  
    PriorityHigh
    PriorityCritical
)

// Priority rules (configurable)
func classifyEvent(event Event) EventPriority {
    // Has attendees = higher priority
    // Keywords: "meeting", "deadline", "urgent"
    // Calendar source (work calendar = higher)
    // Time sensitivity (imminent = higher)
}
```

## Files to Modify

- `internal/storage/event.go` - Enhanced event interface with `OccurredWithin()`
- `internal/storage/memory.go` - Range-based storage queries, alert state persistence
- `internal/storage/state.go` - **NEW**: Persistent state management
- `internal/alerts/scheduler.go` - Wake-up detection, missed event processing
- `internal/alerts/priority.go` - **NEW**: Event priority classification
- `internal/notifications/` - Missed event templates, duration handling
- `internal/config/config.go` - Wake-up handling configuration
- `cmd/calwatch/main.go` - State file initialization, graceful shutdown
- Configuration files - New wake-up and notification options

## Edge Cases and Error Scenarios

### 1. State File Corruption
- Graceful fallback to daemon start time
- Validate state file format before parsing
- Backup/restore mechanisms for critical state

### 2. Clock Changes (NTP sync, manual adjustment)
- Detect backwards time jumps
- Handle forward time jumps vs. sleep detection
- Validate wake-up detection against reasonable thresholds

### 3. Partial Processing Failures
- Some events fail to process during catch-up
- Calendar files corrupted or inaccessible
- Notification system temporarily unavailable

### 4. Resource Exhaustion
- Memory usage during large catch-up operations
- CPU usage limits during background processing
- File descriptor limits for many calendar files

### 5. Recurring Event Complexities
- Events with multiple missed occurrences
- RRULE exceptions (EXDATE) during missed period
- Timezone-sensitive recurring events

## Success Criteria

- Laptop can sleep for days and wake up with proper event catch-up
- No missed calendar alerts after system wake-up  
- Daily indexes correctly updated after multi-day sleep
- User-friendly handling of missed events (not overwhelming)
- Configurable behavior for different use cases
- Robust error handling for all edge cases
- Performance remains acceptable even after extended sleep
- State persistence survives crashes and reboots

## Implementation Complexity Assessment

**High Complexity Areas**:
- Range-based event processing with timezone handling
- Persistent state management with atomic operations
- Performance optimization for large catch-up operations

**Medium Complexity Areas**:
- Wake-up detection and threshold tuning
- Notification policy implementation
- Event priority classification

**Low Complexity Areas**:
- Configuration options
- Template updates for missed events
- Basic D-Bus duration mapping

## Next Steps

Need user confirmation of approach and any specific requirements for missed event handling behavior.