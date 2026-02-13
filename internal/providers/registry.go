package providers

import (
	"fmt"
	"strings"
	"sync"

	"github.com/lingguard/internal/config"
)

// Registry 提供商注册表
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry 创建注册表
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register 注册提供商
func (r *Registry) Register(name string, p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = p
}

// Get 获取提供商
func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// List 列出所有提供商
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// InitFromConfig 从配置初始化提供商
func (r *Registry) InitFromConfig(cfg *config.Config) error {
	for name, pc := range cfg.Providers {
		if pc.APIKey == "" {
			continue
		}

		providerCfg := &ProviderConfig{
			APIKey:      pc.APIKey,
			APIBase:     pc.APIBase,
			Model:       pc.Model,
			Temperature: pc.Temperature,
			MaxTokens:   pc.MaxTokens,
		}

		// 根据 API 端点自动选择 Provider 类型
		var p Provider
		if isAnthropicEndpoint(pc.APIBase) {
			p = NewAnthropicProvider(name, providerCfg)
		} else {
			p = NewOpenAIProvider(name, providerCfg)
		}
		r.Register(name, p)
	}

	if len(r.providers) == 0 {
		return fmt.Errorf("no providers configured")
	}

	return nil
}

// isAnthropicEndpoint 检查是否为 Anthropic 兼容端点
func isAnthropicEndpoint(apiBase string) bool {
	if apiBase == "" {
		return false
	}
	lower := strings.ToLower(apiBase)
	return strings.Contains(lower, "/anthropic")
}
