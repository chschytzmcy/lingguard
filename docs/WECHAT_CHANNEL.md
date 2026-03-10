# 微信渠道集成文档

## 概述

LingGuard 通过 QClaw (管家 OpenClaw) 服务实现微信接入。QClaw 是腾讯提供的 AI 网关服务，支持通过微信 OAuth2 扫码登录，并通过 WebSocket 实时接收和响应微信用户消息。

## 架构设计

### 组件结构

```
┌─────────────────────────────────────────────────────────────┐
│                        LingGuard                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ WeChatChannel│  │ QClawClient  │  │  AGPClient   │      │
│  │              │──│  (HTTP API)  │  │ (WebSocket)  │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                           │                    │
                           ▼                    ▼
┌─────────────────────────────────────────────────────────────┐
│                    QClaw 网关服务                            │
│  ┌──────────────┐                  ┌──────────────┐        │
│  │  jprx HTTP   │                  │  AGP WebSocket│        │
│  │   Gateway    │                  │   (agentwss)  │        │
│  └──────────────┘                  └──────────────┘        │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │   微信用户    │
                    └──────────────┘
```

### 核心模块

1. **WeChatChannel** (`wechat.go`)
   - 实现 Channel 接口
   - 管理 QClaw 和 AGP 客户端
   - 处理消息接收和响应

2. **QClawClient** (`wechat_qclaw_client.go`)
   - HTTP API 客户端
   - 微信 OAuth 登录
   - Token 管理和刷新

3. **AGPClient** (`wechat_agp_client.go`)
   - WebSocket 客户端
   - 实现 AGP (Agent Gateway Protocol)
   - 自动重连、心跳、消息去重

## 配置说明

### 配置文件示例

```json
{
  "channels": {
    "wechat": {
      "enabled": true,
      "environment": "production",
      "guid": "your-device-guid",
      "jwtToken": "",
      "channelToken": "",
      "allowFrom": [],
      "webVersion": "1.4.0"
    }
  }
}
```

### 配置字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `enabled` | bool | 是 | 是否启用微信渠道 |
| `environment` | string | 否 | 环境: "production" 或 "test"，默认 "production" |
| `guid` | string | 是 | 设备唯一标识，建议使用 UUID |
| `jwtToken` | string | 否 | JWT Token（登录后自动更新） |
| `channelToken` | string | 否 | Channel Token（登录后自动更新） |
| `allowFrom` | []string | 否 | 允许的用户 ID 白名单 |
| `webVersion` | string | 否 | QClaw Web 版本号，默认 "1.4.0" |

### 环境配置

**生产环境 (production)**:
- jprx Gateway: `https://jprx.m.qq.com/`
- WebSocket: `wss://mmgrcalltoken.3g.qq.com/agentwss`
- 微信 OAuth AppID: `wx9d11056dd75b7240`

**测试环境 (test)**:
- jprx Gateway: `https://jprx.sparta.html5.qq.com/`
- WebSocket: `wss://jprx.sparta.html5.qq.com/agentwss`
- 微信 OAuth AppID: `wx3dd49afb7e2cf957`

## 使用流程

### 1. 初始化配置

创建配置文件 `~/.lingguard/config.json`:

```json
{
  "channels": {
    "wechat": {
      "enabled": true,
      "guid": "550e8400-e29b-41d4-a716-446655440000"
    }
  }
}
```

### 2. 微信登录

由于首次使用需要微信扫码登录，建议通过 HTTP API 完成登录流程：

**步骤 1: 获取登录 state**
```bash
curl -X POST http://localhost:8080/v1/wechat/login/state
```

响应:
```json
{
  "state": "xxx",
  "qr_url": "https://open.weixin.qq.com/connect/qrconnect?..."
}
```

**步骤 2: 用微信扫描二维码**

在浏览器中打开 `qr_url`，使用微信扫码登录。

**步骤 3: 完成登录**

微信扫码后会跳转到回调地址，从 URL 中提取 `code` 参数，然后调用登录接口:

```bash
curl -X POST http://localhost:8080/v1/wechat/login \
  -H "Content-Type: application/json" \
  -d '{
    "code": "xxx",
    "state": "xxx"
  }'
```

响应:
```json
{
  "success": true,
  "message": "login successful",
  "user": {
    "nickname": "张三",
    "avatar": "https://...",
    "user_id": "xxx"
  }
}
```

登录成功后，`jwtToken` 和 `channelToken` 会自动保存到配置文件。

### 3. 启动服务

```bash
./lingguard gateway
```

服务启动后，微信用户发送消息给你的 QClaw 机器人，LingGuard 会自动接收并处理。

