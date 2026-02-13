// Package agent 核心代理逻辑
package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/providers"
	"github.com/lingguard/internal/tools"
	"github.com/lingguard/pkg/llm"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/memory"
)

// Agent 核心代理结构
type Agent struct {
	id           string
	provider     providers.Provider
	toolRegistry *tools.Registry
	memory       memory.Store
	config       *config.AgentsConfig
}

// NewAgent 创建新代理
func NewAgent(cfg *config.AgentsConfig, provider providers.Provider, mem memory.Store) *Agent {
	return &Agent{
		id:           generateID(),
		provider:     provider,
		toolRegistry: tools.NewRegistry(),
		memory:       mem,
		config:       cfg,
	}
}

// RegisterTool 注册工具
func (a *Agent) RegisterTool(t tools.Tool) {
	a.toolRegistry.Register(t)
}

// ProcessMessage 处理消息
func (a *Agent) ProcessMessage(ctx context.Context, sessionID, userMessage string) (string, error) {
	// 1. 存储用户消息
	userMsg := &memory.Message{
		ID:      generateID(),
		Role:    "user",
		Content: userMessage,
	}
	if err := a.memory.Add(ctx, sessionID, userMsg); err != nil {
		return "", fmt.Errorf("failed to store user message: %w", err)
	}

	// 2. 构建上下文
	messages, err := a.buildContext(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to build context: %w", err)
	}

	// 3. 执行代理循环
	return a.runLoop(ctx, sessionID, messages)
}

// buildContext 构建上下文
func (a *Agent) buildContext(ctx context.Context, sessionID string) ([]llm.Message, error) {
	messages := make([]llm.Message, 0)

	// 添加系统提示
	if a.config.SystemPrompt != "" {
		messages = append(messages, llm.Message{
			Role:    "system",
			Content: a.config.SystemPrompt,
		})
	}

	// 获取历史消息
	history, err := a.memory.Get(ctx, sessionID, a.config.MaxHistoryMessages)
	if err != nil {
		return nil, err
	}

	// 添加历史消息
	for _, msg := range history {
		messages = append(messages, llm.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return messages, nil
}

// runLoop 代理执行循环
func (a *Agent) runLoop(ctx context.Context, sessionID string, messages []llm.Message) (string, error) {
	iterations := 0
	maxIterations := a.config.MaxToolCalls
	if maxIterations == 0 {
		maxIterations = 10
	}

	for iterations < maxIterations {
		iterations++

		// 构建 LLM 请求
		req := &llm.Request{
			Model:    a.provider.Model(),
			Messages: messages,
			Tools:    a.toolRegistry.GetToolDefinitions(),
		}

		// 调用 LLM
		resp, err := a.provider.Complete(ctx, req)
		if err != nil {
			return "", fmt.Errorf("LLM call failed: %w", err)
		}

		// 获取响应消息
		assistantMsg := resp.ToMessage()

		// 存储助手消息
		if assistantMsg.Content != "" || len(assistantMsg.ToolCalls) > 0 {
			memMsg := &memory.Message{
				ID:      generateID(),
				Role:    "assistant",
				Content: assistantMsg.Content,
			}
			a.memory.Add(ctx, sessionID, memMsg)
		}

		// 检查是否有工具调用
		if !resp.HasToolCalls() {
			return resp.GetContent(), nil
		}

		// 添加助手消息到历史
		messages = append(messages, assistantMsg)

		// 执行工具调用
		for _, tc := range resp.GetToolCalls() {
			result, err := a.executeTool(ctx, &tc)

			var resultStr string
			if err != nil {
				resultStr = fmt.Sprintf("Error: %s", err)
			} else {
				resultStr = result
			}

			// 添加工具结果到消息
			toolMsg := llm.Message{
				Role:       "tool",
				Content:    resultStr,
				ToolCallID: tc.ID,
			}
			messages = append(messages, toolMsg)
		}
	}

	return "", fmt.Errorf("max iterations reached")
}

// executeTool 执行工具
func (a *Agent) executeTool(ctx context.Context, tc *llm.ToolCall) (string, error) {
	start := time.Now()

	tool, exists := a.toolRegistry.Get(tc.Function.Name)
	if !exists {
		return "", fmt.Errorf("unknown tool: %s", tc.Function.Name)
	}

	result, err := tool.Execute(ctx, tc.Function.Arguments)
	duration := time.Since(start)

	// 记录工具调用
	logger.ToolCall(tc.Function.Name, tc.Function.Arguments, result, duration, err)

	return result, err
}

func generateID() string {
	return uuid.New().String()[:8]
}
