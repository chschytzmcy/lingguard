package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/pkg/httpclient"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/speech"
	"github.com/lingguard/pkg/stream"
)

// QQ Bot API endpoints
const (
	qqAPIBase   = "https://api.sgroup.qq.com"
	qqTokenURL  = "https://bots.qq.com/app/getAppAccessToken"
	qqTokenType = "QQBot" // 官方 botgo SDK 使用的 token 类型
)

// QQ opcode constants
const (
	qqOpDispatch            = 0
	qqOpHeartbeat           = 1
	qqOpIdentify            = 2
	qqOpResume              = 6
	qqOpReconnect           = 7
	qqOpInvalidSession      = 9
	qqOpHello               = 10
	qqOpHeartbeatAck        = 11
	qqOpHTTPCallbackAck     = 12
	qqOpPlatformCallbackAck = 13
)

// QQ event types
const (
	qqEventReady            = "READY"
	qqEventResumed          = "RESUMED"
	qqEventC2CMessageCreate = "C2C_MESSAGE_CREATE"
	qqEventDirectMessage    = "DIRECT_MESSAGE_CREATE"
	qqEventGroupAtMessage   = "GROUP_AT_MESSAGE_CREATE"
	qqEventAtMessageCreate  = "AT_MESSAGE_CREATE" // 频道 @ 消息
)

// QQ intent flags - 参考 OpenClaw 实现
const (
	qqIntentGuilds         = 1 << 0  // 频道相关
	qqIntentGuildMembers   = 1 << 1  // 频道成员
	qqIntentPublicMessages = 1 << 30 // 频道公开消息（公域）
	qqIntentDirectMessage  = 1 << 12 // 频道私信
	qqIntentGroupAndC2C    = 1 << 25 // 群聊和 C2C 私聊（需申请）
)

// qqIntentLevel Intent 权限级别 - 参考 OpenClaw 的 3 级降级机制
type qqIntentLevel struct {
	Name        string
	Intents     int
	Description string
}

// 3 级权限：从高到低依次尝试
var qqIntentLevels = []qqIntentLevel{
	{
		Name:        "full",
		Intents:     qqIntentPublicMessages | qqIntentDirectMessage | qqIntentGroupAndC2C,
		Description: "群聊+私信+频道",
	},
	{
		Name:        "group+channel",
		Intents:     qqIntentPublicMessages | qqIntentGroupAndC2C,
		Description: "群聊+频道",
	},
	{
		Name:        "channel-only",
		Intents:     qqIntentPublicMessages | qqIntentGuildMembers,
		Description: "仅频道消息",
	},
}

// QQ payload structures
type qqPayload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d"`
	S  int             `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
}

type qqHelloData struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

type qqIdentifyData struct {
	Token      string  `json:"token"` // 格式: "QQBot {access_token}"
	Intents    int     `json:"intents"`
	Shard      []int   `json:"shard,omitempty"`
	Properties qqProps `json:"properties,omitempty"`
}

type qqResumeData struct {
	Token     string `json:"token"`
	SessionID string `json:"session_id"`
	Seq       int    `json:"seq"`
}

// qqAccessTokenResponse 获取 AccessToken 的响应
type qqAccessTokenResponse struct {
	Code        int    `json:"code"`
	Message     string `json:"message"`
	AccessToken string `json:"access_token"`
	ExpiresIn   any    `json:"expires_in"` // 可能是 string 或 int
}

type qqProps struct {
	OS      string `json:"$os"`
	Browser string `json:"$browser"`
	Device  string `json:"$device"`
}

type qqReadyData struct {
	Version   int    `json:"version"`
	SessionID string `json:"session_id"`
	User      qqUser `json:"user"`
	Shard     []int  `json:"shard"`
}

type qqUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Bot      bool   `json:"bot"`
}

type qqC2CMessage struct {
	ID          string         `json:"id"`
	Content     string         `json:"content"`
	Timestamp   string         `json:"timestamp"`
	Author      qqAuthor       `json:"author"`
	Attachments []qqAttachment `json:"attachments,omitempty"`
}

type qqGroupMessage struct {
	ID          string         `json:"id"`
	Content     string         `json:"content"`
	Timestamp   string         `json:"timestamp"`
	Author      qqGroupAuthor  `json:"author"`
	GroupID     string         `json:"group_id"`
	GroupOpenID string         `json:"group_openid"`
	Attachments []qqAttachment `json:"attachments,omitempty"`
}

type qqAuthor struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

type qqGroupAuthor struct {
	MemberOpenID string `json:"member_openid"`
}

type qqAttachment struct {
	Type        string `json:"type"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Filename    string `json:"filename"`
}

// QQChannel QQ机器人渠道 (使用 WebSocket Gateway)
type QQChannel struct {
	cfg              *config.QQConfig
	speechCfg        *config.SpeechConfig
	speechService    speech.Service
	handler          MessageHandler
	streamingHandler StreamingMessageHandler
	allowMap         map[string]bool

	// WebSocket connection
	conn      *websocket.Conn
	connMu    sync.Mutex
	running   bool
	sessionID string
	sequence  int

	// Heartbeat
	heartbeatInterval time.Duration
	heartbeatTicker   *time.Ticker
	lastHeartbeatAck  time.Time

	// Message deduplication
	processedMsgs sync.Map
	dedupeMu      sync.Mutex

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// HTTP client for API calls
	httpClient *http.Client

	// Rate limiting (QQ API: 5 messages per minute per user)
	rateLimiters sync.Map // key: userID, value: *qqRateLimiter

	// AccessToken management (官方 botgo SDK 鉴权方式)
	accessToken    string
	accessTokenMu  sync.RWMutex
	tokenExpiry    time.Time
	tokenRefreshCh chan struct{}

	// Intent level management - 参考 OpenClaw
	intentLevelIndex          int // 当前使用的权限级别索引
	lastSuccessfulIntentLevel int // 上次成功的权限级别

	// Reconnect management - 参考 OpenClaw
	reconnectAttempts        int
	maxReconnectAttempts     int
	reconnectDelays          []time.Duration
	lastConnectTime          time.Time
	quickDisconnectCount     int
	quickDisconnectThreshold time.Duration
	maxQuickDisconnectCount  int

	// msg_seq tracker - 用于对同一条消息的多次回复
	msgSeqTracker sync.Map // key: msgID, value: current seq
	seqBaseTime   int64    // 基于时间戳的基准值

	// Gateway URL cache
	gatewayURL string
}

// qqRateLimiter 用户级限流器
type qqRateLimiter struct {
	mu         sync.Mutex
	timestamps []time.Time   // 最近发送的消息时间戳
	limit      int           // 每分钟限制
	window     time.Duration // 时间窗口
}

func newQQRateLimiter(limit int, window time.Duration) *qqRateLimiter {
	return &qqRateLimiter{
		limit:      limit,
		window:     window,
		timestamps: make([]time.Time, 0, limit),
	}
}

