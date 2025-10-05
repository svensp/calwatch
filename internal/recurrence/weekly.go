package recurrence

import (
	"fmt"
	"strings"
	"time"
)

// WeeklyRecurrence represents a weekly recurring event
type WeeklyRecurrence struct {
	Interval int         // Every N weeks (default 1)
	ByDay    []time.Weekday // Days of the week (if empty, uses base event day)
	Until    *time.Time   // End date (optional)
	Count    *int        // Number of occurrences (optional)
}

func NewWeeklyRecurrence(interval int, byDay []time.Weekday, until *time.Time, count *int) *WeeklyRecurrence {
	if interval <= 0 {
		interval = 1
	}
	return &WeeklyRecurrence{
		Interval: interval,
		ByDay:    byDay,
		Until:    until,
		Count:    count,
	}
}

func (wr *WeeklyRecurrence) OccursOn(date time.Time, baseTime time.Time) bool {
	checkDate := date.Truncate(24 * time.Hour)
	baseDate := baseTime.Truncate(24 * time.Hour)
	
	// Event can't occur before the base date
	if checkDate.Before(baseDate) {
		return false
	}
	
	// Get the target weekdays (if empty, use the base event's weekday)
	targetWeekdays := wr.ByDay
	if len(targetWeekdays) == 0 {
		targetWeekdays = []time.Weekday{baseTime.Weekday()}
	}
	
	// Check if the check date is on one of the target weekdays
	checkWeekday := checkDate.Weekday()
	validWeekday := false
	for _, weekday := range targetWeekdays {
		if checkWeekday == weekday {
			validWeekday = true
			break
		}
	}
	if !validWeekday {
		return false
	}
	
	// Check if this date falls on the correct week interval
	weeksDiff := int(checkDate.Sub(baseDate).Hours() / (24 * 7))
	if weeksDiff%wr.Interval != 0 {
		return false
	}
	
	// Check until date if specified
	if wr.Until != nil && checkDate.After(*wr.Until) {
		return false
	}
	
	// Check count if specified
	if wr.Count != nil {
		// Count total occurrences up to this point
		totalOccurrences := wr.countOccurrencesUntil(checkDate, baseTime)
		if totalOccurrences > *wr.Count {
			return false
		}
	}
	
	return true
}

func (wr *WeeklyRecurrence) OccurredWithin(start, end time.Time, baseTime time.Time, exDates []time.Time) []time.Time {
	var occurrences []time.Time
	
	// Get the target weekdays
	targetWeekdays := wr.ByDay
	if len(targetWeekdays) == 0 {
		targetWeekdays = []time.Weekday{baseTime.Weekday()}
	}
	
	// Start from the beginning of the base week or start time, whichever is later
	baseDate := baseTime.Truncate(24 * time.Hour)
	startDate := start.Truncate(24 * time.Hour)
	
	current := baseDate
	if startDate.After(baseDate) {
		// Fast forward to the first week in range
		weeksDiff := int(startDate.Sub(baseDate).Hours() / (24 * 7))
		intervalStart := (weeksDiff / wr.Interval) * wr.Interval
		current = baseDate.AddDate(0, 0, intervalStart*7)
	}
	
	// Get the start of the current week (Monday)
	weekStart := getWeekStart(current)
	occurrenceCount := 0
	
	for {
		// Check each target weekday in this week
		for _, weekday := range targetWeekdays {
			dayOffset := int(weekday - time.Monday)
			if dayOffset < 0 {
				dayOffset += 7 // Handle Sunday
			}
			
			candidate := weekStart.AddDate(0, 0, dayOffset)
			
			// Must be within our time range
			if candidate.Before(start) || candidate.After(end) {
				continue
			}
			
			// Must be at or after the base time
			if candidate.Before(baseDate) {
				continue
			}
			
			// Check until date if specified
			if wr.Until != nil && candidate.After(*wr.Until) {
				continue
			}
			
			// Check count if specified
			if wr.Count != nil {
				totalOccurrences := wr.countOccurrencesUntil(candidate, baseTime)
				if totalOccurrences > *wr.Count {
					continue
				}
			}
			
			// Add if not an exception date
			if !isExceptionDate(candidate, exDates) {
				occurrences = append(occurrences, candidate)
			}
		}
		
		// Move to next interval week
		weekStart = weekStart.AddDate(0, 0, wr.Interval*7)
		occurrenceCount++
		
		// Break if we've moved past the end time
		if weekStart.After(end.AddDate(0, 0, 7)) {
			break
		}
		
		// Safety check to prevent infinite loops
		if occurrenceCount > 1000 {
			break
		}
	}
	
	return occurrences
}

