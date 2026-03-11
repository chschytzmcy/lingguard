package channels

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/lingguard/pkg/logger"
)

// QClaw 环境 URL 配置
type qclawEnvURLs struct {
	JprxGateway        string
	WxLoginRedirectURI string
	BeaconURL          string
	QClawBaseURL       string
	WeChatWsURL        string
}

var qclawEnvs = map[string]qclawEnvURLs{
	"production": {
		JprxGateway:        "https://jprx.m.qq.com/",
		WxLoginRedirectURI: "https://security.guanjia.qq.com/login",
		BeaconURL:          "https://pcmgrmonitor.3g.qq.com/datareport",
		QClawBaseURL:       "https://mmgrcalltoken.3g.qq.com/aizone/v1",
		WeChatWsURL:        "wss://mmgrcalltoken.3g.qq.com/agentwss",
	},
	"test": {
		JprxGateway:        "https://jprx.sparta.html5.qq.com/",
		WxLoginRedirectURI: "https://security-test.guanjia.qq.com/login",
		BeaconURL:          "https://pcmgrmonitor.3g.qq.com/test/datareport",
		QClawBaseURL:       "https://jprx.sparta.html5.qq.com/aizone/v1",
		WeChatWsURL:        "wss://jprx.sparta.html5.qq.com/agentwss",
	},
}

// QClaw 微信 OAuth 配置
type qclawWxLoginConfig struct {
	AppID       string
	RedirectURI string
}

var qclawWxConfigs = map[string]qclawWxLoginConfig{
	"production": {
		AppID:       "wx9d11056dd75b7240",
		RedirectURI: "https://security.guanjia.qq.com/login",
	},
	"test": {
		AppID:       "wx3dd49afb7e2cf957",
		RedirectURI: "https://security-test.guanjia.qq.com/login",
	},
}

// QClaw API 端点
const (
	qclawEndpointWxLoginState        = "data/4050/forward" // 获取微信登录 state
	qclawEndpointWxLogin             = "data/4026/forward" // 微信登录
	qclawEndpointGetUserInfo         = "data/4027/forward" // 获取用户信息
	qclawEndpointWxLogout            = "data/4028/forward" // 微信登出
	qclawEndpointCreateAPIKey        = "data/4055/forward" // 创建 API Key
	qclawEndpointRefreshToken        = "data/4058/forward" // 刷新 Channel Token
	qclawEndpointCheckInviteCode     = "data/4056/forward" // 检查邀请码
	qclawEndpointSubmitInviteCode    = "data/4057/forward" // 提交邀请码
	qclawEndpointQueryDevice         = "data/4019/forward" // 查询设备信息
	qclawEndpointDisconnectDevice    = "data/4020/forward" // 断开设备连接
	qclawEndpointGenerateContactLink = "data/4018/forward" // 生成联系人链接
	qclawEndpointCheckUpdate         = "data/4066/forward" // 检查更新
)

// QClaw API 响应结构
type qclawAPIResponse struct {
	Ret     int             `json:"ret"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Common  *qclawCommon    `json:"common,omitempty"`
}

type qclawCommon struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// 微信登录相关结构
type qclawWxLoginStateData struct {
	State string `json:"state"`
}

type qclawWxLoginData struct {
	Token                string                `json:"token"`
	OpenClawChannelToken string                `json:"openclaw_channel_token"`
	UserInfo             *qclawWxLoginUserInfo `json:"user_info,omitempty"`
}

type qclawWxLoginUserInfo struct {
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	AvatarURL string `json:"avatar_url"`
	UserID    string `json:"user_id"`
}

type qclawUserInfo struct {
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	GUID     string `json:"guid"`
	UserID   string `json:"userId"`
	LoginKey string `json:"loginKey,omitempty"`
}

type qclawAPIKeyData struct {
	Key string `json:"key"`
}

// 设备查询相关结构
type qclawDeviceData struct {
	GUID      string `json:"guid"`
	Online    bool   `json:"online"`
	Connected bool   `json:"connected"`
	LastSeen  int64  `json:"last_seen,omitempty"`
}

// 联系人链接相关结构
type qclawContactLinkData struct {
	Link     string `json:"link"`
	ExpireAt int64  `json:"expire_at,omitempty"`
}

// 更新检查相关结构
type qclawUpdateData struct {
	HasUpdate   bool   `json:"has_update"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
}

// QClawClient QClaw HTTP API 客户端
type QClawClient struct {
	env        string
	envURLs    qclawEnvURLs
	wxConfig   qclawWxLoginConfig
	webVersion string
	guid       string

	// 认证信息
	jwtToken string
	userInfo *qclawUserInfo
	tokenMu  sync.RWMutex

	httpClient *http.Client
}

