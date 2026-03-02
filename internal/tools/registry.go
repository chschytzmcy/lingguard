// Package tools 工具系统
package tools

import (
	"context"
	"encoding/json"
)

// Tool 工具接口
type Tool interface {
	// Name 工具名称
	Name() string

	// Description 工具描述
	Description() string

	// Parameters JSON Schema 格式的参数定义
	Parameters() map[string]interface{}

	// Execute 执行工具
	Execute(ctx context.Context, params json.RawMessage) (string, error)

	// IsDangerous 是否为危险操作
	IsDangerous() bool

	// ShouldLoadByDefault 是否默认加载到 LLM 提示词
	// 有对应 Skill 的工具返回 false，由 Skill 工具按需加载
	// 没有对应 Skill 的工具返回 true，默认加载
	ShouldLoadByDefault() bool
}

// ToolCall 工具调用
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult 工具执行结果
type ToolResult struct {
	CallID string `json:"callId"`
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// Definition 获取工具定义
func Definition(t Tool) map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        t.Name(),
			"description": t.Description(),
			"parameters":  t.Parameters(),
		},
	}
}
