// Package heartbeat 心跳服务 - 定期唤醒 Agent 检查任务
package heartbeat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	taskSyncPkg "github.com/lingguard/internal/tasksync"
	"github.com/lingguard/pkg/logger"
)

// DefaultInterval 默认心跳间隔 (30分钟)
const DefaultInterval = 30 * time.Minute

// HeartbeatPrompt 心跳提示
const HeartbeatPrompt = `Read HEARTBEAT.md in your workspace (if it exists).
Follow any instructions or tasks listed there.
If nothing needs attention, reply with just: HEARTBEAT_OK`

// HeartbeatOKToken 无任务时的响应标识
const HeartbeatOKToken = "HEARTBEAT_OK"

// AgentCallback Agent 处理回调
type AgentCallback func(ctx context.Context, prompt string) (string, error)

// Config 心跳服务配置
type Config struct {
	Enabled       bool          `json:"enabled"`                 // 是否启用心跳
	Interval      time.Duration `json:"interval"`                // 心跳间隔
	WorkspacePath string        `json:"workspacePath,omitempty"` // 工作空间路径，用于读取 HEARTBEAT.md
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:  true,
		Interval: DefaultInterval,
	}
}

// Service 心跳服务
type Service struct {
	config       *Config
	onHeartbeat  AgentCallback
	taskSyncer   taskSyncPkg.TaskSyncer // 任务看板同步器
	heartbeatDir string                 // HEARTBEAT.md 所在目录

	mu      sync.RWMutex
	running bool
	ticker  *time.Ticker
	stopCh  chan struct{}

	// 统计信息
	lastRunAt    time.Time
	lastStatus   string
	lastResponse string
	runCount     int
}

// NewService 创建心跳服务
func NewService(cfg *Config, onHeartbeat AgentCallback, taskSyncer taskSyncPkg.TaskSyncer) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultInterval
	}
	if taskSyncer == nil {
		taskSyncer = &taskSyncPkg.NoopTaskSyncer{}
	}

	return &Service{
		config:      cfg,
		onHeartbeat: onHeartbeat,
		taskSyncer:  taskSyncer,
		stopCh:      make(chan struct{}),
	}
}

// SetWorkspace 设置工作空间路径
func (s *Service) SetWorkspace(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.heartbeatDir = path
}

// Start 启动心跳服务
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	s.running = true
	s.ticker = time.NewTicker(s.config.Interval)

	go s.runLoop()

	logger.Info("Heartbeat service started", "interval", s.config.Interval)
	return nil
}

// Stop 停止心跳服务
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	if s.ticker != nil {
		s.ticker.Stop()
	}
	close(s.stopCh)

	logger.Info("Heartbeat service stopped")
}

// runLoop 心跳循环
func (s *Service) runLoop() {
	// 首次启动后延迟一个周期再执行（给系统初始化时间）
	// 这样也避免了启动后立即触发心跳

	for {
		select {
		case <-s.ticker.C:
			s.tick()
		case <-s.stopCh:
			return
		}
	}
}

