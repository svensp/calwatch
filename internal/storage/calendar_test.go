package storage

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"calwatch/internal/config"
)

func TestNewCalendar(t *testing.T) {
	alerts := []Alert{
		{
			Offset:      15 * time.Minute,
			Important:   false,
			Source:      AlertSourceConfig,
			Description: "15 minute warning",
			Action:      AlertActionDisplay,
		},
	}

	calendar := NewCalendar("/test/path", "test.tpl", alerts)

	if calendar.GetPath() != "/test/path" {
		t.Errorf("Expected path '/test/path', got %s", calendar.GetPath())
	}

	if calendar.GetTemplate() != "test.tpl" {
		t.Errorf("Expected template 'test.tpl', got %s", calendar.GetTemplate())
	}

	automaticAlerts := calendar.GetAutomaticAlerts()
	if len(automaticAlerts) != 1 {
		t.Errorf("Expected 1 automatic alert, got %d", len(automaticAlerts))
	}

	if automaticAlerts[0].Offset != 15*time.Minute {
		t.Errorf("Expected 15 minute offset, got %v", automaticAlerts[0].Offset)
	}
}

func TestCalendar_UpdateAutomaticAlerts(t *testing.T) {
	calendar := NewCalendar("/test/path", "test.tpl", []Alert{})

	newAlerts := []Alert{
		{
			Offset:      30 * time.Minute,
			Important:   true,
			Source:      AlertSourceConfig,
			Description: "30 minute warning",
			Action:      AlertActionDisplay,
		},
	}

	calendar.UpdateAutomaticAlerts(newAlerts)

	alerts := calendar.GetAutomaticAlerts()
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert after update, got %d", len(alerts))
	}

	if alerts[0].Offset != 30*time.Minute {
		t.Errorf("Expected 30 minute offset, got %v", alerts[0].Offset)
	}

	if !alerts[0].Important {
		t.Error("Expected alert to be important")
	}
}

func TestCalendar_UpdateTemplate(t *testing.T) {
	calendar := NewCalendar("/test/path", "old.tpl", []Alert{})

	calendar.UpdateTemplate("new.tpl")

	if calendar.GetTemplate() != "new.tpl" {
		t.Errorf("Expected template 'new.tpl', got %s", calendar.GetTemplate())
	}
}

func TestCalendar_EventManagement(t *testing.T) {
	calendar := NewCalendar("/test/path", "test.tpl", []Alert{})

	// Create mock event
	mockEvent := &MockEvent{
		uid:     "test-event-1",
		summary: "Test Event",
	}

	// Test adding event
	calendar.AddEvent(mockEvent)

	event, exists := calendar.GetEvent("test-event-1")
	if !exists {
		t.Error("Expected event to exist after adding")
	}

	if event.GetUID() != "test-event-1" {
		t.Errorf("Expected UID 'test-event-1', got %s", event.GetUID())
	}

	// Test getting all events
	allEvents := calendar.GetAllEvents()
	if len(allEvents) != 1 {
		t.Errorf("Expected 1 event, got %d", len(allEvents))
	}

	// Test removing event
	calendar.RemoveEvent("test-event-1")

	_, exists = calendar.GetEvent("test-event-1")
	if exists {
		t.Error("Expected event to not exist after removal")
	}

	allEvents = calendar.GetAllEvents()
	if len(allEvents) != 0 {
		t.Errorf("Expected 0 events after removal, got %d", len(allEvents))
	}
}

func TestCalendar_ConcurrentAccess(t *testing.T) {
	calendar := NewCalendar("/test/path", "test.tpl", []Alert{})

	// Test concurrent alert updates
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			alerts := []Alert{
				{
					Offset:      time.Duration(index) * time.Minute,
					Important:   index%2 == 0,
					Source:      AlertSourceConfig,
					Description: "Test alert",
					Action:      AlertActionDisplay,
				},
			}
			calendar.UpdateAutomaticAlerts(alerts)
		}(i)
	}

	wg.Wait()

	// Should have exactly one alert (the last one to complete)
	alerts := calendar.GetAutomaticAlerts()
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert after concurrent updates, got %d", len(alerts))
	}
}

