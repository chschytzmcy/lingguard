package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/lingguard/pkg/logger"
)

// AGP 协议消息方法
const (
	agpMethodPrompt         = "session.prompt"
	agpMethodCancel         = "session.cancel"
	agpMethodUpdate         = "session.update"
	agpMethodPromptResponse = "session.promptResponse"
)

// AGP 更新类型
const (
	agpUpdateTypeMessageChunk   = "message_chunk"
	agpUpdateTypeToolCall       = "tool_call"
	agpUpdateTypeToolCallUpdate = "tool_call_update"
)

// AGP Tool Call 类型
const (
	agpToolCallKindFunction = "function"
)

// AGP Tool Call 状态
const (
	agpToolCallStatusPending  = "pending"
	agpToolCallStatusRunning  = "running"
	agpToolCallStatusComplete = "complete"
	agpToolCallStatusError    = "error"
)

// AGP 停止原因
const (
	agpStopReasonEndTurn   = "end_turn"
	agpStopReasonCancelled = "cancelled"
	agpStopReasonError     = "error"
	agpStopReasonRefusal   = "refusal"
)

// AGP 消息信封
type agpEnvelope struct {
	MsgID   string          `json:"msg_id"`
	GUID    string          `json:"guid"`
	UserID  string          `json:"user_id"`
	Method  string          `json:"method"`
	Payload json.RawMessage `json:"payload"`
}

// AGP Prompt 消息
type agpPromptPayload struct {
	SessionID string            `json:"session_id"`
	PromptID  string            `json:"prompt_id"`
	AgentApp  string            `json:"agent_app"`
	Content   []agpContentBlock `json:"content"`
}

type agpContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// AGP Cancel 消息
type agpCancelPayload struct {
	SessionID string `json:"session_id"`
	PromptID  string `json:"prompt_id"`
}

// AGP Update 消息
type agpUpdatePayload struct {
	SessionID  string          `json:"session_id"`
	PromptID   string          `json:"prompt_id"`
	UpdateType string          `json:"update_type"`
	Data       json.RawMessage `json:"data"`
}

type agpMessageChunkData struct {
	Text string `json:"text"`
}

// AGP Tool Call 数据
type agpToolCallData struct {
	ToolCallID string           `json:"tool_call_id"`
	Name       string           `json:"name"`
	Kind       string           `json:"kind"`
	Status     string           `json:"status"`
	Input      interface{}      `json:"input,omitempty"`
	Output     interface{}      `json:"output,omitempty"`
	Error      string           `json:"error,omitempty"`
	Location   *agpToolLocation `json:"location,omitempty"`
}

// AGP Tool Location
type agpToolLocation struct {
	Path string `json:"path,omitempty"`
	Line int    `json:"line,omitempty"`
}

// AGP PromptResponse 消息
type agpPromptResponsePayload struct {
	SessionID  string            `json:"session_id"`
	PromptID   string            `json:"prompt_id"`
	StopReason string            `json:"stop_reason"`
	Content    []agpContentBlock `json:"content,omitempty"`
	Error      *agpErrorData     `json:"error,omitempty"`
}

type agpErrorData struct {
	Message string `json:"message"`
}

// AGPClient AGP WebSocket 客户端
type AGPClient struct {
	url    string
	token  string
	guid   string
	userID string

	// 连接配置
	reconnectInterval    time.Duration
	maxReconnectAttempts int
	heartbeatInterval    time.Duration

	// 回调函数
	onConnected    func()
	onDisconnected func(reason string)
	onPrompt       func(*agpPromptPayload)
	onCancel       func(*agpCancelPayload)
	onError        func(error)

	// 连接状态
	conn  *websocket.Conn
	state string // "disconnected", "connecting", "connected", "reconnecting"
	mu    sync.RWMutex

	// 控制
	ctx       context.Context
	cancel    context.CancelFunc
	stopCh    chan struct{}
	stoppedCh chan struct{}

	// 重连
	reconnectAttempts int

	// 心跳
	lastPongTime time.Time
	pongMu       sync.RWMutex

	// 唤醒检测
	lastTickTime time.Time
	tickMu       sync.RWMutex

	// 消息去重
	processedMsgIDs sync.Map
	cleanupTicker   *time.Ticker
}

