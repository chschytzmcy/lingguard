# LingGuard 微信渠道 (QClaw AGP) 实现方案

## 1. 背景与目标

### 1.1 项目背景

LingGuard 需要实现微信消息渠道，让用户可以通过微信与 Agent 交互。腾讯 QClaw（管家 OpenClaw）提供了微信接入能力，我们需要实现与其对接的 AGP (Agent Gateway Protocol) 客户端。

### 1.2 参考项目

| 项目 | 地址 | 说明 |
|------|------|------|
| wechat-access-unqclawed | https://github.com/HenryXiaoYang/wechat-access-unqclawed | OpenClaw 微信通路插件，**主要参考** |
| qclaw-wechat-client | https://github.com/photon-hq/qclaw-wechat-client | QClaw 协议逆向，AGP 客户端实现参考 |

### 1.3 架构关系说明

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          腾讯 QClaw 后台服务                              │
│                    wss://mmgrcalltoken.3g.qq.com/agentwss               │
│                         (WebSocket Server)                               │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    │                               │
                    ▼                               ▼
        ┌───────────────────┐           ┌───────────────────┐
        │ QClaw Desktop App │           │     LingGuard      │
        │   (Electron 应用)  │           │   (本方案实现)      │
        │                   │           │                    │
        │  内置 OpenClaw    │           │  WeChat Channel    │
        │  + wechat-access  │           │  (AGP Client)      │
        │    插件            │           │                    │
        └───────────────────┘           └───────────────────┘
                    │
                    │ 配置写入
                    ▼
        ┌───────────────────┐
        │ 独立部署 OpenClaw  │
        │ + wechat-access   │
        │    插件            │
        └───────────────────┘
```

**关键点：**
- QClaw 和 OpenClaw 是**两个独立进程**
- QClaw Desktop 登录后获取 `channelToken`，写入 OpenClaw 配置
- OpenClaw 的 wechat-access 插件读取配置，作为 **WebSocket 客户端** 连接腾讯服务器
- **LingGuard 不在 QClaw 生态内**，需要用户手动获取 `channelToken` 并配置

---

## 2. AGP 协议详解

### 2.1 连接

```
WebSocket URL: wss://mmgrcalltoken.3g.qq.com/agentwss?token={channelToken}
```

### 2.2 消息格式

所有消息都是 JSON 文本帧，统一信封格式：

```json
{
  "msg_id": "uuid-v4",
  "guid": "device-guid",
  "user_id": "user-id",
  "method": "session.prompt",
  "payload": { ... }
}
```

### 2.3 下行消息 (Server → Client)

| Method | 说明 | Payload 结构 |
|--------|------|-------------|
| `session.prompt` | 用户发消息 | `{ session_id, prompt_id, agent_app, content[] }` |
| `session.cancel` | 取消对话 | `{ session_id, prompt_id }` |

**session.prompt payload 详解：**
```json
{
  "session_id": "会话ID",
  "prompt_id": "消息ID",
  "agent_app": "agent标识",
  "content": [
    { "type": "text", "text": "用户消息内容" }
  ]
}
```

### 2.4 上行消息 (Client → Server)

| Method | 说明 | Payload 结构 |
|--------|------|-------------|
| `session.update` | 流式输出块 | `{ session_id, prompt_id, update_type, content }` |
| `session.promptResponse` | 最终响应 | `{ session_id, prompt_id, stop_reason, content[] }` |

**session.update payload 详解：**
```json
{
  "session_id": "会话ID",
  "prompt_id": "消息ID",
  "update_type": "message_chunk",  // 或 "tool_call", "tool_call_update"
  "content": { "type": "text", "text": "部分响应文本" }
}
```

**session.promptResponse payload 详解：**
```json
{
  "session_id": "会话ID",
  "prompt_id": "消息ID",
  "stop_reason": "end_turn",  // 或 "cancelled", "error", "refusal"
  "content": [
    { "type": "text", "text": "完整响应文本" }
  ]
}
```

### 2.5 心跳机制

- 使用**原生 WebSocket ping/pong**（非应用层心跳）
- 间隔: 20 秒
- 超时: 2x 间隔未收到 pong → 断开重连

### 2.6 重连机制

- **指数退避**: 基础 3s，倍率 1.5，最大 25s
- **系统休眠检测**: 定时器漂移 > 15s → 强制重连
- **消息去重**: 缓存已处理的 msg_id，每 5 分钟清理（最多 1000 条）

---

## 3. 消息处理流程

### 3.1 完整交互流程

```
用户(微信)                QClaw Server              LingGuard
    │                          │                        │
    │──── 发送消息 ────────────▶                        │
    │                          │── session.prompt ─────▶│
    │                          │                        │
    │                          │                        │── 调用 Agent
    │                          │◀─ session.update ──────│   (流式输出)
    │                          │◀─ session.update ──────│
    │                          │◀─ session.update ──────│
    │                          │                        │
    │                          │◀─ session.             │
    │                          │    promptResponse ─────│
    │                          │                        │
    │◀─────── 响应消息 ────────│                        │
