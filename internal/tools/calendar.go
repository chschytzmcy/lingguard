// Package tools 工具实现 - 日历工具
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/pkg/caldav"
	"github.com/lingguard/pkg/logger"
)

// CalendarTool 日历工具
type CalendarTool struct {
	config  *config.CalendarConfig
	clients map[string]*caldav.Client // account name -> client
}

// NewCalendarTool 创建日历工具
func NewCalendarTool(cfg *config.CalendarConfig) *CalendarTool {
	tool := &CalendarTool{
		config:  cfg,
		clients: make(map[string]*caldav.Client),
	}

	logger.Info("Initializing CalendarTool", "enabled", cfg.Enabled, "default", cfg.Default, "accounts", len(cfg.Accounts))

	// Initialize clients for all accounts
	for _, account := range cfg.Accounts {
		logger.Info("Creating CalDAV client", "account", account.Name, "url", account.URL, "preset", account.Preset, "username", account.Username)

		client, err := caldav.NewClientFromConfig(
			account.Name,
			account.Username,
			account.Password,
			account.Preset,
			account.URL,
			account.Timeout,
		)
		if err != nil {
			logger.Error("Failed to create CalDAV client", "account", account.Name, "error", err)
			continue
		}
		// Set token if available
		if account.Token != "" {
			client.SetToken(account.Token)
		}
		tool.clients[account.Name] = client
		logger.Info("Calendar client initialized successfully", "account", account.Name, "url", account.URL)
	}

	logger.Info("CalendarTool initialized", "total_clients", len(tool.clients))
	return tool
}

func (t *CalendarTool) Name() string { return "calendar" }

func (t *CalendarTool) Description() string {
	return `CalDAV 日历管理工具，支持查询、创建、更新、删除日历事件。

**支持的 action**：
- list_calendars: 列出账户下的所有日历
- query: 查询时间范围内的事件
- get: 获取单个事件详情
- create: 创建新事件
- update: 更新事件
- delete: 删除事件
- upcoming: 获取即将到来的事件

**注意**：
- 调用前必须先加载 calendar skill 了解详细用法：skill --name calendar
- 飞书 CalDAV 不支持创建/更新/删除事件，只能查询`
}

func (t *CalendarTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"list_calendars", "query", "get", "create", "update", "delete", "upcoming"},
				"description": "操作类型",
			},
			"account": map[string]interface{}{
				"type":        "string",
				"description": "账户名称（可选，使用默认账户）",
			},
			"calendar": map[string]interface{}{
				"type":        "string",
				"description": "日历路径（list_calendars 可获取）",
			},
			"event_href": map[string]interface{}{
				"type":        "string",
				"description": "事件路径（get/update/delete 时使用）",
			},
			"start": map[string]interface{}{
				"type":        "string",
				"description": "开始时间（格式: 2006-01-02T15:04 或相对时间如 now, +1h, +1d）",
			},
			"end": map[string]interface{}{
				"type":        "string",
				"description": "结束时间（同上）",
			},
			"within": map[string]interface{}{
				"type":        "string",
				"description": "时间范围（upcoming 时使用，如 1h, 24h, 7d）",
			},
			"summary": map[string]interface{}{
				"type":        "string",
				"description": "事件标题（create/update 时使用）",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "事件描述（可选）",
			},
			"location": map[string]interface{}{
				"type":        "string",
				"description": "事件地点（可选）",
			},
			"all_day": map[string]interface{}{
				"type":        "boolean",
				"description": "是否全天事件",
			},
			"uid": map[string]interface{}{
				"type":        "string",
				"description": "事件 UID（update 时可选）",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"TENTATIVE", "CONFIRMED", "CANCELLED"},
				"description": "事件状态",
			},
		},
		"required": []string{"action"},
	}
}

