package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/searchutil"
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

	parentsByID := map[string]*types.Chunk{}
	if parentIDs := collectParentIDsForMediaFallback(chunks); len(parentIDs) > 0 {
		parents, parentErr := h.chunkRepo.ListChunksByIDOnly(ctx, parentIDs)
		if parentErr != nil {
			logger.Warnf(ctx, "open chunks media parent fallback load failed: %v", parentErr)
		} else {
			for _, parent := range parents {
				if parent == nil || strings.TrimSpace(parent.ID) == "" {
					continue
				}
				parentsByID[parent.ID] = parent
			}
		}
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
		refs = append(refs, enrichOpenRetrieveMediaRef(chunk, parentsByID))
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

// enrichOpenRetrieveMediaRef keeps the leaf chunk identity but, when the leaf
// itself has no image candidates, borrows Content/ImageInfo from its
// parent_text. Chatbot history stores retrieve hit IDs (usually text children)
// while PDF/page images often live only on the parent row.
func enrichOpenRetrieveMediaRef(chunk *types.Chunk, parentsByID map[string]*types.Chunk) *types.SearchResult {
	ref := chunkToOpenRetrieveSearchResult(chunk)
	if chunkHasMediaSourceCandidates(chunk) {
		return ref
	}
	if chunk.ChunkType != types.ChunkTypeText {
		return ref
	}
	parentID := strings.TrimSpace(chunk.ParentChunkID)
	if parentID == "" || parentsByID == nil {
		return ref
	}
	parent := parentsByID[parentID]
	if parent == nil || parent.ChunkType != types.ChunkTypeParentText {
		return ref
	}
	if !chunkHasMediaSourceCandidates(parent) {
		return ref
	}
	ref.Content = parent.Content
	if strings.TrimSpace(ref.ImageInfo) == "" {
		ref.ImageInfo = parent.ImageInfo
	}
	return ref
}

func collectParentIDsForMediaFallback(chunks []*types.Chunk) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, chunk := range chunks {
		if chunk == nil || chunk.ChunkType != types.ChunkTypeText {
			continue
		}
		if chunkHasMediaSourceCandidates(chunk) {
			continue
		}
		parentID := strings.TrimSpace(chunk.ParentChunkID)
		if parentID == "" {
			continue
		}
		if _, ok := seen[parentID]; ok {
			continue
		}
		seen[parentID] = struct{}{}
		out = append(out, parentID)
	}
	return out
}

func chunkHasMediaSourceCandidates(chunk *types.Chunk) bool {
	if chunk == nil {
		return false
	}
	if len(searchutil.MarkdownImageRegex.FindAllStringIndex(chunk.Content, -1)) > 0 {
		return true
	}
	raw := strings.TrimSpace(chunk.ImageInfo)
	if raw == "" || raw == "[]" || raw == "null" {
		return false
	}
	var infos []types.ImageInfo
	if err := json.Unmarshal([]byte(raw), &infos); err != nil {
		return false
	}
	for _, info := range infos {
		if strings.TrimSpace(info.URL) != "" || strings.TrimSpace(info.OriginalURL) != "" {
			return true
		}
	}
	return false
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
