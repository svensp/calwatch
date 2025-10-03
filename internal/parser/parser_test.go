package parser

import (
	"strings"
	"testing"
	"time"
)

func TestGocalParser_ValidateICS(t *testing.T) {
	parser := NewGocalParser()

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name: "valid basic calendar",
			data: `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test@example.com
DTSTART:20231015T140000Z
DTEND:20231015T150000Z
DTSTAMP:20231015T120000Z
SUMMARY:Test Event
END:VEVENT
END:VCALENDAR`,
			wantErr: false,
		},
		{
			name:    "missing BEGIN:VCALENDAR",
			data:    "VERSION:2.0\nEND:VCALENDAR",
			wantErr: true,
		},
		{
			name:    "missing END:VCALENDAR",
			data:    "BEGIN:VCALENDAR\nVERSION:2.0",
			wantErr: true,
		},
		{
			name: "unbalanced BEGIN/END",
			data: `BEGIN:VCALENDAR
BEGIN:VEVENT
SUMMARY:Test
END:VCALENDAR`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.ValidateICS([]byte(tt.data))
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateICS() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGocalParser_ParseReader(t *testing.T) {
	parser := NewGocalParser()

	icsData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-event-1@example.com
DTSTART:20231015T140000Z
DTEND:20231015T150000Z
DTSTAMP:20231015T120000Z
SUMMARY:Test Meeting
DESCRIPTION:A test meeting description
LOCATION:Conference Room A
END:VEVENT
BEGIN:VEVENT
UID:test-event-2@example.com
DTSTART:20231016T100000Z
DTEND:20231016T110000Z
DTSTAMP:20231016T080000Z
SUMMARY:Another Meeting
END:VEVENT
END:VCALENDAR`

	reader := strings.NewReader(icsData)
	events, err := parser.ParseReader(reader)

	if err != nil {
		t.Fatalf("ParseReader() error = %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}

	// Check first event
	event1 := events[0]
	if event1.GetUID() != "test-event-1@example.com" {
		t.Errorf("Expected UID 'test-event-1@example.com', got '%s'", event1.GetUID())
	}

	if event1.GetSummary() != "Test Meeting" {
		t.Errorf("Expected summary 'Test Meeting', got '%s'", event1.GetSummary())
	}

	if event1.GetDescription() != "A test meeting description" {
		t.Errorf("Expected description 'A test meeting description', got '%s'", event1.GetDescription())
	}

	if event1.GetLocation() != "Conference Room A" {
		t.Errorf("Expected location 'Conference Room A', got '%s'", event1.GetLocation())
	}

	// Check start time
	expectedStart := time.Date(2023, 10, 15, 14, 0, 0, 0, time.UTC)
	if !event1.GetStartTime().Equal(expectedStart) {
		t.Errorf("Expected start time %v, got %v", expectedStart, event1.GetStartTime())
	}

	// Check second event
	event2 := events[1]
	if event2.GetUID() != "test-event-2@example.com" {
		t.Errorf("Expected UID 'test-event-2@example.com', got '%s'", event2.GetUID())
	}

	if event2.GetSummary() != "Another Meeting" {
		t.Errorf("Expected summary 'Another Meeting', got '%s'", event2.GetSummary())
	}
}

func TestGocalParser_ParseRecurringEvent(t *testing.T) {
	parser := NewGocalParser()

	icsData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:recurring-event@example.com
DTSTART:20231015T140000Z
DTEND:20231015T150000Z
DTSTAMP:20231015T120000Z
SUMMARY:Weekly Meeting
RRULE:FREQ=WEEKLY;BYDAY=SU
END:VEVENT
END:VCALENDAR`

	reader := strings.NewReader(icsData)
	events, err := parser.ParseReader(reader)

	if err != nil {
		t.Fatalf("ParseReader() error = %v", err)
	}

	if len(events) == 0 {
		t.Fatalf("Expected at least 1 event, got %d", len(events))
	}

	event := events[0]
	if event.GetUID() != "recurring-event@example.com" {
		t.Errorf("Expected UID 'recurring-event@example.com', got '%s'", event.GetUID())
	}

	if event.GetSummary() != "Weekly Meeting" {
		t.Errorf("Expected summary 'Weekly Meeting', got '%s'", event.GetSummary())
	}
}

func TestGocalParser_ParseInvalidICS(t *testing.T) {
	parser := NewGocalParser()

	// Test with completely invalid data first
	invalidData := `This is not a valid ICS file`

	err := parser.ValidateICS([]byte(invalidData))
	if err == nil {
		t.Error("Expected error when validating invalid ICS data, got nil")
	}

	// Test parsing invalid ICS that starts like valid but is malformed
	malformedData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:broken
DTSTART:INVALID_DATE
END:VEVENT
END:VCALENDAR`

	reader := strings.NewReader(malformedData)
	_, err = parser.ParseReader(reader)

	if err == nil {
		t.Error("Expected error when parsing malformed ICS data, got nil")
	}
}

func TestGocalParser_MaxEventsLimit(t *testing.T) {
	parser := NewGocalParser()
	parser.SetMaxEvents(1) // Set limit to 1 event

	icsData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:event-1@example.com
DTSTART:20231015T140000Z
DTEND:20231015T150000Z
DTSTAMP:20231015T120000Z
SUMMARY:Event 1
END:VEVENT
BEGIN:VEVENT
UID:event-2@example.com
DTSTART:20231016T140000Z
DTEND:20231016T150000Z
DTSTAMP:20231016T120000Z
SUMMARY:Event 2
END:VEVENT
END:VCALENDAR`

	reader := strings.NewReader(icsData)
	events, err := parser.ParseReader(reader)

	if err != nil {
		t.Fatalf("ParseReader() error = %v", err)
	}

	// Should only get 1 event due to the limit
	if len(events) != 1 {
		t.Errorf("Expected 1 event due to limit, got %d", len(events))
	}
}

func TestGocalParser_EmptyCalendar(t *testing.T) {
	parser := NewGocalParser()

	icsData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
END:VCALENDAR`

	reader := strings.NewReader(icsData)
	events, err := parser.ParseReader(reader)

	if err != nil {
		t.Fatalf("ParseReader() error = %v", err)
	}

	if len(events) != 0 {
		t.Errorf("Expected 0 events for empty calendar, got %d", len(events))
	}
}