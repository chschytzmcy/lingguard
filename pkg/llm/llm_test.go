package llm

import (
	"encoding/json"
	"testing"
)

func TestMessageJSON(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello, world!",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	var unmarshaled Message
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if unmarshaled.Role != "user" {
		t.Errorf("Expected role=user, got %s", unmarshaled.Role)
	}

	if unmarshaled.Content != "Hello, world!" {
		t.Errorf("Expected content='Hello, world!', got %s", unmarshaled.Content)
	}
}

func TestToolCallJSON(t *testing.T) {
	tc := ToolCall{
		ID:   "call_123",
		Type: "function",
		Function: FunctionCall{
			Name:      "shell",
			Arguments: json.RawMessage(`{"command":"ls"}`),
		},
	}

	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("Failed to marshal tool call: %v", err)
	}

	var unmarshaled ToolCall
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal tool call: %v", err)
	}

	if unmarshaled.ID != "call_123" {
		t.Errorf("Expected ID=call_123, got %s", unmarshaled.ID)
	}

	if unmarshaled.Function.Name != "shell" {
		t.Errorf("Expected function name=shell, got %s", unmarshaled.Function.Name)
	}
}

func TestResponseToMessage(t *testing.T) {
	resp := &Response{
		ID:    "resp_123",
		Model: "gpt-4o",
		Choices: []struct {
			Index        int     `json:"index"`
			Message      Message `json:"message"`
			FinishReason string  `json:"finish_reason"`
		}{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: "Hello!",
				},
				FinishReason: "stop",
			},
		},
	}

	msg := resp.ToMessage()
	if msg.Role != "assistant" {
		t.Errorf("Expected role=assistant, got %s", msg.Role)
	}

	if msg.Content != "Hello!" {
		t.Errorf("Expected content='Hello!', got %s", msg.Content)
	}
}

func TestResponseEmpty(t *testing.T) {
	resp := &Response{
		ID:    "resp_123",
		Model: "gpt-4o",
		Choices: []struct {
			Index        int     `json:"index"`
			Message      Message `json:"message"`
			FinishReason string  `json:"finish_reason"`
		}{},
	}

	msg := resp.ToMessage()
	if msg.Role != "assistant" {
		t.Errorf("Expected role=assistant for empty response, got %s", msg.Role)
	}

	content := resp.GetContent()
	if content != "" {
		t.Errorf("Expected empty content, got %s", content)
	}
}

func TestResponseWithToolCalls(t *testing.T) {
	resp := &Response{
		ID:    "resp_123",
		Model: "gpt-4o",
		Choices: []struct {
			Index        int     `json:"index"`
			Message      Message `json:"message"`
			FinishReason string  `json:"finish_reason"`
		}{
			{
				Index: 0,
				Message: Message{
					Role: "assistant",
					ToolCalls: []ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: FunctionCall{
								Name:      "shell",
								Arguments: json.RawMessage(`{"command":"ls"}`),
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
	}

	if !resp.HasToolCalls() {
		t.Error("Expected HasToolCalls() to return true")
	}

	toolCalls := resp.GetToolCalls()
	if len(toolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Function.Name != "shell" {
		t.Errorf("Expected tool name=shell, got %s", toolCalls[0].Function.Name)
	}
}

func TestRequestJSON(t *testing.T) {
	req := &Request{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
		},
		Temperature: 0.7,
		MaxTokens:   1000,
		Stream:      false,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	var unmarshaled Request
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	if unmarshaled.Model != "gpt-4o" {
		t.Errorf("Expected model=gpt-4o, got %s", unmarshaled.Model)
	}

	if len(unmarshaled.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(unmarshaled.Messages))
	}
}
