package storage

import (
	"fmt"
	"sync"
	"time"
	
	"calwatch/internal/config"
	"calwatch/internal/recurrence"
)

// AlertSource indicates where an alert originates from
type AlertSource int

const (
	AlertSourceConfig AlertSource = iota // From automatic_alerts configuration
	AlertSourceVALARM                    // From ICS VALARM component
)

// AlertAction specifies the type of alert action
type AlertAction int

const (
	AlertActionDisplay AlertAction = iota // Display notification (only supported for now)
	AlertActionEmail                      // Email notification (future)
	AlertActionAudio                      // Audio notification (future)
)

// Alert represents a unified alert that can come from config or VALARM
type Alert struct {
	Offset      time.Duration // How far before event to trigger (e.g., 15 minutes)
	Important   bool          // Whether this alert should use critical urgency
	Source      AlertSource   // Whether from config or VALARM
	Description string        // VALARM description or generated description
	Action      AlertAction   // DISPLAY, EMAIL, AUDIO (for future extensibility)
}

// ConvertConfigAlert converts a config.AlertConfig to a storage.Alert
func ConvertConfigAlert(alertConfig config.AlertConfig) (Alert, error) {
	offset, err := alertConfig.Duration()
	if err != nil {
		return Alert{}, err
	}
	
	return Alert{
		Offset:      offset,
		Important:   alertConfig.Important,
		Source:      AlertSourceConfig,
		Description: fmt.Sprintf("%d %s warning", alertConfig.Value, alertConfig.Unit),
		Action:      AlertActionDisplay,
	}, nil
}

// ConvertConfigAlerts converts a slice of config.AlertConfig to storage.Alert
func ConvertConfigAlerts(alertConfigs []config.AlertConfig) ([]Alert, error) {
	alerts := make([]Alert, 0, len(alertConfigs))
	
	for _, alertConfig := range alertConfigs {
		alert, err := ConvertConfigAlert(alertConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to convert alert config: %w", err)
		}
		alerts = append(alerts, alert)
	}
	
	return alerts, nil
}

// DeduplicateAlerts removes duplicate alerts with the same offset
// VALARM alerts take precedence over config alerts for the same offset
func DeduplicateAlerts(alerts []Alert) []Alert {
	seen := make(map[time.Duration]bool)
	var unique []Alert
	
	// Process VALARM alerts first (they take precedence)
	for _, alert := range alerts {
		if alert.Source == AlertSourceVALARM {
			if !seen[alert.Offset] {
				unique = append(unique, alert)
				seen[alert.Offset] = true
			}
		}
	}
	
	// Then process config alerts (only if offset not already seen)
	for _, alert := range alerts {
		if alert.Source == AlertSourceConfig && !seen[alert.Offset] {
			unique = append(unique, alert)
			seen[alert.Offset] = true
		}
	}
	
	return unique
}

// AlertState represents the state of an alert for an event
type AlertState int

const (
	AlertPending AlertState = iota
	AlertSent
	AlertSnoozed
)

// Occurrence represents a specific alert occurrence for an event
type Occurrence struct {
	EventTime   time.Time     // When the event actually occurs
	AlertTime   time.Time     // When this alert should fire
	Offset      time.Duration // Alert offset (5m, 30m, etc.)
	Important   bool          // Whether this alert is marked important
	Late        bool          // Whether this alert is firing late (past intended time)
	EventData   Event         // Reference to the full event
}

// Event represents a calendar event with alert tracking
type Event interface {
	GetUID() string
	GetSummary() string
	GetDescription() string
	GetLocation() string
	GetStartTime() time.Time
	GetEndTime() time.Time
	GetTimezone() *time.Location
	
	// Self-contained methods - events know their own alerts
	OccursOn(date time.Time) bool                    // Enhanced: considers all alerts
	OccurrencesWithin(start, end time.Time) []Occurrence // Uses event's complete alert set
	OccurredWithin(start, end time.Time) []time.Time // Raw event occurrence times
	
	// Alert management - no external parameters needed
	GetAllAlerts() []Alert                           // Complete merged alert set
	GetIntrinsicAlerts() []Alert                     // Event's VALARM alerts only
	GetAutomaticAlerts() []Alert                     // Config-based alerts only
	
	// Alert state tracking
	GetAlertState(alertOffset time.Duration) AlertState
	SetAlertState(alertOffset time.Duration, state AlertState)
}

