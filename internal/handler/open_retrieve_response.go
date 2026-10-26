package handler

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/searchutil"
	"github.com/Tencent/WeKnora/internal/storageurl"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

func resolveOpenRetrieveResourceRewriter(
	c *gin.Context,
	fileService interfaces.FileService,
	storageResolver interfaces.StorageBackendResolver,
) (*storageurl.Rewriter, error) {
	ctx := c.Request.Context()
	mode, err := storageurl.ResolveMode(ctx, c.Query(storageurl.QueryParam))
	if err != nil {
		if err == storageurl.ErrPublicModeForbidden {
			return nil, errors.NewForbiddenError(err.Error())
		}
		return nil, errors.NewBadRequestError(err.Error())
	}
	return storageurl.NewRequestRewriter(ctx, mode, fileService, storageResolver), nil
}

func buildOpenRetrieveChunks(
	ctx context.Context,
	refs []*types.SearchResult,
	rewriter *storageurl.Rewriter,
) []types.OpenRetrieveChunk {
	if len(refs) == 0 {
		return []types.OpenRetrieveChunk{}
	}
	rewritten := rewriter.CopyReferences(ctx, refs)
	out := make([]types.OpenRetrieveChunk, 0, len(rewritten))
	for _, ref := range rewritten {
		if ref == nil {
			continue
		}
		out = append(out, types.OpenRetrieveChunk{
			SearchResult: *ref,
			Images:       collectOpenRetrieveImages(*ref),
		})
	}
	return out
}

func collectOpenRetrieveImages(ref types.SearchResult) []types.OpenRetrieveImage {
	chunkID := strings.TrimSpace(ref.ID)
	seen := make(map[string]struct{})
	var images []types.OpenRetrieveImage

	add := func(url, alt string) {
		url = strings.TrimSpace(url)
		if url == "" || !isHTTPURL(url) {
			return
		}
		if _, ok := seen[url]; ok {
			return
		}
		seen[url] = struct{}{}
		images = append(images, types.OpenRetrieveImage{
			URL:     url,
			Alt:     strings.TrimSpace(alt),
			ChunkID: chunkID,
		})
	}

	for _, match := range searchutil.MarkdownImageRegex.FindAllStringSubmatch(ref.Content, -1) {
		if len(match) < 3 {
			continue
		}
		add(match[2], match[1])
	}

	if ref.ImageInfo != "" {
		var infos []types.ImageInfo
		if err := json.Unmarshal([]byte(ref.ImageInfo), &infos); err == nil {
			for _, info := range infos {
				alt := strings.TrimSpace(info.Caption)
				if alt == "" {
					alt = strings.TrimSpace(info.OCRText)
				}
				if info.URL != "" {
					add(info.URL, alt)
				}
				if info.OriginalURL != "" && info.OriginalURL != info.URL {
					add(info.OriginalURL, alt)
				}
			}
		}
	}

	return images
}

func isHTTPURL(raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

