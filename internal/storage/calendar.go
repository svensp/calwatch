package storage

import (
	"fmt"
	"sync"
	"time"
)

// Calendar represents a calendar entity that manages its events and alert policies
type Calendar struct {
	Path            string            // Directory path
	Template        string            // Notification template
	AutomaticAlerts []Alert           // Live, updateable alert policies
	events          map[string]Event  // Events belonging to this calendar
	mutex           sync.RWMutex      // Protects AutomaticAlerts and events
	
	// Cache for performance optimization (added later if needed)
	alertDayCache   map[string]bool   // "YYYY-MM-DD:UID" -> occurs on day
	cacheMutex      sync.RWMutex      // Protects cache
}

// NewCalendar creates a new Calendar entity
func NewCalendar(path, template string, automaticAlerts []Alert) *Calendar {
	return &Calendar{
		Path:            path,
		Template:        template,
		AutomaticAlerts: automaticAlerts,
		events:          make(map[string]Event),
		alertDayCache:   make(map[string]bool),
	}
}

// UpdateAutomaticAlerts updates the calendar's alert policies
// All events automatically see the updated alerts via pointer reference
func (c *Calendar) UpdateAutomaticAlerts(newAlerts []Alert) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.AutomaticAlerts = newAlerts
	
	// Invalidate cache since alert policies changed
	c.invalidateCache()
}

// UpdateTemplate updates the calendar's notification template
func (c *Calendar) UpdateTemplate(template string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.Template = template
}

// AddEvent adds an event to this calendar's collection
func (c *Calendar) AddEvent(event Event) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.events[event.GetUID()] = event
	
	// Invalidate cache since events changed
	c.invalidateCache()
}

// RemoveEvent removes an event from this calendar's collection
func (c *Calendar) RemoveEvent(uid string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	delete(c.events, uid)
	
	// Invalidate cache since events changed
	c.invalidateCache()
}

// GetEvent retrieves an event by UID
func (c *Calendar) GetEvent(uid string) (Event, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	event, exists := c.events[uid]
	return event, exists
}

// GetAllEvents returns all events in this calendar
func (c *Calendar) GetAllEvents() []Event {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	events := make([]Event, 0, len(c.events))
	for _, event := range c.events {
		events = append(events, event)
	}
	return events
}

// GetAutomaticAlerts returns a copy of the current automatic alerts
func (c *Calendar) GetAutomaticAlerts() []Alert {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	// Return a copy to prevent external modification
	alerts := make([]Alert, len(c.AutomaticAlerts))
	copy(alerts, c.AutomaticAlerts)
	return alerts
}

// GetTemplate returns the current notification template
func (c *Calendar) GetTemplate() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	return c.Template
}

// GetPath returns the calendar's directory path
func (c *Calendar) GetPath() string {
	return c.Path // Immutable, no lock needed
}

// GetEventsForDay returns all events from this calendar that occur on the given date
// This includes events whose alerts fire on the given date
func (c *Calendar) GetEventsForDay(date time.Time) []Event {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	var dayEvents []Event
	for _, event := range c.events {
		// Event determines its own daily index inclusion (considers all alerts)
		if event.OccursOn(date) {
			dayEvents = append(dayEvents, event)
		}
	}
	return dayEvents
}

// invalidateCache clears the alert day cache
// Must be called with c.mutex locked
func (c *Calendar) invalidateCache() {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	
	c.alertDayCache = make(map[string]bool)
}

// cacheKey generates a cache key for a date and event UID
func (c *Calendar) cacheKey(date time.Time, uid string) string {
	return fmt.Sprintf("%s:%s", date.Format("2006-01-02"), uid)
}

// String returns a string representation of the calendar
func (c *Calendar) String() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	return fmt.Sprintf("Calendar{Path: %s, Events: %d, Alerts: %d}", 
		c.Path, len(c.events), len(c.AutomaticAlerts))
}