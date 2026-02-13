# LingGuard - 个人智能助手架构设计文档

## 1. 项目概述

### 1.1 项目名称
**LingGuard** - 一款基于Go语言的超轻量级个人AI智能助手

### 1.2 设计理念
参考 [nanobot](https://github.com/HKUDS/nanobot) 项目的设计思想，打造一个：
- **极简轻量**：核心代码控制在5000行以内
- **高性能**：充分利用Go的并发特性
- **易扩展**：模块化设计，支持插件机制
- **企业友好**：支持飞书等企业级即时通讯平台

### 1.3 核心特性
| 特性 | 描述 |
|------|------|
| 渠道接入 | 飞书（支持WebSocket长连接，无需公网IP） |
| 多LLM支持 | OpenAI, Anthropic, DeepSeek, GLM, Qwen 等 |
| Provider自动匹配 | 根据模型名自动选择合适的 Provider |
| 会话管理 | 内存会话管理，支持历史消息窗口 |
| 技能系统 | 可扩展的工具和技能插件 |
| 记忆系统 | 持久化对话记忆和上下文管理 |
| 定时任务 | Cron风格的定时执行 |
| 安全沙箱 | 工作空间限制和权限控制 |

---

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLI / Gateway                            │
│                    (命令行 & 网关入口)                            │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                           Bus Layer                              │
│                      (消息路由层)                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │  Router  │  │ Dispatcher│  │  Queue   │  │  Events  │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
└─────────────────────────────────────────────────────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        ▼                       ▼                       ▼
┌───────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Channels    │     │      Agent      │     │   Scheduler     │
│  (渠道适配层)  │     │   (核心代理)     │     │   (定时任务)     │
│ ┌───────────┐ │     │ ┌─────────────┐ │     │ ┌─────────────┐ │
│ │  Feishu   │ │     │ │   Loop      │ │     │ │    Cron     │ │
│ │ (WebSocket)│ │     │ │   Session   │ │     │ │  Heartbeat  │ │
│ └───────────┘ │     │ │   Context   │ │     │ └─────────────┘ │
└───────────────┘     │ │   Memory    │ │     └─────────────────┘
                      │ │   Tools     │ │
                      │ └─────────────┘ │
                      └─────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Providers Layer                           │
│                      (LLM提供商层)                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │  OpenAI  │  │ Anthropic│  │DeepSeek  │  │   GLM    │        │
│  │  Qwen    │  │ MiniMax  │  │Moonshot  │  │  vLLM    │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
│                                                                  │
│  Provider 自动匹配：根据模型名自动选择 Provider                    │
│  - "gpt-4o" → openai                                            │
│  - "claude-*" → anthropic                                        │
│  - "qwen-*" → qwen                                               │
│  - "glm-*" → glm                                                 │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Infrastructure                             │
│                        (基础设施层)                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │  Config  │  │  Storage │  │  Logger  │  │ Security │        │
│  │  Cache   │  │  Vector  │  │ Metrics  │  │ Sandbox  │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 数据流架构

```
用户消息 ──▶ Feishu ──▶ Bus ──▶ Agent Loop
                                      │
                                      ▼
                              ┌──────────────┐
                              │ 构建Context  │
                              │ (System +    │
                              │  Session +   │
                              │  MemoryWindow)│
                              └──────────────┘
                                      │
                                      ▼
                              ┌──────────────┐
                              │   LLM调用    │
                              │ (Provider    │
                              │  自动匹配)   │
                              └──────────────┘
                                      │
                    ┌─────────────────┼─────────────────┐
                    ▼                 ▼                 ▼
              ┌──────────┐      ┌──────────┐     ┌──────────┐
              │ 文本响应 │      │ 工具调用 │     │ 技能触发 │
              └──────────┘      └──────────┘     └──────────┘
                    │                 │                 │
                    │                 ▼                 │
                    │           ┌──────────┐           │
                    │           │ 执行工具 │           │
                    │           │(MaxIter) │           │
                    │           └──────────┘           │
                    │                 │                 │
                    └─────────────────┼─────────────────┘
                                      ▼
                              ┌──────────────┐
                              │ 更新Session  │
                              └──────────────┘
                                      │
                                      ▼
                              响应 ──▶ Feishu ──▶ 用户
```

---

## 3. 核心模块设计

### 3.1 目录结构

```
lingguard/
├── cmd/
│   ├── lingguard/          # 主程序入口
│   │   └── main.go
│   └── cli/                # CLI命令
│       ├── root.go         # 根命令
│       ├── agent.go        # agent交互命令
│       ├── gateway.go      # 网关启动命令
│       ├── cron.go         # 定时任务管理
│       └── status.go       # 状态查看
├── internal/
│   ├── agent/              # 核心代理逻辑
│   │   ├── agent.go        # Agent主结构
│   │   ├── loop.go         # Agent执行循环
│   │   └── context.go      # 上下文构建
│   ├── session/            # 会话管理（已实现）
│   │   ├── manager.go      # 会话管理器
│   │   └── session.go      # 会话结构
│   ├── tools/              # 内置工具（已实现）
│   │   ├── registry.go     # 工具注册中心
│   │   ├── shell.go        # Shell执行
│   │   └── file.go         # 文件操作
│   ├── providers/          # LLM提供商（已实现）
│   │   ├── provider.go     # Provider接口
│   │   ├── registry.go     # 提供商注册
│   │   ├── spec.go         # Provider规范（自动匹配）
│   │   ├── openai.go       # OpenAI兼容
│   │   └── anthropic.go    # Anthropic兼容
│   ├── channels/           # 渠道集成
│   │   ├── channel.go      # Channel接口
│   │   ├── manager.go      # 渠道管理
│   │   └── feishu.go       # 飞书
│   ├── bus/                # 消息总线
│   │   ├── bus.go          # 总线核心
│   │   ├── router.go       # 消息路由
│   │   └── events.go       # 事件系统
│   ├── skills/             # 技能系统（待实现）
│   │   ├── loader.go       # 技能加载器
│   │   └── manager.go      # 技能管理器
│   ├── scheduler/          # 定时任务（待实现）
│   │   ├── scheduler.go    # 调度器
│   │   └── cron.go         # Cron解析
│   └── config/             # 配置管理（已实现）
│       └── config.go       # 配置结构
├── pkg/
│   ├── llm/                # LLM客户端封装（已实现）
│   │   ├── llm.go          # 通用类型
│   │   └── stream.go       # 流式响应
│   ├── memory/             # 记忆系统（已实现）
│   │   └── memory.go       # 内存存储
│   ├── logger/             # 日志（已实现）
│   │   └── logger.go
│   └── feishu/             # 飞书SDK封装（待实现）
│       ├── client.go       # 飞书客户端
│       └── websocket.go    # WebSocket连接
├── configs/
│   ├── config.json         # 实际配置
│   └── config.example.json # 配置示例
├── docs/
│   ├── ARCHITECTURE.md     # 架构文档
│   └── API.md              # API文档
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### 3.2 核心接口定义

#### 3.2.1 Provider接口 (LLM提供商)

```go
// internal/providers/provider.go

package providers

import (
    "context"
    "github.com/lingguard/pkg/llm"
)

// Provider LLM提供商接口
type Provider interface {
    // Name 返回提供商名称
    Name() string

    // Model 返回当前使用的模型
    Model() string

    // Complete 发送消息并获取完成响应
    Complete(ctx context.Context, req *llm.Request) (*llm.Response, error)

    // Stream 发送消息并获取流式响应
    Stream(ctx context.Context, req *llm.Request) (<-chan llm.StreamEvent, error)

    // SupportsTools 是否支持工具调用
    SupportsTools() bool

    // SupportsVision 是否支持视觉
    SupportsVision() bool
}

// ProviderConfig 提供商配置
type ProviderConfig struct {
    APIKey      string
    APIBase     string
    Model       string
    Temperature float64
    MaxTokens   int
}
```

#### 3.2.2 Provider 自动匹配

```go
// internal/providers/spec.go

package providers

// ProviderSpec 定义 Provider 的匹配规则
type ProviderSpec struct {
    Name         string   // 配置中的 provider 名称
    Keywords     []string // 模型名关键词（用于自动匹配）
    APIKeyPrefix string   // API Key 前缀
}

// BuiltinSpecs 内置 Provider 规范
var BuiltinSpecs = []ProviderSpec{
    {Name: "openai", Keywords: []string{"gpt", "o1", "o3"}},
    {Name: "anthropic", Keywords: []string{"claude"}},
    {Name: "deepseek", Keywords: []string{"deepseek"}},
    {Name: "qwen", Keywords: []string{"qwen", "tongyi", "dashscope"}},
    {Name: "glm", Keywords: []string{"glm", "chatglm", "codegeex"}},
    {Name: "minimax", Keywords: []string{"minimax"}},
    {Name: "moonshot", Keywords: []string{"moonshot", "kimi"}},
    {Name: "gemini", Keywords: []string{"gemini"}},
    {Name: "groq", Keywords: []string{"llama", "mixtral", "gemma"}, APIKeyPrefix: "gsk_"},
}

// FindSpecByModel 根据模型名查找 Provider 规范
func FindSpecByModel(model string) *ProviderSpec
```

#### 3.2.3 Registry Provider 注册表

```go
// internal/providers/registry.go

// Registry 提供商注册表
type Registry struct {
    providers   map[string]Provider
    defaultName string
}

// MatchProvider 根据模型名自动匹配 Provider
func (r *Registry) MatchProvider(model string) (Provider, bool) {
    // 1. 尝试解析 "provider/model" 格式
    // 2. 检查 model 是否是已注册的 provider 名称
    // 3. 通过关键词匹配
    // 4. 返回默认 Provider
}

// SetDefault 设置默认 Provider
func (r *Registry) SetDefault(name string)
```

#### 3.2.4 会话管理

```go
// internal/session/manager.go

package session

// Session 会话
type Session struct {
    Key       string
    Messages  []*memory.Message
    CreatedAt time.Time
    UpdatedAt time.Time
}

// Manager 会话管理器
type Manager struct {
    store    memory.Store
    sessions map[string]*Session
    window   int // 历史消息窗口大小
}

// GetOrCreate 获取或创建会话
func (m *Manager) GetOrCreate(key string) *Session

// AddMessage 添加消息
func (s *Session) AddMessage(role, content string)

// GetHistory 获取历史消息（限制窗口大小）
func (s *Session) GetHistory(window int) []*memory.Message

// Clear 清空会话
func (s *Session) Clear()
```

#### 3.2.5 配置结构

```go
// internal/config/config.go

// AgentsConfig 代理配置（已更新）
type AgentsConfig struct {
    Workspace         string  `json:"workspace"`
    Model             string  `json:"model"`             // 默认模型/Provider名称
    MaxTokens         int     `json:"maxTokens"`         // 最大输出 tokens
    Temperature       float64 `json:"temperature"`       // 温度参数
    MaxToolIterations int     `json:"maxToolIterations"` // 最大工具迭代次数
    MemoryWindow      int     `json:"memoryWindow"`      // 历史消息窗口大小
    SystemPrompt      string  `json:"systemPrompt"`
}
```

### 3.3 Agent核心实现

```go
// internal/agent/agent.go

package agent

// Agent 核心代理结构
type Agent struct {
    id           string
    provider     providers.Provider
    toolRegistry *tools.Registry
    sessions     *session.Manager  // 会话管理
    config       *config.AgentsConfig
}

// NewAgent 创建新代理
func NewAgent(cfg *config.AgentsConfig, provider providers.Provider) *Agent {
    return &Agent{
        id:           generateID(),
        provider:     provider,
        toolRegistry: tools.NewRegistry(),
        sessions:     session.NewManager(memory.NewMemoryStore(), cfg.MemoryWindow),
        config:       cfg,
    }
}

// ProcessMessage 处理消息
func (a *Agent) ProcessMessage(ctx context.Context, sessionID, userMessage string) (string, error) {
    // 1. 获取或创建会话并添加用户消息
    s := a.sessions.GetOrCreate(sessionID)
    s.AddMessage("user", userMessage)

    // 2. 构建上下文
    messages, err := a.buildContext(sessionID)
    if err != nil {
        return "", fmt.Errorf("failed to build context: %w", err)
    }

    // 3. 执行代理循环
    return a.runLoop(ctx, sessionID, messages)
}

// buildContext 构建上下文
func (a *Agent) buildContext(sessionID string) ([]llm.Message, error) {
    messages := make([]llm.Message, 0)

    // 添加系统提示
    if a.config.SystemPrompt != "" {
        messages = append(messages, llm.Message{
            Role:    "system",
            Content: a.config.SystemPrompt,
        })
    }

    // 获取会话历史消息（使用 MemoryWindow）
    s := a.sessions.GetOrCreate(sessionID)
    for _, msg := range s.GetHistory(a.config.MemoryWindow) {
        messages = append(messages, llm.Message{
            Role:    msg.Role,
            Content: msg.Content,
        })
    }

    return messages, nil
}

// runLoop 代理执行循环
func (a *Agent) runLoop(ctx context.Context, sessionID string, messages []llm.Message) (string, error) {
    iterations := 0
    maxIterations := a.config.MaxToolIterations
    if maxIterations <= 0 {
        maxIterations = 10
    }

    for iterations < maxIterations {
        iterations++
        // ... LLM调用和工具执行逻辑
    }

    return "", fmt.Errorf("max iterations reached")
}
```

---

## 4. 配置示例

### 4.1 完整配置文件

```json
{
  "providers": {
    "qwen": {
      "apiKey": "sk-xxx",
      "apiBase": "https://dashscope.aliyuncs.com/compatible-mode/v1",
      "model": "qwen3-max-2026-01-23",
      "temperature": 0.7,
      "maxTokens": 4096
    },
    "glm": {
      "apiKey": "xxx.xxx",
      "apiBase": "https://open.bigmodel.cn/api/anthropic",
      "model": "glm-5",
      "temperature": 0.7,
      "maxTokens": 4096
    },
    "minimax": {
      "apiKey": "xxx",
      "apiBase": "https://api.minimaxi.com/anthropic",
      "model": "MiniMax-M2.5",
      "temperature": 0.7,
      "maxTokens": 4096
    }
  },
  "agents": {
    "workspace": "~/.lingguard/workspace",
    "model": "glm",
    "maxTokens": 8192,
    "temperature": 0.7,
    "maxToolIterations": 20,
    "memoryWindow": 50,
    "systemPrompt": "你是灵侍，一个乐于助人的 AI 助手。你可以使用工具帮助用户完成各种任务。"
  },
  "channels": {
    "feishu": {
      "enabled": true,
      "appId": "cli_xxx",
      "appSecret": "xxx",
      "encryptKey": "",
      "verificationToken": "",
      "allowFrom": []
    }
  },
  "tools": {
    "restrictToWorkspace": false,
    "workspace": "~/.lingguard/workspace"
  },
  "storage": {
    "type": "postgres",
    "host": "localhost",
    "port": 5432,
    "database": "lingguard",
    "username": "postgres",
    "password": "postgres",
    "sslmode": "disable",
    "vectorDbUrl": "http://localhost:6333"
  },
  "logging": {
    "level": "info",
    "format": "text",
    "output": "~/.lingguard/logs/lingguard.log"
  }
}
```

### 4.2 Provider 自动匹配说明

| model 配置值 | 匹配规则 | 使用的 Provider |
|-------------|---------|----------------|
| `"glm"` | 直接匹配 provider 名称 | glm |
| `"glm/glm-4-plus"` | 解析 `provider/model` 格式 | glm |
| `"qwen-max"` | 关键词匹配 `qwen` | qwen |
| `"gpt-4o"` | 关键词匹配 `gpt` | openai |
| `"claude-3-opus"` | 关键词匹配 `claude` | anthropic |
| `"deepseek-chat"` | 关键词匹配 `deepseek` | deepseek |

---

## 5. CLI设计

### 5.1 命令列表

```bash
# 初始化配置
lingguard init

# 与Agent交互
lingguard agent -m "Hello"
lingguard agent  # 交互模式

# 启动网关（连接飞书）
lingguard gateway

# 查看状态
lingguard status
```

### 5.2 状态显示示例

```
LingGuard Status
================
Config: configs/config.json

Providers:
  - glm: glm-5 (configured)
  - qwen: qwen3-max-2026-01-23 (configured)
  - minimax: MiniMax-M2.5 (configured)

Agent:
  Model: glm
  Workspace: ~/.lingguard/workspace
  Max Iterations: 20
  Memory Window: 50

Channels:
  - Feishu: enabled
```

---

## 6. 开发路线图

### Phase 1: 核心功能 ✅ (已完成)

| 功能 | 状态 | 说明 |
|------|------|------|
| 配置结构简化 | ✅ | AgentsConfig 新字段，移除 Mapping |
| Provider 自动匹配 | ✅ | spec.go, MatchProvider() |
| 会话管理 | ✅ | session/manager.go |
| Agent 核心循环 | ✅ | ProcessMessage, runLoop |
| Provider 抽象层 | ✅ | OpenAI/Anthropic 兼容 |
| 基础工具 | ✅ | Shell, File |
| CLI 命令 | ✅ | init, agent, status |
| 内存存储 | ✅ | MemoryStore |

### Phase 2: 渠道集成 (待实现)

| 功能 | 状态 | 说明 |
|------|------|------|
| 飞书 WebSocket 长连接 | ⏳ | 需要实现 |
| 消息收发 | ⏳ | 需要实现 |
| 权限控制 | ⏳ | allowFrom 白名单 |
| Bus 消息路由 | ⏳ | 需要实现 |

### Phase 3: 高级功能 (待实现)

| 功能 | 状态 | 说明 |
|------|------|------|
| 技能系统 | ⏳ | skills/ 模块 |
| 持久化存储 | ⏳ | PostgreSQL |
| 向量记忆 | ⏳ | Qdrant 集成 |
| 定时任务 | ⏳ | scheduler/ 模块 |
| 多模态支持 | ⏳ | Vision |
| 子代理 | ⏳ | Subagent |

### Phase 4: 优化与扩展 (待实现)

| 功能 | 状态 | 说明 |
|------|------|------|
| 性能优化 | ⏳ | 缓存、并发优化 |
| 监控指标 | ⏳ | Prometheus |
| Web 管理界面 | ⏳ | 可选 |
| 流式响应 | ⏳ | SSE 支持 |

---

## 7. 技术选型

| 组件 | 技术选型 | 说明 |
|------|----------|------|
| 语言 | Go 1.23+ | 高性能并发 |
| CLI框架 | Cobra | 成熟的CLI框架 |
| 日志 | Zap | 高性能结构化日志 |
| HTTP客户端 | net/http | 标准库 |
| WebSocket | gorilla/websocket | 飞书长连接 |
| 数据库 | PostgreSQL | 生产级关系型数据库 |
| 向量数据库 | Qdrant | 高性能语义搜索 |

---

## 8. 参考资料

- [nanobot](https://github.com/HKUDS/nanobot) - 参考架构设计
- [OpenAI API](https://platform.openai.com/docs/api-reference) - LLM API规范
- [Anthropic API](https://docs.anthropic.com/) - Claude API
- [飞书开放平台](https://open.feishu.cn/document/) - 飞书开发文档
- [飞书WebSocket长连接](https://open.feishu.cn/document/ukTMukTMukTM/uYjNwUjL2YDM14iN2ATN) - 长连接模式说明
