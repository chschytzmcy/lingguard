// Package memory 记忆系统 - 基于 nanobot 的文件持久化方案
package memory

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// FileStore 基于文件的持久化存储（参考 nanobot）
// 使用 MEMORY.md 存储长期记忆，HISTORY.md 存储事件日志
type FileStore struct {
	memoryDir       string // 记忆目录路径
	memoryFile      string // MEMORY.md 文件路径
	historyFile     string // HISTORY.md 文件路径
	maxHistoryLines int    // 历史记录最大行数
}

// NewFileStore 创建文件存储
func NewFileStore(memoryDir string) *FileStore {
	return &FileStore{
		memoryDir:       memoryDir,
		memoryFile:      filepath.Join(memoryDir, "MEMORY.md"),
		historyFile:     filepath.Join(memoryDir, "HISTORY.md"),
		maxHistoryLines: 1000,
	}
}

// Init 初始化存储目录和文件
func (s *FileStore) Init() error {
	// 展开路径中的 ~
	expandedDir := expandHome(s.memoryDir)
	if expandedDir != s.memoryDir {
		s.memoryDir = expandedDir
		s.memoryFile = filepath.Join(expandedDir, "MEMORY.md")
		s.historyFile = filepath.Join(expandedDir, "HISTORY.md")
	}

	if err := os.MkdirAll(s.memoryDir, 0755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}

	// 初始化 MEMORY.md（如果不存在）
	if _, err := os.Stat(s.memoryFile); os.IsNotExist(err) {
		initialContent := `# Memory

This file stores long-term memories and important facts.

## User Preferences
<!-- 用户偏好设置 -->

## Project Context
<!-- 项目上下文信息 -->

## Important Facts
<!-- 重要事实记录 -->
`
		if err := os.WriteFile(s.memoryFile, []byte(initialContent), 0644); err != nil {
			return fmt.Errorf("create memory file: %w", err)
		}
	}

	// 初始化 HISTORY.md（如果不存在）
	if _, err := os.Stat(s.historyFile); os.IsNotExist(err) {
		header := `# History

This file records events and conversations in chronological order.

---
`
		if err := os.WriteFile(s.historyFile, []byte(header), 0644); err != nil {
			return fmt.Errorf("create history file: %w", err)
		}
	}

	return nil
}

// Memory operations

// AddMemory 添加长期记忆到 MEMORY.md
func (s *FileStore) AddMemory(category, content string) error {
	if err := s.ensureInit(); err != nil {
		return err
	}

	// 读取现有内容
	data, err := os.ReadFile(s.memoryFile)
	if err != nil {
		return fmt.Errorf("read memory file: %w", err)
	}

	contentStr := string(data)
	timestamp := time.Now().Format("2006-01-02 15:04")

	// 查找或创建分类
	categoryHeader := fmt.Sprintf("## %s", category)
	newEntry := fmt.Sprintf("- [%s] %s", timestamp, content)

	if strings.Contains(contentStr, categoryHeader) {
		// 在分类后添加（跳过注释行）
		lines := strings.Split(contentStr, "\n")
		var newLines []string
		inserted := false

		for i := 0; i < len(lines); i++ {
			line := lines[i]
			newLines = append(newLines, line)

			if !inserted && strings.HasPrefix(strings.TrimSpace(line), categoryHeader) {
				// 找到分类标题，跳过后续的注释行
				for i+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i+1]), "<!--") {
					i++
					newLines = append(newLines, lines[i])
				}
				// 插入新条目
				newLines = append(newLines, newEntry)
				inserted = true
			}
		}
		contentStr = strings.Join(newLines, "\n")
	} else {
		// 添加新分类
		contentStr = contentStr + fmt.Sprintf("\n\n## %s\n%s", category, newEntry)
	}

	return os.WriteFile(s.memoryFile, []byte(contentStr), 0644)
}

// GetMemory 获取 MEMORY.md 全部内容
func (s *FileStore) GetMemory() (string, error) {
	if err := s.ensureInit(); err != nil {
		return "", err
	}

	data, err := os.ReadFile(s.memoryFile)
	if err != nil {
		return "", fmt.Errorf("read memory file: %w", err)
	}
	return string(data), nil
}

// SearchMemory 搜索记忆（使用 Go 原生实现，避免命令注入）
func (s *FileStore) SearchMemory(query string) ([]string, error) {
	if err := s.ensureInit(); err != nil {
		return nil, err
	}

	// 读取文件内容
	data, err := os.ReadFile(s.memoryFile)
	if err != nil {
		return nil, fmt.Errorf("read memory file: %w", err)
	}

	return grepSearch(string(data), query)
}

