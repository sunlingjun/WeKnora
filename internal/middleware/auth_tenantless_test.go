package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

func TestTenantOptionalAPISurface(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodGet, "/api/v1/auth/me", true},
		{http.MethodPut, "/api/v1/auth/me", true},
		{http.MethodPut, "/api/v1/auth/me/preferences", true},
		{http.MethodPost, "/api/v1/auth/switch-tenant", true},
		{http.MethodPost, "/api/v1/auth/join-by-invite-cas", true},
		{http.MethodPost, "/api/v1/tenants", true},
		{http.MethodGet, "/api/v1/me/invitations", true},
		{http.MethodPost, "/api/v1/me/invitations/12/accept", true},
		{http.MethodGet, "/api/v1/knowledge-bases", false},
		{http.MethodGet, "/api/v1/tenants", false},
	}
	for _, tt := range tests {
		if got := isTenantOptionalAPI(tt.path, tt.method); got != tt.want {
			t.Fatalf("isTenantOptionalAPI(%s %s) = %v, want %v", tt.method, tt.path, got, tt.want)
		}
	}
}

func TestResolveFirstMembershipTarget(t *testing.T) {
	members := newFakeMemberService()
	members.seedActive("tenantless-user", 42, types.TenantRoleViewer)
	tenants := &fakeTenantService{tenant: &types.Tenant{ID: 42}}

	got := resolveFirstMembershipTarget(
		context.Background(),
		&types.User{ID: "tenantless-user"},
		members,
		tenants,
	)
	if got != 42 {
		t.Fatalf("resolved tenant = %d, want 42", got)
	}
}

func TestAuthenticateJWTUser_DeletedJWTTenantFallsBackToTenantlessOnAuthMe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)

	user := &types.User{ID: "u1", TenantID: 0}
	tenants := &fakeTenantService{missing: map[uint64]bool{99: true}}
	members := newFakeMemberService()

	ok := authenticateJWTUser(c, tenants, members, cfgWithRBAC(true), user, 99)
	if !ok {
		t.Fatalf("authenticateJWTUser returned false, body=%s", w.Body.String())
	}
	if w.Code != 0 && w.Code != http.StatusOK {
		t.Fatalf("unexpected status %d body=%s", w.Code, w.Body.String())
	}
	if _, hasTenant := types.TenantIDFromContext(c.Request.Context()); hasTenant {
		t.Fatal("expected tenantless session after deleted JWT tenant")
	}
}

func TestAuthenticateJWTUser_StaleXTenantIDIgnoredOnAuthMe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	c.Request.Header.Set("X-Tenant-ID", "99")

	user := &types.User{ID: "u1", TenantID: 0}
	tenants := &fakeTenantService{missing: map[uint64]bool{99: true}}
	members := newFakeMemberService()

	ok := authenticateJWTUser(c, tenants, members, cfgWithRBAC(true), user, 0)
	if !ok {
		t.Fatalf("authenticateJWTUser returned false, body=%s", w.Body.String())
	}
	if w.Code == http.StatusForbidden || w.Code == http.StatusUnauthorized || w.Code == http.StatusBadRequest {
		t.Fatalf("stale X-Tenant-ID on /auth/me must not hard-fail, status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthenticateJWTUser_DeletedJWTTenantRecoversOtherMembership(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)

	user := &types.User{ID: "u1", TenantID: 0}
	keep := &types.Tenant{ID: 42, Name: "keep"}
	tenants := &fakeTenantService{
		missing: map[uint64]bool{99: true},
		tenants: map[uint64]*types.Tenant{42: keep},
	}
	members := newFakeMemberService()
	members.seedActive("u1", 42, types.TenantRoleOwner)

	ok := authenticateJWTUser(c, tenants, members, cfgWithRBAC(true), user, 99)
	if !ok {
		t.Fatalf("authenticateJWTUser returned false, body=%s", w.Body.String())
	}
	got, has := types.TenantIDFromContext(c.Request.Context())
	if !has || got != 42 {
		t.Fatalf("tenant context = (%d, %v), want 42", got, has)
	}
}
