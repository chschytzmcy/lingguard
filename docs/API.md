# LingGuard API 文档

## 概述

LingGuard 提供统一的 LLM API 接口，兼容 OpenAI API 规范，支持多种 LLM 提供商。

---

## 1. LLM Provider API

### 1.1 统一请求格式

所有 LLM Provider 都遵循 OpenAI 兼容的 API 格式：

```
POST {apiBase}/chat/completions
```

### 1.2 请求头

| Header | 值 | 说明 |
|--------|-----|------|
| `Content-Type` | `application/json` | 内容类型 |
| `Authorization` | `Bearer {apiKey}` | API 密钥 |

### 1.3 请求体

```json
{
  "model": "string",           // 必填：模型名称
  "messages": [                // 必填：消息数组
    {
      "role": "system|user|assistant|tool",
      "content": "string",
      "tool_calls": [],        // 可选：工具调用（assistant 角色）
      "tool_call_id": "string" // 可选：工具调用 ID（tool 角色）
    }
  ],
  "tools": [],                 // 可选：工具定义
  "temperature": 0.7,          // 可选：温度参数 (0-2)
  "max_tokens": 4096,          // 可选：最大 token 数
  "stream": false              // 可选：是否流式响应
}
```

### 1.4 响应格式

```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "glm-5",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "响应内容",
        "tool_calls": []
      },
      "finish_reason": "stop|tool_calls|length"
    }
  ],
  "usage": {
    "prompt_tokens": 100,
    "completion_tokens": 50,
    "total_tokens": 150
  }
}
```

### 1.5 流式响应 (SSE)

当 `stream: true` 时，返回 Server-Sent Events：

```
data: {"id":"chatcmpl-xxx","choices":[{"delta":{"content":"Hello"},"index":0}]}

data: {"id":"chatcmpl-xxx","choices":[{"delta":{"content":" world"},"index":0}]}

data: [DONE]
```

---

## 2. 支持的 Provider 端点

| Provider | apiBase | 说明 |
|----------|---------|------|
| OpenRouter | `https://openrouter.ai/api/v1` | 推荐，支持所有模型 |
| Anthropic | `https://api.anthropic.com/v1` | Claude 直连 |
| OpenAI | `https://api.openai.com/v1` | GPT 直连 |
| DeepSeek | `https://api.deepseek.com/v1` | DeepSeek |
| Groq | `https://api.groq.com/openai/v1` | 高速推理 |
| Gemini | `https://generativelanguage.googleapis.com/v1beta` | Google Gemini |
| vLLM | `http://localhost:8000/v1` | 本地模型 |
| GLM | `https://open.bigmodel.cn/api/anthropic` | 智谱 AI |
| MiniMax | `https://api.minimaxi.com/anthropic` | MiniMax AI |
| DashScope | `https://dashscope.aliyuncs.com/compatible-mode/v1` | 阿里云通义 |

---

## 3. Provider 自动匹配

LingGuard 支持根据模型名自动选择 Provider：

### 3.1 匹配规则

1. **直接匹配 Provider 名称**: 如果 model 值是已注册的 provider 名称，直接使用该 provider
2. **解析 `provider/model` 格式**: 支持 `glm/glm-4-plus` 格式
3. **关键词匹配**: 根据模型名中的关键词自动匹配
4. **默认 Provider**: 如果以上都不匹配，使用默认 provider

### 3.2 内置关键词

| Provider | 关键词 |
|----------|--------|
| openai | gpt, o1, o3 |
| anthropic | claude |
| deepseek | deepseek |
| qwen | qwen, tongyi, dashscope |
| glm | glm, chatglm, codegeex |
| minimax | minimax |
| moonshot | moonshot, kimi |
| gemini | gemini |
| groq | llama, mixtral, gemma |

### 3.3 配置示例

```json
{
  "agents": {
    "model": "glm"  // 直接使用 glm provider
  }
}
```

或使用 `provider/model` 格式：