func TestCalendar_ConcurrentEventAccess(t *testing.T) {
	calendar := NewCalendar("/test/path", "test.tpl", []Alert{})

	// Test concurrent event additions
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			mockEvent := &MockEvent{
				uid:     fmt.Sprintf("event-%d", index),
				summary: fmt.Sprintf("Event %d", index),
			}
			calendar.AddEvent(mockEvent)
		}(i)
	}

	wg.Wait()

	// Should have 10 events
	allEvents := calendar.GetAllEvents()
	if len(allEvents) != 10 {
		t.Errorf("Expected 10 events after concurrent additions, got %d", len(allEvents))
	}
}

func TestCalendar_GetEventsForDay(t *testing.T) {
	calendar := NewCalendar("/test/path", "test.tpl", []Alert{})

	// Create mock events with different dates
	date1 := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	date2 := time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC)

	mockEvent1 := &MockEvent{
		uid:       "event-1",
		summary:   "Event 1",
		startTime: date1,
		occursOnDates: map[string]bool{
			"2024-01-15": true,
		},
	}

	mockEvent2 := &MockEvent{
		uid:       "event-2",
		summary:   "Event 2",
		startTime: date2,
		occursOnDates: map[string]bool{
			"2024-01-16": true,
		},
	}

	calendar.AddEvent(mockEvent1)
	calendar.AddEvent(mockEvent2)

	// Test getting events for specific days
	eventsFor15th := calendar.GetEventsForDay(date1)
	if len(eventsFor15th) != 1 {
		t.Errorf("Expected 1 event for Jan 15th, got %d", len(eventsFor15th))
	}

	eventsFor16th := calendar.GetEventsForDay(date2)
	if len(eventsFor16th) != 1 {
		t.Errorf("Expected 1 event for Jan 16th, got %d", len(eventsFor16th))
	}

	// Test getting events for a day with no events
	date3 := time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC)
	eventsFor17th := calendar.GetEventsForDay(date3)
	if len(eventsFor17th) != 0 {
		t.Errorf("Expected 0 events for Jan 17th, got %d", len(eventsFor17th))
	}
}

// MockEvent for testing
type MockEvent struct {
	uid           string
	summary       string
	startTime     time.Time
	occursOnDates map[string]bool
}

func (m *MockEvent) GetUID() string {
	return m.uid
}

func (m *MockEvent) GetSummary() string {
	return m.summary
}

func (m *MockEvent) GetDescription() string {
	return ""
}

func (m *MockEvent) GetLocation() string {
	return ""
}

func (m *MockEvent) GetStartTime() time.Time {
	return m.startTime
}

func (m *MockEvent) GetEndTime() time.Time {
	return m.startTime.Add(time.Hour)
}

func (m *MockEvent) GetTimezone() *time.Location {
	return time.UTC
}

func (m *MockEvent) OccursOn(date time.Time) bool {
	dateStr := date.Format("2006-01-02")
	return m.occursOnDates[dateStr]
}

func (m *MockEvent) OccurredWithin(start, end time.Time) []time.Time {
	return []time.Time{m.startTime}
}

func (m *MockEvent) OccurrencesWithin(start, end time.Time) []Occurrence {
	return []Occurrence{}
}

func (m *MockEvent) GetAllAlerts() []Alert {
	return []Alert{}
}

func (m *MockEvent) GetIntrinsicAlerts() []Alert {
	return []Alert{}
}

func (m *MockEvent) GetAutomaticAlerts() []Alert {
	return []Alert{}
}

func (m *MockEvent) GetAlertState(alertOffset time.Duration) AlertState {
	return AlertPending
}

func (m *MockEvent) SetAlertState(alertOffset time.Duration, state AlertState) {
}

