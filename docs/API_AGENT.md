# LingGuard Agent API 文档

> 版本: 1.0.0
> 基础 URL: `http://localhost:18989`

## 概述

LingGuard Agent API 是一个智能体服务接口，与 OpenAI Chat API 不同，它是**有状态**的，自动管理会话历史、工具执行和记忆系统。

### 与 OpenAI Chat API 的区别

| 特性 | OpenAI Chat API | LingGuard Agent API |
|------|-----------------|---------------------|
| 状态管理 | 无状态（客户端管理历史） | 有状态（Session 自动管理） |
| 工具执行 | 返回工具调用，客户端执行 | 自动执行，返回最终结果 |
| 记忆系统 | 无 | 内置长期记忆 + 向量检索 |
| 技能系统 | 无 | 渐进式加载，按需注入 |

---

## 认证

所有 API 请求需要在 Header 中携带 Token：

```http
Authorization: Bearer <your-token>
```

Token 在 `config.json` 中配置：

```json
{
  "api": {
    "enabled": true,
    "auth": {
      "type": "token",
      "tokens": ["your-secret-token"]
    }
  }
}
```

---

## API 端点

### 对话 API

#### POST /v1/agents/{agent_id}/chat

与智能体进行对话。支持流式和非流式响应。

**路径参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `agent_id` | string | 智能体标识：`default`（默认）、`coding`、`assistant` |

**请求体**

```json
{
  "message": "帮我分析今天的日程安排",
  "media": ["https://example.com/image.png"],
  "session_id": "user-123-device-456",
  "stream": true,
  "clear_history": false,
  "tools": ["calendar", "web_search"],
  "system_prompt": "你是一个专业的助手"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `message` | string | 是 | 用户消息内容 |
| `media` | []string | 否 | 多媒体 URL 列表（图片、文件等） |
| `session_id` | string | 否 | 会话 ID，不传则新建 |
| `stream` | bool | 否 | 是否流式响应，默认 `false` |
| `clear_history` | bool | 否 | 是否清空历史后对话，默认 `false` |
| `tools` | []string | 否 | 指定可用工具，不传则使用默认集 |
| `system_prompt` | string | 否 | 覆盖默认系统提示词 |

**非流式响应**

```json
{
  "id": "resp-a1b2c3d4",
  "session_id": "user-123-device-456",
  "agent_id": "default",
  "content": "根据您的日历，今天有以下安排：\n\n1. 10:00 - 项目评审会议\n2. 14:00 - 客户电话\n3. 16:00 - 团队周会",
  "tool_calls": [
    {
      "id": "tc-001",
      "tool": "calendar",
      "action": "query",
      "params": {"start": "today", "end": "today"},
      "result": "找到 3 个事件",
      "status": "completed"
    }
  ],
  "usage": {
    "input_tokens": 150,
    "output_tokens": 280,
    "total_tokens": 430
  },
  "created_at": "2026-03-06T10:30:00Z"
}
```

**流式响应（SSE）**

请求设置 `"stream": true`，返回 Server-Sent Events：

```
event: connected
data: {"session_id": "user-123-device-456"}

event: thinking
data: {"content": "让我查一下您的日历..."}

event: tool_call
data: {"id": "tc-001", "tool": "calendar", "action": "query", "params": {"start": "today"}}

event: tool_result
data: {"id": "tc-001", "tool": "calendar", "status": "completed", "result": "找到 3 个事件"}

event: content
data: {"delta": "根据您的日历，"}

event: content
data: {"delta": "今天有以下安排："}

event: content
data: {"delta": "\n\n1. 10:00 - 项目评审会议"}

event: completed
data: {"id": "resp-a1b2c3d4", "usage": {"input_tokens": 150, "output_tokens": 280}}

