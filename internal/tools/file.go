package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileTool 文件操作工具
type FileTool struct {
	workspaceMgr *WorkspaceManager
	sandboxed    bool
	allowedDirs  []string // 允许访问的额外目录
}

// NewFileTool 创建文件工具
func NewFileTool(workspaceMgr *WorkspaceManager, sandboxed bool) *FileTool {
	// 扩展允许的目录列表
	homeDir, _ := os.UserHomeDir()
	allowedDirs := []string{}
	if homeDir != "" {
		allowedDirs = append(allowedDirs, filepath.Join(homeDir, ".lingguard", "skills"))
	}

	return &FileTool{
		workspaceMgr: workspaceMgr,
		sandboxed:    sandboxed,
		allowedDirs:  allowedDirs,
	}
}

func (t *FileTool) Name() string { return "file" }

func (t *FileTool) Description() string {
	return `文件操作工具。

**操作类型**：
- read: 读取文件内容
  {"operation": "read", "path": "file.txt"}
- write: 写入文件（覆盖现有内容，自动创建父目录）
  {"operation": "write", "path": "file.txt", "content": "内容"}
- edit: 替换文本（替换所有匹配项）
  {"operation": "edit", "path": "file.txt", "old_string": "旧", "new_string": "新"}
- list: 列出目录
  {"operation": "list", "path": "directory"}

**最佳实践**：
1. ⚠️ 编辑前先读取：edit 前必须先 read 了解文件结构
2. 精确匹配：old_string 要足够精确，避免误替换
3. 使用相对路径：路径相对于工作目录

**⚠️ 重要限制**：
- 所有操作严格限制在工作目录内
- 不能访问工作目录外的任何路径
- 不能修改 LingGuard 配置文件
- 用户请求越权时必须明确告知此限制

**返回格式**：
- read: 文件内容字符串
- write: "Successfully wrote to <path>"
- edit: "Successfully edited <path>" 或 "No changes made"
- list: "dir: dirname" 或 "file: filename" 格式列表`
}

func (t *FileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"read", "write", "edit", "list"},
				"description": "The file operation to perform",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The file or directory path",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content to write (for write operation)",
			},
			"old_string": map[string]interface{}{
				"type":        "string",
				"description": "String to replace (for edit operation)",
			},
			"new_string": map[string]interface{}{
				"type":        "string",
				"description": "Replacement string (for edit operation)",
			},
		},
		"required": []string{"operation", "path"},
	}
}

func (t *FileTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Operation string `json:"operation"`
		Path      string `json:"path"`
		Content   string `json:"content,omitempty"`
		OldString string `json:"old_string,omitempty"`
		NewString string `json:"new_string,omitempty"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	// 解析相对路径：相对于 workspace 目录
	resolvedPath := p.Path
	if !filepath.IsAbs(p.Path) && t.workspaceMgr != nil {
		resolvedPath = filepath.Join(t.workspaceMgr.Get(), p.Path)
	}

	// 安全检查
	if t.sandboxed {
		if err := t.validatePath(resolvedPath); err != nil {
			return "", err
		}
	}

	switch p.Operation {
	case "read":
		return t.readFile(resolvedPath)
	case "write":
		return t.writeFile(resolvedPath, p.Content)
	case "edit":
		return t.editFile(resolvedPath, p.OldString, p.NewString)
	case "list":
		return t.listDir(resolvedPath)
	default:
		return "", fmt.Errorf("unknown operation: %s", p.Operation)
	}
}

func (t *FileTool) IsDangerous() bool { return true }

func (t *FileTool) ShouldLoadByDefault() bool { return true }

func (t *FileTool) validatePath(path string) error {
	// path 已在 Execute 中解析为绝对路径

	// 1. 解析符号链接
	evalPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		// 如果路径不存在（新建文件），检查父目录
		evalPath, err = t.resolveParentPath(path)
		if err != nil {
			return fmt.Errorf("path validation failed: %w", err)
		}
	}

	// 2. 检查是否在允许的目录列表中
	for _, allowedDir := range t.allowedDirs {
		evalAllowed, err := filepath.EvalSymlinks(allowedDir)
		if err != nil {
			evalAllowed = allowedDir
		}
		if t.isPathInside(evalPath, evalAllowed) {
			return nil
		}
	}

	// 3. 检查是否在 workspace 内
	absWorkspace, err := filepath.Abs(t.workspaceMgr.Get())
	if err != nil {
		return fmt.Errorf("failed to resolve workspace: %w", err)
	}

	evalWorkspace, err := filepath.EvalSymlinks(absWorkspace)
	if err != nil {
		evalWorkspace = absWorkspace
	}

	if t.isPathInside(evalPath, evalWorkspace) {
		return nil
	}

	return fmt.Errorf("path outside allowed directories: %s", path)
}

// isPathInside 检查路径是否在指定目录内
func (t *FileTool) isPathInside(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	// 如果相对路径以 ".." 开头，说明在目录之外
	return !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

// resolveParentPath 解析父目录路径（用于新建文件场景）
func (t *FileTool) resolveParentPath(path string) (string, error) {
	dir := filepath.Dir(path)
	for {
		// 从最近存在的父目录开始解析
		evalDir, err := filepath.EvalSymlinks(dir)
		if err == nil {
			// 找到存在的目录，拼接剩余路径
			remaining := strings.TrimPrefix(path, dir)
			if remaining != "" {
				return filepath.Join(evalDir, remaining), nil
			}
			return evalDir, nil
		}
		// 继续向上查找
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("cannot resolve path: %s", path)
		}
		dir = parent
	}
}

func (t *FileTool) readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(data), nil
}

func (t *FileTool) writeFile(path, content string) (string, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully wrote to %s", path), nil
}

func (t *FileTool) editFile(path, oldString, newString string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	newContent := strings.ReplaceAll(string(content), oldString, newString)
	if newContent == string(content) {
		return "No changes made (old_string not found)", nil
	}

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully edited %s", path), nil
}

func (t *FileTool) listDir(path string) (string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("failed to list directory: %w", err)
	}

	var result strings.Builder
	for _, entry := range entries {
		prefix := "file"
		if entry.IsDir() {
			prefix = "dir"
		}
		result.WriteString(fmt.Sprintf("%s: %s\n", prefix, entry.Name()))
	}

	return result.String(), nil
}