// History operations

// AddHistory 添加历史记录到 HISTORY.md
func (s *FileStore) AddHistory(eventType, summary string, details map[string]string) error {
	if err := s.ensureInit(); err != nil {
		return err
	}

	// 构建条目
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	var entry strings.Builder
	entry.WriteString(fmt.Sprintf("\n### [%s] %s\n", timestamp, eventType))
	entry.WriteString(fmt.Sprintf("%s\n", summary))

	// 添加详细信息
	if len(details) > 0 {
		for k, v := range details {
			entry.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
		}
	}
	entry.WriteString("\n---\n")

	// 追加到文件（使用 bufio 提升性能）
	f, err := os.OpenFile(s.historyFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	if _, err := writer.WriteString(entry.String()); err != nil {
		return fmt.Errorf("write history: %w", err)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush history: %w", err)
	}

	// 可选：清理过旧的历史
	return s.trimHistory()
}

// GetRecentHistory 获取最近的历史记录
func (s *FileStore) GetRecentHistory(lines int) ([]string, error) {
	if err := s.ensureInit(); err != nil {
		return nil, err
	}

	f, err := os.Open(s.historyFile)
	if err != nil {
		return nil, fmt.Errorf("open history file: %w", err)
	}
	defer f.Close()

	// 使用 tail 方式读取最后 N 行
	var result []string
	scanner := bufio.NewScanner(f)
	allLines := []string{}
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	start := len(allLines) - lines
	if start < 0 {
		start = 0
	}
	result = allLines[start:]

	return result, nil
}

// SearchHistory 搜索历史记录（使用 Go 原生实现）
func (s *FileStore) SearchHistory(query string) ([]string, error) {
	if err := s.ensureInit(); err != nil {
		return nil, err
	}

	// 读取文件内容
	data, err := os.ReadFile(s.historyFile)
	if err != nil {
		return nil, fmt.Errorf("read history file: %w", err)
	}

	return grepSearch(string(data), query)
}

// Session operations (实现 Store 接口)

// Add 添加会话消息
func (s *FileStore) Add(ctx context.Context, sessionID string, msg *Message) error {
	if err := s.ensureInit(); err != nil {
		return err
	}

	// 记录到历史
	details := map[string]string{
		"session_id": sessionID,
		"role":       msg.Role,
	}
	if len(msg.Metadata) > 0 {
		for k, v := range msg.Metadata {
			details[k] = fmt.Sprintf("%v", v)
		}
	}

	return s.AddHistory(fmt.Sprintf("Message/%s", msg.Role), msg.Content, details)
}

// Get 获取会话消息（从历史中提取，使用 Go 原生实现）
func (s *FileStore) Get(ctx context.Context, sessionID string, limit int) ([]*Message, error) {
	if err := s.ensureInit(); err != nil {
		return nil, err
	}

	// 读取文件内容
	data, err := os.ReadFile(s.historyFile)
	if err != nil {
		return nil, fmt.Errorf("read history file: %w", err)
	}

	// 搜索包含 session_id 的行及其下一行
	sessionPattern := fmt.Sprintf("session_id: %s", sessionID)
	var messages []*Message
	lines := strings.Split(string(data), "\n")

	for i, line := range lines {
		if strings.Contains(line, sessionPattern) {
			// 找到匹配，检查上下文寻找消息类型
			for j := max(0, i-5); j < min(len(lines), i+3); j++ {
				if strings.Contains(lines[j], "Message/") {
					msg := &Message{
						Timestamp: time.Now(),
					}
					if strings.Contains(lines[j], "user") {
						msg.Role = "user"
					} else if strings.Contains(lines[j], "assistant") {
						msg.Role = "assistant"
					}
					if msg.Role != "" {
						messages = append(messages, msg)
					}
				}
			}
		}
	}

	// 限制数量
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}

	return messages, nil
}

// Clear 清除会话
func (s *FileStore) Clear(ctx context.Context, sessionID string) error {
	// 文件存储不真正清除，只记录清除事件
	return s.AddHistory("Session Clear", fmt.Sprintf("Session %s cleared", sessionID), map[string]string{
		"session_id": sessionID,
	})
}

// Close 关闭存储
func (s *FileStore) Close() error {
	return nil
}

// Helper methods

