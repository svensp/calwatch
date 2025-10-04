package alerts

import (
	"strings"
	"time"
	
	"calwatch/internal/storage"
)

// EventPriority represents the priority level of an event
type EventPriority int

const (
	PriorityLow EventPriority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// String returns a string representation of the priority
func (p EventPriority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// PriorityClassifier determines event priorities based on various factors
type PriorityClassifier struct {
	highPriorityKeywords     []string
	criticalPriorityKeywords []string
	workCalendarPaths        []string // Paths that indicate work calendars
}

// NewPriorityClassifier creates a new priority classifier with default rules
func NewPriorityClassifier() *PriorityClassifier {
	return &PriorityClassifier{
		highPriorityKeywords: []string{
			"meeting", "interview", "appointment", "deadline", "due", 
			"presentation", "conference", "call", "sync", "standup",
			"1:1", "one-on-one", "review", "demo", "launch",
		},
		criticalPriorityKeywords: []string{
			"urgent", "asap", "emergency", "critical", "important",
			"board meeting", "client meeting", "customer meeting",
			"deadline today", "overdue", "final", "last chance",
		},
		workCalendarPaths: []string{
			"work", "office", "company", "corp", "business",
		},
	}
}

// ClassifyEvent determines the priority of an event based on various factors
func (pc *PriorityClassifier) ClassifyEvent(event storage.Event) EventPriority {
	priority := PriorityNormal // Default priority
	
	// Check for critical keywords first (highest priority)
	if pc.containsCriticalKeywords(event) {
		priority = PriorityCritical
	} else if pc.containsHighPriorityKeywords(event) {
		priority = PriorityHigh
	}
	
	// Boost priority for work-related events
	if pc.isWorkEvent(event) && priority < PriorityHigh {
		priority = PriorityHigh
	}
	
	// Boost priority for events with attendees (meetings)
	if pc.hasAttendees(event) && priority < PriorityHigh {
		priority = PriorityHigh
	}
	
	// Boost priority for time-sensitive events (starting soon)
	if pc.isTimeSensitive(event) && priority < PriorityHigh {
		priority = PriorityHigh
	}
	
	// Lower priority for all-day events (usually less urgent)
	if pc.isAllDayEvent(event) && priority > PriorityLow {
		priority = PriorityLow
	}
	
	return priority
}

// containsCriticalKeywords checks if the event contains critical priority keywords
func (pc *PriorityClassifier) containsCriticalKeywords(event storage.Event) bool {
	text := pc.getSearchableText(event)
	
	for _, keyword := range pc.criticalPriorityKeywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	
	return false
}

// containsHighPriorityKeywords checks if the event contains high priority keywords
func (pc *PriorityClassifier) containsHighPriorityKeywords(event storage.Event) bool {
	text := pc.getSearchableText(event)
	
	for _, keyword := range pc.highPriorityKeywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	
	return false
}

// getSearchableText combines summary and description for keyword searching
func (pc *PriorityClassifier) getSearchableText(event storage.Event) string {
	text := strings.ToLower(event.GetSummary() + " " + event.GetDescription())
	return text
}

// isWorkEvent determines if this is a work-related event based on various factors
func (pc *PriorityClassifier) isWorkEvent(event storage.Event) bool {
	// TODO: In a full implementation, this would check:
	// 1. Calendar source path (e.g., ~/.calendars/work/)
	// 2. Domain of organizer/attendees (@company.com)
	// 3. Work-related keywords
	
	text := pc.getSearchableText(event)
	workKeywords := []string{
		"work", "office", "team", "project", "client", "customer",
		"business", "corporate", "professional", "company",
	}
	
	for _, keyword := range workKeywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	
	return false
}

// hasAttendees checks if the event has attendees (indicating it's a meeting)
func (pc *PriorityClassifier) hasAttendees(event storage.Event) bool {
	// TODO: In a full implementation, this would check for:
	// 1. ATTENDEE properties in the ICS file
	// 2. Keywords like "with", "and", multiple names
	// 3. Meeting-specific patterns
	
	text := pc.getSearchableText(event)
	meetingKeywords := []string{
		"with", "meeting", "call", "sync", "standup", "1:1",
		"conference", "zoom", "teams", "meet", "hangout",
	}
	
	for _, keyword := range meetingKeywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	
	return false
}

// isTimeSensitive checks if the event is starting soon
func (pc *PriorityClassifier) isTimeSensitive(event storage.Event) bool {
	now := time.Now()
	startTime := event.GetStartTime()
	
	// Consider events starting within the next 2 hours as time-sensitive
	timeSensitiveThreshold := 2 * time.Hour
	
	return startTime.Sub(now) <= timeSensitiveThreshold && startTime.After(now)
}

// isAllDayEvent checks if this is an all-day event
func (pc *PriorityClassifier) isAllDayEvent(event storage.Event) bool {
	startTime := event.GetStartTime()
	endTime := event.GetEndTime()
	
	// All-day events typically:
	// 1. Start at midnight (00:00)
	// 2. Have duration of 24 hours or more
	duration := endTime.Sub(startTime)
	
	return startTime.Hour() == 0 && startTime.Minute() == 0 && duration >= 24*time.Hour
}

// FilterByPriority filters alert requests to only include high and critical priority events
func (pc *PriorityClassifier) FilterByPriority(alerts []AlertRequest, minPriority EventPriority) []AlertRequest {
	var filtered []AlertRequest
	
	for _, alert := range alerts {
		eventPriority := pc.ClassifyEvent(alert.Event)
		if eventPriority >= minPriority {
			filtered = append(filtered, alert)
		}
	}
	
	return filtered
}

// AddHighPriorityKeyword adds a custom high priority keyword
func (pc *PriorityClassifier) AddHighPriorityKeyword(keyword string) {
	pc.highPriorityKeywords = append(pc.highPriorityKeywords, strings.ToLower(keyword))
}

// AddCriticalPriorityKeyword adds a custom critical priority keyword  
func (pc *PriorityClassifier) AddCriticalPriorityKeyword(keyword string) {
	pc.criticalPriorityKeywords = append(pc.criticalPriorityKeywords, strings.ToLower(keyword))
}

// AddWorkCalendarPath adds a path pattern that indicates work calendars
func (pc *PriorityClassifier) AddWorkCalendarPath(path string) {
	pc.workCalendarPaths = append(pc.workCalendarPaths, strings.ToLower(path))
}