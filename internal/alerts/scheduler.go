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
	CheckMissedAlerts(lastTick, currentTime time.Time, wakeupConfig config.WakeupHandlingConfig) []AlertRequest
	ScheduleNextCheck() time.Duration
	SetEventStorage(storage storage.EventStorage)
	SetDirectoryConfigs(configs []config.DirectoryConfig)
	SetStateManager(stateManager storage.StateManager)
	GetNextCheckTime() time.Time
	DetectWakeup() (bool, time.Duration)
}

// MinuteBasedScheduler implements AlertScheduler with minute-level precision
type MinuteBasedScheduler struct {
	eventStorage        storage.EventStorage
	directoryConfigs    []config.DirectoryConfig
	stateManager        storage.StateManager
	priorityClassifier  *PriorityClassifier
	lastCheckTime       time.Time
}

// NewMinuteBasedScheduler creates a new minute-based alert scheduler
func NewMinuteBasedScheduler() *MinuteBasedScheduler {
	return &MinuteBasedScheduler{
		priorityClassifier: NewPriorityClassifier(),
		lastCheckTime:      time.Now().Truncate(time.Minute),
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

// SetStateManager sets the state manager for persistent state tracking
func (s *MinuteBasedScheduler) SetStateManager(stateManager storage.StateManager) {
	s.stateManager = stateManager
}

// CheckAlerts checks for events that should trigger alerts right now
func (s *MinuteBasedScheduler) CheckAlerts() []AlertRequest {
	if s.eventStorage == nil {
		return nil
	}

	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	
	// Get the last tick time - use current time minus 1 minute as fallback for first run
	var lastTick time.Time
	if s.stateManager != nil {
		lastTick = s.stateManager.GetLastAlertTick()
	}
	if lastTick.IsZero() {
		lastTick = now.Add(-time.Minute) // Default to 1 minute ago for first run
	}
	
	// Get events for today and tomorrow (to catch alerts for events starting early tomorrow)
	var allEvents []storage.Event
	allEvents = append(allEvents, s.eventStorage.GetEventsForDay(today)...)
	allEvents = append(allEvents, s.eventStorage.GetEventsForDay(today.Add(24*time.Hour))...)

	var alertRequests []AlertRequest

	// Check each event against all directory configs to find applicable alerts
	for _, event := range allEvents {
		alertRequests = append(alertRequests, s.checkEventAlerts(event, lastTick, now)...)
	}

	s.lastCheckTime = now.Truncate(time.Minute)
	
	// Update last alert tick in state manager if available
	if s.stateManager != nil {
		s.stateManager.SetLastAlertTick(s.lastCheckTime)
	}
	
	return alertRequests
}

// checkEventAlerts checks a single event for all applicable alerts
func (s *MinuteBasedScheduler) checkEventAlerts(event storage.Event, lastTick, now time.Time) []AlertRequest {
	var requests []AlertRequest

	// For each directory config, check if any alerts should fire
	for _, dirConfig := range s.directoryConfigs {
		for _, alertConfig := range dirConfig.AutomaticAlerts {
			alertOffset, err := alertConfig.Duration()
			if err != nil {
				continue // Skip invalid alert configs
			}

			// Check if this event should alert for this offset using range-based checking
			if event.ShouldAlert(lastTick, now, alertOffset) {
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

// DetectWakeup detects if the system has been asleep/shutdown by comparing current time with last tick
func (s *MinuteBasedScheduler) DetectWakeup() (bool, time.Duration) {
	if s.stateManager == nil {
		return false, 0
	}
	
	now := time.Now()
	lastTick := s.stateManager.GetLastAlertTick()
	
	// If we don't have a last tick, this might be first run
	if lastTick.IsZero() {
		return false, 0
	}
	
	// Calculate time gap since last tick
	gap := now.Sub(lastTick)
	
	// Consider it a wake-up if gap is more than 2 minutes
	// (normal minute-based checking should have max 1 minute gap)
	wakeupThreshold := 2 * time.Minute
	
	return gap > wakeupThreshold, gap
}

// CheckMissedAlerts processes missed events during sleep/shutdown periods
func (s *MinuteBasedScheduler) CheckMissedAlerts(lastTick, currentTime time.Time, wakeupConfig config.WakeupHandlingConfig) []AlertRequest {
	if s.eventStorage == nil || !wakeupConfig.Enable {
		return nil
	}
	
	// Skip if policy is to skip missed events
	if wakeupConfig.MissedEventPolicy == "skip" {
		return nil
	}
	
	// Calculate missed period with limits
	missedStart := lastTick
	missedEnd := currentTime
	
	// Limit how far back we process (respect max_missed_days)
	maxLookback := time.Duration(wakeupConfig.MaxMissedDays) * 24 * time.Hour
	if missedEnd.Sub(missedStart) > maxLookback {
		missedStart = missedEnd.Add(-maxLookback)
	}
	
	// Get all events within the missed period
	eventsInRange := s.eventStorage.GetEventsWithinRange(missedStart, missedEnd)
	
	var missedAlerts []AlertRequest
	processStart := time.Now()
	
	// Process each event for missed alerts
	for _, event := range eventsInRange {
		// Check if we've exceeded our catchup time limit
		if wakeupConfig.MaxCatchupTime.Type == "timed" {
			if maxCatchupDuration, err := wakeupConfig.MaxCatchupTime.ToDuration(); err == nil {
				if time.Since(processStart) > maxCatchupDuration {
					break // Stop processing to avoid blocking the daemon
				}
			}
		}
		
		// Get all occurrences of this event within the missed period
		occurrences := event.OccurredWithin(missedStart, missedEnd)
		
		for _, occurrence := range occurrences {
			alertRequests := s.checkMissedEventAlerts(event, occurrence, lastTick, wakeupConfig)
			missedAlerts = append(missedAlerts, alertRequests...)
		}
	}
	
	// Apply policy-based filtering
	return s.applyMissedEventPolicy(missedAlerts, wakeupConfig)
}

// checkMissedEventAlerts checks if alerts should have fired for a specific event occurrence
func (s *MinuteBasedScheduler) checkMissedEventAlerts(event storage.Event, occurrence time.Time, lastTick time.Time, wakeupConfig config.WakeupHandlingConfig) []AlertRequest {
	var requests []AlertRequest
	
	// For each directory config, check if any alerts should have fired
	for _, dirConfig := range s.directoryConfigs {
		for _, alertConfig := range dirConfig.AutomaticAlerts {
			alertOffset, err := alertConfig.Duration()
			if err != nil {
				continue
			}
			
			// Calculate when the alert should have fired
			alertTime := occurrence.Add(-alertOffset)
			
			// Check if alert time was during the missed period
			if alertTime.After(lastTick) && alertTime.Before(time.Now()) {
				// This alert was missed, create a missed alert request
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

// applyMissedEventPolicy applies the configured policy to filter missed alerts
func (scheduler *MinuteBasedScheduler) applyMissedEventPolicy(alerts []AlertRequest, wakeupConfig config.WakeupHandlingConfig) []AlertRequest {
	if len(alerts) == 0 {
		return alerts
	}
	
	switch wakeupConfig.MissedEventPolicy {
	case "all":
		return alerts
		
	case "summary":
		if len(alerts) > wakeupConfig.SummaryThreshold {
			// TODO: Create a summary alert instead of individual alerts
			// For now, return first few alerts as a simplified approach
			maxAlerts := wakeupConfig.SummaryThreshold
			if len(alerts) > maxAlerts {
				return alerts[:maxAlerts]
			}
		}
		return alerts
		
	case "priority_only":
		// Filter to only show high and critical priority events
		if scheduler.priorityClassifier != nil {
			return scheduler.priorityClassifier.FilterByPriority(alerts, PriorityHigh)
		}
		return alerts
		
	case "skip":
		return nil
		
	default:
		return alerts
	}
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

// CheckMissedAlerts delegates to the base scheduler
func (s *AdvancedAlertScheduler) CheckMissedAlerts(lastTick, currentTime time.Time, wakeupConfig config.WakeupHandlingConfig) []AlertRequest {
	return s.MinuteBasedScheduler.CheckMissedAlerts(lastTick, currentTime, wakeupConfig)
}

// DetectWakeup delegates to the base scheduler
func (s *AdvancedAlertScheduler) DetectWakeup() (bool, time.Duration) {
	return s.MinuteBasedScheduler.DetectWakeup()
}

// SetStateManager delegates to the base scheduler
func (s *AdvancedAlertScheduler) SetStateManager(stateManager storage.StateManager) {
	s.MinuteBasedScheduler.SetStateManager(stateManager)
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