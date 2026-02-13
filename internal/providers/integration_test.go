//go:build integration
// +build integration

package providers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lingguard/pkg/llm"
)

// 测试 Qwen3 (通义千问 - DashScope) - OpenAI 兼容
func TestQwenProvider(t *testing.T) {
	cfg := &ProviderConfig{
		APIKey:  "sk-791d6c7cc3094af99290577b709b47e6",
		APIBase: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		Model:   "qwen-max",
	}

	provider := NewOpenAIProvider("qwen", cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &llm.Request{
		Model: cfg.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "你好，请用一句话介绍你自己"},
		},
		MaxTokens: 100,
	}

	resp, err := provider.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Qwen API call failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("No choices in response")
	}

	fmt.Printf("Qwen Response: %s\n", resp.Choices[0].Message.Content)
}

// 测试工具调用 - Qwen
func TestQwenWithTools(t *testing.T) {
	cfg := &ProviderConfig{
		APIKey:  "sk-791d6c7cc3094af99290577b709b47e6",
		APIBase: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		Model:   "qwen-max",
	}

	provider := NewOpenAIProvider("qwen", cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &llm.Request{
		Model: cfg.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "帮我执行 echo hello 命令"},
		},
		Tools: []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "shell",
					"description": "Execute shell commands",
					"parameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"command": map[string]interface{}{
								"type":        "string",
								"description": "The shell command to execute",
							},
						},
						"required": []string{"command"},
					},
				},
			},
		},
		MaxTokens: 500,
	}

	resp, err := provider.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Qwen API call failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("No choices in response")
	}

	if len(resp.Choices[0].Message.ToolCalls) > 0 {
		fmt.Printf("Qwen Tool Call: %s\n", resp.Choices[0].Message.ToolCalls[0].Function.Name)
		fmt.Printf("Arguments: %s\n", resp.Choices[0].Message.ToolCalls[0].Function.Arguments)
	} else {
		fmt.Printf("Qwen Response (no tool call): %s\n", resp.Choices[0].Message.Content)
	}
}

// 测试 GLM (智谱) - Anthropic 兼容编程接口
func TestGLMAnthropicProvider(t *testing.T) {
	cfg := &ProviderConfig{
		APIKey:  "bf780a4d27454909ad02ba7c9a0e6d19.LsBBLU4U3bCCdvqm",
		APIBase: "https://open.bigmodel.cn/api/anthropic",
		Model:   "glm-5",
	}

	provider := NewAnthropicProvider("glm", cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &llm.Request{
		Model: cfg.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "你好，请用一句话介绍你自己"},
		},
		MaxTokens: 100,
	}

	resp, err := provider.Complete(ctx, req)
	if err != nil {
		t.Fatalf("GLM Anthropic API call failed: %v", err)
	}

	content := resp.GetContent()
	if content == "" && len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	if content == "" {
		t.Fatal("No content in response")
	}

	fmt.Printf("GLM (Anthropic) Response: %s\n", content)
}

// 测试工具调用 - GLM (Anthropic 接口)
func TestGLMAnthropicWithTools(t *testing.T) {
	cfg := &ProviderConfig{
		APIKey:  "bf780a4d27454909ad02ba7c9a0e6d19.LsBBLU4U3bCCdvqm",
		APIBase: "https://open.bigmodel.cn/api/anthropic",
		Model:   "glm-5",
	}

	provider := NewAnthropicProvider("glm", cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &llm.Request{
		Model: cfg.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "帮我执行 echo hello 命令"},
		},
		Tools: []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "shell",
					"description": "Execute shell commands",
					"parameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"command": map[string]interface{}{
								"type":        "string",
								"description": "The shell command to execute",
							},
						},
						"required": []string{"command"},
					},
				},
			},
		},
		MaxTokens: 500,
	}

	resp, err := provider.Complete(ctx, req)
	if err != nil {
		t.Fatalf("GLM Anthropic API call failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("No choices in response")
	}

	if len(resp.Choices[0].Message.ToolCalls) > 0 {
		fmt.Printf("GLM (Anthropic) Tool Call: %s\n", resp.Choices[0].Message.ToolCalls[0].Function.Name)
		fmt.Printf("Arguments: %s\n", resp.Choices[0].Message.ToolCalls[0].Function.Arguments)
	} else {
		fmt.Printf("GLM (Anthropic) Response (no tool call): %s\n", resp.Choices[0].Message.Content)
	}
}

// 测试代码生成 - GLM (Anthropic 接口)
func TestGLMAnthropicCodeGeneration(t *testing.T) {
	cfg := &ProviderConfig{
		APIKey:  "bf780a4d27454909ad02ba7c9a0e6d19.LsBBLU4U3bCCdvqm",
		APIBase: "https://open.bigmodel.cn/api/anthropic",
		Model:   "glm-5",
	}

	provider := NewAnthropicProvider("glm", cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &llm.Request{
		Model: cfg.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "写一个 Go 语言的 Hello World 程序"},
		},
		MaxTokens: 1000,
	}

	resp, err := provider.Complete(ctx, req)
	if err != nil {
		t.Fatalf("GLM Anthropic Code API call failed: %v", err)
	}

	content := resp.GetContent()
	if content == "" && len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	if content == "" {
		t.Fatal("No content in response")
	}

	fmt.Printf("GLM (Anthropic) Code Response:\n%s\n", content)
}