func (t *CalendarTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Action      string `json:"action"`
		Account     string `json:"account"`
		Calendar    string `json:"calendar"`
		EventHref   string `json:"event_href"`
		Start       string `json:"start"`
		End         string `json:"end"`
		Within      string `json:"within"`
		Summary     string `json:"summary"`
		Description string `json:"description"`
		Location    string `json:"location"`
		AllDay      *bool  `json:"all_day"`
		UID         string `json:"uid"`
		Status      string `json:"status"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	// Check if any clients are available
	if len(t.clients) == 0 {
		return "", fmt.Errorf("没有可用的日历账户。请在配置文件中添加 CalDAV 账户:\n" +
			"```json\n" +
			"{\n" +
			"  \"tools\": {\n" +
			"    \"calendar\": {\n" +
			"      \"enabled\": true,\n" +
			"      \"accounts\": [{\n" +
			"        \"name\": \"feishu\",\n" +
			"        \"url\": \"https://caldav.feishu.cn/\",\n" +
			"        \"username\": \"u_xxxxxxxx\",\n" +
			"        \"password\": \"your-token\"\n" +
			"      }]\n" +
			"    }\n" +
			"  }\n" +
			"}\n" +
			"```\n")
	}

	// Get client
	account := p.Account
	if account == "" {
		account = t.config.Default
	}
	if account == "" && len(t.clients) > 0 {
		// Use first available account
		for name := range t.clients {
			account = name
			break
		}
	}

	client, ok := t.clients[account]
	if !ok {
		return "", fmt.Errorf("calendar account not found: %s (available: %v)", account, t.getAccountNames())
	}

	switch p.Action {
	case "list_calendars":
		return t.listCalendars(ctx, client)
	case "query":
		return t.queryEvents(ctx, client, p.Calendar, p.Start, p.End)
	case "get":
		return t.getEvent(ctx, client, p.EventHref)
	case "create":
		return t.createEvent(ctx, client, p.Calendar, &p)
	case "update":
		return t.updateEvent(ctx, client, &p)
	case "delete":
		return t.deleteEvent(ctx, client, p.EventHref)
	case "upcoming":
		return t.getUpcoming(ctx, client, p.Calendar, p.Within)
	default:
		return "", fmt.Errorf("unknown action: %s", p.Action)
	}
}

