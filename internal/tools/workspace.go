package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// expandPath 展开 ~ 为用户主目录
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

// WorkspaceManager 工作目录管理器
type WorkspaceManager struct {
	workspace string
}

// NewWorkspaceManager 创建工作目录管理器
func NewWorkspaceManager(workspace string, configPath string) *WorkspaceManager {
	return &WorkspaceManager{
		workspace: expandPath(workspace),
	}
}

// Get 获取当前工作目录
func (m *WorkspaceManager) Get() string {
	return m.workspace
}

// WorkspaceTool 工作目录工具（只读）
type WorkspaceTool struct {
	manager *WorkspaceManager
}

// NewWorkspaceTool 创建工作目录工具
func NewWorkspaceTool(manager *WorkspaceManager) *WorkspaceTool {
	return &WorkspaceTool{manager: manager}
}

func (t *WorkspaceTool) Name() string { return "workspace" }

func (t *WorkspaceTool) Description() string {
	return "Get current workspace directory and list contents. Workspace is configured in config.json and cannot be changed at runtime."
}

func (t *WorkspaceTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"pwd", "ls"},
				"description": "Operation: 'pwd' to show current workspace, 'ls' to list contents",
			},
		},
		"required": []string{"operation"},
	}
}

func (t *WorkspaceTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Operation string `json:"operation"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	switch p.Operation {
	case "pwd":
		return t.pwd()
	case "ls":
		return t.ls()
	default:
		return "", fmt.Errorf("unknown operation: %s (only 'pwd' and 'ls' are allowed)", p.Operation)
	}
}

func (t *WorkspaceTool) IsDangerous() bool { return false }

func (t *WorkspaceTool) ShouldLoadByDefault() bool { return true }

func (t *WorkspaceTool) pwd() (string, error) {
	workspace := t.manager.Get()
	return fmt.Sprintf("Workspace: %s", workspace), nil
}

func (t *WorkspaceTool) ls() (string, error) {
	targetPath := t.manager.Get()

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to list directory: %w", err)
	}

	var result string
	result = fmt.Sprintf("Workspace: %s\n", targetPath)
	result += "─────────────────\n"

	for _, entry := range entries {
		prefix := "📄"
		if entry.IsDir() {
			prefix = "📁"
		}
		info, _ := entry.Info()
		size := ""
		if info != nil && !entry.IsDir() {
			size = fmt.Sprintf(" (%d bytes)", info.Size())
		}
		result += fmt.Sprintf("%s %s%s\n", prefix, entry.Name(), size)
	}

	return result, nil
}
