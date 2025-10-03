package alerts

import (
	"fmt"
	"os"
	"time"

	"calwatch/internal/config"
	"calwatch/internal/storage"
)

// AlertRequest represents a request to send a notification
type AlertRequest struct {
	Event       storage.Event
	AlertOffset time.Duration
	Template    string
}

// AlertScheduler manages alert timing and scheduling logic
type AlertScheduler interface {
	CheckAlerts() []AlertRequest
	ScheduleNextCheck() time.Duration
	SetEventStorage(storage storage.EventStorage)
	SetDirectoryConfigs(configs []config.DirectoryConfig)
	GetNextCheckTime() time.Time
}

// MinuteBasedScheduler implements AlertScheduler with minute-level precision
type MinuteBasedScheduler struct {
	eventStorage     storage.EventStorage
	directoryConfigs []config.DirectoryConfig
	lastCheckTime    time.Time
}

// NewMinuteBasedScheduler creates a new minute-based alert scheduler
func NewMinuteBasedScheduler() *MinuteBasedScheduler {
	return &MinuteBasedScheduler{
		lastCheckTime: time.Now().Truncate(time.Minute),
	}
}

// SetEventStorage sets the event storage to use for checking alerts
func (s *MinuteBasedScheduler) SetEventStorage(storage storage.EventStorage) {
	s.eventStorage = storage
}

// SetDirectoryConfigs sets the directory configurations for alert timing
func (s *MinuteBasedScheduler) SetDirectoryConfigs(configs []config.DirectoryConfig) {
	s.directoryConfigs = configs
}

// CheckAlerts checks for events that should trigger alerts right now
func (s *MinuteBasedScheduler) CheckAlerts() []AlertRequest {
	if s.eventStorage == nil {
		return nil
	}

	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	
	// Get events for today and tomorrow (to catch alerts for events starting early tomorrow)
	var allEvents []storage.Event
	allEvents = append(allEvents, s.eventStorage.GetEventsForDay(today)...)
	allEvents = append(allEvents, s.eventStorage.GetEventsForDay(today.Add(24*time.Hour))...)

	var alertRequests []AlertRequest

	// Check each event against all directory configs to find applicable alerts
	for _, event := range allEvents {
		alertRequests = append(alertRequests, s.checkEventAlerts(event, now)...)
	}

	s.lastCheckTime = now.Truncate(time.Minute)
	return alertRequests
}

// checkEventAlerts checks a single event for all applicable alerts
func (s *MinuteBasedScheduler) checkEventAlerts(event storage.Event, now time.Time) []AlertRequest {
	var requests []AlertRequest

	// For each directory config, check if any alerts should fire
	for _, dirConfig := range s.directoryConfigs {
		for _, alertConfig := range dirConfig.AutomaticAlerts {
			alertOffset, err := alertConfig.Duration()
			if err != nil {
				continue // Skip invalid alert configs
			}

			// Check if this event should alert for this offset
			if event.ShouldAlert(now, alertOffset) {
				// Mark alert as sent to prevent duplicates
				event.SetAlertState(alertOffset, storage.AlertSent)

				// Create alert request
				request := AlertRequest{
					Event:       event,
					AlertOffset: alertOffset,
					Template:    dirConfig.Template,
				}
				requests = append(requests, request)
			}
		}
	}

	return requests
}

// ScheduleNextCheck returns the duration until the next check should occur
func (s *MinuteBasedScheduler) ScheduleNextCheck() time.Duration {
	now := time.Now()
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	return nextMinute.Sub(now)
}

// GetNextCheckTime returns the absolute time of the next check
func (s *MinuteBasedScheduler) GetNextCheckTime() time.Time {
	now := time.Now()
	return now.Truncate(time.Minute).Add(time.Minute)
}

// AdvancedAlertScheduler provides more sophisticated alert scheduling
type AdvancedAlertScheduler struct {
	*MinuteBasedScheduler
	alertHistory map[string]time.Time // Track when alerts were last sent
}

// NewAdvancedAlertScheduler creates a new advanced alert scheduler
func NewAdvancedAlertScheduler() *AdvancedAlertScheduler {
	return &AdvancedAlertScheduler{
		MinuteBasedScheduler: NewMinuteBasedScheduler(),
		alertHistory:         make(map[string]time.Time),
	}
}

// CheckAlerts checks for events with additional duplicate prevention
func (s *AdvancedAlertScheduler) CheckAlerts() []AlertRequest {
	requests := s.MinuteBasedScheduler.CheckAlerts()
	
	// Filter out requests that were sent too recently
	var filteredRequests []AlertRequest
	now := time.Now()
	
	for _, request := range requests {
		alertKey := s.getAlertKey(request)
		
		// Check if we've sent this alert recently (within the last hour)
		if lastSent, exists := s.alertHistory[alertKey]; exists {
			if now.Sub(lastSent) < time.Hour {
				continue // Skip this alert, sent too recently
			}
		}
		
		// Record this alert as being sent
		s.alertHistory[alertKey] = now
		filteredRequests = append(filteredRequests, request)
	}
	
	// Clean up old entries from alert history (older than 24 hours)
	s.cleanupAlertHistory(now)
	
	return filteredRequests
}

