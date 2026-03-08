package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lingguard/internal/config"
)

func TestRefiner_Deduplicate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 添加重复条目
	store.AddMemory("Project Context", "我的项目叫 LingGuard")
	store.AddMemory("Project Context", "项目名称是 LingGuard")
	store.AddMemory("Project Context", "LingGuard 是我的项目")

	// 添加不重复条目
	store.AddMemory("User Preferences", "我喜欢用 Go 语言")

	// 创建提炼器，使用较低的相似度阈值以便测试
	refiner := NewRefiner(store, nil, &config.RefineConfig{
		Enabled:             true,
		SimilarityThreshold: 0.4, // 降低阈值以检测中文相似内容
		KeepBackup:          true,
	})

	// 执行提炼
	result, err := refiner.Refine(context.Background())
	if err != nil {
		t.Fatalf("failed to refine: %v", err)
	}

	// 验证结果
	if result.TotalEntries != 4 {
		t.Errorf("expected 4 total entries, got %d", result.TotalEntries)
	}

	// 应该合并了重复条目
	if result.RemovedEntries == 0 {
		t.Error("expected some entries to be removed as duplicates")
	}

	// 检查备份是否创建
	if result.BackupPath == "" {
		t.Error("expected backup path to be set")
	}
}

func TestRefiner_ParseEntries(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 添加测试条目
	store.AddMemory("Test Category", "Test fact 1")
	store.AddMemory("Test Category", "Test fact 2")

	refiner := NewRefiner(store, nil, nil)

	// 读取并解析
	content, err := store.GetMemory()
	if err != nil {
		t.Fatalf("failed to get memory: %v", err)
	}

	entries := refiner.parseEntries(content)

	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	for _, entry := range entries {
		if entry.Category != "Test Category" {
			t.Errorf("expected category 'Test Category', got %s", entry.Category)
		}
		if !entry.Timestamp.After(time.Time{}) {
			t.Error("expected timestamp to be set")
		}
	}
}

func TestRefiner_CalculateSimilarity(t *testing.T) {
	refiner := NewRefiner(nil, nil, nil)

	tests := []struct {
		a, b     string
		minScore float32
	}{
		{"我的项目叫 LingGuard", "项目名称是 LingGuard", 0.45},
		{"我喜欢用 Go 语言", "我喜欢用 Go 语言", 0.99},
		{"今天天气很好", "这是一个完全不同的内容", 0.0},
		{"LingGuard 是一个 AI 助手", "LingGuard AI 助手项目", 0.55},
	}

	for _, tt := range tests {
		similarity := refiner.calculateSimilarity(tt.a, tt.b)
		if similarity < tt.minScore {
			t.Errorf("similarity(%q, %q) = %f, want >= %f", tt.a, tt.b, similarity, tt.minScore)
		}
	}
}

func TestRefiner_ShouldTriggerRefine(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 测试默认配置（不自动触发）
	refiner := NewRefiner(store, nil, nil)
	if refiner.ShouldTriggerRefine() {
		t.Error("should not trigger with nil config")
	}

	// 测试手动禁用
	refiner = NewRefiner(store, nil, &config.RefineConfig{
		Enabled:     true,
		AutoTrigger: false,
	})
	if refiner.ShouldTriggerRefine() {
		t.Error("should not trigger with AutoTrigger=false")
	}

	// 测试阈值触发（添加足够的条目）
	for i := 0; i < 55; i++ {
		store.AddMemory("Test", fmt.Sprintf("Fact %d", i))
	}

	refiner = NewRefiner(store, nil, &config.RefineConfig{
		Enabled:     true,
		AutoTrigger: true,
		Threshold:   50,
	})
	if !refiner.ShouldTriggerRefine() {
		t.Error("should trigger when entries >= threshold")
	}
}

func TestRefiner_Backup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 添加一些内容
	store.AddMemory("Test", "Some content")

	refiner := NewRefiner(store, nil, &config.RefineConfig{
		KeepBackup: true,
	})

	backupPath, err := refiner.createBackup()
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	// 检查备份文件存在
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("backup file not found: %s", backupPath)
	}

	// 检查备份内容与原文件一致
	original, _ := os.ReadFile(filepath.Join(tmpDir, "MEMORY.md"))
	backup, _ := os.ReadFile(backupPath)
	if string(original) != string(backup) {
		t.Error("backup content does not match original")
	}
}
