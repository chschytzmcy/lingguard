package trace

import (
	"context"
	"sync"
	"time"
)

// traceKey 上下文键类型
type traceKey struct{}

// spanKey 上下文键类型
type spanKey struct{}

// WithTrace 将 Trace 存入上下文
func WithTrace(ctx context.Context, trace *Trace) context.Context {
	return context.WithValue(ctx, traceKey{}, trace)
}

// GetTrace 从上下文获取 Trace
func GetTrace(ctx context.Context) *Trace {
	if trace, ok := ctx.Value(traceKey{}).(*Trace); ok {
		return trace
	}
	return nil
}

// WithSpan 将 Span 存入上下文
func WithSpan(ctx context.Context, span *Span) context.Context {
	return context.WithValue(ctx, spanKey{}, span)
}

// GetSpan 从上下文获取 Span
func GetSpan(ctx context.Context) *Span {
	if span, ok := ctx.Value(spanKey{}).(*Span); ok {
		return span
	}
	return nil
}

// GetParentSpanID 获取当前 Span 的父 Span ID
func GetParentSpanID(ctx context.Context) string {
	if span := GetSpan(ctx); span != nil {
		return span.ID
	}
	return ""
}

// Collector 追踪采集器接口
type Collector interface {
	// Trace 生命周期
	StartTrace(ctx context.Context, sessionID string, traceType TraceType, name string, input string) (*Trace, context.Context)
	EndTrace(trace *Trace, output string, err error)

	// Span 生命周期
	StartSpan(ctx context.Context, traceID string, spanType SpanType, name string, input string) (*Span, context.Context)
	StartChildSpan(ctx context.Context, parentSpanID string, spanType SpanType, name string, input string) (*Span, context.Context)
	EndSpan(span *Span, output string, err error)

	// LLM Span 便捷方法
	StartLLMSpan(ctx context.Context, traceID string, provider, model string, input string) (*Span, context.Context)
	EndLLMSpan(span *Span, output string, inputTokens, outputTokens int, err error)

	// Tool Span 便捷方法
	StartToolSpan(ctx context.Context, traceID string, toolName string, input string) (*Span, context.Context)
	EndToolSpan(span *Span, output string, err error)

	// Context Span 便捷方法
	StartContextSpan(ctx context.Context, traceID string, input string) (*Span, context.Context)
	EndContextSpan(span *Span, output string, err error)

	// 获取存储
	GetStore() Store
	Close() error
}

// CollectorImpl 追踪采集器实现
type CollectorImpl struct {
	store Store

	// 活跃的 traces（内存缓存）
	activeTraces sync.Map
	activeSpans  sync.Map
}

// NewCollector 创建追踪采集器
func NewCollector(store Store) *CollectorImpl {
	return &CollectorImpl{
		store: store,
	}
}

// GetStore 获取存储
func (c *CollectorImpl) GetStore() Store {
	return c.store
}

// StartTrace 开始追踪
func (c *CollectorImpl) StartTrace(ctx context.Context, sessionID string, traceType TraceType, name string, input string) (*Trace, context.Context) {
	trace := &Trace{
		SessionID: sessionID,
		TraceType: traceType,
		Name:      name,
		StartTime: time.Now(),
		Status:    StatusRunning,
		Input:     input,
	}

	if err := c.store.CreateTrace(trace); err != nil {
		// 创建失败，返回空 trace
		return trace, ctx
	}

	// 缓存活跃 trace
	c.activeTraces.Store(trace.ID, trace)

	return trace, WithTrace(ctx, trace)
}

// EndTrace 结束追踪
func (c *CollectorImpl) EndTrace(trace *Trace, output string, err error) {
	trace.EndTime = time.Now()
	trace.Output = output

	if err != nil {
		trace.Status = StatusError
		trace.Error = err.Error()
	} else {
		trace.Status = StatusSuccess
	}

	// 从活跃缓存中移除
	c.activeTraces.Delete(trace.ID)

	// 更新存储
	if updateErr := c.store.UpdateTrace(trace); updateErr != nil {
		// 忽略更新错误
	}
}

// StartSpan 开始 Span
func (c *CollectorImpl) StartSpan(ctx context.Context, traceID string, spanType SpanType, name string, input string) (*Span, context.Context) {
	span := &Span{
		TraceID:   traceID,
		SpanType:  spanType,
		Name:      name,
		StartTime: time.Now(),
		Status:    StatusRunning,
		Input:     input,
	}

	// 设置父 Span ID（如果上下文中有）
	if parentSpanID := GetParentSpanID(ctx); parentSpanID != "" {
		span.ParentID = parentSpanID
	}

	if err := c.store.CreateSpan(span); err != nil {
		return span, ctx
	}

	// 缓存活跃 span
	c.activeSpans.Store(span.ID, span)

	return span, WithSpan(ctx, span)
}

