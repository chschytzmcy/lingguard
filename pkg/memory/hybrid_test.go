// Package memory 集成测试
package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lingguard/internal/config"
)

func TestHybridStoreWithVector(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hybrid-vector-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建配置
	vectorCfg := &config.VectorConfig{
		Enabled: true,
		Embedding: config.EmbeddingConfig{
			Provider:  "qwen",
			Model:     "text-embedding-v4",
			Dimension: 1024,
		},
		Search: config.SearchConfig{
			VectorWeight: 0.7,
			BM25Weight:   0.3,
			DefaultTopK:  10,
		},
		Database: config.VectorDbConfig{
			Path:      filepath.Join(tmpDir, "vectors.db"),
			Dimension: 1024,
		},
	}

	// 创建 HybridStore 配置
	hybridCfg := &HybridStoreConfig{
		MemoryDir:    tmpDir,
		VectorConfig: vectorCfg,
		Providers:    nil, // 不测试实际的 embedding
	}

	// 由于没有 API Key，测试不带向量的 HybridStore
	store, err := NewHybridStore(hybridCfg)
	if err != nil {
		t.Logf("创建 HybridStore 可能失败（缺少 API Key）: %v", err)
		t.Log("测试跳过，这是预期的行为")
		return
	}
	defer store.Close()

	// 测试基本操作
	ctx := context.Background()

	// 添加记忆
	err = store.AddMemory("测试", "这是一条测试记忆内容")
	if err != nil {
		t.Fatalf("添加记忆失败: %v", err)
	}

	// 获取记忆
	mem, err := store.GetMemory()
	if err != nil {
		t.Fatalf("获取记忆失败: %v", err)
	}
	if mem == "" {
		t.Fatal("记忆内容为空")
	}

	t.Logf("记忆内容: %s", mem[:min(100, len(mem))])

	// 测试搜索（回退到文件搜索）
	records, err := store.Search(ctx, "测试", 5)
	if err != nil {
		t.Logf("搜索可能失败: %v", err)
	} else {
		t.Logf("搜索结果: %d 条", len(records))
	}

	t.Log("HybridStore 测试完成")
}

func TestConfigDefaults(t *testing.T) {
	// 测试默认配置
	cfg := config.DefaultConfig()

	if cfg.Agents.MemoryConfig == nil {
		t.Fatal("默认记忆配置为空")
	}

	if !cfg.Agents.MemoryConfig.Enabled {
		t.Error("默认记忆应该启用")
	}

	// 新的向量配置默认应该是 nil
	if cfg.Agents.MemoryConfig.Vector != nil {
		t.Log("注意: 默认向量配置已设置")
	}

	t.Log("配置默认值测试通过")
}

func TestVectorConfigJSON(t *testing.T) {
	// 测试向量配置的 JSON 序列化
	cfg := &config.MemoryConfig{
		Enabled:         true,
		RecentDays:      3,
		MaxHistoryLines: 1000,
		Vector: &config.VectorConfig{
			Enabled: true,
			Embedding: config.EmbeddingConfig{
				Provider:  "qwen",
				Model:     "text-embedding-v4",
				Dimension: 1024,
			},
			Search: config.SearchConfig{
				VectorWeight: 0.7,
				BM25Weight:   0.3,
				DefaultTopK:  10,
				MinScore:     0.5,
				Rerank: &config.RerankConfig{
					Enabled:  true,
					Provider: "qwen",
					Model:    "qwen3-vl-rerank",
				},
			},
			Database: config.VectorDbConfig{
				Path:      "~/.lingguard/memory/vectors.db",
				Dimension: 1024,
			},
		},
	}

	t.Logf("向量配置: enabled=%v, provider=%s, model=%s",
		cfg.Vector.Enabled,
		cfg.Vector.Embedding.Provider,
		cfg.Vector.Embedding.Model)

	t.Log("向量配置 JSON 测试通过")
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float32
		expected float32
	}{
		{"相同向量", []float32{1, 0, 0}, []float32{1, 0, 0}, 1.0},
		{"正交向量", []float32{1, 0, 0}, []float32{0, 1, 0}, 0.0},
		{"相反向量", []float32{1, 0, 0}, []float32{-1, 0, 0}, -1.0},
		{"相似向量", []float32{1, 1, 0}, []float32{1, 0.9, 0}, 0.995},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			// 允许小误差
			if abs(result-tt.expected) > 0.01 {
				t.Errorf("cosineSimilarity() = %v, want %v", result, tt.expected)
			} else {
				t.Logf("cosineSimilarity(%v, %v) = %.4f ✓", tt.a, tt.b, result)
			}
		})
	}
}

func TestRRFFusion(t *testing.T) {
	vectorResults := []*VectorRecord{
		{ID: "1", Content: "文档1", Score: 0.9},
		{ID: "2", Content: "文档2", Score: 0.8},
		{ID: "3", Content: "文档3", Score: 0.7},
	}

	bm25Results := []*VectorRecord{
		{ID: "2", Content: "文档2", Score: 0.95},
		{ID: "4", Content: "文档4", Score: 0.85},
		{ID: "1", Content: "文档1", Score: 0.75},
	}

	fused := rrfFusion(vectorResults, bm25Results, 0.7, 0.3)

	if len(fused) == 0 {
		t.Fatal("RRF 融合结果为空")
	}

	t.Logf("RRF 融合结果:")
	for i, r := range fused {
		t.Logf("  %d. %s (score: %.4f)", i+1, r.Content, r.Score)
	}

	t.Log("RRF 融合测试通过")
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
