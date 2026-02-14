// Package subagent 子代理系统
package subagent

// SubagentConfig 子代理配置
type SubagentConfig struct {
	// MaxIterations 最大迭代次数，默认 15
	MaxIterations int

	// SystemPrompt 子代理系统提示模板
	SystemPrompt string

	// EnabledTools 允许的工具列表（白名单）
	// 如果为空，则使用默认工具集
	EnabledTools []string
}

// DefaultSubagentConfig 默认子代理配置
func DefaultSubagentConfig() *SubagentConfig {
	return &SubagentConfig{
		MaxIterations: 15,
		SystemPrompt: `You are a focused subagent working on a specific task.

Your goal: Complete the assigned task efficiently and report results.

Guidelines:
1. Focus only on the given task
2. Use available tools as needed
3. Provide a clear summary when done
4. If blocked, explain why and what's needed

{{if .Task}}Task: {{.Task}}{{end}}
{{if .Context}}Context: {{.Context}}{{end}}`,
		EnabledTools: []string{
			"shell",
			"read",
			"write",
			"edit",
			"glob",
			"grep",
			"skill",
		},
	}
}

// DefaultEnabledTools 返回默认允许的工具列表
// 子代理不应该有 task 工具，以防止无限嵌套
func DefaultEnabledTools() []string {
	return []string{
		"shell",
		"read",
		"write",
		"edit",
		"glob",
		"grep",
		"skill",
	}
}
