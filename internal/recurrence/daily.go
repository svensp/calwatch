package recurrence

import (
	"fmt"
	"time"
)

// DailyRecurrence represents a daily recurring event
type DailyRecurrence struct {
	Interval int       // Every N days (default 1)
	Until    *time.Time // End date (optional)
	Count    *int      // Number of occurrences (optional)
}

func NewDailyRecurrence(interval int, until *time.Time, count *int) *DailyRecurrence {
	if interval <= 0 {
		interval = 1
	}
	return &DailyRecurrence{
		Interval: interval,
		Until:    until,
		Count:    count,
	}
}

func (dr *DailyRecurrence) OccursOn(date time.Time, baseTime time.Time) bool {
	checkDate := date.Truncate(24 * time.Hour)
	baseDate := baseTime.Truncate(24 * time.Hour)
	
	// Event can't occur before the base date
	if checkDate.Before(baseDate) {
		return false
	}
	
	// Check if this date falls on the interval
	daysDiff := int(checkDate.Sub(baseDate).Hours() / 24)
	if daysDiff%dr.Interval != 0 {
		return false
	}
	
	// Check until date if specified (UNTIL is inclusive per RFC 5545)
	if dr.Until != nil && checkDate.After(*dr.Until) {
		return false
	}
	
	// Check count if specified
	if dr.Count != nil {
		occurrenceNumber := (daysDiff / dr.Interval) + 1
		if occurrenceNumber > *dr.Count {
			return false
		}
	}
	
	return true
}

func (dr *DailyRecurrence) OccurredWithin(start, end time.Time, baseTime time.Time, exDates []time.Time) []time.Time {
	var occurrences []time.Time
	
	// Start from the base time or the start of the range, whichever is later
	current := baseTime
	if start.After(baseTime) {
		// Fast forward to the first occurrence within the range
		daysDiff := int(start.Sub(baseTime).Hours() / 24)
		intervalStart := (daysDiff / dr.Interval) * dr.Interval
		current = baseTime.AddDate(0, 0, intervalStart)
		
		// Make sure we're at or after the start time
		for current.Before(start) {
			current = current.AddDate(0, 0, dr.Interval)
		}
	}
	
	occurrenceCount := 0
	
	for {
		// Check if we've exceeded the end time
		if current.After(end) {
			break
		}
		
		// Check until date if specified
		if dr.Until != nil && current.After(*dr.Until) {
			break
		}
		
		// Check count if specified
		if dr.Count != nil {
			daysDiff := int(current.Sub(baseTime).Hours() / 24)
			occurrenceNumber := (daysDiff / dr.Interval) + 1
			if occurrenceNumber > *dr.Count {
				break
			}
		}
		
		// Add if not an exception date
		if !isExceptionDate(current, exDates) {
			occurrences = append(occurrences, current)
		}
		
		// Move to next occurrence
		current = current.AddDate(0, 0, dr.Interval)
		occurrenceCount++
		
		// Safety check to prevent infinite loops
		if occurrenceCount > 10000 {
			break
		}
	}
	
	return occurrences
}

func (dr *DailyRecurrence) NextOccurrence(after time.Time, baseTime time.Time, exDates []time.Time) *time.Time {
	// Start checking from the day after 'after'
	current := baseTime
	
	if after.After(baseTime) {
		// Fast forward to the first potential occurrence after 'after'
		daysDiff := int(after.Sub(baseTime).Hours() / 24)
		intervalStart := ((daysDiff / dr.Interval) + 1) * dr.Interval
		current = baseTime.AddDate(0, 0, intervalStart)
	}
	
	occurrenceCount := 0
	
	for {
		// Must be after the 'after' time
		if !current.After(after) {
			current = current.AddDate(0, 0, dr.Interval)
			continue
		}
		
		// Check until date if specified
		if dr.Until != nil && current.After(*dr.Until) {
			return nil
		}
		
		// Check count if specified
		if dr.Count != nil {
			daysDiff := int(current.Sub(baseTime).Hours() / 24)
			occurrenceNumber := (daysDiff / dr.Interval) + 1
			if occurrenceNumber > *dr.Count {
				return nil
			}
		}
		
		// Return if not an exception date
		if !isExceptionDate(current, exDates) {
			return &current
		}
		
		// Move to next occurrence
		current = current.AddDate(0, 0, dr.Interval)
		occurrenceCount++
		
		// Safety check to prevent infinite loops
		if occurrenceCount > 1000 {
			return nil
		}
	}
}

func (dr *DailyRecurrence) String() string {
	if dr.Interval == 1 {
		return "Daily"
	}
	return fmt.Sprintf("Every %d days", dr.Interval)
}