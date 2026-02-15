package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
	mu         sync.RWMutex
	workspace  string
	configPath string
}

// NewWorkspaceManager 创建工作目录管理器
func NewWorkspaceManager(workspace string, configPath string) *WorkspaceManager {
	return &WorkspaceManager{
		workspace:  expandPath(workspace),
		configPath: configPath,
	}
}

// Get 获取当前工作目录
func (m *WorkspaceManager) Get() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.workspace
}

// Set 设置工作目录
func (m *WorkspaceManager) Set(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	expanded := expandPath(path)

	// 验证路径是否存在，不存在则创建
	if _, err := os.Stat(expanded); os.IsNotExist(err) {
		if err := os.MkdirAll(expanded, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	m.workspace = expanded
	return nil
}

// WorkspaceTool 工作目录工具
type WorkspaceTool struct {
	manager *WorkspaceManager
}

// NewWorkspaceTool 创建工作目录工具
func NewWorkspaceTool(manager *WorkspaceManager) *WorkspaceTool {
	return &WorkspaceTool{manager: manager}
}

func (t *WorkspaceTool) Name() string { return "workspace" }

func (t *WorkspaceTool) Description() string {
	return "Get or set the current workspace directory. Use 'cd' operation to change directory."
}

func (t *WorkspaceTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"pwd", "cd", "ls"},
				"description": "Operation: 'pwd' to show current dir, 'cd' to change dir, 'ls' to list contents",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path for cd operation (optional for pwd/ls)",
			},
		},
		"required": []string{"operation"},
	}
}

func (t *WorkspaceTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Operation string `json:"operation"`
		Path      string `json:"path,omitempty"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	switch p.Operation {
	case "pwd":
		return t.pwd()
	case "cd":
		return t.cd(p.Path)
	case "ls":
		return t.ls(p.Path)
	default:
		return "", fmt.Errorf("unknown operation: %s", p.Operation)
	}
}

func (t *WorkspaceTool) IsDangerous() bool { return false }

func (t *WorkspaceTool) pwd() (string, error) {
	workspace := t.manager.Get()
	return fmt.Sprintf("Current workspace: %s", workspace), nil
}

func (t *WorkspaceTool) cd(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required for cd operation")
	}

	// 处理相对路径
	var targetPath string
	if filepath.IsAbs(path) {
		targetPath = path
	} else if len(path) > 0 && path[0] == '~' {
		targetPath = expandPath(path)
	} else {
		// 相对于当前工作目录
		targetPath = filepath.Join(t.manager.Get(), path)
	}

	// 规范化路径
	targetPath = filepath.Clean(targetPath)

	// 检查路径是否存在
	info, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("directory does not exist: %s", targetPath)
		}
		return "", fmt.Errorf("failed to access directory: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", targetPath)
	}

	// 更新工作目录
	if err := t.manager.Set(targetPath); err != nil {
		return "", err
	}

	// 自动列出目录内容
	lsResult, _ := t.ls("")

	return fmt.Sprintf("Workspace changed to: %s\n%s", targetPath, lsResult), nil
}

func (t *WorkspaceTool) ls(path string) (string, error) {
	var targetPath string
	if path == "" {
		targetPath = t.manager.Get()
	} else if filepath.IsAbs(path) {
		targetPath = path
	} else if len(path) > 0 && path[0] == '~' {
		targetPath = expandPath(path)
	} else {
		targetPath = filepath.Join(t.manager.Get(), path)
	}

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to list directory: %w", err)
	}

	var result string
	result = fmt.Sprintf("Directory: %s\n", targetPath)
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
