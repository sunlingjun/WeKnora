package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/gin-gonic/gin"
)

func TestNormalizeRequestPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"/api/v1/open/knowledge/retrieve/", "/api/v1/open/knowledge/retrieve"},
		{"/api/v1/open/knowledge/retrieve", "/api/v1/open/knowledge/retrieve"},
		{"  /health/  ", "/health"},
		{"/", "/"},
	}
	for _, tc := range cases {
		if got := normalizeRequestPath(tc.in); got != tc.want {
			t.Fatalf("normalizeRequestPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsNoAuthAPI_OpenRetrievePaths(t *testing.T) {
	t.Parallel()
	if !isNoAuthAPI("/api/v1/open/knowledge/retrieve", "POST") {
		t.Fatal("exact POST path should be no-auth")
	}
	if !isNoAuthAPI("/api/v1/open/knowledge/retrieve/", "POST") {
		t.Fatal("trailing slash POST path should be no-auth (fixes missing authentication)")
	}
	if !isNoAuthAPI("/api/v1/open/knowledge/retrieve/", "post") {
		t.Fatal("method should be case-insensitive")
	}
	if isNoAuthAPI("/api/v1/open/knowledge/retrieve", "GET") {
		t.Fatal("GET must not be no-auth (only POST is registered for open retrieve)")
	}
	if isNoAuthAPI("/api/v1/open/knowledge/retrieve-other", "POST") {
		t.Fatal("wrong path must not match")
	}
}

// TestOpenRetrieve_Post_AuthMiddlewareBypassesWithoutCredentials 复现生产路由顺序：
// 全局 Auth 应先放行 POST /api/v1/open/knowledge/retrieve，再由 OpenRetrieveApiKey 校验。
// 若 noAuthAPI 未包含该路径或方法不匹配，会得到 401 missing authentication（与线上现象一致）。
func TestOpenRetrieve_Post_AuthMiddlewareBypassesWithoutCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := "secret-open-retrieve-key-32chars!!"
	cfg := &config.Config{
		OpenRetrieve: &config.OpenRetrieveConfig{
			Enabled: true,
			APIKey:  key,
		},
	}
	called := false
	r := gin.New()
	r.Use(Auth(nil, nil, nil, nil, nil, nil))
	r.Use(OpenRetrieveApiKey(cfg))
	r.POST("/api/v1/open/knowledge/retrieve", func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/open/knowledge/retrieve", strings.NewReader(`{}`))
	req.Header.Set(headerOpenRetrieveAPIKey, key)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s (expect Auth no-op then OpenKey OK; if 401 here, check noAuthAPI+normalizeRequestPath in auth.go)", rw.Code, rw.Body.String())
	}
	if !called {
		t.Fatal("handler not reached")
	}
}

// TestOpenRetrieve_Post_MissingOpenKey_ReturnsOpenRetrieveUnauthorized 区分全局 Auth 401 与开放 Key 401。
func TestOpenRetrieve_Post_MissingOpenKey_ReturnsOpenRetrieveUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		OpenRetrieve: &config.OpenRetrieveConfig{
			Enabled: true,
			APIKey:  "secret-open-retrieve-key-32chars!!",
		},
	}
	r := gin.New()
	r.Use(Auth(nil, nil, nil, nil, nil, nil))
	r.Use(OpenRetrieveApiKey(cfg))
	r.POST("/api/v1/open/knowledge/retrieve", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/open/knowledge/retrieve", strings.NewReader(`{}`))
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), "OPEN_RETRIEVE_UNAUTHORIZED") {
		t.Fatalf("expected OPEN_RETRIEVE_UNAUTHORIZED JSON, got %q", rw.Body.String())
	}
}

func TestOpenRetrieveApiKey_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		OpenRetrieve: &config.OpenRetrieveConfig{Enabled: false},
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/open/knowledge/retrieve", strings.NewReader(`{}`))
	OpenRetrieveApiKey(cfg)(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestOpenRetrieveApiKey_NoKeysConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		OpenRetrieve: &config.OpenRetrieveConfig{Enabled: true},
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/open/knowledge/retrieve", strings.NewReader(`{}`))
	OpenRetrieveApiKey(cfg)(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestOpenRetrieveApiKey_MissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		OpenRetrieve: &config.OpenRetrieveConfig{
			Enabled: true,
			APIKey:  "secret-open-retrieve-key-32chars!!",
		},
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/open/knowledge/retrieve", strings.NewReader(`{}`))
	OpenRetrieveApiKey(cfg)(c)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestOpenRetrieveApiKey_ValidKeyCallsNext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := "secret-open-retrieve-key-32chars!!"
	cfg := &config.Config{
		OpenRetrieve: &config.OpenRetrieveConfig{
			Enabled: true,
			APIKey:  key,
		},
	}
	called := false
	r := gin.New()
	r.Use(OpenRetrieveApiKey(cfg))
	r.POST("/api/v1/open/knowledge/retrieve", func(c *gin.Context) { called = true; c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, "/api/v1/open/knowledge/retrieve", strings.NewReader(`{}`))
	req.Header.Set(headerOpenRetrieveAPIKey, key)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rw.Code, rw.Body.String())
	}
	if !called {
		t.Fatal("handler not invoked")
	}
}
