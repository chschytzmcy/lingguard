// Package llm LLM 客户端封装
package llm

import (
	"encoding/json"
)

// Message LLM 消息
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`

	// 多模态内容（当 ContentParts 非空时使用，Content 会被忽略）
	ContentParts []ContentPart `json:"-"`
}

// ContentPart 多模态内容部分
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
	VideoURL *VideoURL `json:"video_url,omitempty"` // 视频URL（Qwen-Omni）
	Video    []string  `json:"video,omitempty"`     // 视频帧URL列表（Qwen-VL）
}

// ImageURL 图片 URL
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "low", "high", "auto"
}

// VideoURL 视频 URL（用于 Qwen-Omni 模型）
type VideoURL struct {
	URL string `json:"url"`
}

// MarshalJSON 自定义 JSON 序列化，支持多模态内容
// 确保永远不会将 content 序列化为单个对象
func (m Message) MarshalJSON() ([]byte, error) {
	// 只有当 ContentParts 有有效内容时才使用数组格式
	// 严格验证每个 part
	if len(m.ContentParts) > 0 {
		validParts := make([]ContentPart, 0)
		for _, part := range m.ContentParts {
			// 每个 part 必须有 type 和实际内容
			if part.Type != "" {
				hasContent := part.Text != "" || part.ImageURL != nil || part.VideoURL != nil || len(part.Video) > 0
				if hasContent {
					validParts = append(validParts, part)
				}
			}
		}
		// 只有有有效的 parts 才使用数组格式
		if len(validParts) > 0 {
			return json.Marshal(struct {
				Role       string        `json:"role"`
				Content    []ContentPart `json:"content"`
				ToolCalls  []ToolCall    `json:"tool_calls,omitempty"`
				ToolCallID string        `json:"tool_call_id,omitempty"`
				Name       string        `json:"name,omitempty"`
			}{
				Role:       m.Role,
				Content:    validParts,
				ToolCalls:  m.ToolCalls,
				ToolCallID: m.ToolCallID,
				Name:       m.Name,
			})
		}
	}

	// 普通消息：content 是字符串（绝对不会是对象）
	content := m.Content
	// 如果 Content 为空但有 ToolCalls，仍然发送（某些 API 需要）
	if content == "" && len(m.ToolCalls) > 0 {
		content = ""
	}

	return json.Marshal(struct {
		Role       string     `json:"role"`
		Content    string     `json:"content,omitempty"`
		ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
		ToolCallID string     `json:"tool_call_id,omitempty"`
		Name       string     `json:"name,omitempty"`
	}{
		Role:       m.Role,
		Content:    content,
		ToolCalls:  m.ToolCalls,
		ToolCallID: m.ToolCallID,
		Name:       m.Name,
	})
}

// UnmarshalJSON 自定义 JSON 反序列化，处理 content 为对象的情况
// 某些模型（如 Qwen3.5-Plus, DeepSeek R1）的响应中 content 可能是：
// - string: "普通文本"
// - object: {"text": "内容", "reasoning": "思考过程"}
// - object: {"type": "text", "text": "内容"} (单对象多模态格式)
// - array: [{"type": "text", "text": "内容"}]
func (m *Message) UnmarshalJSON(data []byte) error {
	// 临时结构体，content 使用 RawMessage 来灵活处理
	type tempMsg struct {
		Role       string          `json:"role"`
		Content    json.RawMessage `json:"content,omitempty"`
		ToolCalls  []ToolCall      `json:"tool_calls,omitempty"`
		ToolCallID string          `json:"tool_call_id,omitempty"`
		Name       string          `json:"name,omitempty"`
	}

	var temp tempMsg
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	m.Role = temp.Role
	m.ToolCalls = temp.ToolCalls
	m.ToolCallID = temp.ToolCallID
	m.Name = temp.Name
	m.ContentParts = nil // 确保清空，防止之前的数据残留

	// 处理 content 字段
	if len(temp.Content) > 0 {
		// 尝试解析为字符串
		var strContent string
		if err := json.Unmarshal(temp.Content, &strContent); err == nil {
			m.Content = strContent
		} else {
			// 尝试解析为对象（Qwen/DeepSeek reasoning 格式）
			var objContent struct {
				Text      string `json:"text"`
				Reasoning string `json:"reasoning"`
			}
			if err := json.Unmarshal(temp.Content, &objContent); err == nil {
				// 使用 text 字段作为内容，reasoning 可以选择性添加
				if objContent.Text != "" {
					m.Content = objContent.Text
				} else if objContent.Reasoning != "" {
					// 如果只有 reasoning，使用 reasoning 作为内容
					m.Content = objContent.Reasoning
				}
			} else {
				// 尝试解析为单对象多模态格式 {"type": "text", "text": "..."}
				var singlePart ContentPart
				if err := json.Unmarshal(temp.Content, &singlePart); err == nil && singlePart.Type != "" {
					// 这是一个单对象，提取文本
					if singlePart.Text != "" {
						m.Content = singlePart.Text
					}
					// 注意：不设置 ContentParts，因为我们要将其作为字符串发送
				} else {
					// 尝试解析为通用对象（处理未知格式）
					var genericObj map[string]interface{}
					if err := json.Unmarshal(temp.Content, &genericObj); err == nil {
						// 尝试提取 text 或 content 字段
						if text, ok := genericObj["text"].(string); ok {
							m.Content = text
						} else if content, ok := genericObj["content"].(string); ok {
							m.Content = content
						} else {
							// 将整个对象转为 JSON 字符串作为内容（最后手段）
							m.Content = string(temp.Content)
						}
					} else {
						// 尝试解析为数组（多模态格式）
						var arrContent []ContentPart
						if err := json.Unmarshal(temp.Content, &arrContent); err == nil && len(arrContent) > 0 {
							// 验证数组内容有效性
							validParts := make([]ContentPart, 0)
							for _, part := range arrContent {
								if part.Type != "" && (part.Text != "" || part.ImageURL != nil || part.VideoURL != nil) {
									validParts = append(validParts, part)
								}
							}
							if len(validParts) > 0 {
								m.ContentParts = validParts
								// 提取文本内容
								for _, part := range validParts {
									if part.Type == "text" && part.Text != "" {
										m.Content = part.Text
										break
									}
								}
							}
						}
						// 如果都失败了，保持 Content 为空
					}
				}
			}
		}
	}

	return nil
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用
type FunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// UnmarshalJSON 自定义反序列化，正确处理 arguments 字段
// OpenAI 格式的 arguments 是一个 JSON 字符串，需要先解码字符串再存储
func (f *FunctionCall) UnmarshalJSON(data []byte) error {
	// 临时结构体
	type tempFunc struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	var temp tempFunc
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	f.Name = temp.Name

	// 检查 arguments 是否是被引号包围的字符串
	// 如果是，需要先解码字符串得到真正的 JSON
	if len(temp.Arguments) > 0 && temp.Arguments[0] == '"' {
		// arguments 是一个 JSON 字符串，需要解码
		var unquoted string
		if err := json.Unmarshal(temp.Arguments, &unquoted); err != nil {
			return err
		}
		f.Arguments = json.RawMessage(unquoted)
	} else {
		// arguments 已经是 JSON 对象
		f.Arguments = temp.Arguments
	}

	return nil
}

// MarshalJSON 自定义序列化，将 arguments 编码为 JSON 字符串格式
// OpenAI API 要求 arguments 必须是字符串类型
func (f FunctionCall) MarshalJSON() ([]byte, error) {
	// 将 Arguments（json.RawMessage）转换为字符串
	argumentsStr := string(f.Arguments)

	return json.Marshal(struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}{
		Name:      f.Name,
		Arguments: argumentsStr,
	})
}

// ToolDefinition 工具定义
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction 工具函数定义
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// Request LLM 请求
type Request struct {
	Model       string                   `json:"model"`
	Messages    []Message                `json:"messages"`
	Tools       []map[string]interface{} `json:"tools,omitempty"`
	Temperature float64                  `json:"temperature,omitempty"`
	MaxTokens   int                      `json:"max_tokens,omitempty"`
	Stream      bool                     `json:"stream,omitempty"`
}

// Response LLM 响应
type Response struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
}

// UnmarshalJSON 自定义 JSON 反序列化，处理嵌套 Message 的 content 为对象的情况
func (r *Response) UnmarshalJSON(data []byte) error {
	// 临时结构体，避免递归调用
	type tempResponse struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index        int             `json:"index"`
			Message      json.RawMessage `json:"message"`
			FinishReason string          `json:"finish_reason"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}

	var temp tempResponse
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	r.ID = temp.ID
	r.Object = temp.Object
	r.Created = temp.Created
	r.Model = temp.Model
	r.Usage = temp.Usage
	r.Choices = make([]struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	}, len(temp.Choices))

	for i, choice := range temp.Choices {
		r.Choices[i].Index = choice.Index
		r.Choices[i].FinishReason = choice.FinishReason

		// 手动解析 Message，会调用 Message.UnmarshalJSON
		var msg Message
		if err := json.Unmarshal(choice.Message, &msg); err != nil {
			return err
		}
		r.Choices[i].Message = msg
	}

	return nil
}