event: error
data: {"code": "tool_error", "message": "工具执行失败"}
```

---

### 会话 API

#### GET /v1/sessions

获取会话列表。

**查询参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `limit` | int | 返回数量，默认 20 |
| `offset` | int | 偏移量，默认 0 |
| `agent_id` | string | 按智能体筛选 |

**响应**

```json
{
  "sessions": [
    {
      "id": "user-123-device-456",
      "title": "日程安排分析",
      "agent_id": "default",
      "message_count": 12,
      "created_at": "2026-03-05T14:00:00Z",
      "updated_at": "2026-03-06T10:30:00Z"
    }
  ],
  "total": 5,
  "limit": 20,
  "offset": 0
}
```

---

#### GET /v1/sessions/{session_id}

获取会话详情，包含历史消息。

**查询参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `limit` | int | 返回消息数量，默认 50 |

**响应**

```json
{
  "id": "user-123-device-456",
  "title": "日程安排分析",
  "agent_id": "default",
  "messages": [
    {
      "id": "msg-001",
      "role": "user",
      "content": "帮我分析今天的日程",
      "created_at": "2026-03-06T10:00:00Z"
    },
    {
      "id": "msg-002",
      "role": "assistant",
      "content": "根据您的日历...",
      "tool_calls": [...],
      "created_at": "2026-03-06T10:00:05Z"
    }
  ],
  "message_count": 12,
  "created_at": "2026-03-05T14:00:00Z",
  "updated_at": "2026-03-06T10:30:00Z"
}
```

---

#### DELETE /v1/sessions/{session_id}

删除会话及其历史记录。

**响应**

```json
{
  "message": "session deleted",
  "id": "user-123-device-456"
}
```

---

#### POST /v1/sessions/{session_id}/clear

清空会话历史，保留会话本身。

**响应**

```json
{
  "message": "session cleared",
  "id": "user-123-device-456"
}
```

---

### 任务 API

用于长时间运行的异步任务。

#### POST /v1/tasks

创建异步任务。

**请求体**

```json
{
  "prompt": "帮我重构整个项目的代码结构，优化性能",
  "session_id": "user-123",
  "agent_id": "coding",
  "tools": ["shell", "file", "opencode"],
  "callback_url": "https://your-server.com/callback"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `prompt` | string | 是 | 任务描述 |
| `session_id` | string | 否 | 关联的会话 ID |
| `agent_id` | string | 否 | 智能体 ID，默认 `default` |
| `tools` | []string | 否 | 可用工具列表 |
| `callback_url` | string | 否 | 任务完成回调 URL |

**响应**

```json
{
  "id": "task-abc123",
  "status": "pending",
  "prompt": "帮我重构整个项目的代码结构...",
  "agent_id": "coding",
  "created_at": "2026-03-06T10:00:00Z"
}
```

---

#### GET /v1/tasks/{task_id}

查询任务状态。

**响应**

```json
{
  "id": "task-abc123",
  "status": "running",
  "progress": 45,
  "progress_message": "正在分析代码结构...",
  "result": null,
  "error": null,
  "agent_id": "coding",
  "created_at": "2026-03-06T10:00:00Z",
  "updated_at": "2026-03-06T10:05:00Z"
}
```

**状态值**

| 状态 | 说明 |
|------|------|
| `pending` | 等待执行 |
| `running` | 执行中 |
| `completed` | 已完成 |
| `failed` | 执行失败 |
| `cancelled` | 已取消 |

---

#### POST /v1/tasks/{task_id}/cancel

取消正在执行的任务。

**响应**

```json
{
  "id": "task-abc123",
  "status": "cancelled",
  "message": "task cancelled by user"
}
```

---

#### GET /v1/tasks/{task_id}/events

获取任务执行事件的 SSE 流。

**响应**

```
event: started
data: {"task_id": "task-abc123", "timestamp": "2026-03-06T10:00:00Z"}

event: progress
data: {"progress": 20, "message": "正在读取项目文件..."}

event: tool_call
data: {"tool": "file", "action": "list", "path": "/src"}

event: progress
data: {"progress": 45, "message": "正在分析代码结构..."}

event: content
data: {"delta": "我建议进行以下重构..."}

event: completed
data: {"result": "重构方案已生成，共 5 个优化点", "progress": 100}
```

---

### 工具 API

#### GET /v1/tools

获取可用工具列表。

**响应**

```json
{
  "tools": [
    {
      "name": "shell",
      "description": "执行 shell 命令",
      "dangerous": true,
      "enabled": true
    },
    {
      "name": "file",
      "description": "文件读写操作",
      "dangerous": false,
      "enabled": true
    },
    {
      "name": "calendar",
      "description": "日历管理（飞书/钉钉）",
      "dangerous": false,
      "enabled": true
    },
    {
      "name": "web_search",
      "description": "网页搜索",
      "dangerous": false,
      "enabled": true
    },
    {
      "name": "aigc",
      "description": "AI 图像/视频生成",
      "dangerous": false,
      "enabled": false
    }
  ]
}
```

---

#### POST /v1/tools/{tool_name}/execute

直接执行工具（不经过 Agent）。

**请求体**

```json
{
  "action": "query",
  "params": {
    "start": "today",
    "end": "tomorrow"
  }
}
```

**响应**

```json
{
  "tool": "calendar",
  "action": "query",
  "result": "找到 3 个事件：\n1. 10:00 - 项目评审\n2. 14:00 - 客户电话\n3. 16:00 - 周会",
  "success": true,
  "duration_ms": 320
}
```

---

### 智能体 API

#### GET /v1/agents

获取可用智能体列表。

**响应**

```json
{
  "agents": [
    {
      "id": "default",
      "name": "灵侍",
      "description": "通用智能助手",
      "enabled": true,
      "default": true
    },
    {
      "id": "coding",
      "name": "编程助手",
      "description": "专注于代码开发和调试",
      "enabled": true,
      "default": false
    }
  ]
}
```

---

#### GET /v1/agents/{agent_id}

获取智能体详情。

**响应**

```json
{
  "id": "default",
  "name": "灵侍",
  "description": "通用智能助手",
  "provider": "glm",
  "model": "glm-5",
  "tools": ["shell", "file", "calendar", "web_search", "memory"],
  "skills": ["calendar", "weather", "clawhub"],
  "system_prompt": "你是灵侍，一个乐于助人的 AI 助手..."
}
```

---

## 错误响应

所有错误使用统一格式：

```json
{
  "error": {
    "code": "session_not_found",
    "message": "会话不存在",
    "details": {
      "session_id": "user-xxx"
    }
  }
}
```

**错误码**

| HTTP 状态码 | 错误码 | 说明 |
|------------|--------|------|
| 400 | `invalid_request` | 请求参数错误 |
| 400 | `missing_message` | 缺少消息内容 |
| 401 | `unauthorized` | Token 无效或过期 |
| 403 | `forbidden` | 无权限访问 |
| 404 | `session_not_found` | 会话不存在 |
| 404 | `agent_not_found` | 智能体不存在 |
| 404 | `task_not_found` | 任务不存在 |
| 409 | `session_busy` | 会话正在处理其他请求 |
| 429 | `rate_limit_exceeded` | 请求频率超限 |
| 500 | `internal_error` | 服务器内部错误 |
| 503 | `provider_error` | LLM 服务不可用 |
| 504 | `timeout` | 请求超时 |

---

## SDK 示例

### Swift (iOS)

```swift
import Foundation

class LingGuardClient {
    let baseURL: String
    let token: String

    init(baseURL: String, token: String) {
        self.baseURL = baseURL
        self.token = token
    }

    // 简单对话
    func chat(message: String, sessionId: String? = nil) async throws -> ChatResponse {
        var request = URLRequest(url: URL(string: "\(baseURL)/v1/agents/default/chat")!)
        request.httpMethod = "POST"
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")

        var body: [String: Any] = ["message": message, "stream": false]
        if let sessionId = sessionId {
            body["session_id"] = sessionId
        }
        request.httpBody = try JSONSerialization.data(withJSONObject: body)

        let (data, _) = try await URLSession.shared.data(for: request)
        return try JSONDecoder().decode(ChatResponse.self, from: data)
    }

    // 流式对话
    func streamChat(message: String, sessionId: String? = nil) -> AsyncThrowingStream<StreamEvent, Error> {
        AsyncThrowingStream { continuation in
            Task {
                var request = URLRequest(url: URL(string: "\(baseURL)/v1/agents/default/chat")!)
                request.httpMethod = "POST"
                request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
                request.setValue("application/json", forHTTPHeaderField: "Content-Type")

                var body: [String: Any] = ["message": message, "stream": true]
                if let sessionId = sessionId {
                    body["session_id"] = sessionId
                }
                request.httpBody = try JSONSerialization.data(withJSONObject: body)

                let (bytes, _) = try await URLSession.shared.bytes(for: request)

                for try await line in bytes.lines {
                    if line.hasPrefix("data: ") {
                        let jsonStr = String(line.dropFirst(6))
                        let event = try parseSSEEvent(jsonStr)
                        continuation.yield(event)

                        if case .completed = event {
                            continuation.finish()
                        }
                    }
                }
            }
        }
    }
}

// 使用示例
let client = LingGuardClient(baseURL: "http://localhost:18989", token: "your-token")

// 非流式
let response = try await client.chat(message: "今天有什么安排？")
print(response.content)

// 流式
for try await event in client.streamChat(message: "帮我搜索最新的 AI 新闻") {
    switch event {
    case .content(let delta):
        print(delta, terminator: "")
    case .completed:
        print("\n完成！")
    default:
        break
    }
}
```

### Kotlin (Android)

```kotlin
class LingGuardClient(
    private val baseURL: String,
    private val token: String
) {
    private val client = OkHttpClient()
    private val json = Json { ignoreUnknownKeys = true }

    suspend fun chat(
        message: String,
        sessionId: String? = null,
        agentId: String = "default"
    ): ChatResponse = withContext(Dispatchers.IO) {
        val body = buildJsonObject {
            put("message", message)
            put("stream", false)
            sessionId?.let { put("session_id", it) }
        }

        val request = Request.Builder()
            .url("$baseURL/v1/agents/$agentId/chat")
            .header("Authorization", "Bearer $token")
            .post(body.toString().toRequestBody("application/json".toMediaType()))
            .build()

        client.newCall(request).execute().use { response ->
            json.decodeFromString(response.body!!.string())
        }
    }

    fun streamChat(
        message: String,
        sessionId: String? = null,
        agentId: String = "default",
        onEvent: (StreamEvent) -> Unit
    ) {
        val body = buildJsonObject {
            put("message", message)
            put("stream", true)
            sessionId?.let { put("session_id", it) }
        }

        val request = Request.Builder()
            .url("$baseURL/v1/agents/$agentId/chat")
            .header("Authorization", "Bearer $token")
            .header("Accept", "text/event-stream")
            .post(body.toString().toRequestBody("application/json".toMediaType()))
            .build()

        client.newCall(request).execute().use { response ->
            response.body!!.source().buffer().use { buffer ->
                while (!buffer.exhausted()) {
                    val line = buffer.readUtf8Line() ?: break
                    if (line.startsWith("data: ")) {
                        val event = parseSSEEvent(line.removePrefix("data: "))
                        onEvent(event)
                    }
                }
            }
        }
    }
}

// 使用示例
val client = LingGuardClient("http://localhost:18989", "your-token")

// 非流式
lifecycleScope.launch {
    val response = client.chat("今天有什么安排？")
    println(response.content)
}

// 流式
client.streamChat("帮我搜索最新的 AI 新闻") { event ->
    when (event) {
        is StreamEvent.Content -> print(event.delta)
        is StreamEvent.Completed -> println("\n完成！")
        else -> {}
    }
}
```

### TypeScript (Web)

```typescript
interface ChatRequest {
  message: string;
  media?: string[];
  session_id?: string;
  stream?: boolean;
  clear_history?: boolean;
  tools?: string[];
}

interface ChatResponse {
  id: string;
  session_id: string;
  agent_id: string;
  content: string;
  tool_calls: ToolCall[];
  usage: Usage;
  created_at: string;
}

class LingGuardClient {
  private baseURL: string;
  private token: string;

  constructor(baseURL: string, token: string) {
    this.baseURL = baseURL;
    this.token = token;
  }

  // 简单对话
  async chat(message: string, sessionId?: string): Promise<ChatResponse> {
    const response = await fetch(`${this.baseURL}/v1/agents/default/chat`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${this.token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        message,
        session_id: sessionId,
        stream: false,
      }),
    });

    if (!response.ok) {
      throw new Error(`API Error: ${response.status}`);
    }

    return response.json();
  }

  // 流式对话
  async *streamChat(
    message: string,
    sessionId?: string
  ): AsyncGenerator<StreamEvent> {
    const response = await fetch(`${this.baseURL}/v1/agents/default/chat`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${this.token}`,
        'Content-Type': 'application/json',
        'Accept': 'text/event-stream',
      },
      body: JSON.stringify({
        message,
        session_id: sessionId,
        stream: true,
      }),
    });

    const reader = response.body!.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const event = JSON.parse(line.slice(6));
          yield event;
        }
      }
    }
  }
}

