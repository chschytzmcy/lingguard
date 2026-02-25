package trace

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// Store 追踪存储接口
type Store interface {
	// Trace CRUD
	CreateTrace(trace *Trace) error
	GetTrace(id string) (*Trace, error)
	UpdateTrace(trace *Trace) error
	DeleteTrace(id string) error
	ListTraces(filter *TraceFilter) ([]*Trace, error)

	// Span CRUD
	CreateSpan(span *Span) error
	GetSpan(id string) (*Span, error)
	UpdateSpan(span *Span) error
	DeleteSpan(id string) error
	ListSpans(filter *SpanFilter) ([]*Span, error)
	GetSpansByTrace(traceID string) ([]*Span, error)

	// 统计
	GetTraceStats() (*TraceStats, error)

	// 清理
	CleanupOldTraces(days int) (int64, error)

	// 事件订阅
	Subscribe() <-chan TraceEvent
	Close() error
}

// SQLiteStore SQLite 存储
type SQLiteStore struct {
	db       *sql.DB
	dbPath   string
	mu       sync.RWMutex
	eventChs []chan TraceEvent
	eventMu  sync.RWMutex
}

// NewSQLiteStore 创建 SQLite 存储
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	// 连接数据库
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// 设置连接池
	db.SetMaxOpenConns(1) // SQLite 单连接
	db.SetMaxIdleConns(1)

	store := &SQLiteStore{
		db:     db,
		dbPath: dbPath,
	}

	// 初始化表
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return store, nil
}

