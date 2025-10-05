package recurrence

import (
	"fmt"
	"time"
)

// MonthlyRecurrence represents a monthly recurring event
type MonthlyRecurrence struct {
	Interval   int         // Every N months (default 1)
	ByMonthDay []int       // Days of the month (1-31, if empty uses base event day)
	Until      *time.Time  // End date (optional)
	Count      *int        // Number of occurrences (optional)
}

func NewMonthlyRecurrence(interval int, byMonthDay []int, until *time.Time, count *int) *MonthlyRecurrence {
	if interval <= 0 {
		interval = 1
	}
	return &MonthlyRecurrence{
		Interval:   interval,
		ByMonthDay: byMonthDay,
		Until:      until,
		Count:      count,
	}
}

func (mr *MonthlyRecurrence) OccursOn(date time.Time, baseTime time.Time) bool {
	checkDate := date.Truncate(24 * time.Hour)
	baseDate := baseTime.Truncate(24 * time.Hour)
	
	// Event can't occur before the base date
	if checkDate.Before(baseDate) {
		return false
	}
	
	// Get target month days
	targetDays := mr.ByMonthDay
	if len(targetDays) == 0 {
		targetDays = []int{baseTime.Day()}
	}
	
	// Check if the check date is on one of the target days
	validDay := false
	for _, day := range targetDays {
		// Handle last day of month (negative values)
		actualDay := day
		if day < 0 {
			daysInMonth := getDaysInMonth(checkDate.Year(), checkDate.Month())
			actualDay = daysInMonth + day + 1
		}
		
		// Handle cases where the target day doesn't exist in this month
		if actualDay > getDaysInMonth(checkDate.Year(), checkDate.Month()) {
			actualDay = getDaysInMonth(checkDate.Year(), checkDate.Month())
		}
		
		if checkDate.Day() == actualDay {
			validDay = true
			break
		}
	}
	if !validDay {
		return false
	}
	
	// Check if this date falls on the correct month interval
	monthsDiff := getMonthsDiff(baseDate, checkDate)
	if monthsDiff%mr.Interval != 0 {
		return false
	}
	
	// Check until date if specified
	if mr.Until != nil && checkDate.After(*mr.Until) {
		return false
	}
	
	// Check count if specified
	if mr.Count != nil {
		totalOccurrences := mr.countOccurrencesUntil(checkDate, baseTime)
		if totalOccurrences > *mr.Count {
			return false
		}
	}
	
	return true
}

func (mr *MonthlyRecurrence) OccurredWithin(start, end time.Time, baseTime time.Time, exDates []time.Time) []time.Time {
	var occurrences []time.Time
	
	targetDays := mr.ByMonthDay
	if len(targetDays) == 0 {
		targetDays = []int{baseTime.Day()}
	}
	
	baseDate := baseTime.Truncate(24 * time.Hour)
	startDate := start.Truncate(24 * time.Hour)
	endDate := end.Truncate(24 * time.Hour)
	
	// Start from the base month or the month containing start, whichever is later
	current := time.Date(baseDate.Year(), baseDate.Month(), 1, 0, 0, 0, 0, baseDate.Location())
	if startDate.After(baseDate) {
		monthsDiff := getMonthsDiff(baseDate, startDate)
		intervalStart := (monthsDiff / mr.Interval) * mr.Interval
		current = baseDate.AddDate(0, intervalStart, 0)
		current = time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, current.Location())
	}
	
	occurrenceCount := 0
	
	for {
		// Check each target day in this month
		for _, day := range targetDays {
			actualDay := day
			if day < 0 {
				daysInMonth := getDaysInMonth(current.Year(), current.Month())
				actualDay = daysInMonth + day + 1
			}
			
			// Handle cases where the target day doesn't exist in this month
			if actualDay > getDaysInMonth(current.Year(), current.Month()) {
				actualDay = getDaysInMonth(current.Year(), current.Month())
			}
			
			if actualDay < 1 {
				continue
			}
			
			candidate := time.Date(current.Year(), current.Month(), actualDay, 
				baseTime.Hour(), baseTime.Minute(), baseTime.Second(), 0, baseTime.Location())
			
			// Must be within our time range
			if candidate.Before(start) || candidate.After(end) {
				continue
			}
			
			// Must be at or after the base time
			if candidate.Before(baseDate) {
				continue
			}
			
			// Check until date if specified
			if mr.Until != nil && candidate.After(*mr.Until) {
				continue
			}
			
			// Check count if specified
			if mr.Count != nil {
				totalOccurrences := mr.countOccurrencesUntil(candidate, baseTime)
				if totalOccurrences > *mr.Count {
					continue
				}
			}
			
			// Add if not an exception date
			if !isExceptionDate(candidate, exDates) {
				occurrences = append(occurrences, candidate)
			}
		}
		
		// Move to next interval month
		current = current.AddDate(0, mr.Interval, 0)
		occurrenceCount++
		
		// Break if we've moved past the end time
		if current.After(endDate.AddDate(0, 1, 0)) {
			break
		}
		
		// Safety check to prevent infinite loops
		if occurrenceCount > 1000 {
			break
		}
	}
	
	return occurrences
}