// 使用示例
const client = new LingGuardClient('http://localhost:18989', 'your-token');

// 非流式
const response = await client.chat('今天有什么安排？');
console.log(response.content);

// 流式
for await (const event of client.streamChat('帮我搜索最新的 AI 新闻')) {
  switch (event.event) {
    case 'content':
      process.stdout.write(event.data.delta);
      break;
    case 'completed':
      console.log('\n完成！');
      break;
  }
}
```

---

## 最佳实践

### Session ID 设计

推荐格式：`{source}-{user_id}-{device_id}`

```javascript
// APP 端生成示例
const sessionId = `app-${userId}-${deviceId}`;
// 结果: app-user123-device456
```

### 流式响应处理

```swift
// 推荐：累积内容，最后更新 UI
var fullContent = ""

for try await event in client.streamChat(message: "...") {
    switch event {
    case .content(let delta):
        fullContent += delta
        // 可选：实时更新（节流）
        updateUI(fullContent)
    case .completed:
        // 最终更新
        saveMessage(fullContent)
    case .toolCall(let tool, let action):
        showToolIndicator(tool, action)
    case .error(let message):
        showError(message)
    default:
        break
    }
}
```

### 错误重试

```swift
func chatWithRetry(message: String, maxRetries: Int = 3) async throws -> ChatResponse {
    var lastError: Error?

    for i in 0..<maxRetries {
        do {
            return try await client.chat(message: message)
        } catch let error as APIError where error.isRetryable {
            lastError = error
            try await Task.sleep(nanoseconds: UInt64(pow(2.0, Double(i))) * 1_000_000_000)
        }
    }

    throw lastError!
}
```

---

## 实施计划

### 架构设计

```
cmd/lingguard/
└── main.go                    # 添加 api 子命令