// initSchema 初始化数据库结构
func (s *SQLiteStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS traces (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		trace_type TEXT NOT NULL DEFAULT 'chat',
		name TEXT,
		start_time DATETIME NOT NULL,
		end_time DATETIME,
		duration_ms INTEGER DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'running',
		error TEXT,
		input_tokens INTEGER DEFAULT 0,
		output_tokens INTEGER DEFAULT 0,
		total_tokens INTEGER DEFAULT 0,
		input TEXT,
		output TEXT,
		metadata TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS spans (
		id TEXT PRIMARY KEY,
		trace_id TEXT NOT NULL,
		parent_id TEXT,
		span_type TEXT NOT NULL,
		name TEXT NOT NULL,
		provider TEXT,
		model TEXT,
		start_time DATETIME NOT NULL,
		end_time DATETIME,
		duration_ms INTEGER DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'running',
		error TEXT,
		input_tokens INTEGER DEFAULT 0,
		output_tokens INTEGER DEFAULT 0,
		total_tokens INTEGER DEFAULT 0,
		input TEXT,
		output TEXT,
		metadata TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (trace_id) REFERENCES traces(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_traces_session ON traces(session_id);
	CREATE INDEX IF NOT EXISTS idx_traces_start_time ON traces(start_time);
	CREATE INDEX IF NOT EXISTS idx_traces_status ON traces(status);
	CREATE INDEX IF NOT EXISTS idx_traces_type ON traces(trace_type);
	CREATE INDEX IF NOT EXISTS idx_spans_trace ON spans(trace_id);
	CREATE INDEX IF NOT EXISTS idx_spans_type ON spans(span_type);
	CREATE INDEX IF NOT EXISTS idx_spans_parent ON spans(parent_id);
	`

	_, err := s.db.Exec(schema)
	return err
}

// CreateTrace 创建追踪
func (s *SQLiteStore) CreateTrace(trace *Trace) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 生成 ID
	if trace.ID == "" {
		trace.ID = uuid.New().String()[:8]
	}

	// 设置时间
	now := time.Now()
	if trace.StartTime.IsZero() {
		trace.StartTime = now
	}
	if trace.CreatedAt.IsZero() {
		trace.CreatedAt = now
	}

	// 设置默认值
	if trace.Status == "" {
		trace.Status = StatusRunning
	}
	if trace.TraceType == "" {
		trace.TraceType = TraceTypeChat
	}

	// 序列化 metadata
	var metadataJSON []byte
	if trace.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(trace.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}

	query := `
		INSERT INTO traces (
			id, session_id, trace_type, name, start_time, end_time,
			duration_ms, status, error, input_tokens, output_tokens,
			total_tokens, input, output, metadata, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		trace.ID, trace.SessionID, trace.TraceType, trace.Name, trace.StartTime, trace.EndTime,
		trace.DurationMs, trace.Status, trace.Error, trace.InputTokens, trace.OutputTokens,
		trace.TotalTokens, trace.Input, trace.Output, metadataJSON, trace.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("insert trace: %w", err)
	}

	// 发送事件
	s.emitEvent(TraceEvent{
		Type:      EventTypeTraceCreate,
		TraceID:   trace.ID,
		Trace:     trace,
		Timestamp: now,
	})

	return nil
}

// GetTrace 获取追踪
func (s *SQLiteStore) GetTrace(id string) (*Trace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, session_id, trace_type, name, start_time, end_time,
			duration_ms, status, error, input_tokens, output_tokens,
			total_tokens, input, output, metadata, created_at
		FROM traces WHERE id = ?
	`

	trace := &Trace{}
	var metadataJSON []byte
	var endTime sql.NullTime

	err := s.db.QueryRow(query, id).Scan(
		&trace.ID, &trace.SessionID, &trace.TraceType, &trace.Name, &trace.StartTime, &endTime,
		&trace.DurationMs, &trace.Status, &trace.Error, &trace.InputTokens, &trace.OutputTokens,
		&trace.TotalTokens, &trace.Input, &trace.Output, &metadataJSON, &trace.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("trace not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query trace: %w", err)
	}

	if endTime.Valid {
		trace.EndTime = endTime.Time
	}

	// 解析 metadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &trace.Metadata); err != nil {
			trace.Metadata = nil
		}
	}

	return trace, nil
}

// UpdateTrace 更新追踪
func (s *SQLiteStore) UpdateTrace(trace *Trace) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 计算耗时
	if !trace.EndTime.IsZero() {
		trace.DurationMs = trace.EndTime.Sub(trace.StartTime).Milliseconds()
	}

	// 计算总 token
	trace.TotalTokens = trace.InputTokens + trace.OutputTokens

	// 序列化 metadata
	var metadataJSON []byte
	if trace.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(trace.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}

	query := `
		UPDATE traces SET
			session_id = ?, trace_type = ?, name = ?, start_time = ?, end_time = ?,
			duration_ms = ?, status = ?, error = ?, input_tokens = ?, output_tokens = ?,
			total_tokens = ?, input = ?, output = ?, metadata = ?
		WHERE id = ?
	`

	result, err := s.db.Exec(query,
		trace.SessionID, trace.TraceType, trace.Name, trace.StartTime, trace.EndTime,
		trace.DurationMs, trace.Status, trace.Error, trace.InputTokens, trace.OutputTokens,
		trace.TotalTokens, trace.Input, trace.Output, metadataJSON,
		trace.ID,
	)

	if err != nil {
		return fmt.Errorf("update trace: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("trace not found: %s", trace.ID)
	}

	// 发送事件
	eventType := EventTypeTraceUpdate
	if trace.Status != StatusRunning {
		eventType = EventTypeTraceComplete
	}
	s.emitEvent(TraceEvent{
		Type:      eventType,
		TraceID:   trace.ID,
		Trace:     trace,
		Timestamp: time.Now(),
	})

	return nil
}

// DeleteTrace 删除追踪
func (s *SQLiteStore) DeleteTrace(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 先删除关联的 spans
	_, err := s.db.Exec("DELETE FROM spans WHERE trace_id = ?", id)
	if err != nil {
		return fmt.Errorf("delete spans: %w", err)
	}

	// 删除 trace
	result, err := s.db.Exec("DELETE FROM traces WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete trace: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("trace not found: %s", id)
	}

	return nil
}

// ListTraces 列出追踪
func (s *SQLiteStore) ListTraces(filter *TraceFilter) ([]*Trace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, session_id, trace_type, name, start_time, end_time,
			duration_ms, status, error, input_tokens, output_tokens,
			total_tokens, input, output, metadata, created_at
		FROM traces
	`

	args := []interface{}{}
	conditions := []string{}

	if filter != nil {
		if filter.SessionID != "" {
			conditions = append(conditions, "session_id = ?")
			args = append(args, filter.SessionID)
		}
		if filter.Status != nil {
			conditions = append(conditions, "status = ?")
			args = append(args, *filter.Status)
		}
		if filter.TraceType != nil {
			conditions = append(conditions, "trace_type = ?")
			args = append(args, *filter.TraceType)
		}
		if filter.StartTime != nil {
			conditions = append(conditions, "start_time >= ?")
			args = append(args, *filter.StartTime)
		}
		if filter.EndTime != nil {
			conditions = append(conditions, "start_time <= ?")
			args = append(args, *filter.EndTime)
		}
	}

	if len(conditions) > 0 {
		query += " WHERE "
		for i, cond := range conditions {
			if i > 0 {
				query += " AND "
			}
			query += cond
		}
	}

	query += " ORDER BY start_time DESC"

	if filter != nil {
		if filter.Limit > 0 {
			query += fmt.Sprintf(" LIMIT %d", filter.Limit)
		}
		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", filter.Offset)
		}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query traces: %w", err)
	}
	defer rows.Close()

	traces := make([]*Trace, 0)
	for rows.Next() {
		trace := &Trace{}
		var metadataJSON []byte
		var endTime sql.NullTime

		err := rows.Scan(
			&trace.ID, &trace.SessionID, &trace.TraceType, &trace.Name, &trace.StartTime, &endTime,
			&trace.DurationMs, &trace.Status, &trace.Error, &trace.InputTokens, &trace.OutputTokens,
			&trace.TotalTokens, &trace.Input, &trace.Output, &metadataJSON, &trace.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan trace: %w", err)
		}

		if endTime.Valid {
			trace.EndTime = endTime.Time
		}

		// 解析 metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &trace.Metadata); err != nil {
				trace.Metadata = nil
			}
		}

		traces = append(traces, trace)
	}

	return traces, nil
}

