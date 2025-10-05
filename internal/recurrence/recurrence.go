package recurrence

import (
	"time"
)

// Recurrence defines the interface for handling recurring events
type Recurrence interface {
	// OccursOn checks if the event occurs on a specific date
	OccursOn(date time.Time, baseTime time.Time) bool
	
	// OccurredWithin returns all occurrences within the given time range
	OccurredWithin(start, end time.Time, baseTime time.Time, exDates []time.Time) []time.Time
	
	// NextOccurrence finds the next occurrence after the given time
	NextOccurrence(after time.Time, baseTime time.Time, exDates []time.Time) *time.Time
	
	// String returns a human-readable description of the recurrence
	String() string
}

// NoRecurrence represents a non-recurring event
type NoRecurrence struct{}

func (nr *NoRecurrence) OccursOn(date time.Time, baseTime time.Time) bool {
	baseDate := baseTime.Truncate(24 * time.Hour)
	checkDate := date.Truncate(24 * time.Hour)
	return baseDate.Equal(checkDate)
}

func (nr *NoRecurrence) OccurredWithin(start, end time.Time, baseTime time.Time, exDates []time.Time) []time.Time {
	// Check if base occurrence falls within range and is not an exception
	if (baseTime.After(start) || baseTime.Equal(start)) && 
		(baseTime.Before(end) || baseTime.Equal(end)) &&
		!isExceptionDate(baseTime, exDates) {
		return []time.Time{baseTime}
	}
	return []time.Time{}
}

func (nr *NoRecurrence) NextOccurrence(after time.Time, baseTime time.Time, exDates []time.Time) *time.Time {
	if baseTime.After(after) && !isExceptionDate(baseTime, exDates) {
		return &baseTime
	}
	return nil
}

func (nr *NoRecurrence) String() string {
	return "No recurrence"
}

// isExceptionDate checks if a given time is in the exception dates list
func isExceptionDate(checkTime time.Time, exDates []time.Time) bool {
	for _, exDate := range exDates {
		if checkTime.Equal(exDate) {
			return true
		}
	}
	return false
}