internal/api/
├── server.go                  # HTTP 服务器入口
├── router.go                  # 路由注册
├── middleware/
│   ├── auth.go                # Token 认证中间件
│   ├── cors.go                # CORS 中间件
│   ├── ratelimit.go           # 限流中间件
│   └── logger.go              # 请求日志
├── handlers/
│   ├── chat.go                # Chat API 处理器
│   ├── session.go             # Session API 处理器
│   ├── task.go                # Task API 处理器
│   ├── tool.go                # Tool API 处理器
│   └── agent.go               # Agent API 处理器
├── models/
│   ├── request.go             # 请求结构体
│   ├── response.go            # 响应结构体
│   └── sse.go                 # SSE 事件结构体
├── sse/
│   ├── writer.go              # SSE 写入器
│   └── event.go               # 事件类型定义
└── task/
    ├── manager.go             # 任务管理器
    └── store.go               # 任务存储（内存/文件）
```

### Phase 1: 基础框架 (Day 1-2)

**目标**: 搭建 HTTP 服务器骨架，支持基础路由和中间件

#### 1.1 配置扩展

修改 `internal/config/config.go`:

```go
type APIConfig struct {
    Enabled     bool              `json:"enabled"`
    Port        int               `json:"port,omitempty"`        // 默认 18989
    Host        string            `json:"host,omitempty"`        // 默认 0.0.0.0
    Auth        *AuthConfig       `json:"auth,omitempty"`
    RateLimit   *RateLimitConfig  `json:"rateLimit,omitempty"`
    CORS        *CORSConfig       `json:"cors,omitempty"`
}

