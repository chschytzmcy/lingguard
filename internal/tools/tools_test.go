package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestShellToolBasic(t *testing.T) {
	tool := NewShellTool("", false)

	if tool.Name() != "shell" {
		t.Errorf("Expected name=shell, got %s", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}

	if !tool.IsDangerous() {
		t.Error("Shell tool should be dangerous")
	}

	params := tool.Parameters()
	if params["type"] != "object" {
		t.Error("Parameters should be object type")
	}
}

func TestShellToolExecute(t *testing.T) {
	tool := NewShellTool("", false)
	ctx := context.Background()

	params := json.RawMessage(`{"command":"echo hello"}`)
	_, err := tool.Execute(ctx, params)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// 检查输出包含 hello
}

func TestShellToolWithTimeout(t *testing.T) {
	tool := NewShellTool("", false)
	ctx := context.Background()

	params := json.RawMessage(`{"command":"sleep 0.1","timeout":1}`)
	_, err := tool.Execute(ctx, params)

	if err != nil {
		t.Fatalf("Execute with timeout failed: %v", err)
	}
}

func TestShellToolInvalidParams(t *testing.T) {
	tool := NewShellTool("", false)
	ctx := context.Background()

	params := json.RawMessage(`{}`)
	_, err := tool.Execute(ctx, params)

	if err != nil {
		t.Error("Empty params should still work with default timeout")
	}
}

func TestShellToolSandbox(t *testing.T) {
	tool := NewShellTool("/tmp", true)

	if !tool.sandboxed {
		t.Error("Tool should be sandboxed")
	}
}

func TestShellToolDangerousCommand(t *testing.T) {
	tool := NewShellTool("", true)
	ctx := context.Background()

	// 测试危险命令检测
	params := json.RawMessage(`{"command":"rm -rf /"}`)
	_, err := tool.Execute(ctx, params)

	if err == nil {
		t.Error("Dangerous command should be blocked in sandbox mode")
	}
}

func TestFileToolBasic(t *testing.T) {
	tool := NewFileTool("", false)

	if tool.Name() != "file" {
		t.Errorf("Expected name=file, got %s", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}

	if !tool.IsDangerous() {
		t.Error("File tool should be dangerous")
	}
}

func TestFileToolReadWrite(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "lingguard-file-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tool := NewFileTool(tmpDir, false)
	ctx := context.Background()

	testFile := filepath.Join(tmpDir, "test.txt")

	// 写入文件
	writeParams := json.RawMessage(`{"operation":"write","path":"` + testFile + `","content":"Hello World"}`)
	result, err := tool.Execute(ctx, writeParams)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	t.Logf("Write result: %s", result)

	// 读取文件
	readParams := json.RawMessage(`{"operation":"read","path":"` + testFile + `"}`)
	result, err = tool.Execute(ctx, readParams)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if result != "Hello World" {
		t.Errorf("Expected content='Hello World', got %s", result)
	}
}

func TestFileToolEdit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-file-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tool := NewFileTool(tmpDir, false)
	ctx := context.Background()

	testFile := filepath.Join(tmpDir, "edit.txt")

	// 先写入
	tool.Execute(ctx, json.RawMessage(`{"operation":"write","path":"`+testFile+`","content":"Hello World"}`))

	// 编辑
	editParams := json.RawMessage(`{"operation":"edit","path":"` + testFile + `","old_string":"World","new_string":"Go"}`)
	_, err = tool.Execute(ctx, editParams)
	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}

	// 验证结果
	result, _ := tool.Execute(ctx, json.RawMessage(`{"operation":"read","path":"`+testFile+`"}`))
	if result != "Hello Go" {
		t.Errorf("Expected 'Hello Go', got %s", result)
	}
}

func TestFileToolList(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-file-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建一些文件
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("1"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	tool := NewFileTool(tmpDir, false)
	ctx := context.Background()

	params := json.RawMessage(`{"operation":"list","path":"` + tmpDir + `"}`)
	_, err = tool.Execute(ctx, params)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// 检查结果包含文件
}

func TestFileToolInvalidOperation(t *testing.T) {
	tool := NewFileTool("", false)
	ctx := context.Background()

	params := json.RawMessage(`{"operation":"invalid","path":"/tmp"}`)
	_, err := tool.Execute(ctx, params)

	if err == nil {
		t.Error("Invalid operation should return error")
	}
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry()

	shellTool := NewShellTool("", false)
	fileTool := NewFileTool("", false)

	registry.Register(shellTool)
	registry.Register(fileTool)

	// 测试 Get
	tl, ok := registry.Get("shell")
	if !ok {
		t.Error("Should find shell tool")
	}
	if tl.Name() != "shell" {
		t.Errorf("Expected shell tool, got %s", tl.Name())
	}

	// 测试 List
	tools := registry.List()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}

	// 测试 GetToolDefinitions
	defs := registry.GetToolDefinitions()
	if len(defs) != 2 {
		t.Errorf("Expected 2 definitions, got %d", len(defs))
	}
}

func TestToolDefinition(t *testing.T) {
	tool := NewShellTool("", false)
	def := Definition(tool)

	if def["type"] != "function" {
		t.Error("Definition type should be function")
	}

	fn, ok := def["function"].(map[string]interface{})
	if !ok {
		t.Fatal("Function should be a map")
	}

	if fn["name"] != "shell" {
		t.Errorf("Expected function name=shell, got %s", fn["name"])
	}
}