func (r *qqRateLimiter) allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	// 清理过期的记录
	valid := r.timestamps[:0]
	for _, t := range r.timestamps {
		if now.Sub(t) <= r.window {
			valid = append(valid, t)
		}
	}
	r.timestamps = valid

	return len(r.timestamps) < r.limit
}

func (r *qqRateLimiter) record() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.timestamps = append(r.timestamps, time.Now())
}

// NewQQChannel 创建QQ渠道
func NewQQChannel(cfg *config.QQConfig, speechCfg *config.SpeechConfig, providers map[string]config.ProviderConfig, handler MessageHandler) *QQChannel {
	allowMap := make(map[string]bool)
	for _, id := range cfg.AllowFrom {
		allowMap[id] = true
	}

	qc := &QQChannel{
		cfg:            cfg,
		speechCfg:      speechCfg,
		handler:        handler,
		allowMap:       allowMap,
		httpClient:     httpclient.Default(),
		rateLimiters:   sync.Map{},
		tokenRefreshCh: make(chan struct{}, 1),
		// Reconnect config - 参考 OpenClaw
		maxReconnectAttempts:     100,
		reconnectDelays:          []time.Duration{1 * time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second},
		quickDisconnectThreshold: 5 * time.Second,
		maxQuickDisconnectCount:  3,
		// msg_seq 基准时间
		seqBaseTime: time.Now().Unix() % 100000000,
	}

	// 初始化语音识别服务
	if speechCfg != nil && speechCfg.Enabled {
		apiKey := speechCfg.APIKey
		if apiKey == "" && speechCfg.Provider != "" {
			if providerCfg, ok := providers[speechCfg.Provider]; ok {
				apiKey = providerCfg.APIKey
			}
		}
		svc, err := speech.NewService(&speech.Config{
			Provider: speechCfg.Provider,
			APIKey:   apiKey,
			APIBase:  speechCfg.APIBase,
			Model:    speechCfg.Model,
			Format:   speechCfg.Format,
			Language: speechCfg.Language,
			Timeout:  speechCfg.Timeout,
		})
		if err != nil {
			logger.Warn("Failed to init speech service for QQ", "error", err)
		} else {
			qc.speechService = svc
			logger.Info("Speech recognition enabled for QQ channel", "provider", speechCfg.Provider)
		}
	}

	// 检查是否实现了流式处理器接口
	if sh, ok := handler.(StreamingMessageHandler); ok {
		qc.streamingHandler = sh
	}
	return qc
}

// Name 返回渠道名称（使用配置中的 name 字段）
func (q *QQChannel) Name() string {
	if q.cfg.Name != "" {
		return q.cfg.Name
	}
	return "qq" // 默认名称
}

// Start 启动渠道
func (q *QQChannel) Start(ctx context.Context) error {
	if q.running {
		return nil
	}

	// Create context for graceful shutdown
	q.ctx, q.cancel = context.WithCancel(ctx)
	q.running = true

	// 获取初始 AccessToken
	if err := q.fetchAccessToken(); err != nil {
		return fmt.Errorf("get initial access token: %w", err)
	}

	// 启动 token 自动刷新
	q.startTokenRefresh()

	// Start connection loop in goroutine
	go q.connectionLoop()

	logger.Info("QQ channel started (WebSocket Gateway)")
	logger.Info("Features: Intent downgrade, Session Resume, Quick disconnect detection")
	return nil
}

// Stop 停止渠道
func (q *QQChannel) Stop() error {
	if q.cancel != nil {
		q.cancel()
	}
	q.running = false
	q.stopHeartbeat()
	q.closeConnection()
	logger.Info("QQ channel stopped")
	return nil
}

// IsRunning 检查是否运行中
func (q *QQChannel) IsRunning() bool {
	return q.running
}

// fetchAccessToken 从 QQ 服务器获取 AccessToken（官方 botgo SDK 鉴权方式）
func (q *QQChannel) fetchAccessToken() error {
	reqBody := map[string]string{
		"appId":        q.cfg.AppID,
		"clientSecret": q.cfg.AppSecret,
	}
	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal token request: %w", err)
	}

	req, err := http.NewRequestWithContext(q.ctx, "POST", qqTokenURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request token: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read token response: %w", err)
	}

	var tokenResp qqAccessTokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return fmt.Errorf("unmarshal token response: %w", err)
	}

	if tokenResp.Code != 0 {
		return fmt.Errorf("token API error: code=%d, message=%s", tokenResp.Code, tokenResp.Message)
	}

	if tokenResp.AccessToken == "" {
		return fmt.Errorf("empty access token received")
	}

	// 存储 token 和过期时间（提前 5 分钟刷新，参考 OpenClaw）
	expiresIn := parseExpiresIn(tokenResp.ExpiresIn)
	q.accessTokenMu.Lock()
	q.accessToken = tokenResp.AccessToken
	q.tokenExpiry = time.Now().Add(time.Duration(expiresIn-300) * time.Second)
	q.accessTokenMu.Unlock()

	logger.Info("QQ AccessToken obtained", "expiresIn", expiresIn)
	return nil
}

// getAccessToken 获取当前有效的 AccessToken，如果过期则刷新
func (q *QQChannel) getAccessToken() (string, error) {
	q.accessTokenMu.RLock()
	token := q.accessToken
	expiry := q.tokenExpiry
	q.accessTokenMu.RUnlock()

	// 如果 token 有效（提前 5 分钟），直接返回
	if token != "" && time.Now().Before(expiry) {
		return token, nil
	}

	// Token 过期或不存在，需要刷新
	if err := q.fetchAccessToken(); err != nil {
		return "", err
	}

	q.accessTokenMu.RLock()
	token = q.accessToken
	q.accessTokenMu.RUnlock()

	return token, nil
}

// startTokenRefresh 启动 token 自动刷新协程 - 参考 OpenClaw 后台刷新
func (q *QQChannel) startTokenRefresh() {
	go func() {
		for {
			q.accessTokenMu.RLock()
			expiry := q.tokenExpiry
			q.accessTokenMu.RUnlock()

			// 计算到期的剩余时间，提前 5 分钟刷新
			sleepDuration := time.Until(expiry)
			if sleepDuration <= 0 {
				sleepDuration = 30 * time.Second
			}

			select {
			case <-q.ctx.Done():
				return
			case <-time.After(sleepDuration):
				if err := q.fetchAccessToken(); err != nil {
					logger.Error("Failed to refresh QQ AccessToken", "error", err)
				}
			}
		}
	}()
}

// Send 主动发送消息
func (q *QQChannel) Send(ctx context.Context, to string, content string) error {
	return q.sendC2CMessage(ctx, to, content, "")
}

