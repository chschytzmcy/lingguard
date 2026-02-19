// Package memory - 混合存储实现 (FileStore + VectorStore)
package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lingguard/internal/config"
	"github.com/lingguard/pkg/embedding"
)

// HybridStore 混合存储，组合 FileStore + VectorStore
// 实现完全向后兼容，同时提供向量检索能力
type HybridStore struct {
	fileStore   *FileStore
	vectorStore *SQLiteVecStore
	embedding   embedding.Model

	// 配置
	vectorEnabled bool
	searchConfig  *config.SearchConfig

	// 缓冲写入
	buffer     []*VectorRecord
	bufferMu   sync.Mutex
	bufferSize int
	flushTimer *time.Timer

	// 状态
	closed bool
}

// HybridStoreConfig 混合存储配置
type HybridStoreConfig struct {
	MemoryDir    string
	VectorConfig *config.VectorConfig
	Providers    map[string]config.ProviderConfig // 用于获取 API Key
}

// NewHybridStore 创建混合存储
func NewHybridStore(cfg *HybridStoreConfig) (*HybridStore, error) {
	// 创建 FileStore
	fileStore := NewFileStore(cfg.MemoryDir)
	if err := fileStore.Init(); err != nil {
		return nil, fmt.Errorf("init file store: %w", err)
	}

	store := &HybridStore{
		fileStore:     fileStore,
		bufferSize:    10, // 缓冲10条记录
		vectorEnabled: cfg.VectorConfig != nil && cfg.VectorConfig.Enabled,
	}

	// 如果启用向量检索，初始化向量存储
	if store.vectorEnabled {
		// 创建 embedding 模型
		emb, err := createEmbeddingModel(cfg.VectorConfig, cfg.Providers)
		if err != nil {
			// 向量初始化失败不影响基本功能
			store.vectorEnabled = false
		} else {
			store.embedding = emb

			// 创建向量存储
			vectorStoreCfg := &VectorStoreConfig{
				DatabasePath: getVectorDbPath(cfg.VectorConfig, cfg.MemoryDir),
				Dimension:    getEmbeddingDimension(cfg.VectorConfig),
			}

			// 创建重排序器
			var reranker Reranker
			if cfg.VectorConfig.Search.Rerank != nil && cfg.VectorConfig.Search.Rerank.Enabled {
				reranker = createReranker(cfg.VectorConfig, cfg.Providers)
			} else {
				reranker = NewNoOpReranker()
			}

			vectorStore, err := NewSQLiteVecStore(vectorStoreCfg, emb, reranker)
			if err != nil {
				// 向量存储初始化失败不影响基本功能
				store.vectorEnabled = false
			} else {
				store.vectorStore = vectorStore
				store.searchConfig = &cfg.VectorConfig.Search
			}
		}
	}

	return store, nil
}

// Add 添加消息 (实现 Store 接口)
func (s *HybridStore) Add(ctx context.Context, sessionID string, msg *Message) error {
	// 始终写入文件存储
	if err := s.fileStore.Add(ctx, sessionID, msg); err != nil {
		return err
	}

	// 如果启用向量，异步索引
	if s.vectorEnabled && s.embedding != nil && msg.Content != "" {
		record := &VectorRecord{
			ID:      uuid.New().String(),
			Content: msg.Content,
			Metadata: map[string]interface{}{
				"session_id": sessionID,
				"role":       msg.Role,
			},
			Timestamp: time.Now(),
		}

		// 添加到缓冲
		s.addToBuffer(record)
	}

	return nil
}

// Get 获取消息 (实现 Store 接口)
func (s *HybridStore) Get(ctx context.Context, sessionID string, limit int) ([]*Message, error) {
	return s.fileStore.Get(ctx, sessionID, limit)
}

// Clear 清除会话 (实现 Store 接口)
func (s *HybridStore) Clear(ctx context.Context, sessionID string) error {
	return s.fileStore.Clear(ctx, sessionID)
}

// Close 关闭存储
func (s *HybridStore) Close() error {
	s.closed = true

	// 刷新缓冲
	s.flushBuffer()

	// 关闭向量存储
	if s.vectorStore != nil {
		s.vectorStore.Close()
	}

	return s.fileStore.Close()
}

// FileStore 获取底层文件存储
func (s *HybridStore) FileStore() *FileStore {
	return s.fileStore
}

// VectorStore 获取底层向量存储
func (s *HybridStore) VectorStore() *SQLiteVecStore {
	return s.vectorStore
}

