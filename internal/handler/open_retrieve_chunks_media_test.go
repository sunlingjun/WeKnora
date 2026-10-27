package handler

import (
	"testing"

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
