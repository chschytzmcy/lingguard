// Package tasksync 提供任务同步接口，避免循环导入
package tasksync

import "context"

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// TaskEvent 任务事件类型
type TaskEvent string

const (
	TaskEventCreated   TaskEvent = "created"
	TaskEventStarted   TaskEvent = "started"
	TaskEventCompleted TaskEvent = "completed"
	TaskEventFailed    TaskEvent = "failed"
)

// TaskSource 任务来源
type TaskSource string

const (
	TaskSourceCron      TaskSource = "cron"
	TaskSourceSubagent  TaskSource = "subagent"
	TaskSourceHeartbeat TaskSource = "heartbeat"
	TaskSourceAgent     TaskSource = "agent"
)

// TaskAssignee 任务分配者
type TaskAssignee string

const (
	TaskAssigneeUser TaskAssignee = "user"
	TaskAssigneeAI   TaskAssignee = "ai"
	TaskAssigneeBoth TaskAssignee = "both"
)

// TaskPriority 任务优先级
type TaskPriority string

const (
	TaskPriorityLow    TaskPriority = "low"
	TaskPriorityMedium TaskPriority = "medium"
	TaskPriorityHigh   TaskPriority = "high"
)

// TaskMetadata 任务元数据
type TaskMetadata struct {
	Source           string `json:"source,omitempty"`
	Command          string `json:"command,omitempty"`
	WorkingDirectory string `json:"workingDirectory,omitempty"`
}

// TaskSyncEvent 任务同步事件
type TaskSyncEvent struct {
	Source      TaskSource    `json:"source"`
	Event       TaskEvent     `json:"event"`
	ExternalID  string        `json:"externalId"`
	Title       string        `json:"title"`
	Description string        `json:"description,omitempty"`
	Status      TaskStatus    `json:"status"`
	Assignee    TaskAssignee  `json:"assignee"`
	SessionID   string        `json:"sessionId,omitempty"`
	SubagentID  string        `json:"subagentId,omitempty"`
	Priority    TaskPriority  `json:"priority,omitempty"`
	Tags        []string      `json:"tags,omitempty"`
	Result      string        `json:"result,omitempty"`
	Error       string        `json:"error,omitempty"`
	Metadata    *TaskMetadata `json:"metadata,omitempty"`
}

// TaskSyncer 任务同步器接口
type TaskSyncer interface {
	Sync(ctx context.Context, event *TaskSyncEvent) error
}

// NoopTaskSyncer 空同步器
type NoopTaskSyncer struct{}

func (s *NoopTaskSyncer) Sync(ctx context.Context, event *TaskSyncEvent) error {
	return nil
}