func (t *CalendarTool) listCalendars(ctx context.Context, client *caldav.Client) (string, error) {
	calendars, err := client.ListCalendars(ctx)
	if err != nil {
		return "", fmt.Errorf("list calendars: %w", err)
	}

	if len(calendars) == 0 {
		return "No calendars found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d calendars:\n\n", len(calendars)))

	for _, cal := range calendars {
		sb.WriteString(fmt.Sprintf("📁 %s\n", cal.Name))
		sb.WriteString(fmt.Sprintf("   Href: %s\n", cal.Href))
		if cal.Description != "" {
			sb.WriteString(fmt.Sprintf("   Description: %s\n", cal.Description))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func (t *CalendarTool) queryEvents(ctx context.Context, client *caldav.Client, calendarHref, startStr, endStr string) (string, error) {
	if calendarHref == "" {
		return "", fmt.Errorf("calendar parameter is required (use list_calendars first)")
	}

	// Parse time range
	start, err := parseTimeOrRelative(startStr, time.Now())
	if err != nil {
		return "", fmt.Errorf("invalid start time: %w", err)
	}

	end, err := parseTimeOrRelative(endStr, start.Add(24*time.Hour))
	if err != nil {
		return "", fmt.Errorf("invalid end time: %w", err)
	}

	events, err := client.QueryEvents(ctx, calendarHref, start, end)
	if err != nil {
		return "", fmt.Errorf("query events: %w", err)
	}

	if len(events) == 0 {
		return fmt.Sprintf("No events found between %s and %s.", start.Format("2006-01-02 15:04"), end.Format("2006-01-02 15:04")), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d events (%s ~ %s):\n\n", len(events), start.Format("2006-01-02 15:04"), end.Format("2006-01-02 15:04")))

	for i, event := range events {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, caldav.FormatEventForDisplay(&event)))
		sb.WriteString(fmt.Sprintf("   Href: %s\n\n", event.Href))
	}

	return sb.String(), nil
}

func (t *CalendarTool) getEvent(ctx context.Context, client *caldav.Client, eventHref string) (string, error) {
	if eventHref == "" {
		return "", fmt.Errorf("event_href parameter is required")
	}

	event, err := client.GetEvent(ctx, eventHref)
	if err != nil {
		return "", fmt.Errorf("get event: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("📅 Event Details:\n\n")
	sb.WriteString(caldav.FormatEventForDisplay(event))
	sb.WriteString(fmt.Sprintf("\nUID: %s\n", event.UID))
	sb.WriteString(fmt.Sprintf("Href: %s\n", event.Href))

	return sb.String(), nil
}

func (t *CalendarTool) createEvent(ctx context.Context, client *caldav.Client, calendarHref string, p *struct {
	Action      string `json:"action"`
	Account     string `json:"account"`
	Calendar    string `json:"calendar"`
	EventHref   string `json:"event_href"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Within      string `json:"within"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Location    string `json:"location"`
	AllDay      *bool  `json:"all_day"`
	UID         string `json:"uid"`
	Status      string `json:"status"`
}) (string, error) {
	if calendarHref == "" {
		return "", fmt.Errorf("calendar parameter is required (use list_calendars first)")
	}
	if p.Summary == "" {
		return "", fmt.Errorf("summary parameter is required")
	}
	if p.Start == "" {
		return "", fmt.Errorf("start parameter is required")
	}

	// Parse start time
	start, err := parseTimeOrRelative(p.Start, time.Now())
	if err != nil {
		return "", fmt.Errorf("invalid start time: %w", err)
	}

	// Parse end time (default to 1 hour after start)
	end, err := parseTimeOrRelative(p.End, start.Add(time.Hour))
	if err != nil {
		end = start.Add(time.Hour)
	}

	// Check all-day flag
	allDay := p.AllDay != nil && *p.AllDay

	event := &caldav.Event{
		Summary:     p.Summary,
		Description: p.Description,
		Location:    p.Location,
		Start:       start,
		End:         end,
		AllDay:      allDay,
		Status:      "CONFIRMED",
	}

	if p.Status != "" {
		event.Status = p.Status
	}

	created, err := client.CreateEvent(ctx, calendarHref, event)
	if err != nil {
		return "", fmt.Errorf("create event: %w", err)
	}

	return fmt.Sprintf("Event created!\n%s\nHref: %s", caldav.FormatEventForDisplay(created), created.Href), nil
}

func (t *CalendarTool) updateEvent(ctx context.Context, client *caldav.Client, p *struct {
	Action      string `json:"action"`
	Account     string `json:"account"`
	Calendar    string `json:"calendar"`
	EventHref   string `json:"event_href"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Within      string `json:"within"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Location    string `json:"location"`
	AllDay      *bool  `json:"all_day"`
	UID         string `json:"uid"`
	Status      string `json:"status"`
}) (string, error) {
	if p.EventHref == "" {
		return "", fmt.Errorf("event_href parameter is required")
	}

	// Get existing event first
	event, err := client.GetEvent(ctx, p.EventHref)
	if err != nil {
		return "", fmt.Errorf("get event: %w", err)
	}

	// Update fields if provided
	if p.Summary != "" {
		event.Summary = p.Summary
	}
	if p.Description != "" {
		event.Description = p.Description
	}
	if p.Location != "" {
		event.Location = p.Location
	}
	if p.Status != "" {
		event.Status = p.Status
	}
	if p.AllDay != nil {
		event.AllDay = *p.AllDay
	}

	// Parse times if provided
	if p.Start != "" {
		start, err := parseTimeOrRelative(p.Start, time.Now())
		if err != nil {
			return "", fmt.Errorf("invalid start time: %w", err)
		}
		event.Start = start
	}
	if p.End != "" {
		end, err := parseTimeOrRelative(p.End, event.Start.Add(time.Hour))
		if err != nil {
			return "", fmt.Errorf("invalid end time: %w", err)
		}
		event.End = end
	}

	updated, err := client.UpdateEvent(ctx, event)
	if err != nil {
		return "", fmt.Errorf("update event: %w", err)
	}

	return fmt.Sprintf("Event updated!\n%s\nHref: %s", caldav.FormatEventForDisplay(updated), updated.Href), nil
}

func (t *CalendarTool) deleteEvent(ctx context.Context, client *caldav.Client, eventHref string) (string, error) {
	if eventHref == "" {
		return "", fmt.Errorf("event_href parameter is required")
	}

	// Get event details first for confirmation message
	event, err := client.GetEvent(ctx, eventHref)
	var eventInfo string
	if err != nil {
		eventInfo = eventHref
	} else {
		eventInfo = fmt.Sprintf("%s (%s)", event.Summary, event.Start.Format("2006-01-02"))
	}

	if err := client.DeleteEvent(ctx, eventHref); err != nil {
		return "", fmt.Errorf("delete event: %w", err)
	}

	return fmt.Sprintf("Event deleted: %s", eventInfo), nil
}

func (t *CalendarTool) getUpcoming(ctx context.Context, client *caldav.Client, calendarHref, withinStr string) (string, error) {
	if calendarHref == "" {
		return "", fmt.Errorf("calendar parameter is required (use list_calendars first)")
	}

	// Parse within duration - support both Go duration and relative time format
	var within time.Duration = 24 * time.Hour // default 24 hours
	if withinStr != "" {
		// Try relative time format first (supports d, w, y units)
		if end, err := caldav.ParseRelativeTime("+"+withinStr, time.Now()); err == nil {
			within = end.Sub(time.Now())
		} else {
			// Fallback to Go duration format
			d, err := time.ParseDuration(withinStr)
			if err != nil {
				return "", fmt.Errorf("invalid within duration: %w (use format like 1h, 24h, 7d)", err)
			}
			within = d
		}
	}

	events, err := client.GetUpcomingEvents(ctx, calendarHref, within)
	if err != nil {
		return "", fmt.Errorf("get upcoming events: %w", err)
	}

	if len(events) == 0 {
		return fmt.Sprintf("No upcoming events within %s.", within), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📅 Upcoming events (within %s):\n\n", within))

	now := time.Now()
	for i, event := range events {
		// Calculate time until event
		timeUntil := event.Start.Sub(now)
		timeUntilStr := formatDuration(timeUntil)

		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, caldav.FormatEventForDisplay(&event)))
		sb.WriteString(fmt.Sprintf("   ⏰ Starts in: %s\n", timeUntilStr))
		sb.WriteString(fmt.Sprintf("   Href: %s\n\n", event.Href))
	}

	return sb.String(), nil
}

func (t *CalendarTool) IsDangerous() bool { return true }

func (t *CalendarTool) ShouldLoadByDefault() bool { return true }

// getAccountNames returns a list of available account names
func (t *CalendarTool) getAccountNames() []string {
	names := make([]string, 0, len(t.clients))
	for name := range t.clients {
		names = append(names, name)
	}
	return names
}

// parseTimeOrRelative parses a time string or relative time expression
func parseTimeOrRelative(s string, base time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}

	// Try relative time first
	if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") || strings.ToLower(s) == "now" {
		return caldav.ParseRelativeTime(s, base)
	}

	// Try ICS datetime formats
	t, err := caldav.ParseICSDateTime(s)
	if err == nil {
		return t, nil
	}

	// Try common Go formats
	formats := []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}

	for _, format := range formats {
		t, err := time.Parse(format, s)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", s)
}

// formatDuration formats a duration for human-readable display
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "already started"
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 && days == 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}

	if len(parts) == 0 {
		return "now"
	}

	return strings.Join(parts, " ")
}
