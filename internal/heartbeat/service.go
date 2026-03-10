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

	"github.com/lingguard/pkg/logger"
)

// DefaultInterval 默认心跳间隔 (30分钟)
const DefaultInterval = 30 * time.Minute

// HeartbeatPromptTemplate 心跳提示模板
const HeartbeatPromptTemplate = `Read the file at %s (if it exists).
Follow any instructions or tasks listed there.

## 输出规则（重要！）

- 如果所有任务都正常（无会议、系统正常、无告警），只回复: HEARTBEAT_OK
- 只有当有需要通知用户的内容时，才输出具体信息（如即将开始的会议、系统告警、AI资讯等）
- 不要输出"我检查了..."、"一切正常"等无意义的确认信息
- 有效通知示例：会议提醒、磁盘/内存告警、每日资讯汇总`

// HeartbeatOKToken 无任务时的响应标识
const HeartbeatOKToken = "HEARTBEAT_OK"

// AgentCallback Agent 处理回调
type AgentCallback func(ctx context.Context, prompt string) (string, error)

// MessageSender 消息发送接口
type MessageSender interface {
	SendMessage(channelName string, to string, content string) error
}

// LastChannelGetter 获取最后使用渠道的接口
type LastChannelGetter interface {
	GetLastUsedChannel() (channel, chatID string)
}

// Config 心跳服务配置
type Config struct {
	Enabled        bool          `json:"enabled"`                  // 是否启用心跳
	Interval       time.Duration `json:"interval"`                 // 心跳间隔
	WorkspacePath  string        `json:"workspacePath,omitempty"`  // 工作空间路径，用于读取 HEARTBEAT.md
	Target         string        `json:"target,omitempty"`         // 发送目标: "last", "none", "feishu", "qq"
	To             string        `json:"to,omitempty"`             // 收件人 ID
	SilentStart    string        `json:"silentStart,omitempty"`    // 屏蔽期开始时间（如 "00:00"）
	SilentEnd      string        `json:"silentEnd,omitempty"`      // 屏蔽期结束时间（如 "06:00"）
	SilentTimezone string        `json:"silentTimezone,omitempty"` // 屏蔽期时区（如 "Asia/Shanghai"，默认本地时区）
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:  true,
		Interval: DefaultInterval,
		Target:   "last",
	}
}

// Service 心跳服务
type Service struct {
	config       *Config
	onHeartbeat  AgentCallback
	heartbeatDir string // HEARTBEAT.md 所在目录

	// 消息发送相关
	messageSender     MessageSender
	lastChannelGetter LastChannelGetter

	mu      sync.RWMutex
	running bool
	ticker  *time.Ticker
	stopCh  chan struct{}

	// 统计信息
	lastRunAt    time.Time
	lastStatus   string
	lastResponse string
	runCount     int

	// 重复去重
	lastHeartbeatText   string    // 上次发送的心跳内容
	lastHeartbeatSentAt time.Time // 上次发送心跳的时间
}

// NewService 创建心跳服务
func NewService(cfg *Config, onHeartbeat AgentCallback) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultInterval
	}

	return &Service{
		config:      cfg,
		onHeartbeat: onHeartbeat,
		stopCh:      make(chan struct{}, 1), // 带缓冲，避免阻塞
	}
}

// SetWorkspace 设置工作空间路径
func (s *Service) SetWorkspace(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.heartbeatDir = path
}

// SetMessageSender 设置消息发送器
func (s *Service) SetMessageSender(sender MessageSender) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messageSender = sender
}

