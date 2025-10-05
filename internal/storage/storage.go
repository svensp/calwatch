package storage

import (
	"sync"
	"time"
)

// EventStorage manages in-memory event storage with efficient indexing
type EventStorage interface {
	// Event management (existing)
	UpsertEvent(event Event) error
	UpsertEventWithFile(event Event, filename string) error
	DeleteEvent(uid string) error
	DeleteEventByFile(filename string) error
	GetEventsForDay(date time.Time) []Event
	GetEventsWithinRange(start, end time.Time) []Event
	GetUpcomingEvents(from time.Time, duration time.Duration) []Event
	RegenerateIndex(date time.Time) error
	GetAllEvents() []Event
	Clear() error
	
	// Calendar management (new)
	EnsureCalendar(path string, template string, automaticAlerts []Alert) *Calendar
	GetCalendar(path string) (*Calendar, bool)
	GetAllCalendars() map[string]*Calendar
	UpdateCalendarAlerts(path string, automaticAlerts []Alert) error
	RemoveCalendar(path string) error
}

// MemoryEventStorage implements EventStorage using in-memory maps
type MemoryEventStorage struct {
	// Main event storage by UID
	events map[string]Event
	
	// Daily index for fast lookups - map[YYYY-MM-DD][]Event
	dailyIndex map[string][]Event
	
	// File tracking - bidirectional mapping between filenames and UIDs
	fileToUID map[string]string  // filename -> UID
	uidToFile map[string]string  // UID -> filename
	
	// Calendar management - path -> *Calendar
	calendars map[string]*Calendar
	
	// Current indexed date
	currentIndexDate time.Time
	
	// Mutex for thread safety
	mutex sync.RWMutex
}

// NewMemoryEventStorage creates a new in-memory event storage
func NewMemoryEventStorage() *MemoryEventStorage {
	return &MemoryEventStorage{
		events:     make(map[string]Event),
		dailyIndex: make(map[string][]Event),
		fileToUID:  make(map[string]string),
		uidToFile:  make(map[string]string),
		calendars:  make(map[string]*Calendar),
		mutex:      sync.RWMutex{},
	}
}

// UpsertEvent adds or updates an event in storage
func (s *MemoryEventStorage) UpsertEvent(event Event) error {
	return s.UpsertEventWithFile(event, "")
}

// UpsertEventWithFile adds or updates an event in storage with file tracking
func (s *MemoryEventStorage) UpsertEventWithFile(event Event, filename string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	uid := event.GetUID()
	
	// Remove old file mapping if event already exists
	if oldFilename, exists := s.uidToFile[uid]; exists && oldFilename != "" {
		delete(s.fileToUID, oldFilename)
	}
	
	// Store event by UID
	s.events[uid] = event
	
	// Update file mappings if filename provided
	if filename != "" {
		// Remove any existing mapping for this filename (in case file was overwritten)
		if oldUID, exists := s.fileToUID[filename]; exists {
			delete(s.uidToFile, oldUID)
			// Note: we don't delete the old event as it might be from the same file
		}
		
		s.fileToUID[filename] = uid
		s.uidToFile[uid] = filename
	}
	
	// Regenerate daily index if needed
	s.regenerateIndexLocked()
	
	return nil
}

// DeleteEvent removes an event from storage
func (s *MemoryEventStorage) DeleteEvent(uid string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	// Remove file mapping if exists
	if filename, exists := s.uidToFile[uid]; exists {
		delete(s.fileToUID, filename)
		delete(s.uidToFile, uid)
	}
	
	// Remove from main storage
	delete(s.events, uid)
	
	// Regenerate daily index
	s.regenerateIndexLocked()
	
	return nil
}

// DeleteEventByFile removes an event from storage by filename
func (s *MemoryEventStorage) DeleteEventByFile(filename string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	// Find the UID for this filename
	uid, exists := s.fileToUID[filename]
	if !exists {
		// File not found, nothing to delete
		return nil
	}
	
	// Remove file mappings
	delete(s.fileToUID, filename)
	delete(s.uidToFile, uid)
	
	// Remove from main storage
	delete(s.events, uid)
	
	// Regenerate daily index
	s.regenerateIndexLocked()
	
	return nil
}

// GetEventsForDay returns all events that occur on a specific date
func (s *MemoryEventStorage) GetEventsForDay(date time.Time) []Event {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	dateKey := formatDateKey(date)
	
	// Check if we need to regenerate the index
	if !s.currentIndexDate.Equal(date.Truncate(24 * time.Hour)) {
		s.mutex.RUnlock()
		s.RegenerateIndex(date)
		s.mutex.RLock()
	}
	
	if events, exists := s.dailyIndex[dateKey]; exists {
		// Return a copy to prevent external modification
		result := make([]Event, len(events))
		copy(result, events)
		return result
	}
	
	return []Event{}
}