```

### 3.2 处理 session.prompt

```go
func (c *WeChatChannel) handlePrompt(envelope *AGPEnvelope) {
    var payload PromptPayload
    json.Unmarshal(envelope.Payload, &payload)

    // 1. 提取文本内容
    text := extractTextFromContent(payload.Content)

    // 2. 构建消息
    msg := &Message{
        SessionID: payload.SessionID,
        PromptID:  payload.PromptID,
        Content:   text,
        Metadata: map[string]interface{}{
            "agent_app": payload.AgentApp,
            "guid":      envelope.GUID,
            "user_id":   envelope.UserID,
        },
    }

    // 3. 调用 Agent (流式)
    go c.processWithAgent(msg)
}
```

### 3.3 流式响应

```go
func (c *WeChatChannel) processWithAgent(msg *Message) {
    // 获取流式响应
    stream := c.handler.HandleStream(msg)

    var fullText strings.Builder

    for chunk := range stream {
        fullText.WriteString(chunk.Content)

        // 发送 session.update
        c.sendUpdate(msg.SessionID, msg.PromptID, chunk.Content)
    }

    // 发送 session.promptResponse
    c.sendPromptResponse(msg.SessionID, msg.PromptID, fullText.String(), "end_turn")
}
```

---

## 4. 实现方案

### 4.1 文件结构

```
internal/
├── config/
│   └── config.go              # 修改: 添加 WeChatConfig
├── channels/
│   ├── wechat.go              # 新建: WeChat Channel 主实现
│   └── wechat_protocol.go     # 新建: AGP 协议消息结构
cmd/cli/
└── gateway.go                 # 修改: 注册 WeChat 渠道
```

### 4.2 Step 1: 配置结构

**文件:** `internal/config/config.go`

```go
// ChannelsConfig 渠道配置
type ChannelsConfig struct {
    Feishu  *FeishuConfig  `json:"feishu,omitempty"`
    QQ      *QQConfig      `json:"qq,omitempty"`
    WeChat  *WeChatConfig  `json:"wechat,omitempty"`  // 新增
}

// WeChatConfig 微信渠道配置
type WeChatConfig struct {
    // 基础配置
    Enabled      bool   `json:"enabled"`
    ChannelToken string `json:"channelToken"`           // 必填: openclaw_channel_token
    WsURL        string `json:"wsUrl,omitempty"`        // 可选: 默认为生产环境URL

    // 身份标识 (可选，用于上行消息回填)
    GUID         string `json:"guid,omitempty"`         // 设备标识
    UserID       string `json:"userId,omitempty"`       // 用户ID

    // 连接参数
    HeartbeatInterval   int `json:"heartbeatInterval,omitempty"`   // 心跳间隔(秒)，默认20
    ReconnectBaseDelay  int `json:"reconnectBaseDelay,omitempty"`  // 重连基础延迟(秒)，默认3
    ReconnectMaxDelay   int `json:"reconnectMaxDelay,omitempty"`   // 重连最大延迟(秒)，默认25

    // 消息过滤
    AllowFrom    []string `json:"allowFrom,omitempty"`    // 白名单用户ID
}
```

**配置示例:**
```json
{
  "channels": {
    "wechat": {
      "enabled": true,
      "channelToken": "your-openclaw-channel-token-here",
      "guid": "device-001",
      "userId": "user-001"
    }
  }
}
```

### 4.3 Step 2: AGP 协议消息结构

**文件:** `internal/channels/wechat_protocol.go`

```go
package channels

