# LingGuard ACP 能力设计文档

> 版本: v1.0
> 日期: 2026-03-14
> 状态: 设计中

## 1. 概述

### 1.1 背景

ACP (Agent Client Protocol) 是一个基于 JSON-RPC 2.0 的标准化协议，用于 AI 代理与客户端（如 IDE、编排器）之间的通信。OpenClaw 已完整实现 ACP 能力，支持 19+ 主流编码代理（Claude Code、Codex、Kiro、Cline 等）。

本文档描述如何为 LingGuard 实现 OpenClaw 同级别的 ACP 能力。

### 1.2 目标

- 实现完整的 ACP 协议支持，使 LingGuard 可被 IDE/工具通过标准协议调用
- 支持与 acpx 等工具的互操作
- 保持 LingGuard 轻量级、单二进制部署的特性
- 复用现有 Agent、Session、Memory 等核心模块

### 1.3 参考

- [Agent Client Protocol](https://agentclientprotocol.com)
- [OpenClaw ACP 实现](https://github.com/openclaw/openclaw)
- [acpx - Headless ACP CLI](https://github.com/openclaw/acpx)

---

## 2. 架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        ACP Client (IDE/Tools)                           │
│                (Claude Code, Kiro, Copilot, Cline...)                   │
└──────────────────────────────┬──────────────────────────────────────────┘
                               │ JSON-RPC 2.0 / stdio
                               ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         ACP Bridge                                       │
│                    (lingguard acp 命令)                                  │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  JSON-RPC Handler                                                │    │
│  │  - initialize / shutdown                                         │    │
│  │  - session/new / session/load / session/fork                    │    │
│  │  - prompt / cancel                                               │    │
│  │  - session/set_mode / session/set_config_option                 │    │
│  │  - listSessions / loadSession                                    │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                               │                                          │
│                               ▼                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  Session Mapper                                                  │    │
│  │  ACP Session ID <---> Gateway Session Key                        │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                               │                                          │
│                               ▼ 内部调用 / WebSocket                     │
└──────────────────────────────┬──────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        LingGuard Gateway                                 │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  Agent Core (现有)                                                │    │
│  │  - ProcessMessage / ProcessMessageStream                         │    │
│  │  - Session Manager / Memory Store                                │    │
│  │  - Tool Registry / Skills Manager                                │    │
│  │  - Provider Registry (自动匹配)                                   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2.2 数据流

```
IDE 用户输入 "fix the bug"
       │
       ▼
┌──────────────────────┐
│  IDE (ACP Client)    │
│  构建 JSON-RPC 请求   │
└──────────┬───────────┘
           │ {"jsonrpc":"2.0", "method":"prompt", "params":{...}}
           ▼
┌──────────────────────┐
│  ACP Bridge (stdio)  │
│  解析请求，映射会话    │
└──────────┬───────────┘
           │ 调用 Agent.ProcessMessageStream()
           ▼
┌──────────────────────┐
│  Agent Core          │
│  LLM 调用 + 工具执行  │
└──────────┬───────────┘
           │ StreamCallback 返回事件
           ▼
┌──────────────────────┐
│  ACP Bridge          │
│  转换为 ACP 事件格式   │
└──────────┬───────────┘
           │ NDJSON 输出到 stdout
           ▼
┌──────────────────────┐
│  IDE (ACP Client)    │
│  渲染流式响应         │
└──────────────────────┘
```

---

## 3. 模块设计

### 3.1 目录结构

```
lingguard/
├── pkg/
│   └── acp/
│       ├── types.go           # ACP 类型定义
│       ├── request.go         # JSON-RPC 请求处理
│       ├── response.go        # JSON-RPC 响应处理
│       ├── bridge.go          # ACP 桥接器核心
│       ├── session.go         # 会话映射管理
│       ├── events.go          # 事件转换
│       ├── handlers.go        # 方法处理器
│       └── errors.go          # 错误定义
├── cmd/
│   └── cli/
│       └── acp.go             # acp 子命令
└── docs/
    └── ACP_DESIGN.md          # 本文档
```

### 3.2 核心类型定义

#### 3.2.1 JSON-RPC 基础类型 (`pkg/acp/types.go`)

```go
package acp

import (
    "encoding/json"
    "time"
)

// ACP 协议版本
const Version = "0.1.0"

// JSON-RPC 请求
type Request struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      json.RawMessage `json:"id,omitempty"`
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}

// JSON-RPC 响应
type Response struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      json.RawMessage `json:"id,omitempty"`
    Result  interface{}     `json:"result,omitempty"`
    Error   *Error          `json:"error,omitempty"`
}

// JSON-RPC 错误
type Error struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

// 标准错误码
const (
    ErrParseError     = -32700
    ErrInvalidRequest = -32600
    ErrMethodNotFound = -32601
    ErrInvalidParams  = -32602
    ErrInternalError  = -32603
)
```

#### 3.2.2 会话类型 (`pkg/acp/types.go`)

```go
// 会话模式
type SessionMode string

const (
    SessionModePersistent SessionMode = "persistent"
    SessionModeOneshot    SessionMode = "oneshot"
)

// 会话信息
type SessionInfo struct {
    ID           string        `json:"id"`
    Mode         SessionMode   `json:"mode"`
    Model        string        `json:"model,omitempty"`
    CreatedAt    time.Time     `json:"created_at"`
    UpdatedAt    time.Time     `json:"updated_at"`
    Cwd          string        `json:"cwd,omitempty"`
    Capabilities *Capabilities `json:"capabilities,omitempty"`
}

// 能力声明
type Capabilities struct {
    Controls []string `json:"controls"`
}

// 默认能力
var DefaultCapabilities = &Capabilities{
    Controls: []string{
        "session/set_mode",
        "session/set_config_option",
        "session/status",
    },
}
```

#### 3.2.3 事件类型 (`pkg/acp/types.go`)

```go
// 事件类型
type EventType string

const (
    EventAgentMessageChunk EventType = "agent_message_chunk"
    EventAgentThoughtChunk EventType = "agent_thought_chunk"
    EventToolCall          EventType = "tool_call"
    EventToolCallUpdate    EventType = "tool_call_update"
    EventUsageUpdate       EventType = "usage_update"
    EventSessionInfoUpdate EventType = "session_info_update"
    EventDone              EventType = "done"
    EventError             EventType = "error"
)

// 事件
type Event struct {
    Type      EventType   `json:"type"`
    SessionID string      `json:"session_id,omitempty"`
    RequestID string      `json:"request_id,omitempty"`
    Seq       int         `json:"seq,omitempty"`
    Data      interface{} `json:"data,omitempty"`
    Error     *Error      `json:"error,omitempty"`
}

// Done 原因
type DoneReason string

const (
    DoneReasonStop    DoneReason = "stop"
    DoneReasonCancel  DoneReason = "cancel"
    DoneReasonError   DoneReason = "error"
    DoneReasonTimeout DoneReason = "timeout"
)
```

#### 3.2.4 提示参数 (`pkg/acp/types.go`)

```go
// 提示参数
type PromptParams struct {
    SessionID string         `json:"session_id,omitempty"`
    Content   []ContentBlock `json:"content"`
    Cwd       string         `json:"cwd,omitempty"`
    Meta      *PromptMeta    `json:"_meta,omitempty"`
}

// 内容块
type ContentBlock struct {
    Type     string            `json:"type"` // text, resource, resource_link
    Text     string            `json:"text,omitempty"`
    Resource *ResourceBlock    `json:"resource,omitempty"`
    MimeType string            `json:"mimeType,omitempty"`
    URL      string            `json:"url,omitempty"`
}

// 资源块
type ResourceBlock struct {
    Text     string `json:"text,omitempty"`
    MimeType string `json:"mimeType,omitempty"`
}

// 提示元数据 (LingGuard 扩展)
type PromptMeta struct {
    SessionKey      string `json:"sessionKey,omitempty"`
    SessionLabel    string `json:"sessionLabel,omitempty"`
    ResetSession    bool   `json:"resetSession,omitempty"`
    RequireExisting bool   `json:"requireExisting,omitempty"`
}
```

### 3.3 ACP 桥接器 (`pkg/acp/bridge.go`)

```go
package acp

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "sync"
    "sync/atomic"
    "time"

    "github.com/google/uuid"
    "github.com/lingguard/internal/agent"
    "github.com/lingguard/pkg/logger"
    "github.com/lingguard/pkg/stream"
)

// Bridge ACP 桥接器
type Bridge struct {
    agent      *agent.Agent
    gatewayURL string
    token      string

    mu          sync.RWMutex
    sessions    map[string]*ACPSession // ACP session ID -> Session
    requestSeq  int64                  // 全局请求序号

    // 取消管理
    cancelMu   sync.RWMutex
    cancelFunc map[string]context.CancelFunc // sessionID -> cancel
}

// ACPSession ACP 会话
type ACPSession struct {
    ACPID        string
    GatewayKey   string
    Mode         SessionMode
    Cwd          string
    Model        string
    CreatedAt    time.Time
    UpdatedAt    time.Time
    Capabilities *Capabilities
}

// NewBridge 创建桥接器
func NewBridge(ag *agent.Agent, gatewayURL, token string) *Bridge {
    return &Bridge{
        agent:       ag,
        gatewayURL:  gatewayURL,
        token:       token,
        sessions:    make(map[string]*ACPSession),
        cancelFunc:  make(map[string]context.CancelFunc),
    }
}

// Run 运行 ACP 桥接器 (stdio 模式)
func (b *Bridge) Run(ctx context.Context) error {
    scanner := bufio.NewScanner(os.Stdin)
    scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB 缓冲

    writer := bufio.NewWriter(os.Stdout)

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            if !scanner.Scan() {
                if err := scanner.Err(); err != nil {
                    return fmt.Errorf("scan error: %w", err)
                }
                return nil // EOF
            }

            line := scanner.Bytes()
            if len(line) == 0 {
                continue
            }

            // 解析请求
            var req Request
            if err := json.Unmarshal(line, &req); err != nil {
                b.sendError(writer, nil, ErrParseError, "Parse error", nil)
                continue
            }

            // 处理请求
            resp := b.handleRequest(ctx, &req)
            if resp != nil {
                b.sendResponse(writer, resp)
            }
        }
    }
}

// handleRequest 处理 JSON-RPC 请求
func (b *Bridge) handleRequest(ctx context.Context, req *Request) *Response {
    switch req.Method {
    case "initialize":
        return b.handleInitialize(ctx, req)
    case "shutdown":
        return b.handleShutdown(ctx, req)
    case "session/new":
        return b.handleSessionNew(ctx, req)
    case "session/load":
        return b.handleSessionLoad(ctx, req)
    case "session/fork":
        return b.handleSessionFork(ctx, req)
    case "prompt":
        return b.handlePrompt(ctx, req)
    case "cancel":
        return b.handleCancel(ctx, req)
    case "listSessions":
        return b.handleListSessions(ctx, req)
    case "loadSession":
        return b.handleLoadSession(ctx, req)
    case "session/set_mode":
        return b.handleSetMode(ctx, req)
    case "session/set_config_option":
        return b.handleSetConfigOption(ctx, req)
    case "session/status":
        return b.handleSessionStatus(ctx, req)
    default:
        return &Response{
            JSONRPC: "2.0",
            ID:      req.ID,
            Error:   &Error{Code: ErrMethodNotFound, Message: "Method not found"},
        }
    }
}

// handleInitialize 处理 initialize
func (b *Bridge) handleInitialize(ctx context.Context, req *Request) *Response {
    return &Response{
        JSONRPC: "2.0",
        ID:      req.ID,
        Result: map[string]interface{}{
            "version": Version,
            "capabilities": map[string]interface{}{
                "sessions": map[string]interface{}{
                    "persistent": true,
                    "oneshot":    true,
                },
                "streaming": true,
                "tools":     true,
            },
        },
    }
}

// handleSessionNew 处理 session/new
func (b *Bridge) handleSessionNew(ctx context.Context, req *Request) *Response {
    var params struct {
        Mode      SessionMode          `json:"mode"`
        Cwd       string               `json:"cwd"`
        Model     string               `json:"model,omitempty"`
        McpServers []McpServerConfig   `json:"mcpServers,omitempty"`
    }

    if err := json.Unmarshal(req.Params, &params); err != nil {
        return b.errorResponse(req.ID, ErrInvalidParams, "Invalid params", nil)
    }

    // 创建 ACP 会话
    acpID := "acp:" + uuid.New().String()[:8]
    gatewayKey := "acp-" + acpID[4:]

    session := &ACPSession{
        ACPID:        acpID,
        GatewayKey:   gatewayKey,
        Mode:         params.Mode,
        Cwd:          params.Cwd,
        Model:        params.Model,
        CreatedAt:    time.Now(),
        UpdatedAt:    time.Now(),
        Capabilities: DefaultCapabilities,
    }

    b.mu.Lock()
    b.sessions[acpID] = session
    b.mu.Unlock()

    logger.Info("ACP session created", "acpID", acpID, "gatewayKey", gatewayKey)

    return &Response{
        JSONRPC: "2.0",
        ID:      req.ID,
        Result: SessionInfo{
            ID:           acpID,
            Mode:         session.Mode,
            Model:        session.Model,
            CreatedAt:    session.CreatedAt,
            UpdatedAt:    session.UpdatedAt,
            Cwd:          session.Cwd,
            Capabilities: session.Capabilities,
        },
    }
}

// handlePrompt 处理 prompt
func (b *Bridge) handlePrompt(ctx context.Context, req *Request) *Response {
    var params PromptParams
    if err := json.Unmarshal(req.Params, &params); err != nil {
        return b.errorResponse(req.ID, ErrInvalidParams, "Invalid params", nil)
    }

    // 获取或创建会话
    session := b.getOrCreateSession(&params)

    // 构建提示文本
    prompt := b.buildPromptText(&params)

    // 创建带取消的上下文
    promptCtx, cancel := context.WithCancel(ctx)
    b.cancelMu.Lock()
    b.cancelFunc[session.ACPID] = cancel
    b.cancelMu.Unlock()

    // 获取请求序号
    seq := atomic.AddInt64(&b.requestSeq, 1)

    // 创建事件写入器
    writer := bufio.NewWriter(os.Stdout)

    // 流式回调
    callback := func(e stream.Event) {
        acpEvent := b.convertEvent(e, session.ACPID, req.ID, int(seq))
        if acpEvent != nil {
            b.sendEvent(writer, acpEvent)
        }
    }

    // 异步执行 Agent
    go func() {
        defer func() {
            b.cancelMu.Lock()
            delete(b.cancelFunc, session.ACPID)
            b.cancelMu.Unlock()
            cancel()
        }()

        err := b.agent.ProcessMessageStream(promptCtx, session.GatewayKey, prompt, callback)
        
        // 发送完成事件
        if err != nil {
            b.sendEvent(writer, &Event{
                Type:      EventError,
                SessionID: session.ACPID,
                RequestID: string(req.ID),
                Error:     &Error{Code: ErrInternalError, Message: err.Error()},
            })
        } else {
            b.sendEvent(writer, &Event{
                Type:      EventDone,
                SessionID: session.ACPID,
                RequestID: string(req.ID),
                Data:      map[string]string{"reason": string(DoneReasonStop)},
            })
        }
    }()

    // 立即返回 (流式事件通过 notification 发送)
    return &Response{
        JSONRPC: "2.0",
        ID:      req.ID,
        Result: map[string]interface{}{
            "session_id": session.ACPID,
            "status":     "streaming",
        },
    }
}

// handleCancel 处理 cancel
func (b *Bridge) handleCancel(ctx context.Context, req *Request) *Response {
    var params struct {
        SessionID string `json:"session_id"`
    }

    if err := json.Unmarshal(req.Params, &params); err != nil {
        return b.errorResponse(req.ID, ErrInvalidParams, "Invalid params", nil)
    }

    b.cancelMu.RLock()
    cancel, ok := b.cancelFunc[params.SessionID]
    b.cancelMu.RUnlock()

    if ok {
        cancel()
        logger.Info("ACP session cancelled", "sessionID", params.SessionID)
    }

    return &Response{
        JSONRPC: "2.0",
        ID:      req.ID,
        Result:  map[string]string{"status": "cancelled"},
    }
}
```

### 3.4 事件转换 (`pkg/acp/events.go`)

```go
package acp

import (
    "github.com/lingguard/pkg/stream"
)

// convertEvent 转换 Agent 事件为 ACP 事件
func (b *Bridge) convertEvent(e stream.Event, sessionID string, reqID json.RawMessage, seq int) *Event {
    switch e.Type {
    case stream.EventTypeText:
        return &Event{
            Type:      EventAgentMessageChunk,
            SessionID: sessionID,
            RequestID: string(reqID),
            Seq:       seq,
            Data: map[string]string{
                "text": e.Content,
            },
        }

    case stream.EventTypeToolStart:
        return &Event{
            Type:      EventToolCall,
            SessionID: sessionID,
            RequestID: string(reqID),
            Seq:       seq,
            Data: map[string]interface{}{
                "tool":   e.ToolName,
                "status": "running",
            },
        }

    case stream.EventTypeToolEnd:
        status := "completed"
        if e.Error != nil {
            status = "error"
        }
        return &Event{
            Type:      EventToolCallUpdate,
            SessionID: sessionID,
            RequestID: string(reqID),
            Seq:       seq,
            Data: map[string]interface{}{
                "tool":    e.ToolName,
                "status":  status,
                "output":  e.Content,
                "error":   e.Error,
            },
        }

    case stream.EventTypeDone:
        return &Event{
            Type:      EventDone,
            SessionID: sessionID,
            RequestID: string(reqID),
            Seq:       seq,
            Data: map[string]string{
                "reason": string(DoneReasonStop),
            },
        }

    case stream.EventTypeError:
        return &Event{
            Type:      EventError,
            SessionID: sessionID,
            RequestID: string(reqID),
            Seq:       seq,
            Error: &Error{
                Code:    ErrInternalError,
                Message: e.Content,
            },
        }

    default:
        return nil
    }
}
```

### 3.5 CLI 命令 (`cmd/cli/acp.go`)

```go
package cli

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    "github.com/lingguard/internal/config"
    "github.com/lingguard/pkg/acp"
    "github.com/lingguard/pkg/logger"
    "github.com/spf13/cobra"
)

var acpCmd = &cobra.Command{
    Use:   "acp",
    Short: "Run ACP bridge for IDE integration",
    Long: `Run the ACP (Agent Client Protocol) bridge that talks to LingGuard Gateway.

This command speaks ACP over stdio for IDEs and forwards prompts to the Gateway.
It keeps ACP sessions mapped to Gateway session keys.

Examples:
  # Local Gateway (default)
  lingguard acp

  # Remote Gateway
  lingguard acp --url wss://gateway-host:18789 --token <token>

  # Attach to existing session
  lingguard acp --session agent:main:main

  # Reset session before first prompt
  lingguard acp --session agent:main:main --reset-session

Zed editor setup (~/.config/zed/settings.json):
  {
    "agent_servers": {
      "LingGuard ACP": {
        "type": "custom",
        "command": "lingguard",
        "args": ["acp"],
        "env": {}
      }
    }
  }
`,
    Run: func(cmd *cobra.Command, args []string) {
        if err := runACP(); err != nil {
            fmt.Fprintf(os.Stderr, "Error: %v\n", err)
            os.Exit(1)
        }
    },
}

var (
    acpURL       string
    acpToken     string
    acpTokenFile string
    acpSession   string
    acpLabel     string
    acpReset     bool
    acpVerbose   bool
)

func init() {
    rootCmd.AddCommand(acpCmd)

    acpCmd.Flags().StringVar(&acpURL, "url", "", "Gateway WebSocket URL (default: ws://127.0.0.1:18789)")
    acpCmd.Flags().StringVar(&acpToken, "token", "", "Gateway auth token")
    acpCmd.Flags().StringVar(&acpTokenFile, "token-file", "", "Path to token file")
    acpCmd.Flags().StringVar(&acpSession, "session", "", "Gateway session key (e.g., agent:main:main)")
    acpCmd.Flags().StringVar(&acpLabel, "session-label", "", "Session label to resolve")
    acpCmd.Flags().BoolVar(&acpReset, "reset-session", false, "Reset session before first prompt")
    acpCmd.Flags().BoolVar(&acpVerbose, "verbose", false, "Enable verbose logging to stderr")
}

func runACP() error {
    // 加载配置
    cfg, err := config.Load(cfgPath)
    if err != nil {
        return fmt.Errorf("load config: %w", err)
    }

    // 解析 Gateway URL
    gatewayURL := acpURL
    if gatewayURL == "" && cfg.Gateway != nil {
        gatewayURL = cfg.Gateway.URL
    }
    if gatewayURL == "" {
        gatewayURL = "ws://127.0.0.1:18789"
    }

    // 解析 Token
    token := acpToken
    if token == "" && acpTokenFile != "" {
        data, err := os.ReadFile(acpTokenFile)
        if err != nil {
            return fmt.Errorf("read token file: %w", err)
        }
        token = string(data)
    }
    if token == "" && cfg.Gateway != nil {
        token = cfg.Gateway.Token
    }

    // 静默模式 (只输出 JSON 到 stdout)
    if !acpVerbose {
        logger.SetLevel("error")
    }

    // 创建 Agent (用于本地模式)
    builder := NewAgentBuilder(cfg)
    builder.InitSkills(false)
    if err := builder.InitProvider(); err != nil {
        return fmt.Errorf("init provider: %w", err)
    }
    builder.InitWorkspace()

    ag, err := builder.Build()
    if err != nil {
        return fmt.Errorf("create agent: %w", err)
    }

    // 创建 ACP 桥接器
    bridge := acp.NewBridge(ag, gatewayURL, token)

    // 设置默认会话
    if acpSession != "" {
        bridge.SetDefaultSession(acpSession)
    }
    if acpLabel != "" {
        bridge.SetDefaultSessionLabel(acpLabel)
    }
    bridge.SetResetSession(acpReset)

    // 处理信号
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-sigCh
        if acpVerbose {
            fmt.Fprintln(os.Stderr, "ACP bridge shutting down...")
        }
        cancel()
    }()

    if acpVerbose {
        fmt.Fprintln(os.Stderr, "ACP bridge started")
    }

    // 运行桥接器
    return bridge.Run(ctx)
}
```

---

## 4. ACP 协议覆盖

### 4.1 核心方法

| 方法 | 状态 | 说明 |
|------|------|------|
| `initialize` | ✅ 必须 | 协议握手，返回能力声明 |
| `shutdown` | ✅ 必须 | 关闭桥接器 |
| `session/new` | ✅ 必须 | 创建新会话 |
| `session/load` | ⚠️ 部分 | 加载已有会话（仅文本历史） |
| `session/fork` | ⚠️ 部分 | 复制会话 |
| `prompt` | ✅ 必须 | 发送提示 |
| `cancel` | ✅ 必须 | 取消当前执行 |

### 4.2 会话控制

| 方法 | 状态 | 说明 |
|------|------|------|
| `session/set_mode` | ✅ | 设置会话模式 |
| `session/set_config_option` | ✅ | 设置配置选项 |
| `session/status` | ✅ | 获取会话状态 |
| `listSessions` | ✅ | 列出会话 |
| `loadSession` | ⚠️ 部分 | 加载会话历史 |

### 4.3 事件类型

| 事件 | 状态 | 说明 |
|------|------|------|
| `agent_message_chunk` | ✅ | 消息流 |
| `tool_call` | ✅ | 工具调用开始 |
| `tool_call_update` | ✅ | 工具调用更新/完成 |
| `usage_update` | ⚠️ 部分 | Token 使用量（近似） |
| `session_info_update` | ⚠️ 部分 | 会话信息更新 |
| `done` | ✅ | 执行完成 |
| `error` | ✅ | 错误 |

### 4.4 已知限制

| 限制 | 说明 |
|------|------|
| `loadSession` 历史不完整 | 仅回放用户/助手文本，不重建工具调用历史 |
| 无 MCP 客户端方法 | 不支持 `fs/read_text_file`, `fs/write_text_file` |
| 无终端方法 | 不支持 `terminal/*` ACP 客户端终端 |
| 计划/思考流未暴露 | 当前仅输出文本和工具状态 |
| 使用量数据为近似值 | 来源于会话快照，无成本数据 |

---

## 5. 与 OpenClaw 对比

### 5.1 架构差异

| 维度 | LingGuard | OpenClaw |
|-----|-----------|----------|
| **编程语言** | Go 1.24 | TypeScript |
| **协议实现** | 手动 JSON-RPC | @agentclientprotocol/sdk |
| **事件机制** | Channel | EventEmitter |
| **流式处理** | Callback 函数 | async generator |
| **会话存储** | 文件 + 内存 | Gateway 统一管理 |

### 5.2 功能对比

| 功能 | LingGuard | OpenClaw |
|------|:---------:|:--------:|
| 核心 ACP 流程 | ✅ | ✅ |
| 会话持久化 | ✅ | ✅ |
| 流式响应 | ✅ | ✅ |
| 工具执行 | ✅ | ✅ |
| MCP 支持 | ✅ | ✅ |
| 多代理后端 | ❌ | ✅ acpx |
| OAuth 认证 | ❌ | ✅ |
| 语音交互 | ❌ | ✅ Voice Wake + Talk |

### 5.3 性能对比

| 维度 | LingGuard | OpenClaw |
|------|-----------|----------|
| 内存占用 | ~20MB | ~80MB+ |
| 启动时间 | 毫秒级 | 秒级 |
| 部署方式 | 单二进制 | npm + Node |
| 交叉编译 | ✅ 原生 | ❌ |

---

## 6. 实现路线图

### Phase 1: 核心 ACP 实现 (1-2 周)

```
pkg/acp/
├── types.go           # ACP 类型定义
├── request.go         # JSON-RPC 请求解析
├── response.go        # JSON-RPC 响应构建
├── bridge.go          # 桥接器核心逻辑
├── session.go         # 会话映射管理
├── handlers.go        # 方法处理器
└── errors.go          # 错误定义

cmd/cli/
└── acp.go             # acp 子命令
```

**交付物**:
- `initialize`, `session/new`, `prompt`, `cancel` 方法
- 基础事件流 (message, tool, done, error)
- CLI 命令 `lingguard acp`

### Phase 2: 完整功能 (1 周)

```
pkg/acp/
├── events.go          # 完整事件转换
├── controls.go        # set_mode, set_config_option
└── persistence.go     # 会话持久化
```

**交付物**:
- `session/load`, `session/fork`, `listSessions`
- `session/set_mode`, `session/set_config_option`
- `usage_update`, `session_info_update` 事件

### Phase 3: MCP 集成 (1 周)

```
pkg/acp/
└── mcp_proxy.go       # MCP 服务器注入

internal/tools/
└── mcp_acp.go         # ACP 专用 MCP 工具
```

**交付物**:
- MCP 服务器注入到 `session/new`
- 工具执行通过 MCP 代理

### Phase 4: acpx 兼容 (可选, 1-2 周)

```
pkg/acp/
├── adapters/
│   ├── claude.go      # Claude Code 适配器
│   ├── codex.go       # Codex 适配器
│   └── registry.go    # 代理注册表
└── harness.go         # 外部代理管理
```

**交付物**:
- 内置代理适配器 (claude, codex, qwen...)
- 代理注册表和生命周期管理

---

## 7. 测试策略

### 7.1 单元测试

```go
// pkg/acp/bridge_test.go

func TestHandleInitialize(t *testing.T) {
    bridge := NewBridge(nil, "", "")
    req := &Request{
        JSONRPC: "2.0",
        ID:      json.RawMessage(`1`),
        Method:  "initialize",
    }
    resp := bridge.handleRequest(context.Background(), req)
    assert.Equal(t, "2.0", resp.JSONRPC)
    assert.NotNil(t, resp.Result)
}

func TestHandlePrompt(t *testing.T) {
    // Mock agent
    mockAgent := &MockAgent{}
    bridge := NewBridge(mockAgent, "", "")
    
    params := PromptParams{
        Content: []ContentBlock{
            {Type: "text", Text: "Hello"},
        },
    }
    paramsJSON, _ := json.Marshal(params)
    
    req := &Request{
        JSONRPC: "2.0",
        ID:      json.RawMessage(`1`),
        Method:  "prompt",
        Params:  paramsJSON,
    }
    
    resp := bridge.handleRequest(context.Background(), req)
    assert.Equal(t, "streaming", resp.Result.(map[string]interface{})["status"])
}
```

### 7.2 集成测试

```bash
# 使用 acpx 测试
npx acpx@latest --agent "./lingguard acp" "Hello"

# 使用 ACP 客户端测试
lingguard acp client
```

### 7.3 兼容性测试

```bash
# Zed 编辑器集成测试
# 配置 ~/.config/zed/settings.json
# 打开 Agent panel，选择 "LingGuard ACP"
# 发送测试消息
```

---

## 8. 配置示例

### 8.1 LingGuard 配置

```json
// ~/.lingguard/config.json
{
  "providers": {
    "deepseek": { "apiKey": "sk-xxx" },
    "qwen": { "apiKey": "sk-xxx" }
  },
  "agents": {
    "provider": "deepseek",
    "multimodalProvider": "qwen",
    "maxToolIterations": 20
  },
  "gateway": {
    "url": "ws://127.0.0.1:18789",
    "token": "your-token"
  }
}
```

### 8.2 Zed 编辑器配置

```json
// ~/.config/zed/settings.json
{
  "agent_servers": {
    "LingGuard ACP": {
      "type": "custom",
      "command": "lingguard",
      "args": ["acp"],
      "env": {}
    }
  }
}
```

### 8.3 VSCode 配置 (Cline)

```json
// .vscode/settings.json
{
  "cline.acpServer": {
    "command": "lingguard",
    "args": ["acp"]
  }
}
```

---

## 9. 参考资料

- [Agent Client Protocol Specification](https://agentclientprotocol.com)
- [OpenClaw ACP Bridge](https://github.com/openclaw/openclaw/blob/main/docs.acp.md)
- [acpx - Headless ACP CLI](https://github.com/openclaw/acpx)
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
- [Claude Agent ACP Adapter](https://github.com/zed-industries/claude-agent-acp)

---

## 附录 A: 错误码定义

| 错误码 | 名称 | 说明 |
|--------|------|------|
| -32700 | Parse error | JSON 解析失败 |
| -32600 | Invalid Request | 无效的请求对象 |
| -32601 | Method not found | 方法不存在 |
| -32602 | Invalid params | 无效的参数 |
| -32603 | Internal error | 内部错误 |
| -32001 | Session not found | 会话不存在 |
| -32002 | Session busy | 会话繁忙 |
| -32003 | Prompt timeout | 提示超时 |
| -32004 | Cancelled | 已取消 |

## 附录 B: 事件格式示例

### B.1 消息流事件

```json
{"type":"agent_message_chunk","session_id":"acp:abc12345","seq":1,"data":{"text":"Hello"}}
```

### B.2 工具调用事件

```json
{"type":"tool_call","session_id":"acp:abc12345","seq":2,"data":{"tool":"shell","status":"running"}}
{"type":"tool_call_update","session_id":"acp:abc12345","seq":3,"data":{"tool":"shell","status":"completed","output":"file created"}}
```

### B.3 完成事件

```json
{"type":"done","session_id":"acp:abc12345","seq":10,"data":{"reason":"stop"}}
```

### B.4 错误事件

```json
{"type":"error","session_id":"acp:abc12345","error":{"code":-32603,"message":"LLM call failed"}}
```
