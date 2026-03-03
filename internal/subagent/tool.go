package subagent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lingguard/internal/tools"
)

// TaskTool 后台任务启动工具
type TaskTool struct {
	manager *SubagentManager
}

// NewTaskTool 创建任务工具
func NewTaskTool(manager *SubagentManager) *TaskTool {
	return &TaskTool{
		manager: manager,
	}
}

func (t *TaskTool) Name() string { return "task" }

func (t *TaskTool) Description() string {
	return `启动后台任务执行器（subagent）。

## 🚨 触发规则

**必须使用 task 工具的场景**：
- 复杂编码优化：代码重构、大规模文件处理、多文件修改
- 长时间任务：数据分析、批量处理、复杂代码生成

**禁止使用 task 工具的场景**（主代理直接执行）：
- git 操作（下载/上传代码）→ 调用 skill --name git-sync，然后执行 shell
- 代码审查 → 调用 skill --name code-review，然后执行 shell
- 图像/视频生成 → 直接调用 aigc 工具
- 网络搜索 → 直接调用 web_search 工具
- 天气查询 → 直接调用 skill --name weather
- 简单问答 → 直接回复用户

## 子代理能力

子代理可以使用：shell、file、skill 工具
子代理不能使用：task（防止无限嵌套）

返回 task_id，可用 task_status 查询进度和结果。`
}

func (t *TaskTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task": map[string]interface{}{
				"type":        "string",
				"description": "Clear description of the task to perform. Be specific about what needs to be done.",
			},
			"context": map[string]interface{}{
				"type":        "string",
				"description": "Additional context or background information for the task. Include any relevant details, constraints, or preferences.",
			},
		},
		"required": []string{"task"},
	}
}

func (t *TaskTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Task    string `json:"task"`
		Context string `json:"context"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Task == "" {
		return "", fmt.Errorf("task is required")
	}

	// 创建并启动子代理
	sub, err := t.manager.Spawn(ctx, p.Task, p.Context)
	if err != nil {
		return "", fmt.Errorf("failed to spawn subagent: %w", err)
	}

	// 返回任务信息
	result := map[string]interface{}{
		"task_id": sub.ID(),
		"status":  "started",
		"message": "Task started in background. Use 'task_status' tool with the task_id to check progress and get results.",
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return string(resultJSON), nil
}

func (t *TaskTool) IsDangerous() bool { return false }

func (t *TaskTool) ShouldLoadByDefault() bool { return true }

// TaskStatusTool 任务状态查询工具
type TaskStatusTool struct {
	manager *SubagentManager
}

// NewTaskStatusTool 创建任务状态查询工具
func NewTaskStatusTool(manager *SubagentManager) *TaskStatusTool {
	return &TaskStatusTool{
		manager: manager,
	}
}

func (t *TaskStatusTool) Name() string { return "task_status" }

func (t *TaskStatusTool) Description() string {
	return `Check the status and result of a background task.

Use this tool to check if a previously started task has completed and get its results.
The task must have been started using the 'task' tool.`
}

func (t *TaskStatusTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the task to check (returned by the task tool)",
			},
			"list": map[string]interface{}{
				"type":        "boolean",
				"description": "If true, list all tasks instead of checking a specific one",
			},
		},
	}
}

func (t *TaskStatusTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		TaskID string `json:"task_id"`
		List   bool   `json:"list"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	// 列出所有任务
	if p.List {
		return t.listTasks()
	}

	// 查询特定任务
	if p.TaskID == "" {
		return "", fmt.Errorf("task_id is required (or use list=true to see all tasks)")
	}

	return t.getTaskStatus(p.TaskID)
}

func (t *TaskStatusTool) listTasks() (string, error) {
	summaries := t.manager.ListSummaries()

	result := map[string]interface{}{
		"count": len(summaries),
		"tasks": summaries,
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return string(resultJSON), nil
}

func (t *TaskStatusTool) getTaskStatus(taskID string) (string, error) {
	sub, exists := t.manager.GetStatus(taskID)
	if !exists {
		return "", fmt.Errorf("task not found: %s", taskID)
	}

	result := map[string]interface{}{
		"id":        sub.ID(),
		"task":      sub.Task(),
		"status":    sub.Status(),
		"summary":   sub.GetSummary(),
		"toolCalls": sub.GetToolCalls(), // 添加工具调用历史
	}

	// 如果任务完成，包含结果
	if sub.Status() == StatusCompleted {
		result["result"] = sub.Result()
	}

	// 如果任务失败，包含错误信息
	if sub.Status() == StatusFailed {
		result["error"] = sub.Error()
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return string(resultJSON), nil
}

func (t *TaskStatusTool) IsDangerous() bool { return false }

func (t *TaskStatusTool) ShouldLoadByDefault() bool { return true }

// 确保实现了 Tool 接口
var _ tools.Tool = (*TaskTool)(nil)
var _ tools.Tool = (*TaskStatusTool)(nil)