// NewAGPClient 创建 AGP 客户端
func NewAGPClient(url, token, guid, userID string) *AGPClient {
	ctx, cancel := context.WithCancel(context.Background())

	return &AGPClient{
		url:                  url,
		token:                token,
		guid:                 guid,
		userID:               userID,
		reconnectInterval:    3 * time.Second,
		maxReconnectAttempts: 0, // 0 表示无限重试
		heartbeatInterval:    20 * time.Second,
		state:                "disconnected",
		ctx:                  ctx,
		cancel:               cancel,
		stopCh:               make(chan struct{}),
		stoppedCh:            make(chan struct{}),
		lastPongTime:         time.Now(),
		lastTickTime:         time.Now(),
	}
}

// SetCallbacks 设置回调函数
func (c *AGPClient) SetCallbacks(
	onConnected func(),
	onDisconnected func(string),
	onPrompt func(*agpPromptPayload),
	onCancel func(*agpCancelPayload),
	onError func(error),
) {
	c.onConnected = onConnected
	c.onDisconnected = onDisconnected
	c.onPrompt = onPrompt
	c.onCancel = onCancel
	c.onError = onError
}

// Start 启动客户端
func (c *AGPClient) Start() error {
	c.mu.Lock()
	if c.state != "disconnected" {
		c.mu.Unlock()
		return fmt.Errorf("client already started")
	}
	c.state = "connecting"
	c.mu.Unlock()

	// 启动消息去重清理
	c.startMsgIDCleanup()

	// 连接
	go c.connect()

	return nil
}

// Stop 停止客户端
func (c *AGPClient) Stop() {
	c.mu.Lock()
	if c.state == "disconnected" {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	close(c.stopCh)
	c.cancel()

	// 等待停止完成
	<-c.stoppedCh

	c.mu.Lock()
	c.state = "disconnected"
	c.mu.Unlock()
}

// GetState 获取连接状态
func (c *AGPClient) GetState() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// SetToken 更新 token
func (c *AGPClient) SetToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
}

// connect 建立 WebSocket 连接
func (c *AGPClient) connect() {
	defer func() {
		select {
		case <-c.stopCh:
			close(c.stoppedCh)
		default:
		}
	}()

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		// 构建 WebSocket URL
		c.mu.RLock()
		wsURL := fmt.Sprintf("%s?token=%s", c.url, c.token)
		c.mu.RUnlock()

		// 连接
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			logger.Error("AGP connect failed: %v", err)
			if c.onError != nil {
				c.onError(fmt.Errorf("connect failed: %w", err))
			}

			// 重连
			if !c.shouldReconnect() {
				logger.Error("AGP max reconnect attempts reached")
				return
			}
			c.scheduleReconnect()
			continue
		}

		// 连接成功
		c.mu.Lock()
		c.conn = conn
		c.state = "connected"
		c.reconnectAttempts = 0
		c.mu.Unlock()

		logger.Info("AGP connected")
		if c.onConnected != nil {
			c.onConnected()
		}

		// 启动心跳和唤醒检测
		c.startHeartbeat()
		c.startWakeupDetection()

		// 处理消息
		c.handleMessages()

		// 连接断开
		c.mu.Lock()
		c.conn = nil
		if c.state == "connected" {
			c.state = "reconnecting"
		}
		c.mu.Unlock()

		logger.Warn("AGP disconnected")
		if c.onDisconnected != nil {
			c.onDisconnected("connection lost")
		}

		// 检查是否需要重连
		select {
		case <-c.stopCh:
			return
		default:
			if !c.shouldReconnect() {
				return
			}
			c.scheduleReconnect()
		}
	}
}

// handleMessages 处理接收到的消息
func (c *AGPClient) handleMessages() {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return
	}

	// 设置 pong 处理器
	conn.SetPongHandler(func(string) error {
		c.pongMu.Lock()
		c.lastPongTime = time.Now()
		c.pongMu.Unlock()
		return nil
	})

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("AGP read error: %v", err)
			}
			return
		}

		// 解析消息
		var envelope agpEnvelope
		if err := json.Unmarshal(message, &envelope); err != nil {
			logger.Error("AGP unmarshal message failed: %v", err)
			continue
		}

		// 消息去重
		if c.isDuplicate(envelope.MsgID) {
			continue
		}

		// 处理消息
		c.handleMessage(&envelope)
	}
}

