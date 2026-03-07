// Package task 异步任务管理
package task

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lingguard/internal/agent"
	"github.com/lingguard/pkg/logger"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// Task 异步任务
type Task struct {
	ID          string     `json:"id"`
	Status      TaskStatus `json:"status"`
	Progress    int        `json:"progress"`
	ProgressMsg string     `json:"progress_message,omitempty"`
	Message     string     `json:"message"`
	Media       []string   `json:"media,omitempty"`
	Result      string     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
	AgentID     string     `json:"agent_id"`
	SessionID   string     `json:"session_id,omitempty"`
	Stream      bool       `json:"stream"`
	CallbackURL string     `json:"callback_url,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// 内部字段
	events chan Event
	cancel context.CancelFunc
	mu     sync.RWMutex
}

// Event 任务事件
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// EventType 事件类型
const (
	EventStarted   = "started"
	EventProgress  = "progress"
	EventContent   = "content"
	EventCompleted = "completed"
	EventFailed    = "failed"
	EventCancelled = "cancelled"
)

// TaskOption 任务选项
type TaskOption func(*Task)

// WithSessionID 设置会话 ID
func WithSessionID(sessionID string) TaskOption {
	return func(t *Task) {
		t.SessionID = sessionID
	}
}

// WithAgentID 设置智能体 ID
func WithAgentID(agentID string) TaskOption {
	return func(t *Task) {
		t.AgentID = agentID
	}
}

// WithMedia 设置媒体
func WithMedia(media []string) TaskOption {
	return func(t *Task) {
		t.Media = media
	}
}

// WithStream 设置流式
func WithStream(stream bool) TaskOption {
	return func(t *Task) {
		t.Stream = stream
	}
}

// WithCallbackURL 设置回调 URL
func WithCallbackURL(url string) TaskOption {
	return func(t *Task) {
		t.CallbackURL = url
	}
}

// DefaultMaxConcurrent 默认最大并发数
const DefaultMaxConcurrent = 3

// Manager 任务管理器
type Manager struct {
	tasks         map[string]*Task
	mu            sync.RWMutex
	agent         *agent.Agent
	maxConcurrent int
	sem           chan struct{} // 并发信号量
	queue         []*Task       // 等待队列
	queueMu       sync.Mutex
}

// ManagerOption 管理器选项
type ManagerOption func(*Manager)

// WithMaxConcurrent 设置最大并发数
func WithMaxConcurrent(n int) ManagerOption {
	return func(m *Manager) {
		if n > 0 {
			m.maxConcurrent = n
		}
	}
}

// NewManager 创建任务管理器
func NewManager(ag *agent.Agent, opts ...ManagerOption) *Manager {
	m := &Manager{
		tasks:         make(map[string]*Task),
		agent:         ag,
		maxConcurrent: DefaultMaxConcurrent,
	}

	for _, opt := range opts {
		opt(m)
	}

	m.sem = make(chan struct{}, m.maxConcurrent)
	return m
}

// Create 创建新任务
func (m *Manager) Create(message string, opts ...TaskOption) (*Task, error) {
	task := &Task{
		ID:        "task-" + uuid.New().String()[:8],
		Status:    TaskStatusPending,
		Message:   message,
		AgentID:   "default",
		SessionID: "session-" + uuid.New().String()[:8], // 自动生成 session_id
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		events:    make(chan Event, 100),
	}

	// 应用选项（可能覆盖 SessionID）
	for _, opt := range opts {
		opt(task)
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	// 加入 FIFO 队列
	m.queueMu.Lock()
	m.queue = append(m.queue, task)
	m.queueMu.Unlock()

	// 启动队列处理（异步）
	go m.processQueue()

	logger.Info("Task created", "taskId", task.ID, "queueLength", len(m.queue))
	return task, nil
}

// processQueue 处理队列，等待信号量槽位
func (m *Manager) processQueue() {
	m.queueMu.Lock()
	if len(m.queue) == 0 {
		m.queueMu.Unlock()
		return
	}

	// 取出队首任务 (FIFO)
	task := m.queue[0]
	m.queue = m.queue[1:]
	m.queueMu.Unlock()

	// 等待获取信号量槽位（阻塞直到有空位）
	m.sem <- struct{}{}

	// 获得槽位，检查任务是否已取消
	task.mu.Lock()
	if task.Status == TaskStatusCancelled {
		task.mu.Unlock()
		<-m.sem // 已取消，释放槽位
		return
	}
	task.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	task.mu.Lock()
	task.cancel = cancel
	task.mu.Unlock()

	m.execute(ctx, task)
}

// execute 执行任务
func (m *Manager) execute(ctx context.Context, task *Task) {
	task.mu.Lock()
	task.Status = TaskStatusRunning
	task.UpdatedAt = time.Now()

	// 如果没有 session_id，生成一个
	if task.SessionID == "" {
		task.SessionID = "task-session-" + task.ID
	}
	task.mu.Unlock()

	task.emit(Event{Type: EventStarted, Data: map[string]interface{}{
		"task_id":    task.ID,
		"session_id": task.SessionID,
	}})

	var result string
	var err error

	// 根据是否有媒体选择不同的调用方式
	if len(task.Media) > 0 {
		result, err = m.agent.ProcessMessageWithMedia(ctx, task.SessionID, task.Message, task.Media)
	} else {
		result, err = m.agent.ProcessMessage(ctx, task.SessionID, task.Message)
	}

	now := time.Now()
	task.mu.Lock()
	task.UpdatedAt = now
	task.CompletedAt = &now

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		task.mu.Unlock()

		task.emit(Event{Type: EventFailed, Data: map[string]interface{}{
			"error": err.Error(),
		}})
		logger.Error("Task failed", "taskId", task.ID, "error", err)
	} else {
		task.Status = TaskStatusCompleted
		task.Result = result
		task.Progress = 100
		task.mu.Unlock()

		task.emit(Event{Type: EventCompleted, Data: map[string]interface{}{
			"result":   result,
			"task_id":  task.ID,
			"duration": now.Sub(task.CreatedAt).Milliseconds(),
		}})
		logger.Info("Task completed", "taskId", task.ID, "duration", now.Sub(task.CreatedAt).Milliseconds())
	}

	// 关闭事件通道
	close(task.events)

	// 发送回调
	if task.CallbackURL != "" {
		go m.sendCallback(task, result, err)
	}

	// 释放信号量槽位
	<-m.sem
}

// Get 获取任务
func (m *Manager) Get(taskID string) *Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tasks[taskID]
}

// Cancel 取消任务
func (m *Manager) Cancel(taskID string) bool {
	m.mu.RLock()
	task, ok := m.tasks[taskID]
	m.mu.RUnlock()

	if !ok {
		return false
	}

	task.mu.Lock()
	defer task.mu.Unlock()

	if task.Status != TaskStatusPending && task.Status != TaskStatusRunning {
		return false
	}

	if task.cancel != nil {
		task.cancel()
	}

	task.Status = TaskStatusCancelled
	now := time.Now()
	task.UpdatedAt = now
	task.CompletedAt = &now

	task.emit(Event{Type: EventCancelled, Data: map[string]interface{}{
		"task_id": task.ID,
	}})

	logger.Info("Task cancelled", "taskId", task.ID)
	return true
}

// Delete 删除任务
func (m *Manager) Delete(taskID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return false
	}

	// 如果任务正在运行，先取消
	if task.Status == TaskStatusRunning && task.cancel != nil {
		task.cancel()
	}

	delete(m.tasks, taskID)
	logger.Info("Task deleted", "taskId", taskID)
	return true
}

// List 列出任务
func (m *Manager) List(status TaskStatus, limit, offset int) []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Task
	for _, task := range m.tasks {
		if status == "" || task.Status == status {
			result = append(result, task)
		}
	}

	// 按创建时间倒序
	// sort.Slice(result, func(i, j int) bool {
	// 	return result[i].CreatedAt.After(result[j].CreatedAt)
	// })

	// 分页
	if offset >= len(result) {
		return []*Task{}
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end]
}

// Count 统计任务数量
func (m *Manager) Count(status TaskStatus) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if status == "" {
		return len(m.tasks)
	}

	count := 0
	for _, task := range m.tasks {
		if task.Status == status {
			count++
		}
	}
	return count
}

// emit 发送事件
func (t *Task) emit(event Event) {
	select {
	case t.events <- event:
	default:
		// 通道满了，丢弃事件
		logger.Warn("Task event channel full, dropping event", "taskId", t.ID, "type", event.Type)
	}
}

// Events 获取事件通道
func (t *Task) Events() <-chan Event {
	return t.events
}

// CallbackPayload 回调请求体
type CallbackPayload struct {
	TaskID      string     `json:"task_id"`
	Status      TaskStatus `json:"status"`
	Result      string     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
	CompletedAt string     `json:"completed_at"`
	DurationMs  int64      `json:"duration_ms"`
}

// sendCallback 发送回调请求
func (m *Manager) sendCallback(task *Task, result string, execErr error) {
	task.mu.RLock()
	payload := CallbackPayload{
		TaskID:      task.ID,
		Status:      task.Status,
		CompletedAt: task.CompletedAt.Format(time.RFC3339),
		DurationMs:  task.CompletedAt.Sub(task.CreatedAt).Milliseconds(),
	}
	if execErr != nil {
		payload.Error = execErr.Error()
	} else {
		payload.Result = result
	}
	callbackURL := task.CallbackURL
	task.mu.RUnlock()

	body, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Failed to marshal callback payload", "taskId", task.ID, "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", callbackURL, bytes.NewReader(body))
	if err != nil {
		logger.Error("Failed to create callback request", "taskId", task.ID, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error("Failed to send callback", "taskId", task.ID, "url", callbackURL, "error", err)
		return
	}
	defer resp.Body.Close()

	logger.Info("Callback sent", "taskId", task.ID, "url", callbackURL, "status", resp.StatusCode)
}
