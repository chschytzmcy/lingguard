package caldav

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestCalendarConnection 测试 CalDAV 连接
// 使用方法: go test -v -run TestCalendarConnection ./pkg/caldav/
func TestCalendarConnection(t *testing.T) {
	// 从配置文件获取或直接填入测试账号
	username := "u_klws1942"
	password := "EN76npR57m"

	client, err := NewClientFromConfig(
		"feishu",
		username,
		password,
		"",                         // 不使用 preset
		"https://caldav.feishu.cn", // 只需基础 URL，代码自动拼接用户路径
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

		fmt.Printf("找到 %d 个日历:\n", len(calendars))
		for _, cal := range calendars {
			fmt.Printf("  - %s (%s)\n", cal.Name, cal.Href)
		}

		if len(calendars) == 0 {
			t.Log("警告: 未找到任何日历")
		}
	})
}

// TestQueryEvents 测试查询事件
func TestQueryEvents(t *testing.T) {
	username := "u_klws1942"
	password := "EN76npR57m"

	client, err := NewClientFromConfig("feishu", username, password, "", "https://caldav.feishu.cn", 30)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	ctx := context.Background()

	// 先获取日历列表
	calendars, err := client.ListCalendars(ctx)
	if err != nil {
		t.Fatalf("列出日历失败: %v", err)
	}

	if len(calendars) == 0 {
		t.Skip("没有可用的日历")
	}

	// 使用第一个日历测试查询
	calendarHref := calendars[0].Href
	t.Logf("使用日历: %s", calendarHref)

	// 查询未来7天的事件
	start := time.Now()
	end := start.Add(7 * 24 * time.Hour)

	events, err := client.QueryEvents(ctx, calendarHref, start, end)
	if err != nil {
		t.Errorf("查询事件失败: %v", err)
		return
	}

	fmt.Printf("未来7天有 %d 个事件:\n", len(events))
	for _, event := range events {
		fmt.Printf("  - %s (%s)\n", event.Summary, event.Start.Format("2006-01-02 15:04"))
	}
}

// TestCreateAndDeleteEvent 测试创建和删除事件
// 注意：飞书 CalDAV 可能不支持创建事件（返回 409 Conflict）
func TestCreateAndDeleteEvent(t *testing.T) {
	t.Skip("飞书 CalDAV 不支持通过 PUT 创建事件，跳过此测试")

	username := "u_klws1942"
	password := "EN76npR57m"

	client, err := NewClientFromConfig("feishu", username, password, "", "https://caldav.feishu.cn", 30)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	ctx := context.Background()

	// 获取日历
	calendars, err := client.ListCalendars(ctx)
	if err != nil {
		t.Fatalf("列出日历失败: %v", err)
	}
	if len(calendars) == 0 {
		t.Skip("没有可用的日历")
	}

	calendarHref := calendars[0].Href

	// 创建测试事件（1小时后开始）
	testEvent := &Event{
		Summary:     "[测试] LingGuard 日历测试事件",
		Description: "这是一个测试事件，稍后会被删除",
		Start:       time.Now().Add(1 * time.Hour),
		End:         time.Now().Add(2 * time.Hour),
	}

	created, err := client.CreateEvent(ctx, calendarHref, testEvent)
	if err != nil {
		t.Errorf("创建事件失败: %v", err)
		return
	}

	fmt.Printf("✅ 事件创建成功:\n")
	fmt.Printf("   标题: %s\n", created.Summary)
	fmt.Printf("   时间: %s\n", created.Start.Format("2006-01-02 15:04"))
	fmt.Printf("   Href: %s\n", created.Href)

	// 清理：删除测试事件
	t.Cleanup(func() {
		if err := client.DeleteEvent(ctx, created.Href); err != nil {
			t.Logf("清理失败（删除事件）: %v", err)
		} else {
			t.Log("✅ 测试事件已删除")
		}
	})
}