// connectionLoop WebSocket连接循环 - 参考 OpenClaw 的重连机制
func (q *QQChannel) connectionLoop() {
	for q.running {
		select {
		case <-q.ctx.Done():
			return
		default:
			if err := q.connect(); err != nil {
				delay := q.getReconnectDelay()
				logger.Warn("QQ WebSocket connection failed", "error", err, "retryIn", delay)
				q.scheduleReconnect(delay)
				continue
			}

			// 记录连接成功时间
			q.lastConnectTime = time.Now()
			q.reconnectAttempts = 0

			// Connection established, start message loop
			q.messageLoop()

			// Connection lost
			if q.running {
				q.handleDisconnect()
			}
		}
	}
}

// getReconnectDelay 获取重连延迟 - 参考 OpenClaw 递增延迟
func (q *QQChannel) getReconnectDelay() time.Duration {
	idx := q.reconnectAttempts
	if idx >= len(q.reconnectDelays) {
		idx = len(q.reconnectDelays) - 1
	}
	return q.reconnectDelays[idx]
}

// scheduleReconnect 安排重连
func (q *QQChannel) scheduleReconnect(delay time.Duration) {
	q.reconnectAttempts++
	if q.reconnectAttempts > q.maxReconnectAttempts {
		logger.Error("QQ max reconnect attempts reached")
		return
	}
	time.Sleep(delay)
}

// handleDisconnect 处理断开连接 - 参考 OpenClaw 快速断开检测
func (q *QQChannel) handleDisconnect() {
	connectionDuration := time.Since(q.lastConnectTime)

	// 检测是否是快速断开
	if connectionDuration < q.quickDisconnectThreshold && !q.lastConnectTime.IsZero() {
		q.quickDisconnectCount++
		logger.Warn("QQ quick disconnect detected",
			"duration", connectionDuration,
			"count", q.quickDisconnectCount)

		// 如果连续快速断开超过阈值，可能是权限问题
		if q.quickDisconnectCount >= q.maxQuickDisconnectCount {
			logger.Error("QQ too many quick disconnects - possible permission issue",
				"hint", "Check: 1) AppID/Secret correct 2) Bot permissions on QQ Open Platform")
			q.quickDisconnectCount = 0
			// 等待更长时间
			time.Sleep(60 * time.Second)
			return
		}
	} else {
		// 连接持续时间够长，重置计数
		q.quickDisconnectCount = 0
	}

	time.Sleep(5 * time.Second)
}

// connect 建立WebSocket连接
func (q *QQChannel) connect() error {
	q.connMu.Lock()
	defer q.connMu.Unlock()

	// 获取 Gateway URL（动态获取或使用缓存）
	gatewayURL, err := q.getGatewayURL()
	if err != nil {
		return fmt.Errorf("get gateway URL: %w", err)
	}

	logger.Info("QQ connecting to gateway", "url", gatewayURL)

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(q.ctx, gatewayURL, nil)
	if err != nil {
		return fmt.Errorf("dial gateway: %w", err)
	}
	q.conn = conn

	return nil
}

// getGatewayURL 获取 Gateway URL - 参考 OpenClaw 动态获取
func (q *QQChannel) getGatewayURL() (string, error) {
	// 优先使用缓存
	if q.gatewayURL != "" {
		return q.gatewayURL, nil
	}

	accessToken, err := q.getAccessToken()
	if err != nil {
		return "", fmt.Errorf("get access token: %w", err)
	}

	url := qqAPIBase + "/gateway"
	req, err := http.NewRequestWithContext(q.ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create gateway request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("%s %s", qqTokenType, accessToken))

	resp, err := q.httpClient.Do(req)
	if err != nil {
		// 如果获取失败，使用默认 URL
		logger.Warn("Failed to get gateway URL, using default", "error", err)
		return "wss://api.sgroup.qq.com/websocket", nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read gateway response: %w", err)
	}

	var gatewayResp struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(respBody, &gatewayResp); err != nil {
		return "", fmt.Errorf("unmarshal gateway response: %w", err)
	}

	if gatewayResp.URL == "" {
		return "", fmt.Errorf("empty gateway URL received")
	}

	// 缓存 gateway URL
	q.gatewayURL = gatewayResp.URL
	return gatewayResp.URL, nil
}

// closeConnection 关闭WebSocket连接
func (q *QQChannel) closeConnection() {
	q.connMu.Lock()
	defer q.connMu.Unlock()
	if q.conn != nil {
		q.conn.Close()
		q.conn = nil
	}
}

// messageLoop 消息循环
func (q *QQChannel) messageLoop() {
	defer q.closeConnection()

	for {
		select {
		case <-q.ctx.Done():
			return
		default:
			if q.conn == nil {
				return
			}

			_, message, err := q.conn.ReadMessage()
			if err != nil {
				if q.running {
					// 检查是否是认证失败
					if strings.Contains(err.Error(), "4004") {
						logger.Error("QQ 认证失败！",
							"hint", "检查 AppID 和 AppSecret 是否正确")
					} else {
						logger.Info("QQ WebSocket read error", "error", err)
					}
				}
				return
			}

			if err := q.handlePayload(message); err != nil {
				logger.Warn("QQ payload handling error", "error", err)
			}
		}
	}
}

// handlePayload 处理WebSocket消息 - 参考 OpenClaw
func (q *QQChannel) handlePayload(data []byte) error {
	var payload qqPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	// Update sequence number
	if payload.S > 0 {
		q.sequence = payload.S
	}

	switch payload.Op {
	case qqOpHello:
		// Server hello, need to identify or resume
		var helloData qqHelloData
		if err := json.Unmarshal(payload.D, &helloData); err != nil {
			return fmt.Errorf("unmarshal hello: %w", err)
		}
		q.heartbeatInterval = time.Duration(helloData.HeartbeatInterval) * time.Millisecond
		q.startHeartbeat()

		// 如果有 session_id，尝试 Resume；否则 Identify
		if q.sessionID != "" && q.sequence > 0 {
			return q.resume()
		}
		return q.identify()

	case qqOpDispatch:
		return q.handleDispatch(payload.T, payload.D)

	case qqOpHeartbeatAck:
		q.lastHeartbeatAck = time.Now()
		logger.Debug("QQ heartbeat acknowledged")

	case qqOpReconnect:
		logger.Info("QQ server requested reconnect")
		return fmt.Errorf("reconnect requested")

	case qqOpInvalidSession:
		// 参考 OpenClaw：Invalid Session 处理
		var canResume bool
		if err := json.Unmarshal(payload.D, &canResume); err != nil {
			canResume = false
		}

		currentLevel := qqIntentLevels[q.intentLevelIndex]
		logger.Warn("QQ invalid session",
			"intentLevel", currentLevel.Name,
			"canResume", canResume)

		if !canResume {
			q.sessionID = ""
			q.sequence = 0

			// 尝试降级到下一个权限级别
			if q.intentLevelIndex < len(qqIntentLevels)-1 {
				q.intentLevelIndex++
				nextLevel := qqIntentLevels[q.intentLevelIndex]
				logger.Info("QQ downgrading intents", "newLevel", nextLevel.Description)
			} else {
				// 已经是最低权限级别
				logger.Error("QQ all intent levels failed, please check AppID/Secret")
			}
		}

		return fmt.Errorf("invalid session")

	default:
		logger.Debug("QQ unknown opcode", "opcode", payload.Op)
	}

	return nil
}

