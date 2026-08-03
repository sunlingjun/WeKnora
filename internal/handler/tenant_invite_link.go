package handler

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// frontendBaseURLFor returns the externally-visible SPA origin used to
// compose absolute share-link URLs. Lookup order:
//  1. config.FrontendBaseURL (set in YAML or via ${FRONTEND_BASE_URL})
//  2. FRONTEND_BASE_URL env at request time (so an operator can roll
//     the value out without restarting the server)
//  3. empty — handler emits a host-relative URL ("/register?token=…")
//     and the SPA resolves it against the current origin.
//
// Trailing slashes are stripped so callers can append "/register?…"
// cleanly.
func frontendBaseURLFor(cfg *config.Config) string {
	candidate := ""
	if cfg != nil {
		candidate = strings.TrimSpace(cfg.FrontendBaseURL)
	}
	if candidate == "" {
		candidate = strings.TrimSpace(os.Getenv("FRONTEND_BASE_URL"))
	}
	return strings.TrimRight(candidate, "/")
}

// buildInviteRegisterURL composes the registration URL Owners hand
// to invitees. Plaintext token is URL-safe by construction (base64url)
// so no extra escaping is required.
func buildInviteRegisterURL(cfg *config.Config, plainToken string) string {
	if plainToken == "" {
		return ""
	}
	return frontendBaseURLFor(cfg) + "/register?token=" + plainToken
}

// inviteKindCASMarker is stored in TenantInvitation.Message when an
// Owner creates a share-link with link_mode=cas and no custom message.
// projectInvitationWithLink reads it so pending-list re-copy keeps
// /join-cas without a DB schema change.
const inviteKindCASMarker = "invite_kind:cas"

// buildInviteCASURL composes the enterprise CAS join URL Owners hand
// to invitees (SPA route /join-cas). Same token rules as register URLs.
func buildInviteCASURL(cfg *config.Config, plainToken string) string {
	if plainToken == "" {
		return ""
	}
	return frontendBaseURLFor(cfg) + "/join-cas?token=" + plainToken
}

// isCASInviteKind reports whether Message marks a CAS share-link so
// list/create projections can rebuild /join-cas invite URLs.
func isCASInviteKind(message string) bool {
	return strings.Contains(message, inviteKindCASMarker)
}

// normalizeInviteLinkMode maps create-body link_mode to a canonical
// value. Empty / unknown → "register" (local register flow).
func normalizeInviteLinkMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "cas":
		return "cas"
	default:
		return "register"
	}
}

// createInviteLinkRequest is the body for POST /tenants/:id/invite-links.
// Only role + optional message — share-link rows have no specific
// invitee, so the Owner just picks "what role does the holder get".
// LinkMode selects the landing path: ""|"register" → /register,
// "cas" → /join-cas (enterprise CAS join).
type createInviteLinkRequest struct {
	Role     types.TenantRole `json:"role" binding:"required"`
	Message  string           `json:"message"`
	LinkMode string           `json:"link_mode"` // ""|"register"|"cas"; default register
}

// CreateInviteLink godoc
// @Summary      生成共享邀请链接
// @Description  生成一条多次使用的共享邀请链接：谁拿到链接谁就能注册并加入当前空间。
// @Description  链接持续有效，直到过期或被撤销。
// @Tags         空间邀请
// @Accept       json
// @Produce      json
// @Param        id       path  string                   true  "空间 ID"
// @Param        request  body  createInviteLinkRequest  true  "共享链接配置"
// @Success      201  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /tenants/{id}/invite-links [post]
func (h *TenantInvitationHandler) CreateInviteLink(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}
	var req createInviteLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("invalid request body").WithDetails(err.Error()))
		return
	}
	if !req.Role.IsValid() {
		c.Error(apperrors.NewValidationError("role must be one of owner/admin/contributor/viewer"))
		return
	}

	message := req.Message
	if normalizeInviteLinkMode(req.LinkMode) == "cas" && strings.TrimSpace(message) == "" {
		// Persist a marker so pending-list re-copy also emits /join-cas.
		message = inviteKindCASMarker
	}

	caller, _ := types.UserIDFromContext(ctx)
	var invitedBy *string
	if caller != "" && !types.IsSyntheticUserID(caller) {
		invitedBy = &caller
	}

	inv, _, err := h.invitationService.CreateShareLink(ctx, tenantID, req.Role, invitedBy, message)
	if err != nil {
		if errors.Is(err, service.ErrAPIKeyCannotAssignOwner) {
			c.Error(apperrors.NewForbiddenError(err.Error()))
			return
		}
		logger.Errorf(ctx, "CreateShareLink failed: tenant=%d err=%v", tenantID, err)
		c.Error(apperrors.NewInternalServerError("failed to create share link").WithDetails(err.Error()))
		return
	}
	usersByID := map[string]*types.User{}
	if invitedBy != nil {
		if iu, lookupErr := h.userService.GetUserByID(ctx, *invitedBy); lookupErr == nil && iu != nil {
			usersByID[iu.ID] = iu
		}
	}
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    h.projectInvitationWithLink(inv, usersByID, nil),
	})
}