// GetEventsWithinRange returns all events that occur within the specified time range
func (s *MemoryEventStorage) GetEventsWithinRange(start, end time.Time) []Event {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	var eventsInRange []Event
	
	for _, event := range s.events {
		// Use OccurrencesWithin and extract unique event times
		occurrences := event.OccurrencesWithin(start, end)
		eventTimes := make(map[time.Time]bool)
		for _, occ := range occurrences {
			eventTimes[occ.EventTime] = true
		}
		
		// If no occurrences found (no alerts), fallback to checking event dates directly
		if len(eventTimes) == 0 {
			if calEvent, ok := event.(*CalendarEvent); ok {
				rawEventTimes := calEvent.getEventOccurrences(start, end)
				for _, eventTime := range rawEventTimes {
					eventTimes[eventTime] = true
				}
			}
		}
		
		if len(eventTimes) > 0 {
			eventsInRange = append(eventsInRange, event)
		}
	}
	
	return eventsInRange
}

// GetUpcomingEvents returns events that will occur within a specific duration from the given time
func (s *MemoryEventStorage) GetUpcomingEvents(from time.Time, duration time.Duration) []Event {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	var upcoming []Event
	until := from.Add(duration)
	
	for _, event := range s.events {
		// Check if event occurs within the time range using self-contained logic
		// We'll use a small time slice to find any event times in the range
		occurrences := event.OccurrencesWithin(from, until)
		eventTimes := make(map[time.Time]bool)
		for _, occ := range occurrences {
			eventTimes[occ.EventTime] = true
		}
		
		// If no occurrences found (no alerts), fallback to checking event dates directly
		if len(eventTimes) == 0 {
			// Use getEventOccurrences if it were public, or check if event occurs in date range
			if calEvent, ok := event.(*CalendarEvent); ok {
				rawEventTimes := calEvent.getEventOccurrences(from, until)
				for _, eventTime := range rawEventTimes {
					eventTimes[eventTime] = true
				}
			}
		}
		
		if len(eventTimes) > 0 {
			upcoming = append(upcoming, event)
		}
	}
	
	return upcoming
}

// RegenerateIndex rebuilds the daily index for efficient day-based queries
func (s *MemoryEventStorage) RegenerateIndex(date time.Time) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.currentIndexDate = date.Truncate(24 * time.Hour)
	return s.regenerateIndexLocked()
}

// regenerateIndexLocked rebuilds the daily index (must be called with lock held)
func (s *MemoryEventStorage) regenerateIndexLocked() error {
	// Clear existing index
	s.dailyIndex = make(map[string][]Event)
	
	// Generate index for current date and surrounding days (7-day window)
	baseDate := s.currentIndexDate
	if baseDate.IsZero() {
		baseDate = time.Now().Truncate(24 * time.Hour)
	}
	
	// Index 7 days: today and 6 days ahead
	for i := 0; i < 7; i++ {
		indexDate := baseDate.AddDate(0, 0, i)
		dateKey := formatDateKey(indexDate)
		
		var dayEvents []Event
		
		// Check each event to see if it occurs on this date
		for _, event := range s.events {
			if event.OccursOn(indexDate) {
				dayEvents = append(dayEvents, event)
			}
		}
		
		if len(dayEvents) > 0 {
			s.dailyIndex[dateKey] = dayEvents
		}
	}
	
	return nil
}

// GetAllEvents returns all events in storage
func (s *MemoryEventStorage) GetAllEvents() []Event {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	events := make([]Event, 0, len(s.events))
	for _, event := range s.events {
		events = append(events, event)
	}
	
	return events
}

// Clear removes all events from storage
func (s *MemoryEventStorage) Clear() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.events = make(map[string]Event)
	s.dailyIndex = make(map[string][]Event)
	s.fileToUID = make(map[string]string)
	s.uidToFile = make(map[string]string)
	s.calendars = make(map[string]*Calendar)
	s.currentIndexDate = time.Time{}
	
	return nil
}

// GetEventCount returns the total number of events in storage
func (s *MemoryEventStorage) GetEventCount() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return len(s.events)
}

// formatDateKey formats a date as YYYY-MM-DD for use as map key
func formatDateKey(date time.Time) string {
	return date.Format("2006-01-02")
}

// EnsureCalendar creates or returns existing Calendar for the given path
func (s *MemoryEventStorage) EnsureCalendar(path string, template string, automaticAlerts []Alert) *Calendar {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	if calendar, exists := s.calendars[path]; exists {
		return calendar
	}
	
	calendar := NewCalendar(path, template, automaticAlerts)
	s.calendars[path] = calendar
	return calendar
}

// GetCalendar returns the Calendar for the given path
func (s *MemoryEventStorage) GetCalendar(path string) (*Calendar, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	calendar, exists := s.calendars[path]
	return calendar, exists
}

// GetAllCalendars returns all Calendars
func (s *MemoryEventStorage) GetAllCalendars() map[string]*Calendar {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	result := make(map[string]*Calendar)
	for path, calendar := range s.calendars {
		result[path] = calendar
	}
	return result
}

// UpdateCalendarAlerts updates the automatic alerts for a Calendar
func (s *MemoryEventStorage) UpdateCalendarAlerts(path string, automaticAlerts []Alert) error {
	s.mutex.RLock()
	calendar, exists := s.calendars[path]
	s.mutex.RUnlock()
	
	if !exists {
		return nil // Calendar doesn't exist, nothing to update
	}
	
	calendar.UpdateAutomaticAlerts(automaticAlerts)
	return nil
}

// RemoveCalendar removes a Calendar from storage
func (s *MemoryEventStorage) RemoveCalendar(path string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	delete(s.calendars, path)
	return nil
}