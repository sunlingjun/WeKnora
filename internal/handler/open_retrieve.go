package handler

import (
	"net/http"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// OpenRetrieveHandler serves POST /api/v1/open/knowledge/retrieve (no user login).
type OpenRetrieveHandler struct {
	sessionService interfaces.SessionService
}

// NewOpenRetrieveHandler constructs OpenRetrieveHandler.
func NewOpenRetrieveHandler(sessionService interfaces.SessionService) *OpenRetrieveHandler {
	return &OpenRetrieveHandler{sessionService: sessionService}
}

// OpenRetrieveRequest is the JSON body for open knowledge retrieve.
type OpenRetrieveRequest struct {
	Query            string   `json:"query"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	KnowledgeIDs     []string `json:"knowledge_ids"`
	MatchCount       int      `json:"match_count"`
}

// Retrieve runs hybrid retrieval without user-scoped permission checks.
func (h *OpenRetrieveHandler) Retrieve(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())

	var req OpenRetrieveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	if req.Query == "" {
		c.Error(errors.NewBadRequestError("query is required"))
		return
	}
	if len(req.KnowledgeBaseIDs) == 0 && len(req.KnowledgeIDs) == 0 {
		c.Error(errors.NewBadRequestError("at least one of knowledge_base_ids or knowledge_ids is required"))
		return
	}

	logger.Infof(ctx, "open knowledge retrieve, kb_count=%d, knowledge_count=%d",
		len(req.KnowledgeBaseIDs), len(req.KnowledgeIDs))

	results, err := h.sessionService.SearchKnowledgeOpen(ctx, req.KnowledgeBaseIDs, req.KnowledgeIDs, req.Query, req.MatchCount)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    results,
	})
}
