package handler

import (
	"net/http"
	"strings"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

const openRetrieveChunksMediaMaxIDs = 64

// OpenRetrieveChunksMediaRequest is the JSON body for batch chunk media resolve.
type OpenRetrieveChunksMediaRequest struct {
	ChunkIDs []string `json:"chunk_ids"`
}

// OpenRetrieveChunkMediaItem is one chunk with browser-loadable images.
type OpenRetrieveChunkMediaItem struct {
	ChunkID         string                  `json:"chunk_id"`
	KnowledgeBaseID string                  `json:"knowledge_base_id,omitempty"`
	Images          []types.OpenRetrieveImage `json:"images,omitempty"`
}

// ChunksMedia resolves public image URLs for chunks by id (no user login).
func (h *OpenRetrieveHandler) ChunksMedia(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())

	var req OpenRetrieveChunksMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	if len(req.ChunkIDs) == 0 {
		c.Error(errors.NewBadRequestError("chunk_ids is required"))
		return
	}
	if len(req.ChunkIDs) > openRetrieveChunksMediaMaxIDs {
		c.Error(errors.NewBadRequestError("chunk_ids exceeds limit"))
		return
	}

	rewriter, err := resolveOpenRetrieveResourceRewriter(c, h.fileService, h.storageResolver)
	if err != nil {
		logger.Warnf(ctx, "Rejected open chunks media resource URL mode: %v", err)
		_ = c.Error(err)
		return
	}

	ids := dedupeNonEmptyChunkIDs(req.ChunkIDs)
	if len(ids) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []OpenRetrieveChunkMediaItem{}})
		return
	}

	if h.chunkRepo == nil {
		c.Error(errors.NewInternalServerError("chunk repository not configured"))
		return
	}

	chunks, err := h.chunkRepo.ListChunksByIDOnly(ctx, ids)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	refs := make([]*types.SearchResult, 0, len(chunks))
	chunkOrder := make(map[string]int, len(ids))
	for i, id := range ids {
		chunkOrder[id] = i
	}
	for _, chunk := range chunks {
		if chunk == nil || strings.TrimSpace(chunk.ID) == "" {
			continue
		}
		refs = append(refs, chunkToOpenRetrieveSearchResult(chunk))
	}

	rewritten := buildOpenRetrieveChunks(ctx, refs, rewriter)
	out := make([]OpenRetrieveChunkMediaItem, 0, len(rewritten))
	for _, ch := range rewritten {
		chunkID := strings.TrimSpace(ch.ID)
		if chunkID == "" {
			continue
		}
		images := ch.Images
		for i := range images {
			images[i].ChunkID = chunkID
		}
		out = append(out, OpenRetrieveChunkMediaItem{
			ChunkID:         chunkID,
			KnowledgeBaseID: strings.TrimSpace(ch.KnowledgeBaseID),
			Images:          images,
		})
	}

	// Preserve request order for stable clients.
	sortChunkMediaItems(out, chunkOrder)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    out,
	})
}

func chunkToOpenRetrieveSearchResult(chunk *types.Chunk) *types.SearchResult {
	return &types.SearchResult{
		ID:              chunk.ID,
		Content:         chunk.Content,
		KnowledgeID:     chunk.KnowledgeID,
		KnowledgeBaseID: chunk.KnowledgeBaseID,
		ChunkIndex:      chunk.ChunkIndex,
		ImageInfo:       chunk.ImageInfo,
		ChunkType:       string(chunk.ChunkType),
		ParentChunkID:   chunk.ParentChunkID,
	}
}

func dedupeNonEmptyChunkIDs(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, id := range raw {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func sortChunkMediaItems(items []OpenRetrieveChunkMediaItem, order map[string]int) {
	if len(items) < 2 {
		return
	}
	// Simple insertion sort — batch size is capped at 64.
	for i := 1; i < len(items); i++ {
		j := i
		for j > 0 && order[items[j].ChunkID] < order[items[j-1].ChunkID] {
			items[j], items[j-1] = items[j-1], items[j]
			j--
		}
	}
}
