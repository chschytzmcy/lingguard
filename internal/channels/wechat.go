package channels

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/stream"
)

// WeChatChannel 微信渠道（通过 QClaw 接入）
type WeChatChannel struct {
	cfg              *config.WeChatConfig
	handler          MessageHandler
	streamingHandler StreamingMessageHandler
	allowMap         map[string]bool

	// QClaw 客户端
	qclawClient *QClawClient
	agpClient   *AGPClient

	// 运行状态
	running bool
	mu      sync.RWMutex

	// 会话管理
	activeSessions sync.Map // sessionID -> *wechatSession
}

// wechatSession 微信会话信息
type wechatSession struct {
	sessionID string
	promptID  string
	userID    string
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewWeChatChannel 创建微信渠道
func NewWeChatChannel(cfg *config.WeChatConfig, handler MessageHandler) *WeChatChannel {
	// 构建允许列表
	allowMap := make(map[string]bool)
	for _, id := range cfg.AllowFrom {
		allowMap[id] = true
	}

	// 创建 QClaw 客户端
	env := cfg.Environment
	if env == "" {
		env = "production"
	}
	qclawClient := NewQClawClient(env, cfg.GUID, cfg.WebVersion)

	// 恢复 JWT Token
	if cfg.JWTToken != "" {
		qclawClient.SetJWTToken(cfg.JWTToken)
	}

	// 创建 AGP 客户端
	envURLs := qclawEnvs[env]
	agpClient := NewAGPClient(envURLs.WeChatWsURL, cfg.ChannelToken, cfg.GUID, "")

	ch := &WeChatChannel{
		cfg:         cfg,
		handler:     handler,
		allowMap:    allowMap,
		qclawClient: qclawClient,
		agpClient:   agpClient,
	}

	// 检查是否支持流式处理
	if sh, ok := handler.(StreamingMessageHandler); ok {
		ch.streamingHandler = sh
	}

	// 设置 AGP 回调
	agpClient.SetCallbacks(
		ch.onConnected,
		ch.onDisconnected,
		ch.onPrompt,
		ch.onCancel,
		ch.onError,
	)

	return ch
}

// Name 返回渠道名称
func (c *WeChatChannel) Name() string {
	return "wechat"
}

// Start 启动渠道
func (c *WeChatChannel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("wechat channel already running")
	}
	c.running = true
	c.mu.Unlock()

	logger.Info("Starting WeChat channel (QClaw)")

	// 检查是否已登录
	if c.cfg.ChannelToken == "" {
		logger.Warn("WeChat channel token not configured, please login first")
		logger.Info("Use the following steps to login:")
		logger.Info("1. Get login state: curl -X POST http://localhost:8080/v1/wechat/login/state")
		logger.Info("2. Scan QR code with WeChat")
		logger.Info("3. Complete login: curl -X POST http://localhost:8080/v1/wechat/login -d '{\"code\":\"...\",\"state\":\"...\"}'")
		return fmt.Errorf("channel token not configured")
	}

	// 启动 AGP 客户端
	if err := c.agpClient.Start(); err != nil {
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
		return fmt.Errorf("start AGP client failed: %w", err)
	}

	logger.Info("WeChat channel started")
	return nil
}

// Stop 停止渠道
func (c *WeChatChannel) Stop() error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = false
	c.mu.Unlock()

	logger.Info("Stopping WeChat channel")

	// 取消所有活跃会话
	c.activeSessions.Range(func(key, value interface{}) bool {
		if session, ok := value.(*wechatSession); ok {
			session.cancel()
		}
		c.activeSessions.Delete(key)
		return true
	})

	// 停止 AGP 客户端
	c.agpClient.Stop()

	logger.Info("WeChat channel stopped")
	return nil
}

// IsRunning 返回运行状态
func (c *WeChatChannel) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// Send 发送消息（微信渠道不支持主动发送）
func (c *WeChatChannel) Send(ctx context.Context, to string, content string) error {
	return fmt.Errorf("wechat channel does not support active sending")
}

// onConnected AGP 连接成功回调
func (c *WeChatChannel) onConnected() {
	logger.Info("WeChat AGP connected")
}

// onDisconnected AGP 断开连接回调
func (c *WeChatChannel) onDisconnected(reason string) {
	logger.Warn("WeChat AGP disconnected: %s", reason)
}

// onPrompt 收到用户消息回调
func (c *WeChatChannel) onPrompt(payload *agpPromptPayload) {
	logger.Info("WeChat received prompt: session=%s, prompt=%s", payload.SessionID, payload.PromptID)

	// 提取消息内容
	var textParts []string
	for _, block := range payload.Content {
		if block.Type == "text" && block.Text != "" {
			textParts = append(textParts, block.Text)
		}
	}
	content := strings.Join(textParts, "\n")

	if content == "" {
		logger.Warn("WeChat empty message content")
		return
	}

	// 构建消息
	msg := &Message{
		ID:        payload.PromptID,
		SessionID: fmt.Sprintf("wechat-%s", payload.SessionID),
		Content:   content,
		Media:     nil, // 微信暂不支持媒体文件
		Metadata: map[string]any{
			"agent_app": payload.AgentApp,
			"prompt_id": payload.PromptID,
		},
		Channel: "wechat",
		UserID:  payload.SessionID, // 使用 sessionID 作为 userID
	}

	// 检查允许列表
	if len(c.allowMap) > 0 && !c.allowMap[msg.UserID] {
		logger.Warn("WeChat message from unauthorized user: %s", msg.UserID)
		c.agpClient.SendTextResponse(payload.SessionID, payload.PromptID, "抱歉，您没有权限使用此服务")
		return
	}

	// 创建会话上下文
	ctx, cancel := context.WithCancel(context.Background())
	session := &wechatSession{
		sessionID: payload.SessionID,
		promptID:  payload.PromptID,
		userID:    msg.UserID,
		ctx:       ctx,
		cancel:    cancel,
	}
	c.activeSessions.Store(payload.SessionID, session)

	// 处理消息
	go c.handleMessage(session, msg)
}