// CalendarEvent implements the Event interface
type CalendarEvent struct {
	UID         string
	Summary     string
	Description string
	Location    string
	StartTime   time.Time
	EndTime     time.Time
	Timezone    *time.Location
	Recurrence  recurrence.Recurrence // Recurrence rule implementation
	ExDates     []time.Time // Exception dates
	
	// Calendar context and alerts
	Calendar        *Calendar // Pointer to shared calendar entity
	IntrinsicAlerts []Alert   // VALARM-based alerts from ICS
	
	// Alert state tracking per offset
	alertStates map[time.Duration]AlertState
	mutex       sync.RWMutex
}

// NewCalendarEvent creates a new calendar event with calendar context
func NewCalendarEvent(uid, summary, description, location string, 
	startTime, endTime time.Time, timezone *time.Location, rec recurrence.Recurrence,
	calendar *Calendar, intrinsicAlerts []Alert) *CalendarEvent {
	
	return &CalendarEvent{
		UID:             uid,
		Summary:         summary,
		Description:     description,
		Location:        location,
		StartTime:       startTime,
		EndTime:         endTime,
		Timezone:        timezone,
		Recurrence:      rec,
		ExDates:         make([]time.Time, 0),
		Calendar:        calendar,
		IntrinsicAlerts: intrinsicAlerts,
		alertStates:     make(map[time.Duration]AlertState),
	}
}

// NewCalendarEventFromRRule creates a new calendar event with RRULE string
func NewCalendarEventFromRRule(uid, summary, description, location string, 
	startTime, endTime time.Time, timezone *time.Location, rrule string,
	calendar *Calendar, intrinsicAlerts []Alert) (*CalendarEvent, error) {
	
	rec, err := recurrence.ParseRRule(rrule)
	if err != nil {
		return nil, err
	}
	
	return NewCalendarEvent(uid, summary, description, location, startTime, endTime, timezone, rec, calendar, intrinsicAlerts), nil
}

// GetUID returns the unique identifier of the event
func (e *CalendarEvent) GetUID() string {
	return e.UID
}

// GetSummary returns the event summary/title
func (e *CalendarEvent) GetSummary() string {
	return e.Summary
}

// GetDescription returns the event description
func (e *CalendarEvent) GetDescription() string {
	return e.Description
}

// GetLocation returns the event location
func (e *CalendarEvent) GetLocation() string {
	return e.Location
}

// GetStartTime returns the event start time
func (e *CalendarEvent) GetStartTime() time.Time {
	return e.StartTime
}

// GetEndTime returns the event end time
func (e *CalendarEvent) GetEndTime() time.Time {
	return e.EndTime
}

// GetTimezone returns the event timezone
func (e *CalendarEvent) GetTimezone() *time.Location {
	if e.Timezone != nil {
		return e.Timezone
	}
	return time.UTC
}

// GetCalendar returns the event's associated calendar
func (e *CalendarEvent) GetCalendar() *Calendar {
	return e.Calendar
}

// GetAllAlerts returns the complete merged alert set (VALARM + config)
func (e *CalendarEvent) GetAllAlerts() []Alert {
	var allAlerts []Alert
	
	// Add intrinsic VALARM alerts
	allAlerts = append(allAlerts, e.IntrinsicAlerts...)
	
	// Add automatic alerts from calendar (if calendar is set)
	if e.Calendar != nil {
		automaticAlerts := e.Calendar.GetAutomaticAlerts()
		allAlerts = append(allAlerts, automaticAlerts...)
	}
	
	// VALARM alerts take precedence over config alerts with same offset
	return DeduplicateAlerts(allAlerts)
}