type AuthConfig struct {
    Type    string   `json:"type"`              // "token" | "none"
    Tokens  []string `json:"tokens,omitempty"`
}

type RateLimitConfig struct {
    Enabled      bool `json:"enabled"`
    RequestsPer  int  `json:"requestsPer"`       // 每分钟请求数
    Burst        int  `json:"burst"`             // 突发容量
}

type CORSConfig struct {
    Enabled        bool     `json:"enabled"`
    AllowedOrigins []string `json:"allowedOrigins,omitempty"`
}
```

配置示例 (`config.json`):

```json
{
  "api": {
    "enabled": true,
    "port": 18989,
    "auth": {
      "type": "token",
      "tokens": ["your-secret-token"]
    },
    "rateLimit": {
      "enabled": true,
      "requestsPer": 60,
      "burst": 10
    },
    "cors": {
      "enabled": true,
      "allowedOrigins": ["*"]
    }
  }
}
```

#### 1.2 HTTP 服务器

创建 `internal/api/server.go`:

```go
type Server struct {
    config     *config.Config
    httpServer *http.Server
    router     *mux.Router
    sessionMgr session.Manager
    taskMgr    *task.Manager
}
```

#### 1.3 CLI 命令集成

修改 `cmd/lingguard/main.go`:

```go
case "api":
    // 启动 API 服务器
    server := api.NewServer(cfg)
    return server.Start(ctx)
