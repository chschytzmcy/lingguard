package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStore_Init(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "lingguard-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 检查 MEMORY.md 是否创建
	memoryFile := filepath.Join(tmpDir, "MEMORY.md")
	if _, err := os.Stat(memoryFile); os.IsNotExist(err) {
		t.Error("MEMORY.md not created")
	}

	// HISTORY.md 不再创建
	historyFile := filepath.Join(tmpDir, "HISTORY.md")
	if _, err := os.Stat(historyFile); !os.IsNotExist(err) {
		t.Error("HISTORY.md should not be created anymore")
	}
}

func TestFileStore_AddMemory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 添加记忆
	if err := store.AddMemory("User Preferences", "User prefers Go over Python"); err != nil {
		t.Fatalf("failed to add memory: %v", err)
	}

	// 验证内容
	content, err := store.GetMemory()
	if err != nil {
		t.Fatalf("failed to get memory: %v", err)
	}

	if content == "" {
		t.Error("memory content is empty")
	}
}

func TestFileStore_SearchMemory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 添加记忆
	if err := store.AddMemory("User Preferences", "User prefers dark mode"); err != nil {
		t.Fatalf("failed to add memory: %v", err)
	}

	// 验证文件内容
	content, _ := store.GetMemory()
	t.Logf("Memory content:\n%s", content)

	// 搜索记忆（使用简单的关键词）
	results, err := store.SearchMemory("dark")
	if err != nil {
		t.Fatalf("failed to search memory: %v", err)
	}

	t.Logf("Search results: %v", results)

	if len(results) == 0 {
		t.Error("expected to find 'dark' in memory")
	}
}

func TestFileStore_DailyLog(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 写入每日日志
	if err := store.WriteDailyLog("Completed important task"); err != nil {
		t.Fatalf("failed to write daily log: %v", err)
	}

	// 获取最近日志
	logs, err := store.GetRecentDailyLogs(1)
	if err != nil {
		t.Fatalf("failed to get daily logs: %v", err)
	}

	if len(logs) == 0 {
		t.Error("daily logs is empty")
	}
}

func TestFileStore_CleanOldDailyLogs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 创建一些测试日志文件
	now := time.Now()
	dates := []string{
		now.Format("2006-01-02"),                    // 今天
		now.AddDate(0, 0, -1).Format("2006-01-02"),  // 昨天
		now.AddDate(0, 0, -5).Format("2006-01-02"),  // 5 天前
		now.AddDate(0, 0, -10).Format("2006-01-02"), // 10 天前
		now.AddDate(0, 0, -31).Format("2006-01-02"), // 31 天前
	}

	for _, date := range dates {
		dailyFile := filepath.Join(tmpDir, date+".md")
		content := fmt.Sprintf("# Daily Log - %s\n\nTest content\n", date)
		if err := os.WriteFile(dailyFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create daily log %s: %v", date, err)
		}
	}

	// 测试清理（保留 7 天）
	deleted, err := store.CleanOldDailyLogs(7)
	if err != nil {
		t.Fatalf("failed to clean old daily logs: %v", err)
	}

	// 应该删除 10 天前和 31 天前的日志
	if deleted != 2 {
		t.Errorf("expected 2 files deleted, got %d", deleted)
	}

	// 验证今天和昨天的日志还在
	todayFile := filepath.Join(tmpDir, now.Format("2006-01-02")+".md")
	if _, err := os.Stat(todayFile); os.IsNotExist(err) {
		t.Error("today's log should not be deleted")
	}

	yesterdayFile := filepath.Join(tmpDir, now.AddDate(0, 0, -1).Format("2006-01-02")+".md")
	if _, err := os.Stat(yesterdayFile); os.IsNotExist(err) {
		t.Error("yesterday's log should not be deleted")
	}

	// 验证 31 天前的日志已被删除
	oldFile := filepath.Join(tmpDir, now.AddDate(0, 0, -31).Format("2006-01-02")+".md")
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("31 days old log should be deleted")
	}

	// 测试不清理（maxAge = 0）
	deleted2, err := store.CleanOldDailyLogs(0)
	if err != nil {
		t.Fatalf("failed to clean with maxAge=0: %v", err)
	}
	if deleted2 != 0 {
		t.Errorf("expected 0 files deleted with maxAge=0, got %d", deleted2)
	}
}

func TestContextBuilder_BuildContext(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 添加一些记忆
	store.AddMemory("Project Context", "This is a Go project")
	store.WriteDailyLog("Working on memory system")

	builder := NewContextBuilder(store)
	ctx, err := builder.BuildContext(1)
	if err != nil {
		t.Fatalf("failed to build context: %v", err)
	}

	if ctx == "" {
		t.Error("context is empty")
	}
}
