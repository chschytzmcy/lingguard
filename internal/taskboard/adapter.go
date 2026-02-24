package taskboard

import (
	"fmt"
	"time"

	"github.com/lingguard/internal/cron"
	"github.com/lingguard/internal/subagent"
	"github.com/lingguard/pkg/logger"
)

// SubagentAdapter 子代理适配器
type SubagentAdapter struct {
	service *Service
}

// NewSubagentAdapter 创建子代理适配器
func NewSubagentAdapter(service *Service) *SubagentAdapter {
	return &SubagentAdapter{service: service}
}

// OnSubagentCreated 子代理创建时调用
func (a *SubagentAdapter) OnSubagentCreated(sub *subagent.Subagent, parentTaskID string) {
	if a.service == nil {
		return
	}

	task, err := a.service.CreateSubagentTask(sub.ID(), sub.Task(), sub.Context(), parentTaskID)
	if err != nil {
		logger.Warn("Failed to create subagent task", "subagentId", sub.ID(), "error", err)
		return
	}

	logger.Info("Subagent task created", "taskId", task.ID, "subagentId", sub.ID())
}

// OnSubagentStatusChanged 子代理状态变化时调用
func (a *SubagentAdapter) OnSubagentStatusChanged(sub *subagent.Subagent) {
	if a.service == nil {
		return
	}

	var status TaskStatus
	switch sub.Status() {
	case subagent.StatusRunning:
		status = TaskStatusRunning
	case subagent.StatusCompleted:
		status = TaskStatusCompleted
	case subagent.StatusFailed:
		status = TaskStatusFailed
	default:
		return
	}

	if err := a.service.UpdateSubagentStatus(sub.ID(), status, sub.Result(), sub.Error()); err != nil {
		logger.Warn("Failed to update subagent task status", "subagentId", sub.ID(), "error", err)
	}
}

// CronAdapter 定时任务适配器
type CronAdapter struct {
	service *Service
}

// NewCronAdapter 创建定时任务适配器
func NewCronAdapter(service *Service) *CronAdapter {
	return &CronAdapter{service: service}
}

// OnCronJobCreated 定时任务创建时调用
// 只为单次任务创建看板任务，周期性任务在执行时创建
func (a *CronAdapter) OnCronJobCreated(job *cron.CronJob) {
	if a.service == nil {
		return
	}

	// 只为单次任务创建看板任务
	isOneTime := job.Schedule.Kind == cron.ScheduleKindAt
	if !isOneTime {
		logger.Debug("Recurring cron job, will create task on each execution", "cronId", job.ID, "name", job.Name)
		return
	}

	// 检查是否已存在该 cron 任务的看板任务
	tasks, err := a.service.ListTasks(&TaskFilter{
		Source: ptrSource(TaskSourceCron),
		Limit:  100,
	})
	if err != nil {
		logger.Warn("Failed to list cron tasks", "error", err)
	} else {
		for _, t := range tasks {
			if t.SourceRef == job.ID && t.Status != TaskStatusCompleted && t.Status != TaskStatusFailed {
				// 已存在且未完成，跳过
				logger.Debug("Cron task already exists in board", "cronId", job.ID, "taskId", t.ID)
				return
			}
		}
	}

	task, err := a.service.CreateCronTask(job.ID, job.Name, job.Payload.Message)
	if err != nil {
		logger.Warn("Failed to create cron task", "cronId", job.ID, "error", err)
		return
	}

	// 直接设为进行中状态
	if err := a.service.StartTask(task.ID); err != nil {
		logger.Warn("Failed to start cron task", "taskId", task.ID, "error", err)
	}

	logger.Info("One-time cron task created", "taskId", task.ID, "cronId", job.ID, "name", job.Name)
}