// handleDispatch 处理事件分发 - 参考 OpenClaw
func (q *QQChannel) handleDispatch(eventType string, data json.RawMessage) error {
	switch eventType {
	case qqEventReady:
		var ready qqReadyData
		if err := json.Unmarshal(data, &ready); err != nil {
			return fmt.Errorf("unmarshal ready: %w", err)
		}
		q.sessionID = ready.SessionID
		// 记录成功的权限级别
		q.lastSuccessfulIntentLevel = q.intentLevelIndex

		currentLevel := qqIntentLevels[q.intentLevelIndex]
		logger.Info("QQ bot ready",
			"username", ready.User.Username,
			"session", ready.SessionID[:min(8, len(ready.SessionID))],
			"intentLevel", currentLevel.Description)

	case qqEventResumed:
		logger.Info("QQ session resumed")

	case qqEventC2CMessageCreate, qqEventDirectMessage:
		var msg qqC2CMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return fmt.Errorf("unmarshal message: %w", err)
		}
		go q.handleC2CMessage(&msg)

	case qqEventGroupAtMessage:
		var msg qqGroupMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return fmt.Errorf("unmarshal group message: %w", err)
		}
		go q.handleGroupMessage(&msg)

	default:
		logger.Debug("QQ unhandled event", "event", eventType)
	}
	return nil
}

// identify 发送身份认证 - 参考 OpenClaw Intent 机制
func (q *QQChannel) identify() error {
	accessToken, err := q.getAccessToken()
	if err != nil {
		logger.Error("Failed to get QQ AccessToken", "error", err)
		return fmt.Errorf("get access token: %w", err)
	}

	// 使用官方 SDK 的 token 格式: "QQBot {access_token}"
	token := fmt.Sprintf("%s %s", qqTokenType, accessToken)

	// 如果有上次成功的级别，直接使用；否则从当前级别开始
	levelToUse := q.intentLevelIndex
	if q.lastSuccessfulIntentLevel >= 0 {
		levelToUse = q.lastSuccessfulIntentLevel
	}

	intentLevel := qqIntentLevels[min(levelToUse, len(qqIntentLevels)-1)]
	logger.Info("QQ identifying",
		"appId", q.cfg.AppID,
		"intents", intentLevel.Intents,
		"level", intentLevel.Description,
		"tokenType", qqTokenType)

	identify := qqPayload{
		Op: qqOpIdentify,
		D: mustMarshal(qqIdentifyData{
			Token:   token,
			Intents: intentLevel.Intents,
			Shard:   []int{0, 1},
			Properties: qqProps{
				OS:      "linux",
				Browser: "lingguard",
				Device:  "lingguard",
			},
		}),
	}

	return q.sendPayload(&identify)
}

// resume 恢复会话 - 参考 OpenClaw Session Resume
func (q *QQChannel) resume() error {
	accessToken, err := q.getAccessToken()
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	token := fmt.Sprintf("%s %s", qqTokenType, accessToken)
	logger.Info("QQ attempting session resume", "session", q.sessionID[:min(8, len(q.sessionID))])

	resume := qqPayload{
		Op: qqOpResume,
		D: mustMarshal(qqResumeData{
			Token:     token,
			SessionID: q.sessionID,
			Seq:       q.sequence,
		}),
	}

	return q.sendPayload(&resume)
}

// startHeartbeat 启动心跳
func (q *QQChannel) startHeartbeat() {
	if q.heartbeatTicker != nil {
		q.heartbeatTicker.Stop()
	}
	q.heartbeatTicker = time.NewTicker(q.heartbeatInterval)
	q.lastHeartbeatAck = time.Now()

	go func() {
		for {
			select {
			case <-q.ctx.Done():
				return
			case <-q.heartbeatTicker.C:
				if err := q.sendHeartbeat(); err != nil {
					logger.Warn("QQ heartbeat failed", "error", err)
					return
				}
				// Check if we're receiving acks
				if time.Since(q.lastHeartbeatAck) > q.heartbeatInterval*3 {
					logger.Warn("QQ heartbeat timeout, reconnecting...")
					q.closeConnection()
					return
				}
			}
		}
	}()
}

// stopHeartbeat 停止心跳
func (q *QQChannel) stopHeartbeat() {
	if q.heartbeatTicker != nil {
		q.heartbeatTicker.Stop()
		q.heartbeatTicker = nil
	}
}

// sendHeartbeat 发送心跳
func (q *QQChannel) sendHeartbeat() error {
	heartbeat := qqPayload{
		Op: qqOpHeartbeat,
		D:  json.RawMessage("null"),
		S:  q.sequence,
	}
	return q.sendPayload(&heartbeat)
}

