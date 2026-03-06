package caldav

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestFeishuCalendar 测试飞书日历连接
// 使用方法: go test -v -run TestFeishuCalendar ./pkg/caldav/
func TestFeishuCalendar(t *testing.T) {
	username := "u_klws1942"
	password := "EN76npR57m"

	client, err := NewClientFromConfig(
		"feishu",
		username,
		password,
		"",
		"https://caldav.feishu.cn",
		30,
	)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 测试 1: 列出日历
	t.Run("ListCalendars", func(t *testing.T) {
		calendars, err := client.ListCalendars(ctx)
		if err != nil {
			t.Errorf("列出日历失败: %v", err)
			return
		}

		fmt.Printf("飞书日历 - 找到 %d 个日历:\n", len(calendars))
		for _, cal := range calendars {
			fmt.Printf("  - %s (%s)\n", cal.Name, cal.Href)
		}

		if len(calendars) == 0 {
			t.Log("警告: 未找到任何日历")
		}
	})

	// 测试 2: 查询事件
	t.Run("QueryEvents", func(t *testing.T) {
		calendars, err := client.ListCalendars(ctx)
		if err != nil {
			t.Fatalf("列出日历失败: %v", err)
		}
		if len(calendars) == 0 {
			t.Skip("没有可用的日历")
		}

		calendarHref := calendars[0].Href
		t.Logf("使用日历: %s", calendarHref)

		start := time.Now()
		end := start.Add(7 * 24 * time.Hour)

		events, err := client.QueryEvents(ctx, calendarHref, start, end)
		if err != nil {
			t.Errorf("查询事件失败: %v", err)
			return
		}

		fmt.Printf("飞书日历 - 未来7天有 %d 个事件:\n", len(events))
		for _, event := range events {
			fmt.Printf("  - %s (%s)\n", event.Summary, event.Start.Format("2006-01-02 15:04"))
		}
	})
}

// TestDingTalkCalendar 测试钉钉日历连接
// 使用方法: go test -v -run TestDingTalkCalendar ./pkg/caldav/
func TestDingTalkCalendar(t *testing.T) {
	username := "u_zp6vbctn"
	password := "id4oy1cd"

	client, err := NewClientFromConfig(
		"dingtalk",
		username,
		password,
		"",
		"https://calendar.dingtalk.com/dav",
		30,
	)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	ctx := context.Background()

	// 测试 1: 列出日历
	t.Run("ListCalendars", func(t *testing.T) {
		calendars, err := client.ListCalendars(ctx)
		if err != nil {
			t.Errorf("列出日历失败: %v", err)
			return
		}

		fmt.Printf("钉钉日历 - 找到 %d 个日历:\n", len(calendars))
		for _, cal := range calendars {
			fmt.Printf("  - %s (%s)\n", cal.Name, cal.Href)
		}

		if len(calendars) == 0 {
			t.Log("警告: 未找到任何日历")
		}
	})

	// 测试 2: 查询事件
	t.Run("QueryEvents", func(t *testing.T) {
		calendars, err := client.ListCalendars(ctx)
		if err != nil {
			t.Fatalf("列出日历失败: %v", err)
		}
		if len(calendars) == 0 {
			t.Skip("没有可用的日历")
		}

		// 找到主日历
		var calendarHref string
		for _, cal := range calendars {
			if cal.Href == "/dav/"+username+"/primary/" {
				calendarHref = cal.Href
				break
			}
		}
		if calendarHref == "" {
			calendarHref = calendars[0].Href
		}
		t.Logf("使用日历: %s", calendarHref)

		start := time.Now()
		end := start.Add(7 * 24 * time.Hour)

		events, err := client.QueryEvents(ctx, calendarHref, start, end)
		if err != nil {
			t.Errorf("查询事件失败: %v", err)
			return
		}

		fmt.Printf("钉钉日历 - 未来7天有 %d 个事件:\n", len(events))
		for _, event := range events {
			fmt.Printf("  - %s (%s)\n", event.Summary, event.Start.Format("2006-01-02 15:04"))
		}
	})
}
