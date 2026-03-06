// Package caldav provides CalDAV client functionality for calendar management.
package caldav

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lingguard/pkg/httpclient"
	"github.com/lingguard/pkg/logger"
)

// PresetURLs maps preset names to URL templates
var PresetURLs = map[string]string{
	"feishu":   "https://caldav.feishu.cn/",
	"dingtalk": "https://calendar.dingtalk.com/dav/",
	"apple":    "https://caldav.icloud.com",
	"google":   "https://apidata.googleusercontent.com/caldav/v2/{{username}}/events",
}

// Client is a CalDAV client
type Client struct {
	httpClient *http.Client
	account    *AccountConfig
}

// AccountConfig holds CalDAV account configuration
type AccountConfig struct {
	Name     string
	URL      string
	Username string
	Password string
	Token    string
	Timeout  time.Duration
}

// NewClient creates a new CalDAV client
func NewClient(account *AccountConfig) *Client {
	timeout := account.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// Use httpclient with custom timeout (respects proxy settings)
	client := httpclient.WithTimeout(timeout)

	return &Client{
		httpClient: client,
		account:    account,
	}
}

// SetToken sets the bearer token for authentication
func (c *Client) SetToken(token string) {
	c.account.Token = token
}

// NewClientFromConfig creates a CalDAV client from config.CalendarAccount
func NewClientFromConfig(name, username, password, preset, customURL string, timeoutSec int) (*Client, error) {
	var calURL string

	// Determine URL based on preset or custom URL
	if customURL != "" {
		calURL = customURL
		// If URL doesn't contain username path, append it
		if !strings.Contains(calURL, "/"+username+"/") && !strings.HasSuffix(calURL, "/"+username) {
			calURL = strings.TrimSuffix(calURL, "/") + "/" + username + "/"
		}
	} else if preset != "" {
		template, ok := PresetURLs[preset]
		if !ok {
			return nil, fmt.Errorf("unknown preset: %s", preset)
		}
		// Replace {{username}} placeholder
		calURL = strings.ReplaceAll(template, "{{username}}", username)
	} else {
		return nil, fmt.Errorf("either url or preset must be specified")
	}

	timeout := time.Duration(timeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	account := &AccountConfig{
		Name:     name,
		URL:      calURL,
		Username: username,
		Password: password,
		Timeout:  timeout,
	}

	return NewClient(account), nil
}

// doRequest performs an HTTP request with authentication
func (c *Client) doRequest(ctx context.Context, method, href string, body []byte, headers map[string]string) (*http.Response, error) {
	// Build full URL
	fullURL := href
	if !strings.HasPrefix(href, "http") {
		// Parse the base URL to get the scheme and host
		baseURL, err := url.Parse(c.account.URL)
		if err != nil {
			return nil, fmt.Errorf("parse base URL: %w", err)
		}
		// Build full URL: scheme://host + href
		fullURL = baseURL.Scheme + "://" + baseURL.Host + href
	}

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set authentication
	if c.account.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.account.Token)
	} else if c.account.Username != "" && c.account.Password != "" {
		req.SetBasicAuth(c.account.Username, c.account.Password)
	}

	// Set additional headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Default content type
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// ListCalendars lists all calendars for the account
func (c *Client) ListCalendars(ctx context.Context) ([]Calendar, error) {
	// PROPFIND to discover calendars
	propfind := `<?xml version="1.0" encoding="utf-8"?>
<propfind xmlns="DAV:">
  <prop>
    <displayname/>
    <resourcetype/>
    <getcontenttype/>
    <calendar-color xmlns="http://apple.com/ns/ical/"/>
    <calendar-description xmlns="urn:ietf:params:xml:ns:caldav"/>
  </prop>
</propfind>`

	headers := map[string]string{
		"Depth": "1",
	}

	resp, err := c.doRequest(ctx, "PROPFIND", c.account.URL, []byte(propfind), headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMultiStatus {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PROPFIND failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return parseCalendarMultiStatus(body)
}

// caldavMultiStatus represents a DAV multistatus response
type caldavMultiStatus struct {
	XMLName   xml.Name         `xml:"multistatus"`
	Responses []caldavResponse `xml:"response"`
}

type caldavResponse struct {
	Href     string           `xml:"href"`
	PropStat []caldavPropStat `xml:"propstat"`
}

type caldavPropStat struct {
	Prop   caldavProp `xml:"prop"`
	Status string     `xml:"status"`
}

type caldavProp struct {
	DisplayName   string       `xml:"displayname"`
	ResourceType  resourceType `xml:"resourcetype"`
	ContentType   string       `xml:"getcontenttype"`
	CalendarColor string       `xml:"calendar-color"`
	Description   string       `xml:"calendar-description"`
}

type resourceType struct {
	InnerXML string `xml:",innerxml"`
}

// parseCalendarMultiStatus parses the PROPFIND response
func parseCalendarMultiStatus(data []byte) ([]Calendar, error) {
	var ms caldavMultiStatus
	if err := xml.Unmarshal(data, &ms); err != nil {
		return nil, fmt.Errorf("parse multistatus: %w", err)
	}

	var calendars []Calendar
	for _, resp := range ms.Responses {
		for _, ps := range resp.PropStat {
			// Check for 200 OK status and calendar resource type
			if strings.Contains(ps.Status, "200") && isCalendarResource(ps.Prop.ResourceType.InnerXML) {
				calendars = append(calendars, Calendar{
					Href:        resp.Href,
					Name:        ps.Prop.DisplayName,
					Description: ps.Prop.Description,
					Color:       ps.Prop.CalendarColor,
				})
			}
		}
	}

	return calendars, nil
}

// isCalendarResource checks if the resource type XML contains a calendar element
func isCalendarResource(innerXML string) bool {
	lower := strings.ToLower(innerXML)
	// Check for calendar element with various namespace prefixes (C:, c:, caldav:, etc.)
	// Also check for the CalDAV namespace URL
	return strings.Contains(lower, ":calendar") ||
		strings.Contains(lower, "<calendar") ||
		strings.Contains(lower, "urn:ietf:params:xml:ns:caldav")
}

// QueryEvents queries events in a calendar within a time range
// Supports different CalDAV server implementations:
// - DingTalk: calendar-query returns calendar-data directly
// - Feishu: requires two-step (calendar-query + calendar-multiget)
func (c *Client) QueryEvents(ctx context.Context, calendarHref string, start, end time.Time) ([]Event, error) {
	startStr := start.UTC().Format("20060102T150405Z")
	endStr := end.UTC().Format("20060102T150405Z")

	// Try standard calendar-query first (works for DingTalk, Apple, etc.)
	report := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop>
    <D:getetag/>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <C:time-range start="%s" end="%s"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`, startStr, endStr)

	headers := map[string]string{
		"Depth": "1",
	}

	resp, err := c.doRequest(ctx, "REPORT", calendarHref, []byte(report), headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMultiStatus {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("REPORT failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Try to parse events directly (DingTalk style)
	events, err := parseEventMultiStatus(body)
	if err == nil && len(events) > 0 {
		return events, nil
	}

	// Fallback: two-step approach for Feishu
	// First get event hrefs, then use calendar-multiget
	hrefs, err := parseEventHrefs(body)
	if err != nil {
		return nil, fmt.Errorf("parse event hrefs: %w", err)
	}

	if len(hrefs) == 0 {
		return []Event{}, nil
	}

	return c.fetchEventsByHrefs(ctx, calendarHref, hrefs)
}

// parseEventHrefs extracts event hrefs from a calendar-query response
func parseEventHrefs(data []byte) ([]string, error) {
	var ms caldavMultiStatus
	if err := xml.Unmarshal(data, &ms); err != nil {
		return nil, fmt.Errorf("parse multistatus: %w", err)
	}

	var hrefs []string
	for _, resp := range ms.Responses {
		// Check if this is an event (has 200 OK status)
		for _, ps := range resp.PropStat {
			if strings.Contains(ps.Status, "200") {
				hrefs = append(hrefs, resp.Href)
				break
			}
		}
	}
	return hrefs, nil
}

// fetchEventsByHrefs uses calendar-multiget to fetch event data
func (c *Client) fetchEventsByHrefs(ctx context.Context, calendarHref string, hrefs []string) ([]Event, error) {
	if len(hrefs) == 0 {
		return []Event{}, nil
	}

	// Build href elements
	var hrefElements strings.Builder
	for _, href := range hrefs {
		hrefElements.WriteString(fmt.Sprintf("<DAV:href>%s</DAV:href>\n", href))
	}

	// Build calendar-multiget REPORT
	report := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<calendar-multiget xmlns="urn:ietf:params:xml:ns:caldav" xmlns:DAV="DAV:">
  <DAV:prop>
    <DAV:getetag/>
    <calendar-data xmlns="urn:ietf:params:xml:ns:caldav"/>
  </DAV:prop>
  %s
</calendar-multiget>`, hrefElements.String())

	headers := map[string]string{
		"Depth": "1",
	}

	resp, err := c.doRequest(ctx, "REPORT", calendarHref, []byte(report), headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMultiStatus {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("calendar-multiget failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return parseEventMultiStatus(body)
}

// GetEvent retrieves a single event by its href
func (c *Client) GetEvent(ctx context.Context, eventHref string) (*Event, error) {
	resp, err := c.doRequest(ctx, "GET", eventHref, nil, map[string]string{
		"Accept": "text/calendar, application/ics",
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET event failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	event, err := ParseICS(string(body))
	if err != nil {
		return nil, fmt.Errorf("parse ICS: %w", err)
	}

	event.Href = eventHref
	event.ETag = resp.Header.Get("ETag")

	return event, nil
}

// CreateEvent creates a new event in the specified calendar
func (c *Client) CreateEvent(ctx context.Context, calendarHref string, event *Event) (*Event, error) {
	// Generate UID if not set
	if event.UID == "" {
		event.UID = generateUID()
	}

	// Build event href
	eventHref := calendarHref
	if !strings.HasSuffix(eventHref, "/") {
		eventHref += "/"
	}
	eventHref += event.UID + ".ics"

	// Generate ICS content
	icsContent := GenerateICS(event)

	resp, err := c.doRequest(ctx, "PUT", eventHref, []byte(icsContent), map[string]string{
		"Content-Type": "text/calendar; charset=utf-8",
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PUT event failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	event.Href = eventHref
	event.ETag = resp.Header.Get("ETag")

	logger.Info("CalDAV event created", "uid", event.UID, "href", eventHref)

	return event, nil
}

// UpdateEvent updates an existing event
func (c *Client) UpdateEvent(ctx context.Context, event *Event) (*Event, error) {
	if event.Href == "" {
		return nil, fmt.Errorf("event href is required for update")
	}

	// Generate ICS content
	icsContent := GenerateICS(event)

	headers := map[string]string{
		"Content-Type": "text/calendar; charset=utf-8",
	}

	// Use If-Match for optimistic concurrency if ETag is available
	if event.ETag != "" {
		headers["If-Match"] = event.ETag
	}

	resp, err := c.doRequest(ctx, "PUT", event.Href, []byte(icsContent), headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusPreconditionFailed {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PUT event failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	if resp.StatusCode == http.StatusPreconditionFailed {
		return nil, fmt.Errorf("event was modified by another client (ETag mismatch)")
	}

	event.ETag = resp.Header.Get("ETag")

	logger.Info("CalDAV event updated", "uid", event.UID, "href", event.Href)

	return event, nil
}

// DeleteEvent deletes an event by its href
func (c *Client) DeleteEvent(ctx context.Context, eventHref string) error {
	resp, err := c.doRequest(ctx, "DELETE", eventHref, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DELETE event failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	logger.Info("CalDAV event deleted", "href", eventHref)

	return nil
}

// GetUpcomingEvents retrieves events within a duration from now
func (c *Client) GetUpcomingEvents(ctx context.Context, calendarHref string, within time.Duration) ([]Event, error) {
	start := time.Now()
	end := start.Add(within)
	return c.QueryEvents(ctx, calendarHref, start, end)
}

// parseEventMultiStatus parses a REPORT response containing events
func parseEventMultiStatus(data []byte) ([]Event, error) {
	// We need a custom struct to handle calendar-data
	var ms struct {
		XMLName   xml.Name `xml:"multistatus"`
		Responses []struct {
			Href     string `xml:"href"`
			PropStat []struct {
				Prop struct {
					ETag         string `xml:"getetag"`
					CalendarData string `xml:"calendar-data"`
				} `xml:"prop"`
				Status string `xml:"status"`
			} `xml:"propstat"`
		} `xml:"response"`
	}

	// Define XML namespaces for unmarshaling
	data = bytes.ReplaceAll(data, []byte(`xmlns="DAV:"`), []byte(`xmlns="DAV"`))
	data = bytes.ReplaceAll(data, []byte(`xmlns="urn:ietf:params:xml:ns:caldav"`), []byte(`xmlns="caldav"`))

	if err := xml.Unmarshal(data, &ms); err != nil {
		return nil, fmt.Errorf("parse multistatus: %w", err)
	}

	var events []Event
	for _, resp := range ms.Responses {
		for _, ps := range resp.PropStat {
			if strings.Contains(ps.Status, "200") && ps.Prop.CalendarData != "" {
				event, err := ParseICS(ps.Prop.CalendarData)
				if err != nil {
					logger.Warn("Failed to parse event ICS", "href", resp.Href, "error", err)
					continue
				}
				event.Href = resp.Href
				event.ETag = ps.Prop.ETag
				events = append(events, *event)
			}
		}
	}

	return events, nil
}

// ResolveHref resolves a relative href to a full URL
func (c *Client) ResolveHref(href string) string {
	if strings.HasPrefix(href, "http") {
		return href
	}

	baseURL, err := url.Parse(c.account.URL)
	if err != nil {
		return href
	}

	relURL, err := url.Parse(href)
	if err != nil {
		return href
	}

	return baseURL.ResolveReference(relURL).String()
}