// sendPayload 发送WebSocket消息
func (q *QQChannel) sendPayload(payload *qqPayload) error {
	q.connMu.Lock()
	defer q.connMu.Unlock()
	if q.conn == nil {
		return fmt.Errorf("connection not established")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return q.conn.WriteMessage(websocket.TextMessage, data)
}

// handleC2CMessage 处理私聊消息
func (q *QQChannel) handleC2CMessage(msg *qqC2CMessage) {
	// Deduplication check
	if q.isProcessed(msg.ID) {
		logger.Debug("Skipping duplicate QQ message", "id", msg.ID)
		return
	}
	q.markProcessed(msg.ID)

	content := strings.TrimSpace(msg.Content)
	userID := msg.Author.ID
	if userID == "" {
		return
	}

	// Permission check
	if len(q.allowMap) > 0 && !q.allowMap[userID] {
		logger.Warn("Access denied on channel qq", "sender", userID)
		return
	}

	// 处理附件（图片、音频等）
	var imageUrls []string
	var attachmentInfo string
	if len(msg.Attachments) > 0 {
		logger.Info("QQ C2C message has attachments", "count", len(msg.Attachments))
		for _, att := range msg.Attachments {
			// 图片处理
			if att.ContentType != "" && strings.HasPrefix(att.ContentType, "image/") {
				imageBase64, err := q.downloadImageAsBase64(att.URL)
				if err != nil {
					logger.Warn("Failed to download QQ image", "url", att.URL, "error", err)
					attachmentInfo += fmt.Sprintf("\n[图片下载失败: %s]", err.Error())
				} else {
					imageUrls = append(imageUrls, imageBase64)
					attachmentInfo += "\n[用户发送了一张图片，请根据图片内容回复]"
					logger.Info("QQ image downloaded successfully", "base64Len", len(imageBase64))
				}
			} else if att.ContentType != "" && strings.HasPrefix(att.ContentType, "audio/") ||
				// QQ 语音通常是 amr 格式
				strings.HasSuffix(strings.ToLower(att.Filename), ".amr") ||
				strings.HasSuffix(strings.ToLower(att.Filename), ".mp3") ||
				strings.HasSuffix(strings.ToLower(att.Filename), ".wav") ||
				strings.HasSuffix(strings.ToLower(att.Filename), ".ogg") ||
				strings.HasSuffix(strings.ToLower(att.Filename), ".opus") ||
				strings.HasSuffix(strings.ToLower(att.Filename), ".m4a") {
				// 音频处理 - 下载并识别
				audioText := q.processAudioAttachment(q.ctx, att.URL, att.Filename, msg.ID)
				if audioText != "" {
					attachmentInfo += fmt.Sprintf("\n[语音内容: %s]", audioText)
				} else {
					attachmentInfo += fmt.Sprintf("\n[音频附件: %s]", att.Filename)
				}
			} else {
				attachmentInfo += fmt.Sprintf("\n[附件: %s]", att.Filename)
			}
		}
	}

	// 如果既没有文本也没有附件，跳过
	if content == "" && len(msg.Attachments) == 0 {
		return
	}

	// 合并文本和附件信息
	fullContent := content + attachmentInfo

	logger.Info("QQ C2C message received",
		"sender", userID,
		"content", truncateContent(content, 100),
		"attachments", len(msg.Attachments),
		"imageCount", len(imageUrls))

	// Build Message - 图片 base64 放入 Media 字段供多模态模型处理
	channelMsg := &Message{
		ID:        msg.ID,
		SessionID: "qq-" + userID,
		Content:   fullContent,
		Channel:   "qq",
		UserID:    userID,
		Media:     imageUrls,
		Metadata: map[string]any{
			"user_id":     userID,
			"username":    msg.Author.Username,
			"message_id":  msg.ID,
			"msg_type":    "c2c",
			"attachments": msg.Attachments,
		},
	}

	// 检查是否支持流式处理
	if q.streamingHandler != nil {
		q.handleMessageStream(q.ctx, channelMsg, userID, msg.ID)
		return
	}

	// Call handler (non-streaming fallback)
	reply, err := q.handler.HandleMessage(q.ctx, channelMsg)
	if err != nil {
		logger.Error("Handler error", "error", err)
		return
	}

	// Send reply
	if reply != "" {
		if err := q.sendC2CMessage(q.ctx, userID, reply, msg.ID); err != nil {
			logger.Error("Failed to send QQ reply", "error", err)
		}
	}
}

// downloadImageAsBase64 下载 QQ 图片并转换为 base64
func (q *QQChannel) downloadImageAsBase64(url string) (string, error) {
	accessToken, err := q.getAccessToken()
	if err != nil {
		return "", fmt.Errorf("get access token: %w", err)
	}

	req, err := http.NewRequestWithContext(q.ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	// 使用 QQ Bot 认证
	req.Header.Set("Authorization", fmt.Sprintf("%s %s", qqTokenType, accessToken))

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download failed: status=%d", resp.StatusCode)
	}

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read image data: %w", err)
	}

	// 检测图片类型
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png" // 默认
	}

	// 转换为 base64 data URL
	base64Data := base64Encode(imageData)
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data), nil
}

// base64Encode Base64 编码
func base64Encode(data []byte) string {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	result := make([]byte, 0, (len(data)+2)/3*4)

	for i := 0; i < len(data); i += 3 {
		var n uint32
		remaining := len(data) - i

		if remaining >= 3 {
			n = uint32(data[i])<<16 | uint32(data[i+1])<<8 | uint32(data[i+2])
			result = append(result,
				base64Chars[n>>18&0x3F],
				base64Chars[n>>12&0x3F],
				base64Chars[n>>6&0x3F],
				base64Chars[n&0x3F],
			)
		} else if remaining == 2 {
			n = uint32(data[i])<<16 | uint32(data[i+1])<<8
			result = append(result,
				base64Chars[n>>18&0x3F],
				base64Chars[n>>12&0x3F],
				base64Chars[n>>6&0x3F],
				'=',
			)
		} else {
			n = uint32(data[i]) << 16
			result = append(result,
				base64Chars[n>>18&0x3F],
				base64Chars[n>>12&0x3F],
				'=',
				'=',
			)
		}
	}

	return string(result)
}

// handleGroupMessage 处理群消息
func (q *QQChannel) handleGroupMessage(msg *qqGroupMessage) {
	// Deduplication check
	if q.isProcessed(msg.ID) {
		return
	}
	q.markProcessed(msg.ID)

	content := strings.TrimSpace(msg.Content)
	userID := msg.Author.MemberOpenID
	groupID := msg.GroupOpenID

	// 处理附件（图片等）
	var imageUrls []string
	var attachmentInfo string
	if len(msg.Attachments) > 0 {
		logger.Info("QQ group message has attachments", "count", len(msg.Attachments))
		for _, att := range msg.Attachments {
			if att.ContentType != "" && strings.HasPrefix(att.ContentType, "image/") {
				// 下载图片并转换为 base64
				imageBase64, err := q.downloadImageAsBase64(att.URL)
				if err != nil {
					logger.Warn("Failed to download QQ group image", "url", att.URL, "error", err)
					attachmentInfo += fmt.Sprintf("\n[图片下载失败: %s]", err.Error())
				} else {
					imageUrls = append(imageUrls, imageBase64)
					attachmentInfo += "\n[用户发送了一张图片，请根据图片内容回复]"
					logger.Info("QQ group image downloaded successfully", "base64Len", len(imageBase64))
				}
			} else {
				attachmentInfo += fmt.Sprintf("\n[附件: %s]", att.Filename)
			}
		}
	}

	// 如果既没有文本也没有附件，跳过
	if content == "" && len(msg.Attachments) == 0 {
		return
	}

	// 合并文本和附件信息
	fullContent := content + attachmentInfo

	logger.Info("QQ group message received",
		"group", groupID,
		"sender", userID,
		"content", truncateContent(content, 100),
		"attachments", len(msg.Attachments),
		"imageCount", len(imageUrls))

	// Build Message
	channelMsg := &Message{
		ID:        msg.ID,
		SessionID: "qq-group-" + groupID,
		Content:   fullContent,
		Channel:   "qq",
		UserID:    userID,
		Metadata: map[string]any{
			"user_id":     userID,
			"group_id":    groupID,
			"message_id":  msg.ID,
			"msg_type":    "group",
			"image_urls":  imageUrls,
			"attachments": msg.Attachments,
		},
	}

	// Call handler
	reply, err := q.handler.HandleMessage(q.ctx, channelMsg)
	if err != nil {
		logger.Error("Handler error", "error", err)
		return
	}

	// Send reply to group
	if reply != "" {
		if err := q.sendGroupMessage(q.ctx, groupID, reply, msg.ID); err != nil {
			logger.Error("Failed to send QQ group reply", "error", err)
		}
	}
}

