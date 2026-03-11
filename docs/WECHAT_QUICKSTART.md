# 微信渠道快速开始

## 前置条件

1. 已安装 LingGuard
2. 有可用的 LLM Provider (如 DeepSeek, OpenAI 等)
3. 准备一个设备 GUID (可以使用 UUID)

## 步骤 1: 生成 GUID

```bash
# macOS/Linux
uuidgen

# 或使用 Python
python3 -c "import uuid; print(uuid.uuid4())"

# 输出示例: 550e8400-e29b-41d4-a716-446655440000
```

## 步骤 2: 配置文件

创建或编辑 `~/.lingguard/config.json`:

```json
{
  "providers": {
    "deepseek": {
      "apiKey": "sk-your-api-key",
      "model": "deepseek-chat"
    }
  },
  "agents": {
    "workspace": "/tmp/lingguard-workspace",
    "provider": "deepseek",
    "maxToolIterations": 20,
    "memoryWindow": 50,
    "systemPrompt": "你是一个智能助手"
  },
  "channels": {
    "wechat": {
      "enabled": true,
      "environment": "production",
      "guid": "550e8400-e29b-41d4-a716-446655440000"
    }
  },
  "server": {
    "enabled": true,
    "host": "127.0.0.1",
    "port": 8080
  }
}
```

## 步骤 3: 启动服务

```bash
./lingguard gateway
```

服务启动后会提示需要登录:
```
WARN WeChat channel token not configured, please login first
INFO Use the following steps to login:
INFO 1. Get login state: curl -X POST http://localhost:8080/v1/wechat/login/state
INFO 2. Scan QR code with WeChat
INFO 3. Complete login: curl -X POST http://localhost:8080/v1/wechat/login -d '{"code":"...","state":"..."}'
```

## 步骤 4: 微信登录

### 4.1 获取登录 URL

```bash
curl -X POST http://localhost:8080/v1/wechat/login/state
```

响应:
```json
{
  "state": "abc123...",
  "qr_url": "https://open.weixin.qq.com/connect/qrconnect?appid=wx9d11056dd75b7240&redirect_uri=https%3A%2F%2Fsecurity.guanjia.qq.com%2Flogin&response_type=code&scope=snsapi_login&state=abc123..."
}
```

### 4.2 扫码登录

1. 复制 `qr_url` 到浏览器打开
2. 使用微信扫描二维码
3. 确认登录

### 4.3 获取授权码

微信扫码后会跳转到回调地址，URL 格式如下:
```
https://security.guanjia.qq.com/login?code=xxx&state=abc123...
```

从 URL 中提取 `code` 参数。

### 4.4 完成登录

```bash
curl -X POST http://localhost:8080/v1/wechat/login \
  -H "Content-Type: application/json" \
  -d '{
    "code": "从上一步获取的 code",
    "state": "从步骤 4.1 获取的 state"
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

登录成功后，Token 会保存在内存中。

> ⚠️ **注意**: 当前版本 Token 不会自动持久化到配置文件。重启服务后需要重新登录。后续版本会支持自动持久化。
>
> **临时解决方案**: 登录成功后，可以手动将 Token 添加到配置文件中:
> ```json
> {
>   "channels": {
>     "wechat": {
>       "enabled": true,
>       "guid": "your-guid",
>       "jwtToken": "从登录响应获取",
>       "channelToken": "从登录响应获取"
>     }
>   }
> }
> ```

## 步骤 5: 重启服务（如已配置 Token）

如果已经在配置文件中手动设置了 `jwtToken` 和 `channelToken`:

```bash
# 停止服务 (Ctrl+C)
# 重新启动
./lingguard gateway
```

服务应该正常运行:
```
INFO Starting WeChat channel (QClaw)
INFO WeChat channel started
INFO WeChat AGP connected
```

> ⚠️ 如果没有在配置文件中设置 Token，重启后服务会进入待登录状态，需要重新执行步骤 4。

## 步骤 6: 测试

在微信中向你的 QClaw 机器人发送消息，LingGuard 会自动接收并响应。

## 故障排查

### 问题 0: 服务启动但提示 token not configured

**现象**: 服务启动后显示 `WeChat channel token not configured, waiting for login...`

**说明**: 这是正常行为。服务会以待登录状态运行，允许你通过 HTTP API 完成登录。

**解决**: 按照步骤 4 完成微信登录。

### 问题 1: 登录失败

**错误**: `api error: ret=xxx`

**解决**:
- 确认 GUID 配置正确
- 检查 code 和 state 参数是否匹配
- 确认微信扫码成功

### 问题 2: 连接失败

**错误**: `AGP connect failed`

**解决**:
- 检查 channelToken 是否有效
- 尝试刷新 Token:
  ```bash
  curl -X POST http://localhost:8080/v1/wechat/token/refresh
  ```

### 问题 3: 无响应

**错误**: 收到消息但无响应

**解决**:
- 检查 Agent 配置是否正确
- 查看日志中的错误信息
- 确认 LLM Provider 配置有效

## 高级配置

### 用户白名单

限制只有特定用户可以使用:

```json
{
  "channels": {
    "wechat": {
      "enabled": true,
      "guid": "...",
      "allowFrom": ["user_id_1", "user_id_2"]
    }
  }
}
```

### 测试环境

使用测试环境进行开发:

```json
{
  "channels": {
    "wechat": {
      "enabled": true,
      "environment": "test",
      "guid": "..."
    }
  }
}
```

## 下一步

- 阅读 [微信渠道文档](WECHAT_CHANNEL.md) 了解更多细节
- 配置 [技能系统](../skills/) 增强功能
- 启用 [记忆系统](../README.md#记忆系统) 实现上下文记忆

## 获取帮助

- GitHub Issues: https://github.com/your-org/lingguard/issues
- 文档: [WECHAT_CHANNEL.md](WECHAT_CHANNEL.md)
