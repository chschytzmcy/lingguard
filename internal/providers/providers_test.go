package providers

import (
	"testing"

	"github.com/lingguard/internal/config"
)

func TestProviderConfig(t *testing.T) {
	cfg := &ProviderConfig{
		APIKey:      "test-key",
		APIBase:     "https://api.test.com/v1",
		Model:       "test-model",
		Temperature: 0.7,
		MaxTokens:   1000,
	}

	if cfg.APIKey != "test-key" {
		t.Errorf("Expected APIKey=test-key, got %s", cfg.APIKey)
	}
}

func TestNewOpenAIProvider(t *testing.T) {
	cfg := &ProviderConfig{
		APIKey:  "test-key",
		APIBase: "https://api.test.com/v1",
	}

	provider := NewOpenAIProvider("test", cfg)

	if provider.Name() != "test" {
		t.Errorf("Expected name=test, got %s", provider.Name())
	}

	if !provider.SupportsTools() {
		t.Error("OpenAI provider should support tools")
	}

	if !provider.SupportsVision() {
		t.Error("OpenAI provider should support vision")
	}
}

func TestNewOpenAIProviderDefaultBase(t *testing.T) {
	cfg := &ProviderConfig{
		APIKey: "test-key",
		// 不设置 APIBase
	}

	provider := NewOpenAIProvider("openai", cfg)

	if provider.apiBase != "https://api.openai.com/v1" {
		t.Errorf("Expected default APIBase, got %s", provider.apiBase)
	}
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry()

	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}

	// 测试空注册表
	names := registry.List()
	if len(names) != 0 {
		t.Errorf("Expected empty registry, got %d providers", len(names))
	}

	// 注册 provider
	cfg := &ProviderConfig{APIKey: "test-key"}
	provider := NewOpenAIProvider("test", cfg)
	registry.Register("test", provider)

	// 获取 provider
	p, ok := registry.Get("test")
	if !ok {
		t.Error("Should find registered provider")
	}
	if p.Name() != "test" {
		t.Errorf("Expected provider name=test, got %s", p.Name())
	}

	// 列出所有
	names = registry.List()
	if len(names) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(names))
	}
}

func TestRegistryInitFromConfig(t *testing.T) {
	registry := NewRegistry()

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"provider1": {
				APIKey:  "key1",
				APIBase: "https://api1.com/v1",
			},
			"provider2": {
				APIKey:  "key2",
				APIBase: "https://api2.com/v1",
			},
			"empty": {
				APIKey: "", // 空 API Key
			},
		},
	}

	err := registry.InitFromConfig(cfg)
	if err != nil {
		t.Fatalf("InitFromConfig failed: %v", err)
	}

	// 应该注册了 2 个 provider（empty 被跳过）
	names := registry.List()
	if len(names) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(names))
	}
}

func TestRegistryInitFromConfigEmpty(t *testing.T) {
	registry := NewRegistry()

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{},
	}

	err := registry.InitFromConfig(cfg)
	if err == nil {
		t.Error("Expected error when no providers configured")
	}
}