// IsVectorEnabled 检查是否启用向量检索
func (s *HybridStore) IsVectorEnabled() bool {
	return s.vectorEnabled && s.vectorStore != nil
}

// Search 执行搜索
// 如果启用向量检索，使用混合检索；否则使用文件搜索
func (s *HybridStore) Search(ctx context.Context, query string, topK int) ([]*VectorRecord, error) {
	if !s.IsVectorEnabled() {
		// 回退到文件搜索
		return s.searchFromFile(ctx, query, topK)
	}

	// 生成查询向量
	queryVec, err := s.embedding.Embed(ctx, query)
	if err != nil {
		// 向量生成失败，回退到文件搜索
		return s.searchFromFile(ctx, query, topK)
	}

	// 混合检索
	opts := HybridSearchOptions{
		TopK:         topK,
		VectorWeight: s.searchConfig.VectorWeight,
		BM25Weight:   s.searchConfig.BM25Weight,
		MinScore:     s.searchConfig.MinScore,
	}

	if opts.VectorWeight == 0 {
		opts.VectorWeight = 0.7
	}
	if opts.BM25Weight == 0 {
		opts.BM25Weight = 0.3
	}
	if topK <= 0 {
		opts.TopK = 10
	}

	return s.vectorStore.HybridSearch(ctx, queryVec, query, opts)
}

// searchFromFile 从文件搜索 (回退方案)
func (s *HybridStore) searchFromFile(ctx context.Context, query string, topK int) ([]*VectorRecord, error) {
	results, err := s.fileStore.SearchAll(query)
	if err != nil {
		return nil, err
	}

	var records []*VectorRecord
	for fileName, lines := range results {
		for _, line := range lines {
			records = append(records, &VectorRecord{
				ID:        fmt.Sprintf("%s-%d", fileName, len(records)),
				Content:   line,
				Timestamp: time.Now(),
				Score:     1.0,
				Metadata: map[string]interface{}{
					"source": fileName,
				},
			})
		}
	}

	if len(records) > topK {
		records = records[:topK]
	}

	return records, nil
}

// AddMemory 添加长期记忆
func (s *HybridStore) AddMemory(category, content string) error {
	if err := s.fileStore.AddMemory(category, content); err != nil {
		return err
	}

	// 如果启用向量，索引记忆
	if s.IsVectorEnabled() {
		record := &VectorRecord{
			ID:      uuid.New().String(),
			Content: fmt.Sprintf("[%s] %s", category, content),
			Metadata: map[string]interface{}{
				"type":     "memory",
				"category": category,
			},
			Timestamp: time.Now(),
		}
		s.addToBuffer(record)
	}

	return nil
}

// GetMemory 获取 MEMORY.md 内容
func (s *HybridStore) GetMemory() (string, error) {
	return s.fileStore.GetMemory()
}

// SearchMemory 搜索记忆
func (s *HybridStore) SearchMemory(ctx context.Context, query string) ([]*VectorRecord, error) {
	if s.IsVectorEnabled() {
		return s.Search(ctx, query, 10)
	}

	// 回退到 grep 搜索
	lines, err := s.fileStore.SearchMemory(query)
	if err != nil {
		return nil, err
	}

	var records []*VectorRecord
	for i, line := range lines {
		records = append(records, &VectorRecord{
			ID:        fmt.Sprintf("memory-%d", i),
			Content:   line,
			Timestamp: time.Now(),
			Score:     1.0,
		})
	}

	return records, nil
}

// AddHistory 添加历史记录
func (s *HybridStore) AddHistory(eventType, summary string, details map[string]string) error {
	return s.fileStore.AddHistory(eventType, summary, details)
}

// GetRecentHistory 获取最近历史
func (s *HybridStore) GetRecentHistory(lines int) ([]string, error) {
	return s.fileStore.GetRecentHistory(lines)
}

// WriteDailyLog 写入每日日志
func (s *HybridStore) WriteDailyLog(content string) error {
	if err := s.fileStore.WriteDailyLog(content); err != nil {
		return err
	}

	// 如果启用向量，索引日志
	if s.IsVectorEnabled() {
		record := &VectorRecord{
			ID:      uuid.New().String(),
			Content: content,
			Metadata: map[string]interface{}{
				"type": "daily_log",
				"date": time.Now().Format("2006-01-02"),
			},
			Timestamp: time.Now(),
		}
		s.addToBuffer(record)
	}

	return nil
}

