package recurrence

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseRRule parses an RRULE string and returns the appropriate Recurrence instance
func ParseRRule(rrule string) (Recurrence, error) {
	if rrule == "" {
		return &NoRecurrence{}, nil
	}
	
	// Parse RRULE string into key-value pairs
	parts := make(map[string]string)
	for _, part := range strings.Split(rrule, ";") {
		if kv := strings.SplitN(part, "=", 2); len(kv) == 2 {
			parts[strings.ToUpper(kv[0])] = strings.ToUpper(kv[1])
		}
	}
	
	// Get frequency (required)
	freq, exists := parts["FREQ"]
	if !exists {
		return nil, fmt.Errorf("FREQ is required in RRULE")
	}
	
	// Parse common fields
	interval := 1
	if intervalStr, exists := parts["INTERVAL"]; exists {
		if i, err := strconv.Atoi(intervalStr); err == nil && i > 0 {
			interval = i
		}
	}
	
	var until *time.Time
	if untilStr, exists := parts["UNTIL"]; exists {
		if t, err := parseRRuleTime(untilStr); err == nil {
			until = &t
		}
	}
	
	var count *int
	if countStr, exists := parts["COUNT"]; exists {
		if c, err := strconv.Atoi(countStr); err == nil && c > 0 {
			count = &c
		}
	}
	
	// Create appropriate recurrence based on frequency
	switch freq {
	case "DAILY":
		return NewDailyRecurrence(interval, until, count), nil
		
	case "WEEKLY":
		byDay := parseByDay(parts["BYDAY"])
		return NewWeeklyRecurrence(interval, byDay, until, count), nil
		
	case "MONTHLY":
		byMonthDay := parseByMonthDay(parts["BYMONTHDAY"])
		return NewMonthlyRecurrence(interval, byMonthDay, until, count), nil
		
	case "YEARLY":
		byMonth := parseByMonth(parts["BYMONTH"])
		byMonthDay := parseByMonthDay(parts["BYMONTHDAY"])
		return NewYearlyRecurrence(interval, byMonth, byMonthDay, until, count), nil
		
	default:
		return nil, fmt.Errorf("unsupported frequency: %s", freq)
	}
}

// parseRRuleTime parses a time string from RRULE format
func parseRRuleTime(timeStr string) (time.Time, error) {
	// Handle both local time (YYYYMMDDTHHMMSS) and UTC time (YYYYMMDDTHHMMSSZ)
	timeStr = strings.TrimSuffix(timeStr, "Z")
	
	if len(timeStr) == 8 {
		// Date only (YYYYMMDD)
		return time.Parse("20060102", timeStr)
	} else if len(timeStr) == 15 {
		// Date and time (YYYYMMDDTHHMMSS)
		return time.Parse("20060102T150405", timeStr)
	}
	
	return time.Time{}, fmt.Errorf("invalid time format: %s", timeStr)
}

// parseByDay parses BYDAY values into weekdays
func parseByDay(byDayStr string) []time.Weekday {
	if byDayStr == "" {
		return nil
	}
	
	var weekdays []time.Weekday
	days := strings.Split(byDayStr, ",")
	
	for _, day := range days {
		day = strings.TrimSpace(day)
		// Remove any numeric prefix (like +1MO, -1FR)
		if len(day) >= 2 {
			dayCode := day[len(day)-2:]
			
			switch dayCode {
			case "MO":
				weekdays = append(weekdays, time.Monday)
			case "TU":
				weekdays = append(weekdays, time.Tuesday)
			case "WE":
				weekdays = append(weekdays, time.Wednesday)
			case "TH":
				weekdays = append(weekdays, time.Thursday)
			case "FR":
				weekdays = append(weekdays, time.Friday)
			case "SA":
				weekdays = append(weekdays, time.Saturday)
			case "SU":
				weekdays = append(weekdays, time.Sunday)
			}
		}
	}
	
	return weekdays
}

// parseByMonthDay parses BYMONTHDAY values
func parseByMonthDay(byMonthDayStr string) []int {
	if byMonthDayStr == "" {
		return nil
	}
	
	var days []int
	dayStrs := strings.Split(byMonthDayStr, ",")
	
	for _, dayStr := range dayStrs {
		dayStr = strings.TrimSpace(dayStr)
		if day, err := strconv.Atoi(dayStr); err == nil {
			// Validate range: -31 to -1 and 1 to 31
			if (day >= 1 && day <= 31) || (day >= -31 && day <= -1) {
				days = append(days, day)
			}
		}
	}
	
	return days
}