// onCancel 收到取消消息回调
func (c *WeChatChannel) onCancel(payload *agpCancelPayload) {
	logger.Info("WeChat received cancel: session=%s, prompt=%s", payload.SessionID, payload.PromptID)

	// 取消会话
	if value, ok := c.activeSessions.Load(payload.SessionID); ok {
		if session, ok := value.(*wechatSession); ok {
			session.cancel()
		}
		c.activeSessions.Delete(payload.SessionID)
	}

	// 发送取消响应
	c.agpClient.SendCancelledResponse(payload.SessionID, payload.PromptID)
}

// onError AGP 错误回调
func (c *WeChatChannel) onError(err error) {
	logger.Error("WeChat AGP error: %v", err)
}

// handleMessage 处理消息
func (c *WeChatChannel) handleMessage(session *wechatSession, msg *Message) {
	defer func() {
		c.activeSessions.Delete(session.sessionID)
	}()

	// 使用流式处理
	if c.streamingHandler != nil {
		c.handleMessageStreaming(session, msg)
		return
	}

	// 非流式处理
	response, err := c.handler.HandleMessage(session.ctx, msg)
	if err != nil {
		logger.Error("WeChat handle message failed: %v", err)
		c.agpClient.SendErrorResponse(session.sessionID, session.promptID, err.Error())
		return
	}

	// 发送响应
	if err := c.agpClient.SendTextResponse(session.sessionID, session.promptID, response); err != nil {
		logger.Error("WeChat send response failed: %v", err)
	}
}

// handleMessageStreaming 流式处理消息
func (c *WeChatChannel) handleMessageStreaming(session *wechatSession, msg *Message) {
	var fullResponse strings.Builder
	var lastError error

	callback := func(event stream.StreamEvent) {
		select {
		case <-session.ctx.Done():
			return
		default:
		}

		switch event.Type {
		case stream.EventText:
			// 发送文本块
			if err := c.agpClient.SendMessageChunk(session.sessionID, session.promptID, event.Content); err != nil {
				logger.Error("WeChat send chunk failed: %v", err)
			}
			fullResponse.WriteString(event.Content)

		case stream.EventError:
			lastError = event.Error
			logger.Error("WeChat stream error: %v", lastError)

		case stream.EventDone:
			// 流式处理完成
		}
	}

	// 执行流式处理
	if err := c.streamingHandler.HandleMessageStream(session.ctx, msg, callback); err != nil {
		logger.Error("WeChat handle message stream failed: %v", err)
		c.agpClient.SendErrorResponse(session.sessionID, session.promptID, err.Error())
		return
	}

	// 发送最终响应
	if lastError != nil {
		c.agpClient.SendErrorResponse(session.sessionID, session.promptID, lastError.Error())
	} else {
		response := fullResponse.String()
		if response == "" {
			response = "处理完成"
		}
		c.agpClient.SendTextResponse(session.sessionID, session.promptID, response)
	}
}

// GetLoginURL 获取微信登录 URL
func (c *WeChatChannel) GetLoginURL() (string, error) {
	state, err := c.qclawClient.GetWxLoginState()
	if err != nil {
		return "", fmt.Errorf("get wx login state failed: %w", err)
	}

	url := c.qclawClient.BuildWxLoginURL(state)
	return url, nil
}

// Login 微信登录
func (c *WeChatChannel) Login(code, state string) error {
	// 执行登录
	loginData, err := c.qclawClient.WxLogin(code, state)
	if err != nil {
		return fmt.Errorf("wx login failed: %w", err)
	}

	// 更新配置
	c.cfg.JWTToken = loginData.Token
	c.cfg.ChannelToken = loginData.OpenClawChannelToken

	// 更新 AGP 客户端 token
	c.agpClient.SetToken(loginData.OpenClawChannelToken)

	logger.Info("WeChat login successful, user: %s", loginData.UserInfo.Nickname)

	// TODO: 持久化配置到文件

	return nil
}

// RefreshToken 刷新 Channel Token
func (c *WeChatChannel) RefreshToken() error {
	token, err := c.qclawClient.RefreshChannelToken()
	if err != nil {
		return fmt.Errorf("refresh channel token failed: %w", err)
	}

	// 更新配置
	c.cfg.ChannelToken = token

	// 更新 AGP 客户端 token
	c.agpClient.SetToken(token)

	logger.Info("WeChat channel token refreshed")

	// TODO: 持久化配置到文件

	return nil
}
