package recurrence

import (
	"testing"
	"time"
)

func TestParseRRule(t *testing.T) {
	tests := []struct {
		name        string
		rrule       string
		expectType  string
		expectError bool
	}{
		{
			name:        "Empty RRULE",
			rrule:       "",
			expectType:  "NoRecurrence",
			expectError: false,
		},
		{
			name:        "Simple daily",
			rrule:       "FREQ=DAILY",
			expectType:  "DailyRecurrence",
			expectError: false,
		},
		{
			name:        "Daily with interval",
			rrule:       "FREQ=DAILY;INTERVAL=2",
			expectType:  "DailyRecurrence",
			expectError: false,
		},
		{
			name:        "Daily with count",
			rrule:       "FREQ=DAILY;COUNT=5",
			expectType:  "DailyRecurrence",
			expectError: false,
		},
		{
			name:        "Daily with until",
			rrule:       "FREQ=DAILY;UNTIL=20241231",
			expectType:  "DailyRecurrence",
			expectError: false,
		},
		{
			name:        "Weekly",
			rrule:       "FREQ=WEEKLY",
			expectType:  "WeeklyRecurrence",
			expectError: false,
		},
		{
			name:        "Weekly with BYDAY",
			rrule:       "FREQ=WEEKLY;BYDAY=MO,WE,FR",
			expectType:  "WeeklyRecurrence",
			expectError: false,
		},
		{
			name:        "Monthly",
			rrule:       "FREQ=MONTHLY",
			expectType:  "MonthlyRecurrence",
			expectError: false,
		},
		{
			name:        "Monthly with BYMONTHDAY",
			rrule:       "FREQ=MONTHLY;BYMONTHDAY=15,30",
			expectType:  "MonthlyRecurrence",
			expectError: false,
		},
		{
			name:        "Monthly with negative BYMONTHDAY",
			rrule:       "FREQ=MONTHLY;BYMONTHDAY=-1",
			expectType:  "MonthlyRecurrence",
			expectError: false,
		},
		{
			name:        "Yearly",
			rrule:       "FREQ=YEARLY",
			expectType:  "YearlyRecurrence",
			expectError: false,
		},
		{
			name:        "Yearly with BYMONTH",
			rrule:       "FREQ=YEARLY;BYMONTH=1,7",
			expectType:  "YearlyRecurrence",
			expectError: false,
		},
		{
			name:        "Yearly with BYMONTH and BYMONTHDAY",
			rrule:       "FREQ=YEARLY;BYMONTH=12;BYMONTHDAY=25",
			expectType:  "YearlyRecurrence",
			expectError: false,
		},
		{
			name:        "Invalid frequency",
			rrule:       "FREQ=HOURLY",
			expectType:  "",
			expectError: true,
		},
		{
			name:        "Missing frequency",
			rrule:       "INTERVAL=2",
			expectType:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := ParseRRule(tt.rrule)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for RRULE '%s', but got none", tt.rrule)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error for RRULE '%s': %v", tt.rrule, err)
				return
			}
			
			// Check the type
			var actualType string
			switch rec.(type) {
			case *NoRecurrence:
				actualType = "NoRecurrence"
			case *DailyRecurrence:
				actualType = "DailyRecurrence"
			case *WeeklyRecurrence:
				actualType = "WeeklyRecurrence"
			case *MonthlyRecurrence:
				actualType = "MonthlyRecurrence"
			case *YearlyRecurrence:
				actualType = "YearlyRecurrence"
			default:
				actualType = "Unknown"
			}
			
			if actualType != tt.expectType {
				t.Errorf("Expected type %s, got %s", tt.expectType, actualType)
			}
		})
	}
}