// getAlertKey creates a unique key for an alert to track duplicates
func (s *AdvancedAlertScheduler) getAlertKey(request AlertRequest) string {
	return fmt.Sprintf("%s:%s:%s", 
		request.Event.GetUID(), 
		request.Event.GetStartTime().Format("2006-01-02T15:04:05"),
		request.AlertOffset.String(),
	)
}

// cleanupAlertHistory removes old entries from alert history
func (s *AdvancedAlertScheduler) cleanupAlertHistory(now time.Time) {
	cutoff := now.Add(-24 * time.Hour)
	
	for key, timestamp := range s.alertHistory {
		if timestamp.Before(cutoff) {
			delete(s.alertHistory, key)
		}
	}
}

// AlertStats provides statistics about the alert system
type AlertStats struct {
	TotalEvents      int
	PendingAlerts    int
	SentAlerts       int
	UpcomingEvents   int
	LastCheckTime    time.Time
	NextCheckTime    time.Time
}

// GetAlertStats returns statistics about the current alert state
func (s *MinuteBasedScheduler) GetAlertStats() AlertStats {
	stats := AlertStats{
		LastCheckTime: s.lastCheckTime,
		NextCheckTime: s.GetNextCheckTime(),
	}

	if s.eventStorage == nil {
		return stats
	}

	// Count total events
	allEvents := s.eventStorage.GetAllEvents()
	stats.TotalEvents = len(allEvents)

	// Count upcoming events (next 7 days)
	now := time.Now()
	upcomingEvents := s.eventStorage.GetUpcomingEvents(now, 7*24*time.Hour)
	stats.UpcomingEvents = len(upcomingEvents)

	// Count pending and sent alerts for today's events
	today := now.Truncate(24 * time.Hour)
	todaysEvents := s.eventStorage.GetEventsForDay(today)

	for _, event := range todaysEvents {
		for _, dirConfig := range s.directoryConfigs {
			for _, alertConfig := range dirConfig.AutomaticAlerts {
				alertOffset, err := alertConfig.Duration()
				if err != nil {
					continue
				}

				alertState := event.GetAlertState(alertOffset)
				switch alertState {
				case storage.AlertPending:
					stats.PendingAlerts++
				case storage.AlertSent:
					stats.SentAlerts++
				}
			}
		}
	}

	return stats
}

// AlertManager coordinates multiple schedulers and provides a unified interface
type AlertManager struct {
	scheduler        AlertScheduler
	isRunning        bool
	stopChan         chan struct{}
	alertChan        chan []AlertRequest
	tickerInterval   time.Duration
}

// NewAlertManager creates a new alert manager
func NewAlertManager(scheduler AlertScheduler) *AlertManager {
	return &AlertManager{
		scheduler:      scheduler,
		stopChan:       make(chan struct{}),
		alertChan:      make(chan []AlertRequest, 10), // Buffered channel
		tickerInterval: time.Minute,
	}
}

// Start starts the alert manager
func (am *AlertManager) Start() error {
	if am.isRunning {
		return fmt.Errorf("alert manager is already running")
	}

	am.isRunning = true
	go am.run()
	return nil
}

// Stop stops the alert manager
func (am *AlertManager) Stop() error {
	if !am.isRunning {
		return nil
	}

	am.isRunning = false
	close(am.stopChan)
	return nil
}

// GetAlertChannel returns the channel for receiving alert requests
func (am *AlertManager) GetAlertChannel() <-chan []AlertRequest {
	return am.alertChan
}

// run is the main loop for the alert manager
func (am *AlertManager) run() {
	// Calculate initial delay to sync with the next minute boundary
	initialDelay := am.scheduler.ScheduleNextCheck()
	timer := time.NewTimer(initialDelay)

	for {
		select {
		case <-timer.C:
			// Check for alerts
			alertRequests := am.scheduler.CheckAlerts()
			if len(alertRequests) > 0 {
				// Send alerts through channel (non-blocking)
				select {
				case am.alertChan <- alertRequests:
				default:
					// Channel is full, drop the alerts (shouldn't happen with buffered channel)
					fmt.Fprintf(os.Stderr, "Alert channel full, dropping %d alerts\n", len(alertRequests))
				}
			}

			// Schedule next check
			nextDelay := am.scheduler.ScheduleNextCheck()
			timer.Reset(nextDelay)

		case <-am.stopChan:
			timer.Stop()
			close(am.alertChan)
			return
		}
	}
}