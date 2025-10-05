package recurrence

import (
	"testing"
	"time"
)

// Helper functions for testing
func mustParse(t *testing.T, layout, value string) time.Time {
	t.Helper()
	result, err := time.Parse(layout, value)
	if err != nil {
		t.Fatalf("Failed to parse time %s: %v", value, err)
	}
	return result
}

func parseDate(t *testing.T, dateStr string) time.Time {
	t.Helper()
	return mustParse(t, "2006-01-02", dateStr)
}

func parseDateTime(t *testing.T, dateTimeStr string) time.Time {
	t.Helper()
	return mustParse(t, "2006-01-02 15:04:05", dateTimeStr)
}

// Test NoRecurrence
func TestNoRecurrence(t *testing.T) {
	nr := &NoRecurrence{}
	baseTime := parseDateTime(t, "2024-01-15 10:30:00")
	
	// Test OccursOn - should only occur on the base date
	if !nr.OccursOn(baseTime, baseTime) {
		t.Error("NoRecurrence should occur on the base date")
	}
	
	// Should not occur on different dates
	nextDay := baseTime.AddDate(0, 0, 1)
	if nr.OccursOn(nextDay, baseTime) {
		t.Error("NoRecurrence should not occur on different dates")
	}
	
	// Test OccurredWithin
	start := parseDateTime(t, "2024-01-14 00:00:00")
	end := parseDateTime(t, "2024-01-16 23:59:59")
	occurrences := nr.OccurredWithin(start, end, baseTime, nil)
	if len(occurrences) != 1 {
		t.Errorf("NoRecurrence should return 1 occurrence (base event), got %d", len(occurrences))
	}
	if len(occurrences) == 1 && !occurrences[0].Equal(baseTime) {
		t.Errorf("Expected occurrence to be %v, got %v", baseTime, occurrences[0])
	}
	
	// Test NextOccurrence - should return base time if it's after the given time
	before := parseDateTime(t, "2024-01-14 00:00:00")
	next := nr.NextOccurrence(before, baseTime, nil)
	if next == nil || !next.Equal(baseTime) {
		t.Error("NoRecurrence should return base time for NextOccurrence when base is after given time")
	}
	
	// Should return nil when base time is before the given time
	after := parseDateTime(t, "2024-01-16 00:00:00")
	next = nr.NextOccurrence(after, baseTime, nil)
	if next != nil {
		t.Error("NoRecurrence should return nil for NextOccurrence when base is before given time")
	}
	
	// Test String
	expected := "No recurrence"
	if nr.String() != expected {
		t.Errorf("Expected string '%s', got '%s'", expected, nr.String())
	}
}

// Test DailyRecurrence
func TestDailyRecurrence(t *testing.T) {
	baseTime := parseDateTime(t, "2024-01-15 10:30:00")
	
	t.Run("Simple daily recurrence", func(t *testing.T) {
		dr := NewDailyRecurrence(1, nil, nil)
		
		// Should occur on the same day
		if !dr.OccursOn(baseTime, baseTime) {
			t.Error("Daily recurrence should occur on base date")
		}
		
		// Should occur the next day
		nextDay := baseTime.AddDate(0, 0, 1)
		if !dr.OccursOn(nextDay, baseTime) {
			t.Error("Daily recurrence should occur on next day")
		}
		
		// Should not occur before base date
		prevDay := baseTime.AddDate(0, 0, -1)
		if dr.OccursOn(prevDay, baseTime) {
			t.Error("Daily recurrence should not occur before base date")
		}
	})
	
	t.Run("Every 2 days", func(t *testing.T) {
		dr := NewDailyRecurrence(2, nil, nil)
		
		// Should occur on base date
		if !dr.OccursOn(baseTime, baseTime) {
			t.Error("Daily recurrence should occur on base date")
		}
		
		// Should not occur after 1 day
		nextDay := baseTime.AddDate(0, 0, 1)
		if dr.OccursOn(nextDay, baseTime) {
			t.Error("Every 2 days recurrence should not occur after 1 day")
		}
		
		// Should occur after 2 days
		after2Days := baseTime.AddDate(0, 0, 2)
		if !dr.OccursOn(after2Days, baseTime) {
			t.Error("Every 2 days recurrence should occur after 2 days")
		}
	})
	
	t.Run("With count limit", func(t *testing.T) {
		count := 3
		dr := NewDailyRecurrence(1, nil, &count)
		
		// First 3 occurrences should be valid
		for i := 0; i < 3; i++ {
			checkDate := baseTime.AddDate(0, 0, i)
			if !dr.OccursOn(checkDate, baseTime) {
				t.Errorf("Daily recurrence should occur on day %d", i)
			}
		}
		
		// 4th occurrence should not be valid
		fourthDay := baseTime.AddDate(0, 0, 3)
		if dr.OccursOn(fourthDay, baseTime) {
			t.Error("Daily recurrence should not occur after count limit")
		}
	})
	
	t.Run("With until date", func(t *testing.T) {
		until := baseTime.AddDate(0, 0, 2) // 2 days after base
		dr := NewDailyRecurrence(1, &until, nil)
		
		// Should occur on base date and next day
		if !dr.OccursOn(baseTime, baseTime) {
			t.Error("Daily recurrence should occur on base date")
		}
		
		nextDay := baseTime.AddDate(0, 0, 1)
		if !dr.OccursOn(nextDay, baseTime) {
			t.Error("Daily recurrence should occur on next day")
		}
		
		// Should occur on the until date (UNTIL is inclusive)
		untilDay := baseTime.AddDate(0, 0, 2)
		if !dr.OccursOn(untilDay, baseTime) {
			t.Error("Daily recurrence should occur on until date (UNTIL is inclusive)")
		}
		
		// Should not occur after until date
		afterUntil := baseTime.AddDate(0, 0, 3)
		if dr.OccursOn(afterUntil, baseTime) {
			t.Error("Daily recurrence should not occur after until date")
		}
	})
	
	t.Run("OccurredWithin", func(t *testing.T) {
		dr := NewDailyRecurrence(1, nil, nil)
		
		start := baseTime.AddDate(0, 0, -1) // 1 day before
		end := baseTime.AddDate(0, 0, 3)    // 3 days after
		
		occurrences := dr.OccurredWithin(start, end, baseTime, nil)
		
		// Should have 4 occurrences: base day + 3 days after
		expectedCount := 4
		if len(occurrences) != expectedCount {
			t.Errorf("Expected %d occurrences, got %d", expectedCount, len(occurrences))
		}
		
		// Verify the occurrences are on the correct dates
		for i, occurrence := range occurrences {
			expectedDate := baseTime.AddDate(0, 0, i)
			if !occurrence.Equal(expectedDate) {
				t.Errorf("Occurrence %d should be %v, got %v", i, expectedDate, occurrence)
			}
		}
	})
}