func TestParseDailyRRule(t *testing.T) {
	tests := []struct {
		name            string
		rrule           string
		expectedInterval int
		expectedCount   *int
		expectedUntil   *time.Time
	}{
		{
			name:             "Simple daily",
			rrule:            "FREQ=DAILY",
			expectedInterval: 1,
			expectedCount:    nil,
			expectedUntil:    nil,
		},
		{
			name:             "Daily with interval 3",
			rrule:            "FREQ=DAILY;INTERVAL=3",
			expectedInterval: 3,
			expectedCount:    nil,
			expectedUntil:    nil,
		},
		{
			name:             "Daily with count",
			rrule:            "FREQ=DAILY;COUNT=10",
			expectedInterval: 1,
			expectedCount:    intPtr(10),
			expectedUntil:    nil,
		},
		{
			name:             "Daily with until date",
			rrule:            "FREQ=DAILY;UNTIL=20241231",
			expectedInterval: 1,
			expectedCount:    nil,
			expectedUntil:    timePtr(mustParse(t, "20060102", "20241231")),
		},
		{
			name:             "Daily with all parameters",
			rrule:            "FREQ=DAILY;INTERVAL=2;COUNT=5;UNTIL=20241231",
			expectedInterval: 2,
			expectedCount:    intPtr(5),
			expectedUntil:    timePtr(mustParse(t, "20060102", "20241231")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := ParseRRule(tt.rrule)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			dr, ok := rec.(*DailyRecurrence)
			if !ok {
				t.Fatalf("Expected DailyRecurrence, got %T", rec)
			}
			
			if dr.Interval != tt.expectedInterval {
				t.Errorf("Expected interval %d, got %d", tt.expectedInterval, dr.Interval)
			}
			
			if !equalIntPtr(dr.Count, tt.expectedCount) {
				t.Errorf("Expected count %v, got %v", tt.expectedCount, dr.Count)
			}
			
			if !equalTimePtr(dr.Until, tt.expectedUntil) {
				t.Errorf("Expected until %v, got %v", tt.expectedUntil, dr.Until)
			}
		})
	}
}

func TestParseWeeklyRRule(t *testing.T) {
	tests := []struct {
		name             string
		rrule            string
		expectedInterval int
		expectedByDay    []time.Weekday
	}{
		{
			name:             "Simple weekly",
			rrule:            "FREQ=WEEKLY",
			expectedInterval: 1,
			expectedByDay:    nil,
		},
		{
			name:             "Weekly with single day",
			rrule:            "FREQ=WEEKLY;BYDAY=MO",
			expectedInterval: 1,
			expectedByDay:    []time.Weekday{time.Monday},
		},
		{
			name:             "Weekly with multiple days",
			rrule:            "FREQ=WEEKLY;BYDAY=MO,WE,FR",
			expectedInterval: 1,
			expectedByDay:    []time.Weekday{time.Monday, time.Wednesday, time.Friday},
		},
		{
			name:             "Bi-weekly with days",
			rrule:            "FREQ=WEEKLY;INTERVAL=2;BYDAY=TU,TH",
			expectedInterval: 2,
			expectedByDay:    []time.Weekday{time.Tuesday, time.Thursday},
		},
		{
			name:             "Weekly with all days",
			rrule:            "FREQ=WEEKLY;BYDAY=SU,MO,TU,WE,TH,FR,SA",
			expectedInterval: 1,
			expectedByDay:    []time.Weekday{time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := ParseRRule(tt.rrule)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			wr, ok := rec.(*WeeklyRecurrence)
			if !ok {
				t.Fatalf("Expected WeeklyRecurrence, got %T", rec)
			}
			
			if wr.Interval != tt.expectedInterval {
				t.Errorf("Expected interval %d, got %d", tt.expectedInterval, wr.Interval)
			}
			
			if !equalWeekdaySlice(wr.ByDay, tt.expectedByDay) {
				t.Errorf("Expected ByDay %v, got %v", tt.expectedByDay, wr.ByDay)
			}
		})
	}
}

func TestParseMonthlyRRule(t *testing.T) {
	tests := []struct {
		name                string
		rrule               string
		expectedInterval    int
		expectedByMonthDay  []int
	}{
		{
			name:               "Simple monthly",
			rrule:              "FREQ=MONTHLY",
			expectedInterval:   1,
			expectedByMonthDay: nil,
		},
		{
			name:               "Monthly on 15th",
			rrule:              "FREQ=MONTHLY;BYMONTHDAY=15",
			expectedInterval:   1,
			expectedByMonthDay: []int{15},
		},
		{
			name:               "Monthly on multiple days",
			rrule:              "FREQ=MONTHLY;BYMONTHDAY=1,15,31",
			expectedInterval:   1,
			expectedByMonthDay: []int{1, 15, 31},
		},
		{
			name:               "Monthly on last day",
			rrule:              "FREQ=MONTHLY;BYMONTHDAY=-1",
			expectedInterval:   1,
			expectedByMonthDay: []int{-1},
		},
		{
			name:               "Quarterly with specific days",
			rrule:              "FREQ=MONTHLY;INTERVAL=3;BYMONTHDAY=1,15",
			expectedInterval:   3,
			expectedByMonthDay: []int{1, 15},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := ParseRRule(tt.rrule)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			mr, ok := rec.(*MonthlyRecurrence)
			if !ok {
				t.Fatalf("Expected MonthlyRecurrence, got %T", rec)
			}
			
			if mr.Interval != tt.expectedInterval {
				t.Errorf("Expected interval %d, got %d", tt.expectedInterval, mr.Interval)
			}
			
			if !equalIntSlice(mr.ByMonthDay, tt.expectedByMonthDay) {
				t.Errorf("Expected ByMonthDay %v, got %v", tt.expectedByMonthDay, mr.ByMonthDay)
			}
		})
	}
}

