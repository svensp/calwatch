package parser

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	// Read all data first so we can use it for both gocal and VALARM parsing
	icsData, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read ICS data: %w", err)
	}

	// Create gocal parser with minimal time bounds
	// We set a very narrow window to prevent RRULE expansion while still allowing parsing
	start := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2100, 12, 31, 23, 59, 59, 0, time.UTC)

	cal := gocal.NewParser(strings.NewReader(string(icsData)))
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

		event, err := p.convertGocalEvent(gocalEvent, string(icsData))
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
func (p *GocalParser) convertGocalEvent(gocalEvent gocal.Event, icsData string) (storage.Event, error) {
	// TODO: This needs to be updated to accept a Calendar parameter when Calendar integration is complete
	// For now, create a default calendar to make the code compile
	defaultCalendar := storage.NewCalendar("", "", []storage.Alert{})
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

	// Parse VALARM components for this event
	valarmAlerts, err := p.parseVALARMs(icsData, gocalEvent.Uid)
	if err != nil {
		// Log warning but don't fail the entire event
		fmt.Fprintf(os.Stderr, "Warning: Failed to parse VALARMs for event %s: %v\n", gocalEvent.Uid, err)
		valarmAlerts = []storage.Alert{} // Use empty slice on error
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
		defaultCalendar,
		valarmAlerts,
	)

	// Add exception dates if present
	for _, exDate := range gocalEvent.ExcludeDates {
		event.AddExceptionDate(exDate)
	}

	return event, nil
}

// parseVALARMs extracts VALARM components for a specific event from raw ICS data
func (p *GocalParser) parseVALARMs(icsData, eventUID string) ([]storage.Alert, error) {
	var alerts []storage.Alert

	lines := strings.Split(icsData, "\n")
	var currentEvent strings.Builder
	var inTargetEvent bool
	var inEvent bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if line == "BEGIN:VEVENT" {
			inEvent = true
			inTargetEvent = false
			currentEvent.Reset()
			currentEvent.WriteString(line + "\n")
		} else if line == "END:VEVENT" {
			if inEvent {
				currentEvent.WriteString(line + "\n")
				
				// If this was our target event, parse VALARMs
				if inTargetEvent {
					eventBlock := currentEvent.String()
					eventAlerts := p.extractVALARMsFromEventBlock(eventBlock)
					alerts = append(alerts, eventAlerts...)
				}
				
				inEvent = false
				inTargetEvent = false
			}
		} else if inEvent {
			currentEvent.WriteString(line + "\n")
			
			// Check if this is the UID line for our target event
			if strings.HasPrefix(line, "UID:") {
				uid := strings.TrimPrefix(line, "UID:")
				if uid == eventUID {
					inTargetEvent = true
				}
			}
		}
	}

	return alerts, nil
}

// extractVALARMsFromEventBlock extracts all VALARM blocks from a single event block
func (p *GocalParser) extractVALARMsFromEventBlock(eventBlock string) []storage.Alert {
	var alerts []storage.Alert
	
	lines := strings.Split(eventBlock, "\n")
	var currentVALARM strings.Builder
	var inVALARM bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if line == "BEGIN:VALARM" {
			inVALARM = true
			currentVALARM.Reset()
		} else if line == "END:VALARM" {
			if inVALARM {
				valarmBlock := currentVALARM.String()
				alert, err := p.parseVALARMBlock(valarmBlock)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to parse VALARM block: %v\n", err)
				} else {
					alerts = append(alerts, alert)
				}
				inVALARM = false
			}
		} else if inVALARM {
			currentVALARM.WriteString(line + "\n")
		}
	}

	return alerts
}

