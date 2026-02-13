package memory

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreAddAndGet(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// 添加消息
	msg1 := &Message{
		ID:      "msg1",
		Role:    "user",
		Content: "Hello",
	}
	err := store.Add(ctx, "session1", msg1)
	if err != nil {
		t.Fatalf("Failed to add message: %v", err)
	}

	msg2 := &Message{
		ID:      "msg2",
		Role:    "assistant",
		Content: "Hi there!",
	}
	err = store.Add(ctx, "session1", msg2)
	if err != nil {
		t.Fatalf("Failed to add message: %v", err)
	}

	// 获取消息
	messages, err := store.Get(ctx, "session1", 0)
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
}

func TestMemoryStoreLimit(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// 添加 5 条消息
	for i := 0; i < 5; i++ {
		msg := &Message{
			ID:      string(rune('a' + i)),
			Role:    "user",
			Content: string(rune('0' + i)),
		}
		store.Add(ctx, "session1", msg)
	}

	// 限制获取 3 条
	messages, err := store.Get(ctx, "session1", 3)
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages with limit, got %d", len(messages))
	}

	// 应该返回最后 3 条
	if messages[0].Content != "2" {
		t.Errorf("Expected first limited message content='2', got %s", messages[0].Content)
	}
}

func TestMemoryStoreClear(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// 添加消息
	msg := &Message{
		ID:      "msg1",
		Role:    "user",
		Content: "Hello",
	}
	store.Add(ctx, "session1", msg)

	// 清除
	err := store.Clear(ctx, "session1")
	if err != nil {
		t.Fatalf("Failed to clear session: %v", err)
	}

	// 验证已清除
	messages, _ := store.Get(ctx, "session1", 0)
	if len(messages) != 0 {
		t.Errorf("Expected 0 messages after clear, got %d", len(messages))
	}
}

func TestMemoryStoreMultipleSessions(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// 添加到不同会话
	store.Add(ctx, "session1", &Message{ID: "1", Role: "user", Content: "S1-1"})
	store.Add(ctx, "session2", &Message{ID: "2", Role: "user", Content: "S2-1"})
	store.Add(ctx, "session1", &Message{ID: "3", Role: "user", Content: "S1-2"})

	// 验证会话隔离
	msgs1, _ := store.Get(ctx, "session1", 0)
	msgs2, _ := store.Get(ctx, "session2", 0)

	if len(msgs1) != 2 {
		t.Errorf("Expected session1 to have 2 messages, got %d", len(msgs1))
	}

	if len(msgs2) != 1 {
		t.Errorf("Expected session2 to have 1 message, got %d", len(msgs2))
	}
}

func TestMemoryStoreTimestamp(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	before := time.Now()

	msg := &Message{
		ID:      "msg1",
		Role:    "user",
		Content: "Hello",
	}
	store.Add(ctx, "session1", msg)

	after := time.Now()

	messages, _ := store.Get(ctx, "session1", 0)
	if messages[0].Timestamp.Before(before) || messages[0].Timestamp.After(after) {
		t.Error("Timestamp should be set automatically")
	}
}

func TestMemoryStoreClose(t *testing.T) {
	store := NewMemoryStore()
	err := store.Close()
	if err != nil {
		t.Errorf("Close should not return error, got %v", err)
	}
}