func TestParseYearlyRRule(t *testing.T) {
	tests := []struct {
		name                string
		rrule               string
		expectedInterval    int
		expectedByMonth     []time.Month
		expectedByMonthDay  []int
	}{
		{
			name:               "Simple yearly",
			rrule:              "FREQ=YEARLY",
			expectedInterval:   1,
			expectedByMonth:    nil,
			expectedByMonthDay: nil,
		},
		{
			name:               "Yearly in specific months",
			rrule:              "FREQ=YEARLY;BYMONTH=1,7",
			expectedInterval:   1,
			expectedByMonth:    []time.Month{time.January, time.July},
			expectedByMonthDay: nil,
		},
		{
			name:               "Yearly on Christmas",
			rrule:              "FREQ=YEARLY;BYMONTH=12;BYMONTHDAY=25",
			expectedInterval:   1,
			expectedByMonth:    []time.Month{time.December},
			expectedByMonthDay: []int{25},
		},
		{
			name:               "Bi-yearly with multiple parameters",
			rrule:              "FREQ=YEARLY;INTERVAL=2;BYMONTH=6,12;BYMONTHDAY=1,15",
			expectedInterval:   2,
			expectedByMonth:    []time.Month{time.June, time.December},
			expectedByMonthDay: []int{1, 15},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := ParseRRule(tt.rrule)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			yr, ok := rec.(*YearlyRecurrence)
			if !ok {
				t.Fatalf("Expected YearlyRecurrence, got %T", rec)
			}
			
			if yr.Interval != tt.expectedInterval {
				t.Errorf("Expected interval %d, got %d", tt.expectedInterval, yr.Interval)
			}
			
			if !equalMonthSlice(yr.ByMonth, tt.expectedByMonth) {
				t.Errorf("Expected ByMonth %v, got %v", tt.expectedByMonth, yr.ByMonth)
			}
			
			if !equalIntSlice(yr.ByMonthDay, tt.expectedByMonthDay) {
				t.Errorf("Expected ByMonthDay %v, got %v", tt.expectedByMonthDay, yr.ByMonthDay)
			}
		})
	}
}

func TestFormatRRule(t *testing.T) {
	tests := []struct {
		name     string
		rec      Recurrence
		expected string
	}{
		{
			name:     "NoRecurrence",
			rec:      &NoRecurrence{},
			expected: "",
		},
		{
			name:     "Simple daily",
			rec:      NewDailyRecurrence(1, nil, nil),
			expected: "FREQ=DAILY;INTERVAL=1",
		},
		{
			name:     "Daily with count",
			rec:      NewDailyRecurrence(2, nil, intPtr(5)),
			expected: "FREQ=DAILY;INTERVAL=2;COUNT=5",
		},
		{
			name:     "Weekly with days",
			rec:      NewWeeklyRecurrence(1, []time.Weekday{time.Monday, time.Friday}, nil, nil),
			expected: "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,FR",
		},
		{
			name:     "Monthly with days",
			rec:      NewMonthlyRecurrence(1, []int{15, -1}, nil, nil),
			expected: "FREQ=MONTHLY;INTERVAL=1;BYMONTHDAY=15,-1",
		},
		{
			name:     "Yearly with month and day",
			rec:      NewYearlyRecurrence(1, []time.Month{time.December}, []int{25}, nil, nil),
			expected: "FREQ=YEARLY;INTERVAL=1;BYMONTH=12;BYMONTHDAY=25",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatRRule(tt.rec)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// Helper functions for tests
func intPtr(i int) *int {
	return &i
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func equalIntPtr(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func equalTimePtr(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func equalIntSlice(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func equalWeekdaySlice(a, b []time.Weekday) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func equalMonthSlice(a, b []time.Month) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}