// CreateSpan 创建 Span
func (s *SQLiteStore) CreateSpan(span *Span) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 生成 ID
	if span.ID == "" {
		span.ID = uuid.New().String()[:8]
	}

	// 设置时间
	now := time.Now()
	if span.StartTime.IsZero() {
		span.StartTime = now
	}
	if span.CreatedAt.IsZero() {
		span.CreatedAt = now
	}

	// 设置默认值
	if span.Status == "" {
		span.Status = StatusRunning
	}

	// 序列化 metadata
	var metadataJSON []byte
	if span.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(span.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}

	query := `
		INSERT INTO spans (
			id, trace_id, parent_id, span_type, name, provider, model,
			start_time, end_time, duration_ms, status, error,
			input_tokens, output_tokens, total_tokens, input, output, metadata, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		span.ID, span.TraceID, span.ParentID, span.SpanType, span.Name, span.Provider, span.Model,
		span.StartTime, span.EndTime, span.DurationMs, span.Status, span.Error,
		span.InputTokens, span.OutputTokens, span.TotalTokens, span.Input, span.Output, metadataJSON, span.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("insert span: %w", err)
	}

	// 发送事件
	s.emitEvent(TraceEvent{
		Type:      EventTypeSpanCreate,
		TraceID:   span.TraceID,
		SpanID:    span.ID,
		Span:      span,
		Timestamp: now,
	})

	return nil
}

// GetSpan 获取 Span
func (s *SQLiteStore) GetSpan(id string) (*Span, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, trace_id, parent_id, span_type, name, provider, model,
			start_time, end_time, duration_ms, status, error,
			input_tokens, output_tokens, total_tokens, input, output, metadata, created_at
		FROM spans WHERE id = ?
	`

	span := &Span{}
	var metadataJSON []byte
	var endTime sql.NullTime
	var parentID, provider, model sql.NullString

	err := s.db.QueryRow(query, id).Scan(
		&span.ID, &span.TraceID, &parentID, &span.SpanType, &span.Name, &provider, &model,
		&span.StartTime, &endTime, &span.DurationMs, &span.Status, &span.Error,
		&span.InputTokens, &span.OutputTokens, &span.TotalTokens, &span.Input, &span.Output, &metadataJSON, &span.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("span not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query span: %w", err)
	}

	if endTime.Valid {
		span.EndTime = endTime.Time
	}
	if parentID.Valid {
		span.ParentID = parentID.String
	}
	if provider.Valid {
		span.Provider = provider.String
	}
	if model.Valid {
		span.Model = model.String
	}

	// 解析 metadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &span.Metadata); err != nil {
			span.Metadata = nil
		}
	}

	return span, nil
}

