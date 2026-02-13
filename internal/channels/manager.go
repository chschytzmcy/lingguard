package channels

import (
	"context"
	"fmt"
	"sync"
)

// Manager 管理所有消息渠道
type Manager struct {
	mu       sync.RWMutex
	channels map[string]Channel
	handlers []MessageHandler
}

// NewManager 创建新的渠道管理器
func NewManager() *Manager {
	return &Manager{
		channels: make(map[string]Channel),
	}
}

// RegisterChannel 注册渠道
func (m *Manager) RegisterChannel(c Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[c.Name()] = c
}

// RegisterHandler 注册消息处理器
func (m *Manager) RegisterHandler(h MessageHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, h)
}

// StartAll 启动所有渠道
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for name, c := range m.channels {
		if err := c.Start(ctx); err != nil {
			return fmt.Errorf("failed to start channel %s: %w", name, err)
		}
	}
	return nil
}

// StopAll 停止所有渠道
func (m *Manager) StopAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var lastErr error
	for _, c := range m.channels {
		if err := c.Stop(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
