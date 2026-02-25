package trace

import (
	"time"
)

// Service 追踪服务
type Service struct {
	store Store
}

// NewService 创建追踪服务
func NewService(store Store) *Service {
	return &Service{
		store: store,
	}
}

// GetStore 获取存储
func (s *Service) GetStore() Store {
	return s.store
}

// CreateTrace 创建追踪
func (s *Service) CreateTrace(trace *Trace) error {
	return s.store.CreateTrace(trace)
}

// GetTrace 获取追踪
func (s *Service) GetTrace(id string) (*Trace, error) {
	return s.store.GetTrace(id)
}

// UpdateTrace 更新追踪
func (s *Service) UpdateTrace(trace *Trace) error {
	return s.store.UpdateTrace(trace)
}

// DeleteTrace 删除追踪
func (s *Service) DeleteTrace(id string) error {
	return s.store.DeleteTrace(id)
}

// ListTraces 列出追踪
func (s *Service) ListTraces(filter *TraceFilter) ([]*Trace, error) {
	if filter == nil {
		filter = &TraceFilter{}
	}
	if filter.Limit == 0 {
		filter.Limit = 50
	}
	return s.store.ListTraces(filter)
}

// GetTraceDetail 获取追踪详情（包含 Spans）
func (s *Service) GetTraceDetail(id string) (*TraceDetail, error) {
	trace, err := s.store.GetTrace(id)
	if err != nil {
		return nil, err
	}

	spans, err := s.store.GetSpansByTrace(id)
	if err != nil {
		return nil, err
	}

	return &TraceDetail{
		Trace: trace,
		Spans: spans,
	}, nil
}

// GetTraceStats 获取统计信息
func (s *Service) GetTraceStats() (*TraceStats, error) {
	return s.store.GetTraceStats()
}

// GetSpansByTrace 获取 Trace 的所有 Spans
func (s *Service) GetSpansByTrace(traceID string) ([]*Span, error) {
	return s.store.GetSpansByTrace(traceID)
}

// GetSpanTree 获取 Span 树形结构
func (s *Service) GetSpanTree(traceID string) ([]*SpanTree, error) {
	spans, err := s.store.GetSpansByTrace(traceID)
	if err != nil {
		return nil, err
	}
	return ToSpanTree(spans), nil
}

// CleanupOldTraces 清理旧追踪
func (s *Service) CleanupOldTraces(days int) (int64, error) {
	return s.store.CleanupOldTraces(days)
}

// GetTracesBySession 获取会话的所有追踪
func (s *Service) GetTracesBySession(sessionID string) ([]*Trace, error) {
	return s.store.ListTraces(&TraceFilter{
		SessionID: sessionID,
		Limit:     100,
	})
}

// GetRecentTraces 获取最近的追踪
func (s *Service) GetRecentTraces(limit int) ([]*Trace, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.store.ListTraces(&TraceFilter{
		Limit: limit,
	})
}

// GetTracesByTimeRange 获取时间范围内的追踪
func (s *Service) GetTracesByTimeRange(start, end time.Time, limit int) ([]*Trace, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.store.ListTraces(&TraceFilter{
		StartTime: &start,
		EndTime:   &end,
		Limit:     limit,
	})
}

// GetRunningTraces 获取运行中的追踪
func (s *Service) GetRunningTraces() ([]*Trace, error) {
	status := StatusRunning
	return s.store.ListTraces(&TraceFilter{
		Status: &status,
		Limit:  100,
	})
}

// GetFailedTraces 获取失败的追踪
func (s *Service) GetFailedTraces(limit int) ([]*Trace, error) {
	if limit <= 0 {
		limit = 50
	}
	status := StatusError
	return s.store.ListTraces(&TraceFilter{
		Status: &status,
		Limit:  limit,
	})
}

// Subscribe 订阅事件
func (s *Service) Subscribe() <-chan TraceEvent {
	return s.store.Subscribe()
}

// Close 关闭服务
func (s *Service) Close() error {
	return s.store.Close()
}
