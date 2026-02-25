// Package trace LLM 调用追踪系统
package trace

import (
	"time"
)

// Trace 追踪 - 一次完整的请求会话
type Trace struct {
	ID           string                 `json:"id"`
	SessionID    string                 `json:"sessionId"`
	TraceType    TraceType              `json:"traceType"`
	Name         string                 `json:"name"`
	StartTime    time.Time              `json:"startTime"`
	EndTime      time.Time              `json:"endTime,omitempty"`
	DurationMs   int64                  `json:"durationMs"`
	Status       Status                 `json:"status"`
	Error        string                 `json:"error,omitempty"`
	InputTokens  int                    `json:"inputTokens"`
	OutputTokens int                    `json:"outputTokens"`
	TotalTokens  int                    `json:"totalTokens"`
	Input        string                 `json:"input,omitempty"`
	Output       string                 `json:"output,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"createdAt"`
}

// TraceType 追踪类型
type TraceType string

const (
	TraceTypeChat   TraceType = "chat"
	TraceTypeStream TraceType = "stream"
)

// Status 执行状态
type Status string

const (
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusError   Status = "error"
)

// Span 执行步骤 - 单个执行单元
type Span struct {
	ID           string                 `json:"id"`
	TraceID      string                 `json:"traceId"`
	ParentID     string                 `json:"parentId,omitempty"`
	SpanType     SpanType               `json:"spanType"`
	Name         string                 `json:"name"`
	Provider     string                 `json:"provider,omitempty"`
	Model        string                 `json:"model,omitempty"`
	StartTime    time.Time              `json:"startTime"`
	EndTime      time.Time              `json:"endTime,omitempty"`
	DurationMs   int64                  `json:"durationMs"`
	Status       Status                 `json:"status"`
	Error        string                 `json:"error,omitempty"`
	InputTokens  int                    `json:"inputTokens"`
	OutputTokens int                    `json:"outputTokens"`
	TotalTokens  int                    `json:"totalTokens"`
	Input        string                 `json:"input,omitempty"`
	Output       string                 `json:"output,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"createdAt"`
}

// SpanType Span 类型
type SpanType string

const (
	SpanTypeLLM     SpanType = "llm"
	SpanTypeTool    SpanType = "tool"
	SpanTypeContext SpanType = "context"
)

// TraceFilter 追踪过滤条件
type TraceFilter struct {
	SessionID string     `json:"sessionId,omitempty"`
	Status    *Status    `json:"status,omitempty"`
	TraceType *TraceType `json:"traceType,omitempty"`
	StartTime *time.Time `json:"startTime,omitempty"`
	EndTime   *time.Time `json:"endTime,omitempty"`
	Limit     int        `json:"limit,omitempty"`
	Offset    int        `json:"offset,omitempty"`
}

// SpanFilter Span 过滤条件
type SpanFilter struct {
	TraceID  string    `json:"traceId,omitempty"`
	SpanType *SpanType `json:"spanType,omitempty"`
	Status   *Status   `json:"status,omitempty"`
	Limit    int       `json:"limit,omitempty"`
	Offset   int       `json:"offset,omitempty"`
}

// TraceStats 统计信息
type TraceStats struct {
	TotalTraces   int            `json:"totalTraces"`
	TotalSpans    int            `json:"totalSpans"`
	TotalTokens   int64          `json:"totalTokens"`
	InputTokens   int64          `json:"inputTokens"`
	OutputTokens  int64          `json:"outputTokens"`
	AvgDurationMs int64          `json:"avgDurationMs"`
	SuccessRate   float64        `json:"successRate"`
	ByStatus      map[string]int `json:"byStatus"`
	ByType        map[string]int `json:"byType"`
	ByProvider    map[string]int `json:"byProvider"`
	ByModel       map[string]int `json:"byModel"`
}

// TraceDetail Trace 详情（包含 Spans）
type TraceDetail struct {
	Trace *Trace  `json:"trace"`
	Spans []*Span `json:"spans"`
}

// TraceEvent 追踪事件（用于 SSE）
type TraceEvent struct {
	Type      string    `json:"type"` // create, update, complete
	TraceID   string    `json:"traceId"`
	SpanID    string    `json:"spanId,omitempty"`
	Trace     *Trace    `json:"trace,omitempty"`
	Span      *Span     `json:"span,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// 事件类型常量
const (
	EventTypeTraceCreate   = "trace_create"
	EventTypeTraceUpdate   = "trace_update"
	EventTypeTraceComplete = "trace_complete"
	EventTypeSpanCreate    = "span_create"
	EventTypeSpanUpdate    = "span_update"
	EventTypeSpanComplete  = "span_complete"
)

// SpanTree Span 树形结构（用于前端展示）
type SpanTree struct {
	Span     *Span       `json:"span"`
	Children []*SpanTree `json:"children,omitempty"`
}

// ToSpanTree 将 Span 列表转换为树形结构
func ToSpanTree(spans []*Span) []*SpanTree {
	// 构建映射
	spanMap := make(map[string]*SpanTree)
	for _, span := range spans {
		spanMap[span.ID] = &SpanTree{
			Span:     span,
			Children: []*SpanTree{},
		}
	}

	// 构建树
	var roots []*SpanTree
	for _, span := range spans {
		node := spanMap[span.ID]
		if span.ParentID == "" {
			roots = append(roots, node)
		} else {
			if parent, ok := spanMap[span.ParentID]; ok {
				parent.Children = append(parent.Children, node)
			}
		}
	}

	return roots
}