```

**验收标准**:
- `./lingguard api` 启动服务器
- `GET /v1/health` 返回 200
- Token 认证中间件正常工作

---

### Phase 2: Chat API (Day 3-4)

**目标**: 实现核心对话接口，支持流式和非流式响应

#### 2.1 请求/响应模型

创建 `internal/api/models/request.go`:

```go
type ChatRequest struct {
    Message       string   `json:"message"`
    Media         []string `json:"media,omitempty"`
    SessionID     string   `json:"session_id,omitempty"`
    Stream        bool     `json:"stream,omitempty"`
    ClearHistory  bool     `json:"clear_history,omitempty"`
    Tools         []string `json:"tools,omitempty"`
    SystemPrompt  string   `json:"system_prompt,omitempty"`
}
```

创建 `internal/api/models/response.go`:

```go
type ChatResponse struct {
    ID         string      `json:"id"`
    SessionID  string      `json:"session_id"`
    AgentID    string      `json:"agent_id"`
    Content    string      `json:"content"`
    ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
    Usage      *Usage      `json:"usage,omitempty"`
    CreatedAt  time.Time   `json:"created_at"`
}
```

#### 2.2 Chat Handler

创建 `internal/api/handlers/chat.go`:

```go
func (h *ChatHandler) Handle(w http.ResponseWriter, r *http.Request) {
    var req ChatRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, 400, "invalid_request", err.Error())
        return
    }

    if req.Message == "" {
        respondError(w, 400, "missing_message", "message is required")
        return
    }

    // 获取或创建 Session
    sess := h.sessionMgr.GetOrCreate(req.SessionID)

    // 处理 stream
    if req.Stream {
        h.handleStream(w, r, req, sess)
    } else {
        h.handleNonStream(w, r, req, sess)
    }
}
```

#### 2.3 SSE 支持

创建 `internal/api/sse/writer.go`:

```go
type SSEWriter struct {
    w http.ResponseWriter
    f http.Flusher
}

func (s *SSEWriter) WriteEvent(event string, data interface{}) error {
    fmt.Fprintf(s.w, "event: %s\n", event)
    jsonData, _ := json.Marshal(data)
    fmt.Fprintf(s.w, "data: %s\n\n", jsonData)
    s.f.Flush()
    return nil
}
```

#### 2.4 Agent 集成

修改 `internal/agent/agent.go`，添加回调机制：

```go
type StreamCallback interface {
    OnThinking(content string)
    OnToolCall(id, tool, action string, params map[string]interface{})
    OnToolResult(id, tool, status, result string)
    OnContent(delta string)
    OnCompleted(responseID string, usage *Usage)
    OnError(code, message string)
}
```

**验收标准**:
- `POST /v1/agents/default/chat` 非流式响应正常
- `POST /v1/agents/default/chat` (stream=true) SSE 事件正确
- Session 历史自动管理
- 工具调用事件正确推送

---

### Phase 3: Session API (Day 5)

**目标**: 实现会话管理接口

#### 3.1 Session Handler

创建 `internal/api/handlers/session.go`:

```go
func (h *SessionHandler) List(w http.ResponseWriter, r *http.Request) {
    limit := parseInt(r.URL.Query().Get("limit"), 20)
    offset := parseInt(r.URL.Query().Get("offset"), 0)
    agentID := r.URL.Query().Get("agent_id")

    sessions := h.sessionMgr.List(limit, offset, agentID)
    respondJSON(w, 200, map[string]interface{}{
        "sessions": sessions,
        "total":    h.sessionMgr.Count(),
        "limit":    limit,
        "offset":   offset,
    })
}

func (h *SessionHandler) Get(w http.ResponseWriter, r *http.Request) {
    sessionID := mux.Vars(r)["session_id"]
    sess, err := h.sessionMgr.Get(sessionID)
    if err != nil {
        respondError(w, 404, "session_not_found", err.Error())
        return
    }
    respondJSON(w, 200, sess)
}