// SetLastChannelGetter 设置最后渠道获取器
func (s *Service) SetLastChannelGetter(getter LastChannelGetter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastChannelGetter = getter
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

// UpdateConfig 热更新配置（无需重启服务）
// 如果 interval 改变，会重新创建 ticker
func (s *Service) UpdateConfig(cfg *Config) {
	if cfg == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	oldInterval := s.config.Interval
	s.config = cfg

	// 确保间隔有效
	if s.config.Interval <= 0 {
		s.config.Interval = DefaultInterval
	}

	// 如果服务正在运行且间隔改变，重新创建 ticker
	if s.running && s.ticker != nil && oldInterval != s.config.Interval {
		s.ticker.Stop()
		s.ticker = time.NewTicker(s.config.Interval)
		logger.Info("Heartbeat interval updated", "old", oldInterval, "new", s.config.Interval)
	}

	logger.Debug("Heartbeat config updated", "enabled", cfg.Enabled, "interval", cfg.Interval, "target", cfg.Target)
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
	target := s.config.Target
	to := s.config.To
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

	// 生成包含完整路径的 prompt
	heartbeatPath := filepath.Join(heartbeatDir, "HEARTBEAT.md")
	prompt := fmt.Sprintf(HeartbeatPromptTemplate, heartbeatPath)

	// 执行心跳回调
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	start := time.Now()
	response, err := onHeartbeat(ctx, prompt)
	duration := time.Since(start)

	// 更新统计
	s.mu.Lock()
	s.lastRunAt = time.Now()
	s.runCount++
	if err != nil {
		s.lastStatus = "error"
		s.lastResponse = err.Error()
		logger.Error("Heartbeat failed", "duration", duration, "error", err)
	} else {
		s.lastResponse = response
		// 检查是否包含 HEARTBEAT_OK
		if strings.Contains(strings.ToUpper(response), HeartbeatOKToken) {
			s.lastStatus = "ok"
			logger.Info("Heartbeat OK (no action needed)", "duration", duration)
		} else {
			s.lastStatus = "completed"
			logger.Info("Heartbeat completed task", "duration", duration)
		}
	}
	s.mu.Unlock()

	// 发送通知（如果有需要通知的内容）
	if err == nil && target != "none" {
		s.sendNotification(response, target, to)
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

// isInSilentPeriod 检查当前时间是否在屏蔽期内
// silentStart 和 silentEnd 格式为 "HH:MM"（如 "00:00" 到 "06:00"）
// 支持跨午夜的情况（如 "23:00" 到 "06:00"）
// timezone 为时区名称（如 "Asia/Shanghai"），为空则使用本地时区
func isInSilentPeriod(silentStart, silentEnd, timezone string) bool {
	if silentStart == "" || silentEnd == "" {
		return false
	}

	// 解析时区
	var loc *time.Location
	if timezone != "" {
		var err error
		loc, err = time.LoadLocation(timezone)
		if err != nil {
			logger.Debug("Heartbeat: invalid timezone, using local", "timezone", timezone, "error", err)
			loc = time.Local
		}
	} else {
		loc = time.Local
	}

	// 获取指定时区的当前时间
	now := time.Now().In(loc)
	currentMinutes := now.Hour()*60 + now.Minute()

	startMinutes, ok := parseTimeMinutes(silentStart)
	if !ok {
		return false
	}

	endMinutes, ok := parseTimeMinutes(silentEnd)
	if !ok {
		return false
	}

	// 如果开始时间和结束时间相同，说明全天都是屏蔽期
	if startMinutes == endMinutes {
		return false // 00:00-00:00 表示不屏蔽
	}

	// 如果结束时间小于开始时间，说明跨午夜（如 23:00 - 06:00）
	if endMinutes < startMinutes {
		// 当前时间在开始之后或在结束之前
		return currentMinutes >= startMinutes || currentMinutes < endMinutes
	}

	// 正常情况（如 00:00 - 06:00）
	return currentMinutes >= startMinutes && currentMinutes < endMinutes
}

// parseTimeMinutes 解析 "HH:MM" 格式的时间为分钟数
func parseTimeMinutes(timeStr string) (int, bool) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0, false
	}

	hours := 0
	minutes := 0
	if _, err := fmt.Sscanf(parts[0], "%d", &hours); err != nil {
		return 0, false
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &minutes); err != nil {
		return 0, false
	}

	if hours < 0 || hours > 23 || minutes < 0 || minutes > 59 {
		return 0, false
	}

	return hours*60 + minutes, true
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

// sendNotification 发送心跳通知
func (s *Service) sendNotification(response, target, to string) {
	// 检查是否在屏蔽期内
	s.mu.RLock()
	silentStart := s.config.SilentStart
	silentEnd := s.config.SilentEnd
	silentTimezone := s.config.SilentTimezone
	s.mu.RUnlock()

	if isInSilentPeriod(silentStart, silentEnd, silentTimezone) {
		logger.Debug("Heartbeat: in silent period, skipping notification", "start", silentStart, "end", silentEnd, "timezone", silentTimezone)
		return
	}

	// 检查响应是否只有 HEARTBEAT_OK（无需通知）
	trimmed := strings.TrimSpace(response)
	upperResponse := strings.ToUpper(trimmed)

	// 如果响应只是 HEARTBEAT_OK，不发送通知
	if upperResponse == HeartbeatOKToken {
		logger.Debug("Heartbeat: no notification needed (HEARTBEAT_OK)")
		return
	}

	// 去除 HEARTBEAT_OK 标记后的内容
	if strings.HasPrefix(upperResponse, HeartbeatOKToken) {
		trimmed = strings.TrimSpace(trimmed[len(HeartbeatOKToken):])
	} else if strings.HasSuffix(upperResponse, HeartbeatOKToken) {
		trimmed = strings.TrimSpace(trimmed[:len(trimmed)-len(HeartbeatOKToken)])
	}
	upperResponse = strings.ToUpper(trimmed)

	// 如果内容为空，不需要通知
	if trimmed == "" {
		logger.Debug("Heartbeat: no notification needed (empty content)")
		return
	}

	// 过滤掉无意义的"正常"响应（说明没有实际需要通知的内容）
	// 这些模式表示系统正常但 Agent 输出了确认信息
	noisePatterns := []string{
		"一切正常",
		"没有会议",
		"无会议",
		"本周没有",
		"系统正常",
		"磁盘空间充足",
		"内存充足",
		"没有需要",
		"无需通知",
		"检查完成",
		"已检查",
		"正常",
	}

	// 如果响应只包含这些噪音模式，不发送通知
	isNoise := false
	for _, pattern := range noisePatterns {
		if strings.Contains(trimmed, pattern) {
			// 检查是否只有噪音内容（排除掉包含实际数据的情况）
			// 如果响应很短且只包含噪音模式，则认为是噪音
			if len(trimmed) < 100 {
				isNoise = true
				break
			}
		}
	}

	// 检查是否包含有效的通知内容
	// 有效内容特征：包含数字、时间、链接、或特定格式
	hasValidContent := false
	validIndicators := []string{
		"http://", "https://", // 链接
		"📅", "📌", "⚠️", "🚨", "🔔", // 表情符号（表示有结构化内容）
		"会议", "告警", "告警", "资讯", // 关键词
		"使用率", "已用", "剩余", // 系统监控
		"合并", "删除", "提炼", // 记忆操作
	}

	for _, indicator := range validIndicators {
		if strings.Contains(trimmed, indicator) {
			hasValidContent = true
			break
		}
	}

	// 如果响应很短，没有有效内容，且包含噪音模式，则不发送通知
	if isNoise && !hasValidContent {
		logger.Debug("Heartbeat: no notification needed (noise response)", "content", trimmed)
		return
	}

	// 解析目标渠道和收件人
	var channel, chatID string
	switch target {
	case "last":
		// 使用最后使用的渠道
		if s.lastChannelGetter != nil {
			channel, chatID = s.lastChannelGetter.GetLastUsedChannel()
		}
		if to != "" {
			chatID = to // to 字段可以覆盖
		}
	case "none":
		return
	default:
		// 指定渠道名（如 "feishu", "qq"）
		channel = target
		chatID = to
	}

	if channel == "" || chatID == "" {
		logger.Debug("Heartbeat: no delivery target, skipping notification", "target", target, "to", to)
		return
	}

	// 重复去重检查：24小时内相同内容不重复发送
	s.mu.Lock()
	now := time.Now()
	dedupeWindow := 24 * time.Hour
	if s.lastHeartbeatText != "" &&
		trimmed == s.lastHeartbeatText &&
		!s.lastHeartbeatSentAt.IsZero() &&
		now.Sub(s.lastHeartbeatSentAt) < dedupeWindow {
		s.mu.Unlock()
		logger.Debug("Heartbeat: skipping duplicate notification within 24h", "content", trimmed[:min(50, len(trimmed))])
		return
	}
	s.mu.Unlock()

	// 发送消息
	if s.messageSender != nil {
		if err := s.messageSender.SendMessage(channel, chatID, trimmed); err != nil {
			logger.Error("Heartbeat failed to send notification", "channel", channel, "error", err)
		} else {
			// 记录本次发送的内容和时间（用于去重）
			s.mu.Lock()
			s.lastHeartbeatText = trimmed
			s.lastHeartbeatSentAt = now
			s.mu.Unlock()
			logger.Info("Heartbeat notification sent", "channel", channel, "chatID", chatID)
		}
	}
}
