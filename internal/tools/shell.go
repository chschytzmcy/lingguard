package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ShellTool Shell 执行工具
type ShellTool struct {
	workspace string
	sandboxed bool
}

// NewShellTool 创建 Shell 工具
func NewShellTool(workspace string, sandboxed bool) *ShellTool {
	return &ShellTool{
		workspace: expandPath(workspace),
		sandboxed: sandboxed,
	}
}

// expandPath 展开 ~ 为用户主目录
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

func (t *ShellTool) Name() string { return "shell" }

func (t *ShellTool) Description() string {
	return "Execute shell commands. Use with caution."
}

func (t *ShellTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The shell command to execute",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds (default: 30)",
			},
		},
		"required": []string{"command"},
	}
}

func (t *ShellTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Timeout == 0 {
		p.Timeout = 30
	}

	// 安全检查
	if t.sandboxed {
		if err := t.validateCommand(p.Command); err != nil {
			return "", err
		}
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(ctx, time.Duration(p.Timeout)*time.Second)
	defer cancel()

	// 执行命令
	cmd := exec.CommandContext(ctx, "bash", "-c", p.Command)
	if t.workspace != "" {
		cmd.Dir = t.workspace
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := fmt.Sprintf("stdout:\n%s\nstderr:\n%s",
		stdout.String(), stderr.String())

	if err != nil {
		result += fmt.Sprintf("\nerror: %s", err)
	}

	return result, nil
}

func (t *ShellTool) IsDangerous() bool { return true }

func (t *ShellTool) validateCommand(cmd string) error {
	// 危险命令黑名单
	dangerous := []string{
		"rm -rf /",
		"mkfs",
		"dd if=",
		":(){ :|:& };:",
	}

	lowerCmd := strings.ToLower(cmd)
	for _, d := range dangerous {
		if strings.Contains(lowerCmd, d) {
			return fmt.Errorf("dangerous command detected: %s", d)
		}
	}

	return nil
}