### 4. Token 刷新

Channel Token 有效期较长，但如果过期，可以手动刷新:

```bash
curl -X POST http://localhost:8080/v1/wechat/token/refresh
```

## 协议实现

### HTTP 协议 (jprx Gateway)

**请求格式**:
```
POST {jprxGateway}{endpoint}

Headers:
- Content-Type: application/json
- X-Version: 1
- X-Token: <loginKey>
- X-Guid: <device GUID>
- X-Account: <userId>
- X-OpenClaw-Token: <JWT>

Body:
{
  "...endpoint-specific params",
  "web_version": "1.4.0",
  "web_env": "release"
}
```

**响应格式**:
```json
{
  "ret": 0,
  "message": "success",
  "data": {
    "resp": {
      "data": { ... }
    }
  },
  "common": {
    "code": 0,
    "message": "success"
  }
}
```

### WebSocket 协议 (AGP)

**消息信封**:
```json
{
  "msg_id": "uuid-v4",
  "guid": "device-id",
  "user_id": "user-id",
  "method": "session.prompt",
  "payload": { ... }
}
```

**下行消息** (服务器 → 客户端):
- `session.prompt` - 用户发送消息
- `session.cancel` - 取消当前回合

**上行消息** (客户端 → 服务器):
- `session.update` - 流式响应块
- `session.promptResponse` - 最终响应

## 特性

### 1. 自动重连

AGP 客户端实现了健壮的自动重连机制:
- 指数退避: 3s 基础延迟, 1.5x 倍数, 最大 25s
- 无限重试 (可配置最大重试次数)
- 系统唤醒检测 (防止笔记本休眠导致的连接问题)

### 2. 心跳检测

- 每 20 秒发送 WebSocket ping
- pong 超时检测 (2x 心跳间隔)
- 超时自动重连

### 3. 消息去重

- 基于 msg_id 的去重机制
- 定期清理 (每 5 分钟)
- 最大缓存 1000 条消息 ID

### 4. 流式响应

支持流式返回 AI 响应:
```go
// 发送消息块
agpClient.SendMessageChunk(sessionID, promptID, "Hello ")
agpClient.SendMessageChunk(sessionID, promptID, "World!")

// 发送最终响应
agpClient.SendTextResponse(sessionID, promptID, "Hello World!")
```

### 5. Token 自动续期

- JWT Token 自动续期 (通过 X-New-Token 响应头)
- Channel Token 手动刷新接口
- 会话过期自动清理

## 限制

1. **不支持主动发送**: 微信渠道是被动接收模式，不支持主动向用户发送消息
2. **不支持媒体文件**: 当前版本暂不支持图片、视频等媒体文件
3. **依赖 QClaw 服务**: 需要腾讯 QClaw 服务支持，协议可能随服务更新而变化

## 故障排查

### 1. 连接失败

**问题**: AGP 连接失败
**解决**:
- 检查 `channelToken` 是否有效
- 检查网络连接
- 查看日志中的错误信息

### 2. 登录失败

**问题**: 微信登录失败
**解决**:
- 确认 `guid` 配置正确
- 检查 `code` 和 `state` 参数
- 确认微信扫码成功

### 3. Token 过期

**问题**: 会话过期 (code: 21004)
**解决**:
- 重新执行微信登录流程
- 或调用 Token 刷新接口

### 4. 消息无响应

**问题**: 收到消息但无响应
**解决**:
- 检查 `allowFrom` 白名单配置
- 查看 Agent 处理日志
- 确认 AGP 连接状态

## 开发参考

### 添加 HTTP API 端点

在 `cmd/cli/gateway.go` 中添加微信相关的 HTTP API:

```go
// 获取登录 state
router.POST("/v1/wechat/login/state", func(c *gin.Context) {
    // 实现逻辑
})

// 微信登录
router.POST("/v1/wechat/login", func(c *gin.Context) {
    // 实现逻辑
})

// 刷新 Token
router.POST("/v1/wechat/token/refresh", func(c *gin.Context) {
    // 实现逻辑
})
```

### 扩展功能

1. **支持媒体文件**: 在 `agpContentBlock` 中添加图片、视频类型支持
2. **工具调用通知**: 实现 `SendToolCall` 和 `SendToolCallUpdate`
3. **配置持久化**: 自动保存 Token 到配置文件
4. **多设备支持**: 支持多个 GUID 配置

## 参考资料

- [qclaw-wechat-client](https://github.com/photon-hq/qclaw-wechat-client) - TypeScript 参考实现
- QClaw 官方文档 (如有)
- AGP 协议规范 (逆向工程)