// OnCronJobExecuting 定时任务执行时调用
// 对于周期性任务，每次执行创建新的看板任务
func (a *CronAdapter) OnCronJobExecuting(job *cron.CronJob) {
	if a.service == nil {
		return
	}

	isOneTime := job.Schedule.Kind == cron.ScheduleKindAt

	if isOneTime {
		// 单次任务：更新现有任务
		logger.Info("One-time cron job executing", "cronId", job.ID, "name", job.Name)
		tasks, err := a.service.ListTasks(&TaskFilter{
			Source: ptrSource(TaskSourceCron),
			Limit:  100,
		})
		if err != nil {
			logger.Warn("Failed to find cron task", "cronId", job.ID, "error", err)
			return
		}

		for _, task := range tasks {
			if task.SourceRef == job.ID && task.Status == TaskStatusRunning {
				// 确保 Metadata 已初始化
				if task.Metadata == nil {
					task.Metadata = make(map[string]interface{})
				}
				task.Metadata["executingAt"] = time.Now().Format(time.RFC3339)
				if err := a.service.UpdateTask(task); err != nil {
					logger.Warn("Failed to update cron task", "taskId", task.ID, "error", err)
				}
				return
			}
		}
	} else {
		// 周期性任务：创建新的执行任务
		logger.Info("Recurring cron job executing, creating new task", "cronId", job.ID, "name", job.Name)

		// 创建新的执行实例任务
		task := &Task{
			Title:        fmt.Sprintf("[Cron] %s", job.Name),
			Description:  job.Payload.Message,
			Status:       TaskStatusRunning,
			Column:       ColumnInProgress,
			Source:       TaskSourceCron,
			SourceRef:    job.ID,
			Assignee:     "cron-service",
			AssigneeType: AssigneeTypeAgent,
			Metadata: map[string]interface{}{
				"executingAt": time.Now().Format(time.RFC3339),
				"schedule":    job.Schedule.Kind,
			},
		}

		if err := a.service.CreateTask(task); err != nil {
			logger.Warn("Failed to create cron execution task", "cronId", job.ID, "error", err)
			return
		}

		// 手动设置为进行中
		a.service.StartTask(task.ID)
		logger.Info("Recurring cron execution task created", "taskId", task.ID, "cronId", job.ID, "name", job.Name)
	}
}

// OnCronJobCompleted 定时任务执行完成时调用
func (a *CronAdapter) OnCronJobCompleted(job *cron.CronJob, result string, errMsg string) {
	if a.service == nil {
		return
	}

	isOneTime := job.Schedule.Kind == cron.ScheduleKindAt
	logger.Info("Cron job completed", "cronId", job.ID, "name", job.Name, "isOneTime", isOneTime, "hasError", errMsg != "")

	// 查找对应的看板任务
	tasks, err := a.service.ListTasks(&TaskFilter{
		Source: ptrSource(TaskSourceCron),
		Limit:  200,
	})
	if err != nil {
		logger.Warn("Failed to find cron task", "cronId", job.ID, "error", err)
		return
	}

	// 找到最新的进行中任务
	var targetTask *Task
	for i := len(tasks) - 1; i >= 0; i-- {
		task := tasks[i]
		if task.SourceRef == job.ID && task.Status == TaskStatusRunning {
			targetTask = task
			break
		}
	}

	if targetTask == nil {
		logger.Warn("No running cron task found to complete", "cronId", job.ID, "name", job.Name)
		return
	}

	// 完成任务
	if errMsg != "" {
		if err := a.service.FailTask(targetTask.ID, errMsg); err != nil {
			logger.Warn("Failed to fail cron task", "taskId", targetTask.ID, "error", err)
		} else {
			logger.Info("Cron task failed", "taskId", targetTask.ID, "cronId", job.ID, "name", job.Name)
		}
	} else {
		if err := a.service.CompleteTask(targetTask.ID, result); err != nil {
			logger.Warn("Failed to complete cron task", "taskId", targetTask.ID, "error", err)
		} else {
			logger.Info("Cron task completed", "taskId", targetTask.ID, "cronId", job.ID, "name", job.Name)
		}
	}
}

// OnCronJobRemoved 定时任务删除时调用
func (a *CronAdapter) OnCronJobRemoved(job *cron.CronJob) {
	if a.service == nil {
		return
	}

	// 查找并删除所有相关的看板任务
	tasks, err := a.service.ListTasks(&TaskFilter{
		Source: ptrSource(TaskSourceCron),
		Limit:  200,
	})
	if err != nil {
		logger.Warn("Failed to find cron task for removal", "cronId", job.ID, "error", err)
		return
	}

	count := 0
	for _, task := range tasks {
		if task.SourceRef == job.ID {
			if err := a.service.DeleteTask(task.ID); err != nil {
				logger.Warn("Failed to delete cron task", "taskId", task.ID, "error", err)
			} else {
				count++
			}
		}
	}

	if count > 0 {
		logger.Info("Cron tasks deleted", "count", count, "cronId", job.ID)
	}
}

// ptrSource 返回 TaskSource 指针
func ptrSource(s TaskSource) *TaskSource {
	return &s
}
