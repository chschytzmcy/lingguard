---
name: calendar
description: CalDAV 日历管理。当用户说"查看日历"、"查看日程"、"添加日程"、"创建事件"、"日历事件"、"今天有什么安排"、"明天有什么会议"时，必须先加载此 skill
metadata: {"nanobot":{"emoji":"📅"}}
---

# CalDAV 日历管理

使用 `calendar` 工具管理 CalDAV 日历事件。

## 支持的服务

通过预设模板支持以下 CalDAV 服务：

| Preset | 服务 | URL |
|--------|------|-----|
| `feishu` | 飞书日历 | `https://caldav.feishu.cn` |
| `apple` | Apple iCloud | `https://caldav.icloud.com` |
| `google` | Google Calendar | `https://apidata.googleusercontent.com/caldav/v2/{{username}}/events` |

也可以使用自定义 URL 对接其他 CalDAV 服务。配置时只需提供基础 URL，代码会自动拼接用户路径。

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
          "name": "apple",
          "url": "https://caldav.icloud.com",
          "username": "your-apple-id",
          "password": "your-app-specific-password"
        }
      ]
    }
  }
}
```

**注意**：URL 只需配置基础域名，代码会自动根据 username 拼接完整路径。

## 触发关键词

- "查看日历"、"查看日程"、"查看事件"
- "今天有什么安排"、"明天有什么会议"
- "添加日程"、"创建事件"、"新建会议"
- "修改日程"、"更新事件"
- "删除日程"、"取消事件"
- "即将到来的事件"

## 工作流程

### 1. 首次使用：列出日历

```json
{"action": "list_calendars"}
```

返回账户下所有可用的日历，记下要操作的日历 `href`。

### 2. 查询事件

```json
{
  "action": "query",
  "calendar": "/dav/user@example.com/calendar/",
  "start": "2026-03-06T00:00",
  "end": "2026-03-06T23:59"
}
```

时间格式支持：
- 绝对时间：`2026-03-06T15:00`、`2026-03-06`
- 相对时间：`now`、`+1h`、`+1d`、`-30m`

### 3. 获取即将到来的事件

```json
{
  "action": "upcoming",
  "calendar": "/dav/user@example.com/calendar/",
  "within": "24h"
}
```

`within` 格式：`1h`、`24h`、`7d` 等。

### 4. 创建事件

```json
{
  "action": "create",
  "calendar": "/dav/user@example.com/calendar/",
  "summary": "项目评审会议",
  "start": "2026-03-06T14:00",
  "end": "2026-03-06T15:30",
  "description": "讨论Q2项目进展",
  "location": "会议室A"
}
```

### 5. 更新事件

```json
{
  "action": "update",
  "event_href": "/dav/user@example.com/calendar/event123.ics",
  "summary": "项目评审会议（改期）",
  "start": "2026-03-06T16:00"
}
```

### 6. 删除事件

```json
{
  "action": "delete",
  "event_href": "/dav/user@example.com/calendar/event123.ics"
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

### 取消会议

**用户**: 取消明天的项目会议
**调用**: 先用 query 找到事件，然后：
```json
{
  "action": "delete",
  "event_href": "/dav/user/calendar/project-meeting.ics"
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

## 参数说明

| 参数 | 类型 | 说明 |
|------|------|------|
| `action` | string | 操作类型（必填） |
| `account` | string | 账户名称（可选，使用默认账户） |
| `calendar` | string | 日历路径（list_calendars 获取） |
| `event_href` | string | 事件路径（get/update/delete 使用） |
| `start` | string | 开始时间 |
| `end` | string | 结束时间 |
| `within` | string | 时间范围（upcoming 使用） |
| `summary` | string | 事件标题 |
| `description` | string | 事件描述 |
| `location` | string | 事件地点 |
| `all_day` | boolean | 是否全天事件 |
| `status` | string | 事件状态：TENTATIVE/CONFIRMED/CANCELLED |

## 注意事项

1. **首次使用**：先用 `list_calendars` 获取日历路径
2. **身份验证**：需要配置账户用户名和密码（或应用令牌）
3. **时间格式**：推荐使用 ISO 8601 格式 `2006-01-02T15:04`
4. **日历权限**：某些日历可能只读，创建/更新/删除操作会失败
5. **多账户**：支持配置多个日历账户，通过 `account` 参数切换
6. **飞书限制**：飞书 CalDAV 不支持通过 PUT 创建/修改事件，只能查询事件

## 支持的 CalDAV 操作

- `PROPFIND` - 发现日历
- `REPORT` - 查询事件
- `GET` - 获取事件
- `PUT` - 创建/更新事件
- `DELETE` - 删除事件