// parseByMonth parses BYMONTH values
func parseByMonth(byMonthStr string) []time.Month {
	if byMonthStr == "" {
		return nil
	}
	
	var months []time.Month
	monthStrs := strings.Split(byMonthStr, ",")
	
	for _, monthStr := range monthStrs {
		monthStr = strings.TrimSpace(monthStr)
		if month, err := strconv.Atoi(monthStr); err == nil {
			if month >= 1 && month <= 12 {
				months = append(months, time.Month(month))
			}
		}
	}
	
	return months
}

// FormatRRule converts a Recurrence back to an RRULE string (for debugging/serialization)
func FormatRRule(rec Recurrence) string {
	switch r := rec.(type) {
	case *NoRecurrence:
		return ""
		
	case *DailyRecurrence:
		rrule := fmt.Sprintf("FREQ=DAILY;INTERVAL=%d", r.Interval)
		if r.Until != nil {
			rrule += fmt.Sprintf(";UNTIL=%s", r.Until.Format("20060102T150405Z"))
		}
		if r.Count != nil {
			rrule += fmt.Sprintf(";COUNT=%d", *r.Count)
		}
		return rrule
		
	case *WeeklyRecurrence:
		rrule := fmt.Sprintf("FREQ=WEEKLY;INTERVAL=%d", r.Interval)
		if len(r.ByDay) > 0 {
			var days []string
			for _, day := range r.ByDay {
				switch day {
				case time.Monday:
					days = append(days, "MO")
				case time.Tuesday:
					days = append(days, "TU")
				case time.Wednesday:
					days = append(days, "WE")
				case time.Thursday:
					days = append(days, "TH")
				case time.Friday:
					days = append(days, "FR")
				case time.Saturday:
					days = append(days, "SA")
				case time.Sunday:
					days = append(days, "SU")
				}
			}
			rrule += fmt.Sprintf(";BYDAY=%s", strings.Join(days, ","))
		}
		if r.Until != nil {
			rrule += fmt.Sprintf(";UNTIL=%s", r.Until.Format("20060102T150405Z"))
		}
		if r.Count != nil {
			rrule += fmt.Sprintf(";COUNT=%d", *r.Count)
		}
		return rrule
		
	case *MonthlyRecurrence:
		rrule := fmt.Sprintf("FREQ=MONTHLY;INTERVAL=%d", r.Interval)
		if len(r.ByMonthDay) > 0 {
			var days []string
			for _, day := range r.ByMonthDay {
				days = append(days, strconv.Itoa(day))
			}
			rrule += fmt.Sprintf(";BYMONTHDAY=%s", strings.Join(days, ","))
		}
		if r.Until != nil {
			rrule += fmt.Sprintf(";UNTIL=%s", r.Until.Format("20060102T150405Z"))
		}
		if r.Count != nil {
			rrule += fmt.Sprintf(";COUNT=%d", *r.Count)
		}
		return rrule
		
	case *YearlyRecurrence:
		rrule := fmt.Sprintf("FREQ=YEARLY;INTERVAL=%d", r.Interval)
		if len(r.ByMonth) > 0 {
			var months []string
			for _, month := range r.ByMonth {
				months = append(months, strconv.Itoa(int(month)))
			}
			rrule += fmt.Sprintf(";BYMONTH=%s", strings.Join(months, ","))
		}
		if len(r.ByMonthDay) > 0 {
			var days []string
			for _, day := range r.ByMonthDay {
				days = append(days, strconv.Itoa(day))
			}
			rrule += fmt.Sprintf(";BYMONTHDAY=%s", strings.Join(days, ","))
		}
		if r.Until != nil {
			rrule += fmt.Sprintf(";UNTIL=%s", r.Until.Format("20060102T150405Z"))
		}
		if r.Count != nil {
			rrule += fmt.Sprintf(";COUNT=%d", *r.Count)
		}
		return rrule
		
	default:
		return ""
	}
}