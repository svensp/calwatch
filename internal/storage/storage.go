package storage

import (
	"sync"
	"time"
)

// EventStorage manages in-memory event storage with efficient indexing
type EventStorage interface {
	UpsertEvent(event Event) error
	DeleteEvent(uid string) error
	GetEventsForDay(date time.Time) []Event
	GetEventsWithinRange(start, end time.Time) []Event
	GetUpcomingEvents(from time.Time, duration time.Duration) []Event
	RegenerateIndex(date time.Time) error
	GetAllEvents() []Event
	Clear() error
}

// MemoryEventStorage implements EventStorage using in-memory maps
type MemoryEventStorage struct {
	// Main event storage by UID
	events map[string]Event
	
	// Daily index for fast lookups - map[YYYY-MM-DD][]Event
	dailyIndex map[string][]Event
	
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
		mutex:      sync.RWMutex{},
	}
}

// UpsertEvent adds or updates an event in storage
func (s *MemoryEventStorage) UpsertEvent(event Event) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	// Store event by UID
	s.events[event.GetUID()] = event
	
	// Regenerate daily index if needed
	s.regenerateIndexLocked()
	
	return nil
}

// DeleteEvent removes an event from storage
func (s *MemoryEventStorage) DeleteEvent(uid string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
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
		occurrences := event.OccurredWithin(start, end)
		if len(occurrences) > 0 {
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
		nextOccurrence := event.NextOccurrence(from)
		if nextOccurrence != nil && nextOccurrence.Before(until) {
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