// Test WeeklyRecurrence
func TestWeeklyRecurrence(t *testing.T) {
	// Monday, January 15, 2024
	baseTime := parseDateTime(t, "2024-01-15 10:30:00") // Monday
	
	t.Run("Simple weekly recurrence", func(t *testing.T) {
		wr := NewWeeklyRecurrence(1, nil, nil, nil)
		
		// Should occur on the base day (Monday)
		if !wr.OccursOn(baseTime, baseTime) {
			t.Error("Weekly recurrence should occur on base date")
		}
		
		// Should occur next Monday
		nextWeek := baseTime.AddDate(0, 0, 7)
		if !wr.OccursOn(nextWeek, baseTime) {
			t.Error("Weekly recurrence should occur next week")
		}
		
		// Should not occur on Tuesday
		tuesday := baseTime.AddDate(0, 0, 1)
		if wr.OccursOn(tuesday, baseTime) {
			t.Error("Weekly recurrence should not occur on different weekday")
		}
	})
	
	t.Run("Specific weekdays", func(t *testing.T) {
		// Monday and Friday
		byDay := []time.Weekday{time.Monday, time.Friday}
		wr := NewWeeklyRecurrence(1, byDay, nil, nil)
		
		// Should occur on Monday (base day)
		if !wr.OccursOn(baseTime, baseTime) {
			t.Error("Weekly recurrence should occur on Monday")
		}
		
		// Should occur on Friday of the same week
		friday := baseTime.AddDate(0, 0, 4) // 4 days after Monday
		if !wr.OccursOn(friday, baseTime) {
			t.Error("Weekly recurrence should occur on Friday")
		}
		
		// Should not occur on Tuesday
		tuesday := baseTime.AddDate(0, 0, 1)
		if wr.OccursOn(tuesday, baseTime) {
			t.Error("Weekly recurrence should not occur on Tuesday")
		}
	})
	
	t.Run("Every 2 weeks", func(t *testing.T) {
		wr := NewWeeklyRecurrence(2, nil, nil, nil)
		
		// Should occur on base date
		if !wr.OccursOn(baseTime, baseTime) {
			t.Error("Bi-weekly recurrence should occur on base date")
		}
		
		// Should not occur next week
		nextWeek := baseTime.AddDate(0, 0, 7)
		if wr.OccursOn(nextWeek, baseTime) {
			t.Error("Bi-weekly recurrence should not occur next week")
		}
		
		// Should occur in 2 weeks
		twoWeeks := baseTime.AddDate(0, 0, 14)
		if !wr.OccursOn(twoWeeks, baseTime) {
			t.Error("Bi-weekly recurrence should occur in 2 weeks")
		}
	})
}