// GetIntrinsicAlerts returns the event's VALARM alerts only
func (e *CalendarEvent) GetIntrinsicAlerts() []Alert {
	// Return a copy to prevent external modification
	alerts := make([]Alert, len(e.IntrinsicAlerts))
	copy(alerts, e.IntrinsicAlerts)
	return alerts
}

// GetAutomaticAlerts returns the config-based alerts from the calendar
func (e *CalendarEvent) GetAutomaticAlerts() []Alert {
	if e.Calendar != nil {
		return e.Calendar.GetAutomaticAlerts()
	}
	return []Alert{}
}

// OccursOn checks if the event occurs on a specific date (enhanced: considers alert days)
func (e *CalendarEvent) OccursOn(date time.Time) bool {
	// Check if the event itself occurs on this date
	if e.eventOccursOn(date) {
		return true
	}
	
	// Check if any alerts for this event fire on this date
	allAlerts := e.GetAllAlerts()
	if len(allAlerts) == 0 {
		return false // No alerts, so only check event occurrence (already done above)
	}
	
	// Get day boundaries
	dayStart := date.Truncate(24 * time.Hour)
	dayEnd := dayStart.Add(24 * time.Hour)
	
	// Use OccurrencesWithin to find alerts firing on this day
	// Look ahead up to maximum alert offset to find events whose alerts fire today
	maxOffset := e.getMaxAlertOffset()
	searchEnd := dayEnd.Add(maxOffset)
	
	occurrences := e.occurrencesWithin(dayStart, searchEnd)
	for _, occ := range occurrences {
		// Check if this occurrence's alert time falls on the target date
		if occ.AlertTime.After(dayStart) && occ.AlertTime.Before(dayEnd) {
			return true
		}
	}
	
	return false
}

// eventOccursOn checks if the event itself occurs on the given date (original logic)
func (e *CalendarEvent) eventOccursOn(date time.Time) bool {
	if e.Recurrence == nil {
		// No recurrence, check only the base occurrence
		eventDate := e.StartTime.In(e.GetTimezone()).Truncate(24 * time.Hour)
		checkDate := date.In(e.GetTimezone()).Truncate(24 * time.Hour)
		return eventDate.Equal(checkDate) && !e.isExceptionDate(e.StartTime)
	}
	
	// Use recurrence logic to check if it occurs on this date
	return e.Recurrence.OccursOn(date, e.StartTime) && !e.isExceptionDate(e.StartTime)
}

// getMaxAlertOffset returns the maximum alert offset for this event
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

// OccurredWithin returns all occurrences of the event within the given time range
func (e *CalendarEvent) OccurredWithin(start, end time.Time) []time.Time {
	if e.Recurrence == nil {
		// No recurrence, check only the base occurrence
		var occurrences []time.Time
		
		// Ensure start and end are in the event's timezone for proper comparison
		eventTz := e.GetTimezone()
		startInTz := start.In(eventTz)
		endInTz := end.In(eventTz)
		eventStartInTz := e.StartTime.In(eventTz)
		
		// Check if the original occurrence falls within the range (inclusive bounds)
		if (eventStartInTz.After(startInTz) || eventStartInTz.Equal(startInTz)) &&
			(eventStartInTz.Before(endInTz) || eventStartInTz.Equal(endInTz)) &&
			!e.isExceptionDate(e.StartTime) {
			occurrences = append(occurrences, e.StartTime)
		}
		
		return occurrences
	}
	
	// Use recurrence logic to find all occurrences within the range
	return e.Recurrence.OccurredWithin(start, end, e.StartTime, e.ExDates)
}

// NextOccurrence returns the next occurrence of the event after the given time
func (e *CalendarEvent) NextOccurrence(after time.Time) *time.Time {
	if e.Recurrence == nil {
		// No recurrence, check only the base occurrence
		if e.StartTime.After(after) && !e.isExceptionDate(e.StartTime) {
			return &e.StartTime
		}
		return nil
	}
	
	// Use recurrence logic to find the next occurrence
	return e.Recurrence.NextOccurrence(after, e.StartTime, e.ExDates)
}

