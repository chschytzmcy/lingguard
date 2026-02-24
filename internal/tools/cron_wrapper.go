package tools

import (
	"github.com/lingguard/internal/cron"
)

// CronServiceWrapper 包装 cron.Service 实现 CronService 接口
type CronServiceWrapper struct {
	Service        *cron.Service
	Channel        string // 当前渠道名称
	ChannelTo      string // 当前用户/群组 ID
	SourceTaskID   string // 源任务ID（用于关联用户请求任务）
	JobCreated     bool   // 本次会话是否创建了定时任务
	SourceTaskUsed bool   // 源任务ID是否被使用
}

// NewCronServiceWrapper 创建包装器
func NewCronServiceWrapper(service *cron.Service) *CronServiceWrapper {
	return &CronServiceWrapper{Service: service}
}

// SetChannelContext 设置当前渠道上下文
func (w *CronServiceWrapper) SetChannelContext(channel, to string) {
	w.Channel = channel
	w.ChannelTo = to
}

// SetSourceTaskID 设置源任务ID（并重置状态）
func (w *CronServiceWrapper) SetSourceTaskID(taskID string) {
	w.SourceTaskID = taskID
	w.JobCreated = false
	w.SourceTaskUsed = false
}

// ResetSession 重置会话状态（在开始新消息处理时调用）
func (w *CronServiceWrapper) ResetSession() {
	w.JobCreated = false
	w.SourceTaskUsed = false
}

// WasJobCreatedWithSourceTask 检查是否创建了带源任务ID的定时任务
func (w *CronServiceWrapper) WasJobCreatedWithSourceTask() bool {
	return w.JobCreated && w.SourceTaskUsed
}

// ListJobs 列出所有任务
func (w *CronServiceWrapper) ListJobs(includeDisabled bool) []*cron.CronJob {
	return w.Service.ListJobs(includeDisabled)
}

// AddJob 添加任务
func (w *CronServiceWrapper) AddJob(name string, schedule cron.CronSchedule, message string, opts ...cron.JobOption) (*cron.CronJob, error) {
	w.JobCreated = true
	// 如果设置了源任务ID，自动添加到opts
	if w.SourceTaskID != "" {
		opts = append(opts, cron.WithSourceTaskID(w.SourceTaskID))
		w.SourceTaskUsed = true
	}
	return w.Service.AddJob(name, schedule, message, opts...)
}

// RemoveJob 删除任务
func (w *CronServiceWrapper) RemoveJob(id string) bool {
	return w.Service.RemoveJob(id)
}

// EnableJob 启用/禁用任务
func (w *CronServiceWrapper) EnableJob(id string, enabled bool) *cron.CronJob {
	return w.Service.EnableJob(id, enabled)
}
