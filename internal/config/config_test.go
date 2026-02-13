package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if cfg.Agents.MaxHistoryMessages != 50 {
		t.Errorf("Expected MaxHistoryMessages=50, got %d", cfg.Agents.MaxHistoryMessages)
	}

	if cfg.Agents.MaxToolCalls != 10 {
		t.Errorf("Expected MaxToolCalls=10, got %d", cfg.Agents.MaxToolCalls)
	}

	if cfg.Storage.Type != "sqlite" {
		t.Errorf("Expected Storage.Type=sqlite, got %s", cfg.Storage.Type)
	}

	if cfg.Logging.Level != "info" {
		t.Errorf("Expected Logging.Level=info, got %s", cfg.Logging.Level)
	}
}

func TestConfigSaveAndLoad(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "lingguard-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	// 创建测试配置
	cfg := DefaultConfig()
	cfg.Providers["test"] = ProviderConfig{
		APIKey:  "test-key",
		APIBase: "https://api.test.com/v1",
		Model:   "test-model",
	}
	cfg.Agents.DefaultProvider = "test"
	cfg.Agents.SystemPrompt = "Test prompt"
	cfg.Agents.Mapping = map[string]string{
		"dev_agent": "test",
	}

	// 保存配置
	err = cfg.Save(configPath)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// 加载配置
	loadedCfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// 验证加载的配置
	if loadedCfg.Providers["test"].APIKey != "test-key" {
		t.Errorf("Expected APIKey=test-key, got %s", loadedCfg.Providers["test"].APIKey)
	}

	if loadedCfg.Agents.DefaultProvider != "test" {
		t.Errorf("Expected DefaultProvider=test, got %s", loadedCfg.Agents.DefaultProvider)
	}

	if loadedCfg.Agents.SystemPrompt != "Test prompt" {
		t.Errorf("Expected SystemPrompt='Test prompt', got %s", loadedCfg.Agents.SystemPrompt)
	}

	if loadedCfg.Agents.Mapping["dev_agent"] != "test" {
		t.Errorf("Expected dev_agent mapping=test, got %s", loadedCfg.Agents.Mapping["dev_agent"])
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		input   string
		hasHome bool
	}{
		{"~/test", true},
		{"/absolute/path", false},
		{"relative/path", false},
	}

	for _, tt := range tests {
		result := expandPath(tt.input)
		if tt.hasHome && result[0] != '/' {
			t.Errorf("expandPath(%s) should expand ~ to home directory", tt.input)
		}
		if !tt.hasHome && result == tt.input {
			// 路径不变是正常的
		}
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	_, err := Load("/non/existent/path/config.json")
	if err == nil {
		t.Error("Expected error when loading non-existent file")
	}
}

func TestFeishuConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Channels.Feishu = &FeishuConfig{
		Enabled:   true,
		AppID:     "cli_test",
		AppSecret: "secret",
		AllowFrom: []string{"user1", "user2"},
	}

	if !cfg.Channels.Feishu.Enabled {
		t.Error("Feishu should be enabled")
	}

	if len(cfg.Channels.Feishu.AllowFrom) != 2 {
		t.Errorf("Expected 2 allowed users, got %d", len(cfg.Channels.Feishu.AllowFrom))
	}
}