// NewQClawClient 创建 QClaw 客户端
func NewQClawClient(env, guid, webVersion string) *QClawClient {
	if env == "" {
		env = "production"
	}
	if webVersion == "" {
		webVersion = "1.4.0"
	}

	return &QClawClient{
		env:        env,
		envURLs:    qclawEnvs[env],
		wxConfig:   qclawWxConfigs[env],
		webVersion: webVersion,
		guid:       guid,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// SetJWTToken 设置 JWT Token
func (c *QClawClient) SetJWTToken(token string) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	c.jwtToken = token
}

// GetJWTToken 获取 JWT Token
func (c *QClawClient) GetJWTToken() string {
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.jwtToken
}

// SetUserInfo 设置用户信息
func (c *QClawClient) SetUserInfo(info *qclawUserInfo) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	c.userInfo = info
}

// GetUserInfo 获取用户信息
func (c *QClawClient) GetUserInfo() *qclawUserInfo {
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.userInfo
}

// BuildWxLoginURL 构建微信登录 URL
func (c *QClawClient) BuildWxLoginURL(state string) string {
	params := url.Values{}
	params.Set("appid", c.wxConfig.AppID)
	params.Set("redirect_uri", c.wxConfig.RedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "snsapi_login")
	params.Set("state", state)
	return "https://open.weixin.qq.com/connect/qrconnect?" + params.Encode()
}

// GetWxLoginState 获取微信登录 state
func (c *QClawClient) GetWxLoginState() (string, error) {
	reqBody := map[string]interface{}{
		"guid":        c.guid,
		"web_version": c.webVersion,
		"web_env":     "release",
	}

	resp, err := c.request(qclawEndpointWxLoginState, reqBody)
	if err != nil {
		return "", fmt.Errorf("get wx login state failed: %w", err)
	}

	var data qclawWxLoginStateData
	if err := json.Unmarshal(resp, &data); err != nil {
		return "", fmt.Errorf("unmarshal wx login state failed: %w", err)
	}

	return data.State, nil
}

// WxLogin 微信登录
func (c *QClawClient) WxLogin(code, state string) (*qclawWxLoginData, error) {
	reqBody := map[string]interface{}{
		"guid":        c.guid,
		"code":        code,
		"state":       state,
		"web_version": c.webVersion,
		"web_env":     "release",
	}

	resp, err := c.request(qclawEndpointWxLogin, reqBody)
	if err != nil {
		return nil, fmt.Errorf("wx login failed: %w", err)
	}

	var data qclawWxLoginData
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("unmarshal wx login data failed: %w", err)
	}

	// 保存 JWT Token
	c.SetJWTToken(data.Token)

	// 保存用户信息
	if data.UserInfo != nil {
		c.SetUserInfo(&qclawUserInfo{
			Nickname: data.UserInfo.Nickname,
			Avatar:   data.UserInfo.Avatar,
			UserID:   data.UserInfo.UserID,
			GUID:     c.guid,
		})
	}

	return &data, nil
}

// CreateAPIKey 创建 API Key
func (c *QClawClient) CreateAPIKey() (string, error) {
	reqBody := map[string]interface{}{
		"web_version": c.webVersion,
		"web_env":     "release",
	}

	resp, err := c.request(qclawEndpointCreateAPIKey, reqBody)
	if err != nil {
		return "", fmt.Errorf("create api key failed: %w", err)
	}

	var data qclawAPIKeyData
	if err := json.Unmarshal(resp, &data); err != nil {
		return "", fmt.Errorf("unmarshal api key data failed: %w", err)
	}

	return data.Key, nil
}

// RefreshChannelToken 刷新 Channel Token
func (c *QClawClient) RefreshChannelToken() (string, error) {
	reqBody := map[string]interface{}{
		"web_version": c.webVersion,
		"web_env":     "release",
	}

	resp, err := c.request(qclawEndpointRefreshToken, reqBody)
	if err != nil {
		return "", fmt.Errorf("refresh channel token failed: %w", err)
	}

	// 直接返回 token 字符串
	var token string
	if err := json.Unmarshal(resp, &token); err != nil {
		return "", fmt.Errorf("unmarshal channel token failed: %w", err)
	}

	return token, nil
}

// QueryDeviceByGuid 查询设备信息
func (c *QClawClient) QueryDeviceByGuid(guid string) (*qclawDeviceData, error) {
	if guid == "" {
		guid = c.guid
	}
	reqBody := map[string]interface{}{
		"guid":        guid,
		"web_version": c.webVersion,
		"web_env":     "release",
	}

	resp, err := c.request(qclawEndpointQueryDevice, reqBody)
	if err != nil {
		return nil, fmt.Errorf("query device failed: %w", err)
	}

	var data qclawDeviceData
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("unmarshal device data failed: %w", err)
	}

	return &data, nil
}

// DisconnectDevice 断开设备连接
func (c *QClawClient) DisconnectDevice(guid string) error {
	if guid == "" {
		guid = c.guid
	}
	reqBody := map[string]interface{}{
		"guid":        guid,
		"web_version": c.webVersion,
		"web_env":     "release",
	}

	_, err := c.request(qclawEndpointDisconnectDevice, reqBody)
	if err != nil {
		return fmt.Errorf("disconnect device failed: %w", err)
	}

	return nil
}

