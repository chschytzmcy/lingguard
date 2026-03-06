// Package caldav provides CalDAV client functionality for calendar management.
package caldav

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Event represents a calendar event
type Event struct {
	UID         string     `json:"uid"`                   // Unique identifier
	Summary     string     `json:"summary"`               // Event title
	Description string     `json:"description,omitempty"` // Event description
	Location    string     `json:"location,omitempty"`    // Event location
	Start       time.Time  `json:"start"`                 // Start time
	End         time.Time  `json:"end"`                   // End time
	AllDay      bool       `json:"allDay"`                // All-day event flag
	Status      string     `json:"status,omitempty"`      // TENTATIVE, CONFIRMED, CANCELLED
	Categories  []string   `json:"categories,omitempty"`  // Event categories
	Attendees   []Attendee `json:"attendees,omitempty"`   // Event attendees
	Organizer   *Attendee  `json:"organizer,omitempty"`   // Event organizer
	URL         string     `json:"url,omitempty"`         // Event URL
	Href        string     `json:"href,omitempty"`        // CalDAV resource href
	ETag        string     `json:"etag,omitempty"`        // Entity tag for updates
}

// Attendee represents an event attendee
type Attendee struct {
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

// Calendar represents a CalDAV calendar
type Calendar struct {
	Href        string `json:"href"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
}

// ParseICS parses an iCalendar (ICS) content string into an Event
func ParseICS(content string) (*Event, error) {
	event := &Event{
		Status: "CONFIRMED",
	}

	// Normalize line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	// Extract lines
	lines := strings.Split(content, "\n")

	var currentKey string
	var currentValue strings.Builder

	for _, line := range lines {
		// Handle line folding (lines starting with space or tab are continuations)
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			currentValue.WriteString(line[1:])
			continue
		}

		// Save previous key-value pair
		if currentKey != "" {
			parseICSProperty(event, currentKey, currentValue.String())
		}

		// Parse new line
		currentKey = ""
		currentValue.Reset()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Skip VCALENDAR/VEVENT boundaries
		upperLine := strings.ToUpper(line)
		if upperLine == "BEGIN:VCALENDAR" || upperLine == "END:VCALENDAR" ||
			upperLine == "BEGIN:VEVENT" || upperLine == "END:VEVENT" {
			continue
		}

		// Split key:value
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}

		// Handle parameters (KEY;PARAM=VALUE:content)
		keyPart := line[:colonIdx]
		value := line[colonIdx+1:]
		currentKey = keyPart
		currentValue.WriteString(value)
	}

	// Save last key-value pair
	if currentKey != "" {
		parseICSProperty(event, currentKey, currentValue.String())
	}

	// Validate required fields
	if event.UID == "" {
		event.UID = generateUID()
	}

	return event, nil
}

// parseICSProperty parses a single ICS property
func parseICSProperty(event *Event, key, value string) {
	// Extract property name (before any semicolon)
	propName := strings.ToUpper(key)
	if semicolonIdx := strings.Index(key, ";"); semicolonIdx != -1 {
		propName = strings.ToUpper(key[:semicolonIdx])
	}

	switch propName {
	case "UID":
		event.UID = value
	case "SUMMARY":
		event.Summary = unescapeICS(value)
	case "DESCRIPTION":
		event.Description = unescapeICS(value)
	case "LOCATION":
		event.Location = unescapeICS(value)
	case "DTSTART":
		event.Start = parseICSDateTime(key, value)
		event.AllDay = !strings.Contains(key, "TZID") && len(strings.ReplaceAll(value, "-", "")) == 8
	case "DTEND":
		event.End = parseICSDateTime(key, value)
	case "STATUS":
		event.Status = strings.ToUpper(value)
	case "CATEGORIES":
		event.Categories = strings.Split(value, ",")
		for i, cat := range event.Categories {
			event.Categories[i] = strings.TrimSpace(cat)
		}
	case "ORGANIZER":
		event.Organizer = parseICSAttendee(key, value)
	case "ATTENDEE":
		attendee := parseICSAttendee(key, value)
		if attendee != nil {
			event.Attendees = append(event.Attendees, *attendee)
		}
	case "URL":
		event.URL = value
	}
}

// parseICSDateTime parses an ICS date/time value
func parseICSDateTime(key, value string) time.Time {
	// Remove VALUE=DATE for all-day events
	isDateOnly := strings.Contains(key, "VALUE=DATE") || len(strings.ReplaceAll(value, "-", "")) == 8

	// Extract timezone if present
	tzid := ""
	if tzMatch := regexp.MustCompile(`TZID=([^;:]+)`).FindStringSubmatch(key); len(tzMatch) > 1 {
		tzid = tzMatch[1]
	}

	// Check for UTC indicator
	isUTC := strings.HasSuffix(value, "Z")

	// Remove 'Z' suffix for UTC
	value = strings.TrimSuffix(value, "Z")

	// Get location for parsing
	loc := time.UTC
	if tzid != "" && !isUTC {
		if l, err := time.LoadLocation(tzid); err == nil {
			loc = l
		}
	}

	if isDateOnly {
		// Date only format: YYYYMMDD
		t, err := time.ParseInLocation("20060102", value, loc)
		if err == nil {
			return t
		}
	} else {
		// DateTime format: YYYYMMDDTHHMMSS
		value = strings.ReplaceAll(value, "-", "")
		value = strings.ReplaceAll(value, ":", "")

		// Try parsing with different layouts
		layouts := []string{
			"20060102T150405",
			"20060102T1504",
			"20060102",
		}

		for _, layout := range layouts {
			t, err := time.ParseInLocation(layout, value, loc)
			if err == nil {
				return t
			}
		}
	}

	return time.Time{}
}

// parseICSAttendee parses an attendee or organizer line
func parseICSAttendee(key, value string) *Attendee {
	attendee := &Attendee{}

	// Extract email from mailto: prefix
	email := value
	if strings.HasPrefix(strings.ToLower(email), "mailto:") {
		email = email[7:]
	}
	attendee.Email = email

	// Extract name from CN parameter
	if cnMatch := regexp.MustCompile(`CN=([^;:]+)`).FindStringSubmatch(key); len(cnMatch) > 1 {
		attendee.Name = unescapeICS(cnMatch[1])
	}

	return attendee
}

// unescapeICS unescapes ICS special characters
func unescapeICS(s string) string {
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\N", "\n")
	s = strings.ReplaceAll(s, "\\,", ",")
	s = strings.ReplaceAll(s, "\\;", ";")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return s
}

// escapeICS escapes ICS special characters
func escapeICS(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// GenerateICS generates an ICS content string from an Event
func GenerateICS(event *Event) string {
	var sb strings.Builder

	sb.WriteString("BEGIN:VCALENDAR\r\n")
	sb.WriteString("VERSION:2.0\r\n")
	sb.WriteString("PRODID:-//LingGuard//CalDAV Client//EN\r\n")
	sb.WriteString("BEGIN:VEVENT\r\n")

	// UID
	sb.WriteString(fmt.Sprintf("UID:%s\r\n", event.UID))

	// Summary (title)
	if event.Summary != "" {
		sb.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", escapeICS(event.Summary)))
	}

	// Description
	if event.Description != "" {
		sb.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", escapeICS(event.Description)))
	}

	// Location
	if event.Location != "" {
		sb.WriteString(fmt.Sprintf("LOCATION:%s\r\n", escapeICS(event.Location)))
	}

	// Date/Time
	if event.AllDay {
		sb.WriteString(fmt.Sprintf("DTSTART;VALUE=DATE:%s\r\n", event.Start.Format("20060102")))
		if !event.End.IsZero() {
			sb.WriteString(fmt.Sprintf("DTEND;VALUE=DATE:%s\r\n", event.End.Format("20060102")))
		}
	} else {
		sb.WriteString(fmt.Sprintf("DTSTART:%s\r\n", event.Start.Format("20060102T150405")))
		if !event.End.IsZero() {
			sb.WriteString(fmt.Sprintf("DTEND:%s\r\n", event.End.Format("20060102T150405")))
		}
	}

	// Status
	if event.Status != "" {
		sb.WriteString(fmt.Sprintf("STATUS:%s\r\n", event.Status))
	}

	// Categories
	if len(event.Categories) > 0 {
		sb.WriteString(fmt.Sprintf("CATEGORIES:%s\r\n", strings.Join(event.Categories, ",")))
	}

	// Organizer
	if event.Organizer != nil {
		org := event.Organizer
		sb.WriteString(fmt.Sprintf("ORGANIZER;CN=%s:mailto:%s\r\n", escapeICS(org.Name), org.Email))
	}

	// Attendees
	for _, att := range event.Attendees {
		sb.WriteString(fmt.Sprintf("ATTENDEE;CN=%s:mailto:%s\r\n", escapeICS(att.Name), att.Email))
	}

	// URL
	if event.URL != "" {
		sb.WriteString(fmt.Sprintf("URL:%s\r\n", event.URL))
	}

	// Timestamp
	sb.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", time.Now().UTC().Format("20060102T150405Z")))

	sb.WriteString("END:VEVENT\r\n")
	sb.WriteString("END:VCALENDAR\r\n")

	return sb.String()
}

// generateUID generates a unique identifier for events
func generateUID() string {
	return fmt.Sprintf("lingguard-%d@lingguard", time.Now().UnixNano())
}

// ParseICSDateTime parses a date/time string in various formats
func ParseICSDateTime(s string) (time.Time, error) {
	// Common date/time formats
	formats := []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"20060102T150405",
		"20060102T150405Z",
		"20060102",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date/time: %s", s)
}

// FormatEventForDisplay formats an event for human-readable display
func FormatEventForDisplay(event *Event) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("📌 %s\n", event.Summary))

	if !event.Start.IsZero() {
		if event.AllDay {
			sb.WriteString(fmt.Sprintf("📅 %s (全天)\n", event.Start.Format("2006-01-02")))
		} else {
			timeStr := event.Start.Format("2006-01-02 15:04")
			if !event.End.IsZero() {
				if event.Start.Format("2006-01-02") == event.End.Format("2006-01-02") {
					timeStr += " - " + event.End.Format("15:04")
				} else {
					timeStr += " ~ " + event.End.Format("2006-01-02 15:04")
				}
			}
			sb.WriteString(fmt.Sprintf("📅 %s\n", timeStr))
		}
	}

	if event.Location != "" {
		sb.WriteString(fmt.Sprintf("📍 %s\n", event.Location))
	}

	if event.Description != "" {
		desc := event.Description
		if len(desc) > 100 {
			desc = desc[:100] + "..."
		}
		sb.WriteString(fmt.Sprintf("📝 %s\n", desc))
	}

	if event.Status == "CANCELLED" {
		sb.WriteString("❌ 已取消\n")
	}

	return sb.String()
}

// ParseRelativeTime parses relative time expressions like "now", "+1h", "-30m"
func ParseRelativeTime(s string, base time.Time) (time.Time, error) {
	s = strings.TrimSpace(strings.ToLower(s))

	if s == "now" || s == "" {
		return base, nil
	}

	// Parse relative time like "+1h", "-30m", "+1d"
	re := regexp.MustCompile(`^([+-]?)(\d+)([smhdwy])$`)
	matches := re.FindStringSubmatch(s)
	if len(matches) != 4 {
		return time.Time{}, fmt.Errorf("invalid relative time format: %s", s)
	}

	sign := 1
	if matches[1] == "-" {
		sign = -1
	}

	amount, err := strconv.Atoi(matches[2])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid number in relative time: %s", matches[2])
	}
	amount *= sign

	unit := matches[3]

	switch unit {
	case "s":
		return base.Add(time.Duration(amount) * time.Second), nil
	case "m":
		return base.Add(time.Duration(amount) * time.Minute), nil
	case "h":
		return base.Add(time.Duration(amount) * time.Hour), nil
	case "d":
		return base.AddDate(0, 0, amount), nil
	case "w":
		return base.AddDate(0, 0, amount*7), nil
	case "y":
		return base.AddDate(amount, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("unknown time unit: %s", unit)
	}
}