// GetRecentDailyLogs 获取最近几天的日志
func (s *HybridStore) GetRecentDailyLogs(days int) (map[string]string, error) {
	return s.fileStore.GetRecentDailyLogs(days)
}

// 缓冲管理

// addToBuffer 添加到缓冲
func (s *HybridStore) addToBuffer(record *VectorRecord) {
	s.bufferMu.Lock()
	defer s.bufferMu.Unlock()

	s.buffer = append(s.buffer, record)

	// 达到缓冲大小或定时刷新
	if len(s.buffer) >= s.bufferSize {
		go s.flushBuffer()
	} else if s.flushTimer == nil {
		s.flushTimer = time.AfterFunc(5*time.Second, s.flushBuffer)
	}
}

// flushBuffer 刷新缓冲到向量存储
func (s *HybridStore) flushBuffer() {
	s.bufferMu.Lock()
	defer s.bufferMu.Unlock()

	if len(s.buffer) == 0 || s.vectorStore == nil {
		return
	}

	// 取出待处理的记录
	records := s.buffer
	s.buffer = make([]*VectorRecord, 0)

	// 停止定时器
	if s.flushTimer != nil {
		s.flushTimer.Stop()
		s.flushTimer = nil
	}

	// 异步生成向量并存储
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// 批量生成向量
		texts := make([]string, len(records))
		for i, r := range records {
			texts[i] = r.Content
		}

		vectors, err := s.embedding.EmbedBatch(ctx, texts)
		if err != nil {
			return
		}

		// 设置向量
		for i, vec := range vectors {
			if i < len(records) {
				records[i].Vector = vec
			}
		}

		// 存储到向量数据库
		s.vectorStore.Upsert(ctx, records)
	}()
}

// 辅助函数

// createEmbeddingModel 创建 Embedding 模型
func createEmbeddingModel(cfg *config.VectorConfig, providers map[string]config.ProviderConfig) (embedding.Model, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("vector config not enabled")
	}

	embCfg := &embedding.Config{
		Provider:  cfg.Embedding.Provider,
		Model:     cfg.Embedding.Model,
		Dimension: cfg.Embedding.Dimension,
	}

	// 获取 API Key
	if cfg.Embedding.APIKey != "" {
		embCfg.APIKey = cfg.Embedding.APIKey
	} else if providers != nil {
		// 从 Provider 配置继承
		providerName := cfg.Embedding.Provider
		if providerName == "" {
			providerName = "qwen" // 默认使用 qwen
		}
		if p, ok := providers[providerName]; ok {
			embCfg.APIKey = p.APIKey
			embCfg.APIBase = p.APIBase
		}
	}

	if embCfg.APIKey == "" {
		return nil, fmt.Errorf("embedding API key not configured")
	}

	switch embCfg.Provider {
	case "qwen", "dashscope":
		return embedding.NewQwenEmbedding(embCfg), nil
	default:
		// 默认使用 qwen
		return embedding.NewQwenEmbedding(embCfg), nil
	}
}

// createReranker 创建重排序器
func createReranker(cfg *config.VectorConfig, providers map[string]config.ProviderConfig) Reranker {
	rerankCfg := cfg.Search.Rerank
	if rerankCfg == nil {
		return NewNoOpReranker()
	}

	apiKey := rerankCfg.APIKey
	if apiKey == "" && providers != nil {
		// 从 Provider 配置继承
		providerName := rerankCfg.Provider
		if providerName == "" {
			providerName = "qwen"
		}
		if p, ok := providers[providerName]; ok {
			apiKey = p.APIKey
		}
	}

	if apiKey == "" {
		return NewNoOpReranker()
	}

	return NewQwenReranker(&RerankConfig{
		Provider: rerankCfg.Provider,
		Model:    rerankCfg.Model,
		APIKey:   apiKey,
		APIBase:  rerankCfg.APIBase,
	})
}

// getVectorDbPath 获取向量数据库路径
func getVectorDbPath(cfg *config.VectorConfig, memoryDir string) string {
	if cfg.Database.Path != "" {
		return cfg.Database.Path
	}
	return memoryDir + "/vectors.db"
}

// getEmbeddingDimension 获取 Embedding 维度
func getEmbeddingDimension(cfg *config.VectorConfig) int {
	if cfg.Database.Dimension > 0 {
		return cfg.Database.Dimension
	}
	if cfg.Embedding.Dimension > 0 {
		return cfg.Embedding.Dimension
	}
	return embedding.DefaultDimension
}

// 确保 HybridStore 实现 Store 接口
var _ Store = (*HybridStore)(nil)
