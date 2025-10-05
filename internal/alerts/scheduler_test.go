package alerts

import (
	"testing"
	"time"

	"calwatch/internal/config"
	"calwatch/internal/recurrence"
	"calwatch/internal/storage"
)

func TestMinuteBasedScheduler_ScheduleNextCheck(t *testing.T) {
	scheduler := NewMinuteBasedScheduler()

	duration := scheduler.ScheduleNextCheck()

	// Should be less than a minute (time until next minute boundary)
	if duration >= time.Minute {
		t.Errorf("Expected duration < 1 minute, got %v", duration)
	}

	if duration <= 0 {
		t.Errorf("Expected positive duration, got %v", duration)
	}
}

func TestMinuteBasedScheduler_GetNextCheckTime(t *testing.T) {
	scheduler := NewMinuteBasedScheduler()

	nextCheck := scheduler.GetNextCheckTime()
	now := time.Now()

	// Next check should be in the future
	if !nextCheck.After(now) {
		t.Error("Next check time should be in the future")
	}

	// Should be aligned to minute boundary
	if nextCheck.Second() != 0 || nextCheck.Nanosecond() != 0 {
		t.Error("Next check time should be aligned to minute boundary")
	}
}

func TestMinuteBasedScheduler_CheckAlerts(t *testing.T) {
	scheduler := NewMinuteBasedScheduler()
	eventStorage := storage.NewMemoryEventStorage()
	scheduler.SetEventStorage(eventStorage)

	// Create test directory config with only 5-minute alerts
	dirConfig := config.DirectoryConfig{
		Directory: "/test",
		Template:  "test.tpl",
		AutomaticAlerts: []config.AlertConfig{
			{Value: 5, Unit: "minutes"},
		},
	}
	scheduler.SetDirectoryConfigs([]config.DirectoryConfig{dirConfig})

	// Create test event that should trigger alerts
	now := time.Now()
	eventTime := now.Add(30 * time.Minute) // Event starts in 30 minutes (not exactly 5 minutes or 1 hour)

	// Create test calendar with alerts  
	testAlerts := []storage.Alert{
		{Offset: 5 * time.Minute, Important: false, Source: storage.AlertSourceConfig, Action: storage.AlertActionDisplay},
		{Offset: 1 * time.Hour, Important: true, Source: storage.AlertSourceConfig, Action: storage.AlertActionDisplay},
	}
	calendar := storage.NewCalendar("/test/path", "test.tpl", testAlerts)
	
	event := storage.NewCalendarEvent(
		"test-event",
		"Test Meeting",
		"A test meeting",
		"Conference Room",
		eventTime,
		eventTime.Add(time.Hour),
		time.UTC,
		&recurrence.NoRecurrence{},
		calendar,
		[]storage.Alert{},
	)

	// Add event to storage
	eventStorage.UpsertEvent(event)

	// Check alerts
	alerts := scheduler.CheckAlerts()

	// Should have no alerts (event is in 30 minutes, no alerts configured for that offset)
	if len(alerts) != 0 {
		t.Fatalf("Expected 0 alerts for event in 30 minutes, got %d", len(alerts))
	}

	// Now test with event that should trigger 5-minute alert
	eventTime2 := now.Add(5 * time.Minute) // Event starts in exactly 5 minutes
	event2 := storage.NewCalendarEvent(
		"test-event-2",
		"Imminent Meeting",
		"A meeting starting soon",
		"Conference Room",
		eventTime2,
		eventTime2.Add(time.Hour),
		time.UTC,
		&recurrence.NoRecurrence{},
		calendar,
		[]storage.Alert{},
	)
	eventStorage.UpsertEvent(event2)

	// Check alerts again
	alerts = scheduler.CheckAlerts()

	// Should have one alert (5 minutes before the second event)
	if len(alerts) != 1 {
		t.Fatalf("Expected 1 alert for event in 5 minutes, got %d", len(alerts))
	}

	alert := alerts[0]
	if alert.Event.GetUID() != "test-event-2" {
		t.Errorf("Expected event UID 'test-event-2', got '%s'", alert.Event.GetUID())
	}

	if alert.AlertOffset != 5*time.Minute {
		t.Errorf("Expected alert offset 5m, got %v", alert.AlertOffset)
	}

	if alert.Template != "test.tpl" {
		t.Errorf("Expected template 'test.tpl', got '%s'", alert.Template)
	}

	// Check alerts again - should not get duplicate alerts
	alerts = scheduler.CheckAlerts()
	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts on second check (no duplicates), got %d", len(alerts))
	}
}

func TestMinuteBasedScheduler_CheckAlertsNoEvents(t *testing.T) {
	scheduler := NewMinuteBasedScheduler()
	eventStorage := storage.NewMemoryEventStorage()
	scheduler.SetEventStorage(eventStorage)

	// No directory configs set
	alerts := scheduler.CheckAlerts()

	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts with no events, got %d", len(alerts))
	}
}

func TestMinuteBasedScheduler_CheckAlertsNoStorage(t *testing.T) {
	scheduler := NewMinuteBasedScheduler()
	// Don't set event storage

	alerts := scheduler.CheckAlerts()

	if alerts != nil {
		t.Error("Expected nil alerts with no storage set")
	}
}

