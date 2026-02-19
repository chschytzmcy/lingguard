// Package embedding 提供文本嵌入向量生成接口
package embedding

import (
	"context"
)

// Model 文本嵌入模型接口
type Model interface {
	// Name 返回模型名称
	Name() string

	// Dimension 返回向量维度
	Dimension() int

	// Embed 生成单个文本的嵌入向量
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch 批量生成嵌入向量
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// Config 嵌入模型配置
type Config struct {
	Provider  string `json:"provider"`            // 提供商: "qwen", "openai" 等
	Model     string `json:"model"`               // 模型名称
	APIKey    string `json:"apiKey"`              // API Key
	APIBase   string `json:"apiBase,omitempty"`   // API 基础 URL (可选)
	Dimension int    `json:"dimension,omitempty"` // 向量维度 (可选，使用模型默认值)
}

// DefaultDimension 默认向量维度
const DefaultDimension = 1024
