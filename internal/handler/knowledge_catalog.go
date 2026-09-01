package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// KnowledgeCatalogHandler serves the independent workspace knowledge catalog APIs.
type KnowledgeCatalogHandler struct {
	svc interfaces.KnowledgeCatalogService
}

// NewKnowledgeCatalogHandler constructs the catalog HTTP handler.
func NewKnowledgeCatalogHandler(svc interfaces.KnowledgeCatalogService) *KnowledgeCatalogHandler {
	return &KnowledgeCatalogHandler{svc: svc}
}

// ListKnowledgeBases GET /api/v1/knowledge-catalog/knowledge-bases
func (h *KnowledgeCatalogHandler) ListKnowledgeBases(c *gin.Context) {
	ctx := c.Request.Context()
	includeOrgShared, err := parseCatalogBoolDefaultTrue(c.Query("include_org_shared"))
	if err != nil {
		c.Error(apperrors.NewBadRequestError("invalid include_org_shared"))
		return
	}
	kbType := strings.TrimSpace(c.Query("type"))
	if kbType != "" &&
		kbType != types.KnowledgeBaseTypeDocument &&
		kbType != types.KnowledgeBaseTypeFAQ &&
		kbType != types.KnowledgeBaseTypeWiki {
		c.Error(apperrors.NewBadRequestError("invalid type"))
		return
	}

	result, err := h.svc.ListAuthorizedCatalogKBs(ctx, types.ListCatalogKBsQuery{
		IncludeOrgShared: includeOrgShared,
		Type:             kbType,
	})
	if err != nil {
		logger.Errorf(ctx, "catalog list knowledge bases failed: %v", err)
		c.Error(mapCatalogError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// ListKnowledge GET /api/v1/knowledge-catalog/knowledge
func (h *KnowledgeCatalogHandler) ListKnowledge(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := strings.TrimSpace(c.Query("kb_id"))
	if kbID == "" {
		c.Error(apperrors.NewBadRequestError("kb_id is required"))
		return
	}

	limit := types.CatalogKnowledgeDefaultLimit
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			c.Error(apperrors.NewBadRequestError("invalid limit"))
			return
		}
		limit = parsed
	}
	if limit < 1 || limit > types.CatalogKnowledgeMaxLimit {
		c.Error(apperrors.NewBadRequestError("invalid limit"))
		return
	}

	var updatedAfter time.Time
	if raw := strings.TrimSpace(c.Query("updated_after")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339Nano, raw)
		}
		if err != nil {
			c.Error(apperrors.NewBadRequestError("invalid updated_after"))
			return
		}
		updatedAfter = parsed
	}

	result, err := h.svc.ListCatalogKnowledge(ctx, types.ListCatalogKnowledgeQuery{
		KBID:         kbID,
		Limit:        limit,
		Cursor:       strings.TrimSpace(c.Query("cursor")),
		UpdatedAfter: updatedAfter,
		ParseStatus:  strings.TrimSpace(c.Query("parse_status")),
	})
	if err != nil {
		if !errors.Is(err, service.ErrCatalogKBNotFound) {
			logger.Errorf(ctx, "catalog list knowledge failed: kb=%s err=%v", kbID, err)
		}
		c.Error(mapCatalogError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func parseCatalogBoolDefaultTrue(raw string) (bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return true, nil
	}
	return strconv.ParseBool(raw)
}

func mapCatalogError(err error) error {
	switch {
	case errors.Is(err, service.ErrCatalogMissingKBID),
		errors.Is(err, service.ErrCatalogInvalidLimit),
		errors.Is(err, service.ErrCatalogInvalidCursor):
		return apperrors.NewBadRequestError(err.Error())
	case errors.Is(err, service.ErrCatalogKBNotFound):
		return apperrors.NewNotFoundError("Knowledge base not found")
	default:
		return apperrors.NewInternalServerError("Failed to list knowledge catalog")
	}
}