func (mr *MonthlyRecurrence) NextOccurrence(after time.Time, baseTime time.Time, exDates []time.Time) *time.Time {
	targetDays := mr.ByMonthDay
	if len(targetDays) == 0 {
		targetDays = []int{baseTime.Day()}
	}
	
	baseDate := baseTime.Truncate(24 * time.Hour)
	afterDate := after.Truncate(24 * time.Hour)
	
	// Start from the base month or the month containing 'after', whichever is later
	current := time.Date(baseDate.Year(), baseDate.Month(), 1, 0, 0, 0, 0, baseDate.Location())
	if afterDate.After(baseDate) {
		monthsDiff := getMonthsDiff(baseDate, afterDate)
		intervalStart := (monthsDiff / mr.Interval) * mr.Interval
		current = baseDate.AddDate(0, intervalStart, 0)
		current = time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, current.Location())
	}
	
	occurrenceCount := 0
	
	for {
		// Check each target day in this month
		for _, day := range targetDays {
			actualDay := day
			if day < 0 {
				daysInMonth := getDaysInMonth(current.Year(), current.Month())
				actualDay = daysInMonth + day + 1
			}
			
			// Handle cases where the target day doesn't exist in this month
			if actualDay > getDaysInMonth(current.Year(), current.Month()) {
				actualDay = getDaysInMonth(current.Year(), current.Month())
			}
			
			if actualDay < 1 {
				continue
			}
			
			candidate := time.Date(current.Year(), current.Month(), actualDay,
				baseTime.Hour(), baseTime.Minute(), baseTime.Second(), 0, baseTime.Location())
			
			// Must be after the 'after' time
			if !candidate.After(after) {
				continue
			}
			
			// Must be at or after the base time
			if candidate.Before(baseDate) {
				continue
			}
			
			// Check until date if specified
			if mr.Until != nil && candidate.After(*mr.Until) {
				return nil
			}
			
			// Check count if specified
			if mr.Count != nil {
				totalOccurrences := mr.countOccurrencesUntil(candidate, baseTime)
				if totalOccurrences > *mr.Count {
					return nil
				}
			}
			
			// Return if not an exception date
			if !isExceptionDate(candidate, exDates) {
				return &candidate
			}
		}
		
		// Move to next interval month
		current = current.AddDate(0, mr.Interval, 0)
		occurrenceCount++
		
		// Safety check to prevent infinite loops
		if occurrenceCount > 1000 {
			return nil
		}
	}
}

func (mr *MonthlyRecurrence) String() string {
	monthStr := "monthly"
	if mr.Interval != 1 {
		monthStr = fmt.Sprintf("every %d months", mr.Interval)
	}
	
	if len(mr.ByMonthDay) > 0 {
		return fmt.Sprintf("%s on day %v", monthStr, mr.ByMonthDay)
	}
	
	return monthStr
}

// Helper function to get the number of days in a month
func getDaysInMonth(year int, month time.Month) int {
	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	firstOfNextMonth := firstOfMonth.AddDate(0, 1, 0)
	return firstOfNextMonth.AddDate(0, 0, -1).Day()
}

// Helper function to calculate months difference
func getMonthsDiff(start, end time.Time) int {
	years := end.Year() - start.Year()
	months := int(end.Month()) - int(start.Month())
	return years*12 + months
}

// Helper function to count occurrences up to a specific date
func (mr *MonthlyRecurrence) countOccurrencesUntil(untilDate time.Time, baseTime time.Time) int {
	count := 0
	targetDays := mr.ByMonthDay
	if len(targetDays) == 0 {
		targetDays = []int{baseTime.Day()}
	}
	
	baseDate := baseTime.Truncate(24 * time.Hour)
	current := time.Date(baseDate.Year(), baseDate.Month(), 1, 0, 0, 0, 0, baseDate.Location())
	
	for {
		for _, day := range targetDays {
			actualDay := day
			if day < 0 {
				daysInMonth := getDaysInMonth(current.Year(), current.Month())
				actualDay = daysInMonth + day + 1
			}
			
			if actualDay > getDaysInMonth(current.Year(), current.Month()) {
				actualDay = getDaysInMonth(current.Year(), current.Month())
			}
			
			if actualDay < 1 {
				continue
			}
			
			candidate := time.Date(current.Year(), current.Month(), actualDay,
				baseTime.Hour(), baseTime.Minute(), baseTime.Second(), 0, baseTime.Location())
			
			if candidate.After(untilDate) {
				return count
			}
			
			if candidate.Equal(baseDate) || candidate.After(baseDate) {
				count++
			}
		}
		
		current = current.AddDate(0, mr.Interval, 0)
		
		// Safety check
		if current.After(untilDate.AddDate(1, 0, 0)) {
			break
		}
	}
	
	return count
}