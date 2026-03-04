package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionStore_AddAndGet(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	store := NewSessionStore(tmpDir)
	defer store.Close()

	ctx := context.Background()
	sessionID := "test-session-123"

	// 添加消息
	msg1 := &Message{
		Role:    "user",
		Content: "Hello",
	}
	err := store.Add(ctx, sessionID, msg1)
	if err != nil {
		t.Fatalf("Failed to add message: %v", err)
	}

	msg2 := &Message{
		Role:    "assistant",
		Content: "Hi there!",
	}
	err = store.Add(ctx, sessionID, msg2)
	if err != nil {
		t.Fatalf("Failed to add message: %v", err)
	}

	// 获取消息
	messages, err := store.Get(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}

	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}

	if messages[0].Content != "Hello" {
		t.Errorf("Expected first message content='Hello', got %s", messages[0].Content)
	}

	if messages[1].Role != "assistant" {
		t.Errorf("Expected second message role=assistant, got %s", messages[1].Role)
	}

	// 验证文件已创建
	filePath := filepath.Join(tmpDir, "sessions", "test-session-123.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("Session file not created: %s", filePath)
	}
}

func TestSessionStore_Limit(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewSessionStore(tmpDir)
	defer store.Close()

	ctx := context.Background()
	sessionID := "limit-test"

	// 添加 5 条消息
	for i := 0; i < 5; i++ {
		msg := &Message{
			Role:    "user",
			Content: string(rune('A' + i)),
		}
		store.Add(ctx, sessionID, msg)
	}

	// 限制获取 3 条
	messages, err := store.Get(ctx, sessionID, 3)
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages with limit, got %d", len(messages))
	}

	// 应该返回最后 3 条 (C, D, E)
	if messages[0].Content != "C" {
		t.Errorf("Expected first limited message content='C', got %s", messages[0].Content)
	}
}

func TestSessionStore_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewSessionStore(tmpDir)
	defer store.Close()

	ctx := context.Background()
	sessionID := "clear-test"

	// 添加消息
	msg := &Message{
		Role:    "user",
		Content: "Test message",
	}
	store.Add(ctx, sessionID, msg)

	// 验证文件存在
	filePath := filepath.Join(tmpDir, "sessions", "clear-test.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("Session file not created")
	}

	// 清除会话
	err := store.Clear(ctx, sessionID)
	if err != nil {
		t.Fatalf("Failed to clear session: %v", err)
	}

	// 验证文件已删除
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("Session file should be deleted after clear")
	}

	// 获取消息应该返回空
	messages, err := store.Get(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("Failed to get messages after clear: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("Expected 0 messages after clear, got %d", len(messages))
	}
}

func TestSessionStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	sessionID := "persistence-test"

	// 创建第一个 store 实例
	store1 := NewSessionStore(tmpDir)

	msg := &Message{
		Role:    "user",
		Content: "Persistent message",
	}
	store1.Add(ctx, sessionID, msg)
	store1.Close()

	// 创建第二个 store 实例，验证持久化
	store2 := NewSessionStore(tmpDir)
	defer store2.Close()

	messages, err := store2.Get(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("Failed to get messages from new store: %v", err)
	}

	if len(messages) != 1 {
		t.Errorf("Expected 1 message from persisted store, got %d", len(messages))
	}

	if messages[0].Content != "Persistent message" {
		t.Errorf("Expected persisted content='Persistent message', got %s", messages[0].Content)
	}
}

func TestSessionStore_MultipleSessions(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewSessionStore(tmpDir)
	defer store.Close()

	ctx := context.Background()

	// 添加到不同会话
	store.Add(ctx, "session-A", &Message{Role: "user", Content: "A1"})
	store.Add(ctx, "session-B", &Message{Role: "user", Content: "B1"})
	store.Add(ctx, "session-A", &Message{Role: "user", Content: "A2"})

	// 验证会话隔离
	msgsA, _ := store.Get(ctx, "session-A", 0)
	msgsB, _ := store.Get(ctx, "session-B", 0)

	if len(msgsA) != 2 {
		t.Errorf("Expected session-A to have 2 messages, got %d", len(msgsA))
	}

	if len(msgsB) != 1 {
		t.Errorf("Expected session-B to have 1 message, got %d", len(msgsB))
	}

	// 列出所有会话
	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(sessions))
	}
}

func TestSessionStore_Timestamp(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewSessionStore(tmpDir)
	defer store.Close()

	ctx := context.Background()

	before := time.Now()

	msg := &Message{
		Role:    "user",
		Content: "Test",
	}
	store.Add(ctx, "timestamp-test", msg)

	after := time.Now()

	messages, _ := store.Get(ctx, "timestamp-test", 0)
	if messages[0].Timestamp.Before(before) || messages[0].Timestamp.After(after) {
		t.Error("Timestamp should be set automatically within test time range")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple-test", "simple-test"},
		{"feishu-oc_xxx", "feishu-oc_xxx"},
		{"session/with/slashes", "session_with_slashes"},
		{"session:with:colons", "session_with_colons"},
		{"session with spaces", "session_with_spaces"},
		{"session@#$%special", "session____special"},
	}

	for _, test := range tests {
		result := sanitizeFilename(test.input)
		if result != test.expected {
			t.Errorf("sanitizeFilename(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}
