package handler

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestDedupeNonEmptyChunkIDs(t *testing.T) {
	t.Parallel()
	out := dedupeNonEmptyChunkIDs([]string{" a ", "a", "", "b", "b"})
	require.Equal(t, []string{"a", "b"}, out)
}

func TestSortChunkMediaItemsPreservesRequestOrder(t *testing.T) {
	t.Parallel()
	order := map[string]int{"c1": 2, "c0": 0, "c2": 1}
	items := []OpenRetrieveChunkMediaItem{
		{ChunkID: "c1"},
		{ChunkID: "c0"},
		{ChunkID: "c2"},
	}
	sortChunkMediaItems(items, order)
	require.Equal(t, []string{"c0", "c2", "c1"}, []string{items[0].ChunkID, items[1].ChunkID, items[2].ChunkID})
}

func TestEnrichOpenRetrieveMediaRef_FallsBackToParentText(t *testing.T) {
	t.Parallel()
	child := &types.Chunk{
		ID:              "c98ad5aa-ca9b-4eed-a7f6-b32a7e1eaf73",
		KnowledgeBaseID: "kb-1",
		ChunkType:       types.ChunkTypeText,
		ParentChunkID:   "848641f4-9bf0-4cc2-a0d6-1db58904201f",
		Content:         "信： 信用信任， 信心信念， 根在信仰",
	}
	parent := &types.Chunk{
		ID:        "848641f4-9bf0-4cc2-a0d6-1db58904201f",
		ChunkType: types.ChunkTypeParentText,
		Content: "父块正文\n\n![2025.06.06 信和诚_p1_img1.jpg](resource://s21Vg3j-PK8V-yGf-qPVYQ)\n\n" +
			"![2025.06.06 信和诚_p2_img1.jpg](resource://AuhLPtKZAefhZqgzPCA2GQ)\n",
	}
	ref := enrichOpenRetrieveMediaRef(child, map[string]*types.Chunk{parent.ID: parent})
	require.Equal(t, child.ID, ref.ID)
	require.Equal(t, "kb-1", ref.KnowledgeBaseID)
	require.Contains(t, ref.Content, "resource://s21Vg3j-PK8V-yGf-qPVYQ")
	require.Contains(t, ref.Content, "resource://AuhLPtKZAefhZqgzPCA2GQ")
}

func TestEnrichOpenRetrieveMediaRef_KeepsLeafWhenItAlreadyHasImages(t *testing.T) {
	t.Parallel()
	child := &types.Chunk{
		ID:            "694830ae-5f4f-4f78-a6c8-dbc1b936fc75",
		ChunkType:     types.ChunkTypeText,
		ParentChunkID: "parent-other",
		Content:       "![2025.06.05 安和定_p1_img1.jpg](resource://leaf-only)",
	}
	parent := &types.Chunk{
		ID:        "parent-other",
		ChunkType: types.ChunkTypeParentText,
		Content:   "![parent.jpg](resource://parent-only)",
	}
	ref := enrichOpenRetrieveMediaRef(child, map[string]*types.Chunk{parent.ID: parent})
	require.Contains(t, ref.Content, "resource://leaf-only")
	require.NotContains(t, ref.Content, "resource://parent-only")
}

func TestCollectParentIDsForMediaFallback(t *testing.T) {
	t.Parallel()
	chunks := []*types.Chunk{
		{
			ID:            "leaf-no-img",
			ChunkType:     types.ChunkTypeText,
			ParentChunkID: "parent-a",
			Content:       "只有文字",
		},
		{
			ID:            "leaf-with-img",
			ChunkType:     types.ChunkTypeText,
			ParentChunkID: "parent-b",
			Content:       "![a](resource://x)",
		},
		{
			ID:        "already-parent",
			ChunkType: types.ChunkTypeParentText,
			Content:   "![p](resource://y)",
		},
	}
	require.Equal(t, []string{"parent-a"}, collectParentIDsForMediaFallback(chunks))
}

func TestEnrichOpenRetrieveMediaRef_XinHeChengRegression(t *testing.T) {
	t.Parallel()
	// Mirrors production: leaf text has no markdown images; parent_text does.
	// After enrich, collectOpenRetrieveImages (post-rewrite) must keep leaf ID.
	child := &types.Chunk{
		ID:              "c98ad5aa-ca9b-4eed-a7f6-b32a7e1eaf73",
		KnowledgeBaseID: "c18d35bf-7e45-4bfa-86e6-7001f59f700a",
		ChunkType:       types.ChunkTypeText,
		ParentChunkID:   "848641f4-9bf0-4cc2-a0d6-1db58904201f",
		Content:         "信： 信用信任， 信心信念， 根在信仰",
	}
	parent := &types.Chunk{
		ID:        "848641f4-9bf0-4cc2-a0d6-1db58904201f",
		ChunkType: types.ChunkTypeParentText,
		Content: "![2025.06.06 信和诚_p1_img1.jpg](https://cdn.example.com/p1.jpg)\n" +
			"![2025.06.06 信和诚_p2_img1.jpg](https://cdn.example.com/p2.jpg)\n",
	}
	ref := enrichOpenRetrieveMediaRef(child, map[string]*types.Chunk{parent.ID: parent})
	images := collectOpenRetrieveImages(*ref)
	require.Len(t, images, 2)
	require.Equal(t, child.ID, images[0].ChunkID)
	require.Equal(t, "2025.06.06 信和诚_p1_img1.jpg", images[0].Alt)
	require.Equal(t, "https://cdn.example.com/p1.jpg", images[0].URL)
	require.Equal(t, "https://cdn.example.com/p2.jpg", images[1].URL)
}
