package providers

import "strings"

// ProviderSpec 定义 Provider 的匹配规则
type ProviderSpec struct {
	Name         string   // 配置中的 provider 名称
	Keywords     []string // 模型名关键词（用于自动匹配）
	APIKeyPrefix string   // API Key 前缀
}

// BuiltinSpecs 内置 Provider 规范
var BuiltinSpecs = []ProviderSpec{
	{Name: "openai", Keywords: []string{"gpt", "o1", "o3"}},
	{Name: "anthropic", Keywords: []string{"claude"}},
	{Name: "deepseek", Keywords: []string{"deepseek"}},
	{Name: "qwen", Keywords: []string{"qwen", "tongyi", "dashscope"}},
	{Name: "glm", Keywords: []string{"glm", "chatglm", "codegeex"}},
	{Name: "minimax", Keywords: []string{"minimax"}},
	{Name: "moonshot", Keywords: []string{"moonshot", "kimi"}},
	{Name: "gemini", Keywords: []string{"gemini"}},
	{Name: "groq", Keywords: []string{"llama", "mixtral", "gemma"}, APIKeyPrefix: "gsk_"},
	{Name: "openrouter", Keywords: []string{"openrouter"}},
}

// FindSpecByModel 根据模型名查找 Provider 规范
func FindSpecByModel(model string) *ProviderSpec {
	modelLower := strings.ToLower(model)
	for i := range BuiltinSpecs {
		spec := &BuiltinSpecs[i]
		for _, kw := range spec.Keywords {
			if strings.Contains(modelLower, kw) {
				return spec
			}
		}
	}
	return nil
}