// handleMessageStream 流式处理消息
// 注意：QQ API 限制每分钟每用户只能发送 5 条消息，且不支持消息编辑
// 因此不进行流式文本输出，只在完成时发送最终消息
func (q *QQChannel) handleMessageStream(ctx context.Context, msg *Message, userID string, msgID string) {
	var contentBuilder strings.Builder

	err := q.streamingHandler.HandleMessageStream(ctx, msg, func(event stream.StreamEvent) {
		switch event.Type {
		case stream.EventText:
			// 只累积内容，不发送（QQ 限流严格，不支持消息编辑）
			contentBuilder.WriteString(event.Content)

		case stream.EventToolEnd:
			// 检查工具结果是否包含生成的图片，如果有则发送
			if strings.Contains(event.ToolResult, "[GENERATED_IMAGE:") {
				go func() {
					defer func() {
						if r := recover(); r != nil {
							logger.Error("QQ media upload goroutine panic recovered", "error", r)
						}
					}()

					// 处理图片 - 优先使用公网 URL
					imageURLPattern := regexp.MustCompile(`\[IMAGE_URL:([^\]]+)\]`)
					imageURLMatches := imageURLPattern.FindAllStringSubmatch(event.ToolResult, -1)

					for _, match := range imageURLMatches {
						if len(match) > 1 {
							imageURL := match[1]
							logger.Info("Sending generated image to QQ via URL", "url", imageURL[:min(80, len(imageURL))])

							if err := q.sendC2CImageURL(ctx, userID, imageURL, msgID); err != nil {
								logger.Warn("Failed to send image URL to QQ", "error", err)
							}
						}
					}
				}()
			}

			// 处理视频 - 使用公网 URL 发送
			if strings.Contains(event.ToolResult, "[GENERATED_VIDEO:") {
				go func() {
					defer func() {
						if r := recover(); r != nil {
							logger.Error("QQ video upload goroutine panic recovered", "error", r)
						}
					}()

					// 处理视频 - 优先使用公网 URL
					videoURLPattern := regexp.MustCompile(`\[VIDEO_URL:([^\]]+)\]`)
					videoURLMatches := videoURLPattern.FindAllStringSubmatch(event.ToolResult, -1)

					for _, match := range videoURLMatches {
						if len(match) > 1 {
							videoURL := match[1]
							logger.Info("Sending generated video to QQ via URL", "url", videoURL[:min(80, len(videoURL))])

							if err := q.sendC2CVideoURL(ctx, userID, videoURL, msgID); err != nil {
								logger.Warn("Failed to send video URL to QQ", "error", err)
							}
						}
					}
				}()
			}

		case stream.EventDone:
			// 发送完整内容（QQ 限流严格，不支持流式编辑，只发送最终结果）
			content := contentBuilder.String()
			if strings.TrimSpace(content) != "" {
				// 检查是否包含生成的图片标记
				if strings.Contains(content, "[GENERATED_IMAGE:") {
					q.sendC2CWithImages(ctx, userID, content, msgID)
				} else if err := q.sendC2CMessage(ctx, userID, content, msgID); err != nil {
					logger.Error("Failed to send QQ message", "error", err)
				}
			}

		case stream.EventError:
			logger.Error("Stream error", "error", event.Error)
			// 发送内容加错误信息
			content := contentBuilder.String()
			errorContent := content + fmt.Sprintf("\n\n❌ 错误: %s", event.Error.Error())
			if strings.TrimSpace(errorContent) != "" {
				q.sendC2CMessage(ctx, userID, errorContent, msgID)
			}
		}
	})

	if err != nil {
		logger.Error("Stream handling error", "error", err)
	}
}

// getNextMsgSeq 获取并递增消息序号 - 参考 OpenClaw
func (q *QQChannel) getNextMsgSeq(msgID string) int {
	current := 0
	if val, ok := q.msgSeqTracker.Load(msgID); ok {
		current = val.(int)
	}
	next := current + 1
	q.msgSeqTracker.Store(msgID, next)

	// 清理过期的序号（简单策略：保留最近 1000 条）
	// 结合时间戳基准值，确保唯一性
	return int(q.seqBaseTime) + next
}

// getRateLimiter 获取或创建用户限流器
func (q *QQChannel) getRateLimiter(userID string) *qqRateLimiter {
	if limiter, ok := q.rateLimiters.Load(userID); ok {
		return limiter.(*qqRateLimiter)
	}
	newLimiter := newQQRateLimiter(5, time.Minute)
	q.rateLimiters.Store(userID, newLimiter)
	return newLimiter
}

// sendC2CMessage 发送私聊消息 - 支持 msg_id 回复
func (q *QQChannel) sendC2CMessage(ctx context.Context, openid string, content string, msgID string) error {
	if openid == "" {
		return fmt.Errorf("openid is empty")
	}

	// 限流控制
	limiter := q.getRateLimiter(openid)
	if !limiter.allow() {
		return fmt.Errorf("rate limit exceeded: QQ API limits 5 messages per minute per user")
	}
	limiter.record()

	// QQ 消息长度限制
	const maxMsgLen = 2000
	if len(content) > maxMsgLen {
		chunks := splitMessage(content, maxMsgLen-2)
		for i, chunk := range chunks {
			if err := q.sendC2CChunk(ctx, openid, chunk, msgID); err != nil {
				return fmt.Errorf("send chunk %d: %w", i, err)
			}
			if i < len(chunks)-1 {
				time.Sleep(100 * time.Millisecond)
			}
		}
		return nil
	}

	return q.sendC2CChunk(ctx, openid, content, msgID)
}

