package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// CASAuthHandler CAS 认证 Handler
type CASAuthHandler struct {
	casAuthService interfaces.CASAuthService
	userService    interfaces.UserService
	tenantService  interfaces.TenantService
	config         *config.Config
}

// NewCASAuthHandler 创建 CAS 认证 Handler
func NewCASAuthHandler(
	casAuthService interfaces.CASAuthService,
	userService interfaces.UserService,
	tenantService interfaces.TenantService,
	cfg *config.Config,
) *CASAuthHandler {
	return &CASAuthHandler{
		casAuthService: casAuthService,
		userService:    userService,
		tenantService:  tenantService,
		config:         cfg,
	}
}

// ValidateCASSession 验证 CAS 会话
// @Summary      验证 CAS 会话
// @Description  通过 Cookie 中的 _cas_sid 和 _cas_uid 验证 CAS 登录状态
// @Tags         CAS认证
// @Accept       json
// @Produce      json
// @Success      200      {object}  map[string]interface{}  "验证成功，返回用户和租户信息"
// @Failure      401      {object}  errors.AppError  "未登录或会话过期"
// @Router       /api/v1/cas/validate [get]
func (h *CASAuthHandler) ValidateCASSession(c *gin.Context) {
	ctx := c.Request.Context()

	// 1. 从配置获取当前环境的 Cookie 名称
	if h.config == nil || h.config.CAS == nil {
		logger.Errorf(ctx, "CAS config is not initialized")
		c.Error(errors.NewInternalServerError("CAS 配置未初始化"))
		return
	}
	casConfig := h.config.CAS.GetCurrentConfig()
	if casConfig == nil {
		logger.Errorf(ctx, "CAS environment config is not available")
		c.Error(errors.NewInternalServerError("CAS 环境配置不可用"))
		return
	}
	cookieSID := casConfig.CookieSID
	cookieUID := casConfig.CookieUID

	// 2. 从 Cookie 获取 CAS 会话信息（根据环境读取对应的 Cookie）
	casSid, err := c.Cookie(cookieSID)
	if err != nil {
		casSid = ""
	}
	casUid, err := c.Cookie(cookieUID)
	if err != nil {
		casUid = ""
	}

	if casSid == "" || casUid == "" {
		logger.Warn(ctx, "Missing CAS session cookies")
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":      10011,
			"exception": "please to login",
			"msg":       "未获取到登录信息",
		})
		return
	}

	// 3. 获取 Referer 头（CAS API 需要此头进行校验）
	referer := c.GetHeader("Referer")
	if referer == "" {
		// 如果没有 Referer，尝试从 Origin 构建
		origin := c.GetHeader("Origin")
		if origin != "" {
			referer = origin + "/"
		} else {
			// 使用配置中的登录页面地址作为默认 Referer
			referer = fmt.Sprintf("https://%s/", casConfig.LoginHost)
		}
	}
	logger.Debugf(ctx, "Using Referer: %s", referer)

	// 4. 验证 CAS 会话（调用 CAS 接口）
	casUserInfo, err := h.casAuthService.ValidateCASSession(ctx, casSid, casUid, referer)
	if err != nil {
		logger.Errorf(ctx, "Failed to validate CAS session: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":      10011,
			"exception": "please to login",
			"msg":       "CAS 会话验证失败",
		})
		return
	}

	// 4. 自动绑定或创建用户（包含用户存在性检查）
	// AutoBindUser 内部逻辑：
	// - 按优先级查找用户：cas_user_id → email
	// - 如果用户不存在，创建新用户（处理用户名/邮箱冲突）
	// - 如果用户存在，更新 CAS 相关字段
	// - 注意：登录名（cas_login_name）仅用于存储，不作为查找条件
	user, err := h.casAuthService.AutoBindUser(ctx, casUserInfo)
	if err != nil {
		logger.Errorf(ctx, "Failed to bind user: %v", err)
		c.Error(errors.NewInternalServerError("用户绑定失败"))
		return
	}

	// 5. 自动绑定或创建租户（包含租户存在性检查）
	// AutoBindTenant 内部逻辑：
	// - 检查用户是否已有租户（user.TenantID > 0）
	// - 如果用户已有租户，验证租户是否存在，存在则返回
	// - 如果用户没有租户或租户不存在，创建新租户并绑定用户
	tenant, err := h.casAuthService.AutoBindTenant(ctx, casUserInfo, user)
	if err != nil {
		logger.Errorf(ctx, "Failed to bind tenant: %v", err)
		c.Error(errors.NewInternalServerError("租户绑定失败"))
		return
	}

	// 6. 生成 JWT Token（用于后续 API 调用）
	// 注意：不需要创建 CAS 会话记录，CAS 会话信息已在 Cookie 中
	// JWT Token 存储在 auth_tokens 表中，用于管理用户认证状态
	token, refreshToken, err := h.userService.GenerateTokens(ctx, user)
	if err != nil {
		logger.Errorf(ctx, "Failed to generate tokens: %v", err)
		c.Error(errors.NewInternalServerError("Token 生成失败"))
		return
	}

	// 7. 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"user":          user.ToUserInfo(),
			"tenant":        tenant,
			"token":         token,
			"refresh_token": refreshToken,
		},
		"msg": "",
	})
}