// Usage Token 使用量
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamEvent 流式事件
type StreamEvent struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		Delta        Delta  `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *Usage `json:"usage,omitempty"` // 某些 API 在流式结束时会返回 usage
}

// Delta 流式增量
type Delta struct {
	Role      string          `json:"role,omitempty"`
	Content   string          `json:"content,omitempty"`
	ToolCalls []DeltaToolCall `json:"tool_calls,omitempty"`
}

// UnmarshalJSON 自定义 JSON 反序列化，处理 content 为对象的情况
func (d *Delta) UnmarshalJSON(data []byte) error {
	// 临时结构体
	type tempDelta struct {
		Role      string          `json:"role,omitempty"`
		Content   json.RawMessage `json:"content,omitempty"`
		ToolCalls []DeltaToolCall `json:"tool_calls,omitempty"`
	}

	var temp tempDelta
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	d.Role = temp.Role
	d.ToolCalls = temp.ToolCalls

	// 处理 content 字段
	if len(temp.Content) > 0 {
		// 尝试解析为字符串
		var strContent string
		if err := json.Unmarshal(temp.Content, &strContent); err == nil {
			d.Content = strContent
		} else {
			// 尝试解析为对象（Qwen/DeepSeek reasoning 格式）
			var objContent struct {
				Text      string `json:"text"`
				Reasoning string `json:"reasoning"`
			}
			if err := json.Unmarshal(temp.Content, &objContent); err == nil {
				if objContent.Text != "" {
					d.Content = objContent.Text
				} else if objContent.Reasoning != "" {
					d.Content = objContent.Reasoning
				}
			} else {
				// 尝试解析为单对象多模态格式 {"type": "text", "text": "..."}
				var singlePart ContentPart
				if err := json.Unmarshal(temp.Content, &singlePart); err == nil && singlePart.Type != "" && singlePart.Text != "" {
					d.Content = singlePart.Text
				} else {
					// 尝试解析为通用对象
					var genericObj map[string]interface{}
					if err := json.Unmarshal(temp.Content, &genericObj); err == nil {
						if text, ok := genericObj["text"].(string); ok {
							d.Content = text
						}
					}
				}
			}
			// 如果都失败了，Content 保持为空
		}
	}

	return nil
}

// DeltaToolCall 流式增量中的工具调用（包含 index 字段）
type DeltaToolCall struct {
	Index    int           `json:"index"`
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Function DeltaFunction `json:"function"`
}

// DeltaFunction 流式增量中的函数调用
type DeltaFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // 流式时是字符串片段，需要累积
}

// ToMessage 将响应转换为消息
func (r *Response) ToMessage() Message {
	if len(r.Choices) == 0 {
		return Message{Role: "assistant"}
	}
	return r.Choices[0].Message
}

// GetContent 获取响应内容
func (r *Response) GetContent() string {
	if len(r.Choices) == 0 {
		return ""
	}
	return r.Choices[0].Message.Content
}

// GetToolCalls 获取工具调用
func (r *Response) GetToolCalls() []ToolCall {
	if len(r.Choices) == 0 {
		return nil
	}
	return r.Choices[0].Message.ToolCalls
}

// HasToolCalls 检查是否有工具调用
func (r *Response) HasToolCalls() bool {
	return len(r.GetToolCalls()) > 0
}