// sendC2CChunk 发送单个消息块
func (q *QQChannel) sendC2CChunk(ctx context.Context, openid string, content string, msgID string) error {
	accessToken, err := q.getAccessToken()
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	url := fmt.Sprintf("%s/v2/users/%s/messages", qqAPIBase, openid)

	body := map[string]any{
		"content":  content,
		"msg_type": 0,
		"msg_seq":  q.getNextMsgSeq(msgID),
	}

	// 如果有 msgID，添加到请求体（被动回复）
	if msgID != "" {
		body["msg_id"] = msgID
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// 使用官方 SDK 的 Authorization 格式
	req.Header.Set("Authorization", fmt.Sprintf("%s %s", qqTokenType, accessToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Union-Appid", q.cfg.AppID)

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("QQ API error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	logger.Debug("QQ message sent", "to", openid)
	return nil
}

// sendC2CImageMessage 发送私聊图片消息
func (q *QQChannel) sendC2CImageMessage(ctx context.Context, openid string, imagePath string, msgID string) error {
	if openid == "" {
		return fmt.Errorf("openid is empty")
	}

	// 读取图片文件
	fileData, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("read image file: %w", err)
	}

	// 获取 access token
	accessToken, err := q.getAccessToken()
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	// 限流控制
	limiter := q.getRateLimiter(openid)
	if !limiter.allow() {
		return fmt.Errorf("rate limit exceeded: QQ API limits 5 messages per minute per user")
	}
	limiter.record()

	// 使用 multipart/form-data 格式直接发送图片
	url := fmt.Sprintf("%s/v2/users/%s/messages", qqAPIBase, openid)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// 添加 msg_type 字段 (7 = 媒体消息)
	_ = writer.WriteField("msg_type", "7")

	// 添加 msg_seq 字段
	_ = writer.WriteField("msg_seq", fmt.Sprintf("%d", q.getNextMsgSeq(msgID)))

	// 如果有 msgID，添加到请求体
	if msgID != "" {
		_ = writer.WriteField("msg_id", msgID)
	}

	// 添加图片文件 - 使用 file_image 字段
	part, err := writer.CreateFormFile("file_image", filepath.Base(imagePath))
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(fileData); err != nil {
		return fmt.Errorf("write file data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, &body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("%s %s", qqTokenType, accessToken))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Union-Appid", q.cfg.AppID)

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("QQ API error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	logger.Info("QQ image message sent", "to", openid, "path", imagePath)
	return nil
}

// sendC2CImageURL 发送私聊图片消息（使用公网 URL）
// 根据 QQ 官方文档：使用 /v2/users/{openid}/files API 上传并发送图片
func (q *QQChannel) sendC2CImageURL(ctx context.Context, openid string, imageURL string, msgID string) error {
	if openid == "" {
		return fmt.Errorf("openid is empty")
	}

	// 获取 access token
	accessToken, err := q.getAccessToken()
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	// 限流控制
	limiter := q.getRateLimiter(openid)
	if !limiter.allow() {
		return fmt.Errorf("rate limit exceeded: QQ API limits 5 messages per minute per user")
	}
	limiter.record()

	// 使用 /v2/users/{openid}/files API 发送图片
	// 参考: https://bot.q.qq.com/wiki/develop/api-v2/server-inter/message/send-receive/rich-media.html
	apiURL := fmt.Sprintf("%s/v2/users/%s/files", qqAPIBase, openid)

	body := map[string]any{
		"file_type":    1,        // 1=图片
		"url":          imageURL, // 公网 URL
		"srv_send_msg": true,     // 直接发送消息
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	logger.Debug("Sending QQ image via files API", "url", apiURL, "imageURL", imageURL[:min(80, len(imageURL))])

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("%s %s", qqTokenType, accessToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Union-Appid", q.cfg.AppID)

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("QQ API error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	logger.Info("QQ image message sent via files API", "to", openid, "response", string(respBody)[:min(200, len(respBody))])
	return nil
}

// sendC2CVideoURL 发送私聊视频消息（使用公网 URL）
// 根据 QQ 官方文档：使用 /v2/users/{openid}/files API 上传并发送视频
func (q *QQChannel) sendC2CVideoURL(ctx context.Context, openid string, videoURL string, msgID string) error {
	if openid == "" {
		return fmt.Errorf("openid is empty")
	}

	// 获取 access token
	accessToken, err := q.getAccessToken()
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	// 限流控制
	limiter := q.getRateLimiter(openid)
	if !limiter.allow() {
		return fmt.Errorf("rate limit exceeded: QQ API limits 5 messages per minute per user")
	}
	limiter.record()

	// 使用 /v2/users/{openid}/files API 发送视频
	// 参考: https://bot.q.qq.com/wiki/develop/api-v2/server-inter/message/send-receive/rich-media.html
	apiURL := fmt.Sprintf("%s/v2/users/%s/files", qqAPIBase, openid)

	body := map[string]any{
		"file_type":    2,        // 2=视频
		"url":          videoURL, // 公网 URL
		"srv_send_msg": true,     // 直接发送消息
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	logger.Debug("Sending QQ video via files API", "url", apiURL, "videoURL", videoURL[:min(80, len(videoURL))])

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("%s %s", qqTokenType, accessToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Union-Appid", q.cfg.AppID)

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("QQ API error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	logger.Info("QQ video message sent via files API", "to", openid, "response", string(respBody)[:min(200, len(respBody))])
	return nil
}

// sendC2CWithImages 发送私聊消息（支持图片）
// 检测内容中的 [GENERATED_IMAGE:路径] 标记，上传并发送
func (q *QQChannel) sendC2CWithImages(ctx context.Context, openid string, content string, msgID string) error {
	// 正则匹配 [GENERATED_IMAGE:路径]
	imagePattern := regexp.MustCompile(`\[GENERATED_IMAGE:([^\]]+)\]`)
	imageMatches := imagePattern.FindAllStringSubmatch(content, -1)

	// 先发送文本内容（移除所有图片标记）
	textContent := imagePattern.ReplaceAllString(content, "")

	// 发送文本消息
	if strings.TrimSpace(textContent) != "" {
		if err := q.sendC2CMessage(ctx, openid, textContent, msgID); err != nil {
			logger.Warn("Failed to send text message to QQ", "error", err)
		}
	}

	// 逐个发送图片
	for _, match := range imageMatches {
		if len(match) > 1 {
			imagePath := match[1]
			logger.Info("Sending generated image to QQ", "path", imagePath)

			// 等待一小段时间，避免频率限制
			time.Sleep(200 * time.Millisecond)

			if err := q.sendC2CImageMessage(ctx, openid, imagePath, msgID); err != nil {
				logger.Warn("Failed to send image to QQ", "path", imagePath, "error", err)
			}
		}
	}

	return nil
}

// sendGroupMessage 发送群消息
func (q *QQChannel) sendGroupMessage(ctx context.Context, groupOpenID string, content string, msgID string) error {
	if groupOpenID == "" {
		return fmt.Errorf("groupOpenID is empty")
	}

	accessToken, err := q.getAccessToken()
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	url := fmt.Sprintf("%s/v2/groups/%s/messages", qqAPIBase, groupOpenID)

	body := map[string]any{
		"content":  content,
		"msg_type": 0,
		"msg_seq":  q.getNextMsgSeq(msgID),
	}

	if msgID != "" {
		body["msg_id"] = msgID
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("%s %s", qqTokenType, accessToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Union-Appid", q.cfg.AppID)

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("QQ API error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	logger.Debug("QQ group message sent", "to", groupOpenID)
	return nil
}

// splitMessage 分割长消息
func splitMessage(content string, maxLen int) []string {
	var chunks []string

	for len(content) > 0 {
		if idx := strings.Index(content, "\n\n"); idx >= 0 && idx < maxLen-2 {
			chunks = append(chunks, strings.TrimSpace(content[:idx+2]))
			content = content[idx+2:]
			continue
		}
		if len(content) <= maxLen {
			chunks = append(chunks, content)
			break
		}
		chunks = append(chunks, content[:maxLen])
		content = content[maxLen:]
	}

	return chunks
}

// isProcessed 检查消息是否已处理
func (q *QQChannel) isProcessed(messageID string) bool {
	q.dedupeMu.Lock()
	defer q.dedupeMu.Unlock()

	now := time.Now()
	cutoff := now.Add(-1 * time.Hour)
	var toDelete []string
	q.processedMsgs.Range(func(key, value any) bool {
		if t, ok := value.(time.Time); ok && t.Before(cutoff) {
			toDelete = append(toDelete, key.(string))
		}
		return true
	})
	for _, k := range toDelete {
		q.processedMsgs.Delete(k)
	}

	_, exists := q.processedMsgs.Load(messageID)
	return exists
}

// markProcessed 标记消息为已处理
func (q *QQChannel) markProcessed(messageID string) {
	q.processedMsgs.Store(messageID, time.Now())
}

// mustMarshal 辅助函数：必须成功的 JSON 序列化
func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

// truncateContent 截断内容用于日志
func truncateContent(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// parseExpiresIn 解析 ExpiresIn 字段（可能是 string 或 int）
func parseExpiresIn(v any) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int64:
		return val
	case int:
		return int64(val)
	case string:
		var result int64
		fmt.Sscanf(val, "%d", &result)
		return result
	default:
		return 7200 // 默认 2 小时
	}
}

// processAudioAttachment 处理音频附件，下载并进行语音识别
func (q *QQChannel) processAudioAttachment(ctx context.Context, audioURL, filename, messageID string) string {
	if q.speechService == nil {
		logger.Debug("Speech service not available for QQ channel")
		return ""
	}

	// 下载音频文件
	audioPath, err := q.downloadAudioFile(ctx, audioURL, filename, messageID)
	if err != nil {
		logger.Warn("Failed to download QQ audio", "url", audioURL, "error", err)
		return ""
	}
	defer os.Remove(audioPath) // 清理临时文件

	// 检测音频格式并转换
	transcribePath := audioPath
	lowerFilename := strings.ToLower(filename)

	// QQ 语音实际上是 SILK 格式（文件名可能是 .amr 但内容是 SILK）
	// 需要先用 pilk 解码 SILK，再用 ffmpeg 转换为 WAV
	if strings.HasSuffix(lowerFilename, ".amr") || strings.HasSuffix(lowerFilename, ".silk") {
		wavPath, err := q.convertSILKToWAV(audioPath)
		if err != nil {
			logger.Warn("Failed to convert SILK to WAV", "path", audioPath, "error", err)
			// 尝试直接识别原始文件
		} else {
			defer os.Remove(wavPath)
			transcribePath = wavPath
			logger.Debug("SILK converted to WAV", "original", audioPath, "converted", wavPath)
		}
	}

	// 语音识别
	result, err := q.speechService.Transcribe(ctx, transcribePath)
	if err != nil {
		logger.Warn("Failed to transcribe QQ audio", "path", transcribePath, "error", err)
		return ""
	}

	logger.Info("QQ audio transcribed", "text", result.Text, "duration", result.Duration, "messageId", messageID)
	return result.Text
}

// convertSILKToWAV 将 QQ 的 SILK 格式语音转换为 WAV
// QQ 语音实际上是 SILK 编码（文件名可能是 .amr 但内容是 SILK）
// 需要先用 pilk (Python) 解码 SILK 为 PCM，再用 ffmpeg 转换为 WAV
func (q *QQChannel) convertSILKToWAV(silkPath string) (string, error) {
	pcmPath := silkPath + ".pcm"
	wavPath := silkPath + ".wav"

	// Step 1: 使用 Python pilk 解码 SILK 为 PCM
	// 先尝试 24kHz（QQ SILK 常用采样率），如果失败再尝试 16kHz
	sampleRates := []int{24000, 16000, 8000, 44100}
	var decodeErr error

	for _, rate := range sampleRates {
		cmd := exec.Command("python3", "-c",
			fmt.Sprintf("import pilk; pilk.decode('%s', '%s', %d)", silkPath, pcmPath, rate))
		output, err := cmd.CombinedOutput()
		if err == nil {
			// 检查 PCM 文件是否存在且不为空
			if info, statErr := os.Stat(pcmPath); statErr == nil && info.Size() > 0 {
				logger.Debug("SILK decoded to PCM", "path", pcmPath, "sampleRate", rate, "size", info.Size())
				break
			}
		}
		decodeErr = fmt.Errorf("pilk decode failed (rate=%d): %w, output: %s", rate, err, string(output))
	}

	// 检查 PCM 文件是否生成成功
	if _, err := os.Stat(pcmPath); os.IsNotExist(err) {
		return "", fmt.Errorf("SILK decode failed: %w", decodeErr)
	}
	defer os.Remove(pcmPath) // 清理 PCM 文件

	// Step 2: 使用 ffmpeg 将 PCM 转换为 WAV
	// -f s16le: 16-bit signed little-endian PCM 格式
	// -ar 24000: 采样率 24kHz (QQ SILK 默认)
	// -ac 1: 单声道
	cmd := exec.Command("ffmpeg", "-y", "-f", "s16le", "-ar", "24000", "-ac", "1",
		"-i", pcmPath, "-ar", "16000", "-ac", "1", "-f", "wav", wavPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg conversion failed: %w, output: %s", err, string(output))
	}

	// 检查输出文件是否存在
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		return "", fmt.Errorf("converted WAV file not found: %s", wavPath)
	}

	logger.Debug("SILK converted to WAV", "silk", silkPath, "wav", wavPath)
	return wavPath, nil
}

// downloadAudioFile 下载音频文件到本地临时目录
func (q *QQChannel) downloadAudioFile(ctx context.Context, audioURL, filename, messageID string) (string, error) {
	// 创建临时目录
	tmpDir := filepath.Join(os.TempDir(), "lingguard-qq-audio")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	// 生成文件名 - 使用统一的时间戳
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".amr" // QQ 语音默认 amr 格式
	}
	// 使用单一时间戳生成唯一文件名
	timestamp := time.Now().Unix()
	randomID := time.Now().UnixNano() % 1000000
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("qq_audio_%d_%06d%s", timestamp, randomID, ext))

	// 下载文件
	req, err := http.NewRequestWithContext(ctx, "GET", audioURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download audio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download audio failed: status=%d", resp.StatusCode)
	}

	// 保存文件
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read audio data: %w", err)
	}

	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return "", fmt.Errorf("write audio file: %w", err)
	}

	logger.Info("QQ audio downloaded", "path", tmpFile, "size", len(data), "url", audioURL)
	return tmpFile, nil
}
