package parser

import (
	"testing"
	"time"

	"calwatch/internal/storage"
)

func TestParseTrigger(t *testing.T) {
	parser := NewGocalParser()

	tests := []struct {
		name     string
		trigger  string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "15 minutes before",
			trigger:  "-PT15M",
			expected: 15 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "1 hour before",
			trigger:  "-PT1H",
			expected: 1 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "1 day before",
			trigger:  "-P1D",
			expected: 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "complex duration",
			trigger:  "-P1DT2H30M",
			expected: 24*time.Hour + 2*time.Hour + 30*time.Minute,
			wantErr:  false,
		},
		{
			name:     "30 seconds before",
			trigger:  "-PT30S",
			expected: 30 * time.Second,
			wantErr:  false,
		},
		{
			name:     "invalid format",
			trigger:  "invalid",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "missing -P prefix",
			trigger:  "T15M",
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.parseTrigger(tt.trigger)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseVALARMBlock(t *testing.T) {
	parser := NewGocalParser()

	tests := []struct {
		name        string
		valarmBlock string
		expected    storage.Alert
		wantErr     bool
	}{
		{
			name: "complete VALARM",
			valarmBlock: `
ACTION:DISPLAY
DESCRIPTION:15 minute warning
TRIGGER:-PT15M
`,
			expected: storage.Alert{
				Offset:      15 * time.Minute,
				Important:   false,
				Source:      storage.AlertSourceVALARM,
				Description: "15 minute warning",
				Action:      storage.AlertActionDisplay,
			},
			wantErr: false,
		},
		{
			name: "VALARM without description",
			valarmBlock: `
ACTION:DISPLAY
TRIGGER:-PT1H
`,
			expected: storage.Alert{
				Offset:      1 * time.Hour,
				Important:   false,
				Source:      storage.AlertSourceVALARM,
				Description: "Alert 1h0m0s before",
				Action:      storage.AlertActionDisplay,
			},
			wantErr: false,
		},
		{
			name: "VALARM without action",
			valarmBlock: `
DESCRIPTION:Test alert
TRIGGER:-PT30M
`,
			expected: storage.Alert{
				Offset:      30 * time.Minute,
				Important:   false,
				Source:      storage.AlertSourceVALARM,
				Description: "Test alert",
				Action:      storage.AlertActionDisplay,
			},
			wantErr: false,
		},
		{
			name: "invalid TRIGGER",
			valarmBlock: `
ACTION:DISPLAY
DESCRIPTION:Test alert
TRIGGER:invalid
`,
			expected: storage.Alert{},
			wantErr:  true,
		},
		{
			name: "unsupported ACTION",
			valarmBlock: `
ACTION:EMAIL
DESCRIPTION:Test alert
TRIGGER:-PT15M
`,
			expected: storage.Alert{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.parseVALARMBlock(tt.valarmBlock)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.Offset != tt.expected.Offset {
				t.Errorf("Expected offset %v, got %v", tt.expected.Offset, result.Offset)
			}

			if result.Important != tt.expected.Important {
				t.Errorf("Expected important %v, got %v", tt.expected.Important, result.Important)
			}

			if result.Source != tt.expected.Source {
				t.Errorf("Expected source %v, got %v", tt.expected.Source, result.Source)
			}

			if result.Description != tt.expected.Description {
				t.Errorf("Expected description %q, got %q", tt.expected.Description, result.Description)
			}

			if result.Action != tt.expected.Action {
				t.Errorf("Expected action %v, got %v", tt.expected.Action, result.Action)
			}
		})
	}
}

func TestParseVALARMs(t *testing.T) {
	parser := NewGocalParser()

	icsData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-event-1@example.com
DTSTAMP:20241201T000000Z
DTSTART:20241215T100000Z
DTEND:20241215T110000Z
SUMMARY:Test Event
BEGIN:VALARM
ACTION:DISPLAY
DESCRIPTION:15 minute warning
TRIGGER:-PT15M
END:VALARM
BEGIN:VALARM
ACTION:DISPLAY
DESCRIPTION:1 hour warning
TRIGGER:-PT1H
END:VALARM
END:VEVENT
BEGIN:VEVENT
UID:test-event-2@example.com
DTSTAMP:20241201T000000Z
DTSTART:20241215T140000Z
DTEND:20241215T150000Z
SUMMARY:Another Event
BEGIN:VALARM
ACTION:DISPLAY
DESCRIPTION:30 minute warning
TRIGGER:-PT30M
END:VALARM
END:VEVENT
END:VCALENDAR`

	// Test parsing VALARMs for first event
	alerts1, err := parser.parseVALARMs(icsData, "test-event-1@example.com")
	if err != nil {
		t.Fatalf("Unexpected error parsing VALARMs for event 1: %v", err)
	}

	if len(alerts1) != 2 {
		t.Errorf("Expected 2 alerts for event 1, got %d", len(alerts1))
	}

	// Check first alert (order may vary, so check by offset)
	found15Min := false
	found1Hour := false
	for _, alert := range alerts1 {
		if alert.Offset == 15*time.Minute {
			found15Min = true
			if alert.Description != "15 minute warning" {
				t.Errorf("Expected '15 minute warning', got %q", alert.Description)
			}
		} else if alert.Offset == 1*time.Hour {
			found1Hour = true
			if alert.Description != "1 hour warning" {
				t.Errorf("Expected '1 hour warning', got %q", alert.Description)
			}
		}
	}

	if !found15Min {
		t.Error("Expected to find 15 minute alert")
	}
	if !found1Hour {
		t.Error("Expected to find 1 hour alert")
	}

	// Test parsing VALARMs for second event
	alerts2, err := parser.parseVALARMs(icsData, "test-event-2@example.com")
	if err != nil {
		t.Fatalf("Unexpected error parsing VALARMs for event 2: %v", err)
	}

	if len(alerts2) != 1 {
		t.Errorf("Expected 1 alert for event 2, got %d", len(alerts2))
	}

	if alerts2[0].Offset != 30*time.Minute {
		t.Errorf("Expected 30 minute offset, got %v", alerts2[0].Offset)
	}

	// Test parsing VALARMs for non-existent event
	alerts3, err := parser.parseVALARMs(icsData, "non-existent@example.com")
	if err != nil {
		t.Fatalf("Unexpected error parsing VALARMs for non-existent event: %v", err)
	}

	if len(alerts3) != 0 {
		t.Errorf("Expected 0 alerts for non-existent event, got %d", len(alerts3))
	}
}