func (h *SessionHandler) Delete(w http.ResponseWriter, r *http.Request) {
    sessionID := mux.Vars(r)["session_id"]
    h.sessionMgr.Delete(sessionID)
    respondJSON(w, 200, map[string]string{
        "message": "session deleted",
        "id":      sessionID,
    })
}

func (h *SessionHandler) Clear(w http.ResponseWriter, r *http.Request) {
    sessionID := mux.Vars(r)["session_id"]
    h.sessionMgr.ClearHistory(sessionID)
    respondJSON(w, 200, map[string]string{
        "message": "session cleared",
        "id":      sessionID,
    })
}
```

#### 3.2 Session Manager 扩展

修改 `pkg/session/manager.go`:

```go
type SessionInfo struct {
    ID           string    `json:"id"`
    Title        string    `json:"title"`
    AgentID      string    `json:"agent_id"`
    MessageCount int       `json:"message_count"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

func (m *Manager) List(limit, offset int, agentID string) []SessionInfo {
    // 实现列表查询
}

func (m *Manager) Get(id string) (*SessionDetail, error) {
    // 返回包含历史消息的详情
}

func (m *Manager) Delete(id string) error {
    // 删除会话及其存储
}

func (m *Manager) ClearHistory(id string) error {
    // 清空历史，保留会话
}
```

**验收标准**:
- `GET /v1/sessions` 返回会话列表
- `GET /v1/sessions/{id}` 返回会话详情
- `DELETE /v1/sessions/{id}` 删除成功
- `POST /v1/sessions/{id}/clear` 清空成功

---

### Phase 4: Task API (Day 6)

**目标**: 实现异步任务机制

#### 4.1 Task Manager

创建 `internal/api/task/manager.go`:

```go
type Task struct {
    ID             string                 `json:"id"`
    Status         string                 `json:"status"` // pending, running, completed, failed, cancelled
    Progress       int                    `json:"progress"`
    ProgressMsg    string                 `json:"progress_message,omitempty"`
    Prompt         string                 `json:"prompt"`
    Result         string                 `json:"result,omitempty"`
    Error          string                 `json:"error,omitempty"`
    AgentID        string                 `json:"agent_id"`
    SessionID      string                 `json:"session_id,omitempty"`
    CallbackURL    string                 `json:"callback_url,omitempty"`
    CreatedAt      time.Time              `json:"created_at"`
    UpdatedAt      time.Time              `json:"updated_at"`
}

type Manager struct {
    tasks    map[string]*Task
    store    Store
    executor *Executor
    mu       sync.RWMutex
}

func (m *Manager) Create(prompt string, opts ...TaskOption) (*Task, error) {
    task := &Task{
        ID:        generateID("task"),
        Status:    "pending",
        Prompt:    prompt,
        CreatedAt: time.Now(),
    }
    // 存储并启动 goroutine 执行
    go m.execute(task)
    return task, nil
}

func (m *Manager) execute(task *Task) {
    task.Status = "running"
    m.update(task)

    // 调用 Agent 执行
    // 发送进度事件
    // 处理结果/错误
}

func (m *Manager) Cancel(id string) error {
    // 取消任务
}

func (m *Manager) Subscribe(id string) <-chan TaskEvent {
    // 订阅任务事件（用于 SSE）
}
```

#### 4.2 Task Handler

创建 `internal/api/handlers/task.go`:

```go
func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req TaskRequest
    json.NewDecoder(r.Body).Decode(&req)

    task, err := h.taskMgr.Create(req.Prompt,
        WithAgentID(req.AgentID),
        WithSessionID(req.SessionID),
        WithTools(req.Tools),
        WithCallback(req.CallbackURL),
    )

    respondJSON(w, 201, task)
}

func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
    taskID := mux.Vars(r)["task_id"]
    task, err := h.taskMgr.Get(taskID)
    if err != nil {
        respondError(w, 404, "task_not_found", err.Error())
        return
    }
    respondJSON(w, 200, task)
}

func (h *TaskHandler) Events(w http.ResponseWriter, r *http.Request) {
    taskID := mux.Vars(r)["task_id"]

    // 设置 SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")

    sse := NewSSEWriter(w)
    events := h.taskMgr.Subscribe(taskID)

    for event := range events {
        sse.WriteEvent(event.Type, event.Data)
        if event.Type == "completed" || event.Type == "failed" {
            break
        }
    }
}
```

**验收标准**:
- `POST /v1/tasks` 创建任务
- `GET /v1/tasks/{id}` 查询状态
- `POST /v1/tasks/{id}/cancel` 取消任务
- `GET /v1/tasks/{id}/events` SSE 事件流

---

### Phase 5: Tool & Agent API (Day 7)

**目标**: 实现工具查询和智能体管理接口

#### 5.1 Tool Handler

创建 `internal/api/handlers/tool.go`:

```go
func (h *ToolHandler) List(w http.ResponseWriter, r *http.Request) {
    tools := h.toolRegistry.List()
    respondJSON(w, 200, map[string]interface{}{
        "tools": tools,
    })
}

func (h *ToolHandler) Execute(w http.ResponseWriter, r *http.Request) {
    toolName := mux.Vars(r)["tool_name"]

    var req ExecuteRequest
    json.NewDecoder(r.Body).Decode(&req)

    result, err := h.toolRegistry.Execute(toolName, req.Action, req.Params)
    if err != nil {
        respondError(w, 500, "tool_error", err.Error())
        return
    }

    respondJSON(w, 200, result)
}
```

#### 5.2 Agent Handler

创建 `internal/api/handlers/agent.go`:

```go
func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
    agents := h.agentRegistry.List()
    respondJSON(w, 200, map[string]interface{}{
        "agents": agents,
    })
}

func (h *AgentHandler) Get(w http.ResponseWriter, r *http.Request) {
    agentID := mux.Vars(r)["agent_id"]

    agent, err := h.agentRegistry.Get(agentID)
    if err != nil {
        respondError(w, 404, "agent_not_found", err.Error())
        return
    }

    respondJSON(w, 200, agent)
}
```

**验收标准**:
- `GET /v1/tools` 返回工具列表
- `POST /v1/tools/{name}/execute` 直接执行工具
- `GET /v1/agents` 返回智能体列表
- `GET /v1/agents/{id}` 返回智能体详情

---

### Phase 6: 测试 & 文档 (Day 8)

#### 6.1 单元测试

```
internal/api/
├── handlers/
│   ├── chat_test.go
│   ├── session_test.go
│   ├── task_test.go
│   └── tool_test.go
└── middleware/
    └── auth_test.go
```

#### 6.2 集成测试

创建 `tests/api_integration_test.go`:

```go
func TestChatAPI(t *testing.T) {
    // 启动测试服务器
    // 测试非流式对话
    // 测试流式对话
    // 测试 Session 管理
}

func TestTaskAPI(t *testing.T) {
    // 测试任务创建
    // 测试任务状态查询
    // 测试任务取消
}
```

#### 6.3 API 文档更新

- 添加 OpenAPI/Swagger 规范
- 添加 Postman Collection

---

### 依赖关系

```
Phase 1 (基础框架)
    │
    ├── Phase 2 (Chat API) ─────┬── Phase 3 (Session API)
    │                           │
    │                           └── Phase 4 (Task API)
    │
    └── Phase 5 (Tool/Agent API)
                │
                └── Phase 6 (测试 & 文档)
```

### 关键文件路径

| 文件 | 说明 |
|------|------|
| `internal/config/config.go` | 添加 APIConfig |
| `internal/api/server.go` | HTTP 服务器 |
| `internal/api/handlers/chat.go` | Chat API |
| `internal/api/handlers/session.go` | Session API |
| `internal/api/handlers/task.go` | Task API |
| `internal/api/sse/writer.go` | SSE 支持 |
| `internal/api/task/manager.go` | 任务管理器 |
| `cmd/lingguard/main.go` | CLI 集成 |

### 风险点

1. **Session 并发**: 同一会话并发请求需要加锁
2. **SSE 连接管理**: 需要处理客户端断开、超时
3. **任务持久化**: 重启后任务状态恢复（可选）
4. **内存占用**: 大量 Session 的内存管理

---

## 变更日志

### v1.0.0 (2026-03-06)

- 初始版本
- 支持 Chat、Session、Task、Tool API
- 支持 SSE 流式响应
- 支持多智能体切换