// parseVALARMBlock parses a single VALARM block and returns an Alert
func (p *GocalParser) parseVALARMBlock(valarmBlock string) (storage.Alert, error) {
	lines := strings.Split(valarmBlock, "\n")
	
	var trigger string
	var description string
	var action string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "TRIGGER:") {
			trigger = strings.TrimPrefix(line, "TRIGGER:")
		} else if strings.HasPrefix(line, "DESCRIPTION:") {
			description = strings.TrimPrefix(line, "DESCRIPTION:")
		} else if strings.HasPrefix(line, "ACTION:") {
			action = strings.TrimPrefix(line, "ACTION:")
		}
	}

	// Parse TRIGGER to get offset duration
	offset, err := p.parseTrigger(trigger)
	if err != nil {
		return storage.Alert{}, fmt.Errorf("failed to parse TRIGGER '%s': %w", trigger, err)
	}

	// Only support DISPLAY action for now
	if action != "DISPLAY" && action != "" {
		return storage.Alert{}, fmt.Errorf("unsupported VALARM action: %s", action)
	}

	// Use VALARM description or generate default
	if description == "" {
		description = fmt.Sprintf("Alert %v before", offset)
	}

	return storage.Alert{
		Offset:      offset,
		Important:   false, // VALARM doesn't specify importance, use default
		Source:      storage.AlertSourceVALARM,
		Description: description,
		Action:      storage.AlertActionDisplay,
	}, nil
}

// parseTrigger parses TRIGGER field to extract time.Duration
// Supports duration format like -PT15M, -PT1H, -P1DT2H30M
func (p *GocalParser) parseTrigger(trigger string) (time.Duration, error) {
	trigger = strings.TrimSpace(trigger)
	
	// Handle relative triggers (duration format)
	if strings.HasPrefix(trigger, "-P") {
		return p.parseDurationTrigger(trigger)
	}
	
	// TODO: Handle absolute time triggers if needed
	// if strings.Contains(trigger, "T") && (strings.Contains(trigger, "19") || strings.Contains(trigger, "20")) {
	//     return p.parseAbsoluteTrigger(trigger)
	// }
	
	return 0, fmt.Errorf("unsupported TRIGGER format: %s", trigger)
}

// parseDurationTrigger parses ISO 8601 duration format like -PT15M, -P1DT2H30M
func (p *GocalParser) parseDurationTrigger(trigger string) (time.Duration, error) {
	// Remove leading -P
	if !strings.HasPrefix(trigger, "-P") {
		return 0, fmt.Errorf("duration trigger must start with -P")
	}
	
	durationStr := strings.TrimPrefix(trigger, "-P")
	
	var days, hours, minutes, seconds int
	var err error
	
	// Check for time component (T)
	if strings.Contains(durationStr, "T") {
		parts := strings.Split(durationStr, "T")
		
		// Parse date part (days)
		if len(parts[0]) > 0 {
			if strings.HasSuffix(parts[0], "D") {
				dayStr := strings.TrimSuffix(parts[0], "D")
				days, err = strconv.Atoi(dayStr)
				if err != nil {
					return 0, fmt.Errorf("invalid days in duration: %s", dayStr)
				}
			}
		}
		
		// Parse time part
		if len(parts) > 1 {
			timePart := parts[1]
			
			// Parse hours
			if strings.Contains(timePart, "H") {
				hourPattern := regexp.MustCompile(`(\d+)H`)
				if match := hourPattern.FindStringSubmatch(timePart); len(match) > 1 {
					hours, err = strconv.Atoi(match[1])
					if err != nil {
						return 0, fmt.Errorf("invalid hours in duration: %s", match[1])
					}
				}
			}
			
			// Parse minutes
			if strings.Contains(timePart, "M") {
				minutePattern := regexp.MustCompile(`(\d+)M`)
				if match := minutePattern.FindStringSubmatch(timePart); len(match) > 1 {
					minutes, err = strconv.Atoi(match[1])
					if err != nil {
						return 0, fmt.Errorf("invalid minutes in duration: %s", match[1])
					}
				}
			}
			
			// Parse seconds
			if strings.Contains(timePart, "S") {
				secondPattern := regexp.MustCompile(`(\d+)S`)
				if match := secondPattern.FindStringSubmatch(timePart); len(match) > 1 {
					seconds, err = strconv.Atoi(match[1])
					if err != nil {
						return 0, fmt.Errorf("invalid seconds in duration: %s", match[1])
					}
				}
			}
		}
	} else {
		// Only days, no time component
		if strings.HasSuffix(durationStr, "D") {
			dayStr := strings.TrimSuffix(durationStr, "D")
			days, err = strconv.Atoi(dayStr)
			if err != nil {
				return 0, fmt.Errorf("invalid days in duration: %s", dayStr)
			}
		}
	}
	
	// Convert to time.Duration
	duration := time.Duration(days)*24*time.Hour +
		time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second
	
	return duration, nil
}

