package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubCatalogService struct {
	interfaces.KnowledgeCatalogService
	kbs     *types.CatalogKBListResult
	kbsErr  error
	know    *types.CatalogKnowledgeListResult
	knowErr error
	lastKB  string
}

func (s *stubCatalogService) ListAuthorizedCatalogKBs(context.Context, types.ListCatalogKBsQuery) (*types.CatalogKBListResult, error) {
	return s.kbs, s.kbsErr
}

func (s *stubCatalogService) ListCatalogKnowledge(_ context.Context, q types.ListCatalogKnowledgeQuery) (*types.CatalogKnowledgeListResult, error) {
	s.lastKB = q.KBID
	return s.know, s.knowErr
}

func newCatalogRouter(svc interfaces.KnowledgeCatalogService, tenant uint64, role types.TenantRole, scope *types.TenantAPIKeyScope) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenant)
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
		if scope != nil {
			ctx = types.WithTenantAPIKeyScope(ctx, *scope)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	h := NewKnowledgeCatalogHandler(svc)
	r.GET("/knowledge-catalog/knowledge-bases", h.ListKnowledgeBases)
	r.GET("/knowledge-catalog/knowledge", h.ListKnowledge)
	return r
}

func TestCatalogHandler_ListKnowledgeBasesSuccess(t *testing.T) {
	now := time.Date(2026, 9, 1, 1, 0, 0, 0, time.UTC)
	svc := &stubCatalogService{
		kbs: &types.CatalogKBListResult{
			TenantID:    42,
			GeneratedAt: now,
			KnowledgeBases: []types.CatalogKnowledgeBase{
				{ID: "kb-own-1", AccessSource: types.CatalogAccessOwned, OwnerTenantID: 42, CanDownload: true, KnowledgeCount: 2},
			},
			Total: 1,
		},
	}
	scope := &types.TenantAPIKeyScope{KeyID: 1, Capabilities: types.StringArray{string(types.APIKeyCapabilityRetrieve)}}
	r := newCatalogRouter(svc, 42, types.TenantRoleViewer, scope)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/knowledge-catalog/knowledge-bases", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"success":true`)
	require.Contains(t, w.Body.String(), `"kb-own-1"`)
	require.NotContains(t, w.Body.String(), "file_path")
	require.NotContains(t, w.Body.String(), "vector_store_id")
}

func TestCatalogHandler_MissingKBIDIsBadRequest(t *testing.T) {
	r := newCatalogRouter(&stubCatalogService{}, 42, types.TenantRoleViewer, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/knowledge-catalog/knowledge", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCatalogHandler_InvalidLimitIsBadRequest(t *testing.T) {
	r := newCatalogRouter(&stubCatalogService{}, 42, types.TenantRoleViewer, nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/knowledge-catalog/knowledge?kb_id=kb-own-1&limit=0", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/knowledge-catalog/knowledge?kb_id=kb-own-1&limit=501", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCatalogHandler_UnknownKBIsNotFound(t *testing.T) {
	r := newCatalogRouter(&stubCatalogService{knowErr: service.ErrCatalogKBNotFound}, 42, types.TenantRoleViewer, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/knowledge-catalog/knowledge?kb_id=kb-missing", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
	require.NotContains(t, w.Body.String(), "file_path")
}

func TestCatalogHandler_KnowledgeOmitsFilePath(t *testing.T) {
	svc := &stubCatalogService{
		know: &types.CatalogKnowledgeListResult{
			TenantID: 42,
			KBID:     "kb-share-9",
			Items: []types.CatalogKnowledgeItem{
				{ID: "k-1", KnowledgeBaseID: "kb-share-9", KnowledgeType: "file", Title: "报销制度.pdf", HasFile: true},
			},
		},
	}
	r := newCatalogRouter(svc, 42, types.TenantRoleViewer, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/knowledge-catalog/knowledge?kb_id=kb-share-9", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "file_path")
	require.NotContains(t, w.Body.String(), "vector_store_id")

	var payload map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	data, _ := payload["data"].(map[string]any)
	items, _ := data["items"].([]any)
	require.Len(t, items, 1)
	item, _ := items[0].(map[string]any)
	_, hasFilePath := item["file_path"]
	require.False(t, hasFilePath)
	require.Equal(t, "kb-share-9", svc.lastKB)
}

func TestCatalogHandler_InvalidUpdatedAfterIsBadRequest(t *testing.T) {
	r := newCatalogRouter(&stubCatalogService{}, 42, types.TenantRoleViewer, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/knowledge-catalog/knowledge?kb_id=kb-own-1&updated_after=not-a-time", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCatalogHandlerMapsServiceErrors(t *testing.T) {
	require.True(t, errors.Is(service.ErrCatalogKBNotFound, service.ErrCatalogKBNotFound))
	err := apperrors.NewNotFoundError("Knowledge base not found")
	require.Equal(t, 404, err.HTTPCode)
	require.False(t, strings.Contains(err.Message, "file_path"))
}