// Test Alert conversion utilities
func TestConvertConfigAlert(t *testing.T) {
	tests := []struct {
		name        string
		alertConfig config.AlertConfig
		wantOffset  time.Duration
		wantErr     bool
	}{
		{
			name: "valid minutes config",
			alertConfig: config.AlertConfig{
				Value:     15,
				Unit:      "minutes",
				Important: false,
			},
			wantOffset: 15 * time.Minute,
			wantErr:    false,
		},
		{
			name: "valid hours config",
			alertConfig: config.AlertConfig{
				Value:     2,
				Unit:      "hours",
				Important: true,
			},
			wantOffset: 2 * time.Hour,
			wantErr:    false,
		},
		{
			name: "valid days config",
			alertConfig: config.AlertConfig{
				Value:     1,
				Unit:      "days",
				Important: false,
			},
			wantOffset: 24 * time.Hour,
			wantErr:    false,
		},
		{
			name: "invalid unit",
			alertConfig: config.AlertConfig{
				Value:     5,
				Unit:      "invalid",
				Important: false,
			},
			wantOffset: 0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alert, err := ConvertConfigAlert(tt.alertConfig)
			
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

			if alert.Offset != tt.wantOffset {
				t.Errorf("Expected offset %v, got %v", tt.wantOffset, alert.Offset)
			}

			if alert.Important != tt.alertConfig.Important {
				t.Errorf("Expected important %v, got %v", tt.alertConfig.Important, alert.Important)
			}

			if alert.Source != AlertSourceConfig {
				t.Errorf("Expected source %v, got %v", AlertSourceConfig, alert.Source)
			}

			if alert.Action != AlertActionDisplay {
				t.Errorf("Expected action %v, got %v", AlertActionDisplay, alert.Action)
			}

			expectedDesc := fmt.Sprintf("%d %s warning", tt.alertConfig.Value, tt.alertConfig.Unit)
			if alert.Description != expectedDesc {
				t.Errorf("Expected description %q, got %q", expectedDesc, alert.Description)
			}
		})
	}
}

func TestConvertConfigAlerts(t *testing.T) {
	alertConfigs := []config.AlertConfig{
		{Value: 15, Unit: "minutes", Important: false},
		{Value: 1, Unit: "hours", Important: true},
	}

	alerts, err := ConvertConfigAlerts(alertConfigs)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(alerts) != 2 {
		t.Errorf("Expected 2 alerts, got %d", len(alerts))
	}

	// Check first alert
	if alerts[0].Offset != 15*time.Minute {
		t.Errorf("Expected first alert offset 15m, got %v", alerts[0].Offset)
	}

	if alerts[0].Important {
		t.Error("Expected first alert to not be important")
	}

	// Check second alert
	if alerts[1].Offset != time.Hour {
		t.Errorf("Expected second alert offset 1h, got %v", alerts[1].Offset)
	}

	if !alerts[1].Important {
		t.Error("Expected second alert to be important")
	}
}

func TestConvertConfigAlerts_WithError(t *testing.T) {
	alertConfigs := []config.AlertConfig{
		{Value: 15, Unit: "minutes", Important: false},
		{Value: 1, Unit: "invalid", Important: true},
	}

	_, err := ConvertConfigAlerts(alertConfigs)
	if err == nil {
		t.Error("Expected error due to invalid unit")
	}
}

func TestDeduplicateAlerts(t *testing.T) {
	alerts := []Alert{
		{
			Offset:    15 * time.Minute,
			Source:    AlertSourceConfig,
			Important: false,
		},
		{
			Offset:    15 * time.Minute,
			Source:    AlertSourceVALARM,
			Important: true,
		},
		{
			Offset:    30 * time.Minute,
			Source:    AlertSourceConfig,
			Important: false,
		},
		{
			Offset:    1 * time.Hour,
			Source:    AlertSourceConfig,
			Important: true,
		},
	}

	unique := DeduplicateAlerts(alerts)

	if len(unique) != 3 {
		t.Errorf("Expected 3 unique alerts, got %d", len(unique))
	}

	// Check that VALARM alert at 15 minutes takes precedence
	found15MinVALARM := false
	found15MinConfig := false
	for _, alert := range unique {
		if alert.Offset == 15*time.Minute {
			if alert.Source == AlertSourceVALARM {
				found15MinVALARM = true
			}
			if alert.Source == AlertSourceConfig {
				found15MinConfig = true
			}
		}
	}

	if !found15MinVALARM {
		t.Error("Expected VALARM alert at 15 minutes to be present")
	}

	if found15MinConfig {
		t.Error("Expected config alert at 15 minutes to be removed due to VALARM precedence")
	}
}