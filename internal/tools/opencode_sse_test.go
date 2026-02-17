package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestOpenCodeSSE_Events(t *testing.T) {
	client := NewOpenCodeClient(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check server health
	healthy, _, err := client.Health(ctx)
	if err != nil || !healthy {
		t.Skipf("OpenCode server not available: %v", err)
		return
	}

	// Create session
	session, err := client.CreateSession(ctx, "SSE Test Session")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	defer client.DeleteSession(ctx, session.ID)

	// Subscribe to events (no session filter)
	eventChan, cancelSub, err := client.SubscribeEvents(ctx, "")
	if err != nil {
		t.Fatalf("SubscribeEvents failed: %v", err)
	}

	// Collect events in background
	var events []SSEEvent
	go func() {
		for event := range eventChan {
			events = append(events, event)
		}
	}()

	// Send a simple message that triggers tool use
	opts := SendMessageOptions{
		Agent: "build",
		Parts: []MessagePart{
			{Type: "text", Text: "List files in current directory"},
		},
	}
	_, err = client.SendMessage(ctx, session.ID, opts)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Wait for events
	time.Sleep(500 * time.Millisecond)
	cancelSub()
	time.Sleep(100 * time.Millisecond)

	t.Logf("Collected %d events", len(events))

	// Print all event types and their properties
	for _, e := range events {
		propsJSON, _ := json.Marshal(e.Properties)
		t.Logf("  %s: %s", e.Type, string(propsJSON))
	}

	if len(events) == 0 {
		t.Error("No events collected")
	}
}

func TestFormatEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    SSEEvent
		expected string
	}{
		{
			name: "tool running",
			event: SSEEvent{Type: "message.part.updated", Properties: map[string]interface{}{
				"part": map[string]interface{}{
					"type":  "tool",
					"tool":  "bash",
					"state": map[string]interface{}{"status": "running"},
				},
			}},
			expected: "  ⚙️ 执行: bash",
		},
		{
			name: "tool completed",
			event: SSEEvent{Type: "message.part.updated", Properties: map[string]interface{}{
				"part": map[string]interface{}{
					"type":  "tool",
					"tool":  "read",
					"state": map[string]interface{}{"status": "completed"},
				},
			}},
			expected: "  ✓ 完成: read",
		},
		{
			name: "step start",
			event: SSEEvent{Type: "message.part.updated", Properties: map[string]interface{}{
				"part": map[string]interface{}{"type": "step-start"},
			}},
			expected: "  🔄 开始处理...",
		},
		{
			name: "session idle",
			event: SSEEvent{Type: "session.idle", Properties: map[string]interface{}{
				"sessionID": "test",
			}},
			expected: "  ✅ 任务完成",
		},
		{
			name:     "unknown event",
			event:    SSEEvent{Type: "unknown", Properties: map[string]interface{}{}},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatEvent(tt.event)
			if result != tt.expected {
				t.Errorf("FormatEvent() = %q, want %q", result, tt.expected)
			}
		})
	}
}
