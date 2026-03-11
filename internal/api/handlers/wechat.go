// Package handlers 提供 HTTP API 处理器
package handlers

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/lingguard/internal/channels"
	"github.com/lingguard/pkg/logger"
)

// WeChatHandler 微信渠道 HTTP 处理器
type WeChatHandler struct {
	channel *channels.WeChatChannel
}

// NewWeChatHandler 创建微信渠道处理器
func NewWeChatHandler(channel *channels.WeChatChannel) *WeChatHandler {
	return &WeChatHandler{
		channel: channel,
	}
}

// RegisterRoutes 注册路由
func (h *WeChatHandler) RegisterRoutes(router *gin.RouterGroup) {
	wechat := router.Group("/wechat")
	{
		wechat.POST("/login/state", h.GetLoginState)
		wechat.POST("/login", h.Login)
		wechat.POST("/token/refresh", h.RefreshToken)
		wechat.GET("/status", h.GetStatus)
	}
}

// GetLoginState 获取微信登录 state
// @Summary 获取微信登录 state
// @Description 获取微信登录所需的 state 参数和二维码 URL
// @Tags wechat
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]interface{}
// @Router /v1/wechat/login/state [post]
func (h *WeChatHandler) GetLoginState(c *gin.Context) {
	qrURL, err := h.channel.GetLoginURL()
	if err != nil {
		logger.Error("Failed to get WeChat login URL", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "login_state_failed",
				"message": err.Error(),
			},
		})
		return
	}

	// 从 URL 中提取 state 参数
	state := extractStateFromURL(qrURL)

	c.JSON(http.StatusOK, gin.H{
		"state":  state,
		"qr_url": qrURL,
	})
}

// extractStateFromURL 从微信登录 URL 中提取 state 参数
func extractStateFromURL(qrURL string) string {
	parsedURL, err := url.Parse(qrURL)
	if err != nil {
		return ""
	}
	state := parsedURL.Query().Get("state")
	return state
}

// LoginRequest 登录请求
type LoginRequest struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state" binding:"required"`
}

// Login 微信登录
// @Summary 微信登录
// @Description 使用微信授权码完成登录
// @Tags wechat
// @Accept json
// @Produce json
// @Param request body LoginRequest true "登录参数"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /v1/wechat/login [post]
func (h *WeChatHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_request",
				"message": "code and state are required",
			},
		})
		return
	}

	loginResult, err := h.channel.Login(req.Code, req.State)
	if err != nil {
		logger.Error("WeChat login failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "login_failed",
				"message": err.Error(),
			},
		})
		return
	}

	response := gin.H{
		"success": true,
		"message": "login successful",
	}

	// 添加用户信息
	if loginResult.UserInfo != nil {
		response["user"] = gin.H{
			"nickname": loginResult.UserInfo.Nickname,
			"avatar":   loginResult.UserInfo.Avatar,
			"user_id":  loginResult.UserInfo.UserID,
		}
	}

	c.JSON(http.StatusOK, response)
}

// RefreshToken 刷新 Channel Token
// @Summary 刷新 Channel Token
// @Description 刷新微信渠道的 Channel Token
// @Tags wechat
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /v1/wechat/token/refresh [post]
func (h *WeChatHandler) RefreshToken(c *gin.Context) {
	err := h.channel.RefreshToken()
	if err != nil {
		logger.Error("Failed to refresh WeChat token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "refresh_failed",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "token refreshed",
	})
}

// GetStatus 获取渠道状态
// @Summary 获取渠道状态
// @Description 获取微信渠道的当前状态
// @Tags wechat
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /v1/wechat/status [get]
func (h *WeChatHandler) GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"running": h.channel.IsRunning(),
		"channel": "wechat",
	})
}
