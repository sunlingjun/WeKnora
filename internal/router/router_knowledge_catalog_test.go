package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

func TestKnowledgeCatalogAPIKeyWithoutRetrieveIsForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		scope := types.TenantAPIKeyScope{
			Capabilities: types.StringArray{string(types.APIKeyCapabilityIngest)},
		}
		c.Request = c.Request.WithContext(types.WithTenantAPIKeyScope(c.Request.Context(), scope))
		c.Next()
	})
	engine.Use(g.ensureAPIKeyAuthorizer().Middleware())
	RegisterKnowledgeCatalogRoutes(engine.Group("/api/v1"), &handler.KnowledgeCatalogHandler{}, g)

	paths := []string{
		"/api/v1/knowledge-catalog/knowledge-bases",
		"/api/v1/knowledge-catalog/knowledge?kb_id=kb-1",
	}
	for _, path := range paths {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s status=%d want 403 body=%s", path, w.Code, w.Body.String())
		}
	}
}

func TestKnowledgeCatalogExistingListRoutesStillDeclareRetrieve(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	v1 := gin.New().Group("/api/v1")
	RegisterKnowledgeBaseRoutes(v1, &handler.KnowledgeBaseHandler{}, g)
	RegisterKnowledgeCatalogRoutes(v1, &handler.KnowledgeCatalogHandler{}, g)
	RegisterKnowledgeRoutes(v1, &handler.KnowledgeHandler{}, g)

	existing := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/knowledge-bases"},
		{http.MethodGet, "/api/v1/knowledge-bases/shared"},
		{http.MethodGet, "/api/v1/knowledge-bases/:id/knowledge"},
	}
	for _, tc := range existing {
		policy := mustLookupAPIKeyPolicy(t, g, tc.method, tc.path)
		if !policyHasCapability(policy, types.APIKeyCapabilityRetrieve) {
			t.Fatalf("%s %s capabilities = %#v, want retrieve (existing list routes must stay unchanged)",
				tc.method, tc.path, policy.Capabilities)
		}
	}
}
