# Improvement 003: Important Notifications and Occurrence-Based Alert Architecture

## Context

CalWatch currently treats all notifications equally and has architectural issues with how alert timing information is managed. Two key improvements are needed:

1. **UX Enhancement**: Leverage D-Bus notify's "important" capability for critical alerts
2. **Architecture Improvement**: Replace `OccurredWithin` boolean return with proper `Occurrence` objects that carry alert context

## Problem Analysis

### Current Issues

1. **Notification Priority**: All notifications appear with same urgency level, regardless of event importance
2. **Poor Separation of Concerns**: Alert timing logic (5min warning, 2min warning) is scattered outside the event layer where it doesn't belong
3. **Suboptimal API Design**: `OccurredWithin` returns boolean, requiring external logic to determine which specific alerts triggered
4. **Broken Late Notification Feature**: `duration_when_late` config exists but doesn't work because the range-based approach lost the "late" vs "current" distinction that the boolean `OccurredWithin` interface can't provide

### D-Bus Important Notifications

D-Bus notifications support urgency levels:
- `Low` (0): Background notifications
- `Normal` (1): Standard notifications (current behavior)
- `Critical` (2): Important notifications that bypass Do Not Disturb

Critical notifications have enhanced behavior:
- Persist longer or until dismissed
- Bypass notification filters
- May use different visual/audio styling
- Higher priority in notification centers

## Proposed Solution

### 1. Configuration Enhancement

Add `important` boolean property to `automatic_alerts` configuration:

```yaml
directories:
  - path: ~/.calendars/personal
    automatic_alerts:
      - offset: 5m
        important: false    # default
      - offset: 30m  
        important: true     # critical notifications
```

### 2. New Occurrence-Based Architecture

Replace `OccurredWithin(start, end time.Time) bool` with:

```go
type Occurrence struct {
    EventTime   time.Time     // When the event actually occurs
    AlertTime   time.Time     // When this alert should fire  
    Offset      time.Duration // Alert offset (5m, 30m, etc.)
    Important   bool          // Whether this alert is marked important
    Late        bool          // Whether this alert is firing late (past AlertTime)
    EventData   *Event        // Reference to the full event
}

type Event interface {
    // Replace OccurredWithin with:
    OccurrencesWithin(start, end time.Time, alerts []AlertConfig) []Occurrence
}
```

### 3. D-Bus Urgency Integration

Extend notification backend to support urgency levels:

```go
type NotificationRequest struct {
    Title    string
    Body     string
    Urgency  UrgencyLevel  // New field
    Duration time.Duration
    // ... existing fields
}

type UrgencyLevel int
const (
    UrgencyLow UrgencyLevel = iota     // 0 - D-Bus Low
    UrgencyNormal                      // 1 - D-Bus Normal (default)
    UrgencyCritical                    // 2 - D-Bus Critical
)
```

## Implementation Plan

### Phase 1: Configuration Schema Changes
1. **Update config structs** to include `important` field in alert definitions
2. **Add validation** to ensure proper configuration parsing
3. **Update example configs** to demonstrate important alerts
4. **Maintain backwards compatibility** with existing configs (default `important: false`)

### Phase 2: Occurrence Architecture Refactor
1. **Define Occurrence struct** with all necessary context
2. **Update Event interface** to replace `OccurredWithin` with `OccurrencesWithin`
3. **Implement in concrete Event types** (SimpleEvent, RecurringEvent)
4. **Move alert timing logic** from scheduler into event layer where it belongs

### Phase 3: D-Bus Urgency Support
1. **Extend NotificationBackend interface** to accept urgency levels
2. **Update notify-send implementation** to map urgency to D-Bus levels
3. **Add urgency parameter** to D-Bus notification calls
4. **Test important notification behavior** across desktop environments

### Phase 4: Integration and Alert Scheduler Updates
1. **Update AlertScheduler** to use new `OccurrencesWithin` method
2. **Pass through importance flags** from config to notifications
3. **Ensure proper Occurrence handling** in missed event recovery
4. **Update template system** to access Occurrence data
5. **Implement Late flag usage** in notification duration selection (`duration_when_late` vs `duration`)

### Phase 5: Testing and Documentation
1. **Unit tests** for Occurrence generation logic
2. **Integration tests** for important notification delivery
3. **Manual testing** across desktop environments (GNOME, KDE, etc.)
4. **Update README** with important notification documentation

## Technical Details

### Alert Configuration Processing

Current flow (problematic):
```
Config → AlertScheduler → Event.OccurredWithin() → External alert timing logic
```

New flow (improved):
```
Config → Event.OccurrencesWithin(alerts) → Occurrence objects → AlertScheduler
```

### Occurrence Generation Example