// GenerateContactLink 生成联系人链接
func (c *QClawClient) GenerateContactLink() (*qclawContactLinkData, error) {
	reqBody := map[string]interface{}{
		"guid":        c.guid,
		"web_version": c.webVersion,
		"web_env":     "release",
	}

	resp, err := c.request(qclawEndpointGenerateContactLink, reqBody)
	if err != nil {
		return nil, fmt.Errorf("generate contact link failed: %w", err)
	}

	var data qclawContactLinkData
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("unmarshal contact link data failed: %w", err)
	}

	return &data, nil
}

// CheckUpdate 检查更新
func (c *QClawClient) CheckUpdate() (*qclawUpdateData, error) {
	reqBody := map[string]interface{}{
		"web_version": c.webVersion,
		"web_env":     "release",
	}

	resp, err := c.request(qclawEndpointCheckUpdate, reqBody)
	if err != nil {
		return nil, fmt.Errorf("check update failed: %w", err)
	}

	var data qclawUpdateData
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("unmarshal update data failed: %w", err)
	}

	return &data, nil
}

// Logout 登出
func (c *QClawClient) Logout() error {
	reqBody := map[string]interface{}{
		"guid":        c.guid,
		"web_version": c.webVersion,
		"web_env":     "release",
	}

	_, err := c.request(qclawEndpointWxLogout, reqBody)
	if err != nil {
		return fmt.Errorf("logout failed: %w", err)
	}

	// 清除认证状态
	c.SetJWTToken("")
	c.SetUserInfo(nil)

	return nil
}

// GetUserInfoFromAPI 从 API 获取用户信息
func (c *QClawClient) GetUserInfoFromAPI() (*qclawUserInfo, error) {
	reqBody := map[string]interface{}{
		"guid":        c.guid,
		"web_version": c.webVersion,
		"web_env":     "release",
	}

	resp, err := c.request(qclawEndpointGetUserInfo, reqBody)
	if err != nil {
		return nil, fmt.Errorf("get user info failed: %w", err)
	}

	var data qclawUserInfo
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("unmarshal user info failed: %w", err)
	}

	// 更新缓存的用户信息
	c.SetUserInfo(&data)

	return &data, nil
}

// request 发送 HTTP 请求
func (c *QClawClient) request(endpoint string, body map[string]interface{}) (json.RawMessage, error) {
	reqURL := c.envURLs.JprxGateway + endpoint

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request body failed: %w", err)
	}

	req, err := http.NewRequest("POST", reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Version", "1")
	req.Header.Set("X-Guid", c.guid)
	req.Header.Set("X-Session", "")

	// 设置认证信息
	c.tokenMu.RLock()
	loginKey := "m83qdao0AmE5" // 默认 loginKey
	if c.userInfo != nil && c.userInfo.LoginKey != "" {
		loginKey = c.userInfo.LoginKey
	}
	req.Header.Set("X-Token", loginKey)

	if c.jwtToken != "" {
		req.Header.Set("X-OpenClaw-Token", c.jwtToken)
	}
	if c.userInfo != nil && c.userInfo.UserID != "" {
		req.Header.Set("X-Account", c.userInfo.UserID)
	}
	c.tokenMu.RUnlock()

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	// 检查 Token 更新
	if newToken := resp.Header.Get("X-New-Token"); newToken != "" {
		logger.Info("QClaw JWT token renewed")
		c.SetJWTToken(newToken)
	}

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %w", err)
	}

	// 解析响应
	var apiResp qclawAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	// 检查会话过期
	if apiResp.Common != nil && apiResp.Common.Code == 21004 {
		logger.Warn("QClaw session expired, clearing auth state")
		c.SetJWTToken("")
		c.SetUserInfo(nil)
		return nil, fmt.Errorf("session expired")
	}

	// 检查响应状态
	if apiResp.Ret != 0 || (apiResp.Common != nil && apiResp.Common.Code != 0) {
		return nil, fmt.Errorf("api error: ret=%d, message=%s", apiResp.Ret, apiResp.Message)
	}

	// 提取数据（处理腾讯的嵌套信封）
	return c.unwrapData(apiResp.Data)
}

// unwrapData 解包腾讯的嵌套数据结构
func (c *QClawClient) unwrapData(data json.RawMessage) (json.RawMessage, error) {
	// 尝试解析为 {"resp": {"data": ...}} 结构
	var respWrapper struct {
		Resp struct {
			Data json.RawMessage `json:"data"`
		} `json:"resp"`
	}
	if err := json.Unmarshal(data, &respWrapper); err == nil && len(respWrapper.Resp.Data) > 0 {
		return respWrapper.Resp.Data, nil
	}

	// 尝试解析为 {"data": ...} 结构
	var dataWrapper struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &dataWrapper); err == nil && len(dataWrapper.Data) > 0 {
		return dataWrapper.Data, nil
	}

	// 直接返回原始数据
	return data, nil
}
