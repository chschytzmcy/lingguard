// Package memory 记忆系统 - 上下文构建器
package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ContextBuilder 上下文构建器（参考 nanobot）
type ContextBuilder struct {
	store       *FileStore
	hybridStore *HybridStore // 可选：混合存储，支持向量检索
}

// NewContextBuilder 创建上下文构建器
func NewContextBuilder(store *FileStore) *ContextBuilder {
	return &ContextBuilder{store: store}
}

// NewContextBuilderWithHybrid 创建支持向量检索的上下文构建器
func NewContextBuilderWithHybrid(store *HybridStore) *ContextBuilder {
	return &ContextBuilder{
		store:       store.FileStore(),
		hybridStore: store,
	}
}

// BuildContext 构建记忆上下文
// 返回包含长期记忆和最近历史的上下文字符串
func (b *ContextBuilder) BuildContext(includeRecentDays int) (string, error) {
	var context strings.Builder

	// 1. 加载长期记忆（MEMORY.md）
	memory, err := b.store.GetMemory()
	if err != nil {
		return "", fmt.Errorf("load memory: %w", err)
	}

	// 过滤掉注释和空行，只保留有价值的内容
	cleanMemory := b.cleanMemoryContent(memory)
	if cleanMemory != "" {
		context.WriteString("## Long-term Memory\n\n")
		context.WriteString(cleanMemory)
		context.WriteString("\n\n")
	}

	// 2. 加载最近的每日日志
	if includeRecentDays > 0 {
		dailyLogs, err := b.store.GetRecentDailyLogs(includeRecentDays)
		if err == nil && len(dailyLogs) > 0 {
			context.WriteString("## Recent Activity\n\n")
			// 按日期倒序
			for i := 0; i < includeRecentDays; i++ {
				date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
				if log, ok := dailyLogs[date]; ok {
					context.WriteString(fmt.Sprintf("### %s\n", date))
					context.WriteString(log)
					context.WriteString("\n")
				}
			}
		}
	}

	// 3. 加载最近的历史记录
	recentHistory, err := b.store.GetRecentHistory(50)
	if err == nil && len(recentHistory) > 0 {
		context.WriteString("## Recent History\n\n")
		// 只保留最近的几个事件
		start := len(recentHistory) - 20
		if start < 0 {
			start = 0
		}
		for _, line := range recentHistory[start:] {
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "#") {
				context.WriteString(line + "\n")
			}
		}
	}

	return context.String(), nil
}

// BuildContextWithQuery 基于查询构建相关上下文
// 如果启用向量检索，使用语义搜索；否则使用 grep 搜索
func (b *ContextBuilder) BuildContextWithQuery(query string, includeRecentDays int) (string, error) {
	var result strings.Builder

	// 首先获取基础上下文
	baseContext, err := b.BuildContext(includeRecentDays)
	if err != nil {
		return "", err
	}

	// 搜索相关记忆
	if b.hybridStore != nil && b.hybridStore.IsVectorEnabled() {
		// 使用向量语义搜索
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		records, err := b.hybridStore.Search(ctx, query, 10)
		if err == nil && len(records) > 0 {
			result.WriteString("## Relevant Memories (Semantic Search)\n\n")
			for _, record := range records {
				// 限制内容长度
				content := record.Content
				if len(content) > 300 {
					content = content[:300] + "..."
				}
				result.WriteString(fmt.Sprintf("- [%.2f] %s\n", record.Score, content))
			}
			result.WriteString("\n")
		}
	} else {
		// 回退到 grep 搜索
		searchResults, err := b.store.SearchAll(query)
		if err == nil && len(searchResults) > 0 {
			result.WriteString("## Relevant Memories\n\n")
			for file, lines := range searchResults {
				result.WriteString(fmt.Sprintf("### From %s\n", file))
				for _, line := range lines {
					// 限制行长度
					if len(line) > 200 {
						line = line[:200] + "..."
					}
					result.WriteString(line + "\n")
				}
				result.WriteString("\n")
			}
		}
	}

	result.WriteString(baseContext)
	return result.String(), nil
}

// cleanMemoryContent 清理记忆内容，移除注释和格式化
func (b *ContextBuilder) cleanMemoryContent(content string) string {
	var result strings.Builder
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 跳过空行
		if trimmed == "" {
			continue
		}

		// 跳过 HTML 注释
		if strings.HasPrefix(trimmed, "<!--") || strings.HasSuffix(trimmed, "-->") {
			continue
		}

		// 保留标题（降低一级）
		if strings.HasPrefix(trimmed, "# ") {
			result.WriteString("###" + trimmed[1:] + "\n")
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			result.WriteString("####" + trimmed[2:] + "\n")
			continue
		}

		// 保留列表项和内容
		result.WriteString(line + "\n")
	}

	return result.String()
}

// MemoryTools 记忆操作工具（可供 Agent 调用）

// MemoryTools 记忆工具集合
type MemoryTools struct {
	store       *FileStore
	hybridStore *HybridStore // 可选：支持向量检索
}

// NewMemoryTools 创建记忆工具
func NewMemoryTools(store *FileStore) *MemoryTools {
	return &MemoryTools{store: store}
}

// NewMemoryToolsWithHybrid 创建支持向量检索的记忆工具
func NewMemoryToolsWithHybrid(store *HybridStore) *MemoryTools {
	return &MemoryTools{
		store:       store.FileStore(),
		hybridStore: store,
	}
}

// Remember 记录长期记忆
func (t *MemoryTools) Remember(category, fact string) error {
	return t.store.AddMemory(category, fact)
}

// Recall 回忆（搜索记忆）
func (t *MemoryTools) Recall(query string) (map[string][]string, error) {
	return t.store.SearchAll(query)
}

// RecallSemantic 语义搜索记忆（使用向量检索）
func (t *MemoryTools) RecallSemantic(ctx context.Context, query string, topK int) ([]*VectorRecord, error) {
	if t.hybridStore != nil && t.hybridStore.IsVectorEnabled() {
		return t.hybridStore.Search(ctx, query, topK)
	}
	// 回退到关键词搜索
	records, err := t.store.SearchAll(query)
	if err != nil {
		return nil, err
	}
	var results []*VectorRecord
	for file, lines := range records {
		for i, line := range lines {
			results = append(results, &VectorRecord{
				ID:        fmt.Sprintf("%s-%d", file, i),
				Content:   line,
				Timestamp: time.Now(),
				Score:     1.0,
				Metadata:  map[string]interface{}{"source": file},
			})
		}
	}
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// LogEvent 记录事件到每日日志
func (t *MemoryTools) LogEvent(event string) error {
	return t.store.WriteDailyLog(event)
}

// GetContext 获取当前上下文
func (t *MemoryTools) GetContext() (string, error) {
	builder := NewContextBuilder(t.store)
	return builder.BuildContext(3) // 最近3天
}
