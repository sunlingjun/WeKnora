package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

func testCASConfig() *config.Config {
	return &config.Config{
		CAS: &config.CASConfig{
			Environment: "test",
			Test: &config.CASEnvConfig{
				CookieSID: "_cas_t_sid",
				CookieUID: "_cas_t_uid",
			},
		},
	}
}

func requestWithCASCookies(uid string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/knowledge-bases", nil)
	if uid != "" {
		c.Request.AddCookie(&http.Cookie{Name: "_cas_t_uid", Value: uid})
		c.Request.AddCookie(&http.Cookie{Name: "_cas_t_sid", Value: "sid-1"})
	}
	return c
}

func TestJWTIdentityConflictsWithCASCookie(t *testing.T) {
	cfg := testCASConfig()
	user := &types.User{ID: "u1", CASUserID: "uid-a"}

	if jwtIdentityConflictsWithCASCookie(requestWithCASCookies("uid-a"), cfg, user) {
		t.Fatal("same CAS uid must not conflict")
	}
	if !jwtIdentityConflictsWithCASCookie(requestWithCASCookies("uid-b"), cfg, user) {
		t.Fatal("changed CAS uid must conflict with JWT identity")
	}
	if jwtIdentityConflictsWithCASCookie(requestWithCASCookies(""), cfg, user) {
		t.Fatal("missing CAS cookie must not reject a still-valid JWT")
	}
	if jwtIdentityConflictsWithCASCookie(requestWithCASCookies("uid-b"), cfg, &types.User{ID: "local"}) {
		t.Fatal("local users without cas_user_id must not conflict")
	}
	if jwtIdentityConflictsWithCASCookie(requestWithCASCookies("uid-b"), nil, user) {
		t.Fatal("missing CAS config must not conflict")
	}
}