```json
{
  "agents": {
    "model": "glm/glm-4-plus"  // 解析为 glm provider
  }
}
```

---

## 4. 工具调用 API

### 4.1 工具定义格式

```json
{
  "type": "function",
  "function": {
    "name": "shell",
    "description": "Execute shell commands",
    "parameters": {
      "type": "object",
      "properties": {
        "command": {
          "type": "string",
          "description": "The shell command to execute"
        },
        "timeout": {
          "type": "integer",
          "description": "Timeout in seconds"
        }
      },
      "required": ["command"]
    }
  }
}
```

### 4.2 工具调用响应

当 LLM 决定调用工具时，响应中包含 `tool_calls`：

```json
{
  "role": "assistant",
  "content": null,
  "tool_calls": [
    {
      "id": "call_abc123",
      "type": "function",
      "function": {
        "name": "shell",
        "arguments": "{\"command\":\"ls -la\"}"
      }
    }
  ]
}
```

### 4.3 工具结果提交

```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "total 32\ndrwxr-xr-x 4 user user 4096 ..."
}
```

---

## 5. 内置工具

### 5.1 Shell 工具

执行 shell 命令。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| command | string | 是 | Shell 命令 |
| timeout | integer | 否 | 超时秒数，默认 30 |

### 5.2 文件操作工具

| 工具名 | 说明 | 参数 |
|--------|------|------|
| file_read | 读取文件 | `path`: 文件路径 |
| file_write | 写入文件 | `path`, `content` |
| file_edit | 编辑文件 | `path`, `old_string`, `new_string` |
| file_list | 列出目录 | `path`: 目录路径 |

### 5.3 网页工具

| 工具名 | 说明 | 参数 |
|--------|------|------|
| web_fetch | 抓取网页 | `url`: 网页地址 |
| web_search | 搜索网页 | `query`: 搜索关键词 |

### 5.4 Spawn 工具

生成子任务并行处理。

| 参数 | 类型 | 说明 |
|------|------|------|
| tasks | array | 任务列表 |
| tasks[].prompt | string | 任务描述 |
| tasks[].description | string | 任务简述 |

---

## 6. 飞书 Channel API

### 6.1 获取访问令牌

```
POST https://open.feishu.cn/open-api/auth/v3/tenant_access_token/internal/
Content-Type: application/json

{
  "app_id": "cli_xxx",
  "app_secret": "xxx"
}
```

**响应：**

```json
{
  "code": 0,
  "msg": "ok",
  "tenant_access_token": "t-xxx",
  "expire": 7200
}
```

### 6.2 获取 WebSocket 连接地址

```
GET https://open.feishu.cn/open-api/bot/v3/ws
Authorization: Bearer {tenant_access_token}
```

**响应：**

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "url": "wss://ws.feishu.cn/xxx"
  }
}
```

### 6.3 发送消息

```
POST https://open.feishu.cn/open-api/im/v1/messages?receive_id_type=open_id
Authorization: Bearer {tenant_access_token}
Content-Type: application/json

{
  "receive_id": "ou_xxx",
  "msg_type": "text",
  "content": "{\"text\":\"Hello\"}"
}
```

### 6.4 接收消息事件 (WebSocket)

```json
{
  "header": {
    "event_id": "xxx",
    "event_type": "im.message.receive_v1",
    "create_time": "1700000000000"
  },
  "event": {
    "sender": {
      "sender_id": {
        "open_id": "ou_xxx"
      }
    },
    "message": {
      "message_id": "om_xxx",
      "content": "{\"text\":\"Hello\"}",
      "create_time": 1700000000
    }
  }
}
```

---

## 7. 错误处理

### 7.1 错误响应格式

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid API key",
    "code": "invalid_api_key"
  }
}
```

### 7.2 常见错误码

| HTTP 状态码 | 错误类型 | 说明 |
|-------------|----------|------|
| 400 | invalid_request_error | 请求参数错误 |
| 401 | authentication_error | 认证失败 |
| 403 | permission_error | 权限不足 |
| 404 | not_found_error | 资源不存在 |
| 429 | rate_limit_error | 请求频率限制 |
| 500 | api_error | 服务器内部错误 |
| 503 | overloaded_error | 服务过载 |