// handleMessage 处理单条消息
func (c *AGPClient) handleMessage(envelope *agpEnvelope) {
	switch envelope.Method {
	case agpMethodPrompt:
		var payload agpPromptPayload
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			logger.Error("AGP unmarshal prompt payload failed: %v", err)
			return
		}
		if c.onPrompt != nil {
			c.onPrompt(&payload)
		}

	case agpMethodCancel:
		var payload agpCancelPayload
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			logger.Error("AGP unmarshal cancel payload failed: %v", err)
			return
		}
		if c.onCancel != nil {
			c.onCancel(&payload)
		}

	default:
		logger.Warn("AGP unknown method: %s", envelope.Method)
	}
}

// SendMessageChunk 发送消息块
func (c *AGPClient) SendMessageChunk(sessionID, promptID, text string) error {
	data, _ := json.Marshal(agpMessageChunkData{Text: text})
	return c.sendUpdate(sessionID, promptID, agpUpdateTypeMessageChunk, data)
}

// SendToolCall 发送工具调用开始事件
func (c *AGPClient) SendToolCall(sessionID, promptID, toolCallID, name string, input interface{}) error {
	data := agpToolCallData{
		ToolCallID: toolCallID,
		Name:       name,
		Kind:       agpToolCallKindFunction,
		Status:     agpToolCallStatusPending,
		Input:      input,
	}
	dataBytes, _ := json.Marshal(data)
	return c.sendUpdate(sessionID, promptID, agpUpdateTypeToolCall, dataBytes)
}

// SendToolCallUpdate 发送工具调用更新事件
func (c *AGPClient) SendToolCallUpdate(sessionID, promptID, toolCallID, status string, output interface{}, errMsg string) error {
	data := agpToolCallData{
		ToolCallID: toolCallID,
		Status:     status,
		Output:     output,
	}
	if errMsg != "" {
		data.Error = errMsg
	}
	dataBytes, _ := json.Marshal(data)
	return c.sendUpdate(sessionID, promptID, agpUpdateTypeToolCallUpdate, dataBytes)
}

// SendToolCallRunning 发送工具调用运行中事件
func (c *AGPClient) SendToolCallRunning(sessionID, promptID, toolCallID string) error {
	return c.SendToolCallUpdate(sessionID, promptID, toolCallID, agpToolCallStatusRunning, nil, "")
}

// SendToolCallComplete 发送工具调用完成事件
func (c *AGPClient) SendToolCallComplete(sessionID, promptID, toolCallID string, output interface{}) error {
	return c.SendToolCallUpdate(sessionID, promptID, toolCallID, agpToolCallStatusComplete, output, "")
}

// SendToolCallError 发送工具调用错误事件
func (c *AGPClient) SendToolCallError(sessionID, promptID, toolCallID, errMsg string) error {
	return c.SendToolCallUpdate(sessionID, promptID, toolCallID, agpToolCallStatusError, nil, errMsg)
}

// SendTextResponse 发送文本响应
func (c *AGPClient) SendTextResponse(sessionID, promptID, text string) error {
	payload := agpPromptResponsePayload{
		SessionID:  sessionID,
		PromptID:   promptID,
		StopReason: agpStopReasonEndTurn,
		Content: []agpContentBlock{
			{Type: "text", Text: text},
		},
	}
	return c.sendPromptResponse(&payload)
}

// SendErrorResponse 发送错误响应
func (c *AGPClient) SendErrorResponse(sessionID, promptID, errorMsg string) error {
	payload := agpPromptResponsePayload{
		SessionID:  sessionID,
		PromptID:   promptID,
		StopReason: agpStopReasonError,
		Error:      &agpErrorData{Message: errorMsg},
	}
	return c.sendPromptResponse(&payload)
}

// SendCancelledResponse 发送取消响应
func (c *AGPClient) SendCancelledResponse(sessionID, promptID string) error {
	payload := agpPromptResponsePayload{
		SessionID:  sessionID,
		PromptID:   promptID,
		StopReason: agpStopReasonCancelled,
	}
	return c.sendPromptResponse(&payload)
}

