package subagent

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/lingguard/internal/providers"
	"github.com/lingguard/internal/tools"
	"github.com/lingguard/pkg/llm"
	"github.com/lingguard/pkg/logger"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
)

// ToolCallRecord 工具调用记录
type ToolCallRecord struct {
	Tool      string        `json:"tool"`
	Arguments string        `json:"arguments,omitempty"`
	Result    string        `json:"result,omitempty"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}

// Subagent 子代理
type Subagent struct {
	// 基本信息
	id        string
	task      string
	context   string
	sessionID string

	// 状态
	status      TaskStatus
	result      string
	error       string
	startedAt   time.Time
	completedAt time.Time

	// 执行历史
	toolCalls []ToolCallRecord

	// 执行组件
	provider     providers.Provider
	toolRegistry *tools.Registry
	config       *SubagentConfig

	// 并发控制
	mu sync.RWMutex
}

// NewSubagent 创建子代理
func NewSubagent(task, context string, provider providers.Provider, toolRegistry *tools.Registry, config *SubagentConfig) *Subagent {
	if config == nil {
		config = DefaultSubagentConfig()
	}

	return &Subagent{
		id:           generateTaskID(),
		task:         task,
		context:      context,
		status:       StatusPending,
		provider:     provider,
		toolRegistry: toolRegistry,
		config:       config,
	}
}

// ID 返回任务 ID
func (s *Subagent) ID() string {
	return s.id
}

// Task 返回任务描述
func (s *Subagent) Task() string {
	return s.task
}

// Context 返回任务上下文
func (s *Subagent) Context() string {
	return s.context
}

// Status 返回当前状态
func (s *Subagent) Status() TaskStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// Result 返回执行结果
func (s *Subagent) Result() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.result
}

// Error 返回错误信息
func (s *Subagent) Error() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.error
}

// StartedAt 返回开始时间
func (s *Subagent) StartedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.startedAt
}

// CompletedAt 返回完成时间
func (s *Subagent) CompletedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.completedAt
}

// setStatus 设置状态
func (s *Subagent) setStatus(status TaskStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

// setResult 设置结果
func (s *Subagent) setResult(result string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.result = result
}

// setError 设置错误
func (s *Subagent) setError(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.error = err
}

// addToolCall 添加工具调用记录
func (s *Subagent) addToolCall(record ToolCallRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolCalls = append(s.toolCalls, record)
}

// GetToolCalls 返回工具调用历史
func (s *Subagent) GetToolCalls() []ToolCallRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.toolCalls
}

// Run 执行子代理任务（阻塞）
func (s *Subagent) Run(ctx context.Context) {
	s.setStatus(StatusRunning)
	s.mu.Lock()
	s.startedAt = time.Now()
	s.mu.Unlock()

	// 记录任务开始
	logger.Info("Subagent task started", "id", s.id, "task", s.task)

	defer func() {
		s.mu.Lock()
		s.completedAt = time.Now()
		s.mu.Unlock()

		// 记录任务完成
		if s.status == StatusCompleted {
			logger.Info("Subagent task completed", "id", s.id, "duration", s.completedAt.Sub(s.startedAt).String(), "resultLen", len(s.result))
		} else {
			logger.Warn("Subagent task failed", "id", s.id, "error", s.error)
		}
	}()

	// 构建消息
	messages, err := s.buildMessages()
	if err != nil {
		s.setStatus(StatusFailed)
		s.setError(fmt.Sprintf("failed to build messages: %s", err))
		return
	}

	// 执行循环
	result, err := s.runLoop(ctx, messages)
	if err != nil {
		s.setStatus(StatusFailed)
		s.setError(err.Error())
		return
	}

	s.setStatus(StatusCompleted)
	s.setResult(result)
}

// buildMessages 构建消息列表
func (s *Subagent) buildMessages() ([]llm.Message, error) {
	// 使用模板渲染系统提示
	tmpl, err := template.New("system").Parse(s.config.SystemPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse system prompt template: %w", err)
	}

	data := struct {
		Task    string
		Context string
	}{
		Task:    s.task,
		Context: s.context,
	}

	var systemPrompt bytes.Buffer
	if err := tmpl.Execute(&systemPrompt, data); err != nil {
		return nil, fmt.Errorf("failed to execute system prompt template: %w", err)
	}

	messages := []llm.Message{
		{
			Role:    "system",
			Content: systemPrompt.String(),
		},
		{
			Role:    "user",
			Content: "Please complete the task described above.",
		},
	}

	return messages, nil
}

// runLoop 执行代理循环
func (s *Subagent) runLoop(ctx context.Context, messages []llm.Message) (string, error) {
	iterations := 0
	maxIterations := s.config.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 30
	}

	for iterations < maxIterations {
		iterations++

		// 记录迭代进度
		logger.Info("Subagent iteration", "id", s.id, "iteration", iterations, "maxIterations", maxIterations)

		// 构建 LLM 请求
		req := &llm.Request{
			Model:    s.provider.Model(),
			Messages: messages,
			Tools:    s.toolRegistry.GetToolDefinitions(),
		}

		// 调用 LLM
		resp, err := s.provider.Complete(ctx, req)
		if err != nil {
			return "", fmt.Errorf("LLM call failed: %w", err)
		}

		// 获取响应消息
		assistantMsg := resp.ToMessage()

		// 检查是否有工具调用
		if !resp.HasToolCalls() {
			return resp.GetContent(), nil
		}

		// 添加助手消息到历史
		messages = append(messages, assistantMsg)

		// 执行工具调用
		for _, tc := range resp.GetToolCalls() {
			result, err := s.executeTool(ctx, &tc)

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

	return "", fmt.Errorf("max iterations (%d) reached", maxIterations)
}

// executeTool 执行工具
func (s *Subagent) executeTool(ctx context.Context, tc *llm.ToolCall) (string, error) {
	start := time.Now()

	tool, exists := s.toolRegistry.Get(tc.Function.Name)
	if !exists {
		return "", fmt.Errorf("unknown tool: %s", tc.Function.Name)
	}

	result, err := tool.Execute(ctx, tc.Function.Arguments)
	duration := time.Since(start)

	// 记录工具调用
	logger.ToolCall(tc.Function.Name, tc.Function.Arguments, result, duration, err)

	// 记录到执行历史
	s.mu.Lock()
	record := ToolCallRecord{
		Tool:      tc.Function.Name,
		Arguments: string(tc.Function.Arguments),
		Result:    result,
		Duration:  duration,
		Timestamp: start,
	}
	if err != nil {
		record.Error = err.Error()
	}
	s.toolCalls = append(s.toolCalls, record)
	s.mu.Unlock()

	return result, err
}

// generateTaskID 生成任务 ID
func generateTaskID() string {
	return uuid.New().String()[:8]
}
