// Package config 配置管理
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config 主配置结构
type Config struct {
	Providers map[string]ProviderConfig `json:"providers"`
	Agents    AgentsConfig              `json:"agents"`
	Channels  ChannelsConfig            `json:"channels"`
	Tools     ToolsConfig               `json:"tools"`
	Storage   StorageConfig             `json:"storage"`
	Logging   LoggingConfig             `json:"logging"`
}

// ProviderConfig 提供商配置
type ProviderConfig struct {
	APIKey      string  `json:"apiKey"`
	APIBase     string  `json:"apiBase,omitempty"`
	Model       string  `json:"model,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"maxTokens,omitempty"`
	GroupID     string  `json:"groupId,omitempty"`
}

// AgentsConfig 代理配置
type AgentsConfig struct {
	DefaultProvider    string            `json:"defaultProvider"`
	SystemPrompt       string            `json:"systemPrompt,omitempty"`
	MaxHistoryMessages int               `json:"maxHistoryMessages,omitempty"`
	MaxToolCalls       int               `json:"maxToolCalls,omitempty"`
	Mapping            map[string]string `json:"mapping,omitempty"`
}

// GetProviderForAgent 获取指定 agent 使用的 provider，未映射则返回默认
func (a *AgentsConfig) GetProviderForAgent(agentType string) string {
	if a.Mapping != nil {
		if provider, ok := a.Mapping[agentType]; ok {
			return provider
		}
	}
	return a.DefaultProvider
}

// ChannelsConfig 渠道配置
type ChannelsConfig struct {
	Feishu *FeishuConfig `json:"feishu,omitempty"`
}

// FeishuConfig 飞书配置
type FeishuConfig struct {
	Enabled           bool     `json:"enabled"`
	AppID             string   `json:"appId"`
	AppSecret         string   `json:"appSecret"`
	EncryptKey        string   `json:"encryptKey,omitempty"`
	VerificationToken string   `json:"verificationToken,omitempty"`
	AllowFrom         []string `json:"allowFrom,omitempty"`
}

// ToolsConfig 工具配置
type ToolsConfig struct {
	RestrictToWorkspace bool   `json:"restrictToWorkspace"`
	Workspace           string `json:"workspace,omitempty"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type        string `json:"type"`
	Host        string `json:"host,omitempty"`
	Port        int    `json:"port,omitempty"`
	Database    string `json:"database,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	SSLMode     string `json:"sslmode,omitempty"`
	VectorDbURL string `json:"vectorDbUrl,omitempty"`
	Path        string `json:"path,omitempty"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
	Output string `json:"output,omitempty"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Providers: make(map[string]ProviderConfig),
		Agents: AgentsConfig{
			DefaultProvider:    "gpt-4o",
			MaxHistoryMessages: 50,
			MaxToolCalls:       10,
		},
		Channels: ChannelsConfig{},
		Tools: ToolsConfig{
			RestrictToWorkspace: false,
			Workspace:           "~/.lingguard/workspace",
		},
		Storage: StorageConfig{
			Type: "sqlite",
			Path: "~/.lingguard/data.db",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// Load 加载配置
func Load(path string) (*Config, error) {
	expandedPath := expandPath(path)
	data, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save 保存配置
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	expandedPath := expandPath(path)
	dir := filepath.Dir(expandedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(expandedPath, data, 0644)
}

// expandPath 展开 ~ 为用户主目录
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
