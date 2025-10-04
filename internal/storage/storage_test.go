package storage

import (
	"testing"
	"time"
)

func TestMemoryEventStorage_UpsertAndGet(t *testing.T) {
	storage := NewMemoryEventStorage()
	
	// Create test event
	event := NewCalendarEvent(
		"test-uid-1",
		"Test Meeting",
		"A test meeting",
		"Conference Room A",
		time.Date(2023, 10, 15, 14, 0, 0, 0, time.UTC),
		time.Date(2023, 10, 15, 15, 0, 0, 0, time.UTC),
		time.UTC,
		"",
	)
	
	// Test upsert
	err := storage.UpsertEvent(event)
	if err != nil {
		t.Fatalf("UpsertEvent failed: %v", err)
	}
	
	// Test retrieval
	events := storage.GetAllEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
	
	if events[0].GetUID() != "test-uid-1" {
		t.Errorf("Expected UID 'test-uid-1', got '%s'", events[0].GetUID())
	}
}

func TestMemoryEventStorage_Delete(t *testing.T) {
	storage := NewMemoryEventStorage()
	
	// Create and add test event
	event := NewCalendarEvent(
		"test-uid-1",
		"Test Meeting",
		"A test meeting",
		"Conference Room A",
		time.Date(2023, 10, 15, 14, 0, 0, 0, time.UTC),
		time.Date(2023, 10, 15, 15, 0, 0, 0, time.UTC),
		time.UTC,
		"",
	)
	
	storage.UpsertEvent(event)
	
	// Verify event exists
	if storage.GetEventCount() != 1 {
		t.Fatalf("Expected 1 event, got %d", storage.GetEventCount())
	}
	
	// Delete event
	err := storage.DeleteEvent("test-uid-1")
	if err != nil {
		t.Fatalf("DeleteEvent failed: %v", err)
	}
	
	// Verify event is deleted
	if storage.GetEventCount() != 0 {
		t.Errorf("Expected 0 events after deletion, got %d", storage.GetEventCount())
	}
}

func TestMemoryEventStorage_GetEventsForDay(t *testing.T) {
	storage := NewMemoryEventStorage()
	
	// Create events for different days
	event1 := NewCalendarEvent(
		"event-1",
		"Meeting 1",
		"",
		"",
		time.Date(2023, 10, 15, 14, 0, 0, 0, time.UTC),
		time.Date(2023, 10, 15, 15, 0, 0, 0, time.UTC),
		time.UTC,
		"",
	)
	
	event2 := NewCalendarEvent(
		"event-2",
		"Meeting 2",
		"",
		"",
		time.Date(2023, 10, 16, 14, 0, 0, 0, time.UTC),
		time.Date(2023, 10, 16, 15, 0, 0, 0, time.UTC),
		time.UTC,
		"",
	)
	
	storage.UpsertEvent(event1)
	storage.UpsertEvent(event2)
	
	// Test getting events for specific day
	events := storage.GetEventsForDay(time.Date(2023, 10, 15, 0, 0, 0, 0, time.UTC))
	
	if len(events) != 1 {
		t.Fatalf("Expected 1 event for Oct 15, got %d", len(events))
	}
	
	if events[0].GetUID() != "event-1" {
		t.Errorf("Expected event-1, got %s", events[0].GetUID())
	}
	
	// Test getting events for day with no events
	events = storage.GetEventsForDay(time.Date(2023, 10, 17, 0, 0, 0, 0, time.UTC))
	
	if len(events) != 0 {
		t.Errorf("Expected 0 events for Oct 17, got %d", len(events))
	}
}

func TestMemoryEventStorage_GetUpcomingEvents(t *testing.T) {
	storage := NewMemoryEventStorage()
	
	now := time.Date(2023, 10, 15, 10, 0, 0, 0, time.UTC)
	
	// Create events at different times
	event1 := NewCalendarEvent(
		"event-1",
		"Soon",
		"",
		"",
		now.Add(30*time.Minute), // 30 minutes from now
		now.Add(90*time.Minute),
		time.UTC,
		"",
	)
	
	event2 := NewCalendarEvent(
		"event-2",
		"Later",
		"",
		"",
		now.Add(2*time.Hour), // 2 hours from now
		now.Add(3*time.Hour),
		time.UTC,
		"",
	)
	
	event3 := NewCalendarEvent(
		"event-3",
		"Much Later",
		"",
		"",
		now.Add(25*time.Hour), // 25 hours from now
		now.Add(26*time.Hour),
		time.UTC,
		"",
	)
	
	storage.UpsertEvent(event1)
	storage.UpsertEvent(event2)
	storage.UpsertEvent(event3)
	
	// Get events in the next hour
	upcoming := storage.GetUpcomingEvents(now, 1*time.Hour)
	
	if len(upcoming) != 1 {
		t.Fatalf("Expected 1 upcoming event in next hour, got %d", len(upcoming))
	}
	
	if upcoming[0].GetUID() != "event-1" {
		t.Errorf("Expected event-1, got %s", upcoming[0].GetUID())
	}
	
	// Get events in the next 24 hours
	upcoming = storage.GetUpcomingEvents(now, 24*time.Hour)
	
	if len(upcoming) != 2 {
		t.Fatalf("Expected 2 upcoming events in next 24 hours, got %d", len(upcoming))
	}
}

func TestCalendarEvent_AlertStates(t *testing.T) {
	event := NewCalendarEvent(
		"test-uid",
		"Test Event",
		"",
		"",
		time.Date(2023, 10, 15, 14, 0, 0, 0, time.UTC),
		time.Date(2023, 10, 15, 15, 0, 0, 0, time.UTC),
		time.UTC,
		"",
	)
	
	alertOffset := 5 * time.Minute
	
	// Test initial state
	if state := event.GetAlertState(alertOffset); state != AlertPending {
		t.Errorf("Expected AlertPending, got %v", state)
	}
	
	// Test setting state
	event.SetAlertState(alertOffset, AlertSent)
	
	if state := event.GetAlertState(alertOffset); state != AlertSent {
		t.Errorf("Expected AlertSent, got %v", state)
	}
	
	// Test different offset has different state
	if state := event.GetAlertState(10 * time.Minute); state != AlertPending {
		t.Errorf("Expected AlertPending for different offset, got %v", state)
	}
}

func TestCalendarEvent_ShouldAlert(t *testing.T) {
	eventTime := time.Date(2023, 10, 15, 14, 0, 0, 0, time.UTC)
	event := NewCalendarEvent(
		"test-uid",
		"Test Event",
		"",
		"",
		eventTime,
		eventTime.Add(time.Hour),
		time.UTC,
		"",
	)
	
	alertOffset := 5 * time.Minute
	
	// Test before alert time - checking from 20 minutes before to 10 minutes before
	lastTick := eventTime.Add(-20 * time.Minute)
	checkTime := eventTime.Add(-10 * time.Minute)
	if event.ShouldAlert(lastTick, checkTime, alertOffset) {
		t.Errorf("Should not alert 10 minutes before when offset is 5 minutes")
	}
	
	// Test at alert time - checking from 10 minutes before to alert time
	lastTick = eventTime.Add(-10 * time.Minute)
	checkTime = eventTime.Add(-alertOffset)
	if !event.ShouldAlert(lastTick, checkTime, alertOffset) {
		t.Errorf("Should alert at exact alert time")
	}
	
	// Test after alert sent
	event.SetAlertState(alertOffset, AlertSent)
	if event.ShouldAlert(lastTick, checkTime, alertOffset) {
		t.Errorf("Should not alert when already sent")
	}
}