// StartChildSpan 开始子 Span
func (c *CollectorImpl) StartChildSpan(ctx context.Context, parentSpanID string, spanType SpanType, name string, input string) (*Span, context.Context) {
	// 获取父 Span 以获取 traceID
	var traceID string
	if parentSpan, ok := c.activeSpans.Load(parentSpanID); ok {
		traceID = parentSpan.(*Span).TraceID
	} else {
		// 从存储获取
		if parentSpan, err := c.store.GetSpan(parentSpanID); err == nil {
			traceID = parentSpan.TraceID
		}
	}

	span := &Span{
		TraceID:   traceID,
		ParentID:  parentSpanID,
		SpanType:  spanType,
		Name:      name,
		StartTime: time.Now(),
		Status:    StatusRunning,
		Input:     input,
	}

	if err := c.store.CreateSpan(span); err != nil {
		return span, ctx
	}

	// 缓存活跃 span
	c.activeSpans.Store(span.ID, span)

	return span, WithSpan(ctx, span)
}

// EndSpan 结束 Span
func (c *CollectorImpl) EndSpan(span *Span, output string, err error) {
	span.EndTime = time.Now()
	span.Output = output

	if err != nil {
		span.Status = StatusError
		span.Error = err.Error()
	} else {
		span.Status = StatusSuccess
	}

	// 从活跃缓存中移除
	c.activeSpans.Delete(span.ID)

	// 更新存储
	if updateErr := c.store.UpdateSpan(span); updateErr != nil {
		// 忽略更新错误
	}

	// 更新父 Trace 的 token 统计
	if span.SpanType == SpanTypeLLM {
		c.updateTraceTokens(span.TraceID, span.InputTokens, span.OutputTokens)
	}
}

// updateTraceTokens 更新 Trace 的 Token 统计
func (c *CollectorImpl) updateTraceTokens(traceID string, inputTokens, outputTokens int) {
	trace, err := c.store.GetTrace(traceID)
	if err != nil {
		return
	}

	trace.InputTokens += inputTokens
	trace.OutputTokens += outputTokens
	trace.TotalTokens = trace.InputTokens + trace.OutputTokens

	c.store.UpdateTrace(trace)
}

// StartLLMSpan 开始 LLM Span
func (c *CollectorImpl) StartLLMSpan(ctx context.Context, traceID string, provider, model string, input string) (*Span, context.Context) {
	span, ctx := c.StartSpan(ctx, traceID, SpanTypeLLM, "LLM Call: "+provider+"/"+model, input)
	span.Provider = provider
	span.Model = model
	return span, ctx
}

// EndLLMSpan 结束 LLM Span
func (c *CollectorImpl) EndLLMSpan(span *Span, output string, inputTokens, outputTokens int, err error) {
	span.InputTokens = inputTokens
	span.OutputTokens = outputTokens
	span.TotalTokens = inputTokens + outputTokens
	c.EndSpan(span, output, err)
}

// StartToolSpan 开始 Tool Span
func (c *CollectorImpl) StartToolSpan(ctx context.Context, traceID string, toolName string, input string) (*Span, context.Context) {
	return c.StartSpan(ctx, traceID, SpanTypeTool, "Tool: "+toolName, input)
}

// EndToolSpan 结束 Tool Span
func (c *CollectorImpl) EndToolSpan(span *Span, output string, err error) {
	c.EndSpan(span, output, err)
}

// StartContextSpan 开始 Context Span
func (c *CollectorImpl) StartContextSpan(ctx context.Context, traceID string, input string) (*Span, context.Context) {
	return c.StartSpan(ctx, traceID, SpanTypeContext, "Context Build", input)
}

// EndContextSpan 结束 Context Span
func (c *CollectorImpl) EndContextSpan(span *Span, output string, err error) {
	c.EndSpan(span, output, err)
}

// Close 关闭采集器
func (c *CollectorImpl) Close() error {
	return c.store.Close()
}

// NoopCollector 空采集器（用于禁用追踪时）
type NoopCollector struct{}

// NewNoopCollector 创建空采集器
func NewNoopCollector() *NoopCollector {
	return &NoopCollector{}
}

func (c *NoopCollector) StartTrace(ctx context.Context, sessionID string, traceType TraceType, name string, input string) (*Trace, context.Context) {
	return &Trace{ID: "noop"}, ctx
}

func (c *NoopCollector) EndTrace(trace *Trace, output string, err error) {}

func (c *NoopCollector) StartSpan(ctx context.Context, traceID string, spanType SpanType, name string, input string) (*Span, context.Context) {
	return &Span{ID: "noop"}, ctx
}

func (c *NoopCollector) StartChildSpan(ctx context.Context, parentSpanID string, spanType SpanType, name string, input string) (*Span, context.Context) {
	return &Span{ID: "noop"}, ctx
}

func (c *NoopCollector) EndSpan(span *Span, output string, err error) {}

func (c *NoopCollector) StartLLMSpan(ctx context.Context, traceID string, provider, model string, input string) (*Span, context.Context) {
	return &Span{ID: "noop"}, ctx
}

func (c *NoopCollector) EndLLMSpan(span *Span, output string, inputTokens, outputTokens int, err error) {
}

func (c *NoopCollector) StartToolSpan(ctx context.Context, traceID string, toolName string, input string) (*Span, context.Context) {
	return &Span{ID: "noop"}, ctx
}

func (c *NoopCollector) EndToolSpan(span *Span, output string, err error) {}

func (c *NoopCollector) StartContextSpan(ctx context.Context, traceID string, input string) (*Span, context.Context) {
	return &Span{ID: "noop"}, ctx
}

func (c *NoopCollector) EndContextSpan(span *Span, output string, err error) {}

func (c *NoopCollector) GetStore() Store { return nil }

func (c *NoopCollector) Close() error { return nil }
