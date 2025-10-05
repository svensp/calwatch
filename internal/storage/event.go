package storage

import (
	"sync"
	"time"
	
	"calwatch/internal/recurrence"
)

// AlertState represents the state of an alert for an event
type AlertState int

const (
	AlertPending AlertState = iota
	AlertSent
	AlertSnoozed
)

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
	NextOccurrence(after time.Time) *time.Time
	ShouldAlert(lastTick, now time.Time, alertOffset time.Duration) bool
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

// ShouldAlert determines if an alert should be sent for this event within the given time range
func (e *CalendarEvent) ShouldAlert(lastTick, now time.Time, alertOffset time.Duration) bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	// Check if alert was already sent
	if state, exists := e.alertStates[alertOffset]; exists && state == AlertSent {
		return false
	}
	
	// We need to check a broader range to find events whose alert times fall within [lastTick, now]
	// If alert offset is X, we need to look for events that occur up to X time after 'now'
	searchStart := lastTick
	searchEnd := now.Add(alertOffset)
	
	// Find all occurrences within the expanded search range
	occurrences := e.OccurredWithin(searchStart, searchEnd)
	if len(occurrences) == 0 {
		return false
	}
	
	// Check if any occurrence has its alert time within our target range [lastTick, now]
	for _, occurrence := range occurrences {
		alertTime := occurrence.Add(-alertOffset)
		// Alert should fire if: lastTick < alertTime <= now
		if alertTime.After(lastTick) && (now.After(alertTime) || now.Equal(alertTime)) {
			return true
		}
	}
	
	return false
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