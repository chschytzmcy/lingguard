---
name: calendar
description: CalDAV 日历管理。当用户说"查看日历"、"查看日程"、"今天有什么安排"、"明天有什么会议"、"添加日程"、"创建事件"时，加载此 skill
metadata: {"nanobot":{"emoji":"📅"}}
---

# CalDAV 日历管理

## 配置示例

```json
{
  "tools": {
    "calendar": {
      "enabled": true,
      "default": "feishu",
      "accounts": [
        {
          "name": "feishu",
          "url": "https://caldav.feishu.cn",
          "username": "u_xxxxxxxx",
          "password": "your-app-token"
        },
        {
          "name": "dingtalk",
          "url": "https://calendar.dingtalk.com/dav",
          "username": "u_xxxxxxxx",
          "password": "your-app-token"
        }
      ]
    }
  }
}
```

## 示例对话

### 查看今天日程

**用户**: 今天有什么安排？
**调用**:
```json
{
  "action": "query",
  "calendar": "/dav/user/calendar/",
  "start": "2026-03-06T00:00",
  "end": "2026-03-06T23:59"
}
```

### 查看即将到来的事件

**用户**: 未来一小时有什么事吗？
**调用**:
```json
{
  "action": "upcoming",
  "calendar": "/dav/user/calendar/",
  "within": "1h"
}
```

### 创建会议

**用户**: 明天下午3点帮我安排一个项目会议，时长1小时
**调用**:
```json
{
  "action": "create",
  "calendar": "/dav/user/calendar/",
  "summary": "项目会议",
  "start": "2026-03-07T15:00",
  "end": "2026-03-07T16:00"
}
```

### 创建全天事件

**用户**: 3月15日是我的生日，记到日历上
**调用**:
```json
{
  "action": "create",
  "calendar": "/dav/user/calendar/",
  "summary": "我的生日",
  "start": "2026-03-15",
  "all_day": true
}
```

## Heartbeat 集成

在心跳服务中可以主动检查日程：

```markdown
## 日程检查
每30分钟检查即将到来的事件：
{"action": "upcoming", "calendar": "/dav/user/calendar/", "within": "1h"}
如果有即将开始的事件，发送提醒。
```

## 注意事项

1. **首次使用**：先用 `list_calendars` 获取日历路径
2. **飞书限制**：飞书 CalDAV 不支持创建/修改/删除事件，只能查询
3. **日历权限**：某些日历可能只读，创建/更新/删除操作会失败
