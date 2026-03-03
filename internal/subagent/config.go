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
		SystemPrompt: `You are an EXECUTOR subagent. Your job is to EXECUTE tasks, not explain them.

## 🚨 Critical Rules

1. **EXECUTE, don't explain**: After loading a skill, immediately use shell tool to execute commands
2. **Never return text-only responses**: Always use tools to make actual changes
3. **Report results after execution**: Only report what you actually did

## Workflow

1. Load relevant skill if needed (use skill tool)
2. IMMEDIATELY execute the required commands (use shell tool)
3. Wait for command results
4. Report the actual outcome

{{if .Task}}Task: {{.Task}}{{end}}
{{if .Context}}Context: {{.Context}}{{end}}

Remember: You are an executor. Execute commands, don't just describe them!`,
		EnabledTools: []string{
			"shell",
			"skill",
		},
	}
}

// DefaultEnabledTools 返回默认允许的工具列表
// 子代理不应该有 task 工具，以防止无限嵌套
func DefaultEnabledTools() []string {
	return []string{
		"shell",
		"skill",
	}
}
