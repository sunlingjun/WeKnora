package handler

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	secutils "github.com/Tencent/WeKnora/internal/utils"
)

type casImportJSONRequest struct {
	Phones []string         `json:"phones"`
	Names  []string         `json:"names"`
	Role   types.TenantRole `json:"role"`
	Text   string           `json:"phones_text"`
}

// PreviewCASImport godoc
// @Summary      预览导入农信用户
// @Description  按手机号查询农信用户中心并判断知识库/本空间状态，不写库
// @Tags         空间成员
// @Accept       json
// @Accept       mpfd
// @Produce      json
// @Param        id path string true "空间 ID"
// @Success      200 {object} map[string]interface{}
// @Failure      503 {object} map[string]interface{}
// @Security     Bearer
// @Router       /tenants/{id}/members/cas-import/preview [post]
func (h *TenantMemberHandler) PreviewCASImport(c *gin.Context) {
	tenantID, rows, role, ok := h.readCASImportRequest(c, false)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	preview, err := h.casImport.Preview(ctx, tenantID, rows)
	if err != nil {
		h.writeCASImportError(c, err)
		return
	}
	if role != "" {
		preview.Role = role
	} else {
		preview.Role = types.TenantRoleContributor
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": preview})
}

// ConfirmCASImport godoc
// @Summary      确认导入农信用户
// @Description  重新查询农信后写入：已有账号只加成员，无账号才按 AutoBindUser 建号
// @Tags         空间成员
// @Accept       json
// @Accept       mpfd
// @Produce      json
// @Param        id path string true "空间 ID"
// @Success      200 {object} map[string]interface{}
// @Failure      503 {object} map[string]interface{}
// @Security     Bearer
// @Router       /tenants/{id}/members/cas-import [post]
func (h *TenantMemberHandler) ConfirmCASImport(c *gin.Context) {
	tenantID, rows, role, ok := h.readCASImportRequest(c, true)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	caller, _ := types.UserIDFromContext(ctx)
	var invitedBy *string
	if caller != "" && !types.IsSyntheticUserID(caller) {
		invitedBy = &caller
	}
	result, err := h.casImport.Import(ctx, tenantID, role, invitedBy, rows)
	if err != nil {
		h.writeCASImportError(c, err)
		return
	}
	logger.Infof(ctx, "CAS member import done tenant=%d imported=%d skipped=%d failed=%d actor=%s",
		tenantID, result.Imported, result.Skipped, result.Failed, secutils.SanitizeForLog(caller))
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func (h *TenantMemberHandler) readCASImportRequest(c *gin.Context, requireRole bool) (uint64, []types.CASImportRow, types.TenantRole, bool) {
	if h == nil || h.casImport == nil || !h.casImport.Configured() {
		c.Error(apperrors.NewServiceUnavailableError("农信用户导入未配置"))
		return 0, nil, "", false
	}
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return 0, nil, "", false
	}

	rows, role, err := parseCASImportBody(c, h.casImport)
	if err != nil {
		c.Error(apperrors.NewValidationError(err.Error()))
		return 0, nil, "", false
	}
	if requireRole {
		if err := validateCASImportRole(role); err != nil {
			c.Error(apperrors.NewValidationError(err.Error()))
			return 0, nil, "", false
		}
	}
	if len(rows) == 0 {
		c.Error(apperrors.NewValidationError(service.ErrCASImportEmpty.Error()))
		return 0, nil, "", false
	}
	if len(rows) > types.CASImportMaxRows {
		c.Error(apperrors.NewValidationError(service.ErrCASImportTooManyRows.Error()))
		return 0, nil, "", false
	}
	return tenantID, rows, role, true
}

func validateCASImportRole(role types.TenantRole) error {
	if role == types.TenantRoleOwner {
		return service.ErrCASImportOwnerRole
	}
	if role != types.TenantRoleAdmin && role != types.TenantRoleContributor && role != types.TenantRoleViewer {
		return service.ErrCASImportInvalidRole
	}
	return nil
}

func parseCASImportBody(c *gin.Context, svc interface {
	ParseFile(string, io.Reader) ([]types.CASImportRow, error)
	ParsePhonesText(string) []types.CASImportRow
}) ([]types.CASImportRow, types.TenantRole, error) {
	ct := c.ContentType()
	if strings.Contains(ct, "multipart/form-data") {
		return parseCASImportMultipart(c, svc)
	}
	var req casImportJSONRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, "", errors.New("invalid request body")
	}
	rows := zipCASImportPhones(req.Phones, req.Names)
	if req.Text != "" {
		rows = append(rows, svc.ParsePhonesText(req.Text)...)
	}
	return rows, req.Role, nil
}

func parseCASImportMultipart(c *gin.Context, svc interface {
	ParseFile(string, io.Reader) ([]types.CASImportRow, error)
	ParsePhonesText(string) []types.CASImportRow
}) ([]types.CASImportRow, types.TenantRole, error) {
	role := types.TenantRole(strings.TrimSpace(c.PostForm("role")))
	var rows []types.CASImportRow
	if fh, err := c.FormFile("file"); err == nil && fh != nil {
		f, err := fh.Open()
		if err != nil {
			return nil, "", err
		}
		defer f.Close()
		parsed, err := svc.ParseFile(filepath.Base(fh.Filename), f)
		if err != nil {
			return nil, "", err
		}
		rows = append(rows, parsed...)
	}
	if phones := strings.TrimSpace(c.PostForm("phones")); phones != "" {
		rows = append(rows, svc.ParsePhonesText(phones)...)
	}
	return rows, role, nil
}

func zipCASImportPhones(phones, names []string) []types.CASImportRow {
	out := make([]types.CASImportRow, 0, len(phones))
	for i, phone := range phones {
		name := ""
		if i < len(names) {
			name = names[i]
		}
		phone = strings.TrimSpace(phone)
		name = strings.TrimSpace(name)
		if phone == "" && name == "" {
			continue
		}
		out = append(out, types.CASImportRow{Row: i + 1, Phone: phone, Name: name})
	}
	return out
}

func (h *TenantMemberHandler) writeCASImportError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrCASImportNotConfigured):
		c.Error(apperrors.NewServiceUnavailableError(err.Error()))
	case errors.Is(err, service.ErrCASImportTooManyRows),
		errors.Is(err, service.ErrCASImportEmpty),
		errors.Is(err, service.ErrCASImportInvalidRole),
		errors.Is(err, service.ErrCASImportOwnerRole),
		errors.Is(err, service.ErrCASImportUnsupported):
		c.Error(apperrors.NewValidationError(err.Error()))
	default:
		logger.Errorf(c.Request.Context(), "CAS member import failed: %v", err)
		c.Error(apperrors.NewInternalServerError("failed to import 农信 users").WithDetails(err.Error()))
	}
}
