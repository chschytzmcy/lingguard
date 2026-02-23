package tools

import (
	"context"
	"sync"
	"time"

	taskSyncPkg "github.com/lingguard/internal/tasksync"
)

// TasksBoardSyncer 任务看板同步器实现
type TasksBoardSyncer struct {
	tool    *TasksBoardTool
	enabled bool
	mu      sync.RWMutex
}

// NewTasksBoardSyncer 创建任务看板同步器
func NewTasksBoardSyncer(tool *TasksBoardTool) *TasksBoardSyncer {
	return &TasksBoardSyncer{
		tool:    tool,
		enabled: tool != nil && tool.config != nil && tool.config.URL != "",
	}
}

// Sync 同步任务事件到看板
func (s *TasksBoardSyncer) Sync(ctx context.Context, event *taskSyncPkg.TaskSyncEvent) error {
	s.mu.RLock()
	enabled := s.enabled
	s.mu.RUnlock()

	if !enabled {
		return nil
	}

	task := &Task{
		ExternalID:  event.ExternalID,
		Title:       event.Title,
		Description: event.Description,
		Status:      TaskStatus(event.Status),
		Assignee:    TaskAssignee(event.Assignee),
		SessionID:   event.SessionID,
		SubagentID:  event.SubagentID,
		Priority:    TaskPriority(event.Priority),
		Tags:        event.Tags,
		Result:      event.Result,
		Error:       event.Error,
	}
	if event.Metadata != nil {
		task.Metadata = &TaskMetadata{
			Source:           event.Metadata.Source,
			Command:          event.Metadata.Command,
			WorkingDirectory: event.Metadata.WorkingDirectory,
		}
	}

	// 根据事件类型设置时间戳
	now := time.Now().UnixMilli()
	switch event.Event {
	case taskSyncPkg.TaskEventStarted:
		task.StartedAt = &now
	case taskSyncPkg.TaskEventCompleted, taskSyncPkg.TaskEventFailed:
		task.CompletedAt = &now
		task.StartedAt = &now // 确保有开始时间
	}

	// 使用 sync 操作（通过 externalId 更新或创建）
	_, err := s.tool.syncTasks(ctx, []*Task{task})
	return err
}

// SetEnabled 设置是否启用
func (s *TasksBoardSyncer) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled && s.tool != nil && s.tool.config != nil && s.tool.config.URL != ""
}

// IsEnabled 返回是否启用
func (s *TasksBoardSyncer) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}