// tick 执行一次心跳
func (s *Service) tick() {
	s.mu.RLock()
	heartbeatDir := s.heartbeatDir
	onHeartbeat := s.onHeartbeat
	s.mu.RUnlock()

	// 检查是否有回调
	if onHeartbeat == nil {
		logger.Debug("Heartbeat: no callback registered, skipping")
		return
	}

	// 读取 HEARTBEAT.md 文件
	content := s.readHeartbeatFile(heartbeatDir)

	// 如果文件为空或不存在，跳过
	if isHeartbeatEmpty(content) {
		logger.Debug("Heartbeat: no tasks (HEARTBEAT.md empty or not found)")
		return
	}

	logger.Info("Heartbeat: checking for tasks...")

	// 生成任务 ID
	taskID := fmt.Sprintf("heartbeat-%d", time.Now().Unix())

	// 同步任务开始到看板
	s.syncTaskToBoard(taskID, taskSyncPkg.TaskEventStarted, taskSyncPkg.TaskStatusRunning, "", "")

	// 执行心跳回调
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	start := time.Now()
	response, err := onHeartbeat(ctx, HeartbeatPrompt)
	duration := time.Since(start)

	// 更新统计
	s.mu.Lock()
	s.lastRunAt = time.Now()
	s.runCount++
	if err != nil {
		s.lastStatus = "error"
		s.lastResponse = err.Error()
		logger.Error("Heartbeat failed", "duration", duration, "error", err)
		// 同步任务失败到看板
		s.syncTaskToBoard(taskID, taskSyncPkg.TaskEventFailed, taskSyncPkg.TaskStatusFailed, "", err.Error())
	} else {
		s.lastResponse = response
		// 检查是否包含 HEARTBEAT_OK
		if strings.Contains(strings.ToUpper(response), HeartbeatOKToken) {
			s.lastStatus = "ok"
			logger.Info("Heartbeat OK (no action needed)", "duration", duration)
			// 同步任务完成到看板（无任务需要处理）
			s.syncTaskToBoard(taskID, taskSyncPkg.TaskEventCompleted, taskSyncPkg.TaskStatusCompleted, "无任务需要处理", "")
		} else {
			s.lastStatus = "completed"
			logger.Info("Heartbeat completed task", "duration", duration)
			// 同步任务完成到看板
			s.syncTaskToBoard(taskID, taskSyncPkg.TaskEventCompleted, taskSyncPkg.TaskStatusCompleted, response, "")
		}
	}
	s.mu.Unlock()
}

// syncTaskToBoard 同步任务状态到看板
func (s *Service) syncTaskToBoard(taskID string, event taskSyncPkg.TaskEvent, status taskSyncPkg.TaskStatus, result, errMsg string) {
	if s.taskSyncer == nil {
		return
	}

	ctx := context.Background()
	syncEvent := &taskSyncPkg.TaskSyncEvent{
		Source:      taskSyncPkg.TaskSourceHeartbeat,
		Event:       event,
		ExternalID:  taskID,
		Title:       "[心跳] 定期检查任务",
		Description: "检查 HEARTBEAT.md 中的任务",
		Status:      status,
		Assignee:    taskSyncPkg.TaskAssigneeAI,
		Result:      result,
		Error:       errMsg,
		Tags:        []string{"心跳任务"},
	}

	if err := s.taskSyncer.Sync(ctx, syncEvent); err != nil {
		logger.Warn("Failed to sync heartbeat task to board", "error", err)
	}
}

// readHeartbeatFile 读取 HEARTBEAT.md 文件
func (s *Service) readHeartbeatFile(dir string) string {
	if dir == "" {
		// 默认使用 ~/.lingguard/workspace
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".lingguard", "workspace")
	}

	heartbeatPath := filepath.Join(dir, "HEARTBEAT.md")
	content, err := os.ReadFile(heartbeatPath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Debug("Heartbeat failed to read HEARTBEAT.md", "error", err)
		}
		return ""
	}

	return string(content)
}

// isHeartbeatEmpty 检查心跳内容是否为空
func isHeartbeatEmpty(content string) bool {
	// 去除空白字符后检查
	trimmed := strings.TrimSpace(content)
	return trimmed == ""
}

// Status 获取服务状态
func (s *Service) Status() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var nextRun string
	if s.running && s.ticker != nil {
		// ticker 没有直接暴露下次执行时间，使用最后运行时间 + 间隔估算
		if !s.lastRunAt.IsZero() {
			nextRun = s.lastRunAt.Add(s.config.Interval).Format("2006-01-02 15:04:05")
		}
	}

	return map[string]interface{}{
		"running":    s.running,
		"enabled":    s.config.Enabled,
		"interval":   s.config.Interval.String(),
		"lastRunAt":  s.lastRunAt.Format("2006-01-02 15:04:05"),
		"lastStatus": s.lastStatus,
		"runCount":   s.runCount,
		"nextRun":    nextRun,
		"workspace":  s.heartbeatDir,
	}
}

// Trigger 手动触发一次心跳（用于测试或立即执行）
func (s *Service) Trigger() {
	go s.tick()
}

// Running 检查服务是否在运行
func (s *Service) Running() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}