```go
func (e *SimpleEvent) OccurrencesWithin(start, end time.Time, alerts []AlertConfig) []Occurrence {
    var occurrences []Occurrence
    
    // Check if event occurs in time range
    if e.StartTime.After(start) && e.StartTime.Before(end) {
        // Generate occurrence for each configured alert
        for _, alert := range alerts {
            alertTime := e.StartTime.Add(-alert.Offset)
            if alertTime.After(start) && alertTime.Before(end) {
                // Determine if this alert is late (should have fired more than a minute before 'end'/now)
                // Since we check every minute, anything more than ~1 minute overdue is "late"
                minuteThreshold := time.Minute
                isLate := alertTime.Before(end.Add(-minuteThreshold))
                
                occurrences = append(occurrences, Occurrence{
                    EventTime: e.StartTime,
                    AlertTime: alertTime,
                    Offset:    alert.Offset,
                    Important: alert.Important,
                    Late:      isLate,
                    EventData: e,
                })
            }
        }
    }
    
    return occurrences
}
```

### D-Bus Urgency Mapping

```go
func (n *NotifySendBackend) SendNotification(req NotificationRequest) error {
    urgencyFlag := map[UrgencyLevel]string{
        UrgencyLow:      "--urgency=low",
        UrgencyNormal:   "--urgency=normal",  
        UrgencyCritical: "--urgency=critical",
    }[req.Urgency]
    
    cmd := exec.Command("notify-send", urgencyFlag, req.Title, req.Body)
    return cmd.Run()
}
```

## Benefits

### Architecture Benefits
1. **Better Separation of Concerns**: Alert timing logic moves into Event layer where it belongs
2. **Richer Data Model**: Occurrence objects carry all context needed for rendering
3. **Easier Testing**: Alert logic can be unit tested within Event implementations
4. **Future Extensibility**: Occurrence struct can easily gain new fields (priority, categories, etc.)
5. **Fixes Late Notifications**: `Late` flag enables proper `duration_when_late` functionality

### UX Benefits  
1. **Visual Distinction**: Important alerts stand out in notification centers
2. **Bypass Filters**: Critical notifications ignore Do Not Disturb modes
3. **User Control**: Granular control over which alerts are marked important
4. **Better Persistence**: Important notifications may persist longer
5. **Restored Late Notifications**: `duration_when_late` will work again for missed/delayed alerts

### Code Quality Benefits
1. **Eliminated External Logic**: No more scattered alert timing calculations
2. **Type Safety**: Compile-time guarantees about alert data
3. **Easier Debugging**: All alert context available in single object
4. **Reduced Coupling**: Scheduler depends only on Occurrence interface

## Configuration Example

```yaml
directories:
  - path: ~/.calendars/work
    automatic_alerts:
      - offset: 15m
        important: false    # Gentle reminder
      - offset: 5m  
        important: true     # Urgent final warning
        
  - path: ~/.calendars/personal  
    automatic_alerts:
      - offset: 10m
        important: false    # Normal personal reminders

notification:
  backend: "notify-send"
  duration:
    type: "timed"
    value: 5
    unit: "seconds"
```

## Files to Modify

1. **internal/config/config.go** - Add `Important bool` to AlertConfig struct
2. **internal/storage/event.go** - Define Occurrence struct, update Event interface  
3. **internal/storage/simple_event.go** - Implement new OccurrencesWithin method
4. **internal/storage/recurring_event.go** - Implement new OccurrencesWithin method
5. **internal/alerts/scheduler.go** - Update to use Occurrence-based API
6. **internal/notifications/types.go** - Add UrgencyLevel to NotificationRequest
7. **internal/notifications/notify_send.go** - Add urgency support to D-Bus calls
8. **config.example.yaml** - Add important alert examples
9. **README.md** - Document important notification feature

## Testing Strategy

1. **Unit Tests**: Test Occurrence generation for various event types and alert configs
2. **Integration Tests**: Test end-to-end important notification delivery
3. **Manual Testing**: Verify important notifications behave correctly across desktop environments
4. **Regression Tests**: Ensure existing functionality unchanged for non-important alerts
5. **Configuration Tests**: Test backwards compatibility and validation

## Success Criteria

- Important alerts marked in config trigger D-Bus critical notifications
- Alert timing logic fully encapsulated within Event implementations  
- Occurrence objects contain all necessary context for notification rendering
- Backwards compatibility maintained for existing configurations
- Important notifications visually distinguishable in notification centers
- Code architecture improved with better separation of concerns
- `duration_when_late` configuration works correctly for late/missed alerts
- Late notifications properly use extended duration settings

## Next Steps

Ready to proceed with implementation once planning is confirmed complete.