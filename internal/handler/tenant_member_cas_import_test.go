package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubCASImport struct {
	configured bool
	preview    *types.CASImportPreview
	result     *types.CASImportResult
	previewErr error
	importErr  error
	lastRows   []types.CASImportRow
	lastRole   types.TenantRole
}

func (s *stubCASImport) Configured() bool { return s.configured }
func (s *stubCASImport) ParseFile(_ string, _ io.Reader) ([]types.CASImportRow, error) {
	return []types.CASImportRow{{Row: 1, Phone: "13800138000", Name: "张三"}}, nil
}
func (s *stubCASImport) ParsePhonesText(text string) []types.CASImportRow {
	return []types.CASImportRow{{Row: 1, Phone: text}}
}
func (s *stubCASImport) Preview(_ context.Context, _ uint64, rows []types.CASImportRow) (*types.CASImportPreview, error) {
	s.lastRows = rows
	if s.previewErr != nil {
		return nil, s.previewErr
	}
	if s.preview != nil {
		return s.preview, nil
	}
	return &types.CASImportPreview{Total: len(rows), Importable: len(rows), Rows: nil}, nil
}
func (s *stubCASImport) Import(_ context.Context, _ uint64, role types.TenantRole, _ *string, rows []types.CASImportRow) (*types.CASImportResult, error) {
	s.lastRows = rows
	s.lastRole = role
	if s.importErr != nil {
		return nil, s.importErr
	}
	if s.result != nil {
		return s.result, nil
	}
	return &types.CASImportResult{Total: len(rows), Imported: len(rows), Role: role}, nil
}

var _ interfaces.CASMemberImportService = (*stubCASImport)(nil)

func casImportTestRouter(h *TenantMemberHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(errorCapture())
	g := r.Group("/tenants/:id", middleware.RequirePathTenantMatch(&config.Config{
		Tenant: &config.TenantConfig{EnableCrossTenantAccess: true},
	}))
	g.POST("/members/cas-import/preview", h.PreviewCASImport)
	g.POST("/members/cas-import", h.ConfirmCASImport)
	return r
}

func TestPreviewCASImportUnconfigured503(t *testing.T) {
	h := NewTenantMemberHandler(&stubMemberService{}, &stubMemberUserService{})
	r := casImportTestRouter(h)
	w := doJSON(t, r, http.MethodPost, "/tenants/1/members/cas-import/preview",
		map[string]any{"phones": []string{"13800138000"}}, "u-owner")
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPreviewCASImportJSONOK(t *testing.T) {
	imp := &stubCASImport{configured: true}
	h := NewTenantMemberHandler(&stubMemberService{}, &stubMemberUserService{}).WithCASImport(imp)
	r := casImportTestRouter(h)
	w := doJSON(t, r, http.MethodPost, "/tenants/1/members/cas-import/preview",
		map[string]any{"phones": []string{"13800138000"}, "names": []string{"张三"}}, "u-owner")
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, imp.lastRows, 1)
	require.Equal(t, "13800138000", imp.lastRows[0].Phone)
	require.Equal(t, "张三", imp.lastRows[0].Name)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, true, body["success"])
}

func TestConfirmCASImportRejectsOwnerRole(t *testing.T) {
	imp := &stubCASImport{configured: true}
	h := NewTenantMemberHandler(&stubMemberService{}, &stubMemberUserService{}).WithCASImport(imp)
	r := casImportTestRouter(h)
	w := doJSON(t, r, http.MethodPost, "/tenants/1/members/cas-import",
		map[string]any{"phones": []string{"13800138000"}, "role": "owner"}, "u-owner")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestConfirmCASImportOK(t *testing.T) {
	imp := &stubCASImport{configured: true}
	h := NewTenantMemberHandler(&stubMemberService{}, &stubMemberUserService{}).WithCASImport(imp)
	r := casImportTestRouter(h)
	w := doJSON(t, r, http.MethodPost, "/tenants/1/members/cas-import",
		map[string]any{"phones": []string{"13800138000"}, "role": "contributor"}, "u-owner")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, types.TenantRoleContributor, imp.lastRole)
}

func TestConfirmCASImportEmpty400(t *testing.T) {
	imp := &stubCASImport{configured: true}
	h := NewTenantMemberHandler(&stubMemberService{}, &stubMemberUserService{}).WithCASImport(imp)
	r := casImportTestRouter(h)
	w := doJSON(t, r, http.MethodPost, "/tenants/1/members/cas-import",
		map[string]any{"phones": []string{}, "role": "contributor"}, "u-owner")
	require.Equal(t, http.StatusBadRequest, w.Code)
}