// UpdateSpan 更新 Span
func (s *SQLiteStore) UpdateSpan(span *Span) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 计算耗时
	if !span.EndTime.IsZero() {
		span.DurationMs = span.EndTime.Sub(span.StartTime).Milliseconds()
	}

	// 计算总 token
	span.TotalTokens = span.InputTokens + span.OutputTokens

	// 序列化 metadata
	var metadataJSON []byte
	if span.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(span.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}

	query := `
		UPDATE spans SET
			parent_id = ?, span_type = ?, name = ?, provider = ?, model = ?,
			start_time = ?, end_time = ?, duration_ms = ?, status = ?, error = ?,
			input_tokens = ?, output_tokens = ?, total_tokens = ?, input = ?, output = ?, metadata = ?
		WHERE id = ?
	`

	result, err := s.db.Exec(query,
		span.ParentID, span.SpanType, span.Name, span.Provider, span.Model,
		span.StartTime, span.EndTime, span.DurationMs, span.Status, span.Error,
		span.InputTokens, span.OutputTokens, span.TotalTokens, span.Input, span.Output, metadataJSON,
		span.ID,
	)

	if err != nil {
		return fmt.Errorf("update span: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("span not found: %s", span.ID)
	}

	// 发送事件
	eventType := EventTypeSpanUpdate
	if span.Status != StatusRunning {
		eventType = EventTypeSpanComplete
	}
	s.emitEvent(TraceEvent{
		Type:      eventType,
		TraceID:   span.TraceID,
		SpanID:    span.ID,
		Span:      span,
		Timestamp: time.Now(),
	})

	return nil
}

// DeleteSpan 删除 Span
func (s *SQLiteStore) DeleteSpan(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec("DELETE FROM spans WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete span: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("span not found: %s", id)
	}

	return nil
}

