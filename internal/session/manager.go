// Package session 会话管理
package session

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lingguard/pkg/memory"
)

// Session 会话
type Session struct {
	Key       string
	Messages  []*memory.Message
	CreatedAt time.Time
	UpdatedAt time.Time

	// processingMu 用于保证同一会话的消息串行处理
	// 防止并发消息导致会话历史出现连续的 user 消息
	processingMu sync.Mutex

	// isProcessing 标记是否正在处理消息
	isProcessing bool

	// lockedAt 锁定时间，用于超时检测
	lockedAt time.Time
}

// Manager 会话管理器
type Manager struct {
	mu       sync.RWMutex
	store    memory.Store
	sessions map[string]*Session
	window   int // 历史消息窗口大小
}

// NewManager 创建会话管理器
func NewManager(store memory.Store, window int) *Manager {
	return &Manager{
		store:    store,
		sessions: make(map[string]*Session),
		window:   window,
	}
}

// GetOrCreate 获取或创建会话
func (m *Manager) GetOrCreate(key string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[key]; ok {
		return s
	}

	s := &Session{
		Key:       key,
		Messages:  make([]*memory.Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.sessions[key] = s
	return s
}

// AddMessage 添加消息
func (s *Session) AddMessage(role, content string) {
	s.Messages = append(s.Messages, &memory.Message{
		ID:        generateID(),
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
	s.UpdatedAt = time.Now()
}

// AddMessageWithMedia 添加带媒体的消息
func (s *Session) AddMessageWithMedia(role, content string, media []string) {
	s.Messages = append(s.Messages, &memory.Message{
		ID:        generateID(),
		Role:      role,
		Content:   content,
		Media:     media,
		Timestamp: time.Now(),
	})
	s.UpdatedAt = time.Now()
}

// GetHistory 获取历史消息（限制窗口大小）
func (s *Session) GetHistory(window int) []*memory.Message {
	if window <= 0 || len(s.Messages) <= window {
		return s.Messages
	}
	return s.Messages[len(s.Messages)-window:]
}

// Clear 清空会话
func (s *Session) Clear() {
	s.Messages = make([]*memory.Message, 0)
	s.UpdatedAt = time.Now()
}

// TryLockForProcessing 尝试锁定会话以进行消息处理
// 返回 true 表示成功获取锁，false 表示会话正在处理其他消息
func (s *Session) TryLockForProcessing() bool {
	if s.processingMu.TryLock() {
		s.isProcessing = true
		s.lockedAt = time.Now()
		return true
	}
	return false
}

// TryLockWithTimeout 尝试锁定会话，如果锁被持有超过 timeout 则强制释放
// 用于处理长时间运行操作导致的会话阻塞
func (s *Session) TryLockWithTimeout(timeout time.Duration) bool {
	// 首先尝试正常获取锁
	if s.processingMu.TryLock() {
		s.isProcessing = true
		s.lockedAt = time.Now()
		return true
	}

	// 如果锁被持有，检查是否超时
	if s.lockedAt.IsZero() || time.Since(s.lockedAt) > timeout {
		// 超时了，尝试强制释放并重新获取
		// 注意：这是一个妥协方案，可能导致并发问题，但比完全阻塞好
		s.isProcessing = false
		s.processingMu.Unlock()
		if s.processingMu.TryLock() {
			s.isProcessing = true
			s.lockedAt = time.Now()
			return true
		}
	}

	return false
}

// LockForProcessing 锁定会话以进行消息处理（阻塞版本）
// 确保同一会话的消息串行处理，避免并发导致的历史消息错乱
func (s *Session) LockForProcessing() {
	s.processingMu.Lock()
	s.isProcessing = true
}

// UnlockAfterProcessing 释放会话处理锁
func (s *Session) UnlockAfterProcessing() {
	s.isProcessing = false
	s.lockedAt = time.Time{} // 清除锁定时间
	s.processingMu.Unlock()
}

// IsProcessing 检查会话是否正在处理消息
func (s *Session) IsProcessing() bool {
	return s.isProcessing
}

// generateID 生成唯一ID
func generateID() string {
	return uuid.New().String()[:8]
}