func (wr *WeeklyRecurrence) NextOccurrence(after time.Time, baseTime time.Time, exDates []time.Time) *time.Time {
	// Get target weekdays
	targetWeekdays := wr.ByDay
	if len(targetWeekdays) == 0 {
		targetWeekdays = []time.Weekday{baseTime.Weekday()}
	}
	
	baseDate := baseTime.Truncate(24 * time.Hour)
	afterDate := after.Truncate(24 * time.Hour)
	
	// Start from the base week or the week containing 'after', whichever is later
	current := baseDate
	if afterDate.After(baseDate) {
		// Fast forward to the appropriate week
		weeksDiff := int(afterDate.Sub(baseDate).Hours() / (24 * 7))
		intervalStart := (weeksDiff / wr.Interval) * wr.Interval
		current = baseDate.AddDate(0, 0, intervalStart*7)
	}
	
	weekStart := getWeekStart(current)
	occurrenceCount := 0
	
	for {
		// Check each target weekday in this week
		for _, weekday := range targetWeekdays {
			dayOffset := int(weekday - time.Monday)
			if dayOffset < 0 {
				dayOffset += 7 // Handle Sunday
			}
			
			candidate := weekStart.AddDate(0, 0, dayOffset)
			
			// Must be after the 'after' time
			if !candidate.After(after) {
				continue
			}
			
			// Must be at or after the base time
			if candidate.Before(baseDate) {
				continue
			}
			
			// Check until date if specified
			if wr.Until != nil && candidate.After(*wr.Until) {
				return nil
			}
			
			// Check count if specified
			if wr.Count != nil {
				totalOccurrences := wr.countOccurrencesUntil(candidate, baseTime)
				if totalOccurrences > *wr.Count {
					return nil
				}
			}
			
			// Return if not an exception date
			if !isExceptionDate(candidate, exDates) {
				return &candidate
			}
		}
		
		// Move to next interval week
		weekStart = weekStart.AddDate(0, 0, wr.Interval*7)
		occurrenceCount++
		
		// Safety check to prevent infinite loops
		if occurrenceCount > 1000 {
			return nil
		}
	}
}

func (wr *WeeklyRecurrence) String() string {
	weekStr := "weekly"
	if wr.Interval != 1 {
		weekStr = fmt.Sprintf("every %d weeks", wr.Interval)
	}
	
	if len(wr.ByDay) > 0 {
		var days []string
		for _, day := range wr.ByDay {
			days = append(days, day.String()[:3])
		}
		return fmt.Sprintf("%s on %s", weekStr, strings.Join(days, ", "))
	}
	
	return weekStr
}

// Helper function to get the start of the week (Monday)
func getWeekStart(date time.Time) time.Time {
	weekday := date.Weekday()
	daysSinceMonday := int(weekday - time.Monday)
	if daysSinceMonday < 0 {
		daysSinceMonday += 7 // Handle Sunday
	}
	return date.AddDate(0, 0, -daysSinceMonday)
}

// Helper function to count occurrences up to a specific date
func (wr *WeeklyRecurrence) countOccurrencesUntil(untilDate time.Time, baseTime time.Time) int {
	count := 0
	targetWeekdays := wr.ByDay
	if len(targetWeekdays) == 0 {
		targetWeekdays = []time.Weekday{baseTime.Weekday()}
	}
	
	baseDate := baseTime.Truncate(24 * time.Hour)
	weekStart := getWeekStart(baseDate)
	
	for {
		for _, weekday := range targetWeekdays {
			dayOffset := int(weekday - time.Monday)
			if dayOffset < 0 {
				dayOffset += 7
			}
			
			candidate := weekStart.AddDate(0, 0, dayOffset)
			
			if candidate.After(untilDate) {
				return count
			}
			
			if candidate.Equal(baseDate) || candidate.After(baseDate) {
				count++
			}
		}
		
		weekStart = weekStart.AddDate(0, 0, wr.Interval*7)
		
		// Safety check
		if weekStart.After(untilDate.AddDate(1, 0, 0)) {
			break
		}
	}
	
	return count
}