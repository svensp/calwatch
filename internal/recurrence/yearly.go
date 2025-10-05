package recurrence

import (
	"fmt"
	"time"
)

// YearlyRecurrence represents a yearly recurring event
type YearlyRecurrence struct {
	Interval   int         // Every N years (default 1)
	ByMonth    []time.Month // Months of the year (if empty uses base event month)
	ByMonthDay []int       // Days of the month (if empty uses base event day)
	Until      *time.Time  // End date (optional)
	Count      *int        // Number of occurrences (optional)
}

func NewYearlyRecurrence(interval int, byMonth []time.Month, byMonthDay []int, until *time.Time, count *int) *YearlyRecurrence {
	if interval <= 0 {
		interval = 1
	}
	return &YearlyRecurrence{
		Interval:   interval,
		ByMonth:    byMonth,
		ByMonthDay: byMonthDay,
		Until:      until,
		Count:      count,
	}
}

func (yr *YearlyRecurrence) OccursOn(date time.Time, baseTime time.Time) bool {
	checkDate := date.Truncate(24 * time.Hour)
	baseDate := baseTime.Truncate(24 * time.Hour)
	
	// Event can't occur before the base date
	if checkDate.Before(baseDate) {
		return false
	}
	
	// Get target months
	targetMonths := yr.ByMonth
	if len(targetMonths) == 0 {
		targetMonths = []time.Month{baseTime.Month()}
	}
	
	// Check if the check date is in one of the target months
	validMonth := false
	for _, month := range targetMonths {
		if checkDate.Month() == month {
			validMonth = true
			break
		}
	}
	if !validMonth {
		return false
	}
	
	// Get target days
	targetDays := yr.ByMonthDay
	if len(targetDays) == 0 {
		targetDays = []int{baseTime.Day()}
	}
	
	// Check if the check date is on one of the target days
	validDay := false
	for _, day := range targetDays {
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
	
	// Check if this date falls on the correct year interval
	yearsDiff := checkDate.Year() - baseDate.Year()
	if yearsDiff%yr.Interval != 0 {
		return false
	}
	
	// Check until date if specified
	if yr.Until != nil && checkDate.After(*yr.Until) {
		return false
	}
	
	// Check count if specified
	if yr.Count != nil {
		totalOccurrences := yr.countOccurrencesUntil(checkDate, baseTime)
		if totalOccurrences > *yr.Count {
			return false
		}
	}
	
	return true
}

func (yr *YearlyRecurrence) OccurredWithin(start, end time.Time, baseTime time.Time, exDates []time.Time) []time.Time {
	var occurrences []time.Time
	
	targetMonths := yr.ByMonth
	if len(targetMonths) == 0 {
		targetMonths = []time.Month{baseTime.Month()}
	}
	
	targetDays := yr.ByMonthDay
	if len(targetDays) == 0 {
		targetDays = []int{baseTime.Day()}
	}
	
	baseDate := baseTime.Truncate(24 * time.Hour)
	startDate := start.Truncate(24 * time.Hour)
	endDate := end.Truncate(24 * time.Hour)
	
	// Start from the base year or the year containing start, whichever is later
	currentYear := baseDate.Year()
	if startDate.After(baseDate) {
		yearsDiff := startDate.Year() - baseDate.Year()
		intervalStart := (yearsDiff / yr.Interval) * yr.Interval
		currentYear = baseDate.Year() + intervalStart
	}
	
	occurrenceCount := 0
	
	for {
		// Check each target month and day in this year
		for _, month := range targetMonths {
			for _, day := range targetDays {
				actualDay := day
				if day < 0 {
					daysInMonth := getDaysInMonth(currentYear, month)
					actualDay = daysInMonth + day + 1
				}
				
				// Handle cases where the target day doesn't exist in this month
				if actualDay > getDaysInMonth(currentYear, month) {
					actualDay = getDaysInMonth(currentYear, month)
				}
				
				if actualDay < 1 {
					continue
				}
				
				candidate := time.Date(currentYear, month, actualDay,
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
				if yr.Until != nil && candidate.After(*yr.Until) {
					continue
				}
				
				// Check count if specified
				if yr.Count != nil {
					totalOccurrences := yr.countOccurrencesUntil(candidate, baseTime)
					if totalOccurrences > *yr.Count {
						continue
					}
				}
				
				// Add if not an exception date
				if !isExceptionDate(candidate, exDates) {
					occurrences = append(occurrences, candidate)
				}
			}
		}
		
		// Move to next interval year
		currentYear += yr.Interval
		occurrenceCount++
		
		// Break if we've moved past the end time
		if currentYear > endDate.Year()+1 {
			break
		}
		
		// Safety check to prevent infinite loops
		if occurrenceCount > 1000 {
			break
		}
	}
	
	return occurrences
}

func (yr *YearlyRecurrence) NextOccurrence(after time.Time, baseTime time.Time, exDates []time.Time) *time.Time {
	targetMonths := yr.ByMonth
	if len(targetMonths) == 0 {
		targetMonths = []time.Month{baseTime.Month()}
	}
	
	targetDays := yr.ByMonthDay
	if len(targetDays) == 0 {
		targetDays = []int{baseTime.Day()}
	}
	
	baseDate := baseTime.Truncate(24 * time.Hour)
	afterDate := after.Truncate(24 * time.Hour)
	
	// Start from the base year or the year containing 'after', whichever is later
	currentYear := baseDate.Year()
	if afterDate.After(baseDate) {
		yearsDiff := afterDate.Year() - baseDate.Year()
		intervalStart := (yearsDiff / yr.Interval) * yr.Interval
		currentYear = baseDate.Year() + intervalStart
	}
	
	occurrenceCount := 0
	
	for {
		// Check each target month and day in this year
		for _, month := range targetMonths {
			for _, day := range targetDays {
				actualDay := day
				if day < 0 {
					daysInMonth := getDaysInMonth(currentYear, month)
					actualDay = daysInMonth + day + 1
				}
				
				// Handle cases where the target day doesn't exist in this month
				if actualDay > getDaysInMonth(currentYear, month) {
					actualDay = getDaysInMonth(currentYear, month)
				}
				
				if actualDay < 1 {
					continue
				}
				
				candidate := time.Date(currentYear, month, actualDay,
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
				if yr.Until != nil && candidate.After(*yr.Until) {
					return nil
				}
				
				// Check count if specified
				if yr.Count != nil {
					totalOccurrences := yr.countOccurrencesUntil(candidate, baseTime)
					if totalOccurrences > *yr.Count {
						return nil
					}
				}
				
				// Return if not an exception date
				if !isExceptionDate(candidate, exDates) {
					return &candidate
				}
			}
		}
		
		// Move to next interval year
		currentYear += yr.Interval
		occurrenceCount++
		
		// Safety check to prevent infinite loops
		if occurrenceCount > 1000 {
			return nil
		}
	}
}

func (yr *YearlyRecurrence) String() string {
	yearStr := "yearly"
	if yr.Interval != 1 {
		yearStr = fmt.Sprintf("every %d years", yr.Interval)
	}
	
	if len(yr.ByMonth) > 0 && len(yr.ByMonthDay) > 0 {
		return fmt.Sprintf("%s in %v on day %v", yearStr, yr.ByMonth, yr.ByMonthDay)
	} else if len(yr.ByMonth) > 0 {
		return fmt.Sprintf("%s in %v", yearStr, yr.ByMonth)
	} else if len(yr.ByMonthDay) > 0 {
		return fmt.Sprintf("%s on day %v", yearStr, yr.ByMonthDay)
	}
	
	return yearStr
}

// Helper function to count occurrences up to a specific date
func (yr *YearlyRecurrence) countOccurrencesUntil(untilDate time.Time, baseTime time.Time) int {
	count := 0
	targetMonths := yr.ByMonth
	if len(targetMonths) == 0 {
		targetMonths = []time.Month{baseTime.Month()}
	}
	
	targetDays := yr.ByMonthDay
	if len(targetDays) == 0 {
		targetDays = []int{baseTime.Day()}
	}
	
	baseDate := baseTime.Truncate(24 * time.Hour)
	currentYear := baseDate.Year()
	
	for {
		for _, month := range targetMonths {
			for _, day := range targetDays {
				actualDay := day
				if day < 0 {
					daysInMonth := getDaysInMonth(currentYear, month)
					actualDay = daysInMonth + day + 1
				}
				
				if actualDay > getDaysInMonth(currentYear, month) {
					actualDay = getDaysInMonth(currentYear, month)
				}
				
				if actualDay < 1 {
					continue
				}
				
				candidate := time.Date(currentYear, month, actualDay,
					baseTime.Hour(), baseTime.Minute(), baseTime.Second(), 0, baseTime.Location())
				
				if candidate.After(untilDate) {
					return count
				}
				
				if candidate.Equal(baseDate) || candidate.After(baseDate) {
					count++
				}
			}
		}
		
		currentYear += yr.Interval
		
		// Safety check
		if currentYear > untilDate.Year()+1 {
			break
		}
	}
	
	return count
}