func TestAdvancedAlertScheduler_DuplicatePrevention(t *testing.T) {
	scheduler := NewAdvancedAlertScheduler()
	eventStorage := storage.NewMemoryEventStorage()
	scheduler.SetEventStorage(eventStorage)

	// Create test directory config
	dirConfig := config.DirectoryConfig{
		Directory: "/test",
		Template:  "test.tpl",
		AutomaticAlerts: []config.AlertConfig{
			{Value: 5, Unit: "minutes"},
		},
	}
	scheduler.SetDirectoryConfigs([]config.DirectoryConfig{dirConfig})

	// Create test event
	now := time.Now()
	eventTime := now.Add(5 * time.Minute)

	// Create test calendar with 5-minute alert
	testAlerts := []storage.Alert{
		{Offset: 5 * time.Minute, Important: false, Source: storage.AlertSourceConfig, Action: storage.AlertActionDisplay},
	}
	calendar := storage.NewCalendar("/test/path", "test.tpl", testAlerts)

	event := storage.NewCalendarEvent(
		"test-event",
		"Test Meeting",
		"A test meeting",
		"Conference Room",
		eventTime,
		eventTime.Add(time.Hour),
		time.UTC,
		&recurrence.NoRecurrence{},
		calendar,
		[]storage.Alert{},
	)

	eventStorage.UpsertEvent(event)

	// First check should return alerts
	alerts := scheduler.CheckAlerts()
	if len(alerts) != 1 {
		t.Fatalf("Expected 1 alert on first check, got %d", len(alerts))
	}

	// Reset the event's alert state to simulate the same condition
	event.SetAlertState(5*time.Minute, storage.AlertPending)

	// Second check immediately should not return alerts (due to history tracking)
	alerts = scheduler.CheckAlerts()
	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts on second check (duplicate prevention), got %d", len(alerts))
	}
}

func TestMinuteBasedScheduler_GetAlertStats(t *testing.T) {
	scheduler := NewMinuteBasedScheduler()
	eventStorage := storage.NewMemoryEventStorage()
	scheduler.SetEventStorage(eventStorage)

	// Create test directory config
	dirConfig := config.DirectoryConfig{
		Directory: "/test",
		Template:  "test.tpl",
		AutomaticAlerts: []config.AlertConfig{
			{Value: 5, Unit: "minutes"},
		},
	}
	scheduler.SetDirectoryConfigs([]config.DirectoryConfig{dirConfig})

	// Get stats with no events
	stats := scheduler.GetAlertStats()
	if stats.TotalEvents != 0 {
		t.Errorf("Expected 0 total events, got %d", stats.TotalEvents)
	}

	// Add a test event
	now := time.Now()
	
	// Create test calendar with 5-minute alert
	testAlerts := []storage.Alert{
		{Offset: 5 * time.Minute, Important: false, Source: storage.AlertSourceConfig, Action: storage.AlertActionDisplay},
	}
	calendar := storage.NewCalendar("/test/path", "test.tpl", testAlerts)
	
	event := storage.NewCalendarEvent(
		"test-event",
		"Test Meeting",
		"Test Description",
		"Test Location",
		now.Add(time.Hour),
		now.Add(2*time.Hour),
		time.UTC,
		&recurrence.NoRecurrence{},
		calendar,
		[]storage.Alert{},
	)
	eventStorage.UpsertEvent(event)

	// Get stats with one event
	stats = scheduler.GetAlertStats()
	if stats.TotalEvents != 1 {
		t.Errorf("Expected 1 total event, got %d", stats.TotalEvents)
	}

	if stats.LastCheckTime.IsZero() {
		t.Error("Expected non-zero last check time")
	}

	if stats.NextCheckTime.IsZero() {
		t.Error("Expected non-zero next check time")
	}
}

func TestAlertManager_StartStop(t *testing.T) {
	scheduler := NewMinuteBasedScheduler()
	manager := NewAlertManager(scheduler)

	// Test start
	err := manager.Start()
	if err != nil {
		t.Fatalf("Failed to start alert manager: %v", err)
	}

	if !manager.isRunning {
		t.Error("Alert manager should be running after start")
	}

	// Test double start (should fail)
	err = manager.Start()
	if err == nil {
		t.Error("Expected error when starting already running manager")
	}

	// Test stop
	err = manager.Stop()
	if err != nil {
		t.Fatalf("Failed to stop alert manager: %v", err)
	}

	if manager.isRunning {
		t.Error("Alert manager should not be running after stop")
	}

	// Test double stop (should succeed)
	err = manager.Stop()
	if err != nil {
		t.Errorf("Second stop should not fail: %v", err)
	}
}

func TestAlertManager_AlertChannel(t *testing.T) {
	scheduler := NewMinuteBasedScheduler()
	manager := NewAlertManager(scheduler)

	// Get alert channel
	alertChan := manager.GetAlertChannel()
	if alertChan == nil {
		t.Error("Alert channel should not be nil")
	}

	// Channel should be readable (but empty before starting)
	select {
	case <-alertChan:
		t.Error("Alert channel should be empty before starting")
	default:
		// Expected - channel is empty
	}
}