// ListSpans 列出 Spans
func (s *SQLiteStore) ListSpans(filter *SpanFilter) ([]*Span, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, trace_id, parent_id, span_type, name, provider, model,
			start_time, end_time, duration_ms, status, error,
			input_tokens, output_tokens, total_tokens, input, output, metadata, created_at
		FROM spans
	`

	args := []interface{}{}
	conditions := []string{}

	if filter != nil {
		if filter.TraceID != "" {
			conditions = append(conditions, "trace_id = ?")
			args = append(args, filter.TraceID)
		}
		if filter.SpanType != nil {
			conditions = append(conditions, "span_type = ?")
			args = append(args, *filter.SpanType)
		}
		if filter.Status != nil {
			conditions = append(conditions, "status = ?")
			args = append(args, *filter.Status)
		}
	}

	if len(conditions) > 0 {
		query += " WHERE "
		for i, cond := range conditions {
			if i > 0 {
				query += " AND "
			}
			query += cond
		}
	}

	query += " ORDER BY start_time ASC"

	if filter != nil {
		if filter.Limit > 0 {
			query += fmt.Sprintf(" LIMIT %d", filter.Limit)
		}
		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", filter.Offset)
		}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query spans: %w", err)
	}
	defer rows.Close()

	spans := make([]*Span, 0)
	for rows.Next() {
		span := &Span{}
		var metadataJSON []byte
		var endTime sql.NullTime
		var parentID, provider, model sql.NullString

		err := rows.Scan(
			&span.ID, &span.TraceID, &parentID, &span.SpanType, &span.Name, &provider, &model,
			&span.StartTime, &endTime, &span.DurationMs, &span.Status, &span.Error,
			&span.InputTokens, &span.OutputTokens, &span.TotalTokens, &span.Input, &span.Output, &metadataJSON, &span.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan span: %w", err)
		}

		if endTime.Valid {
			span.EndTime = endTime.Time
		}
		if parentID.Valid {
			span.ParentID = parentID.String
		}
		if provider.Valid {
			span.Provider = provider.String
		}
		if model.Valid {
			span.Model = model.String
		}

		// 解析 metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &span.Metadata); err != nil {
				span.Metadata = nil
			}
		}

		spans = append(spans, span)
	}

	return spans, nil
}

// GetSpansByTrace 获取 Trace 的所有 Spans
func (s *SQLiteStore) GetSpansByTrace(traceID string) ([]*Span, error) {
	return s.ListSpans(&SpanFilter{TraceID: traceID})
}

// GetTraceStats 获取统计信息
func (s *SQLiteStore) GetTraceStats() (*TraceStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &TraceStats{
		ByStatus:   make(map[string]int),
		ByType:     make(map[string]int),
		ByProvider: make(map[string]int),
		ByModel:    make(map[string]int),
	}

	// 总 trace 数
	err := s.db.QueryRow("SELECT COUNT(*) FROM traces").Scan(&stats.TotalTraces)
	if err != nil {
		return nil, fmt.Errorf("count traces: %w", err)
	}

	// 总 span 数
	err = s.db.QueryRow("SELECT COUNT(*) FROM spans").Scan(&stats.TotalSpans)
	if err != nil {
		return nil, fmt.Errorf("count spans: %w", err)
	}

	// Token 统计
	err = s.db.QueryRow("SELECT COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(total_tokens), 0) FROM traces").Scan(
		&stats.InputTokens, &stats.OutputTokens, &stats.TotalTokens,
	)
	if err != nil {
		return nil, fmt.Errorf("count tokens: %w", err)
	}

	// 平均耗时
	err = s.db.QueryRow("SELECT COALESCE(AVG(duration_ms), 0) FROM traces WHERE status != 'running'").Scan(&stats.AvgDurationMs)
	if err != nil {
		return nil, fmt.Errorf("avg duration: %w", err)
	}

	// 成功率
	var successCount, totalCompleted int
	err = s.db.QueryRow("SELECT COUNT(*) FROM traces WHERE status = 'success'").Scan(&successCount)
	if err != nil {
		return nil, fmt.Errorf("count success: %w", err)
	}
	err = s.db.QueryRow("SELECT COUNT(*) FROM traces WHERE status != 'running'").Scan(&totalCompleted)
	if err != nil {
		return nil, fmt.Errorf("count completed: %w", err)
	}
	if totalCompleted > 0 {
		stats.SuccessRate = float64(successCount) / float64(totalCompleted) * 100
	}

	// 按状态分组
	rows, err := s.db.Query("SELECT status, COUNT(*) FROM traces GROUP BY status")
	if err != nil {
		return nil, fmt.Errorf("count by status: %w", err)
	}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err == nil {
			stats.ByStatus[status] = count
		}
	}
	rows.Close()

	// 按类型分组
	rows, err = s.db.Query("SELECT trace_type, COUNT(*) FROM traces GROUP BY trace_type")
	if err != nil {
		return nil, fmt.Errorf("count by type: %w", err)
	}
	for rows.Next() {
		var traceType string
		var count int
		if err := rows.Scan(&traceType, &count); err == nil {
			stats.ByType[traceType] = count
		}
	}
	rows.Close()

	// 按提供商分组（从 spans）
	rows, err = s.db.Query("SELECT provider, COUNT(*) FROM spans WHERE provider IS NOT NULL AND provider != '' GROUP BY provider")
	if err != nil {
		return nil, fmt.Errorf("count by provider: %w", err)
	}
	for rows.Next() {
		var provider string
		var count int
		if err := rows.Scan(&provider, &count); err == nil {
			stats.ByProvider[provider] = count
		}
	}
	rows.Close()

	// 按模型分组（从 spans）
	rows, err = s.db.Query("SELECT model, COUNT(*) FROM spans WHERE model IS NOT NULL AND model != '' GROUP BY model")
	if err != nil {
		return nil, fmt.Errorf("count by model: %w", err)
	}
	for rows.Next() {
		var model string
		var count int
		if err := rows.Scan(&model, &count); err == nil {
			stats.ByModel[model] = count
		}
	}
	rows.Close()

	return stats, nil
}

// CleanupOldTraces 清理旧追踪
func (s *SQLiteStore) CleanupOldTraces(days int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -days)

	// 先删除关联的 spans
	_, err := s.db.Exec(`
		DELETE FROM spans WHERE trace_id IN (
			SELECT id FROM traces WHERE created_at < ?
		)
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old spans: %w", err)
	}

	// 删除 traces
	result, err := s.db.Exec("DELETE FROM traces WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old traces: %w", err)
	}

	return result.RowsAffected()
}

// Subscribe 订阅事件
func (s *SQLiteStore) Subscribe() <-chan TraceEvent {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()

	ch := make(chan TraceEvent, 100)
	s.eventChs = append(s.eventChs, ch)
	return ch
}

// emitEvent 发送事件
func (s *SQLiteStore) emitEvent(event TraceEvent) {
	s.eventMu.RLock()
	defer s.eventMu.RUnlock()

	for _, ch := range s.eventChs {
		select {
		case ch <- event:
		default:
			// 通道满，丢弃
		}
	}
}

// Close 关闭存储
func (s *SQLiteStore) Close() error {
	s.eventMu.Lock()
	for _, ch := range s.eventChs {
		close(ch)
	}
	s.eventChs = nil
	s.eventMu.Unlock()

	return s.db.Close()
}