// OccurrencesWithin returns all alert occurrences of the event within the given time range
func (e *CalendarEvent) OccurrencesWithin(start, end time.Time) []Occurrence {
	return e.occurrencesWithin(start, end)
}

// occurrencesWithin is the internal implementation that generates alert occurrences
func (e *CalendarEvent) occurrencesWithin(start, end time.Time) []Occurrence {
	var occurrences []Occurrence
	
	// Get all alerts for this event (VALARM + config)
	allAlerts := e.GetAllAlerts()
	if len(allAlerts) == 0 {
		return occurrences // No alerts configured
	}
	
	// Get maximum alert offset to extend search range
	maxOffset := e.getMaxAlertOffset()
	
	// Extend search range to find events whose alerts might fall in our target range
	searchStart := start
	searchEnd := end.Add(maxOffset)
	
	// Get all event occurrences in the extended range using the old method
	eventOccurrences := e.getEventOccurrences(searchStart, searchEnd)
	
	// For each event occurrence, generate alert occurrences
	for _, eventTime := range eventOccurrences {
		for _, alert := range allAlerts {
			alertTime := eventTime.Add(-alert.Offset)
			
			// Check if this alert time falls within our target range [start, end]
			if alertTime.After(start) && (alertTime.Before(end) || alertTime.Equal(end)) {
				// Determine if this alert is late (should have fired more than 1 minute before 'end'/now)
				// Since we check every minute, anything more than ~1 minute overdue is "late"
				minuteThreshold := time.Minute
				isLate := alertTime.Before(end.Add(-minuteThreshold))
				
				occurrence := Occurrence{
					EventTime: eventTime,
					AlertTime: alertTime,
					Offset:    alert.Offset,
					Important: alert.Important,
					Late:      isLate,
					EventData: e,
				}
				occurrences = append(occurrences, occurrence)
			}
		}
	}
	
	return occurrences
}

// getEventOccurrences returns raw event times (extracted from old OccurredWithin logic)
func (e *CalendarEvent) getEventOccurrences(start, end time.Time) []time.Time {
	if e.Recurrence == nil {
		// No recurrence, check only the base occurrence
		var occurrences []time.Time
		
		// Ensure start and end are in the event's timezone for proper comparison
		eventTz := e.GetTimezone()
		startInTz := start.In(eventTz)
		endInTz := end.In(eventTz)
		eventStartInTz := e.StartTime.In(eventTz)
		
		// Check if the original occurrence falls within the range (inclusive bounds)
		if (eventStartInTz.After(startInTz) || eventStartInTz.Equal(startInTz)) &&
			(eventStartInTz.Before(endInTz) || eventStartInTz.Equal(endInTz)) &&
			!e.isExceptionDate(e.StartTime) {
			occurrences = append(occurrences, e.StartTime)
		}
		
		return occurrences
	}
	
	// Use recurrence logic to find all occurrences within the range
	return e.Recurrence.OccurredWithin(start, end, e.StartTime, e.ExDates)
}


// GetAlertState returns the current alert state for a specific offset
func (e *CalendarEvent) GetAlertState(alertOffset time.Duration) AlertState {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	if state, exists := e.alertStates[alertOffset]; exists {
		return state
	}
	return AlertPending
}

// SetAlertState sets the alert state for a specific offset
func (e *CalendarEvent) SetAlertState(alertOffset time.Duration, state AlertState) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	
	e.alertStates[alertOffset] = state
}

// AddExceptionDate adds a date to the exception list
func (e *CalendarEvent) AddExceptionDate(date time.Time) {
	e.ExDates = append(e.ExDates, date)
}

// isExceptionDate checks if a date is in the exception list
func (e *CalendarEvent) isExceptionDate(date time.Time) bool {
	for _, exDate := range e.ExDates {
		if exDate.Equal(date) {
			return true
		}
	}
	return false
}

// ResetAlertStates resets all alert states (useful for recurring events on new occurrences)
func (e *CalendarEvent) ResetAlertStates() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	
	e.alertStates = make(map[time.Duration]AlertState)
}