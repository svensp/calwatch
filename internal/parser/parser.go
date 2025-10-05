package parser

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apognu/gocal"
	"calwatch/internal/storage"
	"calwatch/internal/recurrence"
)

// CalDAVParser handles parsing of ICS files
type CalDAVParser interface {
	ParseFile(filePath string) ([]storage.Event, error)
	ParseDirectory(dirPath string) ([]storage.Event, error)
	ParseReader(reader io.Reader) ([]storage.Event, error)
	ValidateICS(data []byte) error
}

// GocalParser implements CalDAVParser using the gocal library
type GocalParser struct {
	// Configuration options
	maxEvents int
	timeZone  *time.Location
}

// NewGocalParser creates a new parser instance
func NewGocalParser() *GocalParser {
	return &GocalParser{
		maxEvents: 10000, // Reasonable limit to prevent memory issues
		timeZone:  time.Local,
	}
}

// SetMaxEvents sets the maximum number of events to parse from a single file
func (p *GocalParser) SetMaxEvents(max int) {
	p.maxEvents = max
}

// SetTimeZone sets the default timezone for parsing
func (p *GocalParser) SetTimeZone(tz *time.Location) {
	p.timeZone = tz
}

// ParseFile parses a single ICS file and returns events
func (p *GocalParser) ParseFile(filePath string) ([]storage.Event, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	return p.ParseReader(file)
}

// ParseDirectory parses all ICS files in a directory
func (p *GocalParser) ParseDirectory(dirPath string) ([]storage.Event, error) {
	var allEvents []storage.Event

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Log error but continue processing other files
			return nil
		}

		// Skip directories and non-ICS files
		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(strings.ToLower(info.Name()), ".ics") {
			return nil
		}

		events, parseErr := p.ParseFile(path)
		if parseErr != nil {
			// Log error but continue processing other files
			fmt.Fprintf(os.Stderr, "Error parsing file %s: %v\n", path, parseErr)
			return nil
		}

		allEvents = append(allEvents, events...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", dirPath, err)
	}

	return allEvents, nil
}

// ParseReader parses ICS data from an io.Reader
func (p *GocalParser) ParseReader(reader io.Reader) ([]storage.Event, error) {
	// Create gocal parser with minimal time bounds
	// We set a very narrow window to prevent RRULE expansion while still allowing parsing
	start := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2100, 12, 31, 23, 59, 59, 0, time.UTC)

	cal := gocal.NewParser(reader)
	cal.Start, cal.End = &start, &end

	// Parse the calendar
	if err := cal.Parse(); err != nil {
		return nil, fmt.Errorf("failed to parse ICS data: %w", err)
	}

	// Convert gocal events to our Event interface
	var events []storage.Event
	eventCount := 0

	for _, gocalEvent := range cal.Events {
		// Prevent memory issues with too many events
		if eventCount >= p.maxEvents {
			fmt.Fprintf(os.Stderr, "Warning: Reached maximum event limit (%d), skipping remaining events\n", p.maxEvents)
			break
		}

		event, err := p.convertGocalEvent(gocalEvent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error converting event %s: %v\n", gocalEvent.Uid, err)
			continue
		}

		events = append(events, event)
		eventCount++
	}

	return events, nil
}

// ValidateICS validates ICS data without fully parsing it
func (p *GocalParser) ValidateICS(data []byte) error {
	content := string(data)

	// Basic validation checks
	if !strings.Contains(content, "BEGIN:VCALENDAR") {
		return fmt.Errorf("missing BEGIN:VCALENDAR")
	}

	if !strings.Contains(content, "END:VCALENDAR") {
		return fmt.Errorf("missing END:VCALENDAR")
	}

	// More sophisticated validation - check for matching BEGIN/END pairs
	lines := strings.Split(content, "\n")
	stack := make([]string, 0)
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "BEGIN:") {
			component := strings.TrimPrefix(line, "BEGIN:")
			stack = append(stack, component)
		} else if strings.HasPrefix(line, "END:") {
			component := strings.TrimPrefix(line, "END:")
			if len(stack) == 0 {
				return fmt.Errorf("unexpected END:%s without matching BEGIN", component)
			}
			if stack[len(stack)-1] != component {
				return fmt.Errorf("mismatched BEGIN/END: expected %s, got %s", stack[len(stack)-1], component)
			}
			stack = stack[:len(stack)-1]
		}
	}
	
	if len(stack) > 0 {
		return fmt.Errorf("unclosed BEGIN statements: %v", stack)
	}

	return nil
}

// convertGocalEvent converts a gocal.Event to our storage.Event interface
func (p *GocalParser) convertGocalEvent(gocalEvent gocal.Event) (storage.Event, error) {
	// Extract basic event information
	uid := gocalEvent.Uid
	if uid == "" {
		return nil, fmt.Errorf("event missing UID")
	}

	summary := gocalEvent.Summary
	description := gocalEvent.Description
	location := gocalEvent.Location

	// Handle start and end times
	startTime := *gocalEvent.Start
	endTime := *gocalEvent.End

	// Determine timezone
	timezone := startTime.Location()
	if timezone == nil {
		timezone = p.timeZone
	}

	// Parse recurrence rule if present
	var rec recurrence.Recurrence
	var err error
	if len(gocalEvent.RecurrenceRule) > 0 {
		// Convert map to RRULE string format
		var parts []string
		for key, value := range gocalEvent.RecurrenceRule {
			parts = append(parts, fmt.Sprintf("%s=%s", key, value))
		}
		rruleStr := strings.Join(parts, ";")
		
		// Parse RRULE string into Recurrence instance
		rec, err = recurrence.ParseRRule(rruleStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse RRULE '%s': %w", rruleStr, err)
		}
	} else {
		// No recurrence rule
		rec = &recurrence.NoRecurrence{}
	}

	// Create enhanced calendar event with recurrence support
	event := storage.NewCalendarEvent(
		uid,
		summary,
		description,
		location,
		startTime,
		endTime,
		timezone,
		rec,
	)

	// Add exception dates if present
	for _, exDate := range gocalEvent.ExcludeDates {
		event.AddExceptionDate(exDate)
	}

	return event, nil
}

