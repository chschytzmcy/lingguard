# Heartbeat 任务清单

按顺序执行以下检查，**只在有需要通知的内容时才输出信息**，否则回复 `HEARTBEAT_OK`。

## 1. 日程提醒

使用 `calendar` 工具查询未来 2 小时的日历事件。如有会议，发送提醒。

## 2. 系统监控

检查系统资源，超过门限时告警：

| 监控项 | 门限 | 命令 |
|--------|------|------|
| 磁盘空间 | > 90% | `df -h /` |
| 内存使用 | > 80% | `free -h` |

## 3. 每日记忆提炼

**触发时间**：2:00-2:30

执行记忆归档：
```
memory {"action": "refine", "archiveOld": true, "recentDays": 3}
```

## 4. 每日 AI 资讯

**触发时间**：9:00-9:30

抓取热门文章汇总：
- 机器之心：https://www.jiqizhixin.com/
- 量子位：https://www.qbitai.com/
- 新智元：https://www.163.com/dy/media/1623.html
