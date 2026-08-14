package router

import (
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// Official 0.7.2 split router.go; cas-import must be re-registered on the
// tenant members group or the NXIN 成员管理「农信用户导入」button 404s.
func TestTenantCASImportRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	g := &rbacGuards{}
	RegisterTenantRoutes(engine.Group("/api/v1"), &handler.TenantHandler{}, &handler.TenantMemberHandler{}, nil, nil, g)

	want := map[string]bool{
		http.MethodPost + " /api/v1/tenants/:id/members/cas-import/preview": false,
		http.MethodPost + " /api/v1/tenants/:id/members/cas-import":         false,
	}
	for _, route := range engine.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for path, found := range want {
		if !found {
			t.Errorf("missing route %s after router split", path)
		}
	}
}
