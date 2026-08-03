package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// joinByInviteCASUserService stubs GetCurrentUser + SwitchTenant for the
// join-by-invite-cas handler. Embedding interfaces.UserService keeps the
// surface small while satisfying the interface.
type joinByInviteCASUserService struct {
	interfaces.UserService
	user           *types.User
	getUserErr     error
	switchTenantID uint64
	switchCalls    int
	acceptBefore   *bool // points at invitation stub's acceptCalled flag
}

func (s *joinByInviteCASUserService) GetCurrentUser(context.Context) (*types.User, error) {
	if s.getUserErr != nil {
		return nil, s.getUserErr
	}
	return s.user, nil
}

func (s *joinByInviteCASUserService) SwitchTenant(
	_ context.Context,
	_ *types.User,
	targetTenantID uint64,
	_ string,
) (*types.LoginResponse, error) {
	s.switchCalls++
	s.switchTenantID = targetTenantID
	if s.acceptBefore != nil && !*s.acceptBefore {
		return nil, errors.New("AcceptByToken must run before SwitchTenant")
	}
	return &types.LoginResponse{
		Success: true,
		Message: "switched",
		User:    s.user,
		ActiveTenant: &types.Tenant{
			ID:   targetTenantID,
			Name: "Invite Target",
		},
		Memberships: []types.Membership{{
			TenantID:   targetTenantID,
			TenantName: "Invite Target",
			Role:       types.TenantRoleViewer,
		}},
		Token:        "new-access",
		RefreshToken: "new-refresh",
	}, nil
}

type joinByInviteCASInvitationService struct {
	interfaces.TenantInvitationService
	invite      *types.TenantInvitation
	lookupErr   error
	acceptErr   error
	acceptCalls int
	acceptUser  string
	acceptCalled bool
}

func (s *joinByInviteCASInvitationService) LookupByToken(_ context.Context, _ string) (*types.TenantInvitation, error) {
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	if s.invite == nil {
		return nil, errors.New("not found")
	}
	return s.invite, nil
}

func (s *joinByInviteCASInvitationService) AcceptByToken(_ context.Context, _ string, userID string) (*types.TenantMember, error) {
	s.acceptCalls++
	s.acceptUser = userID
	s.acceptCalled = true
	if s.acceptErr != nil {
		return nil, s.acceptErr
	}
	inv := s.invite
	if inv == nil {
		return nil, errors.New("not found")
	}
	return &types.TenantMember{TenantID: inv.TenantID, Role: inv.Role, UserID: userID}, nil
}

func newJoinByInviteCASRouter(h *AuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(errorCapture())
	r.POST("/auth/join-by-invite-cas", h.JoinByInviteCAS)
	return r
}

func doJoinByInviteCAS(t *testing.T, r *gin.Engine, body any) *httptest.ResponseRecorder {
	t.Helper()
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/join-by-invite-cas", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestJoinByInviteCAS_MissingAuthReturns401(t *testing.T) {
	users := &joinByInviteCASUserService{getUserErr: errors.New("not authenticated")}
	h := &AuthHandler{
		userService:   users,
		invitationSvc: &joinByInviteCASInvitationService{},
	}
	w := doJoinByInviteCAS(t, newJoinByInviteCASRouter(h), map[string]string{"token": "any"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s, want 401", w.Code, w.Body.String())
	}
}

func TestJoinByInviteCAS_BadTokenReturns410(t *testing.T) {
	users := &joinByInviteCASUserService{
		user: &types.User{ID: "u1", Username: "alice", IsActive: true},
	}
	inv := &joinByInviteCASInvitationService{lookupErr: errors.New("gone")}
	h := &AuthHandler{userService: users, invitationSvc: inv}
	w := doJoinByInviteCAS(t, newJoinByInviteCASRouter(h), map[string]string{"token": "bad"})
	if w.Code != http.StatusGone {
		t.Fatalf("status=%d body=%s, want 410", w.Code, w.Body.String())
	}
	if users.switchCalls != 0 {
		t.Fatalf("SwitchTenant calls=%d, want 0 on bad token", users.switchCalls)
	}
}

func TestJoinByInviteCAS_HappyPathReturns200WithInviteTenant(t *testing.T) {
	const inviteTenantID uint64 = 99
	invSvc := &joinByInviteCASInvitationService{
		invite: &types.TenantInvitation{
			TenantID: inviteTenantID,
			Role:     types.TenantRoleContributor,
		},
	}
	users := &joinByInviteCASUserService{
		user:         &types.User{ID: "u1", Username: "alice", IsActive: true},
		acceptBefore: &invSvc.acceptCalled,
	}
	h := &AuthHandler{userService: users, invitationSvc: invSvc}
	w := doJoinByInviteCAS(t, newJoinByInviteCASRouter(h), map[string]string{
		"token": "valid-invite",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	if invSvc.acceptCalls != 1 || invSvc.acceptUser != "u1" {
		t.Fatalf("accept calls=%d user=%q, want 1 / u1", invSvc.acceptCalls, invSvc.acceptUser)
	}
	if users.switchCalls != 1 || users.switchTenantID != inviteTenantID {
		t.Fatalf("switch calls=%d tenant=%d, want 1 / %d", users.switchCalls, users.switchTenantID, inviteTenantID)
	}

	var resp struct {
		Success      bool `json:"success"`
		ActiveTenant *struct {
			ID uint64 `json:"id"`
		} `json:"active_tenant"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if !resp.Success {
		t.Fatalf("success=false body=%s", w.Body.String())
	}
	if resp.ActiveTenant == nil || resp.ActiveTenant.ID != inviteTenantID {
		t.Fatalf("active_tenant=%v, want id=%d", resp.ActiveTenant, inviteTenantID)
	}
}