import "encoding/json"

// AGPMethod AGP 消息方法类型
type AGPMethod string

const (
    MethodPrompt          AGPMethod = "session.prompt"
    MethodCancel          AGPMethod = "session.cancel"
    MethodUpdate          AGPMethod = "session.update"
    MethodPromptResponse  AGPMethod = "session.promptResponse"
)

// UpdateType session.update 的类型
type UpdateType string

const (
    UpdateTypeMessageChunk  UpdateType = "message_chunk"
    UpdateTypeToolCall      UpdateType = "tool_call"
    UpdateTypeToolCallUpdate UpdateType = "tool_call_update"
)

// StopReason session.promptResponse 的停止原因
type StopReason string

const (
    StopReasonEndTurn   StopReason = "end_turn"
    StopReasonCancelled StopReason = "cancelled"
    StopReasonError     StopReason = "error"
    StopReasonRefusal   StopReason = "refusal"
)

// AGPEnvelope AGP 消息信封
type AGPEnvelope struct {
    MsgID   string          `json:"msg_id"`
    GUID    string          `json:"guid,omitempty"`
    UserID  string          `json:"user_id,omitempty"`
    Method  AGPMethod       `json:"method"`
    Payload json.RawMessage `json:"payload"`
}

// ContentBlock 内容块
type ContentBlock struct {
    Type string `json:"type"`  // "text"
    Text string `json:"text"`
}

// PromptPayload session.prompt 下行消息载荷
type PromptPayload struct {
    SessionID string        `json:"session_id"`
    PromptID  string        `json:"prompt_id"`
    AgentApp  string        `json:"agent_app"`
    Content   []ContentBlock `json:"content"`
}

// CancelPayload session.cancel 下行消息载荷
type CancelPayload struct {
    SessionID string `json:"session_id"`
    PromptID  string `json:"prompt_id"`
}

// UpdatePayload session.update 上行消息载荷
type UpdatePayload struct {
    SessionID  string       `json:"session_id"`
    PromptID   string       `json:"prompt_id"`
    UpdateType UpdateType   `json:"update_type"`
    Content    *ContentBlock `json:"content,omitempty"`
}

// PromptResponsePayload session.promptResponse 上行消息载荷
type PromptResponsePayload struct {
    SessionID  string         `json:"session_id"`
    PromptID   string         `json:"prompt_id"`
    StopReason StopReason     `json:"stop_reason"`
    Content    []ContentBlock `json:"content,omitempty"`
}

// ExtractText 从内容块中提取文本
func ExtractText(blocks []ContentBlock) string {
    var texts []string
    for _, block := range blocks {
        if block.Type == "text" {
            texts = append(texts, block.Text)
        }
    }
    // 用换行符连接多个文本块
    result := ""
    for i, t := range texts {
        if i > 0 {
            result += "\n"
        }
        result += t
    }
    return result
}
```

### 4.4 Step 3: WeChat Channel 实现

**文件:** `internal/channels/wechat.go`

```go
package channels

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"

    "github.com/gorilla/websocket"
    "github.com/google/uuid"

    "lingguard/internal/config"
)

const (
    DefaultWsURL             = "wss://mmgrcalltoken.3g.qq.com/agentwss"
    DefaultHeartbeatInterval = 20 * time.Second
    DefaultReconnectBase     = 3 * time.Second
    DefaultReconnectMax      = 25 * time.Second
    SystemWakeupThreshold    = 15 * time.Second
)

