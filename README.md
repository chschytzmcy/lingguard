# LingGuard

一款基于 Go 语言的超轻量级个人 AI 智能助手，参考 [nanobot](https://github.com/HKUDS/nanobot) 设计。

## 特性

### 核心能力
- **多 LLM 支持** - OpenAI, Anthropic, DeepSeek, GLM, Qwen, MiniMax, Moonshot 等
- **Provider 自动匹配** - 根据模型名/API Key 自动选择合适的 Provider
- **流式响应** - 实时输出，飞书消息实时更新

### 渠道集成
- **飞书** - WebSocket 长连接，无需公网 IP，流式消息卡片
- **QQ** - 预留支持

### 工具系统
- **Shell 工具** - 执行命令，支持安全沙箱
- **文件工具** - 读写、编辑、列表
- **Web 工具** - Brave 搜索、网页抓取
- **MCP 支持** - Model Context Protocol，支持 Stdio 和 HTTP 传输

### 智能能力
- **技能系统** - 渐进式加载，按需注入指令
- **持久化记忆** - MEMORY.md + HISTORY.md 方案
- **子代理** - 后台异步执行复杂任务
- **定时任务** - Cron 调度，支持时区

### 部署优势
- **单二进制部署** - 无运行时依赖
- **低内存占用** - ~20MB 内存

## 快速开始

### 1. 构建

```bash
# 克隆项目
git clone https://github.com/your-org/lingguard.git
cd lingguard

# 构建
go build -o lingguard ./cmd/lingguard
```

### 2. 配置

```bash
# 创建配置目录
mkdir -p ~/.lingguard

# 创建配置文件
cat > ~/.lingguard/config.json << 'EOF'
{
  "providers": {
    "deepseek": {
      "apiKey": "sk-xxx"
    }
  },
  "agents": {
    "provider": "deepseek",
    "systemPrompt": "你是灵侍，一个乐于助人的 AI 助手。"
  }
}
EOF
```

### 3. 运行

```bash
# 交互模式
./lingguard agent

# 单次消息
./lingguard agent -m "你好"

# 启动网关
./lingguard gateway
```

## CLI 命令

### Agent 交互

```bash
# 交互模式
./lingguard agent

# 单次消息
./lingguard agent -m "分析当前目录的代码结构"

# 指定配置文件
./lingguard agent -c /path/to/config.json
```

### Gateway 网关

```bash
# 启动网关
./lingguard gateway
```

### 定时任务

```bash
# 添加 cron 表达式任务
./lingguard cron add "早间简报" "cron:0 9 * * *" "生成今日简报"

# 添加带时区的任务
./lingguard cron add "NYC Morning" "cron:0 9 * * *" "Good morning!" --tz "America/New_York"

# 添加间隔任务
./lingguard cron add "Hourly Check" "every:1h" "检查系统状态"

# 添加一次性任务
./lingguard cron add "Reminder" "at:2026-02-20T10:00:00" "别忘了开会"

# 列出任务
./lingguard cron list

# 删除任务
./lingguard cron remove <job-id>

# 手动执行
./lingguard cron run <job-id> --force
```

### 状态查看

```bash
./lingguard status
```

## 配置

### 配置文件位置（优先级从高到低）

1. 环境变量 `$LINGGUARD_CONFIG`
2. 项目目录 `configs/config.json`
3. 当前目录 `./config.json`
4. 用户目录 `~/.lingguard/config.json`

### 完整配置示例

```json
{
  "providers": {
    "glm": {
      "apiKey": "xxx.xxx",
      "apiBase": "https://open.bigmodel.cn/api/anthropic",
      "model": "glm-5",
      "timeout": 120
    },
    "deepseek": {
      "apiKey": "sk-xxx"
    }
  },
  "agents": {
    "workspace": "~/.lingguard/workspace",
    "provider": "glm",
    "maxToolIterations": 20,
    "memoryWindow": 50,
    "systemPrompt": "你是灵侍，一个乐于助人的 AI 助手。",
    "memory": {
      "enabled": true,
      "recentDays": 3,
      "maxHistoryLines": 1000
    }
  },
  "channels": {
    "feishu": {
      "enabled": true,
      "appId": "cli_xxx",
      "appSecret": "xxx",
      "allowFrom": ["ou_xxx"]
    }
  },
  "tools": {
    "restrictToWorkspace": false,
    "workspace": "~/.lingguard/workspace",
    "braveApiKey": "",
    "webMaxChars": 50000,
    "mcpServers": {
      "filesystem": {
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/home/user/documents"]
      },
      "remote-server": {
        "url": "http://localhost:8765/mcp"
      }
    }
  },
  "cron": {
    "enabled": true,
    "storePath": "~/.lingguard/cron/jobs.json"
  },
  "storage": {
    "type": "file",
    "path": "~/.lingguard/memory"
  },
  "logging": {
    "level": "info",
    "format": "text",
    "output": "~/.lingguard/logs/lingguard.log"
  }
}
```

## 内置工具

| 工具名 | 功能描述 | 危险级别 |
|--------|----------|:--------:|
| `shell` | 执行 Shell 命令 | ⚠️ |
| `file` | 文件读写、编辑、列表 | ⚠️ |
| `web_search` | Brave 搜索 API | - |
| `web_fetch` | 网页抓取、HTML 转 Markdown | - |
| `skill` | 按需加载技能指令 | - |
| `memory` | 记忆操作（添加/搜索） | - |
| `cron` | 定时任务管理 | - |
| `message` | 发送消息到渠道 | - |
| `workspace` | 工作区管理 | - |
| `task_spawn` | 创建子代理任务 | - |
| `task_status` | 查询子代理状态 | - |
| `mcp_*` | MCP 服务器工具 | - |

## MCP 支持

LingGuard 支持 Model Context Protocol (MCP)，可以连接外部工具服务器。

### Stdio 传输

```json
{
  "tools": {
    "mcpServers": {
      "filesystem": {
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/dir"]
      }
    }
  }
}
```

### HTTP 传输

```json
{
  "tools": {
    "mcpServers": {
      "remote": {
        "url": "http://localhost:8765/mcp"
      }
    }
  }
}
```

MCP 工具命名格式: `mcp_{serverName}_{toolName}`

## 技能系统

### 内置技能

| 技能 | 描述 |
|------|------|
| `weather` | 天气查询 (心知天气) |
| `git-workflow` | Git 工作流自动化 |
| `code-review` | 代码审查指南 |
| `file` | 文件操作指南 |
| `system` | 系统操作指南 |
| `moltbook` | AI Agent 社交网络 |

### 技能格式

每个技能是一个目录，包含 `SKILL.md` 文件：

```markdown
---
name: skill-name
description: Skill description
homepage: https://example.com
metadata: {"nanobot":{"emoji":"🦞","requires":{"bins":["curl"]}}}
---

# Skill Title

Skill instructions here...
```

### 渐进式加载

- 默认只注入技能摘要到系统提示
- `always=true` 的技能自动加载完整内容
- 其他技能通过 `skill` 工具按需加载

## 记忆系统

参考 nanobot 的文件持久化记忆方案：

```
~/.lingguard/memory/
├── MEMORY.md          # 长期记忆（用户偏好、重要事实）
├── HISTORY.md         # 事件日志
└── 2026-02-16.md      # 每日日志
```

### 记忆工具

```
memory add --category "User Preferences" --content "用户喜欢简洁的回答"
memory search "用户偏好"
memory history --recent 10
```

## 子代理系统

子代理可以在后台异步执行复杂任务：

```
# 创建子任务
task_spawn --task "分析代码库结构" --context "项目目录: /home/user/project"

# 查询状态
task_status --id "task_xxx"
```

子代理特点：
- 独立的工具白名单（无 message、task_spawn）
- 最多 15 次迭代
- 完成后通知主代理

## 与 nanobot 对比

### 基本定位

| 方面 | LingGuard (本项目) | nanobot |
|------|-------------------|---------|
| **语言** | Go | Python |
| **代码量** | ~8,000+ 行 | ~3,700 行核心代码 |
| **核心理念** | 极简、高性能、单二进制部署 | 超轻量级、易研究、易扩展 |
| **内存占用** | ~20MB | ~100MB+ |
| **启动速度** | 毫秒级 | 秒级 |
| **部署方式** | 单二进制文件 | pip/uv/Docker |

### 渠道支持对比

| 渠道 | LingGuard | nanobot |
|------|:---------:|:-------:|
| 飞书 | ✅ WebSocket 长连接 | ✅ |
| QQ | ✅ 私聊 | ✅ 私聊 |
| Telegram | ❌ | ✅ 推荐 |
| Discord | ❌ | ✅ |
| WhatsApp | ❌ | ✅ |
| Slack | ❌ | ✅ |
| Email | ❌ | ✅ |
| 钉钉 | ❌ | ✅ |
| Mochat | ❌ | ✅ 自动配置 |

### LLM 提供商支持

| Provider | LingGuard | nanobot |
|----------|:---------:|:-------:|
| OpenAI / Anthropic / DeepSeek | ✅ | ✅ |
| OpenRouter (推荐) | ✅ | ✅ |
| Qwen / GLM / MiniMax / Moonshot | ✅ | ✅ |
| Gemini / Groq / vLLM | ✅ | ✅ |
| AiHubMix / SiliconFlow | ✅ (部分) | ✅ |
| OpenAI Codex (OAuth) | ❌ | ✅ |
| GitHub Copilot (OAuth) | ❌ | ✅ |

### 功能对比

| 功能 | LingGuard | nanobot |
|------|:---------:|:-------:|
| **核心功能** |||
| Agent Loop | ✅ | ✅ |
| 会话管理 | ✅ | ✅ |
| 记忆系统 | ✅ | ✅ |
| 工具系统 | ✅ | ✅ |
| 技能系统 | ✅ | ✅ |
| **高级功能** |||
| 定时任务 (Cron) | ✅ | ✅ |
| 时区支持 | ✅ | ✅ |
| 子代理 (Subagent) | ✅ | ✅ |
| 流式响应 | ✅ | ✅ |
| MCP (Stdio + HTTP) | ✅ | ✅ |
| Agent Social Network | ✅ Moltbook | ✅ Moltbook + ClawdChat |
| **独有功能** |||
| 渐进式技能加载 | ✅ 独有 | ❌ |
| 多模态支持 | ✅ 图片+视频 | 🚧 计划中 |
| 独立多模态 Provider | ✅ 独有 | ❌ |
| ClawHub 技能库 | ❌ | ✅ |
| OAuth 登录 | ❌ | ✅ Codex/Copilot |
| 语音转录 | ❌ | ✅ Groq Whisper |
| Docker 支持 | ❌ | ✅ |

### 适用场景

| 场景 | 推荐选择 | 理由 |
|------|----------|------|
| **个人桌面使用** | LingGuard | 低资源占用、快速启动 |
| **需要多种聊天平台** | nanobot | 9 种渠道支持 |
| **服务器长期运行** | 两者皆可 | 都支持 Gateway 模式 |
| **研究和二次开发** | nanobot | 代码更精简、Python 易上手 |
| **生产环境部署** | LingGuard | 单二进制、无依赖 |
| **需要 OAuth 登录** | nanobot | 支持 Codex/Copilot |
| **需要语音交互** | nanobot | Groq 语音转录 |
| **节省 Token 成本** | LingGuard | 渐进式技能加载 |

## 目录结构

```
lingguard/
├── cmd/
│   ├── lingguard/       # 主程序入口
│   └── cli/             # CLI 命令
├── internal/
│   ├── agent/           # 核心代理
│   ├── providers/       # LLM 提供商
│   ├── channels/        # 消息渠道
│   ├── tools/           # 内置工具
│   │   ├── mcp.go       # MCP Stdio 客户端
│   │   └── mcp_http.go  # MCP HTTP 客户端
│   ├── skills/          # 技能加载器
│   ├── cron/            # 定时任务
│   ├── subagent/        # 子代理
│   ├── session/         # 会话管理
│   └── config/          # 配置管理
├── pkg/
│   ├── llm/             # LLM 类型
│   ├── stream/          # 流式响应
│   ├── memory/          # 记忆系统
│   └── logger/          # 日志
├── skills/builtin/      # 内置技能
├── configs/             # 配置文件
└── docs/                # 文档
```

## 构建方法

```bash
# 标准构建
go build -o lingguard ./cmd/lingguard

# 优化体积
go build -ldflags="-s -w" -o lingguard ./cmd/lingguard

# 交叉编译
GOOS=linux GOARCH=amd64 go build -o lingguard-linux ./cmd/lingguard
GOOS=darwin GOARCH=amd64 go build -o lingguard-darwin ./cmd/lingguard
GOOS=windows GOARCH=amd64 go build -o lingguard.exe ./cmd/lingguard
```

## 依赖

- Go 1.23+
- [Cobra](https://github.com/spf13/cobra) - CLI 框架
- [robfig/cron](https://github.com/robfig/cron) - Cron 调度
- [larksuite/oapi-sdk-go](https://github.com/larksuite/oapi-sdk-go) - 飞书 SDK

## 文档

- [架构文档](docs/ARCHITECTURE.md) - 系统架构和与 nanobot 的对比
- [API 文档](docs/API.md) - API 接口和使用说明

## License

MIT
