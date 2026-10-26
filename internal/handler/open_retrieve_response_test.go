package handler

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestCollectOpenRetrieveImages_MarkdownAndImageInfo(t *testing.T) {
	ref := types.SearchResult{
		ID:      "chunk-1",
		Content: "说明 ![字形](https://cdn.example.com/a.png) 结束",
		ImageInfo: `[{"url":"https://cdn.example.com/b.png","caption":"图B"}]`,
	}
	images := collectOpenRetrieveImages(ref)
	assert.Len(t, images, 2)
	assert.Equal(t, "https://cdn.example.com/a.png", images[0].URL)
	assert.Equal(t, "字形", images[0].Alt)
	assert.Equal(t, "chunk-1", images[0].ChunkID)
	urls := map[string]bool{images[0].URL: true, images[1].URL: true}
	assert.True(t, urls["https://cdn.example.com/b.png"])
}

func TestCollectOpenRetrieveImages_SkipsNonHTTP(t *testing.T) {
	ref := types.SearchResult{
		ID:      "chunk-2",
		Content: "![x](resource://handle-only)",
	}
	assert.Empty(t, collectOpenRetrieveImages(ref))
}
