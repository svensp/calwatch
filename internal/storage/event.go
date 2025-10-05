package storage

import (
	"sync"
	"time"
	
	"calwatch/internal/config"
	"calwatch/internal/recurrence"
)

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
	OccursOn(date time.Time) bool
	OccurredWithin(start, end time.Time) []time.Time
	OccurrencesWithin(start, end time.Time, alerts []config.AlertConfig) []Occurrence
	NextOccurrence(after time.Time) *time.Time
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
	
	// Alert state tracking per offset
	alertStates map[time.Duration]AlertState
	mutex       sync.RWMutex
}

// NewCalendarEvent creates a new calendar event
func NewCalendarEvent(uid, summary, description, location string, 
	startTime, endTime time.Time, timezone *time.Location, rec recurrence.Recurrence) *CalendarEvent {
	
	return &CalendarEvent{
		UID:         uid,
		Summary:     summary,
		Description: description,
		Location:    location,
		StartTime:   startTime,
		EndTime:     endTime,
		Timezone:    timezone,
		Recurrence:  rec,
		ExDates:     make([]time.Time, 0),
		alertStates: make(map[time.Duration]AlertState),
	}
}

// NewCalendarEventFromRRule creates a new calendar event with RRULE string
func NewCalendarEventFromRRule(uid, summary, description, location string, 
	startTime, endTime time.Time, timezone *time.Location, rrule string) (*CalendarEvent, error) {
	
	rec, err := recurrence.ParseRRule(rrule)
	if err != nil {
		return nil, err
	}
	
	return NewCalendarEvent(uid, summary, description, location, startTime, endTime, timezone, rec), nil
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

// OccursOn checks if the event occurs on a specific date
func (e *CalendarEvent) OccursOn(date time.Time) bool {
	if e.Recurrence == nil {
		// No recurrence, check only the base occurrence
		eventDate := e.StartTime.In(e.GetTimezone()).Truncate(24 * time.Hour)
		checkDate := date.In(e.GetTimezone()).Truncate(24 * time.Hour)
		return eventDate.Equal(checkDate) && !e.isExceptionDate(e.StartTime)
	}
	
	// Use recurrence logic to check if it occurs on this date
	return e.Recurrence.OccursOn(date, e.StartTime) && !e.isExceptionDate(e.StartTime)
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
func (e *CalendarEvent) OccurrencesWithin(start, end time.Time, alerts []config.AlertConfig) []Occurrence {
	var occurrences []Occurrence
	
	// Get all event occurrences within an extended time range to catch alerts
	// We need to look beyond 'end' to find events whose alerts fall within [start, end]
	maxOffset := time.Duration(0)
	for _, alert := range alerts {
		if alertDuration, err := alert.Duration(); err == nil {
			if alertDuration > maxOffset {
				maxOffset = alertDuration
			}
		}
	}
	
	// Extend search range to find events whose alerts might fall in our target range
	searchStart := start
	searchEnd := end.Add(maxOffset)
	
	// Get all event occurrences in the extended range
	eventOccurrences := e.OccurredWithin(searchStart, searchEnd)
	
	// For each event occurrence, generate alert occurrences
	for _, eventTime := range eventOccurrences {
		for _, alertConfig := range alerts {
			alertOffset, err := alertConfig.Duration()
			if err != nil {
				continue // Skip invalid alert configs
			}
			
			alertTime := eventTime.Add(-alertOffset)
			
			// Check if this alert time falls within our target range [start, end]
			if alertTime.After(start) && (alertTime.Before(end) || alertTime.Equal(end)) {
				// Determine if this alert is late (should have fired more than 1 minute before 'end'/now)
				// Since we check every minute, anything more than ~1 minute overdue is "late"
				minuteThreshold := time.Minute
				isLate := alertTime.Before(end.Add(-minuteThreshold))
				
				occurrence := Occurrence{
					EventTime: eventTime,
					AlertTime: alertTime,
					Offset:    alertOffset,
					Important: alertConfig.Important,
					Late:      isLate,
					EventData: e,
				}
				occurrences = append(occurrences, occurrence)
			}
		}
	}
	
	return occurrences
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