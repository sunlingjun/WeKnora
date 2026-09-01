package handler

import (
	"net/http"
	"strconv"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/handler/dto"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

type WebhookEndpointHandler struct {
	svc interfaces.WebhookEndpointService
}

func NewWebhookEndpointHandler(svc interfaces.WebhookEndpointService) *WebhookEndpointHandler {
	return &WebhookEndpointHandler{svc: svc}
}

func (h *WebhookEndpointHandler) tenantID(c *gin.Context) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return 0, apperrors.NewBadRequestError("Invalid workspace ID")
	}
	return id, nil
}

func (h *WebhookEndpointHandler) List(c *gin.Context) {
	tenantID, err := h.tenantID(c)
	if err != nil {
		c.Error(err)
		return
	}
	rows, err := h.svc.List(c.Request.Context(), tenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows})
}

func (h *WebhookEndpointHandler) ListTypes(c *gin.Context) {
	if _, err := h.tenantID(c); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.svc.EventTypes()})
}

func (h *WebhookEndpointHandler) Create(c *gin.Context) {
	tenantID, err := h.tenantID(c)
	if err != nil {
		c.Error(err)
		return
	}
	var req dto.WebhookEndpointCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	row, err := h.svc.Create(c.Request.Context(), tenantID, interfaces.WebhookEndpointCreate{
		Name:        req.Name,
		URL:         req.URL,
		Secret:      req.Secret,
		Events:      req.Events,
		Enabled:     req.Enabled,
		Description: req.Description,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": row})
}

func (h *WebhookEndpointHandler) Update(c *gin.Context) {
	tenantID, err := h.tenantID(c)
	if err != nil {
		c.Error(err)
		return
	}
	var req dto.WebhookEndpointUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	row, err := h.svc.Update(c.Request.Context(), tenantID, c.Param("hook_id"), interfaces.WebhookEndpointUpdate{
		Name:        req.Name,
		URL:         req.URL,
		Secret:      req.Secret,
		Events:      req.Events,
		Enabled:     req.Enabled,
		Description: req.Description,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": row})
}

func (h *WebhookEndpointHandler) Delete(c *gin.Context) {
	tenantID, err := h.tenantID(c)
	if err != nil {
		c.Error(err)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), tenantID, c.Param("hook_id")); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *WebhookEndpointHandler) Test(c *gin.Context) {
	tenantID, err := h.tenantID(c)
	if err != nil {
		c.Error(err)
		return
	}
	if err := h.svc.Test(c.Request.Context(), tenantID, c.Param("hook_id")); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *WebhookEndpointHandler) ListDeliveries(c *gin.Context) {
	tenantID, err := h.tenantID(c)
	if err != nil {
		c.Error(err)
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	rows, err := h.svc.ListDeliveries(c.Request.Context(), tenantID, c.Param("hook_id"), limit)
	if err != nil {
		c.Error(err)
		return
	}
	if rows == nil {
		rows = []*types.TenantWebhookDelivery{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows})
}
