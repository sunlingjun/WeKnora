package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

func TestBuildInviteCASURL(t *testing.T) {
	cfg := &config.Config{FrontendBaseURL: "https://kb.example.com"}
	got := buildInviteCASURL(cfg, "tok-abc")
	want := "https://kb.example.com/join-cas?token=tok-abc"
	if got != want {
		t.Fatalf("buildInviteCASURL=%q, want %q", got, want)
	}
	if buildInviteCASURL(cfg, "") != "" {
		t.Fatalf("empty token should yield empty URL")
	}
}

func TestBuildInviteRegisterURL_Unchanged(t *testing.T) {
	cfg := &config.Config{FrontendBaseURL: "https://kb.example.com"}
	got := buildInviteRegisterURL(cfg, "tok-abc")
	want := "https://kb.example.com/register?token=tok-abc"
	if got != want {
		t.Fatalf("buildInviteRegisterURL=%q, want %q", got, want)
	}
}

type inviteLinkInvitationService struct {
	interfaces.TenantInvitationService
	lastMessage string
	lastRole    types.TenantRole
	lastTenant  uint64
	inv         *types.TenantInvitation
}

func (s *inviteLinkInvitationService) CreateShareLink(
	_ context.Context,
	tenantID uint64,
	role types.TenantRole,
	_ *string,
	message string,
) (*types.TenantInvitation, string, error) {
	s.lastTenant = tenantID
	s.lastRole = role
	s.lastMessage = message
	token := "plain-token-xyz"
	inv := &types.TenantInvitation{
		ID:       7,
		TenantID: tenantID,
		Role:     role,
		Status:   types.TenantInvitationStatusPending,
		Token:    token,
		Message:  message,
	}
	s.inv = inv
	return inv, token, nil
}

type inviteLinkUserService struct {
	interfaces.UserService
}

func TestCreateInviteLink_CASModePersistsMarkerAndJoinCASURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	invSvc := &inviteLinkInvitationService{}
	h := NewTenantInvitationHandler(
		invSvc,
		&inviteLinkUserService{},
		nil,
		&config.Config{FrontendBaseURL: "https://kb.example.com"},
	)
	r := gin.New()
	r.Use(errorCapture())
	r.POST("/tenants/:id/invite-links", h.CreateInviteLink)

	body := []byte(`{"role":"viewer","link_mode":"cas"}`)
	req := httptest.NewRequest(http.MethodPost, "/tenants/42/invite-links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if invSvc.lastMessage != inviteKindCASMarker {
		t.Fatalf("persisted message=%q, want %q", invSvc.lastMessage, inviteKindCASMarker)
	}
	if invSvc.lastTenant != 42 || invSvc.lastRole != types.TenantRoleViewer {
		t.Fatalf("tenant=%d role=%s", invSvc.lastTenant, invSvc.lastRole)
	}

	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			InviteURL string `json:"invite_url"`
			Message   string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !payload.Success {
		t.Fatalf("success=false body=%s", w.Body.String())
	}
	wantURL := "https://kb.example.com/join-cas?token=plain-token-xyz"
	if payload.Data.InviteURL != wantURL {
		t.Fatalf("invite_url=%q, want %q", payload.Data.InviteURL, wantURL)
	}
	if payload.Data.Message != inviteKindCASMarker {
		t.Fatalf("response message=%q, want marker", payload.Data.Message)
	}
}

func TestCreateInviteLink_DefaultRegisterURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	invSvc := &inviteLinkInvitationService{}
	h := NewTenantInvitationHandler(
		invSvc,
		&inviteLinkUserService{},
		nil,
		&config.Config{FrontendBaseURL: "https://kb.example.com"},
	)
	r := gin.New()
	r.Use(errorCapture())
	r.POST("/tenants/:id/invite-links", h.CreateInviteLink)

	body := []byte(`{"role":"viewer"}`)
	req := httptest.NewRequest(http.MethodPost, "/tenants/42/invite-links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if invSvc.lastMessage != "" {
		t.Fatalf("default mode should not set CAS marker, message=%q", invSvc.lastMessage)
	}

	var payload struct {
		Data struct {
			InviteURL string `json:"invite_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(payload.Data.InviteURL, "/register?token=") {
		t.Fatalf("invite_url=%q, want /register", payload.Data.InviteURL)
	}
	if strings.Contains(payload.Data.InviteURL, "/join-cas") {
		t.Fatalf("default invite_url must not use /join-cas: %q", payload.Data.InviteURL)
	}
}

func TestProjectInvitationWithLink_CASMarkerUsesJoinCAS(t *testing.T) {
	h := NewTenantInvitationHandler(
		nil, nil, nil,
		&config.Config{FrontendBaseURL: "https://kb.example.com"},
	)
	inv := &types.TenantInvitation{
		ID:       1,
		TenantID: 9,
		Role:     types.TenantRoleViewer,
		Status:   types.TenantInvitationStatusPending,
		Token:    "tok-1",
		Message:  inviteKindCASMarker,
	}
	resp := h.projectInvitationWithLink(inv, nil, nil)
	want := "https://kb.example.com/join-cas?token=tok-1"
	if resp.InviteURL != want {
		t.Fatalf("InviteURL=%q, want %q", resp.InviteURL, want)
	}
}