// WeChatChannel 微信渠道
type WeChatChannel struct {
    cfg     *config.WeChatConfig
    handler MessageHandler

    // WebSocket 连接
    conn    *websocket.Conn
    connMu  sync.Mutex
    running bool

    // 心跳
    heartbeatInterval time.Duration
    lastPongTime      time.Time

    // 重连
    reconnectBase     time.Duration
    reconnectMax      time.Duration
    reconnectAttempts int

    // 消息去重
    processedMsgs map[string]time.Time
    msgsMu        sync.RWMutex

    // 生命周期
    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}

// NewWeChatChannel 创建微信渠道
func NewWeChatChannel(cfg *config.WeChatConfig, handler MessageHandler) *WeChatChannel {
    ctx, cancel := context.WithCancel(context.Background())

    // 设置默认值
    heartbeatInterval := DefaultHeartbeatInterval
    if cfg.HeartbeatInterval > 0 {
        heartbeatInterval = time.Duration(cfg.HeartbeatInterval) * time.Second
    }

    reconnectBase := DefaultReconnectBase
    if cfg.ReconnectBaseDelay > 0 {
        reconnectBase = time.Duration(cfg.ReconnectBaseDelay) * time.Second
    }

    reconnectMax := DefaultReconnectMax
    if cfg.ReconnectMaxDelay > 0 {
        reconnectMax = time.Duration(cfg.ReconnectMaxDelay) * time.Second
    }

    return &WeChatChannel{
        cfg:               cfg,
        handler:           handler,
        heartbeatInterval: heartbeatInterval,
        reconnectBase:     reconnectBase,
        reconnectMax:      reconnectMax,
        processedMsgs:     make(map[string]time.Time),
        ctx:               ctx,
        cancel:            cancel,
    }
}

// Start 启动渠道
func (c *WeChatChannel) Start() error {
    if !c.cfg.Enabled {
        return nil
    }

    if c.cfg.ChannelToken == "" {
        return fmt.Errorf("wechat channel enabled but channelToken not configured")
    }

    c.running = true
    go c.connectLoop()
    return nil
}

// Stop 停止渠道
func (c *WeChatChannel) Stop() {
    c.running = false
    c.cancel()
    c.wg.Wait()

    c.connMu.Lock()
    if c.conn != nil {
        c.conn.Close()
    }
    c.connMu.Unlock()
}

// Name 返回渠道名称
func (c *WeChatChannel) Name() string {
    return "wechat"
}

// connectLoop 连接循环 (带重连)
func (c *WeChatChannel) connectLoop() {
    for c.running {
        err := c.connect()
        if err != nil {
            // 计算重连延迟 (指数退避)
            delay := c.reconnectBase * time.Duration(1<<uint(c.reconnectAttempts))
            if delay > c.reconnectMax {
                delay = c.reconnectMax
            }
            c.reconnectAttempts++

            select {
            case <-time.After(delay):
            case <-c.ctx.Done():
                return
            }
        }
    }
}

// connect 建立连接
func (c *WeChatChannel) connect() error {
    wsURL := c.cfg.WsURL
    if wsURL == "" {
        wsURL = DefaultWsURL
    }
    wsURL = fmt.Sprintf("%s?token=%s", wsURL, c.cfg.ChannelToken)

    conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
    if err != nil {
        return fmt.Errorf("websocket dial failed: %w", err)
    }

    c.connMu.Lock()
    c.conn = conn
    c.connMu.Unlock()

    // 连接成功，重置重连计数
    c.reconnectAttempts = 0
    c.lastPongTime = time.Now()

    // 设置 pong 处理器
    conn.SetPongHandler(func(string) error {
        c.lastPongTime = time.Now()
        return nil
    })

    // 启动心跳和消息处理
    c.wg.Add(2)
    go c.heartbeatLoop()
    go c.messageLoop()

    return nil
}

