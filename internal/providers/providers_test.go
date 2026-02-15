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

	// 注册 provider（使用新的签名，包含 spec）
	cfg := &ProviderConfig{APIKey: "test-key"}
	provider := NewOpenAIProvider("test", cfg)
	spec := &ProviderSpec{Name: "test", DisplayName: "Test Provider"}
	registry.Register("test", provider, spec)

	// 获取 provider
	p, ok := registry.Get("test")
	if !ok {
		t.Error("Should find registered provider")
	}
	if p.Name() != "test" {
		t.Errorf("Expected provider name=test, got %s", p.Name())
	}

	// 获取 spec
	s := registry.GetSpec("test")
	if s == nil {
		t.Error("Should find registered spec")
	}
	if s.DisplayName != "Test Provider" {
		t.Errorf("Expected DisplayName=Test Provider, got %s", s.DisplayName)
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

func TestFindSpecByModel(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"gpt-4o", "openai"},
		{"gpt-3.5-turbo", "openai"},
		{"claude-3-opus", "anthropic"},
		{"deepseek-chat", "deepseek"},
		{"qwen-max", "qwen"},
		{"glm-4", "glm"},
		{"moonshot-v1", "moonshot"},
		{"gemini-1.5-pro", "gemini"},
		{"llama-3.1-70b", "groq"},
		{"unknown-model", ""}, // 无匹配
	}

	for _, tt := range tests {
		spec := FindSpecByModel(tt.model)
		if tt.expected == "" {
			if spec != nil {
				t.Errorf("FindSpecByModel(%s): expected nil, got %s", tt.model, spec.Name)
			}
		} else {
			if spec == nil {
				t.Errorf("FindSpecByModel(%s): expected %s, got nil", tt.model, tt.expected)
			} else if spec.Name != tt.expected {
				t.Errorf("FindSpecByModel(%s): expected %s, got %s", tt.model, tt.expected, spec.Name)
			}
		}
	}
}

func TestFindSpecByName(t *testing.T) {
	spec := FindSpecByName("openai")
	if spec == nil {
		t.Fatal("FindSpecByName(openai) returned nil")
	}
	if spec.DisplayName != "OpenAI" {
		t.Errorf("Expected DisplayName=OpenAI, got %s", spec.DisplayName)
	}
	if spec.DefaultAPIBase != "https://api.openai.com/v1" {
		t.Errorf("Expected DefaultAPIBase, got %s", spec.DefaultAPIBase)
	}

	// 测试不存在的 provider
	spec = FindSpecByName("nonexistent")
	if spec != nil {
		t.Errorf("FindSpecByName(nonexistent) should return nil")
	}
}

func TestFindSpecByAPIKey(t *testing.T) {
	tests := []struct {
		apiKey   string
		expected string
	}{
		{"sk-or-v1-xxx", "openrouter"},
		{"gsk_xxx", "groq"},
		{"sk-ant-xxx", "anthropic"},
		{"sk-xxx", "openai"},
		{"unknown-key", ""}, // 无匹配
	}

	for _, tt := range tests {
		spec := FindSpecByAPIKey(tt.apiKey)
		if tt.expected == "" {
			if spec != nil {
				t.Errorf("FindSpecByAPIKey(%s): expected nil, got %s", tt.apiKey, spec.Name)
			}
		} else {
			if spec == nil {
				t.Errorf("FindSpecByAPIKey(%s): expected %s, got nil", tt.apiKey, tt.expected)
			} else if spec.Name != tt.expected {
				t.Errorf("FindSpecByAPIKey(%s): expected %s, got %s", tt.apiKey, tt.expected, spec.Name)
			}
		}
	}
}

func TestMatchProvider(t *testing.T) {
	registry := NewRegistry()

	// 注册几个 provider
	cfg1 := &ProviderConfig{APIKey: "key1", Model: "gpt-4o"}
	cfg2 := &ProviderConfig{APIKey: "key2", Model: "claude-3-opus"}
	cfg3 := &ProviderConfig{APIKey: "key3", Model: "deepseek-chat"}

	registry.Register("openai", NewOpenAIProvider("openai", cfg1), FindSpecByName("openai"))
	registry.Register("anthropic", NewAnthropicProvider("anthropic", cfg2), FindSpecByName("anthropic"))
	registry.Register("deepseek", NewOpenAIProvider("deepseek", cfg3), FindSpecByName("deepseek"))

	registry.SetDefault("openai")

	// 测试 1: "provider/model" 格式
	p, spec := registry.MatchProvider("anthropic/claude-3-opus")
	if p == nil {
		t.Error("MatchProvider(anthropic/claude-3-opus) returned nil provider")
	} else if p.Name() != "anthropic" {
		t.Errorf("Expected provider=anthropic, got %s", p.Name())
	}
	if spec == nil || spec.Name != "anthropic" {
		t.Errorf("Expected spec name=anthropic, got %v", spec)
	}

	// 测试 2: 通过模型关键词匹配
	p, spec = registry.MatchProvider("gpt-4-turbo")
	if p == nil {
		t.Error("MatchProvider(gpt-4-turbo) returned nil provider")
	} else if p.Name() != "openai" {
		t.Errorf("Expected provider=openai (by keyword), got %s", p.Name())
	}

	// 测试 3: 返回默认 provider
	p, _ = registry.MatchProvider("unknown-model")
	if p == nil {
		t.Error("MatchProvider(unknown-model) should return default provider")
	} else if p.Name() != "openai" {
		t.Errorf("Expected default provider=openai, got %s", p.Name())
	}
}

func TestProviderSpecNormalizeModel(t *testing.T) {
	// 测试带前缀的 provider (qwen)
	spec := FindSpecByName("qwen")
	if spec == nil {
		t.Fatal("qwen spec not found")
	}

	// 测试添加前缀
	normalized := spec.NormalizeModel("qwen-max")
	if normalized != "dashscope/qwen-max" {
		t.Errorf("Expected dashscope/qwen-max, got %s", normalized)
	}

	// 测试已有前缀时不再添加
	normalized = spec.NormalizeModel("dashscope/qwen-max")
	if normalized != "dashscope/qwen-max" {
		t.Errorf("Expected dashscope/qwen-max (no double prefix), got %s", normalized)
	}

	// 测试不带前缀的 provider (openai)
	spec = FindSpecByName("openai")
	if spec == nil {
		t.Fatal("openai spec not found")
	}
	normalized = spec.NormalizeModel("gpt-4o")
	if normalized != "gpt-4o" {
		t.Errorf("Expected gpt-4o (no prefix), got %s", normalized)
	}
}

func TestListWithSpecs(t *testing.T) {
	registry := NewRegistry()

	cfg := &ProviderConfig{APIKey: "test-key", Model: "gpt-4o"}
	registry.Register("openai", NewOpenAIProvider("openai", cfg), FindSpecByName("openai"))

	infos := registry.ListWithSpecs()
	if len(infos) != 1 {
		t.Fatalf("Expected 1 provider info, got %d", len(infos))
	}

	if infos[0].Name != "openai" {
		t.Errorf("Expected name=openai, got %s", infos[0].Name)
	}
	if infos[0].DisplayName != "OpenAI" {
		t.Errorf("Expected DisplayName=OpenAI, got %s", infos[0].DisplayName)
	}
	if infos[0].Model != "gpt-4o" {
		t.Errorf("Expected Model=gpt-4o, got %s", infos[0].Model)
	}
}