---

## 8. Go SDK 使用示例

### 8.1 使用 net/http 发送请求

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type ChatRequest struct {
    Model    string    `json:"model"`
    Messages []Message `json:"messages"`
    Stream   bool      `json:"stream"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type ChatResponse struct {
    ID      string `json:"id"`
    Model   string `json:"model"`
    Choices []struct {
        Message Message `json:"message"`
    } `json:"choices"`
}

func main() {
    req := ChatRequest{
        Model: "glm-5",
        Messages: []Message{
            {Role: "user", Content: "Hello!"},
        },
        Stream: false,
    }

    body, _ := json.Marshal(req)

    httpReq, _ := http.NewRequest("POST",
        "https://open.bigmodel.cn/api/anthropic/chat/completions",
        bytes.NewReader(body))
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Authorization", "Bearer xxx.xxx")

    resp, err := http.DefaultClient.Do(httpReq)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    respBody, _ := io.ReadAll(resp.Body)

    var chatResp ChatResponse
    json.Unmarshal(respBody, &chatResp)

    fmt.Println(chatResp.Choices[0].Message.Content)
}
```

### 8.2 使用 Provider Registry 自动匹配

```go
package main

import (
    "context"
    "fmt"

    "github.com/lingguard/internal/config"
    "github.com/lingguard/internal/providers"
    "github.com/lingguard/pkg/llm"
)

func main() {
    // 加载配置
    cfg, _ := config.Load("configs/config.json")

    // 创建 Provider 注册表
    registry := providers.NewRegistry()
    registry.InitFromConfig(cfg)

    // 自动匹配 Provider
    provider, ok := registry.MatchProvider("glm")
    if !ok {
        panic("provider not found")
    }

    // 调用 LLM
    req := &llm.Request{
        Model: provider.Model(),
        Messages: []llm.Message{
            {Role: "user", Content: "Hello!"},
        },
    }

    resp, err := provider.Complete(context.Background(), req)
    if err != nil {
        panic(err)
    }

    fmt.Println(resp.GetContent())
}
```

---

## 9. 配置参考

### 9.1 Provider 配置

```json
{
  "providers": {
    "glm": {
      "apiKey": "xxx.xxx",
      "apiBase": "https://open.bigmodel.cn/api/anthropic",
      "model": "glm-5",
      "temperature": 0.7,
      "maxTokens": 4096
    },
    "qwen": {
      "apiKey": "sk-xxx",
      "apiBase": "https://dashscope.aliyuncs.com/compatible-mode/v1",
      "model": "qwen3-max-2026-01-23",
      "temperature": 0.7,
      "maxTokens": 4096
    }
  }
}
```

### 9.2 Agent 配置（新版结构）

```json
{
  "agents": {
    "workspace": "~/.lingguard/workspace",
    "model": "glm",
    "maxTokens": 8192,
    "temperature": 0.7,
    "maxToolIterations": 20,
    "memoryWindow": 50,
    "systemPrompt": "你是灵侍，一个乐于助人的 AI 助手。你可以使用工具帮助用户完成各种任务。"
  }
}
```

### 9.3 配置字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| workspace | string | 工作空间目录 |
| model | string | 默认模型/Provider名称，支持自动匹配 |
| maxTokens | int | 最大输出 tokens |
| temperature | float64 | 温度参数 (0-2) |
| maxToolIterations | int | 最大工具调用迭代次数 |
| memoryWindow | int | 历史消息窗口大小 |
| systemPrompt | string | 系统提示词 |

---

## 10. 参考资料

- [OpenAI API Reference](https://platform.openai.com/docs/api-reference)
- [Anthropic API Reference](https://docs.anthropic.com/en/api)
- [智谱 AI API](https://open.bigmodel.cn/dev/api)
- [飞书开放平台](https://open.feishu.cn/document/)