// heartbeatLoop 心跳循环
func (c *WeChatChannel) heartbeatLoop() {
    defer c.wg.Done()

    ticker := time.NewTicker(c.heartbeatInterval)
    defer ticker.Stop()

    lastTick := time.Now()

    for {
        select {
        case now := <-ticker.C:
            // 检测系统休眠唤醒
            drift := now.Sub(lastTick)
            if drift > c.heartbeatInterval+SystemWakeupThreshold {
                // 系统休眠后唤醒，强制重连
                c.closeConnection()
                return
            }
            lastTick = now

            // 检查 pong 超时
            if time.Since(c.lastPongTime) > 2*c.heartbeatInterval {
                c.closeConnection()
                return
            }

            // 发送 ping
            c.connMu.Lock()
            if c.conn != nil {
                c.conn.WriteMessage(websocket.PingMessage, nil)
            }
            c.connMu.Unlock()

        case <-c.ctx.Done():
            return
        }
    }
}

// messageLoop 消息循环
func (c *WeChatChannel) messageLoop() {
    defer c.wg.Done()
    defer c.closeConnection()

    for {
        select {
        case <-c.ctx.Done():
            return
        default:
            c.connMu.Lock()
            conn := c.conn
            c.connMu.Unlock()

            if conn == nil {
                return
            }

            _, message, err := conn.ReadMessage()
            if err != nil {
                return
            }

            c.handleMessage(message)
        }
    }
}

// handleMessage 处理消息
func (c *WeChatChannel) handleMessage(data []byte) {
    var envelope AGPEnvelope
    if err := json.Unmarshal(data, &envelope); err != nil {
        return
    }

    // 消息去重
    if c.isProcessed(envelope.MsgID) {
        return
    }
    c.markProcessed(envelope.MsgID)

    switch envelope.Method {
    case MethodPrompt:
        c.handlePrompt(&envelope)
    case MethodCancel:
        c.handleCancel(&envelope)
    }
}

// handlePrompt 处理 session.prompt
func (c *WeChatChannel) handlePrompt(envelope *AGPEnvelope) {
    var payload PromptPayload
    if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
        return
    }

    // 提取文本
    text := ExtractText(payload.Content)

    // 构建消息
    msg := &Message{
        SessionID: payload.SessionID,
        PromptID:  payload.PromptID,
        Content:   text,
        Metadata: map[string]interface{}{
            "agent_app": payload.AgentApp,
            "guid":      envelope.GUID,
            "user_id":   envelope.UserID,
        },
    }

    // 异步处理
    go c.processPrompt(msg)
}

// processPrompt 处理消息并流式响应
func (c *WeChatChannel) processPrompt(msg *Message) {
    // 调用 Agent 流式处理
    stream := c.handler.HandleStream(msg)

    var fullText string

    for chunk := range stream {
        fullText += chunk.Content
        c.sendUpdate(msg.SessionID, msg.PromptID, chunk.Content)
    }

    c.sendPromptResponse(msg.SessionID, msg.PromptID, fullText, StopReasonEndTurn)
}

// handleCancel 处理 session.cancel
func (c *WeChatChannel) handleCancel(envelope *AGPEnvelope) {
    var payload CancelPayload
    if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
        return
    }

    // TODO: 取消正在进行的处理

    c.sendPromptResponse(payload.SessionID, payload.PromptID, "", StopReasonCancelled)
}

// sendUpdate 发送 session.update
func (c *WeChatChannel) sendUpdate(sessionID, promptID, text string) {
    payload := UpdatePayload{
        SessionID:  sessionID,
        PromptID:   promptID,
        UpdateType: UpdateTypeMessageChunk,
        Content: &ContentBlock{
            Type: "text",
            Text: text,
        },
    }

    c.sendMessage(MethodUpdate, payload)
}

// sendPromptResponse 发送 session.promptResponse
func (c *WeChatChannel) sendPromptResponse(sessionID, promptID, text string, reason StopReason) {
    var content []ContentBlock
    if text != "" {
        content = []ContentBlock{{Type: "text", Text: text}}
    }

    payload := PromptResponsePayload{
        SessionID:  sessionID,
        PromptID:   promptID,
        StopReason: reason,
        Content:    content,
    }

    c.sendMessage(MethodPromptResponse, payload)
}

