package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/handler/dto"
	"github.com/Tencent/WeKnora/internal/logger"
)

// joinByInviteCASRequest is the body for POST /auth/join-by-invite-cas.
// Token is the share-link plaintext; refresh_token is optional and forwarded
// to SwitchTenant so the previous session refresh token can be revoked.
type joinByInviteCASRequest struct {
	Token        string `json:"token" binding:"required"`
	RefreshToken string `json:"refresh_token"`
}

// JoinByInviteCAS godoc
// @Summary      Join workspace via invite token (CAS / already logged-in)
// @Description  Authenticated user accepts a multi-use share-link invite and
// @Description  switches into the invited tenant. Used after CAS SSO on
// @Description  /join-cas. AcceptByToken is idempotent for existing members.
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        request  body      joinByInviteCASRequest  true  "invite token"
// @Success      200      {object}  types.LoginResponse
// @Failure      400      {object}  apperrors.AppError  "invalid request"
// @Failure      401      {object}  apperrors.AppError  "not authenticated"
// @Failure      403      {object}  apperrors.AppError  "workspace switch failed"
// @Failure      410      {object}  apperrors.AppError  "invite invalid or revoked"
// @Security     Bearer
// @Router       /auth/join-by-invite-cas [post]
func (h *AuthHandler) JoinByInviteCAS(c *gin.Context) {
	ctx := c.Request.Context()

	if h.invitationSvc == nil {
		c.Error(apperrors.NewInternalServerError("invitation service unavailable"))
		return
	}

	var req joinByInviteCASRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("token is required").WithDetails(err.Error()))
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" {
		c.Error(apperrors.NewValidationError("token is required"))
		return
	}

	user, err := h.userService.GetCurrentUser(ctx)
	if err != nil || user == nil {
		c.Error(apperrors.NewUnauthorizedError("not authenticated"))
		return
	}

	inv, err := h.invitationSvc.LookupByToken(ctx, req.Token)
	if err != nil {
		// Collapse unknown / expired / revoked into 410 (same as lookup /
		// register-by-invite) to avoid leaking token slot state.
		c.Error(&apperrors.AppError{
			Code:     apperrors.ErrNotFound,
			Message:  "invitation link is invalid or has been revoked",
			HTTPCode: http.StatusGone,
		})
		return
	}

	// Membership must exist before SwitchTenant; AcceptByToken is idempotent
	// when the user is already an active member of the invited tenant.
	if _, err := h.invitationSvc.AcceptByToken(ctx, req.Token, user.ID); err != nil {
		logger.Errorf(ctx, "join-by-invite-cas: accept failed user=%s: %v", user.ID, err)
		c.Error(&apperrors.AppError{
			Code:     apperrors.ErrNotFound,
			Message:  "invitation link is invalid or has been revoked",
			HTTPCode: http.StatusGone,
		})
		return
	}

	resp, err := h.userService.SwitchTenant(ctx, user, inv.TenantID, req.RefreshToken)
	if err != nil {
		logger.Errorf(ctx, "join-by-invite-cas: SwitchTenant failed user=%s target=%d: %v",
			user.ID, inv.TenantID, err)
		c.Error(apperrors.NewForbiddenError("workspace switch failed").WithDetails(err.Error()))
		return
	}

	c.JSON(http.StatusOK, dto.NewAuthLoginResponse(resp))
}