func (s *FileStore) ensureInit() error {
	if _, err := os.Stat(s.memoryDir); os.IsNotExist(err) {
		return s.Init()
	}
	return nil
}

func (s *FileStore) trimHistory() error {
	// 检查历史文件大小
	info, err := os.Stat(s.historyFile)
	if err != nil {
		return err
	}

	// 如果超过限制，保留最新的行
	if info.Size() > int64(s.maxHistoryLines*100) { // 假设平均每行100字节
		f, err := os.Open(s.historyFile)
		if err != nil {
			return err
		}
		defer f.Close()

		var allLines []string
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			allLines = append(allLines, scanner.Text())
		}

		// 保留头部和最新的行
		header := allLines[:10] // 保留头部
		recent := allLines[len(allLines)-s.maxHistoryLines:]

		var newContent strings.Builder
		for _, line := range header {
			newContent.WriteString(line + "\n")
		}
		newContent.WriteString("\n... (older entries trimmed) ...\n\n")
		for _, line := range recent {
			newContent.WriteString(line + "\n")
		}

		return os.WriteFile(s.historyFile, []byte(newContent.String()), 0644)
	}

	return nil
}

// Daily log methods (参考 nanobot 的每日日志)

// WriteDailyLog 写入每日日志
func (s *FileStore) WriteDailyLog(content string) error {
	today := time.Now().Format("2006-01-02")
	dailyFile := filepath.Join(s.memoryDir, today+".md")

	// 检查文件是否存在，如果不存在则创建带日期头的文件
	var f *os.File
	if _, err := os.Stat(dailyFile); os.IsNotExist(err) {
		f, err = os.Create(dailyFile)
		if err != nil {
			return err
		}
		header := fmt.Sprintf("# Daily Log - %s\n\n", today)
		f.WriteString(header)
	} else {
		f, err = os.OpenFile(dailyFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
	}
	defer f.Close()

	timestamp := time.Now().Format("15:04:05")
	_, writeErr := f.WriteString(fmt.Sprintf("\n## [%s]\n%s\n", timestamp, content))
	return writeErr
}

// GetRecentDailyLogs 获取最近几天的日志
func (s *FileStore) GetRecentDailyLogs(days int) (map[string]string, error) {
	logs := make(map[string]string)

	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		dailyFile := filepath.Join(s.memoryDir, date+".md")

		if data, err := os.ReadFile(dailyFile); err == nil {
			logs[date] = string(data)
		}
	}

	return logs, nil
}

// SearchAll 搜索所有记忆文件（使用 Go 原生实现）
func (s *FileStore) SearchAll(query string) (map[string][]string, error) {
	results := make(map[string][]string)

	// 搜索 MEMORY.md
	if memResults, err := s.SearchMemory(query); err == nil && len(memResults) > 0 {
		results["MEMORY.md"] = memResults
	}

	// 搜索 HISTORY.md
	if histResults, err := s.SearchHistory(query); err == nil && len(histResults) > 0 {
		results["HISTORY.md"] = histResults
	}

	// 搜索每日日志文件
	entries, err := os.ReadDir(s.memoryDir)
	if err != nil {
		return results, nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fileName := entry.Name()
		// 只搜索 .md 文件，排除 MEMORY.md 和 HISTORY.md（已搜索）
		if strings.HasSuffix(fileName, ".md") && fileName != "MEMORY.md" && fileName != "HISTORY.md" {
			filePath := filepath.Join(s.memoryDir, fileName)
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			if lines, err := grepSearch(string(data), query); err == nil && len(lines) > 0 {
				results[fileName] = lines
			}
		}
	}

	return results, nil
}

// Compile-time interface check
var _ Store = (*FileStore)(nil)
var _ io.Closer = (*FileStore)(nil)

// expandHome 展开 ~ 为用户主目录
func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// grepSearch 模拟 grep -i -n 的搜索功能（Go 原生实现，避免命令注入）
func grepSearch(content, query string) ([]string, error) {
	if query == "" {
		return nil, nil
	}

	// 转义正则特殊字符，进行安全的字符串匹配
	pattern := "(?i)" + regexp.QuoteMeta(query)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid search pattern: %w", err)
	}

	var results []string
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if re.MatchString(line) {
			// 格式：行号:匹配内容（类似 grep -n 输出）
			results = append(results, fmt.Sprintf("%d:%s", i+1, line))
		}
	}

	return results, nil
}

// max 返回两个整数中的较大值
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
