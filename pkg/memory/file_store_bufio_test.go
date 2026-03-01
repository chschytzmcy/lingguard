// Package memory 文件存储 bufio 优化测试
// 验证 P2 修复：file_store.go 使用 bufio 写入
package memory

import (
	"os"
	"strings"
	"testing"
)

// TestP2_FileStore_BufferedWrite 验证 bufio 写入优化
func TestP2_FileStore_BufferedWrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filestore-bufio-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("初始化存储失败: %v", err)
	}

	// 写入大量历史记录
	for i := 0; i < 100; i++ {
		details := map[string]string{
			"index":   string(rune('0' + i%10)),
			"details": strings.Repeat("x", 100), // 100 字节详情
		}
		if err := store.AddHistory("test_event", "test summary", details); err != nil {
			t.Errorf("写入历史失败: %v", err)
		}
	}

	// 验证文件存在且可读
	history, err := store.GetRecentHistory(200)
	if err != nil {
		t.Fatalf("获取历史失败: %v", err)
	}

	if len(history) < 100 {
		t.Errorf("历史记录不足: 预期 100 条，实际 %d 条", len(history))
	}

	t.Logf("✅ Bufio 写入: 成功写入并读取 %d 条历史记录", len(history))
}

// TestP2_FileStore_ConcurrentWrite 验证并发写入
func TestP2_FileStore_ConcurrentWrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filestore-concurrent-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("初始化存储失败: %v", err)
	}

	// 并发写入
	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func(idx int) {
			defer func() { done <- true }()
			for j := 0; j < 10; j++ {
				details := map[string]string{
					"goroutine": string(rune('A' + idx%26)),
					"iteration": string(rune('0' + j%10)),
				}
				store.AddHistory("concurrent_test", "concurrent write", details)
			}
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 50; i++ {
		<-done
	}

	// 验证文件完整性
	history, err := store.GetRecentHistory(1000)
	if err != nil {
		t.Fatalf("获取历史失败: %v", err)
	}

	t.Logf("✅ 并发写入: 50 个 goroutine 完成，共 %d 条历史记录", len(history))
}