// sendUpdate 发送 update 消息
func (c *AGPClient) sendUpdate(sessionID, promptID, updateType string, data json.RawMessage) error {
	payload := agpUpdatePayload{
		SessionID:  sessionID,
		PromptID:   promptID,
		UpdateType: updateType,
		Data:       data,
	}
	return c.sendMessage(agpMethodUpdate, payload)
}

// sendPromptResponse 发送 promptResponse 消息
func (c *AGPClient) sendPromptResponse(payload *agpPromptResponsePayload) error {
	return c.sendMessage(agpMethodPromptResponse, payload)
}

// sendMessage 发送消息
func (c *AGPClient) sendMessage(method string, payload interface{}) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload failed: %w", err)
	}

	envelope := agpEnvelope{
		MsgID:   uuid.New().String(),
		GUID:    c.guid,
		UserID:  c.userID,
		Method:  method,
		Payload: payloadBytes,
	}

	message, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal envelope failed: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
		return fmt.Errorf("write message failed: %w", err)
	}

	return nil
}

// startHeartbeat 启动心跳
func (c *AGPClient) startHeartbeat() {
	go func() {
		ticker := time.NewTicker(c.heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-c.stopCh:
				return
			case <-ticker.C:
				c.mu.RLock()
				conn := c.conn
				state := c.state
				c.mu.RUnlock()

				if state != "connected" || conn == nil {
					return
				}

				// 发送 ping
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					logger.Error("AGP send ping failed: %v", err)
					return
				}

				// 检查 pong 超时
				c.pongMu.RLock()
				lastPong := c.lastPongTime
				c.pongMu.RUnlock()

				if time.Since(lastPong) > c.heartbeatInterval*2 {
					logger.Warn("AGP pong timeout, reconnecting")
					conn.Close()
					return
				}
			}
		}
	}()
}

// startWakeupDetection 启动唤醒检测
func (c *AGPClient) startWakeupDetection() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-c.stopCh:
				return
			case <-ticker.C:
				c.tickMu.Lock()
				now := time.Now()
				drift := now.Sub(c.lastTickTime)
				c.lastTickTime = now
				c.tickMu.Unlock()

				// 检测时间漂移（系统休眠/唤醒）
				if drift > 15*time.Second {
					logger.Warn("AGP system wakeup detected (drift: %v), reconnecting", drift)
					c.mu.RLock()
					conn := c.conn
					c.mu.RUnlock()
					if conn != nil {
						conn.Close()
					}
					return
				}

				c.mu.RLock()
				state := c.state
				c.mu.RUnlock()
				if state != "connected" {
					return
				}
			}
		}
	}()
}

// shouldReconnect 判断是否应该重连
func (c *AGPClient) shouldReconnect() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.maxReconnectAttempts > 0 && c.reconnectAttempts >= c.maxReconnectAttempts {
		return false
	}

	c.reconnectAttempts++
	return true
}

// scheduleReconnect 安排重连
func (c *AGPClient) scheduleReconnect() {
	c.mu.RLock()
	attempts := c.reconnectAttempts
	c.mu.RUnlock()

	// 指数退避: 3s * 1.5^attempts, 最大 25s
	delay := c.reconnectInterval * time.Duration(1.5*float64(attempts))
	if delay > 25*time.Second {
		delay = 25 * time.Second
	}

	logger.Info("AGP reconnecting in %v (attempt %d)", delay, attempts)

	select {
	case <-c.stopCh:
		return
	case <-time.After(delay):
	}
}

// isDuplicate 检查消息是否重复
func (c *AGPClient) isDuplicate(msgID string) bool {
	if _, exists := c.processedMsgIDs.LoadOrStore(msgID, true); exists {
		return true
	}
	return false
}

// startMsgIDCleanup 启动消息 ID 清理
func (c *AGPClient) startMsgIDCleanup() {
	c.cleanupTicker = time.NewTicker(5 * time.Minute)
	go func() {
		for {
			select {
			case <-c.stopCh:
				c.cleanupTicker.Stop()
				return
			case <-c.cleanupTicker.C:
				// 清理旧的消息 ID（保留最近 1000 条）
				count := 0
				c.processedMsgIDs.Range(func(key, value interface{}) bool {
					count++
					if count > 1000 {
						c.processedMsgIDs.Delete(key)
					}
					return true
				})
			}
		}
	}()
}