// Test MonthlyRecurrence
func TestMonthlyRecurrence(t *testing.T) {
	// 15th of January 2024
	baseTime := parseDateTime(t, "2024-01-15 10:30:00")
	
	t.Run("Simple monthly recurrence", func(t *testing.T) {
		mr := NewMonthlyRecurrence(1, nil, nil, nil)
		
		// Should occur on base date
		if !mr.OccursOn(baseTime, baseTime) {
			t.Error("Monthly recurrence should occur on base date")
		}
		
		// Should occur on 15th of next month
		nextMonth := parseDateTime(t, "2024-02-15 10:30:00")
		if !mr.OccursOn(nextMonth, baseTime) {
			t.Error("Monthly recurrence should occur on 15th of next month")
		}
		
		// Should not occur on 16th of same month
		sixteenth := parseDateTime(t, "2024-01-16 10:30:00")
		if mr.OccursOn(sixteenth, baseTime) {
			t.Error("Monthly recurrence should not occur on different day")
		}
	})
	
	t.Run("Specific month days", func(t *testing.T) {
		byMonthDay := []int{15, 30}
		mr := NewMonthlyRecurrence(1, byMonthDay, nil, nil)
		
		// Should occur on 15th
		if !mr.OccursOn(baseTime, baseTime) {
			t.Error("Monthly recurrence should occur on 15th")
		}
		
		// Should occur on 30th of same month
		thirtieth := parseDateTime(t, "2024-01-30 10:30:00")
		if !mr.OccursOn(thirtieth, baseTime) {
			t.Error("Monthly recurrence should occur on 30th")
		}
		
		// Should not occur on 20th
		twentieth := parseDateTime(t, "2024-01-20 10:30:00")
		if mr.OccursOn(twentieth, baseTime) {
			t.Error("Monthly recurrence should not occur on 20th")
		}
	})
	
	t.Run("Last day of month (-1)", func(t *testing.T) {
		byMonthDay := []int{-1} // Last day of month
		mr := NewMonthlyRecurrence(1, byMonthDay, nil, nil)
		
		// Should occur on January 31st (last day)
		lastDay := parseDateTime(t, "2024-01-31 10:30:00")
		if !mr.OccursOn(lastDay, baseTime) {
			t.Error("Monthly recurrence should occur on last day of month")
		}
		
		// Should occur on February 29th (leap year)
		febLast := parseDateTime(t, "2024-02-29 10:30:00")
		if !mr.OccursOn(febLast, baseTime) {
			t.Error("Monthly recurrence should occur on last day of February")
		}
	})
}

// Test YearlyRecurrence
func TestYearlyRecurrence(t *testing.T) {
	// January 15, 2024
	baseTime := parseDateTime(t, "2024-01-15 10:30:00")
	
	t.Run("Simple yearly recurrence", func(t *testing.T) {
		yr := NewYearlyRecurrence(1, nil, nil, nil, nil)
		
		// Should occur on base date
		if !yr.OccursOn(baseTime, baseTime) {
			t.Error("Yearly recurrence should occur on base date")
		}
		
		// Should occur on same date next year
		nextYear := parseDateTime(t, "2025-01-15 10:30:00")
		if !yr.OccursOn(nextYear, baseTime) {
			t.Error("Yearly recurrence should occur on same date next year")
		}
		
		// Should not occur on different month/day
		different := parseDateTime(t, "2024-02-15 10:30:00")
		if yr.OccursOn(different, baseTime) {
			t.Error("Yearly recurrence should not occur on different month/day")
		}
	})
	
	t.Run("Specific months", func(t *testing.T) {
		byMonth := []time.Month{time.January, time.July}
		yr := NewYearlyRecurrence(1, byMonth, nil, nil, nil)
		
		// Should occur on January 15th
		if !yr.OccursOn(baseTime, baseTime) {
			t.Error("Yearly recurrence should occur on January 15th")
		}
		
		// Should occur on July 15th
		july := parseDateTime(t, "2024-07-15 10:30:00")
		if !yr.OccursOn(july, baseTime) {
			t.Error("Yearly recurrence should occur on July 15th")
		}
		
		// Should not occur in February
		february := parseDateTime(t, "2024-02-15 10:30:00")
		if yr.OccursOn(february, baseTime) {
			t.Error("Yearly recurrence should not occur in February")
		}
	})
	
	t.Run("Every 2 years", func(t *testing.T) {
		yr := NewYearlyRecurrence(2, nil, nil, nil, nil)
		
		// Should occur on base date
		if !yr.OccursOn(baseTime, baseTime) {
			t.Error("Bi-yearly recurrence should occur on base date")
		}
		
		// Should not occur next year
		nextYear := parseDateTime(t, "2025-01-15 10:30:00")
		if yr.OccursOn(nextYear, baseTime) {
			t.Error("Bi-yearly recurrence should not occur next year")
		}
		
		// Should occur in 2 years
		twoYears := parseDateTime(t, "2026-01-15 10:30:00")
		if !yr.OccursOn(twoYears, baseTime) {
			t.Error("Bi-yearly recurrence should occur in 2 years")
		}
	})
}