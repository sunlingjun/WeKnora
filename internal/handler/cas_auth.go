package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/cascookie"
	"github.com/Tencent/WeKnora/internal/config"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
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
// @Description  通过 Cookie ticketCookie（ZNT）或环境 cookie_sid（_cas_sid/_cas_t_sid）换档并拉档案后换发 JWT；uid 非必须。
// @Tags         CAS认证
// @Accept       json
// @Produce      json
// @Success      200      {object}  map[string]interface{}  "验证成功，返回用户和租户信息"
// @Failure      401      {object}  errors.AppError  "未登录或会话过期"
// @Router       /api/v1/cas/validate [get]
func (h *CASAuthHandler) ValidateCASSession(c *gin.Context) {
	ctx := c.Request.Context()

	if h.config == nil || h.config.CAS == nil {
		logger.Errorf(ctx, "CAS config is not initialized")
		c.Error(apperrors.NewInternalServerError("CAS 配置未初始化"))
		return
	}
	if h.config.CAS.GetCurrentConfig() == nil {
		logger.Errorf(ctx, "CAS environment config is not available")
		c.Error(apperrors.NewInternalServerError("CAS 环境配置不可用"))
		return
	}

	ticketCookie, casSid, _ := cascookie.Read(c, h.config)
	casUserInfo, err := h.casAuthService.ResolveCASUserFromCookies(ctx, ticketCookie, casSid)
	if errors.Is(err, types.ErrCASCredentialsMissing) {
		logger.Warn(ctx, "Missing CAS ticketCookie and cookie_sid")
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":      10011,
			"exception": "please to login",
			"msg":       "未获取到登录信息",
		})
		return
	}
	if errors.Is(err, types.ErrCASTicketInvalid) {
		logger.Errorf(ctx, "Failed to resolve CAS ticket: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":      10011,
			"exception": "please to login",
			"msg":       "CAS 会话验证失败",
		})
		return
	}
	if err != nil {
		logger.Errorf(ctx, "CAS user center unavailable: %v", err)
		c.Error(apperrors.NewInternalServerError("CAS 用户中心不可用"))
		return
	}

	user, err := h.casAuthService.AutoBindUser(ctx, casUserInfo)
	if err != nil {
		logger.Errorf(ctx, "Failed to bind user: %v", err)
		c.Error(apperrors.NewInternalServerError("用户绑定失败"))
		return
	}

	// Ensure the user has a home / bindable workspace. The returned
	// tenant is NOT necessarily the session's active workspace — that
	// follows resolveLoginTenantID inside GenerateTokens (last_active).
	if _, err := h.casAuthService.AutoBindTenant(ctx, casUserInfo, user); err != nil {
		logger.Errorf(ctx, "Failed to bind tenant: %v", err)
		c.Error(apperrors.NewInternalServerError("租户绑定失败"))
		return
	}

	token, refreshToken, err := h.userService.GenerateTokens(ctx, user)
	if err != nil {
		logger.Errorf(ctx, "Failed to generate tokens: %v", err)
		c.Error(apperrors.NewInternalServerError("Token 生成失败"))
		return
	}

	// Align response.tenant with the JWT claim so the SPA can pin
	// X-Tenant-ID to the same workspace the token encodes.
	var activeTenant *types.Tenant
	if _, jwtTenantID, verr := h.userService.ValidateToken(ctx, token); verr == nil && jwtTenantID > 0 {
		if t, terr := h.tenantService.GetTenantByID(ctx, jwtTenantID); terr == nil {
			activeTenant = t
		} else {
			logger.Warnf(ctx, "CAS validate: failed to load active tenant %d: %v", jwtTenantID, terr)
		}
	} else if verr != nil {
		logger.Warnf(ctx, "CAS validate: failed to read tenant claim from new token: %v", verr)
	}

	memberships := h.userService.BuildLoginMemberships(ctx, user, activeTenant)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"user":          user.ToUserInfo(),
			"tenant":        activeTenant,
			"active_tenant": activeTenant,
			"memberships":   memberships,
			"token":         token,
			"refresh_token": refreshToken,
		},
		"msg": "",
	})
}