// sendMessage 发送消息
func (c *WeChatChannel) sendMessage(method AGPMethod, payload interface{}) {
    payloadBytes, err := json.Marshal(payload)
    if err != nil {
        return
    }

    envelope := AGPEnvelope{
        MsgID:   uuid.New().String(),
        GUID:    c.cfg.GUID,
        UserID:  c.cfg.UserID,
        Method:  method,
        Payload: payloadBytes,
    }

    data, err := json.Marshal(envelope)
    if err != nil {
        return
    }

    c.connMu.Lock()
    if c.conn != nil {
        c.conn.WriteMessage(websocket.TextMessage, data)
    }
    c.connMu.Unlock()
}

// closeConnection 关闭连接
func (c *WeChatChannel) closeConnection() {
    c.connMu.Lock()
    if c.conn != nil {
        c.conn.Close()
        c.conn = nil
    }
    c.connMu.Unlock()
}

// isProcessed 检查消息是否已处理
func (c *WeChatChannel) isProcessed(msgID string) bool {
    c.msgsMu.RLock()
    defer c.msgsMu.RUnlock()
    _, exists := c.processedMsgs[msgID]
    return exists
}

// markProcessed 标记消息已处理
func (c *WeChatChannel) markProcessed(msgID string) {
    c.msgsMu.Lock()
    defer c.msgsMu.Unlock()

    c.processedMsgs[msgID] = time.Now()

    // 清理超过 5 分钟的记录
    if len(c.processedMsgs) > 1000 {
        cutoff := time.Now().Add(-5 * time.Minute)
        for id, t := range c.processedMsgs {
            if t.Before(cutoff) {
                delete(c.processedMsgs, id)
            }
        }
    }
}
```

### 4.5 Step 4: 注册 Channel

**文件:** `cmd/cli/gateway.go`

在 gateway 启动逻辑中添加:

```go
// 注册微信渠道
if cfg.Channels.WeChat != nil && cfg.Channels.WeChat.Enabled {
    if cfg.Channels.WeChat.ChannelToken == "" {
        return nil, fmt.Errorf("wechat channel enabled but channelToken not configured")
    }
    mgr.RegisterChannel(channels.NewWeChatChannel(cfg.Channels.WeChat, handler))
    logger.Info("WeChat channel registered")
}
```

---

## 5. 依赖管理

需要添加 WebSocket 客户端依赖:

```bash
go get github.com/gorilla/websocket
go get github.com/google/uuid
```

---

## 6. 验证方法

### 6.1 获取 channelToken

使用 qclaw-wechat-client 获取 token:

```bash
# 安装
npm install -g qclaw-wechat-client

# 或使用 pnpm
pnpm demo  # 交互式登录 + echo bot
```

或参考 qclaw-wechat-client 文档实现自定义登录流程。

### 6.2 配置 LingGuard

编辑配置文件 `~/.lingguard/config.json`:

```json
{
  "channels": {
    "wechat": {
      "enabled": true,
      "channelToken": "your-openclaw-channel-token-here"
    }
  }
}
```

### 6.3 启动测试

```bash
lingguard gateway
```

通过微信向已授权的账号发送消息，验证 Agent 是否正确响应。

---

## 7. 文件修改清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/config/config.go` | 修改 | 添加 WeChatConfig 结构 |
| `internal/channels/wechat_protocol.go` | 新建 | AGP 协议消息结构定义 |
| `internal/channels/wechat.go` | 新建 | WeChat Channel 完整实现 |
| `cmd/cli/gateway.go` | 修改 | 注册 WeChat 渠道 |
| `go.mod` | 修改 | 添加 gorilla/websocket, google/uuid 依赖 |

---

## 8. 后续优化

- [ ] 支持白名单过滤 (AllowFrom 配置)
- [ ] 添加连接状态监控和日志
- [ ] 支持 tool_call 类型的 session.update
- [ ] 实现取消正在进行的 Agent 调用
- [ ] 添加单元测试
