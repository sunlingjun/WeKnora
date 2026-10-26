package handler

import (
	"net/http"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// OpenRetrieveHandler serves POST /api/v1/open/knowledge/retrieve (no user login).
type OpenRetrieveHandler struct {
	sessionService  interfaces.SessionService
	fileService     interfaces.FileService
	storageResolver interfaces.StorageBackendResolver
}

// NewOpenRetrieveHandler constructs OpenRetrieveHandler.
func NewOpenRetrieveHandler(
	sessionService interfaces.SessionService,
	fileService interfaces.FileService,
	storageResolver interfaces.StorageBackendResolver,
) *OpenRetrieveHandler {
	return &OpenRetrieveHandler{
		sessionService:  sessionService,
		fileService:     fileService,
		storageResolver: storageResolver,
	}
}

// OpenRetrieveRequest is the JSON body for open knowledge retrieve.
type OpenRetrieveRequest struct {
	Query            string   `json:"query"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	KnowledgeIDs     []string `json:"knowledge_ids"`
	// MatchCount is the maximum number of chunks to return after rerank/merge/filter.
	// Vector/keyword recall volume uses the tenant RetrievalConfig embedding_top_k (not this field).
	MatchCount int `json:"match_count"`
}

// Retrieve runs hybrid retrieval without user-scoped permission checks.
func (h *OpenRetrieveHandler) Retrieve(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	ctx, warnings := types.WithOpenRetrieveWarningCollector(ctx)

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

	rewriter, err := resolveOpenRetrieveResourceRewriter(c, h.fileService, h.storageResolver)
	if err != nil {
		logger.Warnf(ctx, "Rejected open retrieve resource URL mode: %v", err)
		_ = c.Error(err)
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

	data := buildOpenRetrieveChunks(ctx, results, rewriter)

	body := gin.H{
		"success": true,
		"data":    data,
	}
	if items := warnings.Items(); len(items) > 0 {
		body["warnings"] = items
	}
	c.JSON(http.StatusOK, body)
}