// 测试 MiniMax - Anthropic 兼容编程接口
func TestMiniMaxAnthropicProvider(t *testing.T) {
	cfg := &ProviderConfig{
		APIKey:  "sk-cp-mpX09dQEHgHHOva2vEZ89_c-RUNXA4CrmTFViEMUD-f00iTziDBdEtPvEIqHqD1o_LHQFtfMp-EYIf25nYv-wBM34G0XIw1OeSMev1QVUjOR7__GI6lQXv4",
		APIBase: "https://api.minimaxi.com/anthropic",
		Model:   "MiniMax-M2.5",
	}

	provider := NewAnthropicProvider("minimax", cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &llm.Request{
		Model: cfg.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "你好，请用一句话介绍你自己"},
		},
		MaxTokens: 100,
	}

	resp, err := provider.Complete(ctx, req)
	if err != nil {
		t.Fatalf("MiniMax Anthropic API call failed: %v", err)
	}

	content := resp.GetContent()
	if content == "" && len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	if content == "" {
		t.Fatal("No content in response")
	}

	fmt.Printf("MiniMax (Anthropic) Response: %s\n", content)
}

// 测试工具调用 - MiniMax (Anthropic 接口)
func TestMiniMaxAnthropicWithTools(t *testing.T) {
	cfg := &ProviderConfig{
		APIKey:  "sk-cp-mpX09dQEHgHHOva2vEZ89_c-RUNXA4CrmTFViEMUD-f00iTziDBdEtPvEIqHqD1o_LHQFtfMp-EYIf25nYv-wBM34G0XIw1OeSMev1QVUjOR7__GI6lQXv4",
		APIBase: "https://api.minimaxi.com/anthropic",
		Model:   "MiniMax-M2.5",
	}

	provider := NewAnthropicProvider("minimax", cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &llm.Request{
		Model: cfg.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "帮我执行 echo hello 命令"},
		},
		Tools: []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "shell",
					"description": "Execute shell commands",
					"parameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"command": map[string]interface{}{
								"type":        "string",
								"description": "The shell command to execute",
							},
						},
						"required": []string{"command"},
					},
				},
			},
		},
		MaxTokens: 500,
	}

	resp, err := provider.Complete(ctx, req)
	if err != nil {
		t.Fatalf("MiniMax Anthropic API call failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("No choices in response")
	}

	if len(resp.Choices[0].Message.ToolCalls) > 0 {
		fmt.Printf("MiniMax (Anthropic) Tool Call: %s\n", resp.Choices[0].Message.ToolCalls[0].Function.Name)
		fmt.Printf("Arguments: %s\n", resp.Choices[0].Message.ToolCalls[0].Function.Arguments)
	} else {
		fmt.Printf("MiniMax (Anthropic) Response (no tool call): %s\n", resp.Choices[0].Message.Content)
	}
}

// 测试代码生成 - MiniMax (Anthropic 接口)
func TestMiniMaxAnthropicCodeGeneration(t *testing.T) {
	cfg := &ProviderConfig{
		APIKey:  "sk-cp-mpX09dQEHgHHOva2vEZ89_c-RUNXA4CrmTFViEMUD-f00iTziDBdEtPvEIqHqD1o_LHQFtfMp-EYIf25nYv-wBM34G0XIw1OeSMev1QVUjOR7__GI6lQXv4",
		APIBase: "https://api.minimaxi.com/anthropic",
		Model:   "MiniMax-M2.5",
	}

	provider := NewAnthropicProvider("minimax", cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &llm.Request{
		Model: cfg.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "写一个 Go 语言的 Hello World 程序"},
		},
		MaxTokens: 1000,
	}

	resp, err := provider.Complete(ctx, req)
	if err != nil {
		t.Fatalf("MiniMax Anthropic Code API call failed: %v", err)
	}

	content := resp.GetContent()
	if content == "" && len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	if content == "" {
		t.Fatal("No content in response")
	}

	fmt.Printf("MiniMax (Anthropic) Code Response:\n%s\n", content)
}

// ============ OpenAI 兼容接口测试 (备用) ============

// 测试 GLM (智谱) - OpenAI 兼容接口
func TestGLMOpenAIProvider(t *testing.T) {
	cfg := &ProviderConfig{
		APIKey:  "bf780a4d27454909ad02ba7c9a0e6d19.LsBBLU4U3bCCdvqm",
		APIBase: "https://open.bigmodel.cn/api/paas/v4",
		Model:   "glm-4-flash",
	}

	provider := NewOpenAIProvider("glm", cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &llm.Request{
		Model: cfg.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "你好，请用一句话介绍你自己"},
		},
		MaxTokens: 100,
	}

	resp, err := provider.Complete(ctx, req)
	if err != nil {
		t.Fatalf("GLM OpenAI API call failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("No choices in response")
	}

	fmt.Printf("GLM (OpenAI) Response: %s\n", resp.Choices[0].Message.Content)
}

// 测试 MiniMax - OpenAI 兼容接口
func TestMiniMaxOpenAIProvider(t *testing.T) {
	cfg := &ProviderConfig{
		APIKey:  "sk-cp-mpX09dQEHgHHOva2vEZ89_c-RUNXA4CrmTFViEMUD-f00iTziDBdEtPvEIqHqD1o_LHQFtfMp-EYIf25nYv-wBM34G0XIw1OeSMev1QVUjOR7__GI6lQXv4",
		APIBase: "https://api.minimax.chat/v1",
		Model:   "MiniMax-Text-01",
	}

	provider := NewOpenAIProvider("minimax", cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &llm.Request{
		Model: cfg.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "你好，请用一句话介绍你自己"},
		},
		MaxTokens: 100,
	}

	resp, err := provider.Complete(ctx, req)
	if err != nil {
		t.Fatalf("MiniMax OpenAI API call failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("No choices in response")
	}

	fmt.Printf("MiniMax (OpenAI) Response: %s\n", resp.Choices[0].Message.Content)
}
