// Package logger 日志工具
package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Level 日志级别
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var levelNames = map[Level]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
}

// Logger 日志器
type Logger struct {
	mu       sync.Mutex
	level    Level
	format   string
	file     *os.File
	filePath string
}

var defaultLogger *Logger

// Init 初始化日志器
func Init(level, format, output string) error {
	l := &Logger{
		level:  parseLevel(level),
		format: format,
	}

	if output != "" {
		// 展开路径
		output = expandPath(output)

		// 创建目录
		dir := filepath.Dir(output)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		// 打开日志文件
		f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		l.file = f
		l.filePath = output
	}

	defaultLogger = l
	return nil
}

// GetLogger 获取默认日志器
func GetLogger() *Logger {
	if defaultLogger == nil {
		// 默认输出到 stdout
		defaultLogger = &Logger{
			level:  LevelInfo,
			format: "text",
		}
	}
	return defaultLogger
}

// Debug 调试日志
func Debug(msg string, fields ...interface{}) {
	GetLogger().log(LevelDebug, msg, fields...)
}

// Info 信息日志
func Info(msg string, fields ...interface{}) {
	GetLogger().log(LevelInfo, msg, fields...)
}

// Warn 警告日志
func Warn(msg string, fields ...interface{}) {
	GetLogger().log(LevelWarn, msg, fields...)
}

// Error 错误日志
func Error(msg string, fields ...interface{}) {
	GetLogger().log(LevelError, msg, fields...)
}

// LLMRequest 记录 LLM 请求
func LLMRequest(provider, model string, req interface{}) {
	fields := []interface{}{
		"provider", provider,
		"model", model,
		"request", toJSON(req),
	}
	GetLogger().log(LevelInfo, "[LLM] Request", fields...)
}

// LLMResponse 记录 LLM 响应
func LLMResponse(provider, model string, resp interface{}, duration time.Duration, err error) {
	level := LevelInfo
	msg := "[LLM] Response"
	if err != nil {
		level = LevelError
		msg = "[LLM] Response Error"
	}

	fields := []interface{}{
		"provider", provider,
		"model", model,
		"duration_ms", duration.Milliseconds(),
	}
	if resp != nil {
		fields = append(fields, "response", toJSON(resp))
	}
	if err != nil {
		fields = append(fields, "error", err.Error())
	}

	GetLogger().log(level, msg, fields...)
}

// ToolCall 记录工具调用
func ToolCall(toolName string, params interface{}, result string, duration time.Duration, err error) {
	level := LevelInfo
	msg := "[Tool] Call"
	if err != nil {
		level = LevelError
		msg = "[Tool] Call Error"
	}

	fields := []interface{}{
		"tool", toolName,
		"params", toJSON(params),
		"duration_ms", duration.Milliseconds(),
		"result", truncate(result, 500),
	}
	if err != nil {
		fields = append(fields, "error", err.Error())
	}

	GetLogger().log(level, msg, fields...)
}

// AgentMessage 记录 Agent 消息
func AgentMessage(sessionID, role, content string) {
	GetLogger().log(LevelDebug, "[Agent] Message",
		"session", sessionID,
		"role", role,
		"content", truncate(content, 200),
	)
}

func (l *Logger) log(level Level, msg string, fields ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().Format("2006-01-02 15:04:05.000")
	levelName := levelNames[level]

	var output string
	if l.format == "json" {
		output = l.formatJSON(now, levelName, msg, fields)
	} else {
		output = l.formatText(now, levelName, msg, fields)
	}

	// 只输出到文件，不输出到标准输出
	if l.file != nil {
		l.file.WriteString(output + "\n")
	}
}

func (l *Logger) formatText(time, level, msg string, fields []interface{}) string {
	result := fmt.Sprintf("%s [%s] %s", time, level, msg)
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			result += fmt.Sprintf(" %s=%v", fields[i], fields[i+1])
		}
	}
	return result
}

func (l *Logger) formatJSON(time, level, msg string, fields []interface{}) string {
	data := map[string]interface{}{
		"time":    time,
		"level":   level,
		"message": msg,
	}
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			data[fmt.Sprint(fields[i])] = fields[i+1]
		}
	}
	return toJSON(data)
}

// Close 关闭日志器
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func parseLevel(level string) Level {
	switch level {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

func toJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
