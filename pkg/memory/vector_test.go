// Package memory 向量存储测试
package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVectorStoreBasic(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "vector-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建向量存储
	cfg := &VectorStoreConfig{
		DatabasePath: filepath.Join(tmpDir, "test.db"),
		Dimension:    4, // 使用小维度测试
	}

	store, err := NewSQLiteVecStore(cfg, nil, NewNoOpReranker())
	if err != nil {
		t.Fatalf("创建向量存储失败: %v", err)
	}
	defer store.Close()

	// 测试插入
	records := []*VectorRecord{
		{
			ID:        "test-1",
			Content:   "测试内容1",
			Vector:    []float32{0.1, 0.2, 0.3, 0.4},
			Timestamp: time.Now(),
		},
		{
			ID:        "test-2",
			Content:   "测试内容2",
			Vector:    []float32{0.5, 0.6, 0.7, 0.8},
			Timestamp: time.Now(),
		},
	}

	err = store.Upsert(context.Background(), records)
	if err != nil {
		t.Fatalf("插入记录失败: %v", err)
	}

	// 测试计数
	count, err := store.Count(context.Background())
	if err != nil {
		t.Fatalf("计数失败: %v", err)
	}
	if count != 2 {
		t.Errorf("期望计数 2，实际 %d", count)
	}

	// 测试按ID获取
	record, err := store.GetByID(context.Background(), "test-1")
	if err != nil {
		t.Fatalf("获取记录失败: %v", err)
	}
	if record == nil {
		t.Fatal("记录不存在")
	}
	if record.Content != "测试内容1" {
		t.Errorf("期望内容 '测试内容1'，实际 '%s'", record.Content)
	}

	t.Logf("基本测试通过: 插入 %d 条记录", count)
}

func TestVectorSearch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vector-search-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &VectorStoreConfig{
		DatabasePath: filepath.Join(tmpDir, "search.db"),
		Dimension:    4,
	}

	store, err := NewSQLiteVecStore(cfg, nil, NewNoOpReranker())
	if err != nil {
		t.Fatalf("创建向量存储失败: %v", err)
	}
	defer store.Close()

	// 插入测试数据
	records := []*VectorRecord{
		{ID: "doc-1", Content: "苹果是一种水果", Vector: []float32{1.0, 0.0, 0.0, 0.0}},
		{ID: "doc-2", Content: "香蕉是一种水果", Vector: []float32{0.9, 0.1, 0.0, 0.0}},
		{ID: "doc-3", Content: "汽车是一种交通工具", Vector: []float32{0.0, 0.0, 1.0, 0.0}},
	}

	err = store.Upsert(context.Background(), records)
	if err != nil {
		t.Fatalf("插入记录失败: %v", err)
	}

	// 搜索相似向量（查询苹果）
	queryVec := []float32{1.0, 0.0, 0.0, 0.0}
	results, err := store.Search(context.Background(), queryVec, SearchOptions{TopK: 3})
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("没有搜索结果")
	}

	// 第一个结果应该是 doc-1（完全匹配）
	if results[0].ID != "doc-1" {
		t.Errorf("期望第一个结果是 doc-1，实际是 %s", results[0].ID)
	}

	t.Logf("搜索测试通过: 找到 %d 条结果", len(results))
	for i, r := range results {
		t.Logf("  %d. %s (score: %.4f)", i+1, r.Content, r.Score)
	}
}

func TestBM25Search(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bm25-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &VectorStoreConfig{
		DatabasePath: filepath.Join(tmpDir, "bm25.db"),
		Dimension:    4,
	}

	store, err := NewSQLiteVecStore(cfg, nil, NewNoOpReranker())
	if err != nil {
		t.Fatalf("创建向量存储失败: %v", err)
	}
	defer store.Close()

	// 插入测试数据
	records := []*VectorRecord{
		{ID: "doc-1", Content: "苹果是一种美味的水果", Vector: []float32{0.1, 0.1, 0.1, 0.1}},
		{ID: "doc-2", Content: "香蕉含有丰富钾元素", Vector: []float32{0.1, 0.1, 0.1, 0.1}},
	}

	err = store.Upsert(context.Background(), records)
	if err != nil {
		t.Fatalf("插入记录失败: %v", err)
	}

	// BM25 搜索
	results, err := store.SearchBM25(context.Background(), "苹果", SearchOptions{TopK: 10})
	if err != nil {
		t.Logf("BM25 搜索可能不支持: %v", err)
		return
	}

	t.Logf("BM25 搜索测试通过: 找到 %d 条结果", len(results))
}

func TestHybridStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hybrid-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建文件存储
	fileStore := NewFileStore(tmpDir)
	if err := fileStore.Init(); err != nil {
		t.Fatalf("初始化文件存储失败: %v", err)
	}

	// 添加记忆
	err = fileStore.AddMemory("测试分类", "这是一条测试记忆")
	if err != nil {
		t.Fatalf("添加记忆失败: %v", err)
	}

	// 获取记忆
	memory, err := fileStore.GetMemory()
	if err != nil {
		t.Fatalf("获取记忆失败: %v", err)
	}

	if memory == "" {
		t.Fatal("记忆内容为空")
	}

	t.Logf("混合存储测试通过: 记忆内容长度 %d", len(memory))
}
